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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CertInfo is the extra location info for device
// +k8s:openapi-gen=true
type CertInfo struct {
	Country          string `json:"country,omitempty" protobuf:"bytes,1,opt,name=country"`
	State            string `json:"state,omitempty" protobuf:"bytes,2,opt,name=state"`
	Locality         string `json:"locality,omitempty" protobuf:"bytes,3,opt,name=locality"`
	Organisation     string `json:"org,omitempty" protobuf:"bytes,4,opt,name=org"`
	OrganisationUnit string `json:"orgUnit,omitempty" protobuf:"bytes,5,opt,name=orgUnit"`
}

// EdgeDeviceSpec defines the desired state of EdgeDevice
// +k8s:openapi-gen=true
type EdgeDeviceSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book.kubebuilder.io/beyond_basics/generating_crd.html

	CertInfo CertInfo `json:"certInfo,omitempty" protobuf:"bytes,1,opt,name=certInfo"`
	// Connectivity designate the method by which this device connect to aranya server
	Connectivity Connectivity `json:"connectivity,omitempty" protobuf:"bytes,2,opt,name=connectivity"`
}

// EdgeDeviceStatus defines the observed state of EdgeDevice
// +k8s:openapi-gen=true
type EdgeDeviceStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book.kubebuilder.io/beyond_basics/generating_crd.html
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// EdgeDevice is the Schema for the edgedevices API
// +k8s:openapi-gen=true
type EdgeDevice struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   EdgeDeviceSpec   `json:"spec,omitempty"`
	Status EdgeDeviceStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// EdgeDeviceList contains a list of EdgeDevice
type EdgeDeviceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []EdgeDevice `json:"items"`
}

func init() {
	SchemeBuilder.Register(&EdgeDevice{}, &EdgeDeviceList{})
}

type ConnectivityMethod string

const (
	MQTT ConnectivityMethod = "mqtt"
	GRPC ConnectivityMethod = "grpc"
)

type MQTTConnectPacket struct {
	CleanSession bool                   `json:"cleanSession,omitempty" protobuf:"bytes,1,opt,name=cleanSession"`
	Will         bool                   `json:"will,omitempty" protobuf:"bytes,2,opt,name=will"`
	WillQos      int32                  `json:"willQos,omitempty" protobuf:"bytes,3,opt,name=willQos"`
	WillRetain   bool                   `json:"willRetain,omitempty" protobuf:"bytes,4,opt,name=willRetain"`
	WillTopic    string                 `json:"willTopic,omitempty" protobuf:"bytes,5,opt,name=willTopic"`
	WillMessage  string                 `json:"willMessage,omitempty" protobuf:"bytes,6,opt,name=willMessage"`
	Username     string                 `json:"username,omitempty" protobuf:"bytes,7,opt,name=username"`
	Password     corev1.SecretReference `json:"password,omitempty" protobuf:"bytes,8,opt,name=password"`
	ClientID     string                 `json:"clientID,omitempty" protobuf:"bytes,9,opt,name=clientID"`
	Keepalive    int32                  `json:"keepalive,omitempty" protobuf:"bytes,10,opt,name=keepalive"`
}

type MQTTConfig struct {
	MessageNamespace string `json:"messageNamespace,omitempty" protobuf:"bytes,1,opt,name=messageNamespace"`
	// MQTT broker address in the form of addr:port
	Broker string `json:"server,omitempty" protobuf:"bytes,2,opt,name=server"`
	// MQTT protocol version one of [3.1.1]
	Version string `json:"version,omitempty" protobuf:"bytes,3,opt,name=version"`
	// Transport protocol underlying the MQTT protocol, one of [tcp, websocket]
	Transport string `json:"transport,omitempty" protobuf:"bytes,4,opt,name=transport"`
	// Secret for transport layer security
	TLSSecretRef *corev1.SecretReference `json:"tlsSecretRef,omitempty" protobuf:"bytes,5,opt,name=tlsSecret"`
	// ConnectPacket in MQTT
	ConnectPacket *MQTTConnectPacket `json:"connectPacket,omitempty" protobuf:"bytes,6,opt,name=connectPacket"`
}

type GRPCConfig struct {
	// Secret for transport layer security
	TLSSecretRef *corev1.SecretReference `json:"tlsSecretRef,omitempty" protobuf:"bytes,1,opt,name=tlsSecretRef"`
}

type Connectivity struct {
	// Method of how to establish communication channel between server and devices
	Method ConnectivityMethod `json:"method,omitempty" protobuf:"bytes,1,opt,name=method"`
	// Configurations to tell aranya server how to create server(s) or client(s)
	MQTTConfig MQTTConfig `json:"mqttConfig,omitempty" protobuf:"bytes,2,opt,name=mqttConfig"`
	GRPCConfig GRPCConfig `json:"grpcConfig,omitempty" protobuf:"bytes,3,opt,name=grpcConfig"`
}
