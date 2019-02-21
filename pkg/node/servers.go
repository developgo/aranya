package node

import (
	"errors"
	"sync"
)

var (
	runningServers = make(map[string]*Server)
	mutex          = &sync.RWMutex{}
)

func addRunningServer(server *Server) error {
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
