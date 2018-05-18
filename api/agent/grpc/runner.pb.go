// Code generated by protoc-gen-go. DO NOT EDIT.
// source: runner.proto

/*
Package runner is a generated protocol buffer package.

It is generated from these files:
	runner.proto

It has these top-level messages:
	TryCall
	DataFrame
	HttpHeader
	HttpRespMeta
	CallResultStart
	CallFinished
	ClientMsg
	RunnerMsg
	RunnerStatus
*/
package runner

import proto "github.com/golang/protobuf/proto"
import fmt "fmt"
import math "math"
import google_protobuf "github.com/golang/protobuf/ptypes/empty"

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

// Request to allocate a slot for a call
type TryCall struct {
	ModelsCallJson string `protobuf:"bytes,1,opt,name=models_call_json,json=modelsCallJson" json:"models_call_json,omitempty"`
	SlotHashId     string `protobuf:"bytes,2,opt,name=slot_hash_id,json=slotHashId" json:"slot_hash_id,omitempty"`
}

func (m *TryCall) Reset()                    { *m = TryCall{} }
func (m *TryCall) String() string            { return proto.CompactTextString(m) }
func (*TryCall) ProtoMessage()               {}
func (*TryCall) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{0} }

func (m *TryCall) GetModelsCallJson() string {
	if m != nil {
		return m.ModelsCallJson
	}
	return ""
}

func (m *TryCall) GetSlotHashId() string {
	if m != nil {
		return m.SlotHashId
	}
	return ""
}

// Data sent C2S and S2C - as soon as the runner sees the first of these it
// will start running. If empty content, there must be one of these with eof.
// The runner will send these for the body of the response, AFTER it has sent
// a CallEnding message.
type DataFrame struct {
	Data []byte `protobuf:"bytes,1,opt,name=data,proto3" json:"data,omitempty"`
	Eof  bool   `protobuf:"varint,2,opt,name=eof" json:"eof,omitempty"`
}

func (m *DataFrame) Reset()                    { *m = DataFrame{} }
func (m *DataFrame) String() string            { return proto.CompactTextString(m) }
func (*DataFrame) ProtoMessage()               {}
func (*DataFrame) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{1} }

func (m *DataFrame) GetData() []byte {
	if m != nil {
		return m.Data
	}
	return nil
}

func (m *DataFrame) GetEof() bool {
	if m != nil {
		return m.Eof
	}
	return false
}

type HttpHeader struct {
	Key   string `protobuf:"bytes,1,opt,name=key" json:"key,omitempty"`
	Value string `protobuf:"bytes,2,opt,name=value" json:"value,omitempty"`
}

func (m *HttpHeader) Reset()                    { *m = HttpHeader{} }
func (m *HttpHeader) String() string            { return proto.CompactTextString(m) }
func (*HttpHeader) ProtoMessage()               {}
func (*HttpHeader) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{2} }

func (m *HttpHeader) GetKey() string {
	if m != nil {
		return m.Key
	}
	return ""
}

func (m *HttpHeader) GetValue() string {
	if m != nil {
		return m.Value
	}
	return ""
}

type HttpRespMeta struct {
	StatusCode int32         `protobuf:"varint,1,opt,name=status_code,json=statusCode" json:"status_code,omitempty"`
	Headers    []*HttpHeader `protobuf:"bytes,2,rep,name=headers" json:"headers,omitempty"`
}

func (m *HttpRespMeta) Reset()                    { *m = HttpRespMeta{} }
func (m *HttpRespMeta) String() string            { return proto.CompactTextString(m) }
func (*HttpRespMeta) ProtoMessage()               {}
func (*HttpRespMeta) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{3} }

func (m *HttpRespMeta) GetStatusCode() int32 {
	if m != nil {
		return m.StatusCode
	}
	return 0
}

func (m *HttpRespMeta) GetHeaders() []*HttpHeader {
	if m != nil {
		return m.Headers
	}
	return nil
}

// Call has started to finish - data might not be here yet and it will be sent
// as DataFrames.
type CallResultStart struct {
	// Types that are valid to be assigned to Meta:
	//	*CallResultStart_Http
	Meta isCallResultStart_Meta `protobuf_oneof:"meta"`
}

func (m *CallResultStart) Reset()                    { *m = CallResultStart{} }
func (m *CallResultStart) String() string            { return proto.CompactTextString(m) }
func (*CallResultStart) ProtoMessage()               {}
func (*CallResultStart) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{4} }

type isCallResultStart_Meta interface{ isCallResultStart_Meta() }

type CallResultStart_Http struct {
	Http *HttpRespMeta `protobuf:"bytes,100,opt,name=http,oneof"`
}

func (*CallResultStart_Http) isCallResultStart_Meta() {}

func (m *CallResultStart) GetMeta() isCallResultStart_Meta {
	if m != nil {
		return m.Meta
	}
	return nil
}

func (m *CallResultStart) GetHttp() *HttpRespMeta {
	if x, ok := m.GetMeta().(*CallResultStart_Http); ok {
		return x.Http
	}
	return nil
}

// XXX_OneofFuncs is for the internal use of the proto package.
func (*CallResultStart) XXX_OneofFuncs() (func(msg proto.Message, b *proto.Buffer) error, func(msg proto.Message, tag, wire int, b *proto.Buffer) (bool, error), func(msg proto.Message) (n int), []interface{}) {
	return _CallResultStart_OneofMarshaler, _CallResultStart_OneofUnmarshaler, _CallResultStart_OneofSizer, []interface{}{
		(*CallResultStart_Http)(nil),
	}
}

func _CallResultStart_OneofMarshaler(msg proto.Message, b *proto.Buffer) error {
	m := msg.(*CallResultStart)
	// meta
	switch x := m.Meta.(type) {
	case *CallResultStart_Http:
		b.EncodeVarint(100<<3 | proto.WireBytes)
		if err := b.EncodeMessage(x.Http); err != nil {
			return err
		}
	case nil:
	default:
		return fmt.Errorf("CallResultStart.Meta has unexpected type %T", x)
	}
	return nil
}

func _CallResultStart_OneofUnmarshaler(msg proto.Message, tag, wire int, b *proto.Buffer) (bool, error) {
	m := msg.(*CallResultStart)
	switch tag {
	case 100: // meta.http
		if wire != proto.WireBytes {
			return true, proto.ErrInternalBadWireType
		}
		msg := new(HttpRespMeta)
		err := b.DecodeMessage(msg)
		m.Meta = &CallResultStart_Http{msg}
		return true, err
	default:
		return false, nil
	}
}

func _CallResultStart_OneofSizer(msg proto.Message) (n int) {
	m := msg.(*CallResultStart)
	// meta
	switch x := m.Meta.(type) {
	case *CallResultStart_Http:
		s := proto.Size(x.Http)
		n += proto.SizeVarint(100<<3 | proto.WireBytes)
		n += proto.SizeVarint(uint64(s))
		n += s
	case nil:
	default:
		panic(fmt.Sprintf("proto: unexpected type %T in oneof", x))
	}
	return n
}

// Call has really finished, it might have completed or crashed
type CallFinished struct {
	Success   bool   `protobuf:"varint,1,opt,name=success" json:"success,omitempty"`
	Details   string `protobuf:"bytes,2,opt,name=details" json:"details,omitempty"`
	ErrorCode int32  `protobuf:"varint,3,opt,name=errorCode" json:"errorCode,omitempty"`
	ErrorStr  string `protobuf:"bytes,4,opt,name=errorStr" json:"errorStr,omitempty"`
}

func (m *CallFinished) Reset()                    { *m = CallFinished{} }
func (m *CallFinished) String() string            { return proto.CompactTextString(m) }
func (*CallFinished) ProtoMessage()               {}
func (*CallFinished) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{5} }

func (m *CallFinished) GetSuccess() bool {
	if m != nil {
		return m.Success
	}
	return false
}

func (m *CallFinished) GetDetails() string {
	if m != nil {
		return m.Details
	}
	return ""
}

func (m *CallFinished) GetErrorCode() int32 {
	if m != nil {
		return m.ErrorCode
	}
	return 0
}

func (m *CallFinished) GetErrorStr() string {
	if m != nil {
		return m.ErrorStr
	}
	return ""
}

type ClientMsg struct {
	// Types that are valid to be assigned to Body:
	//	*ClientMsg_Try
	//	*ClientMsg_Data
	Body isClientMsg_Body `protobuf_oneof:"body"`
}

func (m *ClientMsg) Reset()                    { *m = ClientMsg{} }
func (m *ClientMsg) String() string            { return proto.CompactTextString(m) }
func (*ClientMsg) ProtoMessage()               {}
func (*ClientMsg) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{6} }

type isClientMsg_Body interface{ isClientMsg_Body() }

type ClientMsg_Try struct {
	Try *TryCall `protobuf:"bytes,1,opt,name=try,oneof"`
}
type ClientMsg_Data struct {
	Data *DataFrame `protobuf:"bytes,2,opt,name=data,oneof"`
}

func (*ClientMsg_Try) isClientMsg_Body()  {}
func (*ClientMsg_Data) isClientMsg_Body() {}

func (m *ClientMsg) GetBody() isClientMsg_Body {
	if m != nil {
		return m.Body
	}
	return nil
}

func (m *ClientMsg) GetTry() *TryCall {
	if x, ok := m.GetBody().(*ClientMsg_Try); ok {
		return x.Try
	}
	return nil
}

func (m *ClientMsg) GetData() *DataFrame {
	if x, ok := m.GetBody().(*ClientMsg_Data); ok {
		return x.Data
	}
	return nil
}

// XXX_OneofFuncs is for the internal use of the proto package.
func (*ClientMsg) XXX_OneofFuncs() (func(msg proto.Message, b *proto.Buffer) error, func(msg proto.Message, tag, wire int, b *proto.Buffer) (bool, error), func(msg proto.Message) (n int), []interface{}) {
	return _ClientMsg_OneofMarshaler, _ClientMsg_OneofUnmarshaler, _ClientMsg_OneofSizer, []interface{}{
		(*ClientMsg_Try)(nil),
		(*ClientMsg_Data)(nil),
	}
}

func _ClientMsg_OneofMarshaler(msg proto.Message, b *proto.Buffer) error {
	m := msg.(*ClientMsg)
	// body
	switch x := m.Body.(type) {
	case *ClientMsg_Try:
		b.EncodeVarint(1<<3 | proto.WireBytes)
		if err := b.EncodeMessage(x.Try); err != nil {
			return err
		}
	case *ClientMsg_Data:
		b.EncodeVarint(2<<3 | proto.WireBytes)
		if err := b.EncodeMessage(x.Data); err != nil {
			return err
		}
	case nil:
	default:
		return fmt.Errorf("ClientMsg.Body has unexpected type %T", x)
	}
	return nil
}

func _ClientMsg_OneofUnmarshaler(msg proto.Message, tag, wire int, b *proto.Buffer) (bool, error) {
	m := msg.(*ClientMsg)
	switch tag {
	case 1: // body.try
		if wire != proto.WireBytes {
			return true, proto.ErrInternalBadWireType
		}
		msg := new(TryCall)
		err := b.DecodeMessage(msg)
		m.Body = &ClientMsg_Try{msg}
		return true, err
	case 2: // body.data
		if wire != proto.WireBytes {
			return true, proto.ErrInternalBadWireType
		}
		msg := new(DataFrame)
		err := b.DecodeMessage(msg)
		m.Body = &ClientMsg_Data{msg}
		return true, err
	default:
		return false, nil
	}
}

func _ClientMsg_OneofSizer(msg proto.Message) (n int) {
	m := msg.(*ClientMsg)
	// body
	switch x := m.Body.(type) {
	case *ClientMsg_Try:
		s := proto.Size(x.Try)
		n += proto.SizeVarint(1<<3 | proto.WireBytes)
		n += proto.SizeVarint(uint64(s))
		n += s
	case *ClientMsg_Data:
		s := proto.Size(x.Data)
		n += proto.SizeVarint(2<<3 | proto.WireBytes)
		n += proto.SizeVarint(uint64(s))
		n += s
	case nil:
	default:
		panic(fmt.Sprintf("proto: unexpected type %T in oneof", x))
	}
	return n
}

type RunnerMsg struct {
	// Types that are valid to be assigned to Body:
	//	*RunnerMsg_ResultStart
	//	*RunnerMsg_Data
	//	*RunnerMsg_Finished
	Body isRunnerMsg_Body `protobuf_oneof:"body"`
}

func (m *RunnerMsg) Reset()                    { *m = RunnerMsg{} }
func (m *RunnerMsg) String() string            { return proto.CompactTextString(m) }
func (*RunnerMsg) ProtoMessage()               {}
func (*RunnerMsg) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{7} }

type isRunnerMsg_Body interface{ isRunnerMsg_Body() }

type RunnerMsg_ResultStart struct {
	ResultStart *CallResultStart `protobuf:"bytes,1,opt,name=result_start,json=resultStart,oneof"`
}
type RunnerMsg_Data struct {
	Data *DataFrame `protobuf:"bytes,2,opt,name=data,oneof"`
}
type RunnerMsg_Finished struct {
	Finished *CallFinished `protobuf:"bytes,3,opt,name=finished,oneof"`
}

func (*RunnerMsg_ResultStart) isRunnerMsg_Body() {}
func (*RunnerMsg_Data) isRunnerMsg_Body()        {}
func (*RunnerMsg_Finished) isRunnerMsg_Body()    {}

func (m *RunnerMsg) GetBody() isRunnerMsg_Body {
	if m != nil {
		return m.Body
	}
	return nil
}

func (m *RunnerMsg) GetResultStart() *CallResultStart {
	if x, ok := m.GetBody().(*RunnerMsg_ResultStart); ok {
		return x.ResultStart
	}
	return nil
}

func (m *RunnerMsg) GetData() *DataFrame {
	if x, ok := m.GetBody().(*RunnerMsg_Data); ok {
		return x.Data
	}
	return nil
}

func (m *RunnerMsg) GetFinished() *CallFinished {
	if x, ok := m.GetBody().(*RunnerMsg_Finished); ok {
		return x.Finished
	}
	return nil
}

// XXX_OneofFuncs is for the internal use of the proto package.
func (*RunnerMsg) XXX_OneofFuncs() (func(msg proto.Message, b *proto.Buffer) error, func(msg proto.Message, tag, wire int, b *proto.Buffer) (bool, error), func(msg proto.Message) (n int), []interface{}) {
	return _RunnerMsg_OneofMarshaler, _RunnerMsg_OneofUnmarshaler, _RunnerMsg_OneofSizer, []interface{}{
		(*RunnerMsg_ResultStart)(nil),
		(*RunnerMsg_Data)(nil),
		(*RunnerMsg_Finished)(nil),
	}
}

func _RunnerMsg_OneofMarshaler(msg proto.Message, b *proto.Buffer) error {
	m := msg.(*RunnerMsg)
	// body
	switch x := m.Body.(type) {
	case *RunnerMsg_ResultStart:
		b.EncodeVarint(1<<3 | proto.WireBytes)
		if err := b.EncodeMessage(x.ResultStart); err != nil {
			return err
		}
	case *RunnerMsg_Data:
		b.EncodeVarint(2<<3 | proto.WireBytes)
		if err := b.EncodeMessage(x.Data); err != nil {
			return err
		}
	case *RunnerMsg_Finished:
		b.EncodeVarint(3<<3 | proto.WireBytes)
		if err := b.EncodeMessage(x.Finished); err != nil {
			return err
		}
	case nil:
	default:
		return fmt.Errorf("RunnerMsg.Body has unexpected type %T", x)
	}
	return nil
}

func _RunnerMsg_OneofUnmarshaler(msg proto.Message, tag, wire int, b *proto.Buffer) (bool, error) {
	m := msg.(*RunnerMsg)
	switch tag {
	case 1: // body.result_start
		if wire != proto.WireBytes {
			return true, proto.ErrInternalBadWireType
		}
		msg := new(CallResultStart)
		err := b.DecodeMessage(msg)
		m.Body = &RunnerMsg_ResultStart{msg}
		return true, err
	case 2: // body.data
		if wire != proto.WireBytes {
			return true, proto.ErrInternalBadWireType
		}
		msg := new(DataFrame)
		err := b.DecodeMessage(msg)
		m.Body = &RunnerMsg_Data{msg}
		return true, err
	case 3: // body.finished
		if wire != proto.WireBytes {
			return true, proto.ErrInternalBadWireType
		}
		msg := new(CallFinished)
		err := b.DecodeMessage(msg)
		m.Body = &RunnerMsg_Finished{msg}
		return true, err
	default:
		return false, nil
	}
}

func _RunnerMsg_OneofSizer(msg proto.Message) (n int) {
	m := msg.(*RunnerMsg)
	// body
	switch x := m.Body.(type) {
	case *RunnerMsg_ResultStart:
		s := proto.Size(x.ResultStart)
		n += proto.SizeVarint(1<<3 | proto.WireBytes)
		n += proto.SizeVarint(uint64(s))
		n += s
	case *RunnerMsg_Data:
		s := proto.Size(x.Data)
		n += proto.SizeVarint(2<<3 | proto.WireBytes)
		n += proto.SizeVarint(uint64(s))
		n += s
	case *RunnerMsg_Finished:
		s := proto.Size(x.Finished)
		n += proto.SizeVarint(3<<3 | proto.WireBytes)
		n += proto.SizeVarint(uint64(s))
		n += s
	case nil:
	default:
		panic(fmt.Sprintf("proto: unexpected type %T in oneof", x))
	}
	return n
}

type RunnerStatus struct {
	Active int32 `protobuf:"varint,2,opt,name=active" json:"active,omitempty"`
}

func (m *RunnerStatus) Reset()                    { *m = RunnerStatus{} }
func (m *RunnerStatus) String() string            { return proto.CompactTextString(m) }
func (*RunnerStatus) ProtoMessage()               {}
func (*RunnerStatus) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{8} }

func (m *RunnerStatus) GetActive() int32 {
	if m != nil {
		return m.Active
	}
	return 0
}

func init() {
	proto.RegisterType((*TryCall)(nil), "TryCall")
	proto.RegisterType((*DataFrame)(nil), "DataFrame")
	proto.RegisterType((*HttpHeader)(nil), "HttpHeader")
	proto.RegisterType((*HttpRespMeta)(nil), "HttpRespMeta")
	proto.RegisterType((*CallResultStart)(nil), "CallResultStart")
	proto.RegisterType((*CallFinished)(nil), "CallFinished")
	proto.RegisterType((*ClientMsg)(nil), "ClientMsg")
	proto.RegisterType((*RunnerMsg)(nil), "RunnerMsg")
	proto.RegisterType((*RunnerStatus)(nil), "RunnerStatus")
}

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ grpc.ClientConn

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion4

// Client API for RunnerProtocol service

type RunnerProtocolClient interface {
	Engage(ctx context.Context, opts ...grpc.CallOption) (RunnerProtocol_EngageClient, error)
	// Rather than rely on Prometheus for this, expose status that's specific to the runner lifecycle through this.
	Status(ctx context.Context, in *google_protobuf.Empty, opts ...grpc.CallOption) (*RunnerStatus, error)
}

type runnerProtocolClient struct {
	cc *grpc.ClientConn
}

func NewRunnerProtocolClient(cc *grpc.ClientConn) RunnerProtocolClient {
	return &runnerProtocolClient{cc}
}

func (c *runnerProtocolClient) Engage(ctx context.Context, opts ...grpc.CallOption) (RunnerProtocol_EngageClient, error) {
	stream, err := grpc.NewClientStream(ctx, &_RunnerProtocol_serviceDesc.Streams[0], c.cc, "/RunnerProtocol/Engage", opts...)
	if err != nil {
		return nil, err
	}
	x := &runnerProtocolEngageClient{stream}
	return x, nil
}

type RunnerProtocol_EngageClient interface {
	Send(*ClientMsg) error
	Recv() (*RunnerMsg, error)
	grpc.ClientStream
}

type runnerProtocolEngageClient struct {
	grpc.ClientStream
}

func (x *runnerProtocolEngageClient) Send(m *ClientMsg) error {
	return x.ClientStream.SendMsg(m)
}

func (x *runnerProtocolEngageClient) Recv() (*RunnerMsg, error) {
	m := new(RunnerMsg)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *runnerProtocolClient) Status(ctx context.Context, in *google_protobuf.Empty, opts ...grpc.CallOption) (*RunnerStatus, error) {
	out := new(RunnerStatus)
	err := grpc.Invoke(ctx, "/RunnerProtocol/Status", in, out, c.cc, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Server API for RunnerProtocol service

type RunnerProtocolServer interface {
	Engage(RunnerProtocol_EngageServer) error
	// Rather than rely on Prometheus for this, expose status that's specific to the runner lifecycle through this.
	Status(context.Context, *google_protobuf.Empty) (*RunnerStatus, error)
}

func RegisterRunnerProtocolServer(s *grpc.Server, srv RunnerProtocolServer) {
	s.RegisterService(&_RunnerProtocol_serviceDesc, srv)
}

func _RunnerProtocol_Engage_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(RunnerProtocolServer).Engage(&runnerProtocolEngageServer{stream})
}

type RunnerProtocol_EngageServer interface {
	Send(*RunnerMsg) error
	Recv() (*ClientMsg, error)
	grpc.ServerStream
}

type runnerProtocolEngageServer struct {
	grpc.ServerStream
}

func (x *runnerProtocolEngageServer) Send(m *RunnerMsg) error {
	return x.ServerStream.SendMsg(m)
}

func (x *runnerProtocolEngageServer) Recv() (*ClientMsg, error) {
	m := new(ClientMsg)
	if err := x.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func _RunnerProtocol_Status_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(google_protobuf.Empty)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(RunnerProtocolServer).Status(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/RunnerProtocol/Status",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(RunnerProtocolServer).Status(ctx, req.(*google_protobuf.Empty))
	}
	return interceptor(ctx, in, info, handler)
}

var _RunnerProtocol_serviceDesc = grpc.ServiceDesc{
	ServiceName: "RunnerProtocol",
	HandlerType: (*RunnerProtocolServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Status",
			Handler:    _RunnerProtocol_Status_Handler,
		},
	},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "Engage",
			Handler:       _RunnerProtocol_Engage_Handler,
			ServerStreams: true,
			ClientStreams: true,
		},
	},
	Metadata: "runner.proto",
}

func init() { proto.RegisterFile("runner.proto", fileDescriptor0) }

var fileDescriptor0 = []byte{
	// 529 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x84, 0x53, 0x5d, 0x8b, 0x13, 0x31,
	0x14, 0xed, 0xf4, 0xbb, 0x77, 0x66, 0xd7, 0x12, 0x44, 0x86, 0xba, 0x60, 0x19, 0x3f, 0x28, 0x08,
	0xa9, 0x56, 0x7d, 0xf5, 0xc1, 0xba, 0xcb, 0x28, 0x2c, 0x48, 0xaa, 0xbe, 0x0e, 0xe9, 0x24, 0x9d,
	0x19, 0x4d, 0x27, 0x25, 0xc9, 0x2c, 0x14, 0xfc, 0x23, 0xfe, 0x5b, 0x49, 0x66, 0x3a, 0x5b, 0x7c,
	0xd9, 0xb7, 0x9c, 0x7b, 0x72, 0x3f, 0xce, 0xb9, 0x09, 0x04, 0xaa, 0x2a, 0x4b, 0xae, 0xf0, 0x41,
	0x49, 0x23, 0x67, 0x4f, 0x33, 0x29, 0x33, 0xc1, 0x97, 0x0e, 0x6d, 0xab, 0xdd, 0x92, 0xef, 0x0f,
	0xe6, 0x58, 0x93, 0xd1, 0x0f, 0x18, 0x7d, 0x57, 0xc7, 0x35, 0x15, 0x02, 0x2d, 0x60, 0xba, 0x97,
	0x8c, 0x0b, 0x9d, 0xa4, 0x54, 0x88, 0xe4, 0x97, 0x96, 0x65, 0xe8, 0xcd, 0xbd, 0xc5, 0x84, 0x5c,
	0xd6, 0x71, 0x7b, 0xeb, 0xab, 0x96, 0x25, 0x9a, 0x43, 0xa0, 0x85, 0x34, 0x49, 0x4e, 0x75, 0x9e,
	0x14, 0x2c, 0xec, 0xba, 0x5b, 0x60, 0x63, 0x31, 0xd5, 0xf9, 0x17, 0x16, 0xbd, 0x85, 0xc9, 0x67,
	0x6a, 0xe8, 0x8d, 0xa2, 0x7b, 0x8e, 0x10, 0xf4, 0x19, 0x35, 0xd4, 0x15, 0x0b, 0x88, 0x3b, 0xa3,
	0x29, 0xf4, 0xb8, 0xdc, 0xb9, 0xcc, 0x31, 0xb1, 0xc7, 0xe8, 0x3d, 0x40, 0x6c, 0xcc, 0x21, 0xe6,
	0x94, 0x71, 0x65, 0xf9, 0xdf, 0xfc, 0xd8, 0xf4, 0xb7, 0x47, 0xf4, 0x18, 0x06, 0x77, 0x54, 0x54,
	0xbc, 0xe9, 0x56, 0x83, 0xe8, 0x27, 0x04, 0x36, 0x8b, 0x70, 0x7d, 0xb8, 0xe5, 0x86, 0xa2, 0x67,
	0xe0, 0x6b, 0x43, 0x4d, 0xa5, 0x93, 0x54, 0x32, 0xee, 0xf2, 0x07, 0x04, 0xea, 0xd0, 0x5a, 0x32,
	0x8e, 0x5e, 0xc2, 0x28, 0x77, 0x2d, 0x74, 0xd8, 0x9d, 0xf7, 0x16, 0xfe, 0xca, 0xc7, 0xf7, 0x6d,
	0xc9, 0x89, 0x8b, 0x3e, 0xc2, 0x23, 0x2b, 0x97, 0x70, 0x5d, 0x09, 0xb3, 0x31, 0x54, 0x19, 0xf4,
	0x1c, 0xfa, 0xb9, 0x31, 0x87, 0x90, 0xcd, 0xbd, 0x85, 0xbf, 0xba, 0xc0, 0xe7, 0x7d, 0xe3, 0x0e,
	0x71, 0xe4, 0xa7, 0x21, 0xf4, 0xf7, 0xdc, 0xd0, 0xe8, 0x0f, 0x04, 0x36, 0xff, 0xa6, 0x28, 0x0b,
	0x9d, 0x73, 0x86, 0x42, 0x18, 0xe9, 0x2a, 0x4d, 0xb9, 0xd6, 0x6e, 0xa6, 0x31, 0x39, 0x41, 0xcb,
	0x30, 0x6e, 0x68, 0x21, 0x74, 0xa3, 0xec, 0x04, 0xd1, 0x15, 0x4c, 0xb8, 0x52, 0x52, 0xd9, 0xb9,
	0xc3, 0x9e, 0x53, 0x72, 0x1f, 0x40, 0x33, 0x18, 0x3b, 0xb0, 0x31, 0x2a, 0xec, 0xbb, 0xc4, 0x16,
	0x47, 0x1b, 0x98, 0xac, 0x45, 0xc1, 0x4b, 0x73, 0xab, 0x33, 0x74, 0x05, 0x3d, 0xa3, 0x6a, 0x2b,
	0xfd, 0xd5, 0x18, 0x37, 0xeb, 0x8e, 0x3b, 0xc4, 0x86, 0xd1, 0xbc, 0x59, 0x4e, 0xd7, 0xd1, 0x80,
	0xdb, 0xb5, 0x59, 0x49, 0x96, 0xb1, 0x92, 0xb6, 0x92, 0x1d, 0xa3, 0xbf, 0x1e, 0x4c, 0x88, 0x7b,
	0x58, 0xb6, 0xea, 0x07, 0x08, 0x94, 0x33, 0x27, 0xd1, 0xd6, 0x9d, 0xa6, 0xfc, 0x14, 0xff, 0xe7,
	0x5a, 0xdc, 0x21, 0xbe, 0x3a, 0x33, 0xf1, 0xc1, 0x76, 0xe8, 0x35, 0x8c, 0x77, 0x8d, 0x6b, 0x4e,
	0xb4, 0xb5, 0xfa, 0xdc, 0xca, 0xb8, 0x43, 0xda, 0x0b, 0xed, 0x6c, 0xaf, 0x20, 0xa8, 0x47, 0xdb,
	0xb8, 0x4d, 0xa3, 0x27, 0x30, 0xa4, 0xa9, 0x29, 0xee, 0xea, 0xd7, 0x32, 0x20, 0x0d, 0x5a, 0x65,
	0x70, 0x59, 0xdf, 0xfb, 0x66, 0x5f, 0x7f, 0x2a, 0x05, 0x7a, 0x01, 0xc3, 0xeb, 0x32, 0xa3, 0x19,
	0x47, 0x80, 0x5b, 0xcf, 0x66, 0x80, 0x5b, 0xa5, 0x0b, 0xef, 0x8d, 0x87, 0x96, 0x30, 0x3c, 0x55,
	0xc6, 0xf5, 0x77, 0xc2, 0xa7, 0xef, 0x84, 0xaf, 0xed, 0x77, 0x9a, 0x5d, 0xe0, 0xf3, 0x01, 0xb6,
	0x43, 0x47, 0xbf, 0xfb, 0x17, 0x00, 0x00, 0xff, 0xff, 0x0a, 0xb4, 0xdf, 0x3c, 0x8b, 0x03, 0x00,
	0x00,
}
