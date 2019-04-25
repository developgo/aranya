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

package client

import (
	"github.com/denisbrodbeck/machineid"

	"arhat.dev/aranya/pkg/connectivity"
)

func (b *baseAgent) doGetNodeInfoAll(sid uint64) {
	nodeMsg := connectivity.NewNodeMsg(sid, b.getSystemInfo(), b.getResourcesCapacity(), b.getResourcesAllocatable(), b.getNodeConditions())
	if err := b.doPostMsg(nodeMsg); err != nil {
		return
	}
}

func (b *baseAgent) doGetNodeSystemInfo(sid uint64) {
	nodeMsg := connectivity.NewNodeMsg(sid, b.getSystemInfo(), nil, nil, nil)
	if err := b.doPostMsg(nodeMsg); err != nil {
		return
	}
}

func (b *baseAgent) doGetNodeResources(sid uint64) {
	nodeMsg := connectivity.NewNodeMsg(sid, nil, b.getResourcesCapacity(), b.getResourcesAllocatable(), nil)
	if err := b.doPostMsg(nodeMsg); err != nil {
		return
	}
}

func (b *baseAgent) doGetNodeConditions(sid uint64) {
	nodeMsg := connectivity.NewNodeMsg(sid, nil, nil, nil, b.getNodeConditions())
	if err := b.doPostMsg(nodeMsg); err != nil {
		return
	}
}

func (b *baseAgent) getSystemInfo() *connectivity.NodeSystemInfo {
	machineID, _ := machineid.ID()
	return setSystemInfo(&connectivity.NodeSystemInfo{
		Os:            b.runtime.OS(),
		Arch:          b.runtime.Arch(),
		KernelVersion: b.runtime.KernelVersion(),
		MachineId:     machineID,
		RuntimeInfo: &connectivity.ContainerRuntimeInfo{
			Name:    b.runtime.Name(),
			Version: b.runtime.Version(),
		},
	})
}

func (b *baseAgent) getResourcesCapacity() *connectivity.NodeResources {
	// TODO: use real resources
	return &connectivity.NodeResources{
		CpuCount:     4,
		MemoryBytes:  512 * (2 << 20),
		StorageBytes: 1 * (2 << 30),
		PodCount:     uint64(b.Pod.MaxPodCount),
	}
}

func (b *baseAgent) getResourcesAllocatable() *connectivity.NodeResources {
	return &connectivity.NodeResources{
		CpuCount:     4,
		MemoryBytes:  512 * (2 << 20),
		StorageBytes: 1 * (2 << 30),
		PodCount:     uint64(b.Pod.MaxPodCount),
	}
}

func (b *baseAgent) getNodeConditions() *connectivity.NodeConditions {
	// TODO: use real conditions
	return &connectivity.NodeConditions{
		Ready:   connectivity.Healthy,
		Memory:  connectivity.Healthy,
		Disk:    connectivity.Healthy,
		Pid:     connectivity.Healthy,
		Network: connectivity.Healthy,
		Pod:     connectivity.Healthy,
	}
}
