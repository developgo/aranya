// +build linux,rt_podman

package runtime

import (
	"context"

	"arhat.dev/aranya/pkg/virtualnode/connectivity/client/runtime"
	"arhat.dev/aranya/pkg/virtualnode/connectivity/client/runtime/podman"
)

func New(ctx context.Context, config *runtime.Config) (runtime.Interface, error) {
	return podman.NewRuntime(ctx, config)
}
