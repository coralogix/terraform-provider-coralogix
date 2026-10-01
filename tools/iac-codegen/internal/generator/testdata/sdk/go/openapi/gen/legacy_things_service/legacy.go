package legacy_things_service

import (
	"context"
	"net/http"
	"time"
)

type LegacyKind string

const (
	LEGACYKIND_KIND_UNSPECIFIED LegacyKind = "KIND_UNSPECIFIED"
	LEGACYKIND_ALPHA            LegacyKind = "ALPHA"
	LEGACYKIND_BETA             LegacyKind = "BETA"
)

type LegacyTarget struct {
	ConnectorId *string
	Id          *string
}

type LegacyRule struct {
	Name    *string
	Kind    *LegacyKind
	Targets []LegacyTarget
}

type LegacyLabels struct {
	Env *string
}

type LegacyThing struct {
	Id         *string
	Name       *string
	CreateTime *time.Time
	Labels     *LegacyLabels
	Rules      []LegacyRule
}

type CreateLegacyThingRequest struct {
	Thing *LegacyThing
}

type ReplaceLegacyThingRequest struct {
	Thing *LegacyThing
}

type CreateLegacyThingResponse struct {
	Thing *LegacyThing
}

type GetLegacyThingResponse struct {
	Thing *LegacyThing
}

type ReplaceLegacyThingResponse struct {
	Thing *LegacyThing
}

type LegacyThingsServiceAPIService struct{}

type ApiLegacyThingsServiceCreateLegacyThingRequest struct{}
type ApiLegacyThingsServiceGetLegacyThingRequest struct{}
type ApiLegacyThingsServiceReplaceLegacyThingRequest struct{}
type ApiLegacyThingsServiceDeleteLegacyThingRequest struct{}

func (*LegacyThingsServiceAPIService) LegacyThingsServiceCreateLegacyThing(ctx context.Context) ApiLegacyThingsServiceCreateLegacyThingRequest {
	_ = ctx
	return ApiLegacyThingsServiceCreateLegacyThingRequest{}
}

func (*LegacyThingsServiceAPIService) LegacyThingsServiceGetLegacyThing(ctx context.Context, id string) ApiLegacyThingsServiceGetLegacyThingRequest {
	_, _ = ctx, id
	return ApiLegacyThingsServiceGetLegacyThingRequest{}
}

func (*LegacyThingsServiceAPIService) LegacyThingsServiceReplaceLegacyThing(ctx context.Context) ApiLegacyThingsServiceReplaceLegacyThingRequest {
	_ = ctx
	return ApiLegacyThingsServiceReplaceLegacyThingRequest{}
}

func (*LegacyThingsServiceAPIService) LegacyThingsServiceDeleteLegacyThing(ctx context.Context, id string) ApiLegacyThingsServiceDeleteLegacyThingRequest {
	_, _ = ctx, id
	return ApiLegacyThingsServiceDeleteLegacyThingRequest{}
}

func (r ApiLegacyThingsServiceCreateLegacyThingRequest) CreateLegacyThingRequest(createLegacyThingRequest CreateLegacyThingRequest) ApiLegacyThingsServiceCreateLegacyThingRequest {
	_ = createLegacyThingRequest
	return r
}

func (r ApiLegacyThingsServiceReplaceLegacyThingRequest) ReplaceLegacyThingRequest(replaceLegacyThingRequest ReplaceLegacyThingRequest) ApiLegacyThingsServiceReplaceLegacyThingRequest {
	_ = replaceLegacyThingRequest
	return r
}

func (ApiLegacyThingsServiceCreateLegacyThingRequest) Execute() (*CreateLegacyThingResponse, *http.Response, error) {
	return nil, nil, nil
}

func (ApiLegacyThingsServiceGetLegacyThingRequest) Execute() (*GetLegacyThingResponse, *http.Response, error) {
	return nil, nil, nil
}

func (ApiLegacyThingsServiceReplaceLegacyThingRequest) Execute() (*ReplaceLegacyThingResponse, *http.Response, error) {
	return nil, nil, nil
}

func (ApiLegacyThingsServiceDeleteLegacyThingRequest) Execute() (map[string]interface{}, *http.Response, error) {
	return nil, nil, nil
}
