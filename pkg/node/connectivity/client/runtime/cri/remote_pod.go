package cri

import (
	"context"
	"errors"
	"fmt"

	criRuntime "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
)

// remoteRunPodSandbox creates and starts a pod-level sandbox. Runtimes should ensure
// the sandbox is in ready state.
func (r *Runtime) remoteRunPodSandbox(config *criRuntime.PodSandboxConfig, runtimeHandler string) (string, error) {
	ctx, cancel := context.WithTimeout(r.ctx, 2*r.runtimeActionTimeout)
	defer cancel()

	resp, err := r.runtimeSvcClient.RunPodSandbox(ctx, &criRuntime.RunPodSandboxRequest{
		Config:         config,
		RuntimeHandler: runtimeHandler,
	})
	if err != nil {
		return "", err
	}

	if resp.PodSandboxId == "" {
		errorMessage := fmt.Sprintf("PodSandboxId is not set for sandbox %q", config.GetMetadata())
		return "", errors.New(errorMessage)
	}

	return resp.PodSandboxId, nil
}

// remoteStopPodSandbox stops the sandbox. If there are any running containers in the
// sandbox, they should be forced to termination.
func (r *Runtime) remoteStopPodSandbox(podSandBoxID string) error {
	ctx, cancel := context.WithTimeout(r.ctx, r.runtimeActionTimeout)
	defer cancel()

	_, err := r.runtimeSvcClient.StopPodSandbox(ctx, &criRuntime.StopPodSandboxRequest{
		PodSandboxId: podSandBoxID,
	})
	if err != nil {
		return err
	}

	return nil
}

// remoteRemovePodSandbox removes the sandbox. If there are any containers in the
// sandbox, they should be forcibly removed.
func (r *Runtime) remoteRemovePodSandbox(podSandBoxID string) error {
	ctx, cancel := context.WithTimeout(r.ctx, r.runtimeActionTimeout)
	defer cancel()

	_, err := r.runtimeSvcClient.RemovePodSandbox(ctx, &criRuntime.RemovePodSandboxRequest{
		PodSandboxId: podSandBoxID,
	})
	if err != nil {
		return err
	}

	return nil
}

// remoteListPodSandbox returns a list of PodSandboxes.
func (r *Runtime) remoteListPodSandbox(filter *criRuntime.PodSandboxFilter) ([]*criRuntime.PodSandbox, error) {
	ctx, cancel := context.WithTimeout(r.ctx, r.runtimeActionTimeout)
	defer cancel()

	resp, err := r.runtimeSvcClient.ListPodSandbox(ctx, &criRuntime.ListPodSandboxRequest{
		Filter: filter,
	})
	if err != nil {
		return nil, err
	}

	return resp.Items, nil
}

// remotePodSandboxStatus returns the status of the PodSandbox.
func (r *Runtime) remotePodSandboxStatus(podSandBoxID string) (*criRuntime.PodSandboxStatus, error) {
	ctx, cancel := context.WithTimeout(r.ctx, r.runtimeActionTimeout)
	defer cancel()

	resp, err := r.runtimeSvcClient.PodSandboxStatus(ctx, &criRuntime.PodSandboxStatusRequest{
		PodSandboxId: podSandBoxID,
	})
	if err != nil {
		return nil, err
	}

	if resp.Status != nil {
		if err := verifySandboxStatus(resp.Status); err != nil {
			return nil, err
		}
	}

	return resp.Status, nil
}

// remotePortForward prepares a streaming endpoint to forward ports from a PodSandbox, and returns the address.
func (r *Runtime) remotePortForward(req *criRuntime.PortForwardRequest) (*criRuntime.PortForwardResponse, error) {
	ctx, cancel := context.WithTimeout(r.ctx, r.runtimeActionTimeout)
	defer cancel()

	resp, err := r.runtimeSvcClient.PortForward(ctx, req)
	if err != nil {
		return nil, err
	}

	if resp.Url == "" {
		errorMessage := "URL is not set"
		return nil, errors.New(errorMessage)
	}

	return resp, nil
}

// verifySandboxStatus verified whether all required fields are set in PodSandboxStatus.
func verifySandboxStatus(status *criRuntime.PodSandboxStatus) error {
	if status.Id == "" {
		return fmt.Errorf("Id is not set ")
	}

	if status.Metadata == nil {
		return fmt.Errorf("Metadata is not set ")
	}

	metadata := status.Metadata
	if metadata.Name == "" || metadata.Namespace == "" || metadata.Uid == "" {
		return fmt.Errorf("Name, Namespace or Uid is not in metadata %q ", metadata)
	}

	if status.CreatedAt == 0 {
		return fmt.Errorf("CreatedAt is not set ")
	}

	return nil
}
