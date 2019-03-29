// +build conn_fake

package conn

import (
	"context"

	"arhat.dev/aranya/pkg/node/connectivity"
	"arhat.dev/aranya/pkg/node/connectivity/client"
	"arhat.dev/aranya/pkg/node/connectivity/client/runtime"
)

func GetConnectivityClient(ctx context.Context, config *connectivity.Config, rt runtime.Interface) (client.Interface, error) {
	return nil, nil
}
