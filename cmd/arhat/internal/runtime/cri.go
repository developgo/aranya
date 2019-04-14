// +build rt_cri

package runtime

import (
	"context"

	"arhat.dev/aranya/pkg/node/connectivity/client/runtime"
	"arhat.dev/aranya/pkg/node/connectivity/client/runtime/containerd"
)

func GetRuntime(ctx context.Context, config *runtime.Config) (runtime.Interface, error) {
	return containerd.NewRuntime(ctx, config)
}
