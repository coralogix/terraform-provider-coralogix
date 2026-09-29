package things_service

import (
	"context"
	"net/http"
	"time"
)

type ThingKind string

const (
	THINGKIND_THING_KIND_UNSPECIFIED ThingKind = "THING_KIND_UNSPECIFIED"
	THINGKIND_THING_KIND_STANDARD    ThingKind = "THING_KIND_STANDARD"
	THINGKIND_THING_KIND_ADVANCED    ThingKind = "THING_KIND_ADVANCED"
)

type ThingConfig struct {
	Http  *HttpThingConfig
	Queue *QueueThingConfig
}

type HttpThingConfig struct {
	Endpoint string
}

type QueueThingConfig struct {
	Topic string
}

type Thing struct {
	Id           *string
	Name         *string
	Description  *string
	Enabled      *bool
	Kind         *ThingKind
	Config       *ThingConfig
	Destinations []string
	Tags         []string
	Labels       map[string]string
	CreateTime   *time.Time
	UpdateTime   *time.Time
}

type CreateThingRequest struct {
	Name         string
	Description  *string
	Enabled      *bool
	Kind         ThingKind
	Config       ThingConfig
	Destinations []string
	Tags         []string
	Labels       map[string]string
}

type UpdateThingRequest struct {
	Name         *string
	Description  *string
	Enabled      *bool
	Config       *ThingConfig
	Destinations []string
	Tags         []string
	Labels       map[string]string
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
