// +build mqtt

package connectivity

import (
	"context"

	aranya "arhat.dev/aranya/pkg/apis/aranya/v1alpha1"
	"arhat.dev/aranya/pkg/node/connectivity"
	"arhat.dev/aranya/pkg/node/connectivity/client"
	"arhat.dev/aranya/pkg/node/connectivity/client/runtime"
)

func GetConnectivityClient(ctx context.Context, config *connectivity.Config, rt runtime.Interface) (client.Interface, error) {
	if config.Method != string(aranya.DeviceConnectViaMQTT) {
		return nil, ErrConnectivityMethodNotSupported
	}

	return nil, nil
}
