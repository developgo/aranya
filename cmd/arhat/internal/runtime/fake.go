// +build rt_fake

package runtime

import (
	"context"

	"arhat.dev/aranya/pkg/virtualnode/agent/runtime"
	"arhat.dev/aranya/pkg/virtualnode/agent/runtime/fake"
)

func GetRuntime(ctx context.Context, config *runtime.Config) (runtime.Interface, error) {
	return fake.NewFakeRuntime(false)
}
