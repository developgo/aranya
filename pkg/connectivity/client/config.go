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

package client

import (
	"time"
)

type NodeConfig struct {
	Timers struct {
		StatusSyncInterval time.Duration `json:"status_sync_interval" yaml:"status_sync_interval"`
	} `json:"timers" yaml:"timers"`
}

type PodConfig struct {
	MaxPodCount int `json:"max_pod_count" yaml:"max_pod_count"`
	Timers      struct {
		StatusSyncInterval time.Duration `json:"status_sync_interval" yaml:"status_sync_interval"`
	} `json:"timers" yaml:"timers"`
}

// AgentConfig configuration for agent part in arhat
type AgentConfig struct {
	Log struct {
		Level int    `json:"level" yaml:"level"`
		Dir   string `json:"dir" yaml:"dir"`
	} `json:"log" yaml:"log"`

	Features struct {
		AllowHostAttach      bool `json:"allow_host_attach" yaml:"allow_host_attach"`
		AllowHostExec        bool `json:"allow_host_exec" yaml:"allow_host_exec"`
		AllowHostLog         bool `json:"allow_host_log" yaml:"allow_host_log"`
		AllowHostPortForward bool `json:"allow_host_port_forward" yaml:"allow_host_port_forward"`
	} `json:"features" yaml:"features"`

	Node NodeConfig `json:"node" yaml:"node"`
	Pod  PodConfig  `json:"pod" yaml:"pod"`
}

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
	BrokerAddress string        `json:"broker_address" yaml:"broker_address"`
	DialTimeout   time.Duration `json:"dial_timeout" yaml:"dial_timeout"`
	TLS           *TLSConfig    `json:"tls" yaml:"tls"`

	// mqtt connect packet
	ConnectPacket struct {
		CleanSession bool   `json:"cleanSession" yaml:"cleanSession"`
		Will         bool   `json:"will" yaml:"will"`
		WillQos      int32  `json:"willQos" yaml:"willQos"`
		WillRetain   bool   `json:"willRetain" yaml:"willRetain"`
		WillTopic    string `json:"willTopic" yaml:"willTopic"`
		WillMessage  string `json:"willMessage" yaml:"willMessage"`
		Username     string `json:"username" yaml:"username"`
		Password     string `json:"password" yaml:"password"`
		ClientID     string `json:"clientID" yaml:"clientID"`
		Keepalive    int32  `json:"keepalive" yaml:"keepalive"`
	} `json:"connect_packet" yaml:"connect_packet"`
}

// ConnectivityConfig configuration for connectivity part in arhat
type ConnectivityConfig struct {
	MQTTConfig *MQTTConfig `json:"mqtt_config" yaml:"mqtt_config"`
	GRPCConfig *GRPCConfig `json:"grpc_config" yaml:"grpc_config"`
}
