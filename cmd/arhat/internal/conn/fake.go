// +build conn_fake

package conn

import (
	"context"

	"arhat.dev/aranya/pkg/node/agent"
	"arhat.dev/aranya/pkg/node/agent/runtime"
	"arhat.dev/aranya/pkg/node/connectivity"
)

func GetConnectivityClient(ctx context.Context, config *connectivity.Config, rt runtime.Interface) (agent.Interface, error) {
	return nil, nil
}
