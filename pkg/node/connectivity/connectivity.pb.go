// Code generated by protoc-gen-gogo. DO NOT EDIT.
// source: connectivity.proto

package connectivity

import (
	context "context"
	fmt "fmt"
	_ "github.com/gogo/protobuf/gogoproto"
	proto "github.com/gogo/protobuf/proto"
	grpc "google.golang.org/grpc"
	io "io"
	math "math"
	reflect "reflect"
	strings "strings"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.GoGoProtoPackageIsVersion2 // please upgrade the proto package

type Msg struct {
	SessionId uint64 `protobuf:"varint,1,opt,name=session_id,json=sessionId,proto3" json:"session_id,omitempty"`
	Completed bool   `protobuf:"varint,2,opt,name=completed,proto3" json:"completed,omitempty"`
	// Types that are valid to be assigned to Msg:
	//	*Msg_Node
	//	*Msg_Pod
	//	*Msg_Data
	//	*Msg_Ack
	Msg isMsg_Msg `protobuf_oneof:"msg"`
}

func (m *Msg) Reset()      { *m = Msg{} }
func (*Msg) ProtoMessage() {}
func (*Msg) Descriptor() ([]byte, []int) {
	return fileDescriptor_2872c2021a21e8fe, []int{0}
}
func (m *Msg) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *Msg) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_Msg.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalTo(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *Msg) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Msg.Merge(m, src)
}
func (m *Msg) XXX_Size() int {
	return m.Size()
}
func (m *Msg) XXX_DiscardUnknown() {
	xxx_messageInfo_Msg.DiscardUnknown(m)
}

var xxx_messageInfo_Msg proto.InternalMessageInfo

type isMsg_Msg interface {
	isMsg_Msg()
	Equal(interface{}) bool
	MarshalTo([]byte) (int, error)
	Size() int
}

type Msg_Node struct {
	Node *Node `protobuf:"bytes,11,opt,name=node,proto3,oneof"`
}
type Msg_Pod struct {
	Pod *Pod `protobuf:"bytes,12,opt,name=pod,proto3,oneof"`
}
type Msg_Data struct {
	Data *Data `protobuf:"bytes,13,opt,name=data,proto3,oneof"`
}
type Msg_Ack struct {
	Ack *Ack `protobuf:"bytes,14,opt,name=ack,proto3,oneof"`
}

func (*Msg_Node) isMsg_Msg() {}
func (*Msg_Pod) isMsg_Msg()  {}
func (*Msg_Data) isMsg_Msg() {}
func (*Msg_Ack) isMsg_Msg()  {}

func (m *Msg) GetMsg() isMsg_Msg {
	if m != nil {
		return m.Msg
	}
	return nil
}

func (m *Msg) GetSessionId() uint64 {
	if m != nil {
		return m.SessionId
	}
	return 0
}

func (m *Msg) GetCompleted() bool {
	if m != nil {
		return m.Completed
	}
	return false
}

func (m *Msg) GetNode() *Node {
	if x, ok := m.GetMsg().(*Msg_Node); ok {
		return x.Node
	}
	return nil
}

func (m *Msg) GetPod() *Pod {
	if x, ok := m.GetMsg().(*Msg_Pod); ok {
		return x.Pod
	}
	return nil
}

func (m *Msg) GetData() *Data {
	if x, ok := m.GetMsg().(*Msg_Data); ok {
		return x.Data
	}
	return nil
}

func (m *Msg) GetAck() *Ack {
	if x, ok := m.GetMsg().(*Msg_Ack); ok {
		return x.Ack
	}
	return nil
}

// XXX_OneofFuncs is for the internal use of the proto package.
func (*Msg) XXX_OneofFuncs() (func(msg proto.Message, b *proto.Buffer) error, func(msg proto.Message, tag, wire int, b *proto.Buffer) (bool, error), func(msg proto.Message) (n int), []interface{}) {
	return _Msg_OneofMarshaler, _Msg_OneofUnmarshaler, _Msg_OneofSizer, []interface{}{
		(*Msg_Node)(nil),
		(*Msg_Pod)(nil),
		(*Msg_Data)(nil),
		(*Msg_Ack)(nil),
	}
}

func _Msg_OneofMarshaler(msg proto.Message, b *proto.Buffer) error {
	m := msg.(*Msg)
	// msg
	switch x := m.Msg.(type) {
	case *Msg_Node:
		_ = b.EncodeVarint(11<<3 | proto.WireBytes)
		if err := b.EncodeMessage(x.Node); err != nil {
			return err
		}
	case *Msg_Pod:
		_ = b.EncodeVarint(12<<3 | proto.WireBytes)
		if err := b.EncodeMessage(x.Pod); err != nil {
			return err
		}
	case *Msg_Data:
		_ = b.EncodeVarint(13<<3 | proto.WireBytes)
		if err := b.EncodeMessage(x.Data); err != nil {
			return err
		}
	case *Msg_Ack:
		_ = b.EncodeVarint(14<<3 | proto.WireBytes)
		if err := b.EncodeMessage(x.Ack); err != nil {
			return err
		}
	case nil:
	default:
		return fmt.Errorf("Msg.Msg has unexpected type %T", x)
	}
	return nil
}

func _Msg_OneofUnmarshaler(msg proto.Message, tag, wire int, b *proto.Buffer) (bool, error) {
	m := msg.(*Msg)
	switch tag {
	case 11: // msg.node
		if wire != proto.WireBytes {
			return true, proto.ErrInternalBadWireType
		}
		msg := new(Node)
		err := b.DecodeMessage(msg)
		m.Msg = &Msg_Node{msg}
		return true, err
	case 12: // msg.pod
		if wire != proto.WireBytes {
			return true, proto.ErrInternalBadWireType
		}
		msg := new(Pod)
		err := b.DecodeMessage(msg)
		m.Msg = &Msg_Pod{msg}
		return true, err
	case 13: // msg.data
		if wire != proto.WireBytes {
			return true, proto.ErrInternalBadWireType
		}
		msg := new(Data)
		err := b.DecodeMessage(msg)
		m.Msg = &Msg_Data{msg}
		return true, err
	case 14: // msg.ack
		if wire != proto.WireBytes {
			return true, proto.ErrInternalBadWireType
		}
		msg := new(Ack)
		err := b.DecodeMessage(msg)
		m.Msg = &Msg_Ack{msg}
		return true, err
	default:
		return false, nil
	}
}

func _Msg_OneofSizer(msg proto.Message) (n int) {
	m := msg.(*Msg)
	// msg
	switch x := m.Msg.(type) {
	case *Msg_Node:
		s := proto.Size(x.Node)
		n += 1 // tag and wire
		n += proto.SizeVarint(uint64(s))
		n += s
	case *Msg_Pod:
		s := proto.Size(x.Pod)
		n += 1 // tag and wire
		n += proto.SizeVarint(uint64(s))
		n += s
	case *Msg_Data:
		s := proto.Size(x.Data)
		n += 1 // tag and wire
		n += proto.SizeVarint(uint64(s))
		n += s
	case *Msg_Ack:
		s := proto.Size(x.Ack)
		n += 1 // tag and wire
		n += proto.SizeVarint(uint64(s))
		n += s
	case nil:
	default:
		panic(fmt.Sprintf("proto: unexpected type %T in oneof", x))
	}
	return n
}

func (*Msg) XXX_MessageName() string {
	return "connectivity.Msg"
}

type Cmd struct {
	SessionId uint64 `protobuf:"varint,1,opt,name=session_id,json=sessionId,proto3" json:"session_id,omitempty"`
	// Types that are valid to be assigned to Cmd:
	//	*Cmd_NodeCmd
	//	*Cmd_PodCmd
	Cmd isCmd_Cmd `protobuf_oneof:"cmd"`
}

func (m *Cmd) Reset()      { *m = Cmd{} }
func (*Cmd) ProtoMessage() {}
func (*Cmd) Descriptor() ([]byte, []int) {
	return fileDescriptor_2872c2021a21e8fe, []int{1}
}
func (m *Cmd) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *Cmd) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_Cmd.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalTo(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *Cmd) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Cmd.Merge(m, src)
}
func (m *Cmd) XXX_Size() int {
	return m.Size()
}
func (m *Cmd) XXX_DiscardUnknown() {
	xxx_messageInfo_Cmd.DiscardUnknown(m)
}

var xxx_messageInfo_Cmd proto.InternalMessageInfo

type isCmd_Cmd interface {
	isCmd_Cmd()
	Equal(interface{}) bool
	MarshalTo([]byte) (int, error)
	Size() int
}

type Cmd_NodeCmd struct {
	NodeCmd *NodeCmd `protobuf:"bytes,11,opt,name=node_cmd,json=nodeCmd,proto3,oneof"`
}
type Cmd_PodCmd struct {
	PodCmd *PodCmd `protobuf:"bytes,12,opt,name=pod_cmd,json=podCmd,proto3,oneof"`
}

func (*Cmd_NodeCmd) isCmd_Cmd() {}
func (*Cmd_PodCmd) isCmd_Cmd()  {}

func (m *Cmd) GetCmd() isCmd_Cmd {
	if m != nil {
		return m.Cmd
	}
	return nil
}

func (m *Cmd) GetSessionId() uint64 {
	if m != nil {
		return m.SessionId
	}
	return 0
}

func (m *Cmd) GetNodeCmd() *NodeCmd {
	if x, ok := m.GetCmd().(*Cmd_NodeCmd); ok {
		return x.NodeCmd
	}
	return nil
}

func (m *Cmd) GetPodCmd() *PodCmd {
	if x, ok := m.GetCmd().(*Cmd_PodCmd); ok {
		return x.PodCmd
	}
	return nil
}

// XXX_OneofFuncs is for the internal use of the proto package.
func (*Cmd) XXX_OneofFuncs() (func(msg proto.Message, b *proto.Buffer) error, func(msg proto.Message, tag, wire int, b *proto.Buffer) (bool, error), func(msg proto.Message) (n int), []interface{}) {
	return _Cmd_OneofMarshaler, _Cmd_OneofUnmarshaler, _Cmd_OneofSizer, []interface{}{
		(*Cmd_NodeCmd)(nil),
		(*Cmd_PodCmd)(nil),
	}
}

func _Cmd_OneofMarshaler(msg proto.Message, b *proto.Buffer) error {
	m := msg.(*Cmd)
	// cmd
	switch x := m.Cmd.(type) {
	case *Cmd_NodeCmd:
		_ = b.EncodeVarint(11<<3 | proto.WireBytes)
		if err := b.EncodeMessage(x.NodeCmd); err != nil {
			return err
		}
	case *Cmd_PodCmd:
		_ = b.EncodeVarint(12<<3 | proto.WireBytes)
		if err := b.EncodeMessage(x.PodCmd); err != nil {
			return err
		}
	case nil:
	default:
		return fmt.Errorf("Cmd.Cmd has unexpected type %T", x)
	}
	return nil
}

func _Cmd_OneofUnmarshaler(msg proto.Message, tag, wire int, b *proto.Buffer) (bool, error) {
	m := msg.(*Cmd)
	switch tag {
	case 11: // cmd.node_cmd
		if wire != proto.WireBytes {
			return true, proto.ErrInternalBadWireType
		}
		msg := new(NodeCmd)
		err := b.DecodeMessage(msg)
		m.Cmd = &Cmd_NodeCmd{msg}
		return true, err
	case 12: // cmd.pod_cmd
		if wire != proto.WireBytes {
			return true, proto.ErrInternalBadWireType
		}
		msg := new(PodCmd)
		err := b.DecodeMessage(msg)
		m.Cmd = &Cmd_PodCmd{msg}
		return true, err
	default:
		return false, nil
	}
}

func _Cmd_OneofSizer(msg proto.Message) (n int) {
	m := msg.(*Cmd)
	// cmd
	switch x := m.Cmd.(type) {
	case *Cmd_NodeCmd:
		s := proto.Size(x.NodeCmd)
		n += 1 // tag and wire
		n += proto.SizeVarint(uint64(s))
		n += s
	case *Cmd_PodCmd:
		s := proto.Size(x.PodCmd)
		n += 1 // tag and wire
		n += proto.SizeVarint(uint64(s))
		n += s
	case nil:
	default:
		panic(fmt.Sprintf("proto: unexpected type %T in oneof", x))
	}
	return n
}

func (*Cmd) XXX_MessageName() string {
	return "connectivity.Cmd"
}
func init() {
	proto.RegisterType((*Msg)(nil), "connectivity.Msg")
	proto.RegisterType((*Cmd)(nil), "connectivity.Cmd")
}

func init() { proto.RegisterFile("connectivity.proto", fileDescriptor_2872c2021a21e8fe) }

var fileDescriptor_2872c2021a21e8fe = []byte{
	// 391 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x84, 0x92, 0xcd, 0x8a, 0x1a, 0x41,
	0x14, 0x85, 0xeb, 0xa6, 0x8d, 0x3f, 0xa5, 0x09, 0x49, 0x91, 0x40, 0x23, 0x49, 0x21, 0x42, 0xa0,
	0x37, 0x51, 0x31, 0x9b, 0x2c, 0x13, 0x3b, 0x0b, 0xb3, 0x30, 0x84, 0xce, 0x03, 0x48, 0x5b, 0xd5,
	0xe9, 0x34, 0x5a, 0x5d, 0x4d, 0xba, 0x1c, 0x70, 0x37, 0x8f, 0xe0, 0x63, 0xcc, 0xa3, 0xb8, 0x74,
	0xe9, 0x72, 0xec, 0xde, 0x0c, 0xcc, 0xc6, 0x47, 0x18, 0xaa, 0xda, 0x61, 0xfc, 0x83, 0xd9, 0xdd,
	0x7b, 0xea, 0xdc, 0xc3, 0xf9, 0xa0, 0x30, 0x61, 0x32, 0x8e, 0x03, 0xa6, 0xa2, 0xab, 0x48, 0x2d,
	0x3a, 0xc9, 0x7f, 0xa9, 0x24, 0x69, 0x1c, 0x6a, 0xcd, 0xcf, 0x61, 0xa4, 0xfe, 0xcd, 0x27, 0x1d,
	0x26, 0x45, 0x37, 0x94, 0xa1, 0xec, 0x1a, 0xd3, 0x64, 0xfe, 0xd7, 0x6c, 0x66, 0x31, 0x53, 0x71,
	0xdc, 0x7c, 0xc3, 0x04, 0x1f, 0xf3, 0x40, 0xf9, 0xd1, 0xec, 0x51, 0x11, 0x69, 0x78, 0xa4, 0xb4,
	0xef, 0x01, 0x5b, 0xa3, 0x34, 0x24, 0x1f, 0x31, 0x4e, 0x83, 0x34, 0x8d, 0x64, 0x3c, 0x8e, 0xb8,
	0x0d, 0x2d, 0x70, 0x4a, 0x5e, 0x6d, 0xaf, 0xfc, 0xe4, 0xe4, 0x03, 0xae, 0x31, 0x29, 0x92, 0x59,
	0xa0, 0x02, 0x6e, 0xbf, 0x68, 0x81, 0x53, 0xf5, 0x9e, 0x04, 0xe2, 0xe0, 0x52, 0x2c, 0x79, 0x60,
	0xd7, 0x5b, 0xe0, 0xd4, 0xfb, 0xa4, 0x73, 0x04, 0xf2, 0x4b, 0xf2, 0x60, 0x88, 0x3c, 0xe3, 0x20,
	0x9f, 0xb0, 0x95, 0x48, 0x6e, 0x37, 0x8c, 0xf1, 0xed, 0xb1, 0xf1, 0xb7, 0xe4, 0x43, 0xe4, 0xe9,
	0x77, 0x1d, 0xc8, 0x7d, 0xe5, 0xdb, 0xaf, 0x2e, 0x05, 0xfe, 0xf0, 0x95, 0xaf, 0x03, 0xb5, 0x43,
	0x07, 0xfa, 0x6c, 0x6a, 0xbf, 0xbe, 0x14, 0xf8, 0x9d, 0x4d, 0x75, 0xa0, 0xcf, 0xa6, 0x83, 0x97,
	0xd8, 0x12, 0x69, 0xd8, 0x5e, 0x02, 0xb6, 0x5c, 0xc1, 0x9f, 0xa3, 0xed, 0xe3, 0xaa, 0x6e, 0x3b,
	0x66, 0x82, 0xef, 0x99, 0xde, 0x9f, 0x33, 0xb9, 0x42, 0xd7, 0xad, 0xc4, 0xc5, 0x48, 0xba, 0xb8,
	0x92, 0x48, 0x6e, 0x4e, 0x0a, 0xba, 0x77, 0x67, 0x74, 0xc5, 0x45, 0x39, 0x31, 0x93, 0xae, 0xc4,
	0x04, 0xef, 0x7f, 0xc3, 0x0d, 0xf7, 0xc0, 0x47, 0x7a, 0xb8, 0xf4, 0x67, 0x11, 0x33, 0x72, 0xc2,
	0x32, 0x4a, 0xc3, 0xe6, 0x89, 0xe4, 0x0a, 0xee, 0x40, 0x0f, 0x06, 0x5f, 0xd7, 0x5b, 0x8a, 0x36,
	0x5b, 0x8a, 0x76, 0x5b, 0x0a, 0xd7, 0x19, 0x85, 0x9b, 0x8c, 0xc2, 0x2a, 0xa3, 0xb0, 0xce, 0x28,
	0xdc, 0x66, 0x14, 0xee, 0x32, 0x8a, 0x76, 0x19, 0x85, 0x65, 0x4e, 0xd1, 0x2a, 0xa7, 0xb0, 0xce,
	0x29, 0xda, 0xe4, 0x14, 0x4d, 0xca, 0xe6, 0x0f, 0x7c, 0x79, 0x08, 0x00, 0x00, 0xff, 0xff, 0xeb,
	0xbd, 0xa1, 0xc6, 0x7a, 0x02, 0x00, 0x00,
}

func (this *Msg) Equal(that interface{}) bool {
	if that == nil {
		return this == nil
	}

	that1, ok := that.(*Msg)
	if !ok {
		that2, ok := that.(Msg)
		if ok {
			that1 = &that2
		} else {
			return false
		}
	}
	if that1 == nil {
		return this == nil
	} else if this == nil {
		return false
	}
	if this.SessionId != that1.SessionId {
		return false
	}
	if this.Completed != that1.Completed {
		return false
	}
	if that1.Msg == nil {
		if this.Msg != nil {
			return false
		}
	} else if this.Msg == nil {
		return false
	} else if !this.Msg.Equal(that1.Msg) {
		return false
	}
	return true
}
func (this *Msg_Node) Equal(that interface{}) bool {
	if that == nil {
		return this == nil
	}

	that1, ok := that.(*Msg_Node)
	if !ok {
		that2, ok := that.(Msg_Node)
		if ok {
			that1 = &that2
		} else {
			return false
		}
	}
	if that1 == nil {
		return this == nil
	} else if this == nil {
		return false
	}
	if !this.Node.Equal(that1.Node) {
		return false
	}
	return true
}
func (this *Msg_Pod) Equal(that interface{}) bool {
	if that == nil {
		return this == nil
	}

	that1, ok := that.(*Msg_Pod)
	if !ok {
		that2, ok := that.(Msg_Pod)
		if ok {
			that1 = &that2
		} else {
			return false
		}
	}
	if that1 == nil {
		return this == nil
	} else if this == nil {
		return false
	}
	if !this.Pod.Equal(that1.Pod) {
		return false
	}
	return true
}
func (this *Msg_Data) Equal(that interface{}) bool {
	if that == nil {
		return this == nil
	}

	that1, ok := that.(*Msg_Data)
	if !ok {
		that2, ok := that.(Msg_Data)
		if ok {
			that1 = &that2
		} else {
			return false
		}
	}
	if that1 == nil {
		return this == nil
	} else if this == nil {
		return false
	}
	if !this.Data.Equal(that1.Data) {
		return false
	}
	return true
}
func (this *Msg_Ack) Equal(that interface{}) bool {
	if that == nil {
		return this == nil
	}

	that1, ok := that.(*Msg_Ack)
	if !ok {
		that2, ok := that.(Msg_Ack)
		if ok {
			that1 = &that2
		} else {
			return false
		}
	}
	if that1 == nil {
		return this == nil
	} else if this == nil {
		return false
	}
	if !this.Ack.Equal(that1.Ack) {
		return false
	}
	return true
}
func (this *Cmd) Equal(that interface{}) bool {
	if that == nil {
		return this == nil
	}

	that1, ok := that.(*Cmd)
	if !ok {
		that2, ok := that.(Cmd)
		if ok {
			that1 = &that2
		} else {
			return false
		}
	}
	if that1 == nil {
		return this == nil
	} else if this == nil {
		return false
	}
	if this.SessionId != that1.SessionId {
		return false
	}
	if that1.Cmd == nil {
		if this.Cmd != nil {
			return false
		}
	} else if this.Cmd == nil {
		return false
	} else if !this.Cmd.Equal(that1.Cmd) {
		return false
	}
	return true
}
func (this *Cmd_NodeCmd) Equal(that interface{}) bool {
	if that == nil {
		return this == nil
	}

	that1, ok := that.(*Cmd_NodeCmd)
	if !ok {
		that2, ok := that.(Cmd_NodeCmd)
		if ok {
			that1 = &that2
		} else {
			return false
		}
	}
	if that1 == nil {
		return this == nil
	} else if this == nil {
		return false
	}
	if !this.NodeCmd.Equal(that1.NodeCmd) {
		return false
	}
	return true
}
func (this *Cmd_PodCmd) Equal(that interface{}) bool {
	if that == nil {
		return this == nil
	}

	that1, ok := that.(*Cmd_PodCmd)
	if !ok {
		that2, ok := that.(Cmd_PodCmd)
		if ok {
			that1 = &that2
		} else {
			return false
		}
	}
	if that1 == nil {
		return this == nil
	} else if this == nil {
		return false
	}
	if !this.PodCmd.Equal(that1.PodCmd) {
		return false
	}
	return true
}
func (this *Msg) GoString() string {
	if this == nil {
		return "nil"
	}
	s := make([]string, 0, 10)
	s = append(s, "&connectivity.Msg{")
	s = append(s, "SessionId: "+fmt.Sprintf("%#v", this.SessionId)+",\n")
	s = append(s, "Completed: "+fmt.Sprintf("%#v", this.Completed)+",\n")
	if this.Msg != nil {
		s = append(s, "Msg: "+fmt.Sprintf("%#v", this.Msg)+",\n")
	}
	s = append(s, "}")
	return strings.Join(s, "")
}
func (this *Msg_Node) GoString() string {
	if this == nil {
		return "nil"
	}
	s := strings.Join([]string{`&connectivity.Msg_Node{` +
		`Node:` + fmt.Sprintf("%#v", this.Node) + `}`}, ", ")
	return s
}
func (this *Msg_Pod) GoString() string {
	if this == nil {
		return "nil"
	}
	s := strings.Join([]string{`&connectivity.Msg_Pod{` +
		`Pod:` + fmt.Sprintf("%#v", this.Pod) + `}`}, ", ")
	return s
}
func (this *Msg_Data) GoString() string {
	if this == nil {
		return "nil"
	}
	s := strings.Join([]string{`&connectivity.Msg_Data{` +
		`Data:` + fmt.Sprintf("%#v", this.Data) + `}`}, ", ")
	return s
}
func (this *Msg_Ack) GoString() string {
	if this == nil {
		return "nil"
	}
	s := strings.Join([]string{`&connectivity.Msg_Ack{` +
		`Ack:` + fmt.Sprintf("%#v", this.Ack) + `}`}, ", ")
	return s
}
func (this *Cmd) GoString() string {
	if this == nil {
		return "nil"
	}
	s := make([]string, 0, 7)
	s = append(s, "&connectivity.Cmd{")
	s = append(s, "SessionId: "+fmt.Sprintf("%#v", this.SessionId)+",\n")
	if this.Cmd != nil {
		s = append(s, "Cmd: "+fmt.Sprintf("%#v", this.Cmd)+",\n")
	}
	s = append(s, "}")
	return strings.Join(s, "")
}
func (this *Cmd_NodeCmd) GoString() string {
	if this == nil {
		return "nil"
	}
	s := strings.Join([]string{`&connectivity.Cmd_NodeCmd{` +
		`NodeCmd:` + fmt.Sprintf("%#v", this.NodeCmd) + `}`}, ", ")
	return s
}
func (this *Cmd_PodCmd) GoString() string {
	if this == nil {
		return "nil"
	}
	s := strings.Join([]string{`&connectivity.Cmd_PodCmd{` +
		`PodCmd:` + fmt.Sprintf("%#v", this.PodCmd) + `}`}, ", ")
	return s
}
func valueToGoStringConnectivity(v interface{}, typ string) string {
	rv := reflect.ValueOf(v)
	if rv.IsNil() {
		return "nil"
	}
	pv := reflect.Indirect(rv).Interface()
	return fmt.Sprintf("func(v %v) *%v { return &v } ( %#v )", typ, typ, pv)
}

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ grpc.ClientConn

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion4

// ConnectivityClient is the client API for Connectivity service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://godoc.org/google.golang.org/grpc#ClientConn.NewStream.
type ConnectivityClient interface {
	Sync(ctx context.Context, opts ...grpc.CallOption) (Connectivity_SyncClient, error)
}

type connectivityClient struct {
	cc *grpc.ClientConn
}

func NewConnectivityClient(cc *grpc.ClientConn) ConnectivityClient {
	return &connectivityClient{cc}
}

func (c *connectivityClient) Sync(ctx context.Context, opts ...grpc.CallOption) (Connectivity_SyncClient, error) {
	stream, err := c.cc.NewStream(ctx, &_Connectivity_serviceDesc.Streams[0], "/connectivity.Connectivity/Sync", opts...)
	if err != nil {
		return nil, err
	}
	x := &connectivitySyncClient{stream}
	return x, nil
}

type Connectivity_SyncClient interface {
	Send(*Msg) error
	Recv() (*Cmd, error)
	grpc.ClientStream
}

type connectivitySyncClient struct {
	grpc.ClientStream
}

func (x *connectivitySyncClient) Send(m *Msg) error {
	return x.ClientStream.SendMsg(m)
}

func (x *connectivitySyncClient) Recv() (*Cmd, error) {
	m := new(Cmd)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// ConnectivityServer is the server API for Connectivity service.
type ConnectivityServer interface {
	Sync(Connectivity_SyncServer) error
}

func RegisterConnectivityServer(s *grpc.Server, srv ConnectivityServer) {
	s.RegisterService(&_Connectivity_serviceDesc, srv)
}

func _Connectivity_Sync_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(ConnectivityServer).Sync(&connectivitySyncServer{stream})
}

type Connectivity_SyncServer interface {
	Send(*Cmd) error
	Recv() (*Msg, error)
	grpc.ServerStream
}

type connectivitySyncServer struct {
	grpc.ServerStream
}

func (x *connectivitySyncServer) Send(m *Cmd) error {
	return x.ServerStream.SendMsg(m)
}

func (x *connectivitySyncServer) Recv() (*Msg, error) {
	m := new(Msg)
	if err := x.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

var _Connectivity_serviceDesc = grpc.ServiceDesc{
	ServiceName: "connectivity.Connectivity",
	HandlerType: (*ConnectivityServer)(nil),
	Methods:     []grpc.MethodDesc{},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "Sync",
			Handler:       _Connectivity_Sync_Handler,
			ServerStreams: true,
			ClientStreams: true,
		},
	},
	Metadata: "connectivity.proto",
}

func (m *Msg) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalTo(dAtA)
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *Msg) MarshalTo(dAtA []byte) (int, error) {
	var i int
	_ = i
	var l int
	_ = l
	if m.SessionId != 0 {
		dAtA[i] = 0x8
		i++
		i = encodeVarintConnectivity(dAtA, i, uint64(m.SessionId))
	}
	if m.Completed {
		dAtA[i] = 0x10
		i++
		if m.Completed {
			dAtA[i] = 1
		} else {
			dAtA[i] = 0
		}
		i++
	}
	if m.Msg != nil {
		nn1, err := m.Msg.MarshalTo(dAtA[i:])
		if err != nil {
			return 0, err
		}
		i += nn1
	}
	return i, nil
}

func (m *Msg_Node) MarshalTo(dAtA []byte) (int, error) {
	i := 0
	if m.Node != nil {
		dAtA[i] = 0x5a
		i++
		i = encodeVarintConnectivity(dAtA, i, uint64(m.Node.Size()))
		n2, err := m.Node.MarshalTo(dAtA[i:])
		if err != nil {
			return 0, err
		}
		i += n2
	}
	return i, nil
}
func (m *Msg_Pod) MarshalTo(dAtA []byte) (int, error) {
	i := 0
	if m.Pod != nil {
		dAtA[i] = 0x62
		i++
		i = encodeVarintConnectivity(dAtA, i, uint64(m.Pod.Size()))
		n3, err := m.Pod.MarshalTo(dAtA[i:])
		if err != nil {
			return 0, err
		}
		i += n3
	}
	return i, nil
}
func (m *Msg_Data) MarshalTo(dAtA []byte) (int, error) {
	i := 0
	if m.Data != nil {
		dAtA[i] = 0x6a
		i++
		i = encodeVarintConnectivity(dAtA, i, uint64(m.Data.Size()))
		n4, err := m.Data.MarshalTo(dAtA[i:])
		if err != nil {
			return 0, err
		}
		i += n4
	}
	return i, nil
}
func (m *Msg_Ack) MarshalTo(dAtA []byte) (int, error) {
	i := 0
	if m.Ack != nil {
		dAtA[i] = 0x72
		i++
		i = encodeVarintConnectivity(dAtA, i, uint64(m.Ack.Size()))
		n5, err := m.Ack.MarshalTo(dAtA[i:])
		if err != nil {
			return 0, err
		}
		i += n5
	}
	return i, nil
}
func (m *Cmd) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalTo(dAtA)
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *Cmd) MarshalTo(dAtA []byte) (int, error) {
	var i int
	_ = i
	var l int
	_ = l
	if m.SessionId != 0 {
		dAtA[i] = 0x8
		i++
		i = encodeVarintConnectivity(dAtA, i, uint64(m.SessionId))
	}
	if m.Cmd != nil {
		nn6, err := m.Cmd.MarshalTo(dAtA[i:])
		if err != nil {
			return 0, err
		}
		i += nn6
	}
	return i, nil
}

func (m *Cmd_NodeCmd) MarshalTo(dAtA []byte) (int, error) {
	i := 0
	if m.NodeCmd != nil {
		dAtA[i] = 0x5a
		i++
		i = encodeVarintConnectivity(dAtA, i, uint64(m.NodeCmd.Size()))
		n7, err := m.NodeCmd.MarshalTo(dAtA[i:])
		if err != nil {
			return 0, err
		}
		i += n7
	}
	return i, nil
}
func (m *Cmd_PodCmd) MarshalTo(dAtA []byte) (int, error) {
	i := 0
	if m.PodCmd != nil {
		dAtA[i] = 0x62
		i++
		i = encodeVarintConnectivity(dAtA, i, uint64(m.PodCmd.Size()))
		n8, err := m.PodCmd.MarshalTo(dAtA[i:])
		if err != nil {
			return 0, err
		}
		i += n8
	}
	return i, nil
}
func encodeVarintConnectivity(dAtA []byte, offset int, v uint64) int {
	for v >= 1<<7 {
		dAtA[offset] = uint8(v&0x7f | 0x80)
		v >>= 7
		offset++
	}
	dAtA[offset] = uint8(v)
	return offset + 1
}
func (m *Msg) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.SessionId != 0 {
		n += 1 + sovConnectivity(uint64(m.SessionId))
	}
	if m.Completed {
		n += 2
	}
	if m.Msg != nil {
		n += m.Msg.Size()
	}
	return n
}

func (m *Msg_Node) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.Node != nil {
		l = m.Node.Size()
		n += 1 + l + sovConnectivity(uint64(l))
	}
	return n
}
func (m *Msg_Pod) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.Pod != nil {
		l = m.Pod.Size()
		n += 1 + l + sovConnectivity(uint64(l))
	}
	return n
}
func (m *Msg_Data) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.Data != nil {
		l = m.Data.Size()
		n += 1 + l + sovConnectivity(uint64(l))
	}
	return n
}
func (m *Msg_Ack) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.Ack != nil {
		l = m.Ack.Size()
		n += 1 + l + sovConnectivity(uint64(l))
	}
	return n
}
func (m *Cmd) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.SessionId != 0 {
		n += 1 + sovConnectivity(uint64(m.SessionId))
	}
	if m.Cmd != nil {
		n += m.Cmd.Size()
	}
	return n
}

func (m *Cmd_NodeCmd) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.NodeCmd != nil {
		l = m.NodeCmd.Size()
		n += 1 + l + sovConnectivity(uint64(l))
	}
	return n
}
func (m *Cmd_PodCmd) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.PodCmd != nil {
		l = m.PodCmd.Size()
		n += 1 + l + sovConnectivity(uint64(l))
	}
	return n
}

func sovConnectivity(x uint64) (n int) {
	for {
		n++
		x >>= 7
		if x == 0 {
			break
		}
	}
	return n
}
func sozConnectivity(x uint64) (n int) {
	return sovConnectivity(uint64((x << 1) ^ uint64((int64(x) >> 63))))
}
func (this *Msg) String() string {
	if this == nil {
		return "nil"
	}
	s := strings.Join([]string{`&Msg{`,
		`SessionId:` + fmt.Sprintf("%v", this.SessionId) + `,`,
		`Completed:` + fmt.Sprintf("%v", this.Completed) + `,`,
		`Msg:` + fmt.Sprintf("%v", this.Msg) + `,`,
		`}`,
	}, "")
	return s
}
func (this *Msg_Node) String() string {
	if this == nil {
		return "nil"
	}
	s := strings.Join([]string{`&Msg_Node{`,
		`Node:` + strings.Replace(fmt.Sprintf("%v", this.Node), "Node", "Node", 1) + `,`,
		`}`,
	}, "")
	return s
}
func (this *Msg_Pod) String() string {
	if this == nil {
		return "nil"
	}
	s := strings.Join([]string{`&Msg_Pod{`,
		`Pod:` + strings.Replace(fmt.Sprintf("%v", this.Pod), "Pod", "Pod", 1) + `,`,
		`}`,
	}, "")
	return s
}
func (this *Msg_Data) String() string {
	if this == nil {
		return "nil"
	}
	s := strings.Join([]string{`&Msg_Data{`,
		`Data:` + strings.Replace(fmt.Sprintf("%v", this.Data), "Data", "Data", 1) + `,`,
		`}`,
	}, "")
	return s
}
func (this *Msg_Ack) String() string {
	if this == nil {
		return "nil"
	}
	s := strings.Join([]string{`&Msg_Ack{`,
		`Ack:` + strings.Replace(fmt.Sprintf("%v", this.Ack), "Ack", "Ack", 1) + `,`,
		`}`,
	}, "")
	return s
}
func (this *Cmd) String() string {
	if this == nil {
		return "nil"
	}
	s := strings.Join([]string{`&Cmd{`,
		`SessionId:` + fmt.Sprintf("%v", this.SessionId) + `,`,
		`Cmd:` + fmt.Sprintf("%v", this.Cmd) + `,`,
		`}`,
	}, "")
	return s
}
func (this *Cmd_NodeCmd) String() string {
	if this == nil {
		return "nil"
	}
	s := strings.Join([]string{`&Cmd_NodeCmd{`,
		`NodeCmd:` + strings.Replace(fmt.Sprintf("%v", this.NodeCmd), "NodeCmd", "NodeCmd", 1) + `,`,
		`}`,
	}, "")
	return s
}
func (this *Cmd_PodCmd) String() string {
	if this == nil {
		return "nil"
	}
	s := strings.Join([]string{`&Cmd_PodCmd{`,
		`PodCmd:` + strings.Replace(fmt.Sprintf("%v", this.PodCmd), "PodCmd", "PodCmd", 1) + `,`,
		`}`,
	}, "")
	return s
}
func valueToStringConnectivity(v interface{}) string {
	rv := reflect.ValueOf(v)
	if rv.IsNil() {
		return "nil"
	}
	pv := reflect.Indirect(rv).Interface()
	return fmt.Sprintf("*%v", pv)
}
func (m *Msg) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowConnectivity
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: Msg: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: Msg: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field SessionId", wireType)
			}
			m.SessionId = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowConnectivity
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.SessionId |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 2:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field Completed", wireType)
			}
			var v int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowConnectivity
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				v |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			m.Completed = bool(v != 0)
		case 11:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Node", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowConnectivity
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthConnectivity
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthConnectivity
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			v := &Node{}
			if err := v.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			m.Msg = &Msg_Node{v}
			iNdEx = postIndex
		case 12:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Pod", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowConnectivity
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthConnectivity
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthConnectivity
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			v := &Pod{}
			if err := v.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			m.Msg = &Msg_Pod{v}
			iNdEx = postIndex
		case 13:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Data", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowConnectivity
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthConnectivity
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthConnectivity
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			v := &Data{}
			if err := v.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			m.Msg = &Msg_Data{v}
			iNdEx = postIndex
		case 14:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Ack", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowConnectivity
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthConnectivity
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthConnectivity
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			v := &Ack{}
			if err := v.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			m.Msg = &Msg_Ack{v}
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipConnectivity(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
				return ErrInvalidLengthConnectivity
			}
			if (iNdEx + skippy) < 0 {
				return ErrInvalidLengthConnectivity
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func (m *Cmd) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowConnectivity
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: Cmd: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: Cmd: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field SessionId", wireType)
			}
			m.SessionId = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowConnectivity
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.SessionId |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 11:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field NodeCmd", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowConnectivity
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthConnectivity
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthConnectivity
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			v := &NodeCmd{}
			if err := v.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			m.Cmd = &Cmd_NodeCmd{v}
			iNdEx = postIndex
		case 12:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field PodCmd", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowConnectivity
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthConnectivity
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthConnectivity
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			v := &PodCmd{}
			if err := v.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			m.Cmd = &Cmd_PodCmd{v}
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipConnectivity(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
				return ErrInvalidLengthConnectivity
			}
			if (iNdEx + skippy) < 0 {
				return ErrInvalidLengthConnectivity
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func skipConnectivity(dAtA []byte) (n int, err error) {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return 0, ErrIntOverflowConnectivity
			}
			if iNdEx >= l {
				return 0, io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= (uint64(b) & 0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		wireType := int(wire & 0x7)
		switch wireType {
		case 0:
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return 0, ErrIntOverflowConnectivity
				}
				if iNdEx >= l {
					return 0, io.ErrUnexpectedEOF
				}
				iNdEx++
				if dAtA[iNdEx-1] < 0x80 {
					break
				}
			}
			return iNdEx, nil
		case 1:
			iNdEx += 8
			return iNdEx, nil
		case 2:
			var length int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return 0, ErrIntOverflowConnectivity
				}
				if iNdEx >= l {
					return 0, io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				length |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if length < 0 {
				return 0, ErrInvalidLengthConnectivity
			}
			iNdEx += length
			if iNdEx < 0 {
				return 0, ErrInvalidLengthConnectivity
			}
			return iNdEx, nil
		case 3:
			for {
				var innerWire uint64
				var start int = iNdEx
				for shift := uint(0); ; shift += 7 {
					if shift >= 64 {
						return 0, ErrIntOverflowConnectivity
					}
					if iNdEx >= l {
						return 0, io.ErrUnexpectedEOF
					}
					b := dAtA[iNdEx]
					iNdEx++
					innerWire |= (uint64(b) & 0x7F) << shift
					if b < 0x80 {
						break
					}
				}
				innerWireType := int(innerWire & 0x7)
				if innerWireType == 4 {
					break
				}
				next, err := skipConnectivity(dAtA[start:])
				if err != nil {
					return 0, err
				}
				iNdEx = start + next
				if iNdEx < 0 {
					return 0, ErrInvalidLengthConnectivity
				}
			}
			return iNdEx, nil
		case 4:
			return iNdEx, nil
		case 5:
			iNdEx += 4
			return iNdEx, nil
		default:
			return 0, fmt.Errorf("proto: illegal wireType %d", wireType)
		}
	}
	panic("unreachable")
}

var (
	ErrInvalidLengthConnectivity = fmt.Errorf("proto: negative length found during unmarshaling")
	ErrIntOverflowConnectivity   = fmt.Errorf("proto: integer overflow")
)
