package manager

import (
	"context"
	"crypto/tls"
	"sync"

	aranya "arhat.dev/aranya/pkg/apis/aranya/v1alpha1"
	"arhat.dev/aranya/pkg/node/connectivity"
)

var _ Manager = &MQTTManager{}

type mqttClient struct {
}

func NewMQTTManager(config aranya.MQTTConfig, clientCert *tls.Certificate) (*MQTTManager, error) {
	return &MQTTManager{
		baseManager: newBaseServer(),
	}, nil
}

type MQTTManager struct {
	baseManager

	wg sync.WaitGroup
}

func (m *MQTTManager) Start() error {
	m.wg.Add(1)
	m.wg.Wait()
	return nil
}

func (m *MQTTManager) PostCmd(ctx context.Context, c *connectivity.Cmd) (ch <-chan *connectivity.Msg, err error) {
	return m.baseManager.onPostCmd(ctx, c, func(c *connectivity.Cmd) error {
		return nil
	})
}

func (m *MQTTManager) Stop() {
	m.baseManager.onStop(func() {
		m.wg.Done()
	})
}
