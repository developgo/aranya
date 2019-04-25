// +build !ignore_autogenerated

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

// Code generated by deepcopy-gen. DO NOT EDIT.

package v1alpha1

import (
	v1 "k8s.io/api/core/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CertInfo) DeepCopyInto(out *CertInfo) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CertInfo.
func (in *CertInfo) DeepCopy() *CertInfo {
	if in == nil {
		return nil
	}
	out := new(CertInfo)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Connectivity) DeepCopyInto(out *Connectivity) {
	*out = *in
	in.MQTTConfig.DeepCopyInto(&out.MQTTConfig)
	in.GRPCConfig.DeepCopyInto(&out.GRPCConfig)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Connectivity.
func (in *Connectivity) DeepCopy() *Connectivity {
	if in == nil {
		return nil
	}
	out := new(Connectivity)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *EdgeDevice) DeepCopyInto(out *EdgeDevice) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	out.Status = in.Status
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new EdgeDevice.
func (in *EdgeDevice) DeepCopy() *EdgeDevice {
	if in == nil {
		return nil
	}
	out := new(EdgeDevice)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *EdgeDevice) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *EdgeDeviceList) DeepCopyInto(out *EdgeDeviceList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]EdgeDevice, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new EdgeDeviceList.
func (in *EdgeDeviceList) DeepCopy() *EdgeDeviceList {
	if in == nil {
		return nil
	}
	out := new(EdgeDeviceList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *EdgeDeviceList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *EdgeDeviceSpec) DeepCopyInto(out *EdgeDeviceSpec) {
	*out = *in
	out.CertInfo = in.CertInfo
	in.Connectivity.DeepCopyInto(&out.Connectivity)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new EdgeDeviceSpec.
func (in *EdgeDeviceSpec) DeepCopy() *EdgeDeviceSpec {
	if in == nil {
		return nil
	}
	out := new(EdgeDeviceSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *EdgeDeviceStatus) DeepCopyInto(out *EdgeDeviceStatus) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new EdgeDeviceStatus.
func (in *EdgeDeviceStatus) DeepCopy() *EdgeDeviceStatus {
	if in == nil {
		return nil
	}
	out := new(EdgeDeviceStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *GRPCConfig) DeepCopyInto(out *GRPCConfig) {
	*out = *in
	if in.TLSSecretRef != nil {
		in, out := &in.TLSSecretRef, &out.TLSSecretRef
		*out = new(v1.SecretReference)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new GRPCConfig.
func (in *GRPCConfig) DeepCopy() *GRPCConfig {
	if in == nil {
		return nil
	}
	out := new(GRPCConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MQTTConfig) DeepCopyInto(out *MQTTConfig) {
	*out = *in
	if in.TLSSecretRef != nil {
		in, out := &in.TLSSecretRef, &out.TLSSecretRef
		*out = new(v1.SecretReference)
		**out = **in
	}
	if in.ConnectPacket != nil {
		in, out := &in.ConnectPacket, &out.ConnectPacket
		*out = new(MQTTConnectPacket)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MQTTConfig.
func (in *MQTTConfig) DeepCopy() *MQTTConfig {
	if in == nil {
		return nil
	}
	out := new(MQTTConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MQTTConnectPacket) DeepCopyInto(out *MQTTConnectPacket) {
	*out = *in
	out.Password = in.Password
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MQTTConnectPacket.
func (in *MQTTConnectPacket) DeepCopy() *MQTTConnectPacket {
	if in == nil {
		return nil
	}
	out := new(MQTTConnectPacket)
	in.DeepCopyInto(out)
	return out
}
