// +build rt_containerd

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
