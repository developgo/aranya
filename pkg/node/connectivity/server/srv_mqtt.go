package server

import (
	"time"

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

func (m *MqttSrv) PostCmd(c *connectivity.Cmd, timeout time.Duration) (ch <-chan *connectivity.Msg, err error) {
	return m.baseServer.onPostCmd(c, timeout, func(c *connectivity.Cmd) error {
		return nil
	})
}