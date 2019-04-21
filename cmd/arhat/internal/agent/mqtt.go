// +build agent_mqtt

package agent

import (
	"context"

	"arhat.dev/aranya/pkg/virtualnode/agent"
	"arhat.dev/aranya/pkg/virtualnode/agent/runtime"
	"arhat.dev/aranya/pkg/virtualnode/connectivity"
)

func New(arhatCtx context.Context, agentConfig *agent.Config, connectivityConfig *connectivity.Config, rt runtime.Interface) (agent.Interface, error) {
	return nil, nil
}
