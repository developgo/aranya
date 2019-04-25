/*
Copyright 2019 The arhat.dev Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package server

import (
	"context"
	"crypto/tls"
	"sync"

	aranya "arhat.dev/aranya/pkg/apis/aranya/v1alpha1"
	"arhat.dev/aranya/pkg/connectivity"
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
