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

package podman

import (
	"io"

	libpodRuntime "github.com/containers/libpod/libpod"
)

func findContainer(rt *libpodRuntime.Runtime, podName, containerName string) (*libpodRuntime.Container, error) {
	// lookup pod and find target container
	pod, err := rt.LookupPod(podName)
	if err != nil {
		return nil, err
	}

	allContainers, err := pod.AllContainers()
	if err != nil {
		return nil, err
	}

	var target *libpodRuntime.Container
	for _, ctr := range allContainers {
		if ctr.Name() == containerName {
			target = ctr
			break
		}
	}

	if target == nil {
		return nil, libpodRuntime.ErrNoSuchCtr
	}

	return target, nil
}

func newStreamOptions(stdin io.Reader, stdout, stderr io.WriteCloser) *libpodRuntime.AttachStreams {
	return &libpodRuntime.AttachStreams{
		InputStream:  stdin,
		OutputStream: stdout,
		ErrorStream:  stderr,
		AttachInput:  stdin != nil,
		AttachOutput: stdout != nil,
		AttachError:  stderr != nil,
	}
}
