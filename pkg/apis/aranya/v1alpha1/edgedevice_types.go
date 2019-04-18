package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EdgeDeviceSpec defines the desired state of EdgeDevice
// +k8s:openapi-gen=true
type EdgeDeviceSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book.kubebuilder.io/beyond_basics/generating_crd.html

	// Unschedulable controls device schedulability of new pods. By default, device is schedulable.
	// +optional
	Unschedulable bool `json:"unschedulable,omitempty" protobuf:"varint,1,opt,name=unschedulable"`
	// If specified, the device's taints.
	// +optional
	Taints []corev1.Taint `json:"taints,omitempty" protobuf:"bytes,2,opt,name=taints"`
	// Connectivity designate the method by which this device connect to aranya server
	Connectivity Connectivity `json:"connectivity,omitempty" protobuf:"bytes,3,opt,name=connectivity"`
	// If specified, the source to get device configuration from
	// The DynamicKubeletConfig feature gate must be enabled for the Kubelet to use this field
	// +optional
	ConfigSource *DeviceConfigSource `json:"configSource,omitempty" protobuf:"bytes,4,opt,name=configSource"`
}

// EdgeDeviceStatus defines the observed state of EdgeDevice
// +k8s:openapi-gen=true
type EdgeDeviceStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book.kubebuilder.io/beyond_basics/generating_crd.html

	// Capacity represents the total resources of a device.
	// +optional
	Capacity corev1.ResourceList `json:"capacity,omitempty" protobuf:"bytes,1,rep,name=capacity,casttype=ResourceList,castkey=ResourceName"`
	// Allocatable represents the resources of a device that are available for scheduling.
	// Defaults to Capacity.
	// +optional
	Allocatable corev1.ResourceList `json:"allocatable,omitempty" protobuf:"bytes,2,rep,name=allocatable,casttype=ResourceList,castkey=ResourceName"`
	// NodePhase is the recently observed lifecycle phase of the device.
	// The field is never populated, and now is deprecated.
	// +optional
	Phase DevicePhase `json:"phase,omitempty" protobuf:"bytes,3,opt,name=phase,casttype=DevicePhase"`
	// Conditions is an array of current observed device conditions.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions []DeviceCondition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,4,rep,name=conditions"`
	// List of network reachable from the device.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	Networks []Network `json:"networks,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,5,rep,name=networks"`
	// Set of ids/uuids to uniquely identify the device.
	// +optional
	DeviceInfo DeviceSystemInfo `json:"deviceInfo,omitempty" protobuf:"bytes,6,opt,name=deviceInfo"`
	// List of container images on this device
	// +optional
	Images []corev1.ContainerImage `json:"images,omitempty" protobuf:"bytes,7,rep,name=images"`
	// List of attachable volumes in use (mounted) by the device.
	// +optional
	VolumesInUse []corev1.UniqueVolumeName `json:"volumesInUse,omitempty" protobuf:"bytes,8,rep,name=volumesInUse"`
	// List of volumes that are attached to the device.
	// +optional
	VolumesAttached []corev1.AttachedVolume `json:"volumesAttached,omitempty" protobuf:"bytes,9,rep,name=volumesAttached"`
	// Status of the config assigned to the device via the dynamic Kubelet config feature.
	// +optional
	Config *DeviceConfigSource `json:"config,omitempty" protobuf:"bytes,10,opt,name=config"`
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

type DeviceConnectMethod string

const (
	DeviceConnectViaMQTT DeviceConnectMethod = "mqtt"
	DeviceConnectViaGRPC DeviceConnectMethod = "grpc"
)

type DeviceConnectConfig struct {
	MQTT *ConnectViaMQTTConfig `json:"mqtt,omitempty" protobuf:"bytes,1,opt,name=mqtt"`
	GRPC *ConnectViaGRPCConfig `json:"grpc,omitempty" protobuf:"bytes,2,opt,name=grpc"`
}

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
	// MQTT broker address in the form of addr:port
	Server string `json:"server,omitempty" protobuf:"bytes,1,opt,name=server"`
	// MQTT protocol version one of [3.1.1]
	Version string `json:"version,omitempty" protobuf:"bytes,2,opt,name=version"`
	// Transport protocol underlying the MQTT protocol, one of [tcp, websocket]
	Transport string `json:"transport,omitempty" protobuf:"bytes,3,opt,name=transport"`
	// Secret for transport layer security
	TLSSecretRef *corev1.SecretReference `json:"tlsSecretRef,omitempty" protobuf:"bytes,4,opt,name=tlsSecret"`
	// ConnectPacket in MQTT
	ConnectPacket *MQTTConnectPacket `json:"connectPacket,omitempty" protobuf:"bytes,3,opt,name=connectPacket"`
}

type ConnectViaMQTTConfig struct {
	// Namespace for communications between aranya and arhat (string prefix of topic name)
	Namespace string `json:"namespace,omitempty" protobuf:"bytes,1,opt,name=broker"`
	// Config for Aranya server to connect to MQTT Broker
	// multiple config for with identical value will be treated as one, and only one connection will be established
	ForServer MQTTConfig `json:"forServer,omitempty" protobuf:"bytes,2,opt,name=forServer"`
	// Config for Arhat device to connect to MQTT Broker
	ForDevice MQTTConfig `json:"forDevice,omitempty" protobuf:"bytes,3,opt,name=forDevice"`
}

type GRPCConfig struct {
	// Server address of aranya server's public address
	Server string `json:"server,omitempty" protobuf:"bytes,1,opt,name=server"`
	// Secret for transport layer security
	TLSSecretRef *corev1.SecretReference `json:"tlsSecretRef,omitempty" protobuf:"bytes,2,opt,name=tlsSecretRef"`
}

// ConnectViaGRPCConfig configuration used when devices directly connect to aranya server using gRPC
type ConnectViaGRPCConfig struct {
	ForDevice GRPCConfig `json:"forDevice,omitempty" protobuf:"bytes,1,opt,name=forDevice"`
	ForServer GRPCConfig `json:"forServer,omitempty" protobuf:"bytes,2,opt,name=forServer"`
}

type Connectivity struct {
	// Method of how to establish communication channel between server and devices
	Method DeviceConnectMethod `json:"method,omitempty" protobuf:"bytes,1,opt,name=method"`
	// Config both aranya server and arhat devices
	Config DeviceConnectConfig `json:"config,omitempty" protobuf:"bytes,2,opt,name=config"`
}

type DevicePhase string

const (
	DevicePhaseNotFound   DevicePhase = "NotFound"
	DevicePhaseAlive      DevicePhase = "Alive"
	DevicePhaseLost       DevicePhase = "Lost"
	DevicePhaseTerminated DevicePhase = "Terminated"
)

// DeviceSystemInfo is a set of ids/uuids to uniquely identify the device.
// * resembles corev1.NodeSystemInfo
type DeviceSystemInfo struct {
	// MachineID reported by the device. For unique machine identification
	// in the cluster this field is preferred. Learn more from man(5)
	// machine-id: http://man7.org/linux/man-pages/man5/machine-id.5.html
	MachineID string `json:"machineID" protobuf:"bytes,1,opt,name=machineID"`
	// SystemUUID reported by the device. For unique machine identification
	// MachineID is preferred. This field is specific to Red Hat hosts
	// https://access.redhat.com/documentation/en-US/Red_Hat_Subscription_Management/1/html/RHSM/getting-system-uuid.html
	SystemUUID string `json:"systemUUID" protobuf:"bytes,2,opt,name=systemUUID"`
	// Boot ID reported by the device.
	BootID string `json:"bootID" protobuf:"bytes,3,opt,name=bootID"`
	// Kernel Version reported by the device from 'uname -r' (e.g. 3.16.0-0.bpo.4-amd64).
	KernelVersion string `json:"kernelVersion" protobuf:"bytes,4,opt,name=kernelVersion"`
	// OS Image reported by the device from /etc/os-release (e.g. Debian GNU/Linux 7 (wheezy)).
	OSImage string `json:"osImage" protobuf:"bytes,5,opt,name=osImage"`
	// ContainerRuntime Version reported by the device through runtime remote API (e.g. docker://1.5.0).
	ContainerRuntimeVersion string `json:"containerRuntimeVersion" protobuf:"bytes,6,opt,name=containerRuntimeVersion"`
	// Arhat Version reported by the device.
	ArhatVersion string `json:"arhatVersion" protobuf:"bytes,7,opt,name=arhatVersion"`
	// The Operating System reported by the device
	OperatingSystem string `json:"operatingSystem" protobuf:"bytes,9,opt,name=operatingSystem"`
	// The Architecture reported by the device
	Architecture string `json:"architecture" protobuf:"bytes,10,opt,name=architecture"`
}

type DeviceConditionType string

// These are valid conditions of device. Currently, we don't have enough information to decide
// device condition. In the future, we will add more. The proposed set of conditions are:
// NodeReachable, NodeLive, NodeReady, NodeSchedulable, NodeRunnable.
const (
	// DeviceCondSeen means arhat connected to kubernetes for the first time
	DeviceCondSeen DeviceConditionType = "Seen"
	// DeviceCondReady means arhat is healthy and ready to accept pods.
	DeviceCondReady DeviceConditionType = "Ready"
	// DeviceCondOutOfDisk means the arhat will not accept new pods due to insufficient free disk
	// space on the device.
	DeviceCondOutOfDisk DeviceConditionType = "OutOfDisk"
	// DeviceCondMemoryPressure means the arhat is under pressure due to insufficient available memory.
	DeviceCondMemoryPressure DeviceConditionType = "MemoryPressure"
	// DeviceCondDiskPressure means the arhat is under pressure due to insufficient available disk.
	DeviceCondDiskPressure DeviceConditionType = "DiskPressure"
	// DeviceCondPIDPressure means the arhat is under pressure due to insufficient available PID.
	DeviceCondPIDPressure DeviceConditionType = "PIDPressure"
	// DeviceCondNetworkUnavailable means that network for the device is not correctly configured.
	DeviceCondNetworkUnavailable DeviceConditionType = "NetworkUnavailable"
)

// DeviceCondition contains condition information for a device.
type DeviceCondition struct {
	// Type of device condition.
	Type DeviceConditionType `json:"type" protobuf:"bytes,1,opt,name=type,casttype=NodeConditionType"`
	// Status of the condition, one of True, False, Unknown.
	Status corev1.ConditionStatus `json:"status" protobuf:"bytes,2,opt,name=status,casttype=ConditionStatus"`
	// Last time we got an update on a given condition.
	// +optional
	LastHeartbeatTime metav1.Time `json:"lastHeartbeatTime,omitempty" protobuf:"bytes,3,opt,name=lastHeartbeatTime"`
	// Last time the condition transit from one status to another.
	// +optional
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty" protobuf:"bytes,4,opt,name=lastTransitionTime"`
	// (brief) reason for the condition's last transition.
	// +optional
	Reason string `json:"reason,omitempty" protobuf:"bytes,5,opt,name=reason"`
	// Human readable message indicating details about last transition.
	// +optional
	Message string `json:"message,omitempty" protobuf:"bytes,6,opt,name=message"`
}

// DeviceConfigSource specifies a source of device configuration. Exactly one subfield (excluding metadata) must be non-nil.
type DeviceConfigSource struct {
	// For historical context, regarding the below kind, apiVersion, and configMapRef deprecation tags:
	// 1. kind/apiVersion were used by the arhat to persist this struct to disk (they had no protobuf tags)
	// 2. configMapRef and proto tag 1 were used by the API to refer to a configmap,
	//    but used a generic ObjectReference type that didn't really have the fields we needed
	// All uses/persistence of the NodeConfigSource struct prior to 1.11 were gated by alpha feature flags,
	// so there was no persisted data for these fields that needed to be migrated/handled.

	// +k8s:deprecated=kind
	// +k8s:deprecated=apiVersion
	// +k8s:deprecated=configMapRef,protobuf=1

	// ConfigMap is a reference to a Node's ConfigMap
	ConfigMap *corev1.ConfigMapNodeConfigSource `json:"configMap,omitempty" protobuf:"bytes,2,opt,name=configMap"`
}

// NodeConfigStatus describes the status of the config assigned by Node.Spec.ConfigSource.
type NodeConfigStatus struct {
	// Assigned reports the checkpointed config the device will try to use.
	// When Node.Spec.ConfigSource is updated, the device checkpoints the associated
	// config payload to local disk, along with a record indicating intended
	// config. The device refers to this record to choose its config checkpoint, and
	// reports this record in Assigned. Assigned only updates in the status after
	// the record has been checkpointed to disk. When the Kubelet is restarted,
	// it tries to make the Assigned config the Active config by loading and
	// validating the checkpointed payload identified by Assigned.
	// +optional
	Assigned *DeviceConfigSource `json:"assigned,omitempty" protobuf:"bytes,1,opt,name=assigned"`
	// Active reports the checkpointed config the device is actively using.
	// Active will represent either the current version of the Assigned config,
	// or the current LastKnownGood config, depending on whether attempting to use the
	// Assigned config results in an error.
	// +optional
	Active *DeviceConfigSource `json:"active,omitempty" protobuf:"bytes,2,opt,name=active"`
	// LastKnownGood reports the checkpointed config the device will fall back to
	// when it encounters an error attempting to use the Assigned config.
	// The Assigned config becomes the LastKnownGood config when the device determines
	// that the Assigned config is stable and correct.
	// This is currently implemented as a 10-minute soak period starting when the local
	// record of Assigned config is updated. If the Assigned config is Active at the end
	// of this period, it becomes the LastKnownGood. Note that if Spec.ConfigSource is
	// reset to nil (use local defaults), the LastKnownGood is also immediately reset to nil,
	// because the local default config is always assumed good.
	// You should not make assumptions about the device's method of determining config stability
	// and correctness, as this may change or become configurable in the future.
	// +optional
	LastKnownGood *DeviceConfigSource `json:"lastKnownGood,omitempty" protobuf:"bytes,3,opt,name=lastKnownGood"`
	// Error describes any problems reconciling the Spec.ConfigSource to the Active config.
	// Errors may occur, for example, attempting to checkpoint Spec.ConfigSource to the local Assigned
	// record, attempting to checkpoint the payload associated with Spec.ConfigSource, attempting
	// to load or validate the Assigned config, etc.
	// Errors may occur at different points while syncing config. Earlier errors (e.g. download or
	// checkpointing errors) will not result in a rollback to LastKnownGood, and may resolve across
	// Kubelet retries. Later errors (e.g. loading or validating a checkpointed config) will result in
	// a rollback to LastKnownGood. In the latter case, it is usually possible to resolve the error
	// by fixing the config assigned in Spec.ConfigSource.
	// You can find additional information for debugging by searching the error message in the Kubelet log.
	// Error is a human-readable description of the error state; machines can check whether or not Error
	// is empty, but should not rely on the stability of the Error text across Kubelet versions.
	// +optional
	Error string `json:"error,omitempty" protobuf:"bytes,4,opt,name=error"`
}

type Network struct {
	ISP       string `json:"isp" protobuf:"bytes,1,opt,name=isp"`             // Name of the Internet Service Provider
	Interface string `json:"interface" protobuf:"bytes,2,opt,name=interface"` // Name of the Interface connected to this network
	Address   string `json:"address" protobuf:"bytes,3,opt,name=address"`     // Address of the Interface
}
