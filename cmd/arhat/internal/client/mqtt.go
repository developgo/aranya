// +build agent_mqtt

package client

import (
	"context"

	"arhat.dev/aranya/pkg/virtualnode/connectivity"
	"arhat.dev/aranya/pkg/virtualnode/connectivity/client"
	"arhat.dev/aranya/pkg/virtualnode/connectivity/client/runtime"
)

func New(arhatCtx context.Context, agentConfig *client.Config, connectivityConfig *connectivity.Config, rt runtime.Interface) (client.Interface, error) {
	return nil, nil
}
