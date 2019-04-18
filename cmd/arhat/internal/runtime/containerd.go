// +build rt_containerd

package runtime

import (
	"context"

	"arhat.dev/aranya/pkg/node/agent/runtime"
	"arhat.dev/aranya/pkg/node/agent/runtime/containerd"
)

func GetRuntime(ctx context.Context, config *runtime.Config) (runtime.Interface, error) {
	return containerd.NewRuntime(ctx, config)
}
