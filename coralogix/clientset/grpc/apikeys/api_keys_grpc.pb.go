// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.2.0
// - protoc             v4.25.1
// source: api_keys.proto

package __

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

// ApiKeysServiceClient is the client API for ApiKeysService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type ApiKeysServiceClient interface {
	CreateApiKey(ctx context.Context, in *CreateApiKeyRequest, opts ...grpc.CallOption) (*CreateApiKeyResponse, error)
	GetApiKey(ctx context.Context, in *GetApiKeyRequest, opts ...grpc.CallOption) (*GetApiKeyResponse, error)
}

type apiKeysServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewApiKeysServiceClient(cc grpc.ClientConnInterface) ApiKeysServiceClient {
	return &apiKeysServiceClient{cc}
}

func (c *apiKeysServiceClient) CreateApiKey(ctx context.Context, in *CreateApiKeyRequest, opts ...grpc.CallOption) (*CreateApiKeyResponse, error) {
	out := new(CreateApiKeyResponse)
	err := c.cc.Invoke(ctx, "/com.coralogixapis.aaa.apikeys.v2.ApiKeysService/CreateApiKey", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *apiKeysServiceClient) GetApiKey(ctx context.Context, in *GetApiKeyRequest, opts ...grpc.CallOption) (*GetApiKeyResponse, error) {
	out := new(GetApiKeyResponse)
	err := c.cc.Invoke(ctx, "/com.coralogixapis.aaa.apikeys.v2.ApiKeysService/GetApiKey", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ApiKeysServiceServer is the server API for ApiKeysService service.
// All implementations must embed UnimplementedApiKeysServiceServer
// for forward compatibility
type ApiKeysServiceServer interface {
	CreateApiKey(context.Context, *CreateApiKeyRequest) (*CreateApiKeyResponse, error)
	GetApiKey(context.Context, *GetApiKeyRequest) (*GetApiKeyResponse, error)
	mustEmbedUnimplementedApiKeysServiceServer()
}

// UnimplementedApiKeysServiceServer must be embedded to have forward compatible implementations.
type UnimplementedApiKeysServiceServer struct {
}

func (UnimplementedApiKeysServiceServer) CreateApiKey(context.Context, *CreateApiKeyRequest) (*CreateApiKeyResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CreateApiKey not implemented")
}
func (UnimplementedApiKeysServiceServer) GetApiKey(context.Context, *GetApiKeyRequest) (*GetApiKeyResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetApiKey not implemented")
}
func (UnimplementedApiKeysServiceServer) mustEmbedUnimplementedApiKeysServiceServer() {}

// UnsafeApiKeysServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to ApiKeysServiceServer will
// result in compilation errors.
type UnsafeApiKeysServiceServer interface {
	mustEmbedUnimplementedApiKeysServiceServer()
}

func RegisterApiKeysServiceServer(s grpc.ServiceRegistrar, srv ApiKeysServiceServer) {
	s.RegisterService(&ApiKeysService_ServiceDesc, srv)
}

func _ApiKeysService_CreateApiKey_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(CreateApiKeyRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ApiKeysServiceServer).CreateApiKey(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/com.coralogixapis.aaa.apikeys.v2.ApiKeysService/CreateApiKey",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ApiKeysServiceServer).CreateApiKey(ctx, req.(*CreateApiKeyRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _ApiKeysService_GetApiKey_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetApiKeyRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ApiKeysServiceServer).GetApiKey(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/com.coralogixapis.aaa.apikeys.v2.ApiKeysService/GetApiKey",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ApiKeysServiceServer).GetApiKey(ctx, req.(*GetApiKeyRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// ApiKeysService_ServiceDesc is the grpc.ServiceDesc for ApiKeysService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var ApiKeysService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "com.coralogixapis.aaa.apikeys.v2.ApiKeysService",
	HandlerType: (*ApiKeysServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "CreateApiKey",
			Handler:    _ApiKeysService_CreateApiKey_Handler,
		},
		{
			MethodName: "GetApiKey",
			Handler:    _ApiKeysService_GetApiKey_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "api_keys.proto",
}