package node

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/mux"
	corev1 "k8s.io/api/core/v1"
	kubeErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/wait"
	kubeInformers "k8s.io/client-go/informers"
	kubeInformersCoreV1 "k8s.io/client-go/informers/core/v1"
	kubeClient "k8s.io/client-go/kubernetes"
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
	log            = logf.Log.WithName("aranya.node")
	runningServers = make(map[string]*Server)
	mutex          = &sync.RWMutex{}
)

type Server struct {
	ctx  context.Context
	exit context.CancelFunc

	name    string
	address string

	kubeClient *kubeClient.Clientset
	httpSrv    *http.Server
	wq         workqueue.RateLimitingInterface

	podInformerFactory    kubeInformers.SharedInformerFactory
	commonInformerFactory kubeInformers.SharedInformerFactory

	podInformer kubeInformersCoreV1.PodInformer

	podManager       *pod.Manager
	statsManager     *stats.Manager
	secretManager    *secret.Manager
	configMapManager *configmap.Manager

	// status
	status uint32
}

func CreateVirtualNode(ctx context.Context, virtualNodeName, address string) (*Server, error) {
	// create a new kubernetes client using service account
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}

	client, err := kubeClient.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	commonInformerFactory := kubeInformers.NewSharedInformerFactoryWithOptions(client, resyncInterval)
	podInformerFactory := kubeInformers.NewSharedInformerFactoryWithOptions(client, resyncInterval,
		kubeInformers.WithNamespace(corev1.NamespaceAll),
		kubeInformers.WithTweakListOptions(func(options *metav1.ListOptions) {
			options.FieldSelector = fields.OneTermEqualSelector("spec.nodeName", virtualNodeName).String()
		}))

	m := &mux.Router{NotFoundHandler: util.NotFoundHandler()}
	srv := &Server{
		name:                  virtualNodeName,
		address:               address,
		kubeClient:            client,
		httpSrv:               &http.Server{Addr: address, Handler: m},
		podInformerFactory:    podInformerFactory,
		commonInformerFactory: commonInformerFactory,
		podInformer:           podInformerFactory.Core().V1().Pods(),
		wq:                    workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), virtualNodeName+"wq"),
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

func GetRunningServer(name string) *Server {
	mutex.RLock()
	defer mutex.RUnlock()

	return runningServers[name]
}

func AddRunningServer(server *Server) error {
	mutex.Lock()
	defer mutex.Unlock()

	if _, ok := runningServers[server.name]; ok {
		return errors.New("node already running")
	}
	runningServers[server.name] = server
	return nil
}

func DeleteRunningServer(name string) {
	mutex.Lock()
	defer mutex.Unlock()

	if srv, ok := runningServers[name]; ok {
		srv.ForceClose()
		delete(runningServers, name)
	}
}

func (s *Server) StartListenAndServe() error {
	if s.isRunning() || s.isStopped() {
		return errors.New("node already started")
	}

	s.markRunning()
	if err := AddRunningServer(s); err != nil {
		return err
	}

	// serve kubelet node
	go wait.Until(func() {
		defer DeleteRunningServer(s.name)

		log.Info("Start ListenAndServe", "Node.Name", s.name, "Listen.Address", s.httpSrv.Addr)
		if err := s.httpSrv.ListenAndServe(); err != nil {
			log.Error(err, "Could not ListenAndServe", "Node.Name", s.name, "Listen.Address", s.httpSrv.Addr)
			return
		}
	}, 0, s.ctx.Done())

	go s.podInformerFactory.Start(s.ctx.Done())
	go s.commonInformerFactory.Start(s.ctx.Done())

	s.podInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(pod interface{}) {
			if key, err := cache.MetaNamespaceKeyFunc(pod); err != nil {
				log.Error(err, "")
			} else {
				s.wq.AddRateLimited(key)
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
				s.wq.AddRateLimited(key)
			}
		},
		DeleteFunc: func(pod interface{}) {
			if key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(pod); err != nil {
				log.Error(err, "")
			} else {
				s.wq.AddRateLimited(key)
			}
		},
	})

	go wait.Until(func() {
		defer func() {
			s.wq.ShutDown()
			s.exit()
		}()

		if ok := cache.WaitForCacheSync(s.ctx.Done(), s.podInformer.Informer().HasSynced); !ok {
			log.V(2).Info("failed to wait for caches to sync")
			return
		}

		// Perform a reconciliation step that deletes any dangling pods from the provider.
		// This happens only when the virtual-kubelet is starting, and operates on a "best-effort" basis.
		// If by any reason the provider fails to delete a dangling pod, it will stay in the provider and deletion won't be retried.
		s.deleteDanglingPods()

		log.Info("Start workers")

		// start a worker
		// go wait.Until(func() {
		//
		// }, time.Second, s.ctx.Done())

		for s.work(s.wq) {
		}
	}, 0, s.ctx.Done())
	return nil
}

func (s *Server) ForceClose() {
	defer s.markStopped()
	if s.isRunning() {
		log.V(3).Info("ForceClose node", "Node.Name", s.name)
		_ = s.httpSrv.Close()
	}
}

func (s *Server) Shutdown(grace time.Duration) {
	defer s.markStopped()
	if s.isRunning() {
		log.V(3).Info("Shutdown node", "Node.Name", s.name)
		ctx, _ := context.WithTimeout(s.ctx, grace)
		_ = s.httpSrv.Shutdown(ctx)
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

		podFound, err := s.podManager.GetPod(namespace, name)
		if err != nil {
			if !kubeErrors.IsNotFound(err) {
				log.Error(err, "failed to fetch pod with key from lister", "Key", key)
				return err
			}

			if err := s.DeleteEdgePod(namespace, name); err != nil {
				log.Error(err, "failed to delete pod in the provider", "Pod.Namespace", namespace, "Pod.Name", name)
				return err
			}
		}

		// Run the syncHandler, passing it the namespace/name string of the Pod resource to be synced.
		if err := s.SyncEdgePod(podFound); err != nil {
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

func (s *Server) isRunning() bool {
	return atomic.LoadUint32(&s.status) == 1
}

func (s *Server) markRunning() {
	atomic.StoreUint32(&s.status, 1)
}

func (s *Server) markStopped() {
	atomic.StoreUint32(&s.status, 2)
}

func (s *Server) isStopped() bool {
	return atomic.LoadUint32(&s.status) == 2
}

func (s *Server) CreateOrUpdateEdgePod(pod *corev1.Pod) error {
	return nil
}

func (s *Server) DeleteEdgePod(namespace, name string) error {
	return nil
}

func (s *Server) SyncEdgePod(pod *corev1.Pod) error {
	if pod.DeletionTimestamp != nil {
		if err := s.DeleteEdgePod(pod.Namespace, pod.Name); err != nil {
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
