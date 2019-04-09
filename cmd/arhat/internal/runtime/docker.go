// +build rt_docker

package runtime

import (
	"context"

	"arhat.dev/aranya/pkg/node/connectivity/client/runtime"
	"arhat.dev/aranya/pkg/node/connectivity/client/runtime/docker"
)

func GetRuntime(ctx context.Context, config *runtime.Config) (runtime.Interface, error) {
	return docker.NewRuntime(ctx, config)
}