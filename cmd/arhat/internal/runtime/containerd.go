// +build linux,containerd

package runtime

import (
	"arhat.dev/aranya/pkg/node/connectivity/client/runtime"
	"arhat.dev/aranya/pkg/node/connectivity/client/runtime/containerd"
	"context"
)

func GetRuntime(ctx context.Context, config *runtime.Config) (runtime.Interface, error) {
	return containerd.NewRuntime(ctx, config)
}
