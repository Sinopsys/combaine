// Code generated by protoc-gen-go.
// source: rpc.proto
// DO NOT EDIT!

/*
Package rpc is a generated protocol buffer package.

It is generated from these files:
	rpc.proto

It has these top-level messages:
	TimeFrame
	ParsingTask
	ParsingResult
	AggregatingTask
	AggregatingResult
*/
package rpc

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

type TimeFrame struct {
	Previous int64 `protobuf:"varint,1,opt,name=previous" json:"previous,omitempty"`
	Current  int64 `protobuf:"varint,2,opt,name=current" json:"current,omitempty"`
}

func (m *TimeFrame) Reset()                    { *m = TimeFrame{} }
func (m *TimeFrame) String() string            { return proto.CompactTextString(m) }
func (*TimeFrame) ProtoMessage()               {}
func (*TimeFrame) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{0} }

type ParsingTask struct {
	Id    string     `protobuf:"bytes,1,opt,name=id" json:"id,omitempty"`
	Frame *TimeFrame `protobuf:"bytes,2,opt,name=frame" json:"frame,omitempty"`
	// Hostname of target
	Host string `protobuf:"bytes,3,opt,name=host" json:"host,omitempty"`
	// Name of handled parsing config
	ParsingConfigName string `protobuf:"bytes,4,opt,name=parsing_config_name,json=parsingConfigName" json:"parsing_config_name,omitempty"`
	// msgpacked content of the current parsing config
	ParsingConfig []byte `protobuf:"bytes,5,opt,name=parsing_config,json=parsingConfig,proto3" json:"parsing_config,omitempty"`
	// msgpacked content of aggregation configs
	// related to the current parsing config
	AggregationConfigs map[string][]byte `protobuf:"bytes,6,rep,name=aggregation_configs,json=aggregationConfigs" json:"aggregation_configs,omitempty" protobuf_key:"bytes,1,opt,name=key" protobuf_val:"bytes,2,opt,name=value,proto3"`
}

func (m *ParsingTask) Reset()                    { *m = ParsingTask{} }
func (m *ParsingTask) String() string            { return proto.CompactTextString(m) }
func (*ParsingTask) ProtoMessage()               {}
func (*ParsingTask) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{1} }

func (m *ParsingTask) GetFrame() *TimeFrame {
	if m != nil {
		return m.Frame
	}
	return nil
}

func (m *ParsingTask) GetAggregationConfigs() map[string][]byte {
	if m != nil {
		return m.AggregationConfigs
	}
	return nil
}

type ParsingResult struct {
	Data map[string][]byte `protobuf:"bytes,1,rep,name=data" json:"data,omitempty" protobuf_key:"bytes,1,opt,name=key" protobuf_val:"bytes,2,opt,name=value,proto3"`
}

func (m *ParsingResult) Reset()                    { *m = ParsingResult{} }
func (m *ParsingResult) String() string            { return proto.CompactTextString(m) }
func (*ParsingResult) ProtoMessage()               {}
func (*ParsingResult) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{2} }

func (m *ParsingResult) GetData() map[string][]byte {
	if m != nil {
		return m.Data
	}
	return nil
}

type AggregatingTask struct {
	Id    string     `protobuf:"bytes,1,opt,name=id" json:"id,omitempty"`
	Frame *TimeFrame `protobuf:"bytes,2,opt,name=frame" json:"frame,omitempty"`
	// Name of the current aggregation config
	Config string `protobuf:"bytes,3,opt,name=config" json:"config,omitempty"`
	// Name of handled parsing config
	ParsingConfigName string `protobuf:"bytes,4,opt,name=parsing_config_name,json=parsingConfigName" json:"parsing_config_name,omitempty"`
	// Content of the current parsing config
	ParsingConfig []byte `protobuf:"bytes,5,opt,name=parsing_config,json=parsingConfig,proto3" json:"parsing_config,omitempty"`
	// Current aggregation config
	AggregationConfig []byte `protobuf:"bytes,6,opt,name=aggregation_config,json=aggregationConfig,proto3" json:"aggregation_config,omitempty"`
	// hosts
	Hosts []byte `protobuf:"bytes,7,opt,name=hosts,proto3" json:"hosts,omitempty"`
	// parsing results
	ParsingResult *ParsingResult `protobuf:"bytes,8,opt,name=ParsingResult,json=parsingResult" json:"ParsingResult,omitempty"`
}

func (m *AggregatingTask) Reset()                    { *m = AggregatingTask{} }
func (m *AggregatingTask) String() string            { return proto.CompactTextString(m) }
func (*AggregatingTask) ProtoMessage()               {}
func (*AggregatingTask) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{3} }

func (m *AggregatingTask) GetFrame() *TimeFrame {
	if m != nil {
		return m.Frame
	}
	return nil
}

func (m *AggregatingTask) GetParsingResult() *ParsingResult {
	if m != nil {
		return m.ParsingResult
	}
	return nil
}

type AggregatingResult struct {
}

func (m *AggregatingResult) Reset()                    { *m = AggregatingResult{} }
func (m *AggregatingResult) String() string            { return proto.CompactTextString(m) }
func (*AggregatingResult) ProtoMessage()               {}
func (*AggregatingResult) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{4} }

func init() {
	proto.RegisterType((*TimeFrame)(nil), "TimeFrame")
	proto.RegisterType((*ParsingTask)(nil), "ParsingTask")
	proto.RegisterType((*ParsingResult)(nil), "ParsingResult")
	proto.RegisterType((*AggregatingTask)(nil), "AggregatingTask")
	proto.RegisterType((*AggregatingResult)(nil), "AggregatingResult")
}

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ grpc.ClientConn

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion3

// Client API for Worker service

type WorkerClient interface {
	DoParsing(ctx context.Context, in *ParsingTask, opts ...grpc.CallOption) (*ParsingResult, error)
	DoAggregating(ctx context.Context, in *AggregatingTask, opts ...grpc.CallOption) (*AggregatingResult, error)
}

type workerClient struct {
	cc *grpc.ClientConn
}

func NewWorkerClient(cc *grpc.ClientConn) WorkerClient {
	return &workerClient{cc}
}

func (c *workerClient) DoParsing(ctx context.Context, in *ParsingTask, opts ...grpc.CallOption) (*ParsingResult, error) {
	out := new(ParsingResult)
	err := grpc.Invoke(ctx, "/Worker/DoParsing", in, out, c.cc, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *workerClient) DoAggregating(ctx context.Context, in *AggregatingTask, opts ...grpc.CallOption) (*AggregatingResult, error) {
	out := new(AggregatingResult)
	err := grpc.Invoke(ctx, "/Worker/DoAggregating", in, out, c.cc, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Server API for Worker service

type WorkerServer interface {
	DoParsing(context.Context, *ParsingTask) (*ParsingResult, error)
	DoAggregating(context.Context, *AggregatingTask) (*AggregatingResult, error)
}

func RegisterWorkerServer(s *grpc.Server, srv WorkerServer) {
	s.RegisterService(&_Worker_serviceDesc, srv)
}

func _Worker_DoParsing_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ParsingTask)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(WorkerServer).DoParsing(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/Worker/DoParsing",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(WorkerServer).DoParsing(ctx, req.(*ParsingTask))
	}
	return interceptor(ctx, in, info, handler)
}

func _Worker_DoAggregating_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(AggregatingTask)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(WorkerServer).DoAggregating(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/Worker/DoAggregating",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(WorkerServer).DoAggregating(ctx, req.(*AggregatingTask))
	}
	return interceptor(ctx, in, info, handler)
}

var _Worker_serviceDesc = grpc.ServiceDesc{
	ServiceName: "Worker",
	HandlerType: (*WorkerServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "DoParsing",
			Handler:    _Worker_DoParsing_Handler,
		},
		{
			MethodName: "DoAggregating",
			Handler:    _Worker_DoAggregating_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: fileDescriptor0,
}

func init() { proto.RegisterFile("rpc.proto", fileDescriptor0) }

var fileDescriptor0 = []byte{
	// 420 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x09, 0x6e, 0x88, 0x02, 0xff, 0xb4, 0x53, 0x4d, 0x8f, 0xd3, 0x30,
	0x10, 0xdd, 0x24, 0x6d, 0x76, 0x33, 0xfd, 0x60, 0x3b, 0x45, 0x60, 0xe5, 0x14, 0x45, 0x20, 0x55,
	0x02, 0x7c, 0x28, 0x48, 0x8b, 0xb8, 0xad, 0xe8, 0x72, 0x44, 0x28, 0x5a, 0xc4, 0x71, 0x65, 0x5a,
	0x37, 0x44, 0x6d, 0xe3, 0xc8, 0x76, 0x2a, 0xf5, 0x47, 0xf1, 0x3f, 0xf8, 0x59, 0x28, 0x76, 0x08,
	0x49, 0x23, 0x0e, 0x20, 0x71, 0xf3, 0x78, 0xde, 0xbc, 0x99, 0x79, 0xcf, 0x86, 0x40, 0x16, 0x6b,
	0x5a, 0x48, 0xa1, 0x45, 0x7c, 0x0b, 0xc1, 0x7d, 0x76, 0xe0, 0x1f, 0x24, 0x3b, 0x70, 0x0c, 0xe1,
	0xaa, 0x90, 0xfc, 0x98, 0x89, 0x52, 0x11, 0x27, 0x72, 0x16, 0x5e, 0xd2, 0xc4, 0x48, 0xe0, 0x72,
	0x5d, 0x4a, 0xc9, 0x73, 0x4d, 0x5c, 0x93, 0xfa, 0x15, 0xc6, 0x3f, 0x5c, 0x18, 0x7d, 0x62, 0x52,
	0x65, 0x79, 0x7a, 0xcf, 0xd4, 0x0e, 0xa7, 0xe0, 0x66, 0x1b, 0x53, 0x1f, 0x24, 0x6e, 0xb6, 0xc1,
	0x08, 0x86, 0xdb, 0x8a, 0xde, 0xd4, 0x8d, 0x96, 0x40, 0x9b, 0x86, 0x89, 0x4d, 0x20, 0xc2, 0xe0,
	0x9b, 0x50, 0x9a, 0x78, 0xa6, 0xc6, 0x9c, 0x91, 0xc2, 0xbc, 0xb0, 0xa4, 0x0f, 0x6b, 0x91, 0x6f,
	0xb3, 0xf4, 0x21, 0xaf, 0x38, 0x06, 0x06, 0x32, 0xab, 0x53, 0xef, 0x4d, 0xe6, 0x63, 0xc5, 0xf1,
	0x1c, 0xa6, 0x5d, 0x3c, 0x19, 0x46, 0xce, 0x62, 0x9c, 0x4c, 0x3a, 0x50, 0xfc, 0x0c, 0x73, 0x96,
	0xa6, 0x92, 0xa7, 0x4c, 0x67, 0x22, 0xaf, 0xa1, 0x8a, 0xf8, 0x91, 0xb7, 0x18, 0x2d, 0x9f, 0xd1,
	0xd6, 0x1e, 0xf4, 0xf6, 0x37, 0xce, 0x16, 0xab, 0xbb, 0x5c, 0xcb, 0x53, 0x82, 0xac, 0x97, 0x08,
	0xef, 0xe0, 0xe9, 0x1f, 0xe0, 0x78, 0x0d, 0xde, 0x8e, 0x9f, 0x6a, 0x3d, 0xaa, 0x23, 0x3e, 0x86,
	0xe1, 0x91, 0xed, 0x4b, 0x2b, 0xc8, 0x38, 0xb1, 0xc1, 0x3b, 0xf7, 0xad, 0x13, 0x1f, 0x61, 0x52,
	0x4f, 0x90, 0x70, 0x55, 0xee, 0x35, 0xbe, 0x84, 0xc1, 0x86, 0x69, 0x46, 0x1c, 0x33, 0x1f, 0xa1,
	0x9d, 0x2c, 0x5d, 0x31, 0xcd, 0xec, 0x4c, 0x06, 0x15, 0xde, 0x40, 0xd0, 0x5c, 0xfd, 0x55, 0xdf,
	0xef, 0x2e, 0x3c, 0x6a, 0xe6, 0xff, 0x67, 0x1b, 0x9f, 0x80, 0x5f, 0x4b, 0x6f, 0x8d, 0xac, 0xa3,
	0xff, 0x65, 0xe5, 0x2b, 0xc0, 0xbe, 0x95, 0xc4, 0x37, 0xd0, 0x59, 0xcf, 0xa3, 0x6a, 0xfb, 0xea,
	0x61, 0x29, 0x72, 0x69, 0xb7, 0x37, 0x01, 0xbe, 0x39, 0x53, 0x9c, 0x5c, 0x99, 0xed, 0xa6, 0x5d,
	0xa5, 0x9b, 0xd6, 0x36, 0x8c, 0xe7, 0x30, 0x6b, 0xc9, 0x65, 0x2f, 0x97, 0x39, 0xf8, 0x5f, 0x84,
	0xdc, 0x71, 0x89, 0x2f, 0x20, 0x58, 0x89, 0x9a, 0x00, 0xc7, 0xed, 0x47, 0x15, 0x9e, 0x11, 0xc7,
	0x17, 0x78, 0x03, 0x93, 0x95, 0x68, 0xb1, 0xe1, 0x35, 0x3d, 0xb3, 0x22, 0x44, 0xda, 0xeb, 0x16,
	0x5f, 0x7c, 0xf5, 0xcd, 0x0f, 0x7e, 0xfd, 0x33, 0x00, 0x00, 0xff, 0xff, 0xc6, 0x86, 0x36, 0xba,
	0xce, 0x03, 0x00, 0x00,
}