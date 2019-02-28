package connectivity

import (
	"time"
)

func NewMQTTService(name string) Interface {
	return &mqttService{}
}

type mqttService struct {
}

func (m *mqttService) ConsumeOrphanedMessage() <-chan *Msg {
	return nil
}

func (m *mqttService) WaitUntilDeviceConnected() {
}

func (m *mqttService) PostCmd(c *Cmd, timeout time.Duration) (ch <-chan *Msg, err error) {
	return nil, ErrDeviceNotConnected
}
