package things_service

import (
	"context"
	"net/http"
	"time"
)

type ThingKind string

const (
	THINGKIND_THING_KIND_UNSPECIFIED       ThingKind = "THING_KIND_UNSPECIFIED"
	THINGKIND_THING_KIND_STANDARD          ThingKind = "THING_KIND_STANDARD"
	THINGKIND_THING_KIND_ADVANCED          ThingKind = "THING_KIND_ADVANCED"
	THINGKIND_THING_KIND_P5_OR_UNSPECIFIED ThingKind = "THING_KIND_P5_OR_UNSPECIFIED"
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

type ThingStatus struct {
	Health string
}

type ThingSpec struct {
	Mode     *string
	Region   *string
	Revision string
	Source   *ThingSource
	Items    []ThingItem
	Targets  []ThingTarget
}

type ThingSpecCreate struct {
	Mode    *string
	Region  *string
	Items   []ThingItemCreate
	Targets []ThingTargetInput
}

type ThingSpecUpdate struct {
	Mode  *string
	Items []ThingItemUpdate
}

type ThingItem struct {
	Id   string
	Key  *string
	Name string
}

type ThingItemCreate struct {
	Key  *string
	Name string
}

type ThingItemUpdate struct {
	Name string
}

type ThingTarget struct {
	Id   string
	Name string
}

type ThingTargetInput struct {
	Name string
}

type ThingSource struct {
	Origin *string
}

type ThingDetail struct {
	Name string
}

type Thing struct {
	Id           *string
	Name         *string
	Description  *string
	Enabled      *bool
	Kind         *ThingKind
	Config       *ThingConfig
	Status       *ThingStatus
	Spec         *ThingSpec
	Destinations []string
	Tags         []string
	Details      []ThingDetail
	Labels       map[string]string
	CreateTime   *time.Time
	UpdateTime   *time.Time
	ExpireTime   *time.Time
	Windows      []time.Time
}

type CreateThingRequest struct {
	Name         string
	Description  *string
	Enabled      *bool
	Kind         ThingKind
	Config       ThingConfig
	Spec         *ThingSpecCreate
	Destinations []string
	Tags         []string
	Details      []ThingDetail
	Labels       map[string]string
	ExpireTime   *time.Time
	Windows      []time.Time
}

type UpdateThingRequest struct {
	Name         *string
	Description  *string
	Enabled      *bool
	Config       *ThingConfig
	Spec         *ThingSpecUpdate
	Destinations []string
	Tags         []string
	Details      []ThingDetail
	Labels       map[string]string
	ExpireTime   *time.Time
	Windows      []time.Time
}

type ThingsServiceAPIService struct {
	CreateCalls int
}

type ApiThingsServiceCreateThingRequest struct {
	client *ThingsServiceAPIService
}
type ApiThingsServiceGetThingRequest struct{}
type ApiThingsServiceUpdateThingRequest struct{}
type ApiThingsServiceDeleteThingRequest struct{}

func (s *ThingsServiceAPIService) ThingsServiceCreateThing(ctx context.Context) ApiThingsServiceCreateThingRequest {
	_ = ctx
	return ApiThingsServiceCreateThingRequest{client: s}
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

func (r ApiThingsServiceCreateThingRequest) CreateThingRequest(createThingRequest CreateThingRequest) ApiThingsServiceCreateThingRequest {
	_ = createThingRequest
	return r
}

func (ApiThingsServiceUpdateThingRequest) UpdateThingRequest(updateThingRequest UpdateThingRequest) ApiThingsServiceUpdateThingRequest {
	_ = updateThingRequest
	return ApiThingsServiceUpdateThingRequest{}
}

func (ApiThingsServiceUpdateThingRequest) UpdateMask(updateMask string) ApiThingsServiceUpdateThingRequest {
	_ = updateMask
	return ApiThingsServiceUpdateThingRequest{}
}

func (r ApiThingsServiceCreateThingRequest) Execute() (*Thing, *http.Response, error) {
	if r.client != nil {
		r.client.CreateCalls++
	}
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
