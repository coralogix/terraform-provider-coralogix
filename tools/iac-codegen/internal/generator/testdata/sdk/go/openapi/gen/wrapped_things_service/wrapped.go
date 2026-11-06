package wrapped_things_service

import (
	"context"
	"net/http"
)

// ThingId holds the id of a thing, inside a one-property object.
type ThingId struct {
	Id *int64
}

type GetThingResponse struct {
	ThingId ThingId
	Name    string
	Quota   float64
}

type CreateThingBody struct {
	Name  string
	Quota *float64
}

type UpdateThingBody struct {
	Name  *string
	Quota *float64
}

type ThingSummary struct {
	ThingId *ThingId
	Name    *string
}

type ListThingsResponse struct {
	Things []ThingSummary
}

// The Create response holds only the wrapper of the id, as a pointer: it is not required there.
type CreateThingResponse struct {
	ThingId *ThingId
}

type WrappedThingsServiceAPIService struct{}

type ApiWrappedThingsServiceCreateThingRequest struct {
	body CreateThingBody
}
type ApiWrappedThingsServiceGetThingRequest struct {
	id int64
}
type ApiWrappedThingsServiceUpdateThingRequest struct {
	id   int64
	body UpdateThingBody
}
type ApiWrappedThingsServiceDeleteThingRequest struct{}
type ApiWrappedThingsServiceListThingsRequest struct{}

// Created and Updated list the bodies that the calls received, and UpdatedIDs the ids of the
// updates. Create returns CreatedID. Get returns Stored, and GetIDs lists the ids that it was
// asked for.
var (
	Created    []CreateThingBody
	Updated    []UpdateThingBody
	UpdatedIDs []int64
	CreatedID  int64 = 1002
	Stored     GetThingResponse
	GetIDs     []int64
	// GetErr makes Get fail.
	GetErr error
	// Listed is what List returns.
	Listed []ThingSummary
)

func (*WrappedThingsServiceAPIService) WrappedThingsServiceCreateThing(ctx context.Context) ApiWrappedThingsServiceCreateThingRequest {
	_ = ctx
	return ApiWrappedThingsServiceCreateThingRequest{}
}

func (*WrappedThingsServiceAPIService) WrappedThingsServiceGetThing(ctx context.Context, thingId int64) ApiWrappedThingsServiceGetThingRequest {
	_ = ctx
	return ApiWrappedThingsServiceGetThingRequest{id: thingId}
}

func (*WrappedThingsServiceAPIService) WrappedThingsServiceUpdateThing(ctx context.Context, thingId int64) ApiWrappedThingsServiceUpdateThingRequest {
	_ = ctx
	return ApiWrappedThingsServiceUpdateThingRequest{id: thingId}
}

func (*WrappedThingsServiceAPIService) WrappedThingsServiceDeleteThing(ctx context.Context, thingId int64) ApiWrappedThingsServiceDeleteThingRequest {
	_, _ = ctx, thingId
	return ApiWrappedThingsServiceDeleteThingRequest{}
}

func (r ApiWrappedThingsServiceCreateThingRequest) CreateThingBody(createThingBody CreateThingBody) ApiWrappedThingsServiceCreateThingRequest {
	r.body = createThingBody
	return r
}

func (r ApiWrappedThingsServiceUpdateThingRequest) UpdateThingBody(updateThingBody UpdateThingBody) ApiWrappedThingsServiceUpdateThingRequest {
	r.body = updateThingBody
	return r
}

func (r ApiWrappedThingsServiceCreateThingRequest) Execute() (*CreateThingResponse, *http.Response, error) {
	Created = append(Created, r.body)
	id := CreatedID
	return &CreateThingResponse{ThingId: &ThingId{Id: &id}}, nil, nil
}

func (r ApiWrappedThingsServiceGetThingRequest) Execute() (*GetThingResponse, *http.Response, error) {
	GetIDs = append(GetIDs, r.id)
	if GetErr != nil {
		return nil, nil, GetErr
	}
	stored := Stored
	return &stored, nil, nil
}

// Update returns an empty object, as the SDK writes for a response with no fields.
func (r ApiWrappedThingsServiceUpdateThingRequest) Execute() (map[string]interface{}, *http.Response, error) {
	Updated, UpdatedIDs = append(Updated, r.body), append(UpdatedIDs, r.id)
	return map[string]interface{}{}, nil, nil
}

func (ApiWrappedThingsServiceDeleteThingRequest) Execute() (map[string]interface{}, *http.Response, error) {
	return nil, nil, nil
}

func (*WrappedThingsServiceAPIService) WrappedThingsServiceListThings(ctx context.Context) ApiWrappedThingsServiceListThingsRequest {
	_ = ctx
	return ApiWrappedThingsServiceListThingsRequest{}
}

func (ApiWrappedThingsServiceListThingsRequest) Execute() (*ListThingsResponse, *http.Response, error) {
	return &ListThingsResponse{Things: Listed}, nil, nil
}
