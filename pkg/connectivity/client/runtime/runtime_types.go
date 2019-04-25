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

package runtime

import (
	"io"

	"arhat.dev/aranya/pkg/connectivity"
)

type Interface interface {
	// Name the runtime name
	Name() string
	// Version the runtime version
	Version() string
	// OS the kernel name of the container runtime
	OS() string
	// Arch the cpu arch of the container runtime
	Arch() string
	// KernelVersion of the container runtime
	KernelVersion() string
	// CreatePod create a pod according to corev1.PodSpec
	// steps:
	//
	// 		- pull images
	// 		- create pod using `pause` container
	// 		- TODO: create and start init containers
	// 		- create and start containers
	CreatePod(options *connectivity.CreateOptions) (*connectivity.PodStatus, *connectivity.Error)
	DeletePod(options *connectivity.DeleteOptions) (*connectivity.PodStatus, *connectivity.Error)
	ListPods(options *connectivity.ListOptions) ([]*connectivity.PodStatus, *connectivity.Error)
	ExecInContainer(podUID, container string, stdin io.Reader, stdout, stderr io.WriteCloser, resizeCh <-chan *connectivity.TtyResizeOptions, command []string, tty bool) *connectivity.Error
	AttachContainer(podUID, container string, stdin io.Reader, stdout, stderr io.WriteCloser, resizeCh <-chan *connectivity.TtyResizeOptions) *connectivity.Error
	GetContainerLogs(podUID string, options *connectivity.LogOptions, stdout, stderr io.WriteCloser) *connectivity.Error
	PortForward(podUID string, protocol string, port int32, in io.Reader, out io.WriteCloser) *connectivity.Error
}
