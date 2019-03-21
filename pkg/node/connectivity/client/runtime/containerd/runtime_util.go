package containerd

import (
	"context"
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
	_ = cancel

	conn, err := grpc.DialContext(dialCtx, addr, grpc.WithInsecure(), grpc.WithBlock(), grpc.WithDialer(dialer))
	if err != nil {
		return nil, err
	}

	return conn, nil
}
