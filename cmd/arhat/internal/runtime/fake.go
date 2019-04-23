// +build rt_fake

package runtime

import (
	"context"

	"arhat.dev/aranya/pkg/virtualnode/connectivity/client/runtime"
	"arhat.dev/aranya/pkg/virtualnode/connectivity/client/runtime/fake"
)

func New(ctx context.Context, config *runtime.Config) (runtime.Interface, error) {
	return fake.NewFakeRuntime(false)
}
