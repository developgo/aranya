// +build linux,rt_podman

package runtime

import (
	"context"

	"arhat.dev/aranya/pkg/node/agent/runtime"
	"arhat.dev/aranya/pkg/node/agent/runtime/podman"
)

func GetRuntime(ctx context.Context, config *runtime.Config) (runtime.Interface, error) {
	return podman.NewRuntime(ctx, config)
}
