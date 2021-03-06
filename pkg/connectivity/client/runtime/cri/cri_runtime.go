// +build rt_cri

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

package cri

import (
	"context"
	"errors"
	"fmt"
	"io"
	goruntime "runtime"

	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/client-go/util/flowcontrol"
	criRuntime "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
	kubeletContainer "k8s.io/kubernetes/pkg/kubelet/container"

	"arhat.dev/aranya/pkg/connectivity"
	"arhat.dev/aranya/pkg/connectivity/client/runtime"
)

var (
	// ErrCriApiVersionNotSupported is returned when the api version of runtime interface is not supported
	ErrCriApiVersionNotSupported = errors.New("runtime api version is not supported")
)

func NewRuntime(ctx context.Context, config *runtime.Config) (*Runtime, error) {
	runtimeScvConn, err := dialSvcEndpoint(config.EndPoints.Runtime.Address, config.EndPoints.Runtime.DialTimeout)
	if err != nil {
		return nil, err
	}

	imageSvcConn, err := dialSvcEndpoint(config.EndPoints.Image.Address, config.EndPoints.Image.DialTimeout)
	if err != nil {
		return nil, err
	}

	runtimeSvcClient := criRuntime.NewRuntimeServiceClient(runtimeScvConn)

	ctx, cancel := context.WithTimeout(context.Background(), config.EndPoints.Runtime.ActionTimeout)
	defer cancel()

	typedVersion, err := runtimeSvcClient.Version(ctx, &criRuntime.VersionRequest{Version: runtime.KubeRuntimeAPIVersion})
	if err != nil {
		return nil, err
	}

	if typedVersion.Version == "" || typedVersion.RuntimeName == "" || typedVersion.RuntimeApiVersion == "" || typedVersion.RuntimeVersion == "" {
		return nil, fmt.Errorf("not all fields are set in VersionResponse (%q)", *typedVersion)
	}

	if typedVersion.GetRuntimeApiVersion() != runtime.KubeRuntimeAPIVersion {
		return nil, ErrCriApiVersionNotSupported
	}

	return &Runtime{
		Base: runtime.NewRuntimeBase(ctx, config, typedVersion.GetRuntimeName(), typedVersion.GetRuntimeVersion(), goruntime.GOOS, goruntime.GOARCH, ""),

		runtimeSvcClient:    runtimeSvcClient,
		imageSvcClient:      criRuntime.NewImageServiceClient(imageSvcConn),
		containerRefManager: kubeletContainer.NewRefManager(),
	}, nil
}

type Runtime struct {
	runtime.Base
	imageActionBackOff   *flowcontrol.Backoff
	runtimeActionBackOff *flowcontrol.Backoff

	runtimeSvcClient criRuntime.RuntimeServiceClient
	imageSvcClient   criRuntime.ImageServiceClient

	containerRefManager *kubeletContainer.RefManager
}

func (r *Runtime) CreatePod(options *connectivity.CreateOptions) (*connectivity.PodStatus, error) {
	return nil, errors.New("method not implemented")
}

func (r *Runtime) DeletePod(options *connectivity.DeleteOptions) (*connectivity.PodStatus, error) {
	return nil, errors.New("method not implemented")
}

func (r *Runtime) ListPods(options *connectivity.ListOptions) ([]*connectivity.PodStatus, error) {
	return nil, errors.New("method not implemented")
}

func (r *Runtime) ExecInContainer(podUID, container string, stdin io.Reader, stdout, stderr io.WriteCloser, resizeCh <-chan remotecommand.TerminalSize, command []string, tty bool) error {
	return errors.New("method not implemented")
}

func (r *Runtime) AttachContainer(podUID, container string, stdin io.Reader, stdout, stderr io.WriteCloser, resizeCh <-chan remotecommand.TerminalSize) error {
	return errors.New("method not implemented")
}

func (r *Runtime) GetContainerLogs(podUID string, options *connectivity.LogOptions, stdout, stderr io.WriteCloser) error {
	return errors.New("method not implemented")
}

func (r *Runtime) PortForward(podUID string, protocol string, port int32, in io.Reader, out io.WriteCloser) error {
	return errors.New("method not implemented")
}

func (r *Runtime) findPod(namespace, name string) (*criRuntime.PodSandbox, error) {
	pods, err := r.remoteListPodSandbox(&criRuntime.PodSandboxFilter{
		State: &criRuntime.PodSandboxStateValue{
			State: criRuntime.PodSandboxState_SANDBOX_READY,
		},
	})
	if err != nil {
		return nil, err
	}

	for _, p := range pods {
		if p.GetMetadata().GetNamespace() == namespace && p.GetMetadata().GetName() == name {
			return p, nil
		}
	}

	return nil, errors.New("pod not found")
}

func (r *Runtime) findContainer(namespace, name, container string) (*criRuntime.Container, error) {
	pod, err := r.findPod(namespace, name)
	if err != nil {
		return nil, err
	}

	containers, err := r.remoteListContainers(&criRuntime.ContainerFilter{
		State:        &criRuntime.ContainerStateValue{State: criRuntime.ContainerState_CONTAINER_RUNNING},
		PodSandboxId: pod.GetId(),
	})
	if err != nil {
		return nil, err
	}

	for _, c := range containers {
		if c.GetMetadata().GetName() == container {
			return c, nil
		}
	}

	return nil, errors.New("container not found")
}
