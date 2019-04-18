// +build rt_containerd

package cri

import (
	"context"
	"errors"

	criRuntime "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
)

// remoteStatus returns the status of the runtime.
func (r *Runtime) remoteStatus() (*criRuntime.RuntimeStatus, error) {
	ctx, cancel := context.WithTimeout(r.ctx, r.runtimeActionTimeout)
	defer cancel()

	resp, err := r.runtimeSvcClient.Status(ctx, &criRuntime.StatusRequest{})
	if err != nil {
		return nil, err
	}

	if resp.Status == nil || len(resp.Status.Conditions) < 2 {
		errorMessage := "RuntimeReady or NetworkReady condition are not set"
		return nil, errors.New(errorMessage)
	}

	return resp.Status, nil
}
