package cri

import (
	"context"
	"net"
	"time"

	"google.golang.org/grpc"
	kubeletUtil "k8s.io/kubernetes/pkg/kubelet/util"
)

func dialSvcEndpoint(endpoint string, dialTimeout time.Duration) (*grpc.ClientConn, error) {
	addr, dialer, err := kubeletUtil.GetAddressAndDialer(endpoint)
	if err != nil {
		return nil, err
	}

	dialCtx, cancel := context.WithTimeout(context.Background(), dialTimeout)
	defer cancel()

	dialOpts := []grpc.DialOption{
		grpc.WithInsecure(),
		grpc.WithBlock(),
		grpc.WithContextDialer(func(ctx context.Context, addr string) (conn net.Conn, e error) {
			deadline, _ := ctx.Deadline()
			return dialer(addr, deadline.Sub(time.Now()))
		}),
	}

	conn, err := grpc.DialContext(dialCtx, addr, dialOpts...)
	if err != nil {
		return nil, err
	}

	return conn, nil
}
