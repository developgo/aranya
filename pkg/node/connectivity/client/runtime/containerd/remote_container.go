package containerd

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	criRuntime "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
	utilExec "k8s.io/utils/exec"
)

// remoteContainerStatus returns the container status.
func (r *Runtime) remoteContainerStatus(containerID string) (*criRuntime.ContainerStatus, error) {
	ctx, cancel := context.WithTimeout(r.ctx, time.Minute*2)
	defer cancel()

	resp, err := r.runtimeSvcClient.ContainerStatus(ctx, &criRuntime.ContainerStatusRequest{
		ContainerId: containerID,
	})
	if err != nil {
		return nil, err
	}

	if resp.Status != nil {
		if err := verifyContainerStatus(resp.Status); err != nil {
			return nil, err
		}
	}

	return resp.Status, nil
}

// verifyContainerStatus verified whether all required fields are set in remoteContainerStatus.
func verifyContainerStatus(status *criRuntime.ContainerStatus) error {
	if status.Id == "" {
		return fmt.Errorf("Id is not set ")
	}

	if status.Metadata == nil {
		return fmt.Errorf("Metadata is not set ")
	}

	metadata := status.Metadata
	if metadata.Name == "" {
		return fmt.Errorf("Name is not in metadata %q ", metadata)
	}

	if status.CreatedAt == 0 {
		return fmt.Errorf("CreatedAt is not set ")
	}

	if status.Image == nil || status.Image.Image == "" {
		return fmt.Errorf("Image is not set ")
	}

	if status.ImageRef == "" {
		return fmt.Errorf("ImageRef is not set ")
	}

	return nil
}

// remoteCreateContainer creates a new container in the specified PodSandbox.
func (r *Runtime) remoteCreateContainer(podSandBoxID string, config *criRuntime.ContainerConfig, sandboxConfig *criRuntime.PodSandboxConfig) (string, error) {
	ctx, cancel := context.WithTimeout(r.ctx, time.Minute*2)
	defer cancel()

	resp, err := r.runtimeSvcClient.CreateContainer(ctx, &criRuntime.CreateContainerRequest{
		PodSandboxId:  podSandBoxID,
		Config:        config,
		SandboxConfig: sandboxConfig,
	})
	if err != nil {
		return "", err
	}

	if resp.ContainerId == "" {
		errorMessage := fmt.Sprintf("ContainerId is not set for container %q", config.GetMetadata())
		return "", errors.New(errorMessage)
	}

	return resp.ContainerId, nil
}

// remoteStartContainer starts the container in containerd.
func (r *Runtime) remoteStartContainer(containerID string) error {
	ctx, cancel := context.WithTimeout(r.ctx, r.runtimeActionTimeout)
	defer cancel()

	_, err := r.runtimeSvcClient.StartContainer(ctx, &criRuntime.StartContainerRequest{
		ContainerId: containerID,
	})
	if err != nil {
		return err
	}

	return nil
}

// remoteExecSync executes a command in the container, and returns the stdout output.
// If command exits with a non-zero exit code, an error is returned.
func (r *Runtime) remoteExecSync(containerID string, cmd []string, timeout time.Duration) (stdout []byte, stderr []byte, err error) {
	// Do not set timeout when timeout is 0.
	var ctx context.Context
	var cancel context.CancelFunc
	if timeout != 0 {
		// Use timeout + default timeout (2 minutes) as timeout to leave some time for
		// the runtime to do cleanup.
		ctx, cancel = context.WithTimeout(r.ctx, r.runtimeActionTimeout+timeout)
	} else {
		ctx, cancel = context.WithCancel(r.ctx)
	}
	defer cancel()

	timeoutSeconds := int64(timeout.Seconds())
	req := &criRuntime.ExecSyncRequest{
		ContainerId: containerID,
		Cmd:         cmd,
		Timeout:     timeoutSeconds,
	}
	resp, err := r.runtimeSvcClient.ExecSync(ctx, req)
	if err != nil {
		return nil, nil, err
	}

	err = nil
	if resp.ExitCode != 0 {
		err = utilExec.CodeExitError{
			Err:  fmt.Errorf("command '%s' exited with %d: %s", strings.Join(cmd, " "), resp.ExitCode, resp.Stderr),
			Code: int(resp.ExitCode),
		}
	}

	return resp.Stdout, resp.Stderr, err
}

// remoteStopContainer stops a running container with a grace period (i.e., timeout).
func (r *Runtime) remoteStopContainer(containerID string, timeout int64) error {
	// Use timeout + default timeout (2 minutes) as timeout to leave extra time
	// for SIGKILL container and request latency.
	t := r.runtimeActionTimeout + time.Duration(timeout)*time.Second
	ctx, cancel := context.WithTimeout(r.ctx, t)
	defer cancel()

	_, err := r.runtimeSvcClient.StopContainer(ctx, &criRuntime.StopContainerRequest{
		ContainerId: containerID,
		Timeout:     timeout,
	})
	if err != nil {
		return err
	}

	return nil
}

// remoteRemoveContainer removes the container. If the container is running, the container
// should be forced to removal.
func (r *Runtime) remoteRemoveContainer(containerID string) error {
	ctx, cancel := context.WithTimeout(r.ctx, r.runtimeActionTimeout)
	defer cancel()

	_, err := r.runtimeSvcClient.RemoveContainer(ctx, &criRuntime.RemoveContainerRequest{
		ContainerId: containerID,
	})
	if err != nil {
		return err
	}

	return nil
}

// remoteListContainers lists containers by filters.
func (r *Runtime) remoteListContainers(filter *criRuntime.ContainerFilter) ([]*criRuntime.Container, error) {
	ctx, cancel := context.WithTimeout(r.ctx, r.runtimeActionTimeout)
	defer cancel()

	resp, err := r.runtimeSvcClient.ListContainers(ctx, &criRuntime.ListContainersRequest{
		Filter: filter,
	})
	if err != nil {
		return nil, err
	}

	return resp.Containers, nil
}

// remoteUpdateContainerResources updates a containers resource config
func (r *Runtime) remoteUpdateContainerResources(containerID string, resources *criRuntime.LinuxContainerResources) error {
	ctx, cancel := context.WithTimeout(r.ctx, r.runtimeActionTimeout)
	defer cancel()

	_, err := r.runtimeSvcClient.UpdateContainerResources(ctx, &criRuntime.UpdateContainerResourcesRequest{
		ContainerId: containerID,
		Linux:       resources,
	})
	if err != nil {
		return err
	}

	return nil
}

// remoteExec prepares a streaming endpoint to execute a command in the container, and returns the address.
func (r *Runtime) remoteExec(req *criRuntime.ExecRequest) (*criRuntime.ExecResponse, error) {
	ctx, cancel := context.WithTimeout(r.ctx, r.runtimeActionTimeout)
	defer cancel()

	resp, err := r.runtimeSvcClient.Exec(ctx, req)
	if err != nil {
		return nil, err
	}

	if resp.Url == "" {
		errorMessage := "URL is not set"
		return nil, errors.New(errorMessage)
	}

	return resp, nil
}

// remoteAttach prepares a streaming endpoint to attach to a running container, and returns the address.
func (r *Runtime) remoteAttach(req *criRuntime.AttachRequest) (*criRuntime.AttachResponse, error) {
	ctx, cancel := context.WithTimeout(r.ctx, r.runtimeActionTimeout)
	defer cancel()

	resp, err := r.runtimeSvcClient.Attach(ctx, req)
	if err != nil {
		return nil, err
	}

	if resp.Url == "" {
		errorMessage := "URL is not set"
		return nil, errors.New(errorMessage)
	}
	return resp, nil
}

// remoteContainerStats returns the stats of the container.
func (r *Runtime) remoteContainerStats(containerID string) (*criRuntime.ContainerStats, error) {
	ctx, cancel := context.WithTimeout(r.ctx, r.runtimeActionTimeout)
	defer cancel()

	resp, err := r.runtimeSvcClient.ContainerStats(ctx, &criRuntime.ContainerStatsRequest{
		ContainerId: containerID,
	})
	if err != nil {
		return nil, err
	}

	return resp.GetStats(), nil
}

func (r *Runtime) remoteListContainerStats(filter *criRuntime.ContainerStatsFilter) ([]*criRuntime.ContainerStats, error) {
	// Do not set timeout, because writable layer stats collection takes time.
	ctx, cancel := context.WithCancel(r.ctx)
	defer cancel()

	resp, err := r.runtimeSvcClient.ListContainerStats(ctx, &criRuntime.ListContainerStatsRequest{
		Filter: filter,
	})
	if err != nil {
		return nil, err
	}

	return resp.GetStats(), nil
}
