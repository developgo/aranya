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

package connectivity

import (
	"time"
)

type TLSConfig struct {
	CaCert     string `json:"ca_cert" yaml:"ca_cert"`
	Cert       string `json:"cert" yaml:"cert"`
	Key        string `json:"key" yaml:"key"`
	ServerName string `json:"server_name" yaml:"server_name"`
}

type GRPCConfig struct {
	ServerAddress string        `json:"server_address" yaml:"server_address"`
	DialTimeout   time.Duration `json:"dial_timeout" yaml:"dial_timeout"`
	TLS           *TLSConfig    `json:"tls" yaml:"tls"`
}

type MQTTConfig struct {
	BrokerAddress string     `json:"broker_address" yaml:"broker_address"`
	TLS           *TLSConfig `json:"tls" yaml:"tls"`
}

type Config struct {
	MQTTConfig *MQTTConfig `json:"mqtt_config" yaml:"mqtt_config"`
	GRPCConfig *GRPCConfig `json:"grpc_config" yaml:"grpc_config"`
}
