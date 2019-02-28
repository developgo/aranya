package node

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"reflect"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/gorilla/mux"
	"google.golang.org/grpc"
	corev1 "k8s.io/api/core/v1"
	kubeErrors "k8s.io/apimachinery/pkg/api/errors"
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
	kubeletconfigmap "k8s.io/kubernetes/pkg/kubelet/configmap"
	kubeletpod "k8s.io/kubernetes/pkg/kubelet/pod"
	kubeletsecret "k8s.io/kubernetes/pkg/kubelet/secret"
	kubeletstatus "k8s.io/kubernetes/pkg/kubelet/status"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	"arhat.dev/aranya/pkg/node/connectivity"
	"arhat.dev/aranya/pkg/node/pod"
	"arhat.dev/aranya/pkg/node/stats"
	"arhat.dev/aranya/pkg/node/util"
)

const (
	resyncInterval = time.Minute
	maxRetries     = 20
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

	httpSrv         *http.Server
	kubeletListener net.Listener
	grpcSrv         *grpc.Server
	grpcListener    net.Listener
	remoteManager   connectivity.Interface

	podInformerFactory kubeInformers.SharedInformerFactory
	podInformer        kubeInformersCoreV1.PodInformer

	podManager    *pod.Manager
	statusManager kubeletstatus.Manager

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

	basicPodManager := kubeletpod.NewBasicPodManager(
		kubeletpod.NewBasicMirrorClient(client),
		kubeletsecret.NewWatchingSecretManager(client),
		kubeletconfigmap.NewWatchingConfigMapManager(client),
		nil)

	var remoteManager connectivity.Interface
	if grpcListener != nil {
		remoteManager = connectivity.NewGRPCService(nodeObj.Name)
	} else {
		remoteManager = connectivity.NewMQTTService(nodeObj.Name)
	}

	podManager := pod.NewManager(podInformer.Lister(), basicPodManager, remoteManager)
	statsManager := stats.NewManager()

	ctx, exit := context.WithCancel(ctx)

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

	// TODO: metrics

	srv := &Node{
		log:                logger,
		ctx:                ctx,
		exit:               exit,
		name:               nodeObj.Name,
		kubeClient:         client,
		nodeClient:         client.CoreV1().Nodes(),
		remoteManager:      remoteManager,
		kubeletListener:    kubeletListener,
		grpcListener:       grpcListener,
		httpSrv:            &http.Server{Handler: m},
		podInformerFactory: podInformerFactory,
		podInformer:        podInformer,
		wq:                 workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), nodeObj.Name+"-wq"),
		status:             statusReady,
		statusManager:      kubeletstatus.NewManager(client, podManager, podManager),
		podManager:         podManager,
	}

	return srv, nil
}

func (n *Node) Start() error {
	err := func() error {
		n.mu.RLock()
		defer n.mu.RUnlock()

		if n.status == statusRunning || n.status == statusStopped {
			return errors.New("node already started or stopped, do not reuse")
		}
		return nil
	}()

	if err != nil {
		return err
	}

	// we need to get the lock to change this virtual node's status
	n.mu.Lock()
	defer n.mu.Unlock()

	// add to the pool of running server
	if err := AddRunningServer(n); err != nil {
		return err
	}

	// added, expected to run
	n.status = statusRunning

	// node status update routine
	go wait.Until(n.syncNodeStatus, time.Second, n.ctx.Done())
	go n.statusManager.Start()

	// handle final status change
	go func() {
		<-n.ctx.Done()

		n.mu.Lock()
		defer n.mu.Unlock()
		// force close to ensure node closed
		n.wq.ShutDown()
		n.status = statusStopped
	}()

	// start a kubelet server
	go func() {
		n.log.Info("serve kubelet services")
		if err := n.httpSrv.Serve(n.kubeletListener); err != nil && err != http.ErrServerClosed {
			n.log.Error(err, "serve kubelet services failed")
			return
		}
	}()

	// start a grpc server if used
	if n.grpcListener != nil {
		n.grpcSrv = grpc.NewServer()

		connectivity.RegisterConnectivityServer(n.grpcSrv, n.remoteManager.(*connectivity.GRPCService))
		go func() {
			n.log.Info("serve grpc services")
			if err := n.grpcSrv.Serve(n.grpcListener); err != nil && err != grpc.ErrServerStopped {
				n.log.Error(err, "serve grpc services failed")
			}
		}()
	} else {
		// TODO: setup mqtt connection to broker
	}

	// informer routine
	go n.podInformerFactory.Start(n.ctx.Done())

	// handle node change
	n.podInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(pod interface{}) {
			// a new pod need to be created in this virtual node
			if key, err := cache.MetaNamespaceKeyFunc(pod); err != nil {
				n.log.Error(err, "")
			} else {
				// enqueue the create work for workers
				n.wq.AddRateLimited(key)
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			// pod in this virtual node need to update
			oldPod := oldObj.(*corev1.Pod).DeepCopy()
			newPod := newObj.(*corev1.Pod).DeepCopy()

			newPod.ResourceVersion = oldPod.ResourceVersion
			if reflect.DeepEqual(oldPod.ObjectMeta, newPod.ObjectMeta) && reflect.DeepEqual(oldPod.Spec, newPod.Spec) {
				n.log.Info("new pod is same with the old one, no action")
				return
			}

			if key, err := cache.MetaNamespaceKeyFunc(newPod); err != nil {
				n.log.Error(err, "")
			} else {
				// enqueue the update work for workers
				n.wq.AddRateLimited(key)
			}
		},
		DeleteFunc: func(pod interface{}) {
			// pod in this virtual node got deleted
			if key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(pod); err != nil {
				n.log.Error(err, "")
			} else {
				// enqueue the delete work for workers
				n.wq.AddRateLimited(key)
			}
		},
	})

	go func() {
		if ok := cache.WaitForCacheSync(n.ctx.Done(), n.podInformer.Informer().HasSynced); !ok {
			n.log.V(2).Info("failed to wait for caches to sync")
			return
		}

		go n.InitializeRemoteDevice()

		n.log.Info("start node work queue workers")
		go func() {
			for n.work(n.wq) {
			}
		}()
	}()
	return nil
}

// ForceClose close this node immediately
func (n *Node) ForceClose() {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.status == statusRunning {
		n.log.Info("force close virtual node")
		_ = n.httpSrv.Close()
		n.grpcSrv.Stop()

		n.exit()
	}
}

func (n *Node) Shutdown(grace time.Duration) {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.status == statusRunning {
		n.log.Info("shutting down virtual node")

		ctx, _ := context.WithTimeout(n.ctx, grace)

		go n.grpcSrv.GracefulStop()
		go func() {
			time.Sleep(grace)
			n.grpcSrv.Stop()
		}()

		_ = n.httpSrv.Shutdown(ctx)

		n.exit()
	}
}

func (n *Node) work(wq workqueue.RateLimitingInterface) bool {
	obj, shutdown := wq.Get()
	if shutdown {
		return false
	}

	err := func(obj interface{}) error {
		defer wq.Done(obj)

		var (
			key string
			ok  bool
		)

		if key, ok = obj.(string); !ok {
			// As the item in the work queue is actually invalid, we call Forget here else we'd go into a loop of attempting to process a work item that is invalid.
			wq.Forget(obj)
			n.log.Info("expected string in work queue but got %#v", obj)
			return nil
		}

		namespace, name, err := cache.SplitMetaNamespaceKey(key)
		if err != nil {
			// Log the error as a warning, but do not requeue the key as it is invalid.
			n.log.Error(err, "invalid resource key: %q", key)
			return nil
		}

		podFound, err := n.podManager.GetMirrorPod(namespace, name)
		if err != nil {
			if !kubeErrors.IsNotFound(err) {
				n.log.Error(err, "failed to fetch pod with key from lister", "key", key)
				return err
			}

			// pod has been deleted
			if err := n.podManager.DeletePodInDevice(namespace, name); err != nil {
				n.log.Error(err, "failed to delete pod in the provider", "pod.ns", namespace, "pod.name", name)
				return err
			}
		}

		if err := n.podManager.SyncPodInDevice(podFound); err != nil {
			if wq.NumRequeues(key) < maxRetries {
				// Put the item back on the work queue to handle any transient errors.
				n.log.Error(err, "requeue due to failed sync", "key", key)
				wq.AddRateLimited(key)
				return nil
			}
			// We've exceeded the maximum retries, so we must forget the key.
			wq.Forget(key)
			return fmt.Errorf("forgetting %q due to maximum retries reached", key)
		}
		// Finally, if no error occurs we Forget this item so it does not get queued again until another change happens.
		wq.Forget(obj)
		return nil
	}(obj)

	if err != nil {
		n.log.Error(err, "")
		return true
	}

	return true
}

func (n *Node) closing() bool {
	select {
	case <-n.ctx.Done():
		return true
	default:
		return false
	}
}
