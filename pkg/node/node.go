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

	"github.com/gorilla/mux"
	"google.golang.org/grpc"
	corev1 "k8s.io/api/core/v1"
	kubeErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	kubeInformers "k8s.io/client-go/informers"
	kubeInformersCoreV1 "k8s.io/client-go/informers/core/v1"
	kubeClient "k8s.io/client-go/kubernetes"
	kubeClientTypedCoreV1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	"arhat.dev/aranya/pkg/node/configmap"
	"arhat.dev/aranya/pkg/node/pod"
	"arhat.dev/aranya/pkg/node/secret"
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
	ctx  context.Context
	exit context.CancelFunc

	name string

	kubeClient *kubeClient.Clientset
	nodeClient kubeClientTypedCoreV1.NodeInterface
	wq         workqueue.RateLimitingInterface

	httpSrv         *http.Server
	kubeletListener net.Listener
	grpcSrv         *grpc.Server
	grpcListener    net.Listener

	podInformerFactory    kubeInformers.SharedInformerFactory
	commonInformerFactory kubeInformers.SharedInformerFactory

	podInformer kubeInformersCoreV1.PodInformer

	podManager       *pod.Manager
	statsManager     *stats.Manager
	secretManager    *secret.Manager
	configMapManager *configmap.Manager

	// status
	status uint32
	mu     sync.RWMutex
}

func CreateVirtualNode(ctx context.Context, nodeObj corev1.Node, kubeletListener, grpcListener net.Listener, config rest.Config) (*Node, error) {
	// create a new kubernetes client with provided config
	client, err := kubeClient.NewForConfig(&config)
	if err != nil {
		return nil, err
	}

	commonInformerFactory := kubeInformers.NewSharedInformerFactoryWithOptions(client, resyncInterval)
	podInformerFactory := kubeInformers.NewSharedInformerFactoryWithOptions(client, resyncInterval,
		kubeInformers.WithNamespace(corev1.NamespaceAll),
		kubeInformers.WithTweakListOptions(func(options *metav1.ListOptions) {
			options.FieldSelector = fields.OneTermEqualSelector("spec.nodeName", nodeObj.Name).String()
		}))

	m := &mux.Router{NotFoundHandler: util.NotFoundHandler()}
	srv := &Node{
		name:                  nodeObj.Name,
		kubeClient:            client,
		nodeClient:            client.CoreV1().Nodes(),
		kubeletListener:       kubeletListener,
		grpcListener:          grpcListener,
		httpSrv:               &http.Server{Handler: m},
		grpcSrv:               grpc.NewServer(),
		podInformerFactory:    podInformerFactory,
		commonInformerFactory: commonInformerFactory,
		podInformer:           podInformerFactory.Core().V1().Pods(),
		wq:                    workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), nodeObj.Name+"-wq"),
		status:                statusReady,
	}

	// create a context for node
	srv.ctx, srv.exit = context.WithCancel(ctx)

	// init resource managers
	srv.podManager = pod.NewManager(srv.podInformer.Lister())
	srv.configMapManager = configmap.NewManager(srv.commonInformerFactory.Core().V1().ConfigMaps().Lister())
	srv.secretManager = secret.NewManager(srv.commonInformerFactory.Core().V1().Secrets().Lister())
	srv.statsManager = stats.NewManager()

	// register http routes
	{
		m.Use(util.LogMiddleware(log), util.PanicRecoverMiddleware(log))
		m.StrictSlash(true)
		// routes for pod
		{
			// containerLogs
			m.HandleFunc("/containerLogs/{namespace}/{pod}/{container}", srv.podManager.HandlePodContainerLog).Methods(http.MethodGet)
		}
		{
			// logs
			m.HandleFunc("/logs/{logpath:*}", srv.podManager.HandleNodeLog).Methods(http.MethodGet)
		}
		{
			// run is basically a exec in new pod
			m.HandleFunc("/run/{namespace}/{podID}/{containerName}", srv.podManager.HandlePodExec).Methods(http.MethodPost, http.MethodGet)
			m.HandleFunc("/run/{namespace}/{podID}/{uid}/{containerName}", srv.podManager.HandlePodExec).Methods(http.MethodPost, http.MethodGet)
		}
		{
			// exec
			m.HandleFunc("/exec/{namespace}/{podID}/{containerName}", srv.podManager.HandlePodExec).Methods(http.MethodPost, http.MethodGet)
			m.HandleFunc("/exec/{namespace}/{podID}/{uid}/{containerName}", srv.podManager.HandlePodExec).Methods(http.MethodPost, http.MethodGet)
		}
		{
			// attach
			m.HandleFunc("/attach/{namespace}/{podID}/{containerName}", srv.podManager.HandlePodAttach).Methods(http.MethodPost, http.MethodGet)
			m.HandleFunc("/attach/{namespace}/{podID}/{uid}/{containerName}", srv.podManager.HandlePodAttach).Methods(http.MethodPost, http.MethodGet)
		}
		{
			// portForward
			m.HandleFunc("/portForward/{namespace}/{podID}/{uid}", srv.podManager.HandlePodPortForward).Methods(http.MethodPost, http.MethodGet)
			m.HandleFunc("/portForward/{namespace}/{podID}", srv.podManager.HandlePodPortForward).Methods(http.MethodPost, http.MethodGet)
		}
		// routes for stats
		m.HandleFunc("/stats/summary", srv.statsManager.HandleStatsSummary).Methods(http.MethodGet)

		// TODO: metrics
	}

	return srv, nil
}

func (s *Node) Start() error {
	err := func() error {
		s.mu.RLock()
		defer s.mu.RUnlock()

		if s.status == statusRunning || s.status == statusStopped {
			return errors.New("node already started or stopped, do not reuse")
		}
		return nil
	}()

	if err != nil {
		return err
	}

	// we need to get the lock to change this virtual node's status
	s.mu.Lock()
	defer s.mu.Unlock()

	// add to the pool of running server
	if err := AddRunningServer(s); err != nil {
		return err
	}

	// added, expected to run
	s.status = statusRunning

	// handle final status change
	go func() {
		<-s.ctx.Done()

		s.mu.Lock()
		defer s.mu.Unlock()
		// force close to ensure node closed
		s.wq.ShutDown()
		s.status = statusStopped
		DeleteRunningServer(s.name)
	}()

	// start a kubelet server
	go func() {
		log.Info("trying to serve kubelet services", "node.name", s.name)
		if err := s.httpSrv.Serve(s.kubeletListener); err != nil && err != http.ErrServerClosed {
			log.Error(err, "could not serve kubelet services", "node.name", s.name)
			return
		}
	}()

	// start a grpc server if used
	if s.grpcListener != nil {
		go func() {
			log.Info("trying to serve grpc services", "node.name", s.name)
			if err := s.grpcSrv.Serve(s.grpcListener); err != nil && err != grpc.ErrServerStopped {
				log.Error(err, "could not serve grpc services", "node.name", s.name)
			}
		}()
	}

	// informer routine
	go s.podInformerFactory.Start(s.ctx.Done())
	go s.commonInformerFactory.Start(s.ctx.Done())

	// handle node change
	s.podInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(pod interface{}) {
			// a new pod need to be created in this virtual node
			if key, err := cache.MetaNamespaceKeyFunc(pod); err != nil {
				log.Error(err, "")
			} else {
				// enqueue the create work for workers
				s.wq.AddRateLimited(key)
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			// pod in this virtual node need to update
			oldPod := oldObj.(*corev1.Pod).DeepCopy()
			newPod := newObj.(*corev1.Pod).DeepCopy()

			newPod.ResourceVersion = oldPod.ResourceVersion
			if reflect.DeepEqual(oldPod.ObjectMeta, newPod.ObjectMeta) && reflect.DeepEqual(oldPod.Spec, newPod.Spec) {
				log.Info("new pod is same with the old one, no action")
				return
			}

			if key, err := cache.MetaNamespaceKeyFunc(newPod); err != nil {
				log.Error(err, "")
			} else {
				// enqueue the update work for workers
				s.wq.AddRateLimited(key)
			}
		},
		DeleteFunc: func(pod interface{}) {
			// pod in this virtual node got deleted
			if key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(pod); err != nil {
				log.Error(err, "")
			} else {
				// enqueue the delete work for workers
				s.wq.AddRateLimited(key)
			}
		},
	})

	go func() {
		if ok := cache.WaitForCacheSync(s.ctx.Done(), s.podInformer.Informer().HasSynced); !ok {
			log.V(2).Info("failed to wait for caches to sync")
			return
		}

		s.SetupDevice()

		log.Info("start node work queue workers")
		go func() {
			for s.work(s.wq) {
			}
		}()
	}()
	return nil
}

// ForceClose close this node immediately
func (s *Node) ForceClose() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.status == statusRunning {
		log.Info("force close virtual node", "node.name", s.name)
		_ = s.httpSrv.Close()
		s.grpcSrv.Stop()

		s.exit()
	}
}

func (s *Node) Shutdown(grace time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.status == statusRunning {
		log.Info("shutting down virtual node", "node.name", s.name)

		ctx, _ := context.WithTimeout(s.ctx, grace)

		go s.grpcSrv.GracefulStop()
		go func() {

			time.Sleep(grace)
			s.grpcSrv.Stop()
		}()

		_ = s.httpSrv.Shutdown(ctx)

		s.exit()
	}
}

func (s *Node) work(wq workqueue.RateLimitingInterface) bool {
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
			log.Info("expected string in work queue but got %#v", obj)
			return nil
		}

		namespace, name, err := cache.SplitMetaNamespaceKey(key)
		if err != nil {
			// Log the error as a warning, but do not requeue the key as it is invalid.
			log.Error(err, "invalid resource key: %q", key)
			return nil
		}

		podFound, err := s.podManager.GetPod(namespace, name)
		if err != nil {
			if !kubeErrors.IsNotFound(err) {
				log.Error(err, "failed to fetch pod with key from lister", "key", key)
				return err
			}

			// pod has been deleted
			if err := s.DeletePodInDevice(namespace, name); err != nil {
				log.Error(err, "failed to delete pod in the provider", "pod.namespace", namespace, "pod.name", name)
				return err
			}
		}

		if err := s.SyncPodInDevice(podFound); err != nil {
			if wq.NumRequeues(key) < maxRetries {
				// Put the item back on the work queue to handle any transient errors.
				log.Error(err, "requeue due to failed sync", "key", key)
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
		log.Error(err, "")
		return true
	}

	return true
}

func (s *Node) SyncPodInDevice(pod *corev1.Pod) error {
	if pod.DeletionTimestamp != nil {
		if err := s.DeletePodInDevice(pod.Namespace, pod.Name); err != nil {
			log.Error(err, "failed to delete pod in edge device", "pod.name", pod.Name, "pod.namespace", pod.Namespace)
			return err
		}
		return nil
	}

	if pod.Status.Phase == corev1.PodFailed || pod.Status.Phase == corev1.PodSucceeded {
		log.Info("skipping sync of pod", "Pod.Phase", pod.Status.Phase, "pod.namespace", pod.Namespace, "pod.name", pod.Name)
		return nil
	}

	if err := s.CreateOrUpdatePodInDevice(pod); err != nil {
		log.Error(err, "failed to sync edge pod", "pod.namespace", pod.Namespace, "pod.name", pod.Name)
		return err
	}

	return nil
}
