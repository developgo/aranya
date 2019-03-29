// +build conn_grpc

package conn

import (
	"context"

	"google.golang.org/grpc"

	"arhat.dev/aranya/pkg/node/connectivity"
	"arhat.dev/aranya/pkg/node/connectivity/client"
	"arhat.dev/aranya/pkg/node/connectivity/client/runtime"
)

func GetConnectivityClient(ctx context.Context, config *connectivity.Config, rt runtime.Interface) (client.Interface, error) {
	dialCtx, cancel := context.WithTimeout(ctx, config.Server.DialTimeout)
	defer cancel()

	dialOptions := []grpc.DialOption{grpc.WithBlock()}

	if config.Server.TLS != nil {
		dialOptions = append(dialOptions)
	}

	conn, err := grpc.DialContext(dialCtx, config.Server.Address, dialOptions...)
	if err != nil {
		return nil, err
	}

	return client.NewGrpcClient(conn, rt)
}
