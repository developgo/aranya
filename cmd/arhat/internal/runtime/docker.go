// +build rt_docker

package runtime

import (
	"context"

	"arhat.dev/aranya/pkg/node/connectivity/agent/runtime"
	"arhat.dev/aranya/pkg/node/connectivity/agent/runtime/docker"
)

func GetRuntime(ctx context.Context, config *runtime.Config) (runtime.Interface, error) {
	return docker.NewRuntime(ctx, config)
}
