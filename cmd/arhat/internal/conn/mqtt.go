// +build conn_mqtt

package conn

import (
	"context"

	"arhat.dev/aranya/pkg/node/connectivity"
	"arhat.dev/aranya/pkg/node/connectivity/agent"
	"arhat.dev/aranya/pkg/node/connectivity/agent/runtime"
)

func GetConnectivityClient(ctx context.Context, config *connectivity.Config, rt runtime.Interface) (agent.Interface, error) {
	return nil, nil
}
