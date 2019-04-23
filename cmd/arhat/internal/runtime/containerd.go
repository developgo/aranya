// +build rt_containerd

package runtime

import (
	"context"

	"arhat.dev/aranya/pkg/virtualnode/connectivity/client/runtime"
	"arhat.dev/aranya/pkg/virtualnode/connectivity/client/runtime/containerd"
)

func New(ctx context.Context, config *runtime.Config) (runtime.Interface, error) {
	return containerd.NewRuntime(ctx, config)
}
