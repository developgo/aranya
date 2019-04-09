package node

import (
	"errors"
	"sync"
)

var (
	runningServers = make(map[string]*Node)
	mutex          = &sync.RWMutex{}
)

func Add(node *Node) error {
	mutex.Lock()
	defer mutex.Unlock()

	if _, ok := runningServers[node.name]; ok {
		return errors.New("node with same name already running")
	}

	runningServers[node.name] = node
	return nil
}

func Delete(name string) {
	mutex.Lock()
	defer mutex.Unlock()

	if srv, ok := runningServers[name]; ok {
		srv.ForceClose()
		delete(runningServers, name)
	}
}
