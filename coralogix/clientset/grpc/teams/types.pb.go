// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.28.1
// 	protoc        v3.21.8
// source: com/coralogixapis/aaa/organisations/v2/types.proto

package __

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type PlanType int32

const (
	PlanType_PLAN_TYPE_UNSPECIFIED PlanType = 0
	PlanType_PLAN_TYPE_POST_TRIAL  PlanType = 1
	PlanType_PLAN_TYPE_PLAN        PlanType = 2
	PlanType_PLAN_TYPE_TRIAL       PlanType = 3
)

// Enum value maps for PlanType.
var (
	PlanType_name = map[int32]string{
		0: "PLAN_TYPE_UNSPECIFIED",
		1: "PLAN_TYPE_POST_TRIAL",
		2: "PLAN_TYPE_PLAN",
		3: "PLAN_TYPE_TRIAL",
	}
	PlanType_value = map[string]int32{
		"PLAN_TYPE_UNSPECIFIED": 0,
		"PLAN_TYPE_POST_TRIAL":  1,
		"PLAN_TYPE_PLAN":        2,
		"PLAN_TYPE_TRIAL":       3,
	}
)

func (x PlanType) Enum() *PlanType {
	p := new(PlanType)
	*p = x
	return p
}

func (x PlanType) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (PlanType) Descriptor() protoreflect.EnumDescriptor {
	return file_com_coralogixapis_aaa_organisations_v2_types_proto_enumTypes[0].Descriptor()
}

func (PlanType) Type() protoreflect.EnumType {
	return &file_com_coralogixapis_aaa_organisations_v2_types_proto_enumTypes[0]
}

func (x PlanType) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use PlanType.Descriptor instead.
func (PlanType) EnumDescriptor() ([]byte, []int) {
	return file_com_coralogixapis_aaa_organisations_v2_types_proto_rawDescGZIP(), []int{0}
}

type Team struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Id             *TeamId   `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
	ClusterId      string    `protobuf:"bytes,2,opt,name=cluster_id,json=clusterId,proto3" json:"cluster_id,omitempty"`
	Quota          *float32  `protobuf:"fixed32,3,opt,name=quota,proto3,oneof" json:"quota,omitempty"`
	Retention      *int32    `protobuf:"varint,4,opt,name=retention,proto3,oneof" json:"retention,omitempty"`
	PlanType       *PlanType `protobuf:"varint,5,opt,name=plan_type,json=planType,proto3,enum=com.coralogixapis.aaa.organisations.v2.PlanType,oneof" json:"plan_type,omitempty"`
	IsAuditingTeam bool      `protobuf:"varint,6,opt,name=is_auditing_team,json=isAuditingTeam,proto3" json:"is_auditing_team,omitempty"`
	Name           string    `protobuf:"bytes,7,opt,name=name,proto3" json:"name,omitempty"`
}

func (x *Team) Reset() {
	*x = Team{}
	if protoimpl.UnsafeEnabled {
		mi := &file_com_coralogixapis_aaa_organisations_v2_types_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Team) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Team) ProtoMessage() {}

func (x *Team) ProtoReflect() protoreflect.Message {
	mi := &file_com_coralogixapis_aaa_organisations_v2_types_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Team.ProtoReflect.Descriptor instead.
func (*Team) Descriptor() ([]byte, []int) {
	return file_com_coralogixapis_aaa_organisations_v2_types_proto_rawDescGZIP(), []int{0}
}

func (x *Team) GetId() *TeamId {
	if x != nil {
		return x.Id
	}
	return nil
}

func (x *Team) GetClusterId() string {
	if x != nil {
		return x.ClusterId
	}
	return ""
}

func (x *Team) GetQuota() float32 {
	if x != nil && x.Quota != nil {
		return *x.Quota
	}
	return 0
}

func (x *Team) GetRetention() int32 {
	if x != nil && x.Retention != nil {
		return *x.Retention
	}
	return 0
}

func (x *Team) GetPlanType() PlanType {
	if x != nil && x.PlanType != nil {
		return *x.PlanType
	}
	return PlanType_PLAN_TYPE_UNSPECIFIED
}

func (x *Team) GetIsAuditingTeam() bool {
	if x != nil {
		return x.IsAuditingTeam
	}
	return false
}

func (x *Team) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

type TeamInfo struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Id               *TeamId         `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
	OrganisationId   *OrganisationId `protobuf:"bytes,2,opt,name=organisation_id,json=organisationId,proto3" json:"organisation_id,omitempty"`
	OrganisationName string          `protobuf:"bytes,3,opt,name=organisation_name,json=organisationName,proto3" json:"organisation_name,omitempty"`
}

func (x *TeamInfo) Reset() {
	*x = TeamInfo{}
	if protoimpl.UnsafeEnabled {
		mi := &file_com_coralogixapis_aaa_organisations_v2_types_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *TeamInfo) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*TeamInfo) ProtoMessage() {}

func (x *TeamInfo) ProtoReflect() protoreflect.Message {
	mi := &file_com_coralogixapis_aaa_organisations_v2_types_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use TeamInfo.ProtoReflect.Descriptor instead.
func (*TeamInfo) Descriptor() ([]byte, []int) {
	return file_com_coralogixapis_aaa_organisations_v2_types_proto_rawDescGZIP(), []int{1}
}

func (x *TeamInfo) GetId() *TeamId {
	if x != nil {
		return x.Id
	}
	return nil
}

func (x *TeamInfo) GetOrganisationId() *OrganisationId {
	if x != nil {
		return x.OrganisationId
	}
	return nil
}

func (x *TeamInfo) GetOrganisationName() string {
	if x != nil {
		return x.OrganisationName
	}
	return ""
}

type TeamId struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Id uint32 `protobuf:"varint,1,opt,name=id,proto3" json:"id,omitempty"`
}

func (x *TeamId) Reset() {
	*x = TeamId{}
	if protoimpl.UnsafeEnabled {
		mi := &file_com_coralogixapis_aaa_organisations_v2_types_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *TeamId) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*TeamId) ProtoMessage() {}

func (x *TeamId) ProtoReflect() protoreflect.Message {
	mi := &file_com_coralogixapis_aaa_organisations_v2_types_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use TeamId.ProtoReflect.Descriptor instead.
func (*TeamId) Descriptor() ([]byte, []int) {
	return file_com_coralogixapis_aaa_organisations_v2_types_proto_rawDescGZIP(), []int{2}
}

func (x *TeamId) GetId() uint32 {
	if x != nil {
		return x.Id
	}
	return 0
}

type OrganisationId struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Id string `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
}

func (x *OrganisationId) Reset() {
	*x = OrganisationId{}
	if protoimpl.UnsafeEnabled {
		mi := &file_com_coralogixapis_aaa_organisations_v2_types_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *OrganisationId) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*OrganisationId) ProtoMessage() {}

func (x *OrganisationId) ProtoReflect() protoreflect.Message {
	mi := &file_com_coralogixapis_aaa_organisations_v2_types_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use OrganisationId.ProtoReflect.Descriptor instead.
func (*OrganisationId) Descriptor() ([]byte, []int) {
	return file_com_coralogixapis_aaa_organisations_v2_types_proto_rawDescGZIP(), []int{3}
}

func (x *OrganisationId) GetId() string {
	if x != nil {
		return x.Id
	}
	return ""
}

type UserId struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Id string `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
}

func (x *UserId) Reset() {
	*x = UserId{}
	if protoimpl.UnsafeEnabled {
		mi := &file_com_coralogixapis_aaa_organisations_v2_types_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *UserId) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*UserId) ProtoMessage() {}

func (x *UserId) ProtoReflect() protoreflect.Message {
	mi := &file_com_coralogixapis_aaa_organisations_v2_types_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use UserId.ProtoReflect.Descriptor instead.
func (*UserId) Descriptor() ([]byte, []int) {
	return file_com_coralogixapis_aaa_organisations_v2_types_proto_rawDescGZIP(), []int{4}
}

func (x *UserId) GetId() string {
	if x != nil {
		return x.Id
	}
	return ""
}

type UserAccountId struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Id uint32 `protobuf:"varint,1,opt,name=id,proto3" json:"id,omitempty"`
}

func (x *UserAccountId) Reset() {
	*x = UserAccountId{}
	if protoimpl.UnsafeEnabled {
		mi := &file_com_coralogixapis_aaa_organisations_v2_types_proto_msgTypes[5]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *UserAccountId) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*UserAccountId) ProtoMessage() {}

func (x *UserAccountId) ProtoReflect() protoreflect.Message {
	mi := &file_com_coralogixapis_aaa_organisations_v2_types_proto_msgTypes[5]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use UserAccountId.ProtoReflect.Descriptor instead.
func (*UserAccountId) Descriptor() ([]byte, []int) {
	return file_com_coralogixapis_aaa_organisations_v2_types_proto_rawDescGZIP(), []int{5}
}

func (x *UserAccountId) GetId() uint32 {
	if x != nil {
		return x.Id
	}
	return 0
}

type User struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	FirstName     string         `protobuf:"bytes,1,opt,name=first_name,json=firstName,proto3" json:"first_name,omitempty"`
	LastName      string         `protobuf:"bytes,2,opt,name=last_name,json=lastName,proto3" json:"last_name,omitempty"`
	Username      string         `protobuf:"bytes,3,opt,name=username,proto3" json:"username,omitempty"`
	UserAccountId *UserAccountId `protobuf:"bytes,4,opt,name=user_account_id,json=userAccountId,proto3" json:"user_account_id,omitempty"`
}

func (x *User) Reset() {
	*x = User{}
	if protoimpl.UnsafeEnabled {
		mi := &file_com_coralogixapis_aaa_organisations_v2_types_proto_msgTypes[6]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *User) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*User) ProtoMessage() {}

func (x *User) ProtoReflect() protoreflect.Message {
	mi := &file_com_coralogixapis_aaa_organisations_v2_types_proto_msgTypes[6]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use User.ProtoReflect.Descriptor instead.
func (*User) Descriptor() ([]byte, []int) {
	return file_com_coralogixapis_aaa_organisations_v2_types_proto_rawDescGZIP(), []int{6}
}

func (x *User) GetFirstName() string {
	if x != nil {
		return x.FirstName
	}
	return ""
}

func (x *User) GetLastName() string {
	if x != nil {
		return x.LastName
	}
	return ""
}

func (x *User) GetUsername() string {
	if x != nil {
		return x.Username
	}
	return ""
}

func (x *User) GetUserAccountId() *UserAccountId {
	if x != nil {
		return x.UserAccountId
	}
	return nil
}

type TeamCount struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	TeamId          *TeamId `protobuf:"bytes,1,opt,name=team_id,json=teamId,proto3" json:"team_id,omitempty"`
	TeamMemberCount uint32  `protobuf:"varint,2,opt,name=team_member_count,json=teamMemberCount,proto3" json:"team_member_count,omitempty"`
}

func (x *TeamCount) Reset() {
	*x = TeamCount{}
	if protoimpl.UnsafeEnabled {
		mi := &file_com_coralogixapis_aaa_organisations_v2_types_proto_msgTypes[7]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *TeamCount) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*TeamCount) ProtoMessage() {}

func (x *TeamCount) ProtoReflect() protoreflect.Message {
	mi := &file_com_coralogixapis_aaa_organisations_v2_types_proto_msgTypes[7]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use TeamCount.ProtoReflect.Descriptor instead.
func (*TeamCount) Descriptor() ([]byte, []int) {
	return file_com_coralogixapis_aaa_organisations_v2_types_proto_rawDescGZIP(), []int{7}
}

func (x *TeamCount) GetTeamId() *TeamId {
	if x != nil {
		return x.TeamId
	}
	return nil
}

func (x *TeamCount) GetTeamMemberCount() uint32 {
	if x != nil {
		return x.TeamMemberCount
	}
	return 0
}

var File_com_coralogixapis_aaa_organisations_v2_types_proto protoreflect.FileDescriptor

var file_com_coralogixapis_aaa_organisations_v2_types_proto_rawDesc = []byte{
	0x0a, 0x32, 0x63, 0x6f, 0x6d, 0x2f, 0x63, 0x6f, 0x72, 0x61, 0x6c, 0x6f, 0x67, 0x69, 0x78, 0x61,
	0x70, 0x69, 0x73, 0x2f, 0x61, 0x61, 0x61, 0x2f, 0x6f, 0x72, 0x67, 0x61, 0x6e, 0x69, 0x73, 0x61,
	0x74, 0x69, 0x6f, 0x6e, 0x73, 0x2f, 0x76, 0x32, 0x2f, 0x74, 0x79, 0x70, 0x65, 0x73, 0x2e, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x12, 0x26, 0x63, 0x6f, 0x6d, 0x2e, 0x63, 0x6f, 0x72, 0x61, 0x6c, 0x6f,
	0x67, 0x69, 0x78, 0x61, 0x70, 0x69, 0x73, 0x2e, 0x61, 0x61, 0x61, 0x2e, 0x6f, 0x72, 0x67, 0x61,
	0x6e, 0x69, 0x73, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x2e, 0x76, 0x32, 0x22, 0xdb, 0x02, 0x0a,
	0x04, 0x54, 0x65, 0x61, 0x6d, 0x12, 0x3e, 0x0a, 0x02, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28,
	0x0b, 0x32, 0x2e, 0x2e, 0x63, 0x6f, 0x6d, 0x2e, 0x63, 0x6f, 0x72, 0x61, 0x6c, 0x6f, 0x67, 0x69,
	0x78, 0x61, 0x70, 0x69, 0x73, 0x2e, 0x61, 0x61, 0x61, 0x2e, 0x6f, 0x72, 0x67, 0x61, 0x6e, 0x69,
	0x73, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x2e, 0x76, 0x32, 0x2e, 0x54, 0x65, 0x61, 0x6d, 0x49,
	0x64, 0x52, 0x02, 0x69, 0x64, 0x12, 0x1d, 0x0a, 0x0a, 0x63, 0x6c, 0x75, 0x73, 0x74, 0x65, 0x72,
	0x5f, 0x69, 0x64, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x09, 0x63, 0x6c, 0x75, 0x73, 0x74,
	0x65, 0x72, 0x49, 0x64, 0x12, 0x19, 0x0a, 0x05, 0x71, 0x75, 0x6f, 0x74, 0x61, 0x18, 0x03, 0x20,
	0x01, 0x28, 0x02, 0x48, 0x00, 0x52, 0x05, 0x71, 0x75, 0x6f, 0x74, 0x61, 0x88, 0x01, 0x01, 0x12,
	0x21, 0x0a, 0x09, 0x72, 0x65, 0x74, 0x65, 0x6e, 0x74, 0x69, 0x6f, 0x6e, 0x18, 0x04, 0x20, 0x01,
	0x28, 0x05, 0x48, 0x01, 0x52, 0x09, 0x72, 0x65, 0x74, 0x65, 0x6e, 0x74, 0x69, 0x6f, 0x6e, 0x88,
	0x01, 0x01, 0x12, 0x52, 0x0a, 0x09, 0x70, 0x6c, 0x61, 0x6e, 0x5f, 0x74, 0x79, 0x70, 0x65, 0x18,
	0x05, 0x20, 0x01, 0x28, 0x0e, 0x32, 0x30, 0x2e, 0x63, 0x6f, 0x6d, 0x2e, 0x63, 0x6f, 0x72, 0x61,
	0x6c, 0x6f, 0x67, 0x69, 0x78, 0x61, 0x70, 0x69, 0x73, 0x2e, 0x61, 0x61, 0x61, 0x2e, 0x6f, 0x72,
	0x67, 0x61, 0x6e, 0x69, 0x73, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x2e, 0x76, 0x32, 0x2e, 0x50,
	0x6c, 0x61, 0x6e, 0x54, 0x79, 0x70, 0x65, 0x48, 0x02, 0x52, 0x08, 0x70, 0x6c, 0x61, 0x6e, 0x54,
	0x79, 0x70, 0x65, 0x88, 0x01, 0x01, 0x12, 0x28, 0x0a, 0x10, 0x69, 0x73, 0x5f, 0x61, 0x75, 0x64,
	0x69, 0x74, 0x69, 0x6e, 0x67, 0x5f, 0x74, 0x65, 0x61, 0x6d, 0x18, 0x06, 0x20, 0x01, 0x28, 0x08,
	0x52, 0x0e, 0x69, 0x73, 0x41, 0x75, 0x64, 0x69, 0x74, 0x69, 0x6e, 0x67, 0x54, 0x65, 0x61, 0x6d,
	0x12, 0x12, 0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x07, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04,
	0x6e, 0x61, 0x6d, 0x65, 0x42, 0x08, 0x0a, 0x06, 0x5f, 0x71, 0x75, 0x6f, 0x74, 0x61, 0x42, 0x0c,
	0x0a, 0x0a, 0x5f, 0x72, 0x65, 0x74, 0x65, 0x6e, 0x74, 0x69, 0x6f, 0x6e, 0x42, 0x0c, 0x0a, 0x0a,
	0x5f, 0x70, 0x6c, 0x61, 0x6e, 0x5f, 0x74, 0x79, 0x70, 0x65, 0x22, 0xd8, 0x01, 0x0a, 0x08, 0x54,
	0x65, 0x61, 0x6d, 0x49, 0x6e, 0x66, 0x6f, 0x12, 0x3e, 0x0a, 0x02, 0x69, 0x64, 0x18, 0x01, 0x20,
	0x01, 0x28, 0x0b, 0x32, 0x2e, 0x2e, 0x63, 0x6f, 0x6d, 0x2e, 0x63, 0x6f, 0x72, 0x61, 0x6c, 0x6f,
	0x67, 0x69, 0x78, 0x61, 0x70, 0x69, 0x73, 0x2e, 0x61, 0x61, 0x61, 0x2e, 0x6f, 0x72, 0x67, 0x61,
	0x6e, 0x69, 0x73, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x2e, 0x76, 0x32, 0x2e, 0x54, 0x65, 0x61,
	0x6d, 0x49, 0x64, 0x52, 0x02, 0x69, 0x64, 0x12, 0x5f, 0x0a, 0x0f, 0x6f, 0x72, 0x67, 0x61, 0x6e,
	0x69, 0x73, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x5f, 0x69, 0x64, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b,
	0x32, 0x36, 0x2e, 0x63, 0x6f, 0x6d, 0x2e, 0x63, 0x6f, 0x72, 0x61, 0x6c, 0x6f, 0x67, 0x69, 0x78,
	0x61, 0x70, 0x69, 0x73, 0x2e, 0x61, 0x61, 0x61, 0x2e, 0x6f, 0x72, 0x67, 0x61, 0x6e, 0x69, 0x73,
	0x61, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x2e, 0x76, 0x32, 0x2e, 0x4f, 0x72, 0x67, 0x61, 0x6e, 0x69,
	0x73, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x49, 0x64, 0x52, 0x0e, 0x6f, 0x72, 0x67, 0x61, 0x6e, 0x69,
	0x73, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x49, 0x64, 0x12, 0x2b, 0x0a, 0x11, 0x6f, 0x72, 0x67, 0x61,
	0x6e, 0x69, 0x73, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x5f, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x03, 0x20,
	0x01, 0x28, 0x09, 0x52, 0x10, 0x6f, 0x72, 0x67, 0x61, 0x6e, 0x69, 0x73, 0x61, 0x74, 0x69, 0x6f,
	0x6e, 0x4e, 0x61, 0x6d, 0x65, 0x22, 0x18, 0x0a, 0x06, 0x54, 0x65, 0x61, 0x6d, 0x49, 0x64, 0x12,
	0x0e, 0x0a, 0x02, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0d, 0x52, 0x02, 0x69, 0x64, 0x22,
	0x20, 0x0a, 0x0e, 0x4f, 0x72, 0x67, 0x61, 0x6e, 0x69, 0x73, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x49,
	0x64, 0x12, 0x0e, 0x0a, 0x02, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x02, 0x69,
	0x64, 0x22, 0x18, 0x0a, 0x06, 0x55, 0x73, 0x65, 0x72, 0x49, 0x64, 0x12, 0x0e, 0x0a, 0x02, 0x69,
	0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x02, 0x69, 0x64, 0x22, 0x1f, 0x0a, 0x0d, 0x55,
	0x73, 0x65, 0x72, 0x41, 0x63, 0x63, 0x6f, 0x75, 0x6e, 0x74, 0x49, 0x64, 0x12, 0x0e, 0x0a, 0x02,
	0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0d, 0x52, 0x02, 0x69, 0x64, 0x22, 0xbd, 0x01, 0x0a,
	0x04, 0x55, 0x73, 0x65, 0x72, 0x12, 0x1d, 0x0a, 0x0a, 0x66, 0x69, 0x72, 0x73, 0x74, 0x5f, 0x6e,
	0x61, 0x6d, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x09, 0x66, 0x69, 0x72, 0x73, 0x74,
	0x4e, 0x61, 0x6d, 0x65, 0x12, 0x1b, 0x0a, 0x09, 0x6c, 0x61, 0x73, 0x74, 0x5f, 0x6e, 0x61, 0x6d,
	0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x08, 0x6c, 0x61, 0x73, 0x74, 0x4e, 0x61, 0x6d,
	0x65, 0x12, 0x1a, 0x0a, 0x08, 0x75, 0x73, 0x65, 0x72, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x03, 0x20,
	0x01, 0x28, 0x09, 0x52, 0x08, 0x75, 0x73, 0x65, 0x72, 0x6e, 0x61, 0x6d, 0x65, 0x12, 0x5d, 0x0a,
	0x0f, 0x75, 0x73, 0x65, 0x72, 0x5f, 0x61, 0x63, 0x63, 0x6f, 0x75, 0x6e, 0x74, 0x5f, 0x69, 0x64,
	0x18, 0x04, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x35, 0x2e, 0x63, 0x6f, 0x6d, 0x2e, 0x63, 0x6f, 0x72,
	0x61, 0x6c, 0x6f, 0x67, 0x69, 0x78, 0x61, 0x70, 0x69, 0x73, 0x2e, 0x61, 0x61, 0x61, 0x2e, 0x6f,
	0x72, 0x67, 0x61, 0x6e, 0x69, 0x73, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x2e, 0x76, 0x32, 0x2e,
	0x55, 0x73, 0x65, 0x72, 0x41, 0x63, 0x63, 0x6f, 0x75, 0x6e, 0x74, 0x49, 0x64, 0x52, 0x0d, 0x75,
	0x73, 0x65, 0x72, 0x41, 0x63, 0x63, 0x6f, 0x75, 0x6e, 0x74, 0x49, 0x64, 0x22, 0x80, 0x01, 0x0a,
	0x09, 0x54, 0x65, 0x61, 0x6d, 0x43, 0x6f, 0x75, 0x6e, 0x74, 0x12, 0x47, 0x0a, 0x07, 0x74, 0x65,
	0x61, 0x6d, 0x5f, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x2e, 0x2e, 0x63, 0x6f,
	0x6d, 0x2e, 0x63, 0x6f, 0x72, 0x61, 0x6c, 0x6f, 0x67, 0x69, 0x78, 0x61, 0x70, 0x69, 0x73, 0x2e,
	0x61, 0x61, 0x61, 0x2e, 0x6f, 0x72, 0x67, 0x61, 0x6e, 0x69, 0x73, 0x61, 0x74, 0x69, 0x6f, 0x6e,
	0x73, 0x2e, 0x76, 0x32, 0x2e, 0x54, 0x65, 0x61, 0x6d, 0x49, 0x64, 0x52, 0x06, 0x74, 0x65, 0x61,
	0x6d, 0x49, 0x64, 0x12, 0x2a, 0x0a, 0x11, 0x74, 0x65, 0x61, 0x6d, 0x5f, 0x6d, 0x65, 0x6d, 0x62,
	0x65, 0x72, 0x5f, 0x63, 0x6f, 0x75, 0x6e, 0x74, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0d, 0x52, 0x0f,
	0x74, 0x65, 0x61, 0x6d, 0x4d, 0x65, 0x6d, 0x62, 0x65, 0x72, 0x43, 0x6f, 0x75, 0x6e, 0x74, 0x2a,
	0x68, 0x0a, 0x08, 0x50, 0x6c, 0x61, 0x6e, 0x54, 0x79, 0x70, 0x65, 0x12, 0x19, 0x0a, 0x15, 0x50,
	0x4c, 0x41, 0x4e, 0x5f, 0x54, 0x59, 0x50, 0x45, 0x5f, 0x55, 0x4e, 0x53, 0x50, 0x45, 0x43, 0x49,
	0x46, 0x49, 0x45, 0x44, 0x10, 0x00, 0x12, 0x18, 0x0a, 0x14, 0x50, 0x4c, 0x41, 0x4e, 0x5f, 0x54,
	0x59, 0x50, 0x45, 0x5f, 0x50, 0x4f, 0x53, 0x54, 0x5f, 0x54, 0x52, 0x49, 0x41, 0x4c, 0x10, 0x01,
	0x12, 0x12, 0x0a, 0x0e, 0x50, 0x4c, 0x41, 0x4e, 0x5f, 0x54, 0x59, 0x50, 0x45, 0x5f, 0x50, 0x4c,
	0x41, 0x4e, 0x10, 0x02, 0x12, 0x13, 0x0a, 0x0f, 0x50, 0x4c, 0x41, 0x4e, 0x5f, 0x54, 0x59, 0x50,
	0x45, 0x5f, 0x54, 0x52, 0x49, 0x41, 0x4c, 0x10, 0x03, 0x42, 0x04, 0x5a, 0x02, 0x2e, 0x2f, 0x62,
	0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_com_coralogixapis_aaa_organisations_v2_types_proto_rawDescOnce sync.Once
	file_com_coralogixapis_aaa_organisations_v2_types_proto_rawDescData = file_com_coralogixapis_aaa_organisations_v2_types_proto_rawDesc
)

func file_com_coralogixapis_aaa_organisations_v2_types_proto_rawDescGZIP() []byte {
	file_com_coralogixapis_aaa_organisations_v2_types_proto_rawDescOnce.Do(func() {
		file_com_coralogixapis_aaa_organisations_v2_types_proto_rawDescData = protoimpl.X.CompressGZIP(file_com_coralogixapis_aaa_organisations_v2_types_proto_rawDescData)
	})
	return file_com_coralogixapis_aaa_organisations_v2_types_proto_rawDescData
}

var file_com_coralogixapis_aaa_organisations_v2_types_proto_enumTypes = make([]protoimpl.EnumInfo, 1)
var file_com_coralogixapis_aaa_organisations_v2_types_proto_msgTypes = make([]protoimpl.MessageInfo, 8)
var file_com_coralogixapis_aaa_organisations_v2_types_proto_goTypes = []interface{}{
	(PlanType)(0),          // 0: com.coralogixapis.aaa.organisations.v2.PlanType
	(*Team)(nil),           // 1: com.coralogixapis.aaa.organisations.v2.Team
	(*TeamInfo)(nil),       // 2: com.coralogixapis.aaa.organisations.v2.TeamInfo
	(*TeamId)(nil),         // 3: com.coralogixapis.aaa.organisations.v2.TeamId
	(*OrganisationId)(nil), // 4: com.coralogixapis.aaa.organisations.v2.OrganisationId
	(*UserId)(nil),         // 5: com.coralogixapis.aaa.organisations.v2.UserId
	(*UserAccountId)(nil),  // 6: com.coralogixapis.aaa.organisations.v2.UserAccountId
	(*User)(nil),           // 7: com.coralogixapis.aaa.organisations.v2.User
	(*TeamCount)(nil),      // 8: com.coralogixapis.aaa.organisations.v2.TeamCount
}
var file_com_coralogixapis_aaa_organisations_v2_types_proto_depIdxs = []int32{
	3, // 0: com.coralogixapis.aaa.organisations.v2.Team.id:type_name -> com.coralogixapis.aaa.organisations.v2.TeamId
	0, // 1: com.coralogixapis.aaa.organisations.v2.Team.plan_type:type_name -> com.coralogixapis.aaa.organisations.v2.PlanType
	3, // 2: com.coralogixapis.aaa.organisations.v2.TeamInfo.id:type_name -> com.coralogixapis.aaa.organisations.v2.TeamId
	4, // 3: com.coralogixapis.aaa.organisations.v2.TeamInfo.organisation_id:type_name -> com.coralogixapis.aaa.organisations.v2.OrganisationId
	6, // 4: com.coralogixapis.aaa.organisations.v2.User.user_account_id:type_name -> com.coralogixapis.aaa.organisations.v2.UserAccountId
	3, // 5: com.coralogixapis.aaa.organisations.v2.TeamCount.team_id:type_name -> com.coralogixapis.aaa.organisations.v2.TeamId
	6, // [6:6] is the sub-list for method output_type
	6, // [6:6] is the sub-list for method input_type
	6, // [6:6] is the sub-list for extension type_name
	6, // [6:6] is the sub-list for extension extendee
	0, // [0:6] is the sub-list for field type_name
}

func init() { file_com_coralogixapis_aaa_organisations_v2_types_proto_init() }
func file_com_coralogixapis_aaa_organisations_v2_types_proto_init() {
	if File_com_coralogixapis_aaa_organisations_v2_types_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_com_coralogixapis_aaa_organisations_v2_types_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Team); i {
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
		file_com_coralogixapis_aaa_organisations_v2_types_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*TeamInfo); i {
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
		file_com_coralogixapis_aaa_organisations_v2_types_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*TeamId); i {
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
		file_com_coralogixapis_aaa_organisations_v2_types_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*OrganisationId); i {
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
		file_com_coralogixapis_aaa_organisations_v2_types_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*UserId); i {
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
		file_com_coralogixapis_aaa_organisations_v2_types_proto_msgTypes[5].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*UserAccountId); i {
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
		file_com_coralogixapis_aaa_organisations_v2_types_proto_msgTypes[6].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*User); i {
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
		file_com_coralogixapis_aaa_organisations_v2_types_proto_msgTypes[7].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*TeamCount); i {
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
	file_com_coralogixapis_aaa_organisations_v2_types_proto_msgTypes[0].OneofWrappers = []interface{}{}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_com_coralogixapis_aaa_organisations_v2_types_proto_rawDesc,
			NumEnums:      1,
			NumMessages:   8,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_com_coralogixapis_aaa_organisations_v2_types_proto_goTypes,
		DependencyIndexes: file_com_coralogixapis_aaa_organisations_v2_types_proto_depIdxs,
		EnumInfos:         file_com_coralogixapis_aaa_organisations_v2_types_proto_enumTypes,
		MessageInfos:      file_com_coralogixapis_aaa_organisations_v2_types_proto_msgTypes,
	}.Build()
	File_com_coralogixapis_aaa_organisations_v2_types_proto = out.File
	file_com_coralogixapis_aaa_organisations_v2_types_proto_rawDesc = nil
	file_com_coralogixapis_aaa_organisations_v2_types_proto_goTypes = nil
	file_com_coralogixapis_aaa_organisations_v2_types_proto_depIdxs = nil
}