// Code generated by protoc-gen-go. DO NOT EDIT.
// source: device.proto

package service

import proto "github.com/golang/protobuf/proto"
import fmt "fmt"
import math "math"

import (
	context "golang.org/x/net/context"
	grpc "google.golang.org/grpc"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion2 // please upgrade the proto package

type NodeInfo struct {
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *NodeInfo) Reset()         { *m = NodeInfo{} }
func (m *NodeInfo) String() string { return proto.CompactTextString(m) }
func (*NodeInfo) ProtoMessage()    {}
func (*NodeInfo) Descriptor() ([]byte, []int) {
	return fileDescriptor_device_c47453b837515f18, []int{0}
}
func (m *NodeInfo) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_NodeInfo.Unmarshal(m, b)
}
func (m *NodeInfo) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_NodeInfo.Marshal(b, m, deterministic)
}
func (dst *NodeInfo) XXX_Merge(src proto.Message) {
	xxx_messageInfo_NodeInfo.Merge(dst, src)
}
func (m *NodeInfo) XXX_Size() int {
	return xxx_messageInfo_NodeInfo.Size(m)
}
func (m *NodeInfo) XXX_DiscardUnknown() {
	xxx_messageInfo_NodeInfo.DiscardUnknown(m)
}

var xxx_messageInfo_NodeInfo proto.InternalMessageInfo

type NodeInfoReq struct {
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *NodeInfoReq) Reset()         { *m = NodeInfoReq{} }
func (m *NodeInfoReq) String() string { return proto.CompactTextString(m) }
func (*NodeInfoReq) ProtoMessage()    {}
func (*NodeInfoReq) Descriptor() ([]byte, []int) {
	return fileDescriptor_device_c47453b837515f18, []int{1}
}
func (m *NodeInfoReq) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_NodeInfoReq.Unmarshal(m, b)
}
func (m *NodeInfoReq) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_NodeInfoReq.Marshal(b, m, deterministic)
}
func (dst *NodeInfoReq) XXX_Merge(src proto.Message) {
	xxx_messageInfo_NodeInfoReq.Merge(dst, src)
}
func (m *NodeInfoReq) XXX_Size() int {
	return xxx_messageInfo_NodeInfoReq.Size(m)
}
func (m *NodeInfoReq) XXX_DiscardUnknown() {
	xxx_messageInfo_NodeInfoReq.DiscardUnknown(m)
}

var xxx_messageInfo_NodeInfoReq proto.InternalMessageInfo

func init() {
	proto.RegisterType((*NodeInfo)(nil), "service.NodeInfo")
	proto.RegisterType((*NodeInfoReq)(nil), "service.NodeInfoReq")
}

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ grpc.ClientConn

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion4

// DeviceClient is the client API for Device service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://godoc.org/google.golang.org/grpc#ClientConn.NewStream.
type DeviceClient interface {
	SyncDeviceInfo(ctx context.Context, opts ...grpc.CallOption) (Device_SyncDeviceInfoClient, error)
}

type deviceClient struct {
	cc *grpc.ClientConn
}

func NewDeviceClient(cc *grpc.ClientConn) DeviceClient {
	return &deviceClient{cc}
}

func (c *deviceClient) SyncDeviceInfo(ctx context.Context, opts ...grpc.CallOption) (Device_SyncDeviceInfoClient, error) {
	stream, err := c.cc.NewStream(ctx, &_Device_serviceDesc.Streams[0], "/service.Device/SyncDeviceInfo", opts...)
	if err != nil {
		return nil, err
	}
	x := &deviceSyncDeviceInfoClient{stream}
	return x, nil
}

type Device_SyncDeviceInfoClient interface {
	Send(*NodeInfo) error
	Recv() (*NodeInfoReq, error)
	grpc.ClientStream
}

type deviceSyncDeviceInfoClient struct {
	grpc.ClientStream
}

func (x *deviceSyncDeviceInfoClient) Send(m *NodeInfo) error {
	return x.ClientStream.SendMsg(m)
}

func (x *deviceSyncDeviceInfoClient) Recv() (*NodeInfoReq, error) {
	m := new(NodeInfoReq)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// DeviceServer is the server API for Device service.
type DeviceServer interface {
	SyncDeviceInfo(Device_SyncDeviceInfoServer) error
}

func RegisterDeviceServer(s *grpc.Server, srv DeviceServer) {
	s.RegisterService(&_Device_serviceDesc, srv)
}

func _Device_SyncDeviceInfo_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(DeviceServer).SyncDeviceInfo(&deviceSyncDeviceInfoServer{stream})
}

type Device_SyncDeviceInfoServer interface {
	Send(*NodeInfoReq) error
	Recv() (*NodeInfo, error)
	grpc.ServerStream
}

type deviceSyncDeviceInfoServer struct {
	grpc.ServerStream
}

func (x *deviceSyncDeviceInfoServer) Send(m *NodeInfoReq) error {
	return x.ServerStream.SendMsg(m)
}

func (x *deviceSyncDeviceInfoServer) Recv() (*NodeInfo, error) {
	m := new(NodeInfo)
	if err := x.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

var _Device_serviceDesc = grpc.ServiceDesc{
	ServiceName: "service.Device",
	HandlerType: (*DeviceServer)(nil),
	Methods:     []grpc.MethodDesc{},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "SyncDeviceInfo",
			Handler:       _Device_SyncDeviceInfo_Handler,
			ServerStreams: true,
			ClientStreams: true,
		},
	},
	Metadata: "device.proto",
}

func init() { proto.RegisterFile("device.proto", fileDescriptor_device_c47453b837515f18) }

var fileDescriptor_device_c47453b837515f18 = []byte{
	// 108 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xe2, 0xe2, 0x49, 0x49, 0x2d, 0xcb,
	0x4c, 0x4e, 0xd5, 0x2b, 0x28, 0xca, 0x2f, 0xc9, 0x17, 0x62, 0x2f, 0x4e, 0x2d, 0x02, 0x71, 0x95,
	0xb8, 0xb8, 0x38, 0xfc, 0xf2, 0x53, 0x52, 0x3d, 0xf3, 0xd2, 0xf2, 0x95, 0x78, 0xb9, 0xb8, 0x61,
	0xec, 0xa0, 0xd4, 0x42, 0x23, 0x77, 0x2e, 0x36, 0x17, 0xb0, 0x1e, 0x21, 0x5b, 0x2e, 0xbe, 0xe0,
	0xca, 0xbc, 0x64, 0x08, 0x0f, 0x24, 0x2d, 0x24, 0xa8, 0x07, 0x35, 0x40, 0x0f, 0xa6, 0x43, 0x4a,
	0x04, 0x43, 0x28, 0x28, 0xb5, 0x50, 0x83, 0xd1, 0x80, 0x31, 0x89, 0x0d, 0x6c, 0xa7, 0x31, 0x20,
	0x00, 0x00, 0xff, 0xff, 0x5d, 0x09, 0x9a, 0xc4, 0x83, 0x00, 0x00, 0x00,
}
