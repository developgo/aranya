package node

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/gorilla/mux"
	"google.golang.org/grpc"
	corev1 "k8s.io/api/core/v1"
	kubeClient "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	"arhat.dev/aranya/pkg/node/connectivity"
	connectivitySrv "arhat.dev/aranya/pkg/node/connectivity/server"
	"arhat.dev/aranya/pkg/node/pod"
	"arhat.dev/aranya/pkg/node/util"
)

var (
	log = logf.Log.WithName("aranya.node")
)

func CreateVirtualNode(ctx context.Context, nodeObj *corev1.Node, kubeletListener, grpcListener net.Listener, config rest.Config) (*Node, error) {
	// create a new kubernetes client with provided config
	client, err := kubeClient.NewForConfig(&config)
	if err != nil {
		return nil, err
	}

	cert, err := ensureNodeCert(client, nodeObj)
	if err != nil {
		return nil, err
	}

	var connectivityManager connectivitySrv.Interface
	if grpcListener != nil {
		connectivityManager = connectivitySrv.NewGrpcManager(nodeObj.Name)
	} else {
		connectivityManager = connectivitySrv.NewMqttManager(nodeObj.Name)
	}

	ctx, exit := context.WithCancel(ctx)
	podManager := pod.NewManager(ctx, nodeObj.Name, client, connectivityManager)
	// statsManager := stats.NewManager()

	logger := log.WithValues("node", nodeObj.Name)

	m := &mux.Router{NotFoundHandler: util.NotFoundHandler()}
	// register http routes
	m.Use(util.LogMiddleware(logger), util.PanicRecoverMiddleware(logger))
	m.StrictSlash(true)
	//
	// routes for pod
	//
	// containerLogs (kubectl logs)
	// m.HandleFunc("/containerLogs/{namespace}/{podID}/{containerName}", podManager.HandlePodContainerLog).Methods(http.MethodGet)
	m.HandleFunc("/containerLogs/{namespace}/{name}/{uid}/{containerName}", podManager.HandlePodContainerLog).Methods(http.MethodGet)
	// logs
	m.Handle("/logs/{logpath:*}", http.StripPrefix("/logs/", http.FileServer(http.Dir("/var/log/")))).Methods(http.MethodGet)
	// exec (kubectl exec)
	// m.HandleFunc("/exec/{namespace}/{name}/{containerName}", podManager.HandlePodExec).Methods(http.MethodPost, http.MethodGet)
	m.HandleFunc("/exec/{namespace}/{name}/{uid}/{containerName}", podManager.HandlePodExec).Methods(http.MethodPost, http.MethodGet)
	// attach (kubectl attach)
	// m.HandleFunc("/attach/{namespace}/{name}/{containerName}", podManager.HandlePodAttach).Methods(http.MethodPost, http.MethodGet)
	m.HandleFunc("/attach/{namespace}/{name}/{uid}/{containerName}", podManager.HandlePodAttach).Methods(http.MethodPost, http.MethodGet)
	// portForward (kubectl proxy)
	// m.HandleFunc("/portForward/{namespace}/{name}", podManager.HandlePodPortForward).Methods(http.MethodPost, http.MethodGet)
	m.HandleFunc("/portForward/{namespace}/{name}/{uid}", podManager.HandlePodPortForward).Methods(http.MethodPost, http.MethodGet)

	//
	// routes for stats
	//
	// stats summary
	// m.HandleFunc("/stats/summary", statsManager.HandleStatsSummary).Methods(http.MethodGet)

	// TODO: handle metrics

	srv := &Node{
		log:        logger,
		ctx:        ctx,
		exit:       exit,
		name:       nodeObj.Name,
		kubeClient: client,

		httpSrv:         &http.Server{Handler: m, TLSConfig: &tls.Config{Certificates: []tls.Certificate{*cert}}},
		kubeletListener: kubeletListener,

		connectivityManager: connectivityManager,
		grpcListener:        grpcListener,

		status:          statusReady,
		podManager:      podManager,
		nodeStatusCache: newNodeCache(nodeObj.Status),
	}

	return srv, nil
}

type Node struct {
	log  logr.Logger
	ctx  context.Context
	exit context.CancelFunc
	name string

	kubeClient kubeClient.Interface

	// kubelet http server and listener
	httpSrv         *http.Server
	kubeletListener net.Listener
	// grpc server and listener
	grpcSrv      *grpc.Server
	grpcListener net.Listener
	// remote device manager
	connectivityManager connectivitySrv.Interface

	podManager *pod.Manager

	nodeStatusCache *NodeCache

	// status
	status uint32
	mu     sync.RWMutex
}

func (n *Node) Start() error {
	if err := func() error {
		n.mu.RLock()
		defer n.mu.RUnlock()

		if n.status == statusRunning || n.status == statusStopped {
			return errors.New("node already started or stopped, do not reuse")
		}
		return nil
	}(); err != nil {
		return err
	}

	// we need to get the lock to change this virtual node's status
	n.mu.Lock()
	defer n.mu.Unlock()

	// add to the pool of running server
	if err := Add(n); err != nil {
		return err
	}

	// added, expected to run
	n.status = statusRunning

	// handle final status change
	go func() {
		<-n.ctx.Done()

		n.mu.Lock()
		defer n.mu.Unlock()
		// force close to ensure node closed
		n.status = statusStopped
	}()

	// start a kubelet http server
	go func() {
		n.log.Info("serve kubelet services")
		if err := n.httpSrv.Serve(n.kubeletListener); err != nil && err != http.ErrServerClosed {
			n.log.Error(err, "failed to serve kubelet services")
			return
		}
	}()

	// start a grpc server if used
	if n.grpcListener != nil {
		n.grpcSrv = grpc.NewServer()

		connectivity.RegisterConnectivityServer(n.grpcSrv, n.connectivityManager.(*connectivitySrv.GrpcManager))
		go func() {
			n.log.Info("serve grpc services")
			if err := n.grpcSrv.Serve(n.grpcListener); err != nil && err != grpc.ErrServerStopped {
				n.log.Error(err, "failed to serve grpc services")
			}
		}()
	} else {
		// TODO: setup mqtt connection to broker
		n.log.Info("mqtt connectivity not implemented")
	}

	go n.InitializeRemoteDevice()

	return n.podManager.Start()
}

// ForceClose close this node immediately
func (n *Node) ForceClose() {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.status == statusRunning {
		n.log.Info("force close virtual node")
		_ = n.httpSrv.Close()
		if n.grpcSrv != nil {
			n.grpcSrv.Stop()
		}

		n.exit()
	}
}

func (n *Node) Shutdown(grace time.Duration) {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.status == statusRunning {
		n.log.Info("shutting down virtual node")

		ctx, _ := context.WithTimeout(n.ctx, grace)

		if n.grpcSrv != nil {
			go n.grpcSrv.GracefulStop()
			go func() {
				time.Sleep(grace)
				n.grpcSrv.Stop()
			}()
		}

		_ = n.httpSrv.Shutdown(ctx)

		n.exit()
	}
}

func (n *Node) closing() bool {
	select {
	case <-n.ctx.Done():
		return true
	default:
		return false
	}
}
