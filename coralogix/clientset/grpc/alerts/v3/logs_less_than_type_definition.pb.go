// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.28.1
// 	protoc        v3.21.8
// source: com/coralogixapis/alerts/v3/alert_def_type_definition/standard/logs_less_than_type_definition.proto

package __

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

type LogsLessThanTypeDefinition struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	LogsFilter                 *LogsFilter                 `protobuf:"bytes,1,opt,name=logs_filter,json=logsFilter,proto3" json:"logs_filter,omitempty"`
	Threshold                  *wrapperspb.UInt32Value     `protobuf:"bytes,2,opt,name=threshold,proto3" json:"threshold,omitempty"`
	TimeWindow                 *LogsTimeWindow             `protobuf:"bytes,3,opt,name=time_window,json=timeWindow,proto3" json:"time_window,omitempty"`
	UndetectedValuesManagement *UndetectedValuesManagement `protobuf:"bytes,4,opt,name=undetected_values_management,json=undetectedValuesManagement,proto3" json:"undetected_values_management,omitempty"`
	NotificationPayloadFilter  []*wrapperspb.StringValue   `protobuf:"bytes,5,rep,name=notification_payload_filter,json=notificationPayloadFilter,proto3" json:"notification_payload_filter,omitempty"`
}

func (x *LogsLessThanTypeDefinition) Reset() {
	*x = LogsLessThanTypeDefinition{}
	if protoimpl.UnsafeEnabled {
		mi := &file_com_coralogixapis_alerts_v3_alert_def_type_definition_standard_logs_less_than_type_definition_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *LogsLessThanTypeDefinition) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*LogsLessThanTypeDefinition) ProtoMessage() {}

func (x *LogsLessThanTypeDefinition) ProtoReflect() protoreflect.Message {
	mi := &file_com_coralogixapis_alerts_v3_alert_def_type_definition_standard_logs_less_than_type_definition_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use LogsLessThanTypeDefinition.ProtoReflect.Descriptor instead.
func (*LogsLessThanTypeDefinition) Descriptor() ([]byte, []int) {
	return file_com_coralogixapis_alerts_v3_alert_def_type_definition_standard_logs_less_than_type_definition_proto_rawDescGZIP(), []int{0}
}

func (x *LogsLessThanTypeDefinition) GetLogsFilter() *LogsFilter {
	if x != nil {
		return x.LogsFilter
	}
	return nil
}

func (x *LogsLessThanTypeDefinition) GetThreshold() *wrapperspb.UInt32Value {
	if x != nil {
		return x.Threshold
	}
	return nil
}

func (x *LogsLessThanTypeDefinition) GetTimeWindow() *LogsTimeWindow {
	if x != nil {
		return x.TimeWindow
	}
	return nil
}

func (x *LogsLessThanTypeDefinition) GetUndetectedValuesManagement() *UndetectedValuesManagement {
	if x != nil {
		return x.UndetectedValuesManagement
	}
	return nil
}

func (x *LogsLessThanTypeDefinition) GetNotificationPayloadFilter() []*wrapperspb.StringValue {
	if x != nil {
		return x.NotificationPayloadFilter
	}
	return nil
}

var File_com_coralogixapis_alerts_v3_alert_def_type_definition_standard_logs_less_than_type_definition_proto protoreflect.FileDescriptor

var file_com_coralogixapis_alerts_v3_alert_def_type_definition_standard_logs_less_than_type_definition_proto_rawDesc = []byte{
	0x0a, 0x63, 0x63, 0x6f, 0x6d, 0x2f, 0x63, 0x6f, 0x72, 0x61, 0x6c, 0x6f, 0x67, 0x69, 0x78, 0x61,
	0x70, 0x69, 0x73, 0x2f, 0x61, 0x6c, 0x65, 0x72, 0x74, 0x73, 0x2f, 0x76, 0x33, 0x2f, 0x61, 0x6c,
	0x65, 0x72, 0x74, 0x5f, 0x64, 0x65, 0x66, 0x5f, 0x74, 0x79, 0x70, 0x65, 0x5f, 0x64, 0x65, 0x66,
	0x69, 0x6e, 0x69, 0x74, 0x69, 0x6f, 0x6e, 0x2f, 0x73, 0x74, 0x61, 0x6e, 0x64, 0x61, 0x72, 0x64,
	0x2f, 0x6c, 0x6f, 0x67, 0x73, 0x5f, 0x6c, 0x65, 0x73, 0x73, 0x5f, 0x74, 0x68, 0x61, 0x6e, 0x5f,
	0x74, 0x79, 0x70, 0x65, 0x5f, 0x64, 0x65, 0x66, 0x69, 0x6e, 0x69, 0x74, 0x69, 0x6f, 0x6e, 0x2e,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x1b, 0x63, 0x6f, 0x6d, 0x2e, 0x63, 0x6f, 0x72, 0x61, 0x6c,
	0x6f, 0x67, 0x69, 0x78, 0x61, 0x70, 0x69, 0x73, 0x2e, 0x61, 0x6c, 0x65, 0x72, 0x74, 0x73, 0x2e,
	0x76, 0x33, 0x1a, 0x1e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x62, 0x75, 0x66, 0x2f, 0x77, 0x72, 0x61, 0x70, 0x70, 0x65, 0x72, 0x73, 0x2e, 0x70, 0x72, 0x6f,
	0x74, 0x6f, 0x1a, 0x47, 0x63, 0x6f, 0x6d, 0x2f, 0x63, 0x6f, 0x72, 0x61, 0x6c, 0x6f, 0x67, 0x69,
	0x78, 0x61, 0x70, 0x69, 0x73, 0x2f, 0x61, 0x6c, 0x65, 0x72, 0x74, 0x73, 0x2f, 0x76, 0x33, 0x2f,
	0x61, 0x6c, 0x65, 0x72, 0x74, 0x5f, 0x64, 0x65, 0x66, 0x5f, 0x74, 0x79, 0x70, 0x65, 0x5f, 0x64,
	0x65, 0x66, 0x69, 0x6e, 0x69, 0x74, 0x69, 0x6f, 0x6e, 0x2f, 0x6c, 0x6f, 0x67, 0x73, 0x5f, 0x66,
	0x69, 0x6c, 0x74, 0x65, 0x72, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x58, 0x63, 0x6f, 0x6d,
	0x2f, 0x63, 0x6f, 0x72, 0x61, 0x6c, 0x6f, 0x67, 0x69, 0x78, 0x61, 0x70, 0x69, 0x73, 0x2f, 0x61,
	0x6c, 0x65, 0x72, 0x74, 0x73, 0x2f, 0x76, 0x33, 0x2f, 0x61, 0x6c, 0x65, 0x72, 0x74, 0x5f, 0x64,
	0x65, 0x66, 0x5f, 0x74, 0x79, 0x70, 0x65, 0x5f, 0x64, 0x65, 0x66, 0x69, 0x6e, 0x69, 0x74, 0x69,
	0x6f, 0x6e, 0x2f, 0x75, 0x6e, 0x64, 0x65, 0x74, 0x65, 0x63, 0x74, 0x65, 0x64, 0x5f, 0x76, 0x61,
	0x6c, 0x75, 0x65, 0x73, 0x5f, 0x6d, 0x61, 0x6e, 0x61, 0x67, 0x65, 0x6d, 0x65, 0x6e, 0x74, 0x2e,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x54, 0x63, 0x6f, 0x6d, 0x2f, 0x63, 0x6f, 0x72, 0x61, 0x6c,
	0x6f, 0x67, 0x69, 0x78, 0x61, 0x70, 0x69, 0x73, 0x2f, 0x61, 0x6c, 0x65, 0x72, 0x74, 0x73, 0x2f,
	0x76, 0x33, 0x2f, 0x61, 0x6c, 0x65, 0x72, 0x74, 0x5f, 0x64, 0x65, 0x66, 0x5f, 0x74, 0x79, 0x70,
	0x65, 0x5f, 0x64, 0x65, 0x66, 0x69, 0x6e, 0x69, 0x74, 0x69, 0x6f, 0x6e, 0x2f, 0x73, 0x74, 0x61,
	0x6e, 0x64, 0x61, 0x72, 0x64, 0x2f, 0x6c, 0x6f, 0x67, 0x73, 0x5f, 0x74, 0x69, 0x6d, 0x65, 0x77,
	0x69, 0x6e, 0x64, 0x6f, 0x77, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0xc9, 0x03, 0x0a, 0x1a,
	0x4c, 0x6f, 0x67, 0x73, 0x4c, 0x65, 0x73, 0x73, 0x54, 0x68, 0x61, 0x6e, 0x54, 0x79, 0x70, 0x65,
	0x44, 0x65, 0x66, 0x69, 0x6e, 0x69, 0x74, 0x69, 0x6f, 0x6e, 0x12, 0x48, 0x0a, 0x0b, 0x6c, 0x6f,
	0x67, 0x73, 0x5f, 0x66, 0x69, 0x6c, 0x74, 0x65, 0x72, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32,
	0x27, 0x2e, 0x63, 0x6f, 0x6d, 0x2e, 0x63, 0x6f, 0x72, 0x61, 0x6c, 0x6f, 0x67, 0x69, 0x78, 0x61,
	0x70, 0x69, 0x73, 0x2e, 0x61, 0x6c, 0x65, 0x72, 0x74, 0x73, 0x2e, 0x76, 0x33, 0x2e, 0x4c, 0x6f,
	0x67, 0x73, 0x46, 0x69, 0x6c, 0x74, 0x65, 0x72, 0x52, 0x0a, 0x6c, 0x6f, 0x67, 0x73, 0x46, 0x69,
	0x6c, 0x74, 0x65, 0x72, 0x12, 0x3a, 0x0a, 0x09, 0x74, 0x68, 0x72, 0x65, 0x73, 0x68, 0x6f, 0x6c,
	0x64, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1c, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65,
	0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x55, 0x49, 0x6e, 0x74, 0x33, 0x32,
	0x56, 0x61, 0x6c, 0x75, 0x65, 0x52, 0x09, 0x74, 0x68, 0x72, 0x65, 0x73, 0x68, 0x6f, 0x6c, 0x64,
	0x12, 0x4c, 0x0a, 0x0b, 0x74, 0x69, 0x6d, 0x65, 0x5f, 0x77, 0x69, 0x6e, 0x64, 0x6f, 0x77, 0x18,
	0x03, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x2b, 0x2e, 0x63, 0x6f, 0x6d, 0x2e, 0x63, 0x6f, 0x72, 0x61,
	0x6c, 0x6f, 0x67, 0x69, 0x78, 0x61, 0x70, 0x69, 0x73, 0x2e, 0x61, 0x6c, 0x65, 0x72, 0x74, 0x73,
	0x2e, 0x76, 0x33, 0x2e, 0x4c, 0x6f, 0x67, 0x73, 0x54, 0x69, 0x6d, 0x65, 0x57, 0x69, 0x6e, 0x64,
	0x6f, 0x77, 0x52, 0x0a, 0x74, 0x69, 0x6d, 0x65, 0x57, 0x69, 0x6e, 0x64, 0x6f, 0x77, 0x12, 0x79,
	0x0a, 0x1c, 0x75, 0x6e, 0x64, 0x65, 0x74, 0x65, 0x63, 0x74, 0x65, 0x64, 0x5f, 0x76, 0x61, 0x6c,
	0x75, 0x65, 0x73, 0x5f, 0x6d, 0x61, 0x6e, 0x61, 0x67, 0x65, 0x6d, 0x65, 0x6e, 0x74, 0x18, 0x04,
	0x20, 0x01, 0x28, 0x0b, 0x32, 0x37, 0x2e, 0x63, 0x6f, 0x6d, 0x2e, 0x63, 0x6f, 0x72, 0x61, 0x6c,
	0x6f, 0x67, 0x69, 0x78, 0x61, 0x70, 0x69, 0x73, 0x2e, 0x61, 0x6c, 0x65, 0x72, 0x74, 0x73, 0x2e,
	0x76, 0x33, 0x2e, 0x55, 0x6e, 0x64, 0x65, 0x74, 0x65, 0x63, 0x74, 0x65, 0x64, 0x56, 0x61, 0x6c,
	0x75, 0x65, 0x73, 0x4d, 0x61, 0x6e, 0x61, 0x67, 0x65, 0x6d, 0x65, 0x6e, 0x74, 0x52, 0x1a, 0x75,
	0x6e, 0x64, 0x65, 0x74, 0x65, 0x63, 0x74, 0x65, 0x64, 0x56, 0x61, 0x6c, 0x75, 0x65, 0x73, 0x4d,
	0x61, 0x6e, 0x61, 0x67, 0x65, 0x6d, 0x65, 0x6e, 0x74, 0x12, 0x5c, 0x0a, 0x1b, 0x6e, 0x6f, 0x74,
	0x69, 0x66, 0x69, 0x63, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x5f, 0x70, 0x61, 0x79, 0x6c, 0x6f, 0x61,
	0x64, 0x5f, 0x66, 0x69, 0x6c, 0x74, 0x65, 0x72, 0x18, 0x05, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x1c,
	0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66,
	0x2e, 0x53, 0x74, 0x72, 0x69, 0x6e, 0x67, 0x56, 0x61, 0x6c, 0x75, 0x65, 0x52, 0x19, 0x6e, 0x6f,
	0x74, 0x69, 0x66, 0x69, 0x63, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x50, 0x61, 0x79, 0x6c, 0x6f, 0x61,
	0x64, 0x46, 0x69, 0x6c, 0x74, 0x65, 0x72, 0x42, 0x04, 0x5a, 0x02, 0x2e, 0x2f, 0x62, 0x06, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_com_coralogixapis_alerts_v3_alert_def_type_definition_standard_logs_less_than_type_definition_proto_rawDescOnce sync.Once
	file_com_coralogixapis_alerts_v3_alert_def_type_definition_standard_logs_less_than_type_definition_proto_rawDescData = file_com_coralogixapis_alerts_v3_alert_def_type_definition_standard_logs_less_than_type_definition_proto_rawDesc
)

func file_com_coralogixapis_alerts_v3_alert_def_type_definition_standard_logs_less_than_type_definition_proto_rawDescGZIP() []byte {
	file_com_coralogixapis_alerts_v3_alert_def_type_definition_standard_logs_less_than_type_definition_proto_rawDescOnce.Do(func() {
		file_com_coralogixapis_alerts_v3_alert_def_type_definition_standard_logs_less_than_type_definition_proto_rawDescData = protoimpl.X.CompressGZIP(file_com_coralogixapis_alerts_v3_alert_def_type_definition_standard_logs_less_than_type_definition_proto_rawDescData)
	})
	return file_com_coralogixapis_alerts_v3_alert_def_type_definition_standard_logs_less_than_type_definition_proto_rawDescData
}

var file_com_coralogixapis_alerts_v3_alert_def_type_definition_standard_logs_less_than_type_definition_proto_msgTypes = make([]protoimpl.MessageInfo, 1)
var file_com_coralogixapis_alerts_v3_alert_def_type_definition_standard_logs_less_than_type_definition_proto_goTypes = []interface{}{
	(*LogsLessThanTypeDefinition)(nil), // 0: com.coralogixapis.alerts.v3.LogsLessThanTypeDefinition
	(*LogsFilter)(nil),                 // 1: com.coralogixapis.alerts.v3.LogsFilter
	(*wrapperspb.UInt32Value)(nil),     // 2: google.protobuf.UInt32Value
	(*LogsTimeWindow)(nil),             // 3: com.coralogixapis.alerts.v3.LogsTimeWindow
	(*UndetectedValuesManagement)(nil), // 4: com.coralogixapis.alerts.v3.UndetectedValuesManagement
	(*wrapperspb.StringValue)(nil),     // 5: google.protobuf.StringValue
}
var file_com_coralogixapis_alerts_v3_alert_def_type_definition_standard_logs_less_than_type_definition_proto_depIdxs = []int32{
	1, // 0: com.coralogixapis.alerts.v3.LogsLessThanTypeDefinition.logs_filter:type_name -> com.coralogixapis.alerts.v3.LogsFilter
	2, // 1: com.coralogixapis.alerts.v3.LogsLessThanTypeDefinition.threshold:type_name -> google.protobuf.UInt32Value
	3, // 2: com.coralogixapis.alerts.v3.LogsLessThanTypeDefinition.time_window:type_name -> com.coralogixapis.alerts.v3.LogsTimeWindow
	4, // 3: com.coralogixapis.alerts.v3.LogsLessThanTypeDefinition.undetected_values_management:type_name -> com.coralogixapis.alerts.v3.UndetectedValuesManagement
	5, // 4: com.coralogixapis.alerts.v3.LogsLessThanTypeDefinition.notification_payload_filter:type_name -> google.protobuf.StringValue
	5, // [5:5] is the sub-list for method output_type
	5, // [5:5] is the sub-list for method input_type
	5, // [5:5] is the sub-list for extension type_name
	5, // [5:5] is the sub-list for extension extendee
	0, // [0:5] is the sub-list for field type_name
}

func init() {
	file_com_coralogixapis_alerts_v3_alert_def_type_definition_standard_logs_less_than_type_definition_proto_init()
}
func file_com_coralogixapis_alerts_v3_alert_def_type_definition_standard_logs_less_than_type_definition_proto_init() {
	if File_com_coralogixapis_alerts_v3_alert_def_type_definition_standard_logs_less_than_type_definition_proto != nil {
		return
	}
	file_com_coralogixapis_alerts_v3_alert_def_type_definition_logs_filter_proto_init()
	file_com_coralogixapis_alerts_v3_alert_def_type_definition_undetected_values_management_proto_init()
	file_com_coralogixapis_alerts_v3_alert_def_type_definition_standard_logs_timewindow_proto_init()
	if !protoimpl.UnsafeEnabled {
		file_com_coralogixapis_alerts_v3_alert_def_type_definition_standard_logs_less_than_type_definition_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*LogsLessThanTypeDefinition); i {
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
			RawDescriptor: file_com_coralogixapis_alerts_v3_alert_def_type_definition_standard_logs_less_than_type_definition_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   1,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_com_coralogixapis_alerts_v3_alert_def_type_definition_standard_logs_less_than_type_definition_proto_goTypes,
		DependencyIndexes: file_com_coralogixapis_alerts_v3_alert_def_type_definition_standard_logs_less_than_type_definition_proto_depIdxs,
		MessageInfos:      file_com_coralogixapis_alerts_v3_alert_def_type_definition_standard_logs_less_than_type_definition_proto_msgTypes,
	}.Build()
	File_com_coralogixapis_alerts_v3_alert_def_type_definition_standard_logs_less_than_type_definition_proto = out.File
	file_com_coralogixapis_alerts_v3_alert_def_type_definition_standard_logs_less_than_type_definition_proto_rawDesc = nil
	file_com_coralogixapis_alerts_v3_alert_def_type_definition_standard_logs_less_than_type_definition_proto_goTypes = nil
	file_com_coralogixapis_alerts_v3_alert_def_type_definition_standard_logs_less_than_type_definition_proto_depIdxs = nil
}