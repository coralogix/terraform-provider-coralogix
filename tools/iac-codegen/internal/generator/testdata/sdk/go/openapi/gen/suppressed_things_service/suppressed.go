package suppressed_things_service

import (
	"context"
	"net/http"
)

type SuppressedThing struct {
	UniqueIdentifier  *string
	Id                *string
	Name              string
	Window            *Window
	ScheduleOperation ThingOperation
}

type ThingOperation string

const (
	THINGOPERATION_THING_OPERATION_UNSPECIFIED ThingOperation = "THING_OPERATION_UNSPECIFIED"
	THINGOPERATION_THING_OPERATION_MUTE        ThingOperation = "THING_OPERATION_MUTE"
	THINGOPERATION_THING_OPERATION_ACTIVATE    ThingOperation = "THING_OPERATION_ACTIVATE"
)

type Window struct {
	Timezone   string
	DaysOfWeek []int32
}

type CreateSuppressedThingRequest struct {
	SuppressedThing *SuppressedThing
}

type UpdateSuppressedThingRequest struct {
	SuppressedThing *SuppressedThing
}

type CreateSuppressedThingResponse struct {
	SuppressedThing *SuppressedThing
}

type GetSuppressedThingResponse struct {
	SuppressedThing *SuppressedThing
}

type UpdateSuppressedThingResponse struct {
	SuppressedThing *SuppressedThing
}

// Sent holds the rule of the last Create or Update body, Got the id of the last Get, and Stored
// is what the next call returns. Tests of the generated resource set and read them.
var (
	Sent   *SuppressedThing
	Got    string
	Stored *SuppressedThing
)

type SuppressedThingsServiceAPIService struct{}

type ApiSuppressedThingsServiceCreateSuppressedThingRequest struct {
	body CreateSuppressedThingRequest
}
type ApiSuppressedThingsServiceGetSuppressedThingRequest struct{ id string }
type ApiSuppressedThingsServiceUpdateSuppressedThingRequest struct {
	body UpdateSuppressedThingRequest
}
type ApiSuppressedThingsServiceDeleteSuppressedThingRequest struct{}

func (*SuppressedThingsServiceAPIService) SuppressedThingsServiceCreateSuppressedThing(ctx context.Context) ApiSuppressedThingsServiceCreateSuppressedThingRequest {
	_ = ctx
	return ApiSuppressedThingsServiceCreateSuppressedThingRequest{}
}

func (*SuppressedThingsServiceAPIService) SuppressedThingsServiceGetSuppressedThing(ctx context.Context, thingId string) ApiSuppressedThingsServiceGetSuppressedThingRequest {
	_ = ctx
	return ApiSuppressedThingsServiceGetSuppressedThingRequest{id: thingId}
}

func (*SuppressedThingsServiceAPIService) SuppressedThingsServiceUpdateSuppressedThing(ctx context.Context) ApiSuppressedThingsServiceUpdateSuppressedThingRequest {
	_ = ctx
	return ApiSuppressedThingsServiceUpdateSuppressedThingRequest{}
}

func (*SuppressedThingsServiceAPIService) SuppressedThingsServiceDeleteSuppressedThing(ctx context.Context, thingId string) ApiSuppressedThingsServiceDeleteSuppressedThingRequest {
	_, _ = ctx, thingId
	return ApiSuppressedThingsServiceDeleteSuppressedThingRequest{}
}

func (r ApiSuppressedThingsServiceCreateSuppressedThingRequest) CreateSuppressedThingRequest(createSuppressedThingRequest CreateSuppressedThingRequest) ApiSuppressedThingsServiceCreateSuppressedThingRequest {
	r.body = createSuppressedThingRequest
	return r
}

func (r ApiSuppressedThingsServiceUpdateSuppressedThingRequest) UpdateSuppressedThingRequest(updateSuppressedThingRequest UpdateSuppressedThingRequest) ApiSuppressedThingsServiceUpdateSuppressedThingRequest {
	r.body = updateSuppressedThingRequest
	return r
}

func (r ApiSuppressedThingsServiceCreateSuppressedThingRequest) Execute() (*CreateSuppressedThingResponse, *http.Response, error) {
	Sent = r.body.SuppressedThing
	return &CreateSuppressedThingResponse{SuppressedThing: Stored}, nil, nil
}

func (r ApiSuppressedThingsServiceGetSuppressedThingRequest) Execute() (*GetSuppressedThingResponse, *http.Response, error) {
	Got = r.id
	return &GetSuppressedThingResponse{SuppressedThing: Stored}, nil, nil
}

func (r ApiSuppressedThingsServiceUpdateSuppressedThingRequest) Execute() (*UpdateSuppressedThingResponse, *http.Response, error) {
	Sent = r.body.SuppressedThing
	return &UpdateSuppressedThingResponse{SuppressedThing: Stored}, nil, nil
}

func (ApiSuppressedThingsServiceDeleteSuppressedThingRequest) Execute() (map[string]interface{}, *http.Response, error) {
	return nil, nil, nil
}
