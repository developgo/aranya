// +build linux,rt_podman

package runtime

import (
	"context"

	"arhat.dev/aranya/pkg/node/connectivity/client/runtime"
	"arhat.dev/aranya/pkg/node/connectivity/client/runtime/podman"
)

func GetRuntime(ctx context.Context, config *runtime.Config) (runtime.Interface, error) {
	return podman.NewRuntime(ctx, config)
}
