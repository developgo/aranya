package manager

import (
	"context"
	"crypto/tls"

	aranya "arhat.dev/aranya/pkg/apis/aranya/v1alpha1"
	"arhat.dev/aranya/pkg/node/connectivity"
)

var _ Interface = &MQTTManager{}

type mqttClient struct {
}

func NewMQTTManager(config aranya.MQTTConfig, clientCert *tls.Certificate) (*MQTTManager, error) {
	return &MQTTManager{
		baseManager: newBaseServer(),
	}, nil
}

type MQTTManager struct {
	baseManager
}

func (m *MQTTManager) Start() error {
	return nil
}

func (m *MQTTManager) PostCmd(ctx context.Context, c *connectivity.Cmd) (ch <-chan *connectivity.Msg, err error) {
	return m.baseManager.onPostCmd(ctx, c, func(c *connectivity.Cmd) error {
		return nil
	})
}

func (m *MQTTManager) Stop() {
	m.baseManager.onStop(func() {

	})
}
