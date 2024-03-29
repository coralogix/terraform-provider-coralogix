// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.28.1
// 	protoc        v3.21.8
// source: com/coralogixapis/events2metrics/v2/events2metrics_internal_service.proto

package __

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	l2m "terraform-provider-coralogix/coralogix/clientset/grpc/logs2metrics/v2"

	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type ListE2MRequestInternal struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *ListE2MRequestInternal) Reset() {
	*x = ListE2MRequestInternal{}
	if protoimpl.UnsafeEnabled {
		mi := &file_com_coralogixapis_events2metrics_v2_events2metrics_internal_service_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ListE2MRequestInternal) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ListE2MRequestInternal) ProtoMessage() {}

func (x *ListE2MRequestInternal) ProtoReflect() protoreflect.Message {
	mi := &file_com_coralogixapis_events2metrics_v2_events2metrics_internal_service_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ListE2MRequestInternal.ProtoReflect.Descriptor instead.
func (*ListE2MRequestInternal) Descriptor() ([]byte, []int) {
	return file_com_coralogixapis_events2metrics_v2_events2metrics_internal_service_proto_rawDescGZIP(), []int{0}
}

type ListE2MResponseInternal struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	E2M []*E2M `protobuf:"bytes,1,rep,name=e2m,proto3" json:"e2m,omitempty"`
}

func (x *ListE2MResponseInternal) Reset() {
	*x = ListE2MResponseInternal{}
	if protoimpl.UnsafeEnabled {
		mi := &file_com_coralogixapis_events2metrics_v2_events2metrics_internal_service_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ListE2MResponseInternal) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ListE2MResponseInternal) ProtoMessage() {}

func (x *ListE2MResponseInternal) ProtoReflect() protoreflect.Message {
	mi := &file_com_coralogixapis_events2metrics_v2_events2metrics_internal_service_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ListE2MResponseInternal.ProtoReflect.Descriptor instead.
func (*ListE2MResponseInternal) Descriptor() ([]byte, []int) {
	return file_com_coralogixapis_events2metrics_v2_events2metrics_internal_service_proto_rawDescGZIP(), []int{1}
}

func (x *ListE2MResponseInternal) GetE2M() []*E2M {
	if x != nil {
		return x.E2M
	}
	return nil
}

type CreateE2MRequestInternal struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	E2M *E2MCreateParams `protobuf:"bytes,1,opt,name=e2m,proto3" json:"e2m,omitempty"`
}

func (x *CreateE2MRequestInternal) Reset() {
	*x = CreateE2MRequestInternal{}
	if protoimpl.UnsafeEnabled {
		mi := &file_com_coralogixapis_events2metrics_v2_events2metrics_internal_service_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *CreateE2MRequestInternal) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CreateE2MRequestInternal) ProtoMessage() {}

func (x *CreateE2MRequestInternal) ProtoReflect() protoreflect.Message {
	mi := &file_com_coralogixapis_events2metrics_v2_events2metrics_internal_service_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CreateE2MRequestInternal.ProtoReflect.Descriptor instead.
func (*CreateE2MRequestInternal) Descriptor() ([]byte, []int) {
	return file_com_coralogixapis_events2metrics_v2_events2metrics_internal_service_proto_rawDescGZIP(), []int{2}
}

func (x *CreateE2MRequestInternal) GetE2M() *E2MCreateParams {
	if x != nil {
		return x.E2M
	}
	return nil
}

type CreateE2MResponseInternal struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	E2M *E2M `protobuf:"bytes,1,opt,name=e2m,proto3" json:"e2m,omitempty"`
}

func (x *CreateE2MResponseInternal) Reset() {
	*x = CreateE2MResponseInternal{}
	if protoimpl.UnsafeEnabled {
		mi := &file_com_coralogixapis_events2metrics_v2_events2metrics_internal_service_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *CreateE2MResponseInternal) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CreateE2MResponseInternal) ProtoMessage() {}

func (x *CreateE2MResponseInternal) ProtoReflect() protoreflect.Message {
	mi := &file_com_coralogixapis_events2metrics_v2_events2metrics_internal_service_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CreateE2MResponseInternal.ProtoReflect.Descriptor instead.
func (*CreateE2MResponseInternal) Descriptor() ([]byte, []int) {
	return file_com_coralogixapis_events2metrics_v2_events2metrics_internal_service_proto_rawDescGZIP(), []int{3}
}

func (x *CreateE2MResponseInternal) GetE2M() *E2M {
	if x != nil {
		return x.E2M
	}
	return nil
}

var File_com_coralogixapis_events2metrics_v2_events2metrics_internal_service_proto protoreflect.FileDescriptor

var file_com_coralogixapis_events2metrics_v2_events2metrics_internal_service_proto_rawDesc = []byte{
	0x0a, 0x49, 0x63, 0x6f, 0x6d, 0x2f, 0x63, 0x6f, 0x72, 0x61, 0x6c, 0x6f, 0x67, 0x69, 0x78, 0x61,
	0x70, 0x69, 0x73, 0x2f, 0x65, 0x76, 0x65, 0x6e, 0x74, 0x73, 0x32, 0x6d, 0x65, 0x74, 0x72, 0x69,
	0x63, 0x73, 0x2f, 0x76, 0x32, 0x2f, 0x65, 0x76, 0x65, 0x6e, 0x74, 0x73, 0x32, 0x6d, 0x65, 0x74,
	0x72, 0x69, 0x63, 0x73, 0x5f, 0x69, 0x6e, 0x74, 0x65, 0x72, 0x6e, 0x61, 0x6c, 0x5f, 0x73, 0x65,
	0x72, 0x76, 0x69, 0x63, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x23, 0x63, 0x6f, 0x6d,
	0x2e, 0x63, 0x6f, 0x72, 0x61, 0x6c, 0x6f, 0x67, 0x69, 0x78, 0x61, 0x70, 0x69, 0x73, 0x2e, 0x65,
	0x76, 0x65, 0x6e, 0x74, 0x73, 0x32, 0x6d, 0x65, 0x74, 0x72, 0x69, 0x63, 0x73, 0x2e, 0x76, 0x32,
	0x1a, 0x43, 0x63, 0x6f, 0x6d, 0x2f, 0x63, 0x6f, 0x72, 0x61, 0x6c, 0x6f, 0x67, 0x69, 0x78, 0x61,
	0x70, 0x69, 0x73, 0x2f, 0x65, 0x76, 0x65, 0x6e, 0x74, 0x73, 0x32, 0x6d, 0x65, 0x74, 0x72, 0x69,
	0x63, 0x73, 0x2f, 0x76, 0x32, 0x2f, 0x65, 0x76, 0x65, 0x6e, 0x74, 0x73, 0x32, 0x6d, 0x65, 0x74,
	0x72, 0x69, 0x63, 0x73, 0x5f, 0x64, 0x65, 0x66, 0x69, 0x6e, 0x69, 0x74, 0x69, 0x6f, 0x6e, 0x2e,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x31, 0x63, 0x6f, 0x6d, 0x2f, 0x63, 0x6f, 0x72, 0x61, 0x6c,
	0x6f, 0x67, 0x69, 0x78, 0x61, 0x70, 0x69, 0x73, 0x2f, 0x6c, 0x6f, 0x67, 0x73, 0x32, 0x6d, 0x65,
	0x74, 0x72, 0x69, 0x63, 0x73, 0x2f, 0x76, 0x32, 0x2f, 0x61, 0x75, 0x64, 0x69, 0x74, 0x5f, 0x6c,
	0x6f, 0x67, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x1c, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65,
	0x2f, 0x61, 0x70, 0x69, 0x2f, 0x61, 0x6e, 0x6e, 0x6f, 0x74, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x73,
	0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x18, 0x0a, 0x16, 0x4c, 0x69, 0x73, 0x74, 0x45, 0x32,
	0x4d, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x49, 0x6e, 0x74, 0x65, 0x72, 0x6e, 0x61, 0x6c,
	0x22, 0x55, 0x0a, 0x17, 0x4c, 0x69, 0x73, 0x74, 0x45, 0x32, 0x4d, 0x52, 0x65, 0x73, 0x70, 0x6f,
	0x6e, 0x73, 0x65, 0x49, 0x6e, 0x74, 0x65, 0x72, 0x6e, 0x61, 0x6c, 0x12, 0x3a, 0x0a, 0x03, 0x65,
	0x32, 0x6d, 0x18, 0x01, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x28, 0x2e, 0x63, 0x6f, 0x6d, 0x2e, 0x63,
	0x6f, 0x72, 0x61, 0x6c, 0x6f, 0x67, 0x69, 0x78, 0x61, 0x70, 0x69, 0x73, 0x2e, 0x65, 0x76, 0x65,
	0x6e, 0x74, 0x73, 0x32, 0x6d, 0x65, 0x74, 0x72, 0x69, 0x63, 0x73, 0x2e, 0x76, 0x32, 0x2e, 0x45,
	0x32, 0x4d, 0x52, 0x03, 0x65, 0x32, 0x6d, 0x22, 0x62, 0x0a, 0x18, 0x43, 0x72, 0x65, 0x61, 0x74,
	0x65, 0x45, 0x32, 0x4d, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x49, 0x6e, 0x74, 0x65, 0x72,
	0x6e, 0x61, 0x6c, 0x12, 0x46, 0x0a, 0x03, 0x65, 0x32, 0x6d, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b,
	0x32, 0x34, 0x2e, 0x63, 0x6f, 0x6d, 0x2e, 0x63, 0x6f, 0x72, 0x61, 0x6c, 0x6f, 0x67, 0x69, 0x78,
	0x61, 0x70, 0x69, 0x73, 0x2e, 0x65, 0x76, 0x65, 0x6e, 0x74, 0x73, 0x32, 0x6d, 0x65, 0x74, 0x72,
	0x69, 0x63, 0x73, 0x2e, 0x76, 0x32, 0x2e, 0x45, 0x32, 0x4d, 0x43, 0x72, 0x65, 0x61, 0x74, 0x65,
	0x50, 0x61, 0x72, 0x61, 0x6d, 0x73, 0x52, 0x03, 0x65, 0x32, 0x6d, 0x22, 0x57, 0x0a, 0x19, 0x43,
	0x72, 0x65, 0x61, 0x74, 0x65, 0x45, 0x32, 0x4d, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65,
	0x49, 0x6e, 0x74, 0x65, 0x72, 0x6e, 0x61, 0x6c, 0x12, 0x3a, 0x0a, 0x03, 0x65, 0x32, 0x6d, 0x18,
	0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x28, 0x2e, 0x63, 0x6f, 0x6d, 0x2e, 0x63, 0x6f, 0x72, 0x61,
	0x6c, 0x6f, 0x67, 0x69, 0x78, 0x61, 0x70, 0x69, 0x73, 0x2e, 0x65, 0x76, 0x65, 0x6e, 0x74, 0x73,
	0x32, 0x6d, 0x65, 0x74, 0x72, 0x69, 0x63, 0x73, 0x2e, 0x76, 0x32, 0x2e, 0x45, 0x32, 0x4d, 0x52,
	0x03, 0x65, 0x32, 0x6d, 0x32, 0xb6, 0x03, 0x0a, 0x1c, 0x45, 0x76, 0x65, 0x6e, 0x74, 0x73, 0x32,
	0x4d, 0x65, 0x74, 0x72, 0x69, 0x63, 0x49, 0x6e, 0x74, 0x65, 0x72, 0x6e, 0x61, 0x6c, 0x53, 0x65,
	0x72, 0x76, 0x69, 0x63, 0x65, 0x12, 0xc7, 0x01, 0x0a, 0x0f, 0x4c, 0x69, 0x73, 0x74, 0x45, 0x32,
	0x4d, 0x49, 0x6e, 0x74, 0x65, 0x72, 0x6e, 0x61, 0x6c, 0x12, 0x3b, 0x2e, 0x63, 0x6f, 0x6d, 0x2e,
	0x63, 0x6f, 0x72, 0x61, 0x6c, 0x6f, 0x67, 0x69, 0x78, 0x61, 0x70, 0x69, 0x73, 0x2e, 0x65, 0x76,
	0x65, 0x6e, 0x74, 0x73, 0x32, 0x6d, 0x65, 0x74, 0x72, 0x69, 0x63, 0x73, 0x2e, 0x76, 0x32, 0x2e,
	0x4c, 0x69, 0x73, 0x74, 0x45, 0x32, 0x4d, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x49, 0x6e,
	0x74, 0x65, 0x72, 0x6e, 0x61, 0x6c, 0x1a, 0x3c, 0x2e, 0x63, 0x6f, 0x6d, 0x2e, 0x63, 0x6f, 0x72,
	0x61, 0x6c, 0x6f, 0x67, 0x69, 0x78, 0x61, 0x70, 0x69, 0x73, 0x2e, 0x65, 0x76, 0x65, 0x6e, 0x74,
	0x73, 0x32, 0x6d, 0x65, 0x74, 0x72, 0x69, 0x63, 0x73, 0x2e, 0x76, 0x32, 0x2e, 0x4c, 0x69, 0x73,
	0x74, 0x45, 0x32, 0x4d, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x49, 0x6e, 0x74, 0x65,
	0x72, 0x6e, 0x61, 0x6c, 0x22, 0x39, 0xc2, 0xb8, 0x02, 0x17, 0x0a, 0x15, 0x4c, 0x69, 0x73, 0x74,
	0x20, 0x61, 0x6c, 0x6c, 0x20, 0x45, 0x32, 0x4d, 0x20, 0x69, 0x6e, 0x74, 0x65, 0x72, 0x6e, 0x61,
	0x6c, 0x82, 0xd3, 0xe4, 0x93, 0x02, 0x18, 0x12, 0x16, 0x2f, 0x61, 0x70, 0x69, 0x2f, 0x76, 0x32,
	0x2f, 0x65, 0x76, 0x65, 0x6e, 0x74, 0x73, 0x32, 0x6d, 0x65, 0x74, 0x72, 0x69, 0x63, 0x73, 0x12,
	0xcb, 0x01, 0x0a, 0x11, 0x43, 0x72, 0x65, 0x61, 0x74, 0x65, 0x45, 0x32, 0x4d, 0x49, 0x6e, 0x74,
	0x65, 0x72, 0x6e, 0x61, 0x6c, 0x12, 0x3d, 0x2e, 0x63, 0x6f, 0x6d, 0x2e, 0x63, 0x6f, 0x72, 0x61,
	0x6c, 0x6f, 0x67, 0x69, 0x78, 0x61, 0x70, 0x69, 0x73, 0x2e, 0x65, 0x76, 0x65, 0x6e, 0x74, 0x73,
	0x32, 0x6d, 0x65, 0x74, 0x72, 0x69, 0x63, 0x73, 0x2e, 0x76, 0x32, 0x2e, 0x43, 0x72, 0x65, 0x61,
	0x74, 0x65, 0x45, 0x32, 0x4d, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x49, 0x6e, 0x74, 0x65,
	0x72, 0x6e, 0x61, 0x6c, 0x1a, 0x3e, 0x2e, 0x63, 0x6f, 0x6d, 0x2e, 0x63, 0x6f, 0x72, 0x61, 0x6c,
	0x6f, 0x67, 0x69, 0x78, 0x61, 0x70, 0x69, 0x73, 0x2e, 0x65, 0x76, 0x65, 0x6e, 0x74, 0x73, 0x32,
	0x6d, 0x65, 0x74, 0x72, 0x69, 0x63, 0x73, 0x2e, 0x76, 0x32, 0x2e, 0x43, 0x72, 0x65, 0x61, 0x74,
	0x65, 0x45, 0x32, 0x4d, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x49, 0x6e, 0x74, 0x65,
	0x72, 0x6e, 0x61, 0x6c, 0x22, 0x37, 0xc2, 0xb8, 0x02, 0x10, 0x0a, 0x0e, 0x43, 0x72, 0x65, 0x61,
	0x74, 0x65, 0x20, 0x6e, 0x65, 0x77, 0x20, 0x45, 0x32, 0x4d, 0x82, 0xd3, 0xe4, 0x93, 0x02, 0x1d,
	0x22, 0x16, 0x2f, 0x61, 0x70, 0x69, 0x2f, 0x76, 0x32, 0x2f, 0x65, 0x76, 0x65, 0x6e, 0x74, 0x73,
	0x32, 0x6d, 0x65, 0x74, 0x72, 0x69, 0x63, 0x73, 0x3a, 0x03, 0x65, 0x32, 0x6d, 0x42, 0x04, 0x5a,
	0x02, 0x2e, 0x2f, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_com_coralogixapis_events2metrics_v2_events2metrics_internal_service_proto_rawDescOnce sync.Once
	file_com_coralogixapis_events2metrics_v2_events2metrics_internal_service_proto_rawDescData = file_com_coralogixapis_events2metrics_v2_events2metrics_internal_service_proto_rawDesc
)

func file_com_coralogixapis_events2metrics_v2_events2metrics_internal_service_proto_rawDescGZIP() []byte {
	file_com_coralogixapis_events2metrics_v2_events2metrics_internal_service_proto_rawDescOnce.Do(func() {
		file_com_coralogixapis_events2metrics_v2_events2metrics_internal_service_proto_rawDescData = protoimpl.X.CompressGZIP(file_com_coralogixapis_events2metrics_v2_events2metrics_internal_service_proto_rawDescData)
	})
	return file_com_coralogixapis_events2metrics_v2_events2metrics_internal_service_proto_rawDescData
}

var file_com_coralogixapis_events2metrics_v2_events2metrics_internal_service_proto_msgTypes = make([]protoimpl.MessageInfo, 4)
var file_com_coralogixapis_events2metrics_v2_events2metrics_internal_service_proto_goTypes = []interface{}{
	(*ListE2MRequestInternal)(nil),    // 0: com.coralogixapis.events2metrics.v2.ListE2MRequestInternal
	(*ListE2MResponseInternal)(nil),   // 1: com.coralogixapis.events2metrics.v2.ListE2MResponseInternal
	(*CreateE2MRequestInternal)(nil),  // 2: com.coralogixapis.events2metrics.v2.CreateE2MRequestInternal
	(*CreateE2MResponseInternal)(nil), // 3: com.coralogixapis.events2metrics.v2.CreateE2MResponseInternal
	(*E2M)(nil),                       // 4: com.coralogixapis.events2metrics.v2.E2M
	(*E2MCreateParams)(nil),           // 5: com.coralogixapis.events2metrics.v2.E2MCreateParams
}
var file_com_coralogixapis_events2metrics_v2_events2metrics_internal_service_proto_depIdxs = []int32{
	4, // 0: com.coralogixapis.events2metrics.v2.ListE2MResponseInternal.e2m:type_name -> com.coralogixapis.events2metrics.v2.E2M
	5, // 1: com.coralogixapis.events2metrics.v2.CreateE2MRequestInternal.e2m:type_name -> com.coralogixapis.events2metrics.v2.E2MCreateParams
	4, // 2: com.coralogixapis.events2metrics.v2.CreateE2MResponseInternal.e2m:type_name -> com.coralogixapis.events2metrics.v2.E2M
	0, // 3: com.coralogixapis.events2metrics.v2.Events2MetricInternalService.ListE2MInternal:input_type -> com.coralogixapis.events2metrics.v2.ListE2MRequestInternal
	2, // 4: com.coralogixapis.events2metrics.v2.Events2MetricInternalService.CreateE2MInternal:input_type -> com.coralogixapis.events2metrics.v2.CreateE2MRequestInternal
	1, // 5: com.coralogixapis.events2metrics.v2.Events2MetricInternalService.ListE2MInternal:output_type -> com.coralogixapis.events2metrics.v2.ListE2MResponseInternal
	3, // 6: com.coralogixapis.events2metrics.v2.Events2MetricInternalService.CreateE2MInternal:output_type -> com.coralogixapis.events2metrics.v2.CreateE2MResponseInternal
	5, // [5:7] is the sub-list for method output_type
	3, // [3:5] is the sub-list for method input_type
	3, // [3:3] is the sub-list for extension type_name
	3, // [3:3] is the sub-list for extension extendee
	0, // [0:3] is the sub-list for field type_name
}

func init() { file_com_coralogixapis_events2metrics_v2_events2metrics_internal_service_proto_init() }
func file_com_coralogixapis_events2metrics_v2_events2metrics_internal_service_proto_init() {
	if File_com_coralogixapis_events2metrics_v2_events2metrics_internal_service_proto != nil {
		return
	}
	file_com_coralogixapis_events2metrics_v2_events2metrics_definition_proto_init()
	l2m.File_com_coralogixapis_logs2metrics_v2_audit_log_proto_init()
	//file_google_api_annotations_proto_init()
	if !protoimpl.UnsafeEnabled {
		file_com_coralogixapis_events2metrics_v2_events2metrics_internal_service_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ListE2MRequestInternal); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_com_coralogixapis_events2metrics_v2_events2metrics_internal_service_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ListE2MResponseInternal); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_com_coralogixapis_events2metrics_v2_events2metrics_internal_service_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*CreateE2MRequestInternal); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_com_coralogixapis_events2metrics_v2_events2metrics_internal_service_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*CreateE2MResponseInternal); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_com_coralogixapis_events2metrics_v2_events2metrics_internal_service_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   4,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_com_coralogixapis_events2metrics_v2_events2metrics_internal_service_proto_goTypes,
		DependencyIndexes: file_com_coralogixapis_events2metrics_v2_events2metrics_internal_service_proto_depIdxs,
		MessageInfos:      file_com_coralogixapis_events2metrics_v2_events2metrics_internal_service_proto_msgTypes,
	}.Build()
	File_com_coralogixapis_events2metrics_v2_events2metrics_internal_service_proto = out.File
	file_com_coralogixapis_events2metrics_v2_events2metrics_internal_service_proto_rawDesc = nil
	file_com_coralogixapis_events2metrics_v2_events2metrics_internal_service_proto_goTypes = nil
	file_com_coralogixapis_events2metrics_v2_events2metrics_internal_service_proto_depIdxs = nil
}
