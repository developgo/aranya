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
