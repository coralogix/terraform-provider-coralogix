package member_hook_service

import (
	"context"
	"fmt"
	"net/http"
)

type MemberStatus string

const (
	MEMBERSTATUS_MEMBER_STATUS_UNSPECIFIED MemberStatus = "MEMBER_STATUS_UNSPECIFIED"
	MEMBERSTATUS_MEMBER_STATUS_ACTIVE      MemberStatus = "MEMBER_STATUS_ACTIVE"
	MEMBERSTATUS_MEMBER_STATUS_INACTIVE    MemberStatus = "MEMBER_STATUS_INACTIVE"
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

type LoginMode string

const (
	LOGINMODE_LOGIN_MODE_UNSPECIFIED LoginMode = "LOGIN_MODE_UNSPECIFIED"
	LOGINMODE_LOGIN_MODE_SSO         LoginMode = "LOGIN_MODE_SSO"
	LOGINMODE_LOGIN_MODE_LOCAL       LoginMode = "LOGIN_MODE_LOCAL"
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

// The response of the search: the next page token is a value, which is the branch this
// fixture exercises.
type MemberHookSearchResponse struct {
	NextPageToken int64
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
type MemberHookServiceAPIService struct {
	SearchResponses  []*MemberHookSearchResponse
	SearchPageCalls  int
	SearchPageTokens []string
	SearchStatuses   []MemberStatus
	CreateResults    *CreateMembersResponse
	CreateBodies     [][]CreateMemberRequest
	UpdateBodies     []UpdateMemberRequest
	DeleteBody       *UpdateMemberStatusRequest
}

type ApiMemberHookServiceSearchMembersRequest struct {
	client *MemberHookServiceAPIService
}

type ApiMemberHookServiceCreateMembersRequest struct {
	client *MemberHookServiceAPIService
}

type ApiMemberHookServiceUpdateMembersRequest struct {
	client *MemberHookServiceAPIService
}

type ApiMemberHookServiceUpdateMembersStatusesRequest struct {
	client *MemberHookServiceAPIService
}

func (c *MemberHookServiceAPIService) MemberHookServiceSearchMembers(ctx context.Context) ApiMemberHookServiceSearchMembersRequest {
	return ApiMemberHookServiceSearchMembersRequest{client: c}
}

func (c *MemberHookServiceAPIService) MemberHookServiceCreateMembers(ctx context.Context) ApiMemberHookServiceCreateMembersRequest {
	return ApiMemberHookServiceCreateMembersRequest{client: c}
}

func (c *MemberHookServiceAPIService) MemberHookServiceUpdateMembers(ctx context.Context) ApiMemberHookServiceUpdateMembersRequest {
	return ApiMemberHookServiceUpdateMembersRequest{client: c}
}

func (c *MemberHookServiceAPIService) MemberHookServiceUpdateMembersStatuses(ctx context.Context) ApiMemberHookServiceUpdateMembersStatusesRequest {
	return ApiMemberHookServiceUpdateMembersStatusesRequest{client: c}
}

func (r ApiMemberHookServiceSearchMembersRequest) PageSize(pageSize int64) ApiMemberHookServiceSearchMembersRequest {
	_ = pageSize
	return r
}

func (r ApiMemberHookServiceSearchMembersRequest) Status(status MemberStatus) ApiMemberHookServiceSearchMembersRequest {
	r.client.SearchStatuses = append(r.client.SearchStatuses, status)
	return r
}

func (r ApiMemberHookServiceSearchMembersRequest) PageToken(pageToken int64) ApiMemberHookServiceSearchMembersRequest {
	r.client.SearchPageTokens = append(r.client.SearchPageTokens, fmt.Sprintf("%d", pageToken))
	return r
}

func (r ApiMemberHookServiceSearchMembersRequest) Execute() (*MemberHookSearchResponse, *http.Response, error) {
	r.client.SearchPageCalls++
	resp := r.client.SearchResponses[0]
	r.client.SearchResponses = r.client.SearchResponses[1:]
	return resp, nil, nil
}

func (r ApiMemberHookServiceCreateMembersRequest) CreateMemberRequest(createMemberRequest []CreateMemberRequest) ApiMemberHookServiceCreateMembersRequest {
	r.client.CreateBodies = append(r.client.CreateBodies, createMemberRequest)
	return r
}

func (r ApiMemberHookServiceCreateMembersRequest) Execute() (*CreateMembersResponse, *http.Response, error) {
	return r.client.CreateResults, nil, nil
}

func (r ApiMemberHookServiceUpdateMembersRequest) UpdateMemberRequest(updateMemberRequest []UpdateMemberRequest) ApiMemberHookServiceUpdateMembersRequest {
	r.client.UpdateBodies = updateMemberRequest
	return r
}

func (r ApiMemberHookServiceUpdateMembersRequest) Execute() (map[string]interface{}, *http.Response, error) {
	return nil, nil, nil
}

func (r ApiMemberHookServiceUpdateMembersStatusesRequest) UpdateMemberStatusRequest(updateMemberStatusRequest UpdateMemberStatusRequest) ApiMemberHookServiceUpdateMembersStatusesRequest {
	r.client.DeleteBody = &updateMemberStatusRequest
	return r
}

func (r ApiMemberHookServiceUpdateMembersStatusesRequest) Execute() (map[string]interface{}, *http.Response, error) {
	return nil, nil, nil
}
