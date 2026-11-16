package member_service

import (
	"context"
	"net/http"
)

type MemberStatus string

const (
	MEMBERSTATUS_MEMBER_STATUS_UNSPECIFIED MemberStatus = "MEMBER_STATUS_UNSPECIFIED"
	MEMBERSTATUS_MEMBER_STATUS_ACTIVE      MemberStatus = "MEMBER_STATUS_ACTIVE"
	MEMBERSTATUS_MEMBER_STATUS_INACTIVE    MemberStatus = "MEMBER_STATUS_INACTIVE"
)

type LoginMode string

const (
	LOGINMODE_LOGIN_MODE_UNSPECIFIED LoginMode = "LOGIN_MODE_UNSPECIFIED"
	LOGINMODE_LOGIN_MODE_SSO         LoginMode = "LOGIN_MODE_SSO"
	LOGINMODE_LOGIN_MODE_LOCAL       LoginMode = "LOGIN_MODE_LOCAL"
)

type OnboardingMode string

const (
	ONBOARDINGMODE_ONBOARDING_MODE_UNSPECIFIED OnboardingMode = "ONBOARDING_MODE_UNSPECIFIED"
	ONBOARDINGMODE_ONBOARDING_MODE_INVITE_LINK OnboardingMode = "ONBOARDING_MODE_INVITE_LINK"
	ONBOARDINGMODE_ONBOARDING_MODE_NO_INVITE   OnboardingMode = "ONBOARDING_MODE_NO_INVITE"
)

type CreateMemberStatus string

const (
	CREATEMEMBERSTATUS_CREATE_MEMBER_STATUS_UNSPECIFIED CreateMemberStatus = "CREATE_MEMBER_STATUS_UNSPECIFIED"
	CREATEMEMBERSTATUS_CREATE_MEMBER_STATUS_CREATED     CreateMemberStatus = "CREATE_MEMBER_STATUS_CREATED"
	CREATEMEMBERSTATUS_CREATE_MEMBER_STATUS_INVITED     CreateMemberStatus = "CREATE_MEMBER_STATUS_INVITED"
	CREATEMEMBERSTATUS_CREATE_MEMBER_STATUS_FAILED      CreateMemberStatus = "CREATE_MEMBER_STATUS_FAILED"
)

type Member struct {
	MemberId    *string
	Login       *string
	DisplayName *string
	GroupIds    []int64
	Status      *MemberStatus
	LoginModes  []LoginMode
}

type MemberTemplate struct {
	Login       *string
	DisplayName *string
	Status      *MemberStatus
	LoginModes  []LoginMode
}

type CreateMemberRequest struct {
	Template       *MemberTemplate
	OnboardingMode *OnboardingMode
}

type UpdateMemberRequest struct {
	MemberId *string
	Template *MemberTemplate
}

type CreateMemberResult struct {
	Status   *CreateMemberStatus
	Member   *Member
	Message  *string
	Username *string
}

type SearchMembersResponse struct {
	NextPageToken *string
	Members       []Member
}

type CreateMembersResponse struct {
	Results []CreateMemberResult
}

type UpdateMembersResponse struct {
}

type UpdateMemberStatusRequest struct {
	Status    *MemberStatus
	MemberIds []string
}

// The service records every call for the runtime tests.
type MemberServiceAPIService struct {
	SearchResponses  []*SearchMembersResponse
	SearchPageCalls  int
	SearchPageTokens []string
	SearchStatuses   []MemberStatus
	CreateResults    *CreateMembersResponse
	CreateBodies     [][]CreateMemberRequest
	UpdateBodies     []UpdateMemberRequest
	DeleteBody       *UpdateMemberStatusRequest
}

type ApiMemberServiceSearchMembersRequest struct {
	client *MemberServiceAPIService
}

type ApiMemberServiceCreateMembersRequest struct {
	client *MemberServiceAPIService
}

type ApiMemberServiceUpdateMembersRequest struct {
	client *MemberServiceAPIService
}

type ApiMemberServiceUpdateMembersStatusesRequest struct {
	client *MemberServiceAPIService
}

func (c *MemberServiceAPIService) MemberServiceSearchMembers(ctx context.Context) ApiMemberServiceSearchMembersRequest {
	return ApiMemberServiceSearchMembersRequest{client: c}
}

func (c *MemberServiceAPIService) MemberServiceCreateMembers(ctx context.Context) ApiMemberServiceCreateMembersRequest {
	return ApiMemberServiceCreateMembersRequest{client: c}
}

func (c *MemberServiceAPIService) MemberServiceUpdateMembers(ctx context.Context) ApiMemberServiceUpdateMembersRequest {
	return ApiMemberServiceUpdateMembersRequest{client: c}
}

func (c *MemberServiceAPIService) MemberServiceUpdateMembersStatuses(ctx context.Context) ApiMemberServiceUpdateMembersStatusesRequest {
	return ApiMemberServiceUpdateMembersStatusesRequest{client: c}
}

func (r ApiMemberServiceSearchMembersRequest) Status(status MemberStatus) ApiMemberServiceSearchMembersRequest {
	r.client.SearchStatuses = append(r.client.SearchStatuses, status)
	return r
}

func (r ApiMemberServiceSearchMembersRequest) PageSize(pageSize int64) ApiMemberServiceSearchMembersRequest {
	_ = pageSize
	return r
}

func (r ApiMemberServiceSearchMembersRequest) PageToken(pageToken string) ApiMemberServiceSearchMembersRequest {
	r.client.SearchPageTokens = append(r.client.SearchPageTokens, pageToken)
	return r
}

func (r ApiMemberServiceSearchMembersRequest) Execute() (*SearchMembersResponse, *http.Response, error) {
	r.client.SearchPageCalls++
	resp := r.client.SearchResponses[0]
	r.client.SearchResponses = r.client.SearchResponses[1:]
	return resp, nil, nil
}

func (r ApiMemberServiceCreateMembersRequest) CreateMemberRequest(createMemberRequest []CreateMemberRequest) ApiMemberServiceCreateMembersRequest {
	r.client.CreateBodies = append(r.client.CreateBodies, createMemberRequest)
	return r
}

func (r ApiMemberServiceCreateMembersRequest) Execute() (*CreateMembersResponse, *http.Response, error) {
	return r.client.CreateResults, nil, nil
}

func (r ApiMemberServiceUpdateMembersRequest) UpdateMemberRequest(updateMemberRequest []UpdateMemberRequest) ApiMemberServiceUpdateMembersRequest {
	r.client.UpdateBodies = updateMemberRequest
	return r
}

func (r ApiMemberServiceUpdateMembersRequest) Execute() (map[string]interface{}, *http.Response, error) {
	return nil, nil, nil
}

func (r ApiMemberServiceUpdateMembersStatusesRequest) UpdateMemberStatusRequest(updateMemberStatusRequest UpdateMemberStatusRequest) ApiMemberServiceUpdateMembersStatusesRequest {
	r.client.DeleteBody = &updateMemberStatusRequest
	return r
}

func (r ApiMemberServiceUpdateMembersStatusesRequest) Execute() (map[string]interface{}, *http.Response, error) {
	return nil, nil, nil
}
