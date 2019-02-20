package server

import (
	"errors"
	"net/http"
	"sync"
	"sync/atomic"

	"github.com/go-logr/logr"
	"github.com/gorilla/mux"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	runningServers = make(map[string]*Server)
	mutex          = &sync.RWMutex{}
)

func CreateServer(virtualNodeName, address string, logger logr.Logger, kubeClient client.Client) *Server {
	return &Server{
		name:       virtualNodeName,
		address:    address,
		log:        logger.WithName("node." + virtualNodeName),
		kubeClient: kubeClient,
	}
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

	kubeClient client.Client
	log        logr.Logger
	httpSrv    *http.Server
}

func (s *Server) StartListenAndServe() error {
	if s.isStarted() {
		return errors.New("server already started")
	}
	s.markStarted()

	if err := AddRunningServer(s); err != nil {
		return err
	}

	m := &mux.Router{NotFoundHandler: http.HandlerFunc(NotFoundHandler)}
	{
		m.StrictSlash(true)
		AddRoutesForPod(m)
		AddRoutesForMetrics(m)
	}

	s.httpSrv = &http.Server{
		// listen on the host network
		Addr:    s.address,
		Handler: m,
	}

	go wait.Until(func() {
		defer DeleteRunningServer(s.name)

		if err := s.httpSrv.ListenAndServe(); err != nil {
			s.log.Error(err, "Could not ListenAndServe", "Listen.Address", s.httpSrv.Addr, "Node.Name", s.name)
			return
		}
	}, 0, wait.NeverStop)

	return nil
}

func (s *Server) ForceClose() {
	if s.isStarted() {
		_ = s.httpSrv.Close()

		s.log = nil
		s.kubeClient = nil
		s.httpSrv = nil
	}
}

func (s *Server) isStarted() bool {
	return atomic.LoadUint32(&s.started) == 1
}

func (s *Server) markStarted() {
	atomic.StoreUint32(&s.started, 1)
}
