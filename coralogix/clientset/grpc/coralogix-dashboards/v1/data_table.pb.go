// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.28.1
// 	protoc        v3.21.8
// source: com/coralogixapis/dashboards/v1/ast/widgets/data_table.proto

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

type RowStyle int32

const (
	RowStyle_ROW_STYLE_UNSPECIFIED RowStyle = 0
	RowStyle_ROW_STYLE_ONE_LINE    RowStyle = 1
	RowStyle_ROW_STYLE_TWO_LINE    RowStyle = 2
	RowStyle_ROW_STYLE_CONDENSED   RowStyle = 3
	RowStyle_ROW_STYLE_JSON        RowStyle = 4
	RowStyle_ROW_STYLE_LIST        RowStyle = 5
)

// Enum value maps for RowStyle.
var (
	RowStyle_name = map[int32]string{
		0: "ROW_STYLE_UNSPECIFIED",
		1: "ROW_STYLE_ONE_LINE",
		2: "ROW_STYLE_TWO_LINE",
		3: "ROW_STYLE_CONDENSED",
		4: "ROW_STYLE_JSON",
		5: "ROW_STYLE_LIST",
	}
	RowStyle_value = map[string]int32{
		"ROW_STYLE_UNSPECIFIED": 0,
		"ROW_STYLE_ONE_LINE":    1,
		"ROW_STYLE_TWO_LINE":    2,
		"ROW_STYLE_CONDENSED":   3,
		"ROW_STYLE_JSON":        4,
		"ROW_STYLE_LIST":        5,
	}
)

func (x RowStyle) Enum() *RowStyle {
	p := new(RowStyle)
	*p = x
	return p
}

func (x RowStyle) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (RowStyle) Descriptor() protoreflect.EnumDescriptor {
	return file_com_coralogixapis_dashboards_v1_ast_widgets_data_table_proto_enumTypes[0].Descriptor()
}

func (RowStyle) Type() protoreflect.EnumType {
	return &file_com_coralogixapis_dashboards_v1_ast_widgets_data_table_proto_enumTypes[0]
}

func (x RowStyle) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use RowStyle.Descriptor instead.
func (RowStyle) EnumDescriptor() ([]byte, []int) {
	return file_com_coralogixapis_dashboards_v1_ast_widgets_data_table_proto_rawDescGZIP(), []int{0}
}

type DataTable struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Query          *DataTable_Query       `protobuf:"bytes,1,opt,name=query,proto3" json:"query,omitempty"`
	ResultsPerPage *wrapperspb.Int32Value `protobuf:"bytes,2,opt,name=results_per_page,json=resultsPerPage,proto3" json:"results_per_page,omitempty"`
	RowStyle       RowStyle               `protobuf:"varint,3,opt,name=row_style,json=rowStyle,proto3,enum=com.coralogixapis.dashboards.v1.ast.widgets.RowStyle" json:"row_style,omitempty"`
	Columns        []*DataTable_Column    `protobuf:"bytes,4,rep,name=columns,proto3" json:"columns,omitempty"`
	OrderBy        *OrderingField         `protobuf:"bytes,5,opt,name=order_by,json=orderBy,proto3" json:"order_by,omitempty"`
}

func (x *DataTable) Reset() {
	*x = DataTable{}
	if protoimpl.UnsafeEnabled {
		mi := &file_com_coralogixapis_dashboards_v1_ast_widgets_data_table_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *DataTable) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*DataTable) ProtoMessage() {}

func (x *DataTable) ProtoReflect() protoreflect.Message {
	mi := &file_com_coralogixapis_dashboards_v1_ast_widgets_data_table_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use DataTable.ProtoReflect.Descriptor instead.
func (*DataTable) Descriptor() ([]byte, []int) {
	return file_com_coralogixapis_dashboards_v1_ast_widgets_data_table_proto_rawDescGZIP(), []int{0}
}

func (x *DataTable) GetQuery() *DataTable_Query {
	if x != nil {
		return x.Query
	}
	return nil
}

func (x *DataTable) GetResultsPerPage() *wrapperspb.Int32Value {
	if x != nil {
		return x.ResultsPerPage
	}
	return nil
}

func (x *DataTable) GetRowStyle() RowStyle {
	if x != nil {
		return x.RowStyle
	}
	return RowStyle_ROW_STYLE_UNSPECIFIED
}

func (x *DataTable) GetColumns() []*DataTable_Column {
	if x != nil {
		return x.Columns
	}
	return nil
}

func (x *DataTable) GetOrderBy() *OrderingField {
	if x != nil {
		return x.OrderBy
	}
	return nil
}

type DataTable_Query struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// Types that are assignable to Value:
	//	*DataTable_Query_Logs
	Value isDataTable_Query_Value `protobuf_oneof:"value"`
}

func (x *DataTable_Query) Reset() {
	*x = DataTable_Query{}
	if protoimpl.UnsafeEnabled {
		mi := &file_com_coralogixapis_dashboards_v1_ast_widgets_data_table_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *DataTable_Query) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*DataTable_Query) ProtoMessage() {}

func (x *DataTable_Query) ProtoReflect() protoreflect.Message {
	mi := &file_com_coralogixapis_dashboards_v1_ast_widgets_data_table_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use DataTable_Query.ProtoReflect.Descriptor instead.
func (*DataTable_Query) Descriptor() ([]byte, []int) {
	return file_com_coralogixapis_dashboards_v1_ast_widgets_data_table_proto_rawDescGZIP(), []int{0, 0}
}

func (m *DataTable_Query) GetValue() isDataTable_Query_Value {
	if m != nil {
		return m.Value
	}
	return nil
}

func (x *DataTable_Query) GetLogs() *DataTable_LogsQuery {
	if x, ok := x.GetValue().(*DataTable_Query_Logs); ok {
		return x.Logs
	}
	return nil
}

type isDataTable_Query_Value interface {
	isDataTable_Query_Value()
}

type DataTable_Query_Logs struct {
	Logs *DataTable_LogsQuery `protobuf:"bytes,1,opt,name=logs,proto3,oneof"`
}

func (*DataTable_Query_Logs) isDataTable_Query_Value() {}

type DataTable_LogsQuery struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	LuceneQuery *LuceneQuery        `protobuf:"bytes,1,opt,name=lucene_query,json=luceneQuery,proto3" json:"lucene_query,omitempty"`
	Filters     []*Filter_LogFilter `protobuf:"bytes,2,rep,name=filters,proto3" json:"filters,omitempty"`
}

func (x *DataTable_LogsQuery) Reset() {
	*x = DataTable_LogsQuery{}
	if protoimpl.UnsafeEnabled {
		mi := &file_com_coralogixapis_dashboards_v1_ast_widgets_data_table_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *DataTable_LogsQuery) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*DataTable_LogsQuery) ProtoMessage() {}

func (x *DataTable_LogsQuery) ProtoReflect() protoreflect.Message {
	mi := &file_com_coralogixapis_dashboards_v1_ast_widgets_data_table_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use DataTable_LogsQuery.ProtoReflect.Descriptor instead.
func (*DataTable_LogsQuery) Descriptor() ([]byte, []int) {
	return file_com_coralogixapis_dashboards_v1_ast_widgets_data_table_proto_rawDescGZIP(), []int{0, 1}
}

func (x *DataTable_LogsQuery) GetLuceneQuery() *LuceneQuery {
	if x != nil {
		return x.LuceneQuery
	}
	return nil
}

func (x *DataTable_LogsQuery) GetFilters() []*Filter_LogFilter {
	if x != nil {
		return x.Filters
	}
	return nil
}

type DataTable_Column struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Field *wrapperspb.StringValue `protobuf:"bytes,1,opt,name=field,proto3" json:"field,omitempty"`
	Width *wrapperspb.Int32Value  `protobuf:"bytes,2,opt,name=width,proto3" json:"width,omitempty"`
}

func (x *DataTable_Column) Reset() {
	*x = DataTable_Column{}
	if protoimpl.UnsafeEnabled {
		mi := &file_com_coralogixapis_dashboards_v1_ast_widgets_data_table_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *DataTable_Column) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*DataTable_Column) ProtoMessage() {}

func (x *DataTable_Column) ProtoReflect() protoreflect.Message {
	mi := &file_com_coralogixapis_dashboards_v1_ast_widgets_data_table_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use DataTable_Column.ProtoReflect.Descriptor instead.
func (*DataTable_Column) Descriptor() ([]byte, []int) {
	return file_com_coralogixapis_dashboards_v1_ast_widgets_data_table_proto_rawDescGZIP(), []int{0, 2}
}

func (x *DataTable_Column) GetField() *wrapperspb.StringValue {
	if x != nil {
		return x.Field
	}
	return nil
}

func (x *DataTable_Column) GetWidth() *wrapperspb.Int32Value {
	if x != nil {
		return x.Width
	}
	return nil
}

var File_com_coralogixapis_dashboards_v1_ast_widgets_data_table_proto protoreflect.FileDescriptor

var file_com_coralogixapis_dashboards_v1_ast_widgets_data_table_proto_rawDesc = []byte{
	0x0a, 0x3c, 0x63, 0x6f, 0x6d, 0x2f, 0x63, 0x6f, 0x72, 0x61, 0x6c, 0x6f, 0x67, 0x69, 0x78, 0x61,
	0x70, 0x69, 0x73, 0x2f, 0x64, 0x61, 0x73, 0x68, 0x62, 0x6f, 0x61, 0x72, 0x64, 0x73, 0x2f, 0x76,
	0x31, 0x2f, 0x61, 0x73, 0x74, 0x2f, 0x77, 0x69, 0x64, 0x67, 0x65, 0x74, 0x73, 0x2f, 0x64, 0x61,
	0x74, 0x61, 0x5f, 0x74, 0x61, 0x62, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x2b,
	0x63, 0x6f, 0x6d, 0x2e, 0x63, 0x6f, 0x72, 0x61, 0x6c, 0x6f, 0x67, 0x69, 0x78, 0x61, 0x70, 0x69,
	0x73, 0x2e, 0x64, 0x61, 0x73, 0x68, 0x62, 0x6f, 0x61, 0x72, 0x64, 0x73, 0x2e, 0x76, 0x31, 0x2e,
	0x61, 0x73, 0x74, 0x2e, 0x77, 0x69, 0x64, 0x67, 0x65, 0x74, 0x73, 0x1a, 0x30, 0x63, 0x6f, 0x6d,
	0x2f, 0x63, 0x6f, 0x72, 0x61, 0x6c, 0x6f, 0x67, 0x69, 0x78, 0x61, 0x70, 0x69, 0x73, 0x2f, 0x64,
	0x61, 0x73, 0x68, 0x62, 0x6f, 0x61, 0x72, 0x64, 0x73, 0x2f, 0x76, 0x31, 0x2f, 0x61, 0x73, 0x74,
	0x2f, 0x66, 0x69, 0x6c, 0x74, 0x65, 0x72, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x40, 0x63,
	0x6f, 0x6d, 0x2f, 0x63, 0x6f, 0x72, 0x61, 0x6c, 0x6f, 0x67, 0x69, 0x78, 0x61, 0x70, 0x69, 0x73,
	0x2f, 0x64, 0x61, 0x73, 0x68, 0x62, 0x6f, 0x61, 0x72, 0x64, 0x73, 0x2f, 0x76, 0x31, 0x2f, 0x61,
	0x73, 0x74, 0x2f, 0x77, 0x69, 0x64, 0x67, 0x65, 0x74, 0x73, 0x2f, 0x63, 0x6f, 0x6d, 0x6d, 0x6f,
	0x6e, 0x2f, 0x71, 0x75, 0x65, 0x72, 0x69, 0x65, 0x73, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a,
	0x3b, 0x63, 0x6f, 0x6d, 0x2f, 0x63, 0x6f, 0x72, 0x61, 0x6c, 0x6f, 0x67, 0x69, 0x78, 0x61, 0x70,
	0x69, 0x73, 0x2f, 0x64, 0x61, 0x73, 0x68, 0x62, 0x6f, 0x61, 0x72, 0x64, 0x73, 0x2f, 0x76, 0x31,
	0x2f, 0x63, 0x6f, 0x6d, 0x6d, 0x6f, 0x6e, 0x2f, 0x6f, 0x72, 0x64, 0x65, 0x72, 0x69, 0x6e, 0x67,
	0x5f, 0x66, 0x69, 0x65, 0x6c, 0x64, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x1e, 0x67, 0x6f,
	0x6f, 0x67, 0x6c, 0x65, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2f, 0x77, 0x72,
	0x61, 0x70, 0x70, 0x65, 0x72, 0x73, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0xc3, 0x06, 0x0a,
	0x09, 0x44, 0x61, 0x74, 0x61, 0x54, 0x61, 0x62, 0x6c, 0x65, 0x12, 0x52, 0x0a, 0x05, 0x71, 0x75,
	0x65, 0x72, 0x79, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x3c, 0x2e, 0x63, 0x6f, 0x6d, 0x2e,
	0x63, 0x6f, 0x72, 0x61, 0x6c, 0x6f, 0x67, 0x69, 0x78, 0x61, 0x70, 0x69, 0x73, 0x2e, 0x64, 0x61,
	0x73, 0x68, 0x62, 0x6f, 0x61, 0x72, 0x64, 0x73, 0x2e, 0x76, 0x31, 0x2e, 0x61, 0x73, 0x74, 0x2e,
	0x77, 0x69, 0x64, 0x67, 0x65, 0x74, 0x73, 0x2e, 0x44, 0x61, 0x74, 0x61, 0x54, 0x61, 0x62, 0x6c,
	0x65, 0x2e, 0x51, 0x75, 0x65, 0x72, 0x79, 0x52, 0x05, 0x71, 0x75, 0x65, 0x72, 0x79, 0x12, 0x45,
	0x0a, 0x10, 0x72, 0x65, 0x73, 0x75, 0x6c, 0x74, 0x73, 0x5f, 0x70, 0x65, 0x72, 0x5f, 0x70, 0x61,
	0x67, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1b, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c,
	0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x49, 0x6e, 0x74, 0x33, 0x32,
	0x56, 0x61, 0x6c, 0x75, 0x65, 0x52, 0x0e, 0x72, 0x65, 0x73, 0x75, 0x6c, 0x74, 0x73, 0x50, 0x65,
	0x72, 0x50, 0x61, 0x67, 0x65, 0x12, 0x52, 0x0a, 0x09, 0x72, 0x6f, 0x77, 0x5f, 0x73, 0x74, 0x79,
	0x6c, 0x65, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0e, 0x32, 0x35, 0x2e, 0x63, 0x6f, 0x6d, 0x2e, 0x63,
	0x6f, 0x72, 0x61, 0x6c, 0x6f, 0x67, 0x69, 0x78, 0x61, 0x70, 0x69, 0x73, 0x2e, 0x64, 0x61, 0x73,
	0x68, 0x62, 0x6f, 0x61, 0x72, 0x64, 0x73, 0x2e, 0x76, 0x31, 0x2e, 0x61, 0x73, 0x74, 0x2e, 0x77,
	0x69, 0x64, 0x67, 0x65, 0x74, 0x73, 0x2e, 0x52, 0x6f, 0x77, 0x53, 0x74, 0x79, 0x6c, 0x65, 0x52,
	0x08, 0x72, 0x6f, 0x77, 0x53, 0x74, 0x79, 0x6c, 0x65, 0x12, 0x57, 0x0a, 0x07, 0x63, 0x6f, 0x6c,
	0x75, 0x6d, 0x6e, 0x73, 0x18, 0x04, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x3d, 0x2e, 0x63, 0x6f, 0x6d,
	0x2e, 0x63, 0x6f, 0x72, 0x61, 0x6c, 0x6f, 0x67, 0x69, 0x78, 0x61, 0x70, 0x69, 0x73, 0x2e, 0x64,
	0x61, 0x73, 0x68, 0x62, 0x6f, 0x61, 0x72, 0x64, 0x73, 0x2e, 0x76, 0x31, 0x2e, 0x61, 0x73, 0x74,
	0x2e, 0x77, 0x69, 0x64, 0x67, 0x65, 0x74, 0x73, 0x2e, 0x44, 0x61, 0x74, 0x61, 0x54, 0x61, 0x62,
	0x6c, 0x65, 0x2e, 0x43, 0x6f, 0x6c, 0x75, 0x6d, 0x6e, 0x52, 0x07, 0x63, 0x6f, 0x6c, 0x75, 0x6d,
	0x6e, 0x73, 0x12, 0x50, 0x0a, 0x08, 0x6f, 0x72, 0x64, 0x65, 0x72, 0x5f, 0x62, 0x79, 0x18, 0x05,
	0x20, 0x01, 0x28, 0x0b, 0x32, 0x35, 0x2e, 0x63, 0x6f, 0x6d, 0x2e, 0x63, 0x6f, 0x72, 0x61, 0x6c,
	0x6f, 0x67, 0x69, 0x78, 0x61, 0x70, 0x69, 0x73, 0x2e, 0x64, 0x61, 0x73, 0x68, 0x62, 0x6f, 0x61,
	0x72, 0x64, 0x73, 0x2e, 0x76, 0x31, 0x2e, 0x63, 0x6f, 0x6d, 0x6d, 0x6f, 0x6e, 0x2e, 0x4f, 0x72,
	0x64, 0x65, 0x72, 0x69, 0x6e, 0x67, 0x46, 0x69, 0x65, 0x6c, 0x64, 0x52, 0x07, 0x6f, 0x72, 0x64,
	0x65, 0x72, 0x42, 0x79, 0x1a, 0x68, 0x0a, 0x05, 0x51, 0x75, 0x65, 0x72, 0x79, 0x12, 0x56, 0x0a,
	0x04, 0x6c, 0x6f, 0x67, 0x73, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x40, 0x2e, 0x63, 0x6f,
	0x6d, 0x2e, 0x63, 0x6f, 0x72, 0x61, 0x6c, 0x6f, 0x67, 0x69, 0x78, 0x61, 0x70, 0x69, 0x73, 0x2e,
	0x64, 0x61, 0x73, 0x68, 0x62, 0x6f, 0x61, 0x72, 0x64, 0x73, 0x2e, 0x76, 0x31, 0x2e, 0x61, 0x73,
	0x74, 0x2e, 0x77, 0x69, 0x64, 0x67, 0x65, 0x74, 0x73, 0x2e, 0x44, 0x61, 0x74, 0x61, 0x54, 0x61,
	0x62, 0x6c, 0x65, 0x2e, 0x4c, 0x6f, 0x67, 0x73, 0x51, 0x75, 0x65, 0x72, 0x79, 0x48, 0x00, 0x52,
	0x04, 0x6c, 0x6f, 0x67, 0x73, 0x42, 0x07, 0x0a, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x1a, 0xc0,
	0x01, 0x0a, 0x09, 0x4c, 0x6f, 0x67, 0x73, 0x51, 0x75, 0x65, 0x72, 0x79, 0x12, 0x62, 0x0a, 0x0c,
	0x6c, 0x75, 0x63, 0x65, 0x6e, 0x65, 0x5f, 0x71, 0x75, 0x65, 0x72, 0x79, 0x18, 0x01, 0x20, 0x01,
	0x28, 0x0b, 0x32, 0x3f, 0x2e, 0x63, 0x6f, 0x6d, 0x2e, 0x63, 0x6f, 0x72, 0x61, 0x6c, 0x6f, 0x67,
	0x69, 0x78, 0x61, 0x70, 0x69, 0x73, 0x2e, 0x64, 0x61, 0x73, 0x68, 0x62, 0x6f, 0x61, 0x72, 0x64,
	0x73, 0x2e, 0x76, 0x31, 0x2e, 0x61, 0x73, 0x74, 0x2e, 0x77, 0x69, 0x64, 0x67, 0x65, 0x74, 0x73,
	0x2e, 0x63, 0x6f, 0x6d, 0x6d, 0x6f, 0x6e, 0x2e, 0x4c, 0x75, 0x63, 0x65, 0x6e, 0x65, 0x51, 0x75,
	0x65, 0x72, 0x79, 0x52, 0x0b, 0x6c, 0x75, 0x63, 0x65, 0x6e, 0x65, 0x51, 0x75, 0x65, 0x72, 0x79,
	0x12, 0x4f, 0x0a, 0x07, 0x66, 0x69, 0x6c, 0x74, 0x65, 0x72, 0x73, 0x18, 0x02, 0x20, 0x03, 0x28,
	0x0b, 0x32, 0x35, 0x2e, 0x63, 0x6f, 0x6d, 0x2e, 0x63, 0x6f, 0x72, 0x61, 0x6c, 0x6f, 0x67, 0x69,
	0x78, 0x61, 0x70, 0x69, 0x73, 0x2e, 0x64, 0x61, 0x73, 0x68, 0x62, 0x6f, 0x61, 0x72, 0x64, 0x73,
	0x2e, 0x76, 0x31, 0x2e, 0x61, 0x73, 0x74, 0x2e, 0x46, 0x69, 0x6c, 0x74, 0x65, 0x72, 0x2e, 0x4c,
	0x6f, 0x67, 0x46, 0x69, 0x6c, 0x74, 0x65, 0x72, 0x52, 0x07, 0x66, 0x69, 0x6c, 0x74, 0x65, 0x72,
	0x73, 0x1a, 0x6f, 0x0a, 0x06, 0x43, 0x6f, 0x6c, 0x75, 0x6d, 0x6e, 0x12, 0x32, 0x0a, 0x05, 0x66,
	0x69, 0x65, 0x6c, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1c, 0x2e, 0x67, 0x6f, 0x6f,
	0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x53, 0x74, 0x72,
	0x69, 0x6e, 0x67, 0x56, 0x61, 0x6c, 0x75, 0x65, 0x52, 0x05, 0x66, 0x69, 0x65, 0x6c, 0x64, 0x12,
	0x31, 0x0a, 0x05, 0x77, 0x69, 0x64, 0x74, 0x68, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1b,
	0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66,
	0x2e, 0x49, 0x6e, 0x74, 0x33, 0x32, 0x56, 0x61, 0x6c, 0x75, 0x65, 0x52, 0x05, 0x77, 0x69, 0x64,
	0x74, 0x68, 0x2a, 0x96, 0x01, 0x0a, 0x08, 0x52, 0x6f, 0x77, 0x53, 0x74, 0x79, 0x6c, 0x65, 0x12,
	0x19, 0x0a, 0x15, 0x52, 0x4f, 0x57, 0x5f, 0x53, 0x54, 0x59, 0x4c, 0x45, 0x5f, 0x55, 0x4e, 0x53,
	0x50, 0x45, 0x43, 0x49, 0x46, 0x49, 0x45, 0x44, 0x10, 0x00, 0x12, 0x16, 0x0a, 0x12, 0x52, 0x4f,
	0x57, 0x5f, 0x53, 0x54, 0x59, 0x4c, 0x45, 0x5f, 0x4f, 0x4e, 0x45, 0x5f, 0x4c, 0x49, 0x4e, 0x45,
	0x10, 0x01, 0x12, 0x16, 0x0a, 0x12, 0x52, 0x4f, 0x57, 0x5f, 0x53, 0x54, 0x59, 0x4c, 0x45, 0x5f,
	0x54, 0x57, 0x4f, 0x5f, 0x4c, 0x49, 0x4e, 0x45, 0x10, 0x02, 0x12, 0x17, 0x0a, 0x13, 0x52, 0x4f,
	0x57, 0x5f, 0x53, 0x54, 0x59, 0x4c, 0x45, 0x5f, 0x43, 0x4f, 0x4e, 0x44, 0x45, 0x4e, 0x53, 0x45,
	0x44, 0x10, 0x03, 0x12, 0x12, 0x0a, 0x0e, 0x52, 0x4f, 0x57, 0x5f, 0x53, 0x54, 0x59, 0x4c, 0x45,
	0x5f, 0x4a, 0x53, 0x4f, 0x4e, 0x10, 0x04, 0x12, 0x12, 0x0a, 0x0e, 0x52, 0x4f, 0x57, 0x5f, 0x53,
	0x54, 0x59, 0x4c, 0x45, 0x5f, 0x4c, 0x49, 0x53, 0x54, 0x10, 0x05, 0x42, 0x04, 0x5a, 0x02, 0x2e,
	0x2f, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_com_coralogixapis_dashboards_v1_ast_widgets_data_table_proto_rawDescOnce sync.Once
	file_com_coralogixapis_dashboards_v1_ast_widgets_data_table_proto_rawDescData = file_com_coralogixapis_dashboards_v1_ast_widgets_data_table_proto_rawDesc
)

func file_com_coralogixapis_dashboards_v1_ast_widgets_data_table_proto_rawDescGZIP() []byte {
	file_com_coralogixapis_dashboards_v1_ast_widgets_data_table_proto_rawDescOnce.Do(func() {
		file_com_coralogixapis_dashboards_v1_ast_widgets_data_table_proto_rawDescData = protoimpl.X.CompressGZIP(file_com_coralogixapis_dashboards_v1_ast_widgets_data_table_proto_rawDescData)
	})
	return file_com_coralogixapis_dashboards_v1_ast_widgets_data_table_proto_rawDescData
}

var file_com_coralogixapis_dashboards_v1_ast_widgets_data_table_proto_enumTypes = make([]protoimpl.EnumInfo, 1)
var file_com_coralogixapis_dashboards_v1_ast_widgets_data_table_proto_msgTypes = make([]protoimpl.MessageInfo, 4)
var file_com_coralogixapis_dashboards_v1_ast_widgets_data_table_proto_goTypes = []interface{}{
	(RowStyle)(0),                  // 0: com.coralogixapis.dashboards.v1.ast.widgets.RowStyle
	(*DataTable)(nil),              // 1: com.coralogixapis.dashboards.v1.ast.widgets.DataTable
	(*DataTable_Query)(nil),        // 2: com.coralogixapis.dashboards.v1.ast.widgets.DataTable.Query
	(*DataTable_LogsQuery)(nil),    // 3: com.coralogixapis.dashboards.v1.ast.widgets.DataTable.LogsQuery
	(*DataTable_Column)(nil),       // 4: com.coralogixapis.dashboards.v1.ast.widgets.DataTable.Column
	(*wrapperspb.Int32Value)(nil),  // 5: google.protobuf.Int32Value
	(*OrderingField)(nil),          // 6: com.coralogixapis.dashboards.v1.common.OrderingField
	(*LuceneQuery)(nil),            // 7: com.coralogixapis.dashboards.v1.ast.widgets.common.LuceneQuery
	(*Filter_LogFilter)(nil),       // 8: com.coralogixapis.dashboards.v1.ast.Filter.LogFilter
	(*wrapperspb.StringValue)(nil), // 9: google.protobuf.StringValue
}
var file_com_coralogixapis_dashboards_v1_ast_widgets_data_table_proto_depIdxs = []int32{
	2,  // 0: com.coralogixapis.dashboards.v1.ast.widgets.DataTable.query:type_name -> com.coralogixapis.dashboards.v1.ast.widgets.DataTable.Query
	5,  // 1: com.coralogixapis.dashboards.v1.ast.widgets.DataTable.results_per_page:type_name -> google.protobuf.Int32Value
	0,  // 2: com.coralogixapis.dashboards.v1.ast.widgets.DataTable.row_style:type_name -> com.coralogixapis.dashboards.v1.ast.widgets.RowStyle
	4,  // 3: com.coralogixapis.dashboards.v1.ast.widgets.DataTable.columns:type_name -> com.coralogixapis.dashboards.v1.ast.widgets.DataTable.Column
	6,  // 4: com.coralogixapis.dashboards.v1.ast.widgets.DataTable.order_by:type_name -> com.coralogixapis.dashboards.v1.common.OrderingField
	3,  // 5: com.coralogixapis.dashboards.v1.ast.widgets.DataTable.Query.logs:type_name -> com.coralogixapis.dashboards.v1.ast.widgets.DataTable.LogsQuery
	7,  // 6: com.coralogixapis.dashboards.v1.ast.widgets.DataTable.LogsQuery.lucene_query:type_name -> com.coralogixapis.dashboards.v1.ast.widgets.common.LuceneQuery
	8,  // 7: com.coralogixapis.dashboards.v1.ast.widgets.DataTable.LogsQuery.filters:type_name -> com.coralogixapis.dashboards.v1.ast.Filter.LogFilter
	9,  // 8: com.coralogixapis.dashboards.v1.ast.widgets.DataTable.Column.field:type_name -> google.protobuf.StringValue
	5,  // 9: com.coralogixapis.dashboards.v1.ast.widgets.DataTable.Column.width:type_name -> google.protobuf.Int32Value
	10, // [10:10] is the sub-list for method output_type
	10, // [10:10] is the sub-list for method input_type
	10, // [10:10] is the sub-list for extension type_name
	10, // [10:10] is the sub-list for extension extendee
	0,  // [0:10] is the sub-list for field type_name
}

func init() { file_com_coralogixapis_dashboards_v1_ast_widgets_data_table_proto_init() }
func file_com_coralogixapis_dashboards_v1_ast_widgets_data_table_proto_init() {
	if File_com_coralogixapis_dashboards_v1_ast_widgets_data_table_proto != nil {
		return
	}
	file_com_coralogixapis_dashboards_v1_ast_filter_proto_init()
	file_com_coralogixapis_dashboards_v1_ast_widgets_common_queries_proto_init()
	file_com_coralogixapis_dashboards_v1_common_ordering_field_proto_init()
	if !protoimpl.UnsafeEnabled {
		file_com_coralogixapis_dashboards_v1_ast_widgets_data_table_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*DataTable); i {
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
		file_com_coralogixapis_dashboards_v1_ast_widgets_data_table_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*DataTable_Query); i {
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
		file_com_coralogixapis_dashboards_v1_ast_widgets_data_table_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*DataTable_LogsQuery); i {
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
		file_com_coralogixapis_dashboards_v1_ast_widgets_data_table_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*DataTable_Column); i {
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
	file_com_coralogixapis_dashboards_v1_ast_widgets_data_table_proto_msgTypes[1].OneofWrappers = []interface{}{
		(*DataTable_Query_Logs)(nil),
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_com_coralogixapis_dashboards_v1_ast_widgets_data_table_proto_rawDesc,
			NumEnums:      1,
			NumMessages:   4,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_com_coralogixapis_dashboards_v1_ast_widgets_data_table_proto_goTypes,
		DependencyIndexes: file_com_coralogixapis_dashboards_v1_ast_widgets_data_table_proto_depIdxs,
		EnumInfos:         file_com_coralogixapis_dashboards_v1_ast_widgets_data_table_proto_enumTypes,
		MessageInfos:      file_com_coralogixapis_dashboards_v1_ast_widgets_data_table_proto_msgTypes,
	}.Build()
	File_com_coralogixapis_dashboards_v1_ast_widgets_data_table_proto = out.File
	file_com_coralogixapis_dashboards_v1_ast_widgets_data_table_proto_rawDesc = nil
	file_com_coralogixapis_dashboards_v1_ast_widgets_data_table_proto_goTypes = nil
	file_com_coralogixapis_dashboards_v1_ast_widgets_data_table_proto_depIdxs = nil
}