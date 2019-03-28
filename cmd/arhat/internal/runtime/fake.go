// +build test

package runtime

import (
	"context"

	"arhat.dev/aranya/pkg/node/connectivity/client/runtime"
	"arhat.dev/aranya/pkg/node/connectivity/client/runtime/fake"
)

func GetRuntime(ctx context.Context, config *runtime.Config) (runtime.Interface, error) {
	return fake.NewFakeRuntime()
}
