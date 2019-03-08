package server

import (
	"time"

	"arhat.dev/aranya/pkg/node/connectivity"
)

func NewMqttSrv(name string) *MqttSrv {
	return &MqttSrv{}
}

type MqttSrv struct {
}

func (m *MqttSrv) ConsumeOrphanedMessage() <-chan *connectivity.Msg {
	return nil
}

func (m *MqttSrv) WaitUntilDeviceConnected() {
}

func (m *MqttSrv) PostCmd(c *connectivity.Cmd, timeout time.Duration) (ch <-chan *connectivity.Msg, err error) {
	return nil, ErrDeviceNotConnected
}
