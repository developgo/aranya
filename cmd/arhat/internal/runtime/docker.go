// +build rt_docker

package runtime

import (
	"context"

	"arhat.dev/aranya/pkg/virtualnode/connectivity/client/runtime"
	"arhat.dev/aranya/pkg/virtualnode/connectivity/client/runtime/docker"
)

func New(ctx context.Context, config *runtime.Config) (runtime.Interface, error) {
	return docker.NewRuntime(ctx, config)
}
