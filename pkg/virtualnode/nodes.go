/*
Copyright 2019 The arhat.dev Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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
