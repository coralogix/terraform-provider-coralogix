package always_things_service

import (
	"context"
	"net/http"
)

type AlwaysThing struct {
	Enabled    bool
	Note       *string
	Limits     *AlwaysThingLimits
	Config     *AlwaysThingConfig
	SourceFile *string
	SourceUrl  *string
}

type AlwaysThingLimits struct {
	Max *int32
	Min *int32
}

type AlwaysThingLimitsUpdate struct {
	Max *int32
	Min *int32
}

type AlwaysThingConfig struct {
	Http  *AlwaysThingHttp
	Queue *AlwaysThingQueue
}

type AlwaysThingHttp struct {
	Endpoint string
}

type AlwaysThingQueue struct {
	Topic string
}

type AlwaysThingsServiceUpdateAlwaysThingRequest struct {
	Enabled    *bool
	Note       *string
	Limits     *AlwaysThingLimitsUpdate
	Config     *AlwaysThingConfig
	SourceFile *string
	SourceUrl  *string
}

type AlwaysThingsServiceAPIService struct{}

type ApiAlwaysThingsServiceGetAlwaysThingRequest struct{}
type ApiAlwaysThingsServiceUpdateAlwaysThingRequest struct{}

func (*AlwaysThingsServiceAPIService) AlwaysThingsServiceGetAlwaysThing(ctx context.Context) ApiAlwaysThingsServiceGetAlwaysThingRequest {
	_ = ctx
	return ApiAlwaysThingsServiceGetAlwaysThingRequest{}
}

func (*AlwaysThingsServiceAPIService) AlwaysThingsServiceUpdateAlwaysThing(ctx context.Context) ApiAlwaysThingsServiceUpdateAlwaysThingRequest {
	_ = ctx
	return ApiAlwaysThingsServiceUpdateAlwaysThingRequest{}
}

func (r ApiAlwaysThingsServiceUpdateAlwaysThingRequest) UpdateMask(updateMask string) ApiAlwaysThingsServiceUpdateAlwaysThingRequest {
	_ = updateMask
	return r
}

func (r ApiAlwaysThingsServiceUpdateAlwaysThingRequest) AlwaysThingsServiceUpdateAlwaysThingRequest(alwaysThingsServiceUpdateAlwaysThingRequest AlwaysThingsServiceUpdateAlwaysThingRequest) ApiAlwaysThingsServiceUpdateAlwaysThingRequest {
	_ = alwaysThingsServiceUpdateAlwaysThingRequest
	return r
}

func (ApiAlwaysThingsServiceGetAlwaysThingRequest) Execute() (*AlwaysThing, *http.Response, error) {
	return nil, nil, nil
}

func (ApiAlwaysThingsServiceUpdateAlwaysThingRequest) Execute() (*AlwaysThing, *http.Response, error) {
	return nil, nil, nil
}
