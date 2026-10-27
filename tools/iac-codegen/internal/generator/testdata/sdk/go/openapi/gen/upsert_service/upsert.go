package upsert_service

import (
	"context"
	"net/http"
)

type Target struct {
	Meta  Meta
	Other *Other
	Spec  *Spec
}

type Meta struct {
	Enabled *bool
	Version *string
}

type Spec struct {
	Name  string
	Token *string
	Zone  *string
}

type Other struct {
	Crn *string
}

type SetTargetRequest struct {
	Enabled bool
	Spec    Spec
}

type GetTargetResponse struct {
	Target Target
}

type SetTargetResponse struct {
	Target Target
}

// Stored is the target of the fake API, nil when the company has none. Set replaces it, and Get
// returns it, or a target with the other arm when there is none. Sets lists the bodies of Set.
// Tests of the generated resource set and read them.
var (
	Stored *Target
	Sets   []SetTargetRequest
)

type UpsertServiceAPIService struct{}

type ApiUpsertServiceGetTargetRequest struct{}
type ApiUpsertServiceSetTargetRequest struct{ body SetTargetRequest }

func (*UpsertServiceAPIService) UpsertServiceGetTarget(ctx context.Context) ApiUpsertServiceGetTargetRequest {
	_ = ctx
	return ApiUpsertServiceGetTargetRequest{}
}

func (*UpsertServiceAPIService) UpsertServiceSetTarget(ctx context.Context) ApiUpsertServiceSetTargetRequest {
	_ = ctx
	return ApiUpsertServiceSetTargetRequest{}
}

func (r ApiUpsertServiceSetTargetRequest) SetTargetRequest(setTargetRequest SetTargetRequest) ApiUpsertServiceSetTargetRequest {
	r.body = setTargetRequest
	return r
}

func (ApiUpsertServiceGetTargetRequest) Execute() (*GetTargetResponse, *http.Response, error) {
	if Stored == nil {
		return &GetTargetResponse{Target: Target{Other: &Other{}}}, nil, nil
	}
	return &GetTargetResponse{Target: *Stored}, nil, nil
}

// Execute stores the body as the API does: it fills a zone that the body does not have, derives
// the token, and sets the version.
func (r ApiUpsertServiceSetTargetRequest) Execute() (*SetTargetResponse, *http.Response, error) {
	Sets = append(Sets, r.body)
	spec := r.body.Spec
	if spec.Zone == nil {
		zone := "server-zone"
		spec.Zone = &zone
	}
	token, version, enabled := "derived", "v1", r.body.Enabled
	spec.Token = &token
	Stored = &Target{Meta: Meta{Enabled: &enabled, Version: &version}, Spec: &spec}
	return &SetTargetResponse{Target: *Stored}, nil, nil
}
