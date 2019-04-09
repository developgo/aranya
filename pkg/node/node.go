package node

import (
	"context"
	"errors"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/gorilla/mux"
	"google.golang.org/grpc"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/wait"
	kubeInformers "k8s.io/client-go/informers"
	kubeInformersCoreV1 "k8s.io/client-go/informers/core/v1"
	kubeClient "k8s.io/client-go/kubernetes"
	kubeClientTypedCoreV1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	kubeletConfigMap "k8s.io/kubernetes/pkg/kubelet/configmap"
	kubeletPod "k8s.io/kubernetes/pkg/kubelet/pod"
	kubeletSecret "k8s.io/kubernetes/pkg/kubelet/secret"
	kubeletStatus "k8s.io/kubernetes/pkg/kubelet/status"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	"arhat.dev/aranya/pkg/node/connectivity"
	connectivitySrv "arhat.dev/aranya/pkg/node/connectivity/server"
	"arhat.dev/aranya/pkg/node/pod"
	"arhat.dev/aranya/pkg/node/stats"
	"arhat.dev/aranya/pkg/node/util"
)

const (
	resyncInterval = time.Minute
)

var (
	log = logf.Log.WithName("aranya.node")
)

type Node struct {
	log  logr.Logger
	ctx  context.Context
	exit context.CancelFunc
	name string

	kubeClient kubeClient.Interface
	nodeClient kubeClientTypedCoreV1.NodeInterface
	wq         workqueue.RateLimitingInterface

	httpSrv             *http.Server
	kubeletListener     net.Listener
	grpcSrv             *grpc.Server
	grpcListener        net.Listener
	connectivityManager connectivitySrv.Interface

	podInformerFactory kubeInformers.SharedInformerFactory
	podInformer        kubeInformersCoreV1.PodInformer

	podManager       *pod.Manager
	podStatusManager kubeletStatus.Manager

	podCache *PodCache

	// status
	status uint32
	mu     sync.RWMutex
}

func CreateVirtualNode(ctx context.Context, nodeObj *corev1.Node, kubeletListener, grpcListener net.Listener, config rest.Config) (*Node, error) {
	// create a new kubernetes client with provided config
	client, err := kubeClient.NewForConfig(&config)
	if err != nil {
		return nil, err
	}

	podInformerFactory := kubeInformers.NewSharedInformerFactoryWithOptions(client, resyncInterval,
		kubeInformers.WithNamespace(corev1.NamespaceAll),
		kubeInformers.WithTweakListOptions(func(options *metav1.ListOptions) {
			options.FieldSelector = fields.OneTermEqualSelector("spec.nodeName", nodeObj.Name).String()
		}))
	podInformer := podInformerFactory.Core().V1().Pods()

	basicPodManager := kubeletPod.NewBasicPodManager(
		kubeletPod.NewBasicMirrorClient(client),
		kubeletSecret.NewWatchingSecretManager(client),
		kubeletConfigMap.NewWatchingConfigMapManager(client),
		nil)

	var connectivityManager connectivitySrv.Interface
	if grpcListener != nil {
		connectivityManager = connectivitySrv.NewGrpcManager(nodeObj.Name)
	} else {
		connectivityManager = connectivitySrv.NewMqttManager(nodeObj.Name)
	}

	ctx, exit := context.WithCancel(ctx)
	podManager := pod.NewManager(ctx, podInformer.Lister(), basicPodManager, connectivityManager)
	statsManager := stats.NewManager()

	logger := log.WithValues("name", nodeObj.Name)

	m := &mux.Router{NotFoundHandler: util.NotFoundHandler()}
	// register http routes
	m.Use(util.LogMiddleware(logger), util.PanicRecoverMiddleware(logger))
	m.StrictSlash(true)
	//
	// routes for pod
	//
	// containerLogs (kubectl logs)
	m.HandleFunc("/containerLogs/{namespace}/{podID}/{containerName}", podManager.HandlePodContainerLog).Methods(http.MethodGet)
	// logs
	m.Handle("/logs/{logpath:*}", http.StripPrefix("/logs/", http.FileServer(http.Dir("/var/log/")))).Methods(http.MethodGet)
	// run (kubectl run) is basically a exec in new pod
	m.HandleFunc("/run/{namespace}/{podID}/{containerName}", podManager.HandlePodExec).Methods(http.MethodPost, http.MethodGet)
	m.HandleFunc("/run/{namespace}/{podID}/{uid}/{containerName}", podManager.HandlePodExec).Methods(http.MethodPost, http.MethodGet)
	// exec (kubectl exec)
	m.HandleFunc("/exec/{namespace}/{podID}/{containerName}", podManager.HandlePodExec).Methods(http.MethodPost, http.MethodGet)
	m.HandleFunc("/exec/{namespace}/{podID}/{uid}/{containerName}", podManager.HandlePodExec).Methods(http.MethodPost, http.MethodGet)
	// attach (kubectl attach)
	m.HandleFunc("/attach/{namespace}/{podID}/{containerName}", podManager.HandlePodAttach).Methods(http.MethodPost, http.MethodGet)
	m.HandleFunc("/attach/{namespace}/{podID}/{uid}/{containerName}", podManager.HandlePodAttach).Methods(http.MethodPost, http.MethodGet)
	// portForward (kubectl proxy)
	m.HandleFunc("/portForward/{namespace}/{podID}", podManager.HandlePodPortForward).Methods(http.MethodPost, http.MethodGet)
	m.HandleFunc("/portForward/{namespace}/{podID}/{uid}", podManager.HandlePodPortForward).Methods(http.MethodPost, http.MethodGet)

	//
	// routes for stats
	//
	// stats summary
	m.HandleFunc("/stats/summary", statsManager.HandleStatsSummary).Methods(http.MethodGet)

	// TODO: handle metrics

	srv := &Node{
		log:                 logger,
		ctx:                 ctx,
		exit:                exit,
		name:                nodeObj.Name,
		kubeClient:          client,
		nodeClient:          client.CoreV1().Nodes(),
		connectivityManager: connectivityManager,
		kubeletListener:     kubeletListener,
		grpcListener:        grpcListener,
		httpSrv:             &http.Server{Handler: m},
		podInformerFactory:  podInformerFactory,
		podInformer:         podInformer,
		wq:                  workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), nodeObj.Name+"-wq"),
		status:              statusReady,
		podStatusManager:    kubeletStatus.NewManager(client, podManager, podManager),
		podManager:          podManager,
		podCache:            newPodCache(),
	}

	return srv, nil
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
		n.wq.ShutDown()
		n.status = statusStopped
	}()

	// node status update routine
	go wait.Until(n.syncNodeStatus, time.Second, n.ctx.Done())
	go n.podStatusManager.Start()

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

	// informer routine
	go n.podInformerFactory.Start(n.ctx.Done())

	// handle node change
	n.podInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(newObj interface{}) {
			n.createPodInDevice(newObj.(*corev1.Pod))
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldPod := oldObj.(*corev1.Pod)
			n.deletePodInDevice(oldPod.Namespace, oldPod.Name)
			n.createPodInDevice(newObj.(*corev1.Pod))
		},
		DeleteFunc: func(oldObj interface{}) {
			oldPod := oldObj.(*corev1.Pod)
			n.deletePodInDevice(oldPod.Namespace, oldPod.Name)
		},
	})

	go n.InitializeRemoteDevice()

	return nil
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
