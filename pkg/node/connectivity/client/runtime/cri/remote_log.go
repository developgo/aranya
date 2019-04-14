package cri

import (
	"context"

	criRuntime "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
)

func (r *Runtime) ReopenContainerLog(containerID string) error {
	ctx, cancel := context.WithTimeout(r.ctx, r.runtimeActionTimeout)
	defer cancel()

	_, err := r.runtimeSvcClient.ReopenContainerLog(ctx, &criRuntime.ReopenContainerLogRequest{ContainerId: containerID})
	if err != nil {
		return err
	}
	return nil
}
