package virtualnode

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"

	"github.com/go-logr/logr"
	"github.com/gorilla/mux"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	kubeClient "k8s.io/client-go/kubernetes"
	kubeNodeClient "k8s.io/client-go/kubernetes/typed/core/v1"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	"arhat.dev/aranya/pkg/constant"
	"arhat.dev/aranya/pkg/virtualnode/manager"
	"arhat.dev/aranya/pkg/virtualnode/pod"
	"arhat.dev/aranya/pkg/virtualnode/util"
)

var log = logf.Log.WithName("node")

type CreationOptions struct {
	// required fields
	NodeObject            *corev1.Node
	KubeClient            kubeClient.Interface
	KubeletServerListener net.Listener

	Manager manager.Manager

	// optional
	GRPCServerListener net.Listener
}

func CreateVirtualNode(parentCtx context.Context, opt *CreationOptions) (*VirtualNode, error) {
	ctx, exit := context.WithCancel(parentCtx)
	nodeLogger := log.WithValues("name", opt.NodeObject.Name)

	m := &mux.Router{NotFoundHandler: util.NotFoundHandler(nodeLogger)}
	// register http routes

	m.Use(util.LogMiddleware(nodeLogger), util.PanicRecoverMiddleware(nodeLogger))
	m.StrictSlash(true)
	//
	// routes for pod
	//
	// logs (kubectl cluster-info dump)
	// m.Handle("/logs/{logpath:*}", http.StripPrefix("/logs/", http.FileServer(http.Dir("/var/log/")))).Methods(http.MethodGet)

	// handle pod related
	podManager := pod.NewManager(ctx, opt.NodeObject.Name, opt.KubeClient, opt.Manager)
	// containerLogs (kubectl logs)
	m.HandleFunc("/containerLogs/{namespace}/{name}/{container}", podManager.HandlePodContainerLog).Methods(http.MethodGet)
	// exec (kubectl exec)
	m.HandleFunc("/exec/{namespace}/{name}/{container}", podManager.HandlePodExec).Methods(http.MethodPost, http.MethodGet)
	m.HandleFunc("/exec/{namespace}/{name}/{uid}/{container}", podManager.HandlePodExec).Methods(http.MethodPost, http.MethodGet)
	// attach (kubectl attach)
	m.HandleFunc("/attach/{namespace}/{name}/{container}", podManager.HandlePodAttach).Methods(http.MethodPost, http.MethodGet)
	m.HandleFunc("/attach/{namespace}/{name}/{uid}/{container}", podManager.HandlePodAttach).Methods(http.MethodPost, http.MethodGet)
	// portForward (kubectl proxy)
	m.HandleFunc("/portForward/{namespace}/{name}", podManager.HandlePodPortForward).Methods(http.MethodPost, http.MethodGet)
	m.HandleFunc("/portForward/{namespace}/{name}/{uid}", podManager.HandlePodPortForward).Methods(http.MethodPost, http.MethodGet)

	// TODO: handle metrics
	// m.HandleFunc("/metrics", ).Methods(http.MethodGet)
	// m.HandleFunc("/metrics/cadvisor", ).Methods(http.MethodGet)

	// TODO: handle stats
	// m.HandleFunc("/stats/summary", ).Methods(http.MethodGet)

	srv := &VirtualNode{
		opt: opt,

		log:  nodeLogger,
		ctx:  ctx,
		exit: exit,
		name: opt.NodeObject.Name,

		kubeNodeClient: opt.KubeClient.CoreV1().Nodes(),

		kubeletSrv:      &http.Server{Handler: m},
		podManager:      podManager,
		nodeStatusCache: newNodeCache(opt.NodeObject.Status),
	}

	return srv, nil
}

type VirtualNode struct {
	once sync.Once
	opt  *CreationOptions

	log  logr.Logger
	ctx  context.Context
	exit context.CancelFunc
	name string

	kubeNodeClient kubeNodeClient.NodeInterface

	kubeletSrv      *http.Server
	podManager      *pod.Manager
	nodeStatusCache *NodeCache

	// status
	status uint32
	mu     sync.RWMutex
}

func (vn *VirtualNode) Start() (err error) {
	err = fmt.Errorf("server has started, do not start again")
	vn.once.Do(func() {
		if err = Add(vn); err != nil {
			vn.log.Error(err, "failed to add virtual node to collections")
			return
		}

		// start a kubelet http server
		go func() {
			vn.log.Info("trying to start kubelet http server")
			defer func() {
				vn.log.Info("kubelet http server exited")

				Delete(vn.name)
			}()

			if err := vn.kubeletSrv.Serve(vn.opt.KubeletServerListener); err != nil && err != http.ErrServerClosed {
				vn.log.Error(err, "failed to start kubelet http server")
				return
			}
		}()

		go func() {
			vn.log.Info("trying to start connectivity manager")
			defer func() {
				vn.log.Info("connectivity manager exited")

				Delete(vn.name)
			}()

			if err := vn.opt.Manager.Start(); err != nil {
				vn.log.Error(err, "failed to start connectivity manager")
				return
			}
		}()

		go func() {
			vn.log.Info("trying to start pod manager")
			defer func() {
				vn.log.Info("pod manager exited")

				Delete(vn.name)
			}()

			if err := vn.podManager.Start(); err != nil {
				vn.log.Error(err, "failed to start pod manager")
				return
			}
		}()

		// initialize remote device
		go func() {
			vn.log.Info("starting to handle device connect")
			defer func() {
				vn.log.Info("stopped waiting for device connect")

				Delete(vn.name)

				vn.log.Info("trying to delete node object by virtual node")
				err := vn.kubeNodeClient.Delete(vn.name, metav1.NewDeleteOptions(0))
				if err != nil && !errors.IsNotFound(err) {
					vn.log.Error(err, "failed to delete node object by virtual node")
				}
			}()

			for !vn.closing() {
				select {
				case <-vn.opt.Manager.Connected():
					// we are good to go
					vn.log.Info("device connected, starting to handle")
				case <-vn.ctx.Done():
					return
				}

				go func() {
					vn.log.Info("starting to handle global messages")
					defer vn.log.Info("stopped handling global messages")

					for {
						select {
						case <-vn.ctx.Done():
							return
						case msg, more := <-vn.opt.Manager.GlobalMessages():
							if !more {
								return
							}
							vn.log.Info("handling global message")
							vn.handleGlobalMsg(msg)
						}
					}
				}()

				vn.log.Info("starting to handle node status update")
				go wait.Until(vn.syncNodeStatus,
					constant.DefaultNodeStatusSyncInterval,
					vn.opt.Manager.Disconnected())

				vn.log.Info("trying to sync device info")
				if err := vn.generateCacheForNodeInDevice(); err != nil {
					vn.log.Error(err, "failed to sync device node info")
					goto waitForDeviceDisconnect
				}

				vn.log.Info("trying to sync device pods")
				if err := vn.podManager.SyncDevicePods(); err != nil {
					vn.log.Error(err, "failed to sync device pods")
					goto waitForDeviceDisconnect
				}
			waitForDeviceDisconnect:
				select {
				case <-vn.opt.Manager.Disconnected():
					vn.log.Info("device disconnected, wait for next connection")
					continue
				case <-vn.ctx.Done():
					return
				}
			}
		}()
	})

	return err
}

// ForceClose close this node immediately
func (vn *VirtualNode) ForceClose() {
	vn.mu.Lock()
	defer vn.mu.Unlock()

	vn.log.Info("force close virtual node")

	_ = vn.kubeletSrv.Close()
	vn.opt.Manager.Stop()
	vn.podManager.Stop()
	vn.exit()
}

func (vn *VirtualNode) CreationOptions() CreationOptions {
	vn.mu.RLock()
	defer vn.mu.RUnlock()

	return *vn.opt
}

func (vn *VirtualNode) closing() bool {
	select {
	case <-vn.ctx.Done():
		return true
	default:
		return false
	}
}
