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
	kubeClient "k8s.io/client-go/kubernetes"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	connectivityManager "arhat.dev/aranya/pkg/node/connectivity/manager"
	"arhat.dev/aranya/pkg/node/pod"
	"arhat.dev/aranya/pkg/node/util"
)

var (
	log = logf.Log.WithName("aranya.node")
)

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

	logger := log.WithValues("node", opt.NodeObject.Name)

	m := &mux.Router{NotFoundHandler: util.NotFoundHandler(logger)}
	// register http routes
	m.Use(util.LogMiddleware(logger), util.PanicRecoverMiddleware(logger))
	m.StrictSlash(true)
	//
	// routes for pod
	//
	// containerLogs (kubectl logs)
	m.HandleFunc("/containerLogs/{namespace}/{name}/{containerName}", podManager.HandlePodContainerLog).Methods(http.MethodGet)
	m.HandleFunc("/containerLogs/{namespace}/{name}/{uid}/{containerName}", podManager.HandlePodContainerLog).Methods(http.MethodGet)
	// logs
	m.Handle("/logs/{logpath:*}", http.StripPrefix("/logs/", http.FileServer(http.Dir("/var/log/")))).Methods(http.MethodGet)
	// exec (kubectl exec)
	m.HandleFunc("/exec/{namespace}/{name}/{containerName}", podManager.HandlePodExec).Methods(http.MethodPost, http.MethodGet)
	m.HandleFunc("/exec/{namespace}/{name}/{uid}/{containerName}", podManager.HandlePodExec).Methods(http.MethodPost, http.MethodGet)
	// attach (kubectl attach)
	m.HandleFunc("/attach/{namespace}/{name}/{containerName}", podManager.HandlePodAttach).Methods(http.MethodPost, http.MethodGet)
	m.HandleFunc("/attach/{namespace}/{name}/{uid}/{containerName}", podManager.HandlePodAttach).Methods(http.MethodPost, http.MethodGet)
	// portForward (kubectl proxy)
	m.HandleFunc("/portForward/{namespace}/{name}", podManager.HandlePodPortForward).Methods(http.MethodPost, http.MethodGet)
	m.HandleFunc("/portForward/{namespace}/{name}/{uid}", podManager.HandlePodPortForward).Methods(http.MethodPost, http.MethodGet)

	//
	// routes for stats
	//
	// stats summary
	// m.HandleFunc("/stats/summary", statsManager.HandleStatsSummary).Methods(http.MethodGet)

	// TODO: handle metrics

	srv := &Node{
		opt: opt,

		log:  logger,
		ctx:  ctx,
		exit: exit,
		name: opt.NodeObject.Name,

		kubeClient:          opt.KubeClient,
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

	kubeClient          kubeClient.Interface
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
			err := n.kubeletSrv.Serve(n.opt.KubeletServerListener)
			if err != nil && err != http.ErrServerClosed {
				n.log.Error(err, "failed to start kubelet http server")
			}
		}()

		go func() {
			n.log.Info("starting connectivity manager")
			err := n.connectivityManager.Start()
			if err != nil {
				n.log.Error(err, "failed to start connectivity manager")
			}
		}()

		go func() {
			err := n.podManager.Start()
			if err != nil {
				n.log.Error(err, "failed to start pod manager")
			}
		}()

		go n.InitializeRemoteDevice()

		return
	})
	return
}

// ForceClose close this node immediately
func (n *Node) ForceClose() {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.status == statusRunning {
		n.log.Info("force close virtual node")
		_ = n.kubeletSrv.Close()
		n.connectivityManager.Close()

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
