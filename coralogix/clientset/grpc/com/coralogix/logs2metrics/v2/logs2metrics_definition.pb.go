// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.28.1
// 	protoc        v3.21.8
// source: com/coralogixapis/logs2metrics/v2/logs2metrics_definition.proto

package __

import (
	reflect "reflect"
	sync "sync"

	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	_ "google.golang.org/protobuf/types/descriptorpb"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type L2M struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Id           string           `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
	Name         string           `protobuf:"bytes,2,opt,name=name,proto3" json:"name,omitempty"`
	Description  string           `protobuf:"bytes,3,opt,name=description,proto3" json:"description,omitempty"`
	Query        *LogsQuery       `protobuf:"bytes,5,opt,name=query,proto3" json:"query,omitempty"`
	CreateTime   string           `protobuf:"bytes,8,opt,name=create_time,json=createTime,proto3" json:"create_time,omitempty"`
	UpdateTime   string           `protobuf:"bytes,9,opt,name=update_time,json=updateTime,proto3" json:"update_time,omitempty"`
	Permutations *L2MPermutations `protobuf:"bytes,10,opt,name=permutations,proto3" json:"permutations,omitempty"`
	MetricLabels []*MetricLabel   `protobuf:"bytes,12,rep,name=metric_labels,json=metricLabels,proto3" json:"metric_labels,omitempty"`
	MetricFields []*MetricField   `protobuf:"bytes,13,rep,name=metric_fields,json=metricFields,proto3" json:"metric_fields,omitempty"`
}

func (x *L2M) Reset() {
	*x = L2M{}
	if protoimpl.UnsafeEnabled {
		mi := &file_com_coralogixapis_logs2metrics_v2_logs2metrics_definition_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *L2M) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*L2M) ProtoMessage() {}

func (x *L2M) ProtoReflect() protoreflect.Message {
	mi := &file_com_coralogixapis_logs2metrics_v2_logs2metrics_definition_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use L2M.ProtoReflect.Descriptor instead.
func (*L2M) Descriptor() ([]byte, []int) {
	return file_com_coralogixapis_logs2metrics_v2_logs2metrics_definition_proto_rawDescGZIP(), []int{0}
}

func (x *L2M) GetId() string {
	if x != nil {
		return x.Id
	}
	return ""
}

func (x *L2M) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *L2M) GetDescription() string {
	if x != nil {
		return x.Description
	}
	return ""
}

func (x *L2M) GetQuery() *LogsQuery {
	if x != nil {
		return x.Query
	}
	return nil
}

func (x *L2M) GetCreateTime() string {
	if x != nil {
		return x.CreateTime
	}
	return ""
}

func (x *L2M) GetUpdateTime() string {
	if x != nil {
		return x.UpdateTime
	}
	return ""
}

func (x *L2M) GetPermutations() *L2MPermutations {
	if x != nil {
		return x.Permutations
	}
	return nil
}

func (x *L2M) GetMetricLabels() []*MetricLabel {
	if x != nil {
		return x.MetricLabels
	}
	return nil
}

func (x *L2M) GetMetricFields() []*MetricField {
	if x != nil {
		return x.MetricFields
	}
	return nil
}

type L2MPermutations struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Limit            int32 `protobuf:"varint,1,opt,name=limit,proto3" json:"limit,omitempty"`
	HasExceededLimit bool  `protobuf:"varint,2,opt,name=has_exceeded_limit,json=hasExceededLimit,proto3" json:"has_exceeded_limit,omitempty"`
}

func (x *L2MPermutations) Reset() {
	*x = L2MPermutations{}
	if protoimpl.UnsafeEnabled {
		mi := &file_com_coralogixapis_logs2metrics_v2_logs2metrics_definition_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *L2MPermutations) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*L2MPermutations) ProtoMessage() {}

func (x *L2MPermutations) ProtoReflect() protoreflect.Message {
	mi := &file_com_coralogixapis_logs2metrics_v2_logs2metrics_definition_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use L2MPermutations.ProtoReflect.Descriptor instead.
func (*L2MPermutations) Descriptor() ([]byte, []int) {
	return file_com_coralogixapis_logs2metrics_v2_logs2metrics_definition_proto_rawDescGZIP(), []int{1}
}

func (x *L2MPermutations) GetLimit() int32 {
	if x != nil {
		return x.Limit
	}
	return 0
}

func (x *L2MPermutations) GetHasExceededLimit() bool {
	if x != nil {
		return x.HasExceededLimit
	}
	return false
}

type MetricLabel struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	TargetLabel string `protobuf:"bytes,1,opt,name=target_label,json=targetLabel,proto3" json:"target_label,omitempty"`
	SourceField string `protobuf:"bytes,2,opt,name=source_field,json=sourceField,proto3" json:"source_field,omitempty"`
}

func (x *MetricLabel) Reset() {
	*x = MetricLabel{}
	if protoimpl.UnsafeEnabled {
		mi := &file_com_coralogixapis_logs2metrics_v2_logs2metrics_definition_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *MetricLabel) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*MetricLabel) ProtoMessage() {}

func (x *MetricLabel) ProtoReflect() protoreflect.Message {
	mi := &file_com_coralogixapis_logs2metrics_v2_logs2metrics_definition_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use MetricLabel.ProtoReflect.Descriptor instead.
func (*MetricLabel) Descriptor() ([]byte, []int) {
	return file_com_coralogixapis_logs2metrics_v2_logs2metrics_definition_proto_rawDescGZIP(), []int{2}
}

func (x *MetricLabel) GetTargetLabel() string {
	if x != nil {
		return x.TargetLabel
	}
	return ""
}

func (x *MetricLabel) GetSourceField() string {
	if x != nil {
		return x.SourceField
	}
	return ""
}

type MetricField struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	TargetBaseMetricName string `protobuf:"bytes,1,opt,name=target_base_metric_name,json=targetBaseMetricName,proto3" json:"target_base_metric_name,omitempty"`
	SourceField          string `protobuf:"bytes,2,opt,name=source_field,json=sourceField,proto3" json:"source_field,omitempty"`
}

func (x *MetricField) Reset() {
	*x = MetricField{}
	if protoimpl.UnsafeEnabled {
		mi := &file_com_coralogixapis_logs2metrics_v2_logs2metrics_definition_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *MetricField) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*MetricField) ProtoMessage() {}

func (x *MetricField) ProtoReflect() protoreflect.Message {
	mi := &file_com_coralogixapis_logs2metrics_v2_logs2metrics_definition_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use MetricField.ProtoReflect.Descriptor instead.
func (*MetricField) Descriptor() ([]byte, []int) {
	return file_com_coralogixapis_logs2metrics_v2_logs2metrics_definition_proto_rawDescGZIP(), []int{3}
}

func (x *MetricField) GetTargetBaseMetricName() string {
	if x != nil {
		return x.TargetBaseMetricName
	}
	return ""
}

func (x *MetricField) GetSourceField() string {
	if x != nil {
		return x.SourceField
	}
	return ""
}

var File_com_coralogixapis_logs2metrics_v2_logs2metrics_definition_proto protoreflect.FileDescriptor

var file_com_coralogixapis_logs2metrics_v2_logs2metrics_definition_proto_rawDesc = []byte{
	0x0a, 0x3f, 0x63, 0x6f, 0x6d, 0x2f, 0x63, 0x6f, 0x72, 0x61, 0x6c, 0x6f, 0x67, 0x69, 0x78, 0x61,
	0x70, 0x69, 0x73, 0x2f, 0x6c, 0x6f, 0x67, 0x73, 0x32, 0x6d, 0x65, 0x74, 0x72, 0x69, 0x63, 0x73,
	0x2f, 0x76, 0x32, 0x2f, 0x6c, 0x6f, 0x67, 0x73, 0x32, 0x6d, 0x65, 0x74, 0x72, 0x69, 0x63, 0x73,
	0x5f, 0x64, 0x65, 0x66, 0x69, 0x6e, 0x69, 0x74, 0x69, 0x6f, 0x6e, 0x2e, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x12, 0x21, 0x63, 0x6f, 0x6d, 0x2e, 0x63, 0x6f, 0x72, 0x61, 0x6c, 0x6f, 0x67, 0x69, 0x78,
	0x61, 0x70, 0x69, 0x73, 0x2e, 0x6c, 0x6f, 0x67, 0x73, 0x32, 0x6d, 0x65, 0x74, 0x72, 0x69, 0x63,
	0x73, 0x2e, 0x76, 0x32, 0x1a, 0x20, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2f, 0x70, 0x72, 0x6f,
	0x74, 0x6f, 0x62, 0x75, 0x66, 0x2f, 0x64, 0x65, 0x73, 0x63, 0x72, 0x69, 0x70, 0x74, 0x6f, 0x72,
	0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x32, 0x63, 0x6f, 0x6d, 0x2f, 0x63, 0x6f, 0x72, 0x61,
	0x6c, 0x6f, 0x67, 0x69, 0x78, 0x61, 0x70, 0x69, 0x73, 0x2f, 0x6c, 0x6f, 0x67, 0x73, 0x32, 0x6d,
	0x65, 0x74, 0x72, 0x69, 0x63, 0x73, 0x2f, 0x76, 0x32, 0x2f, 0x6c, 0x6f, 0x67, 0x73, 0x5f, 0x71,
	0x75, 0x65, 0x72, 0x79, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0xd3, 0x03, 0x0a, 0x03, 0x4c,
	0x32, 0x4d, 0x12, 0x0e, 0x0a, 0x02, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x02,
	0x69, 0x64, 0x12, 0x12, 0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09,
	0x52, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x12, 0x20, 0x0a, 0x0b, 0x64, 0x65, 0x73, 0x63, 0x72, 0x69,
	0x70, 0x74, 0x69, 0x6f, 0x6e, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0b, 0x64, 0x65, 0x73,
	0x63, 0x72, 0x69, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x12, 0x42, 0x0a, 0x05, 0x71, 0x75, 0x65, 0x72,
	0x79, 0x18, 0x05, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x2c, 0x2e, 0x63, 0x6f, 0x6d, 0x2e, 0x63, 0x6f,
	0x72, 0x61, 0x6c, 0x6f, 0x67, 0x69, 0x78, 0x61, 0x70, 0x69, 0x73, 0x2e, 0x6c, 0x6f, 0x67, 0x73,
	0x32, 0x6d, 0x65, 0x74, 0x72, 0x69, 0x63, 0x73, 0x2e, 0x76, 0x32, 0x2e, 0x4c, 0x6f, 0x67, 0x73,
	0x51, 0x75, 0x65, 0x72, 0x79, 0x52, 0x05, 0x71, 0x75, 0x65, 0x72, 0x79, 0x12, 0x1f, 0x0a, 0x0b,
	0x63, 0x72, 0x65, 0x61, 0x74, 0x65, 0x5f, 0x74, 0x69, 0x6d, 0x65, 0x18, 0x08, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x0a, 0x63, 0x72, 0x65, 0x61, 0x74, 0x65, 0x54, 0x69, 0x6d, 0x65, 0x12, 0x1f, 0x0a,
	0x0b, 0x75, 0x70, 0x64, 0x61, 0x74, 0x65, 0x5f, 0x74, 0x69, 0x6d, 0x65, 0x18, 0x09, 0x20, 0x01,
	0x28, 0x09, 0x52, 0x0a, 0x75, 0x70, 0x64, 0x61, 0x74, 0x65, 0x54, 0x69, 0x6d, 0x65, 0x12, 0x56,
	0x0a, 0x0c, 0x70, 0x65, 0x72, 0x6d, 0x75, 0x74, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x18, 0x0a,
	0x20, 0x01, 0x28, 0x0b, 0x32, 0x32, 0x2e, 0x63, 0x6f, 0x6d, 0x2e, 0x63, 0x6f, 0x72, 0x61, 0x6c,
	0x6f, 0x67, 0x69, 0x78, 0x61, 0x70, 0x69, 0x73, 0x2e, 0x6c, 0x6f, 0x67, 0x73, 0x32, 0x6d, 0x65,
	0x74, 0x72, 0x69, 0x63, 0x73, 0x2e, 0x76, 0x32, 0x2e, 0x4c, 0x32, 0x4d, 0x50, 0x65, 0x72, 0x6d,
	0x75, 0x74, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x52, 0x0c, 0x70, 0x65, 0x72, 0x6d, 0x75, 0x74,
	0x61, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x12, 0x53, 0x0a, 0x0d, 0x6d, 0x65, 0x74, 0x72, 0x69, 0x63,
	0x5f, 0x6c, 0x61, 0x62, 0x65, 0x6c, 0x73, 0x18, 0x0c, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x2e, 0x2e,
	0x63, 0x6f, 0x6d, 0x2e, 0x63, 0x6f, 0x72, 0x61, 0x6c, 0x6f, 0x67, 0x69, 0x78, 0x61, 0x70, 0x69,
	0x73, 0x2e, 0x6c, 0x6f, 0x67, 0x73, 0x32, 0x6d, 0x65, 0x74, 0x72, 0x69, 0x63, 0x73, 0x2e, 0x76,
	0x32, 0x2e, 0x4d, 0x65, 0x74, 0x72, 0x69, 0x63, 0x4c, 0x61, 0x62, 0x65, 0x6c, 0x52, 0x0c, 0x6d,
	0x65, 0x74, 0x72, 0x69, 0x63, 0x4c, 0x61, 0x62, 0x65, 0x6c, 0x73, 0x12, 0x53, 0x0a, 0x0d, 0x6d,
	0x65, 0x74, 0x72, 0x69, 0x63, 0x5f, 0x66, 0x69, 0x65, 0x6c, 0x64, 0x73, 0x18, 0x0d, 0x20, 0x03,
	0x28, 0x0b, 0x32, 0x2e, 0x2e, 0x63, 0x6f, 0x6d, 0x2e, 0x63, 0x6f, 0x72, 0x61, 0x6c, 0x6f, 0x67,
	0x69, 0x78, 0x61, 0x70, 0x69, 0x73, 0x2e, 0x6c, 0x6f, 0x67, 0x73, 0x32, 0x6d, 0x65, 0x74, 0x72,
	0x69, 0x63, 0x73, 0x2e, 0x76, 0x32, 0x2e, 0x4d, 0x65, 0x74, 0x72, 0x69, 0x63, 0x46, 0x69, 0x65,
	0x6c, 0x64, 0x52, 0x0c, 0x6d, 0x65, 0x74, 0x72, 0x69, 0x63, 0x46, 0x69, 0x65, 0x6c, 0x64, 0x73,
	0x22, 0x55, 0x0a, 0x0f, 0x4c, 0x32, 0x4d, 0x50, 0x65, 0x72, 0x6d, 0x75, 0x74, 0x61, 0x74, 0x69,
	0x6f, 0x6e, 0x73, 0x12, 0x14, 0x0a, 0x05, 0x6c, 0x69, 0x6d, 0x69, 0x74, 0x18, 0x01, 0x20, 0x01,
	0x28, 0x05, 0x52, 0x05, 0x6c, 0x69, 0x6d, 0x69, 0x74, 0x12, 0x2c, 0x0a, 0x12, 0x68, 0x61, 0x73,
	0x5f, 0x65, 0x78, 0x63, 0x65, 0x65, 0x64, 0x65, 0x64, 0x5f, 0x6c, 0x69, 0x6d, 0x69, 0x74, 0x18,
	0x02, 0x20, 0x01, 0x28, 0x08, 0x52, 0x10, 0x68, 0x61, 0x73, 0x45, 0x78, 0x63, 0x65, 0x65, 0x64,
	0x65, 0x64, 0x4c, 0x69, 0x6d, 0x69, 0x74, 0x22, 0x53, 0x0a, 0x0b, 0x4d, 0x65, 0x74, 0x72, 0x69,
	0x63, 0x4c, 0x61, 0x62, 0x65, 0x6c, 0x12, 0x21, 0x0a, 0x0c, 0x74, 0x61, 0x72, 0x67, 0x65, 0x74,
	0x5f, 0x6c, 0x61, 0x62, 0x65, 0x6c, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0b, 0x74, 0x61,
	0x72, 0x67, 0x65, 0x74, 0x4c, 0x61, 0x62, 0x65, 0x6c, 0x12, 0x21, 0x0a, 0x0c, 0x73, 0x6f, 0x75,
	0x72, 0x63, 0x65, 0x5f, 0x66, 0x69, 0x65, 0x6c, 0x64, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52,
	0x0b, 0x73, 0x6f, 0x75, 0x72, 0x63, 0x65, 0x46, 0x69, 0x65, 0x6c, 0x64, 0x22, 0x67, 0x0a, 0x0b,
	0x4d, 0x65, 0x74, 0x72, 0x69, 0x63, 0x46, 0x69, 0x65, 0x6c, 0x64, 0x12, 0x35, 0x0a, 0x17, 0x74,
	0x61, 0x72, 0x67, 0x65, 0x74, 0x5f, 0x62, 0x61, 0x73, 0x65, 0x5f, 0x6d, 0x65, 0x74, 0x72, 0x69,
	0x63, 0x5f, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x14, 0x74, 0x61,
	0x72, 0x67, 0x65, 0x74, 0x42, 0x61, 0x73, 0x65, 0x4d, 0x65, 0x74, 0x72, 0x69, 0x63, 0x4e, 0x61,
	0x6d, 0x65, 0x12, 0x21, 0x0a, 0x0c, 0x73, 0x6f, 0x75, 0x72, 0x63, 0x65, 0x5f, 0x66, 0x69, 0x65,
	0x6c, 0x64, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0b, 0x73, 0x6f, 0x75, 0x72, 0x63, 0x65,
	0x46, 0x69, 0x65, 0x6c, 0x64, 0x42, 0x04, 0x5a, 0x02, 0x2e, 0x2f, 0x62, 0x06, 0x70, 0x72, 0x6f,
	0x74, 0x6f, 0x33,
}

var (
	file_com_coralogixapis_logs2metrics_v2_logs2metrics_definition_proto_rawDescOnce sync.Once
	file_com_coralogixapis_logs2metrics_v2_logs2metrics_definition_proto_rawDescData = file_com_coralogixapis_logs2metrics_v2_logs2metrics_definition_proto_rawDesc
)

func file_com_coralogixapis_logs2metrics_v2_logs2metrics_definition_proto_rawDescGZIP() []byte {
	file_com_coralogixapis_logs2metrics_v2_logs2metrics_definition_proto_rawDescOnce.Do(func() {
		file_com_coralogixapis_logs2metrics_v2_logs2metrics_definition_proto_rawDescData = protoimpl.X.CompressGZIP(file_com_coralogixapis_logs2metrics_v2_logs2metrics_definition_proto_rawDescData)
	})
	return file_com_coralogixapis_logs2metrics_v2_logs2metrics_definition_proto_rawDescData
}

var file_com_coralogixapis_logs2metrics_v2_logs2metrics_definition_proto_msgTypes = make([]protoimpl.MessageInfo, 4)
var file_com_coralogixapis_logs2metrics_v2_logs2metrics_definition_proto_goTypes = []interface{}{
	(*L2M)(nil),             // 0: com.coralogixapis.logs2metrics.v2.L2M
	(*L2MPermutations)(nil), // 1: com.coralogixapis.logs2metrics.v2.L2MPermutations
	(*MetricLabel)(nil),     // 2: com.coralogixapis.logs2metrics.v2.MetricLabel
	(*MetricField)(nil),     // 3: com.coralogixapis.logs2metrics.v2.MetricField
	(*LogsQuery)(nil),       // 4: com.coralogixapis.logs2metrics.v2.LogsQuery
}
var file_com_coralogixapis_logs2metrics_v2_logs2metrics_definition_proto_depIdxs = []int32{
	4, // 0: com.coralogixapis.logs2metrics.v2.L2M.query:type_name -> com.coralogixapis.logs2metrics.v2.LogsQuery
	1, // 1: com.coralogixapis.logs2metrics.v2.L2M.permutations:type_name -> com.coralogixapis.logs2metrics.v2.L2MPermutations
	2, // 2: com.coralogixapis.logs2metrics.v2.L2M.metric_labels:type_name -> com.coralogixapis.logs2metrics.v2.MetricLabel
	3, // 3: com.coralogixapis.logs2metrics.v2.L2M.metric_fields:type_name -> com.coralogixapis.logs2metrics.v2.MetricField
	4, // [4:4] is the sub-list for method output_type
	4, // [4:4] is the sub-list for method input_type
	4, // [4:4] is the sub-list for extension type_name
	4, // [4:4] is the sub-list for extension extendee
	0, // [0:4] is the sub-list for field type_name
}

func init() { file_com_coralogixapis_logs2metrics_v2_logs2metrics_definition_proto_init() }
func file_com_coralogixapis_logs2metrics_v2_logs2metrics_definition_proto_init() {
	if File_com_coralogixapis_logs2metrics_v2_logs2metrics_definition_proto != nil {
		return
	}
	file_com_coralogixapis_logs2metrics_v2_logs_query_proto_init()
	if !protoimpl.UnsafeEnabled {
		file_com_coralogixapis_logs2metrics_v2_logs2metrics_definition_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*L2M); i {
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
		file_com_coralogixapis_logs2metrics_v2_logs2metrics_definition_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*L2MPermutations); i {
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
		file_com_coralogixapis_logs2metrics_v2_logs2metrics_definition_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*MetricLabel); i {
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
		file_com_coralogixapis_logs2metrics_v2_logs2metrics_definition_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*MetricField); i {
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
			RawDescriptor: file_com_coralogixapis_logs2metrics_v2_logs2metrics_definition_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   4,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_com_coralogixapis_logs2metrics_v2_logs2metrics_definition_proto_goTypes,
		DependencyIndexes: file_com_coralogixapis_logs2metrics_v2_logs2metrics_definition_proto_depIdxs,
		MessageInfos:      file_com_coralogixapis_logs2metrics_v2_logs2metrics_definition_proto_msgTypes,
	}.Build()
	File_com_coralogixapis_logs2metrics_v2_logs2metrics_definition_proto = out.File
	file_com_coralogixapis_logs2metrics_v2_logs2metrics_definition_proto_rawDesc = nil
	file_com_coralogixapis_logs2metrics_v2_logs2metrics_definition_proto_goTypes = nil
	file_com_coralogixapis_logs2metrics_v2_logs2metrics_definition_proto_depIdxs = nil
}