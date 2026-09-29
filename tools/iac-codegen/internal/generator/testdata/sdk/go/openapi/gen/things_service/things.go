package things_service

import (
	"context"
	"net/http"
)

type Thing struct {
	Id        *string
	Name      *string
	Enabled   *bool
	Count     *int64
	Ordered   []string
	Unordered []string
	Labels    map[string]string
}

type CreateThingRequest struct {
	Name      string
	Enabled   *bool
	Count     *int64
	Ordered   []string
	Unordered []string
	Labels    map[string]string
}

type UpdateThingRequest struct {
	Name      *string
	Enabled   *bool
	Count     *int64
	Ordered   []string
	Unordered []string
	Labels    map[string]string
}

type ThingsServiceAPIService struct{}

type ApiThingsServiceCreateThingRequest struct{}
type ApiThingsServiceGetThingRequest struct{}
type ApiThingsServiceUpdateThingRequest struct{}
type ApiThingsServiceDeleteThingRequest struct{}

func (*ThingsServiceAPIService) ThingsServiceCreateThing(ctx context.Context) ApiThingsServiceCreateThingRequest {
	_ = ctx
	return ApiThingsServiceCreateThingRequest{}
}

func (*ThingsServiceAPIService) ThingsServiceGetThing(ctx context.Context, id string) ApiThingsServiceGetThingRequest {
	_, _ = ctx, id
	return ApiThingsServiceGetThingRequest{}
}

func (*ThingsServiceAPIService) ThingsServiceUpdateThing(ctx context.Context, id string) ApiThingsServiceUpdateThingRequest {
	_, _ = ctx, id
	return ApiThingsServiceUpdateThingRequest{}
}

func (*ThingsServiceAPIService) ThingsServiceDeleteThing(ctx context.Context, id string) ApiThingsServiceDeleteThingRequest {
	_, _ = ctx, id
	return ApiThingsServiceDeleteThingRequest{}
}

func (ApiThingsServiceCreateThingRequest) CreateThingRequest(createThingRequest CreateThingRequest) ApiThingsServiceCreateThingRequest {
	_ = createThingRequest
	return ApiThingsServiceCreateThingRequest{}
}

func (ApiThingsServiceUpdateThingRequest) UpdateThingRequest(updateThingRequest UpdateThingRequest) ApiThingsServiceUpdateThingRequest {
	_ = updateThingRequest
	return ApiThingsServiceUpdateThingRequest{}
}

func (ApiThingsServiceUpdateThingRequest) UpdateMask(updateMask string) ApiThingsServiceUpdateThingRequest {
	_ = updateMask
	return ApiThingsServiceUpdateThingRequest{}
}

func (ApiThingsServiceCreateThingRequest) Execute() (*Thing, *http.Response, error) {
	return nil, nil, nil
}

func (ApiThingsServiceGetThingRequest) Execute() (*Thing, *http.Response, error) {
	return nil, nil, nil
}

func (ApiThingsServiceUpdateThingRequest) Execute() (*Thing, *http.Response, error) {
	return nil, nil, nil
}

func (ApiThingsServiceDeleteThingRequest) Execute() (map[string]interface{}, *http.Response, error) {
	return nil, nil, nil
}
