// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.34.2
// 	protoc        v5.28.2
// source: com/coralogixapis/dashboards/v1/ast/widgets/common/thresholds.proto

package v1

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	wrapperspb "google.golang.org/protobuf/types/known/wrapperspb"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type ThresholdType int32

const (
	ThresholdType_THRESHOLD_TYPE_UNSPECIFIED ThresholdType = 0
	ThresholdType_THRESHOLD_TYPE_RELATIVE    ThresholdType = 1
	ThresholdType_THRESHOLD_TYPE_ABSOLUTE    ThresholdType = 2
)

// Enum value maps for ThresholdType.
var (
	ThresholdType_name = map[int32]string{
		0: "THRESHOLD_TYPE_UNSPECIFIED",
		1: "THRESHOLD_TYPE_RELATIVE",
		2: "THRESHOLD_TYPE_ABSOLUTE",
	}
	ThresholdType_value = map[string]int32{
		"THRESHOLD_TYPE_UNSPECIFIED": 0,
		"THRESHOLD_TYPE_RELATIVE":    1,
		"THRESHOLD_TYPE_ABSOLUTE":    2,
	}
)

func (x ThresholdType) Enum() *ThresholdType {
	p := new(ThresholdType)
	*p = x
	return p
}

func (x ThresholdType) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (ThresholdType) Descriptor() protoreflect.EnumDescriptor {
	return file_com_coralogixapis_dashboards_v1_ast_widgets_common_thresholds_proto_enumTypes[0].Descriptor()
}

func (ThresholdType) Type() protoreflect.EnumType {
	return &file_com_coralogixapis_dashboards_v1_ast_widgets_common_thresholds_proto_enumTypes[0]
}

func (x ThresholdType) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use ThresholdType.Descriptor instead.
func (ThresholdType) EnumDescriptor() ([]byte, []int) {
	return file_com_coralogixapis_dashboards_v1_ast_widgets_common_thresholds_proto_rawDescGZIP(), []int{0}
}

type ThresholdBy int32

const (
	ThresholdBy_THRESHOLD_BY_UNSPECIFIED ThresholdBy = 0
	ThresholdBy_THRESHOLD_BY_VALUE       ThresholdBy = 1
	ThresholdBy_THRESHOLD_BY_BACKGROUND  ThresholdBy = 2
)

// Enum value maps for ThresholdBy.
var (
	ThresholdBy_name = map[int32]string{
		0: "THRESHOLD_BY_UNSPECIFIED",
		1: "THRESHOLD_BY_VALUE",
		2: "THRESHOLD_BY_BACKGROUND",
	}
	ThresholdBy_value = map[string]int32{
		"THRESHOLD_BY_UNSPECIFIED": 0,
		"THRESHOLD_BY_VALUE":       1,
		"THRESHOLD_BY_BACKGROUND":  2,
	}
)

func (x ThresholdBy) Enum() *ThresholdBy {
	p := new(ThresholdBy)
	*p = x
	return p
}

func (x ThresholdBy) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (ThresholdBy) Descriptor() protoreflect.EnumDescriptor {
	return file_com_coralogixapis_dashboards_v1_ast_widgets_common_thresholds_proto_enumTypes[1].Descriptor()
}

func (ThresholdBy) Type() protoreflect.EnumType {
	return &file_com_coralogixapis_dashboards_v1_ast_widgets_common_thresholds_proto_enumTypes[1]
}

func (x ThresholdBy) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use ThresholdBy.Descriptor instead.
func (ThresholdBy) EnumDescriptor() ([]byte, []int) {
	return file_com_coralogixapis_dashboards_v1_ast_widgets_common_thresholds_proto_rawDescGZIP(), []int{1}
}

type Threshold struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	From  *wrapperspb.DoubleValue `protobuf:"bytes,1,opt,name=from,proto3" json:"from,omitempty"`
	Color *wrapperspb.StringValue `protobuf:"bytes,2,opt,name=color,proto3" json:"color,omitempty"`
}

func (x *Threshold) Reset() {
	*x = Threshold{}
	if protoimpl.UnsafeEnabled {
		mi := &file_com_coralogixapis_dashboards_v1_ast_widgets_common_thresholds_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Threshold) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Threshold) ProtoMessage() {}

func (x *Threshold) ProtoReflect() protoreflect.Message {
	mi := &file_com_coralogixapis_dashboards_v1_ast_widgets_common_thresholds_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Threshold.ProtoReflect.Descriptor instead.
func (*Threshold) Descriptor() ([]byte, []int) {
	return file_com_coralogixapis_dashboards_v1_ast_widgets_common_thresholds_proto_rawDescGZIP(), []int{0}
}

func (x *Threshold) GetFrom() *wrapperspb.DoubleValue {
	if x != nil {
		return x.From
	}
	return nil
}

func (x *Threshold) GetColor() *wrapperspb.StringValue {
	if x != nil {
		return x.Color
	}
	return nil
}

var File_com_coralogixapis_dashboards_v1_ast_widgets_common_thresholds_proto protoreflect.FileDescriptor

var file_com_coralogixapis_dashboards_v1_ast_widgets_common_thresholds_proto_rawDesc = []byte{
	0x0a, 0x43, 0x63, 0x6f, 0x6d, 0x2f, 0x63, 0x6f, 0x72, 0x61, 0x6c, 0x6f, 0x67, 0x69, 0x78, 0x61,
	0x70, 0x69, 0x73, 0x2f, 0x64, 0x61, 0x73, 0x68, 0x62, 0x6f, 0x61, 0x72, 0x64, 0x73, 0x2f, 0x76,
	0x31, 0x2f, 0x61, 0x73, 0x74, 0x2f, 0x77, 0x69, 0x64, 0x67, 0x65, 0x74, 0x73, 0x2f, 0x63, 0x6f,
	0x6d, 0x6d, 0x6f, 0x6e, 0x2f, 0x74, 0x68, 0x72, 0x65, 0x73, 0x68, 0x6f, 0x6c, 0x64, 0x73, 0x2e,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x32, 0x63, 0x6f, 0x6d, 0x2e, 0x63, 0x6f, 0x72, 0x61, 0x6c,
	0x6f, 0x67, 0x69, 0x78, 0x61, 0x70, 0x69, 0x73, 0x2e, 0x64, 0x61, 0x73, 0x68, 0x62, 0x6f, 0x61,
	0x72, 0x64, 0x73, 0x2e, 0x76, 0x31, 0x2e, 0x61, 0x73, 0x74, 0x2e, 0x77, 0x69, 0x64, 0x67, 0x65,
	0x74, 0x73, 0x2e, 0x63, 0x6f, 0x6d, 0x6d, 0x6f, 0x6e, 0x1a, 0x1e, 0x67, 0x6f, 0x6f, 0x67, 0x6c,
	0x65, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2f, 0x77, 0x72, 0x61, 0x70, 0x70,
	0x65, 0x72, 0x73, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x71, 0x0a, 0x09, 0x54, 0x68, 0x72,
	0x65, 0x73, 0x68, 0x6f, 0x6c, 0x64, 0x12, 0x30, 0x0a, 0x04, 0x66, 0x72, 0x6f, 0x6d, 0x18, 0x01,
	0x20, 0x01, 0x28, 0x0b, 0x32, 0x1c, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x44, 0x6f, 0x75, 0x62, 0x6c, 0x65, 0x56, 0x61, 0x6c,
	0x75, 0x65, 0x52, 0x04, 0x66, 0x72, 0x6f, 0x6d, 0x12, 0x32, 0x0a, 0x05, 0x63, 0x6f, 0x6c, 0x6f,
	0x72, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1c, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65,
	0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x53, 0x74, 0x72, 0x69, 0x6e, 0x67,
	0x56, 0x61, 0x6c, 0x75, 0x65, 0x52, 0x05, 0x63, 0x6f, 0x6c, 0x6f, 0x72, 0x2a, 0x69, 0x0a, 0x0d,
	0x54, 0x68, 0x72, 0x65, 0x73, 0x68, 0x6f, 0x6c, 0x64, 0x54, 0x79, 0x70, 0x65, 0x12, 0x1e, 0x0a,
	0x1a, 0x54, 0x48, 0x52, 0x45, 0x53, 0x48, 0x4f, 0x4c, 0x44, 0x5f, 0x54, 0x59, 0x50, 0x45, 0x5f,
	0x55, 0x4e, 0x53, 0x50, 0x45, 0x43, 0x49, 0x46, 0x49, 0x45, 0x44, 0x10, 0x00, 0x12, 0x1b, 0x0a,
	0x17, 0x54, 0x48, 0x52, 0x45, 0x53, 0x48, 0x4f, 0x4c, 0x44, 0x5f, 0x54, 0x59, 0x50, 0x45, 0x5f,
	0x52, 0x45, 0x4c, 0x41, 0x54, 0x49, 0x56, 0x45, 0x10, 0x01, 0x12, 0x1b, 0x0a, 0x17, 0x54, 0x48,
	0x52, 0x45, 0x53, 0x48, 0x4f, 0x4c, 0x44, 0x5f, 0x54, 0x59, 0x50, 0x45, 0x5f, 0x41, 0x42, 0x53,
	0x4f, 0x4c, 0x55, 0x54, 0x45, 0x10, 0x02, 0x2a, 0x60, 0x0a, 0x0b, 0x54, 0x68, 0x72, 0x65, 0x73,
	0x68, 0x6f, 0x6c, 0x64, 0x42, 0x79, 0x12, 0x1c, 0x0a, 0x18, 0x54, 0x48, 0x52, 0x45, 0x53, 0x48,
	0x4f, 0x4c, 0x44, 0x5f, 0x42, 0x59, 0x5f, 0x55, 0x4e, 0x53, 0x50, 0x45, 0x43, 0x49, 0x46, 0x49,
	0x45, 0x44, 0x10, 0x00, 0x12, 0x16, 0x0a, 0x12, 0x54, 0x48, 0x52, 0x45, 0x53, 0x48, 0x4f, 0x4c,
	0x44, 0x5f, 0x42, 0x59, 0x5f, 0x56, 0x41, 0x4c, 0x55, 0x45, 0x10, 0x01, 0x12, 0x1b, 0x0a, 0x17,
	0x54, 0x48, 0x52, 0x45, 0x53, 0x48, 0x4f, 0x4c, 0x44, 0x5f, 0x42, 0x59, 0x5f, 0x42, 0x41, 0x43,
	0x4b, 0x47, 0x52, 0x4f, 0x55, 0x4e, 0x44, 0x10, 0x02, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x33,
}

var (
	file_com_coralogixapis_dashboards_v1_ast_widgets_common_thresholds_proto_rawDescOnce sync.Once
	file_com_coralogixapis_dashboards_v1_ast_widgets_common_thresholds_proto_rawDescData = file_com_coralogixapis_dashboards_v1_ast_widgets_common_thresholds_proto_rawDesc
)

func file_com_coralogixapis_dashboards_v1_ast_widgets_common_thresholds_proto_rawDescGZIP() []byte {
	file_com_coralogixapis_dashboards_v1_ast_widgets_common_thresholds_proto_rawDescOnce.Do(func() {
		file_com_coralogixapis_dashboards_v1_ast_widgets_common_thresholds_proto_rawDescData = protoimpl.X.CompressGZIP(file_com_coralogixapis_dashboards_v1_ast_widgets_common_thresholds_proto_rawDescData)
	})
	return file_com_coralogixapis_dashboards_v1_ast_widgets_common_thresholds_proto_rawDescData
}

var file_com_coralogixapis_dashboards_v1_ast_widgets_common_thresholds_proto_enumTypes = make([]protoimpl.EnumInfo, 2)
var file_com_coralogixapis_dashboards_v1_ast_widgets_common_thresholds_proto_msgTypes = make([]protoimpl.MessageInfo, 1)
var file_com_coralogixapis_dashboards_v1_ast_widgets_common_thresholds_proto_goTypes = []any{
	(ThresholdType)(0),             // 0: com.coralogixapis.dashboards.v1.ast.widgets.common.ThresholdType
	(ThresholdBy)(0),               // 1: com.coralogixapis.dashboards.v1.ast.widgets.common.ThresholdBy
	(*Threshold)(nil),              // 2: com.coralogixapis.dashboards.v1.ast.widgets.common.Threshold
	(*wrapperspb.DoubleValue)(nil), // 3: google.protobuf.DoubleValue
	(*wrapperspb.StringValue)(nil), // 4: google.protobuf.StringValue
}
var file_com_coralogixapis_dashboards_v1_ast_widgets_common_thresholds_proto_depIdxs = []int32{
	3, // 0: com.coralogixapis.dashboards.v1.ast.widgets.common.Threshold.from:type_name -> google.protobuf.DoubleValue
	4, // 1: com.coralogixapis.dashboards.v1.ast.widgets.common.Threshold.color:type_name -> google.protobuf.StringValue
	2, // [2:2] is the sub-list for method output_type
	2, // [2:2] is the sub-list for method input_type
	2, // [2:2] is the sub-list for extension type_name
	2, // [2:2] is the sub-list for extension extendee
	0, // [0:2] is the sub-list for field type_name
}

func init() { file_com_coralogixapis_dashboards_v1_ast_widgets_common_thresholds_proto_init() }
func file_com_coralogixapis_dashboards_v1_ast_widgets_common_thresholds_proto_init() {
	if File_com_coralogixapis_dashboards_v1_ast_widgets_common_thresholds_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_com_coralogixapis_dashboards_v1_ast_widgets_common_thresholds_proto_msgTypes[0].Exporter = func(v any, i int) any {
			switch v := v.(*Threshold); i {
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
			RawDescriptor: file_com_coralogixapis_dashboards_v1_ast_widgets_common_thresholds_proto_rawDesc,
			NumEnums:      2,
			NumMessages:   1,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_com_coralogixapis_dashboards_v1_ast_widgets_common_thresholds_proto_goTypes,
		DependencyIndexes: file_com_coralogixapis_dashboards_v1_ast_widgets_common_thresholds_proto_depIdxs,
		EnumInfos:         file_com_coralogixapis_dashboards_v1_ast_widgets_common_thresholds_proto_enumTypes,
		MessageInfos:      file_com_coralogixapis_dashboards_v1_ast_widgets_common_thresholds_proto_msgTypes,
	}.Build()
	File_com_coralogixapis_dashboards_v1_ast_widgets_common_thresholds_proto = out.File
	file_com_coralogixapis_dashboards_v1_ast_widgets_common_thresholds_proto_rawDesc = nil
	file_com_coralogixapis_dashboards_v1_ast_widgets_common_thresholds_proto_goTypes = nil
	file_com_coralogixapis_dashboards_v1_ast_widgets_common_thresholds_proto_depIdxs = nil
}