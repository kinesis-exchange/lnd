// Code generated by protoc-gen-go. DO NOT EDIT.
// source: rpc.proto

/*
Package extpreimage is a generated protocol buffer package.

It is generated from these files:
	rpc.proto

It has these top-level messages:
	GetPreimageRequest
	GetPreimageResponse
*/
package extpreimage

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

type Symbol int32

const (
	Symbol_BTC Symbol = 0
	Symbol_LTC Symbol = 1
)

var Symbol_name = map[int32]string{
	0: "BTC",
	1: "LTC",
}
var Symbol_value = map[string]int32{
	"BTC": 0,
	"LTC": 1,
}

func (x Symbol) String() string {
	return proto.EnumName(Symbol_name, int32(x))
}
func (Symbol) EnumDescriptor() ([]byte, []int) { return fileDescriptor0, []int{0} }

type GetPreimageRequest struct {
	// Hash of the payment for which we want to retrieve the preimage
	PaymentHash []byte `protobuf:"bytes,1,opt,name=payment_hash,json=paymentHash,proto3" json:"payment_hash,omitempty"`
	// The amount of the payment, in integer units (e.g. Satoshis)
	Amount int64 `protobuf:"varint,5,opt,name=amount" json:"amount,omitempty"`
	// Symbol of the amount
	Symbol Symbol `protobuf:"varint,6,opt,name=symbol,enum=extpreimage.Symbol" json:"symbol,omitempty"`
	// time lock of the payment extended to us
	TimeLock int64 `protobuf:"varint,10,opt,name=time_lock,json=timeLock" json:"time_lock,omitempty"`
	// current height of the blockchain
	BestHeight int64 `protobuf:"varint,11,opt,name=best_height,json=bestHeight" json:"best_height,omitempty"`
}

func (m *GetPreimageRequest) Reset()                    { *m = GetPreimageRequest{} }
func (m *GetPreimageRequest) String() string            { return proto.CompactTextString(m) }
func (*GetPreimageRequest) ProtoMessage()               {}
func (*GetPreimageRequest) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{0} }

func (m *GetPreimageRequest) GetPaymentHash() []byte {
	if m != nil {
		return m.PaymentHash
	}
	return nil
}

func (m *GetPreimageRequest) GetAmount() int64 {
	if m != nil {
		return m.Amount
	}
	return 0
}

func (m *GetPreimageRequest) GetSymbol() Symbol {
	if m != nil {
		return m.Symbol
	}
	return Symbol_BTC
}

func (m *GetPreimageRequest) GetTimeLock() int64 {
	if m != nil {
		return m.TimeLock
	}
	return 0
}

func (m *GetPreimageRequest) GetBestHeight() int64 {
	if m != nil {
		return m.BestHeight
	}
	return 0
}

type GetPreimageResponse struct {
	// preimage for the requested payment
	PaymentPreimage []byte `protobuf:"bytes,1,opt,name=payment_preimage,json=paymentPreimage,proto3" json:"payment_preimage,omitempty"`
}

func (m *GetPreimageResponse) Reset()                    { *m = GetPreimageResponse{} }
func (m *GetPreimageResponse) String() string            { return proto.CompactTextString(m) }
func (*GetPreimageResponse) ProtoMessage()               {}
func (*GetPreimageResponse) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{1} }

func (m *GetPreimageResponse) GetPaymentPreimage() []byte {
	if m != nil {
		return m.PaymentPreimage
	}
	return nil
}

func init() {
	proto.RegisterType((*GetPreimageRequest)(nil), "extpreimage.GetPreimageRequest")
	proto.RegisterType((*GetPreimageResponse)(nil), "extpreimage.GetPreimageResponse")
	proto.RegisterEnum("extpreimage.Symbol", Symbol_name, Symbol_value)
}

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ grpc.ClientConn

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion4

// Client API for ExternalPreimageService service

type ExternalPreimageServiceClient interface {
	GetPreimage(ctx context.Context, in *GetPreimageRequest, opts ...grpc.CallOption) (ExternalPreimageService_GetPreimageClient, error)
}

type externalPreimageServiceClient struct {
	cc *grpc.ClientConn
}

func NewExternalPreimageServiceClient(cc *grpc.ClientConn) ExternalPreimageServiceClient {
	return &externalPreimageServiceClient{cc}
}

func (c *externalPreimageServiceClient) GetPreimage(ctx context.Context, in *GetPreimageRequest, opts ...grpc.CallOption) (ExternalPreimageService_GetPreimageClient, error) {
	stream, err := grpc.NewClientStream(ctx, &_ExternalPreimageService_serviceDesc.Streams[0], c.cc, "/extpreimage.ExternalPreimageService/GetPreimage", opts...)
	if err != nil {
		return nil, err
	}
	x := &externalPreimageServiceGetPreimageClient{stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

type ExternalPreimageService_GetPreimageClient interface {
	Recv() (*GetPreimageResponse, error)
	grpc.ClientStream
}

type externalPreimageServiceGetPreimageClient struct {
	grpc.ClientStream
}

func (x *externalPreimageServiceGetPreimageClient) Recv() (*GetPreimageResponse, error) {
	m := new(GetPreimageResponse)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// Server API for ExternalPreimageService service

type ExternalPreimageServiceServer interface {
	GetPreimage(*GetPreimageRequest, ExternalPreimageService_GetPreimageServer) error
}

func RegisterExternalPreimageServiceServer(s *grpc.Server, srv ExternalPreimageServiceServer) {
	s.RegisterService(&_ExternalPreimageService_serviceDesc, srv)
}

func _ExternalPreimageService_GetPreimage_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(GetPreimageRequest)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(ExternalPreimageServiceServer).GetPreimage(m, &externalPreimageServiceGetPreimageServer{stream})
}

type ExternalPreimageService_GetPreimageServer interface {
	Send(*GetPreimageResponse) error
	grpc.ServerStream
}

type externalPreimageServiceGetPreimageServer struct {
	grpc.ServerStream
}

func (x *externalPreimageServiceGetPreimageServer) Send(m *GetPreimageResponse) error {
	return x.ServerStream.SendMsg(m)
}

var _ExternalPreimageService_serviceDesc = grpc.ServiceDesc{
	ServiceName: "extpreimage.ExternalPreimageService",
	HandlerType: (*ExternalPreimageServiceServer)(nil),
	Methods:     []grpc.MethodDesc{},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "GetPreimage",
			Handler:       _ExternalPreimageService_GetPreimage_Handler,
			ServerStreams: true,
		},
	},
	Metadata: "rpc.proto",
}

func init() { proto.RegisterFile("rpc.proto", fileDescriptor0) }

var fileDescriptor0 = []byte{
	// 280 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x74, 0x91, 0xc1, 0x4a, 0xc3, 0x40,
	0x10, 0x86, 0x5d, 0x8a, 0xd1, 0x4e, 0x8a, 0x86, 0x29, 0x68, 0xa8, 0x87, 0xc6, 0x9e, 0xa2, 0x42,
	0x90, 0xfa, 0x02, 0x62, 0x11, 0x7b, 0xe8, 0x41, 0xd2, 0xde, 0xc3, 0x26, 0x0c, 0x4d, 0x68, 0x36,
	0x1b, 0xb3, 0x5b, 0x69, 0x5f, 0xcd, 0xa7, 0x93, 0x6c, 0x37, 0xd0, 0x22, 0xde, 0x76, 0xbe, 0xf9,
	0x77, 0xff, 0x7f, 0x76, 0xa0, 0xdf, 0xd4, 0x59, 0x54, 0x37, 0x52, 0x4b, 0x74, 0x69, 0xa7, 0xeb,
	0x86, 0x0a, 0xc1, 0xd7, 0x34, 0xf9, 0x61, 0x80, 0x1f, 0xa4, 0x3f, 0x6d, 0x1d, 0xd3, 0xd7, 0x96,
	0x94, 0xc6, 0x7b, 0x18, 0xd4, 0x7c, 0x2f, 0xa8, 0xd2, 0x49, 0xce, 0x55, 0xee, 0xb3, 0x80, 0x85,
	0x83, 0xd8, 0xb5, 0x6c, 0xce, 0x55, 0x8e, 0x37, 0xe0, 0x70, 0x21, 0xb7, 0x95, 0xf6, 0xcf, 0x03,
	0x16, 0xf6, 0x62, 0x5b, 0xe1, 0x13, 0x38, 0x6a, 0x2f, 0x52, 0x59, 0xfa, 0x4e, 0xc0, 0xc2, 0xab,
	0xe9, 0x30, 0x3a, 0xf2, 0x8b, 0x96, 0xa6, 0x15, 0x5b, 0x09, 0xde, 0x41, 0x5f, 0x17, 0x82, 0x92,
	0x52, 0x66, 0x1b, 0x1f, 0xcc, 0x3b, 0x97, 0x2d, 0x58, 0xc8, 0x6c, 0x83, 0x63, 0x70, 0x53, 0x52,
	0x3a, 0xc9, 0xa9, 0x58, 0xe7, 0xda, 0x77, 0x4d, 0x1b, 0x5a, 0x34, 0x37, 0x64, 0xf2, 0x0a, 0xc3,
	0x93, 0xec, 0xaa, 0x96, 0x95, 0x22, 0x7c, 0x00, 0xaf, 0x0b, 0xdf, 0xf9, 0xda, 0x01, 0xae, 0x2d,
	0xef, 0xae, 0x3c, 0x8e, 0xc0, 0x39, 0x24, 0xc2, 0x0b, 0xe8, 0xbd, 0xad, 0x66, 0xde, 0x59, 0x7b,
	0x58, 0xac, 0x66, 0x1e, 0x9b, 0x0a, 0xb8, 0x7d, 0xdf, 0x69, 0x6a, 0x2a, 0x5e, 0x76, 0xfa, 0x25,
	0x35, 0xdf, 0x45, 0x46, 0x18, 0x83, 0x7b, 0x64, 0x8c, 0xe3, 0x93, 0x11, 0xff, 0x7e, 0xe7, 0x28,
	0xf8, 0x5f, 0x70, 0xc8, 0xfc, 0xcc, 0x52, 0xc7, 0x6c, 0xe7, 0xe5, 0x37, 0x00, 0x00, 0xff, 0xff,
	0xfd, 0xc1, 0xa2, 0x11, 0xaa, 0x01, 0x00, 0x00,
}
