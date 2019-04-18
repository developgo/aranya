package manager

import (
	"context"
	"crypto/tls"

	aranya "arhat.dev/aranya/pkg/apis/aranya/v1alpha1"
	"arhat.dev/aranya/pkg/node/connectivity"
)

type mqttClient struct {
}

func NewMQTTManager(name string, config aranya.MQTTConfig, cert *tls.Certificate) Interface {
	return &MQTTManager{
		baseServer: newBaseServer(name),
	}
}

type MQTTManager struct {
	baseServer
}

func (m *MQTTManager) Start() error {
	return nil
}

func (m *MQTTManager) PostCmd(ctx context.Context, c *connectivity.Cmd) (ch <-chan *connectivity.Msg, err error) {
	return m.baseServer.onPostCmd(ctx, c, func(c *connectivity.Cmd) error {
		return nil
	})
}

func (m *MQTTManager) Close() {
	m.baseServer.onClose(func() {

	})
}
