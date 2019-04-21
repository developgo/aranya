package virtualnode

import (
	"errors"
	"sync"
)

var (
	runningServers = make(map[string]*VirtualNode)
	mutex          = &sync.RWMutex{}
)

func Add(node *VirtualNode) error {
	mutex.Lock()
	defer mutex.Unlock()

	if _, ok := runningServers[node.name]; ok {
		return errors.New("node with same name already running")
	}

	runningServers[node.name] = node
	return nil
}

func Get(name string) (*VirtualNode, bool) {
	mutex.RLock()
	defer mutex.RUnlock()

	node, ok := runningServers[name]
	if ok {
		return node, true
	}
	return nil, false
}

func Delete(name string) {
	mutex.Lock()
	defer mutex.Unlock()

	if srv, ok := runningServers[name]; ok {
		srv.ForceClose()
		delete(runningServers, name)
	}
}