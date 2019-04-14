package containerd

import (
	"context"
	"errors"

	"github.com/containerd/containerd"

	"arhat.dev/aranya/pkg/constant"
)

func (r *containerdRuntime) findContainer(ctx context.Context, podUID, container string) (containerd.Container, error) {
	containers, err := r.runtimeClient.Containers(ctx)
	if err != nil {
		return nil, err
	}
	for _, ctr := range containers {
		labels, err := ctr.Labels(ctx)
		if err != nil {
			continue
		}

		if labels[constant.ContainerLabelPodUID] == podUID && labels[constant.ContainerLabelPodContainer] == container {
			return ctr, nil
		}
	}
	return nil, errors.New("container not found")
}
