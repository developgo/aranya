package server

import (
	"context"

	"arhat.dev/aranya/pkg/node/connectivity"
)

func NewMqttManager(name string) Interface {
	return &MqttSrv{
		baseServer: newBaseServer(name),
	}
}

type MqttSrv struct {
	baseServer
}

func (m *MqttSrv) PostCmd(ctx context.Context, c *connectivity.Cmd) (ch <-chan *connectivity.Msg, err error) {
	return m.baseServer.onPostCmd(ctx, c, func(c *connectivity.Cmd) error {
		return nil
	})
}
