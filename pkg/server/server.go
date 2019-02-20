package server

import (
	"errors"
	"fmt"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"net/http"
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/mux"
	corev1 "k8s.io/api/core/v1"
	kubeErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	kubeInformers "k8s.io/client-go/informers"
	kubeInformersCoreV1 "k8s.io/client-go/informers/core/v1"
	kubeClient "k8s.io/client-go/kubernetes"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

const (
	resyncInterval = time.Minute
	maxRetries     = 20
)

var (
	log            = logf.Log.WithName("aranya.node")
	runningServers = make(map[string]*Server)
	mutex          = &sync.RWMutex{}
)

func CreateServer(virtualNodeName, address string) (*Server, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}

	client, err := kubeClient.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	m := &mux.Router{NotFoundHandler: http.HandlerFunc(NotFoundHandler)}
	m.Use(LogMiddleware(log), PanicRecoverMiddleware(log))
	m.StrictSlash(true)
	// register http path
	{
		// routes for pod
		m.HandleFunc("/containerLogs/{namespace}/{pod}/{container}", HandleFuncPodLog).Methods(http.MethodGet)
		m.HandleFunc("/exec/{namespace}/{pod}/{container}", HandleFuncPodExec).Methods(http.MethodPost)

		// routes for metrics
		m.HandleFunc("/stats/summary", HandleFuncMetricsSummary).Methods(http.MethodGet)
	}

	srv := &Server{
		name:    virtualNodeName,
		address: address,

		kubeClient: client,
		httpSrv: &http.Server{
			// listen on the host network
			Addr:    address,
			Handler: m,
		},
		podInformerFactory: kubeInformers.NewSharedInformerFactoryWithOptions(client, resyncInterval,
			kubeInformers.WithNamespace(corev1.NamespaceAll),
			kubeInformers.WithTweakListOptions(func(options *metav1.ListOptions) {
				options.FieldSelector = fields.OneTermEqualSelector("spec.nodeName", virtualNodeName).String()
			})),
		scmInformerFactory: kubeInformers.NewSharedInformerFactoryWithOptions(client, resyncInterval),
	}


	srv.podInformer = srv.podInformerFactory.Core().V1().Pods()
	srv.res = &Resource{
		PodLister:       srv.podInformer.Lister(),
		SecretLister:    srv.scmInformerFactory.Core().V1().Secrets().Lister(),
		ConfigMapLister: srv.scmInformerFactory.Core().V1().ConfigMaps().Lister(),
	}

	return srv, nil
}

func GetRunningServer(name string) *Server {
	mutex.RLock()
	defer mutex.RUnlock()

	return runningServers[name]
}

func AddRunningServer(server *Server) error {
	mutex.Lock()
	defer mutex.Unlock()

	if _, ok := runningServers[server.name]; ok {
		return errors.New("server already running")
	}
	runningServers[server.name] = server
	return nil
}

func DeleteRunningServer(name string) {
	mutex.Lock()
	defer mutex.Unlock()

	srv := runningServers[name]
	srv.ForceClose()
	delete(runningServers, name)
}

type Server struct {
	name    string
	address string

	started uint32

	kubeClient *kubeClient.Clientset
	httpSrv    *http.Server

	podInformerFactory kubeInformers.SharedInformerFactory
	scmInformerFactory kubeInformers.SharedInformerFactory
	podInformer        kubeInformersCoreV1.PodInformer
	res                *Resource
}

func (s *Server) StartListenAndServe() error {
	if s.isStarted() {
		return errors.New("server already started")
	}

	s.markStarted()
	if err := AddRunningServer(s); err != nil {
		return err
	}

	go wait.Until(func() {
		defer DeleteRunningServer(s.name)

		log.Info("Start ListenAndServe", "Node.Name", s.name, "Listen.Address", s.httpSrv.Addr)
		if err := s.httpSrv.ListenAndServe(); err != nil {
			log.Error(err, "Could not ListenAndServe", "Node.Name", s.name, "Listen.Address", s.httpSrv.Addr)
			return
		}
	}, 0, wait.NeverStop)

	podInformer := s.podInformerFactory.Core().V1().Pods()

	go s.podInformerFactory.Start(wait.NeverStop)
	go s.scmInformerFactory.Start(wait.NeverStop)

	wq := workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "pods")
	podInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(pod interface{}) {
			if key, err := cache.MetaNamespaceKeyFunc(pod); err != nil {
				log.Error(err, "")
			} else {
				wq.AddRateLimited(key)
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldPod := oldObj.(*corev1.Pod).DeepCopy()
			newPod := newObj.(*corev1.Pod).DeepCopy()

			newPod.ResourceVersion = oldPod.ResourceVersion
			if reflect.DeepEqual(oldPod.ObjectMeta, newPod.ObjectMeta) && reflect.DeepEqual(oldPod.Spec, newPod.Spec) {
				return
			}

			if key, err := cache.MetaNamespaceKeyFunc(newPod); err != nil {
				log.Error(err, "")
			} else {
				wq.AddRateLimited(key)
			}
		},
		DeleteFunc: func(pod interface{}) {
			if key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(pod); err != nil {
				log.Error(err, "")
			} else {
				wq.AddRateLimited(key)
			}
		},
	})

	go wait.Until(func() {
		defer wq.ShutDown()

		if ok := cache.WaitForCacheSync(wait.NeverStop, podInformer.Informer().HasSynced); !ok {
			log.V(2).Info("failed to wait for caches to sync")
			return
		}

		// Perform a reconciliation step that deletes any dangling pods from the provider.
		// This happens only when the virtual-kubelet is starting, and operates on a "best-effort" basis.
		// If by any reason the provider fails to delete a dangling pod, it will stay in the provider and deletion won't be retried.
		s.deleteDanglingPods()

		log.Info("Start workers")

		go wait.Until(func() {
			for s.work(wq) {
			}
		}, time.Second, wait.NeverStop)

	}, 0, wait.NeverStop)
	return nil
}

func (s *Server) ForceClose() {
	if s.isStarted() {
		_ = s.httpSrv.Close()
	}
}

func (s *Server) deleteDanglingPods() {

}

func (s *Server) work(wq workqueue.RateLimitingInterface) bool {
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

		pod, err := s.res.GetPod(types.NamespacedName{Namespace: namespace, Name: name})
		if err != nil {
			if !kubeErrors.IsNotFound(err) {
				log.Error(err, "failed to fetch pod with key from lister", "Key", key)
				return err
			}

			if err := s.DeleteEdgePod(types.NamespacedName{Name: name, Namespace: namespace}); err != nil {
				log.Error(err, "failed to delete pod in the provider", "Pod.Namespace", namespace, "Pod.Name", name)
				return err
			}
		}

		// Run the syncHandler, passing it the namespace/name string of the Pod resource to be synced.
		if err := s.SyncEdgePod(pod); err != nil {
			if wq.NumRequeues(key) < maxRetries {
				// Put the item back on the work queue to handle any transient errors.
				log.Error(err, "requeue due to failed sync", "Key", key)
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

func (s *Server) isStarted() bool {
	return atomic.LoadUint32(&s.started) == 1
}

func (s *Server) markStarted() {
	atomic.StoreUint32(&s.started, 1)
}

func (s *Server) CreateOrUpdateEdgePod(pod *corev1.Pod) error {
	return nil
}

func (s *Server) DeleteEdgePod(name types.NamespacedName) error {
	return nil
}

func (s *Server) SyncEdgePod(pod *corev1.Pod) error {
	if pod.DeletionTimestamp != nil {
		if err := s.DeleteEdgePod(types.NamespacedName{Namespace: pod.Namespace, Name: pod.Name}); err != nil {
			log.Error(err, "failed to delete pod in edge device", "Pod.Name", pod.Name, "Pod.Namespace", pod.Namespace)
			return err
		}
		return nil
	}

	if pod.Status.Phase == corev1.PodFailed || pod.Status.Phase == corev1.PodSucceeded {
		log.Info("skipping sync of pod", "Pod.Phase", pod.Status.Phase, "Pod.Namespace", pod.Namespace, "Pod.Name", pod.Name)
		return nil
	}

	if err := s.CreateOrUpdateEdgePod(pod); err != nil {
		log.Error(err, "failed to sync edge pod", "Pod.Namespace", pod.Namespace, "Pod.Name", pod.Name)
		return err
	}

	return nil
}
