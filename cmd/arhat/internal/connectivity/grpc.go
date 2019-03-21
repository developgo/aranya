package connectivity

import (
	aranya "arhat.dev/aranya/pkg/apis/aranya/v1alpha1"
	"arhat.dev/aranya/pkg/node/connectivity"
	"arhat.dev/aranya/pkg/node/connectivity/client"
	"arhat.dev/aranya/pkg/node/connectivity/client/runtime"
	"context"
	"google.golang.org/grpc"
)

func GetConnectivityClient(ctx context.Context, config *connectivity.Config, rt runtime.Interface) (client.Interface, error) {
	if config.Method != string(aranya.DeviceConnectViaGRPC) {
		return nil, ErrConnectivityMethodNotSupported
	}

	dialCtx, cancel := context.WithTimeout(ctx, config.DialTimeout)
	defer cancel()

	dialOptions := []grpc.DialOption{grpc.WithBlock()}

	if config.TLSCert != "" && config.TLSKey != "" {
		dialOptions = append(dialOptions)
	}

	conn, err := grpc.DialContext(dialCtx, config.ServerEndpoint, dialOptions...)
	if err != nil {
		return nil, err
	}

	return client.NewGrpcClient(conn, rt)
}
