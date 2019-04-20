package node

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"

	"github.com/go-logr/logr"
	"github.com/gorilla/mux"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	kubeClient "k8s.io/client-go/kubernetes"
	kubeNodeClient "k8s.io/client-go/kubernetes/typed/core/v1"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	"arhat.dev/aranya/pkg/constant"
	connectivityManager "arhat.dev/aranya/pkg/node/manager"
	"arhat.dev/aranya/pkg/node/pod"
	"arhat.dev/aranya/pkg/node/util"
)

var log = logf.Log.WithName("aranya.node")

type CreationOptions struct {
	// required fields
	NodeObject            *corev1.Node
	KubeClient            kubeClient.Interface
	KubeletServerListener net.Listener

	ConnectivityManager connectivityManager.Interface

	// optional
	GRPCServerListener net.Listener
}

func CreateVirtualNode(parentCtx context.Context, opt *CreationOptions) (*Node, error) {
	ctx, exit := context.WithCancel(parentCtx)

	podManager := pod.NewManager(ctx, opt.NodeObject.Name, opt.KubeClient, opt.ConnectivityManager)

	logger := log.WithValues("name", opt.NodeObject.Name)

	m := &mux.Router{NotFoundHandler: util.NotFoundHandler(logger)}
	// register http routes
	m.Use(util.LogMiddleware(logger), util.PanicRecoverMiddleware(logger))
	m.StrictSlash(true)
	//
	// routes for pod
	//
	// containerLogs (kubectl logs)
	m.HandleFunc("/containerLogs/{namespace}/{name}/{container}", podManager.HandlePodContainerLog).Methods(http.MethodGet)
	// logs
	m.Handle("/logs/{logpath:*}", http.StripPrefix("/logs/", http.FileServer(http.Dir("/var/log/")))).Methods(http.MethodGet)
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

	srv := &Node{
		opt: opt,

		log:  logger,
		ctx:  ctx,
		exit: exit,
		name: opt.NodeObject.Name,

		kubeNodeClient:      opt.KubeClient.CoreV1().Nodes(),
		connectivityManager: opt.ConnectivityManager,

		kubeletSrv:      &http.Server{Handler: m},
		podManager:      podManager,
		nodeStatusCache: newNodeCache(opt.NodeObject.Status),
	}

	return srv, nil
}

type Node struct {
	once sync.Once
	opt  *CreationOptions

	log  logr.Logger
	ctx  context.Context
	exit context.CancelFunc
	name string

	kubeNodeClient      kubeNodeClient.NodeInterface
	connectivityManager connectivityManager.Interface

	kubeletSrv      *http.Server
	podManager      *pod.Manager
	nodeStatusCache *NodeCache

	// status
	status uint32
	mu     sync.RWMutex
}

func (n *Node) Start() (err error) {
	err = fmt.Errorf("server has started, do not start again")
	n.once.Do(func() {
		if err = Add(n); err != nil {
			n.log.Error(err, "failed to add virtual node to collections")
			return
		}

		// start a kubelet http server
		go func() {
			n.log.Info("starting kubelet http server")
			if err := n.kubeletSrv.Serve(n.opt.KubeletServerListener); err != nil {
				n.log.Error(err, "failed to start kubelet http server")
			}
		}()

		go func() {
			n.log.Info("starting connectivity manager")
			if err := n.connectivityManager.Start(); err != nil {
				n.log.Error(err, "failed to start connectivity manager")
			}
		}()

		go func() {
			n.log.Info("starting pod manager")
			if err := n.podManager.Start(); err != nil {
				n.log.Error(err, "failed to start pod manager")
			}
		}()

		// initialize remote device
		go func() {
			for !n.closing() {
				select {
				case <-n.connectivityManager.DeviceConnected():
					// we are good to go
				case <-n.ctx.Done():
					return
				}

				n.log.Info("device connected, starting to handle")

				stopCh := make(chan struct{})
				go func() {
					for {
						select {
						case <-stopCh:
							return
						case <-n.ctx.Done():
							return
						case msg, more := <-n.connectivityManager.GlobalMessages():
							if !more {
								return
							}
							n.handleGlobalMsg(msg)
						}
					}
				}()

				go wait.Until(n.syncNodeStatus, constant.DefaultNodeStatusSyncInterval, stopCh)

				n.log.Info("trying to sync device pods")
				if err := n.podManager.SyncDevicePods(); err != nil {
					n.log.Error(err, "failed to sync device pods")
					goto waitForDeviceDisconnect
				}

				n.log.Info("trying to sync device info")
				if err := n.generateCacheForNodeInDevice(); err != nil {
					n.log.Error(err, "failed to sync device node info")
					goto waitForDeviceDisconnect
				}

			waitForDeviceDisconnect:
				close(stopCh)

				select {
				case <-n.connectivityManager.DeviceDisconnected():
					continue
				case <-n.ctx.Done():
					return
				}
			}
		}()
	})

	return err
}

// ForceClose close this node immediately
func (n *Node) ForceClose() {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.status == statusRunning {
		n.log.Info("force close virtual node")
		_ = n.kubeletSrv.Close()
		n.connectivityManager.Stop()

		n.exit()
	}
}

func (n *Node) CreationOptions() CreationOptions {
	n.mu.RLock()
	defer n.mu.RUnlock()

	return *n.opt
}

func (n *Node) closing() bool {
	select {
	case <-n.ctx.Done():
		return true
	default:
		return false
	}
}
