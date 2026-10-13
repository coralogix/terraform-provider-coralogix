package shared_things_service

import (
	"context"
	"net/http"
)

type SharedFrame string

const (
	SHAREDFRAME_SHARED_FRAME_UNSPECIFIED SharedFrame = "SHARED_FRAME_UNSPECIFIED"
	SHAREDFRAME_SHARED_FRAME_ONE_DAY     SharedFrame = "SHARED_FRAME_ONE_DAY"
	SHAREDFRAME_SHARED_FRAME_SEVEN_DAYS  SharedFrame = "SHARED_FRAME_SEVEN_DAYS"
)

type SharedCount struct {
	Limit *int32
}

type SharedRatio struct {
	Ratio *float32
}

type SharedThing struct {
	Id      *string
	Name    *string
	Note    *string
	Size    *float32
	Frame   *SharedFrame
	ByCount *SharedCount
	ByRatio *SharedRatio
	Compact *bool
}

// SharedThing1 is the inline Create and Update body. It has the title of the SharedThing component.
type SharedThing1 struct {
	Id      *string
	Name    *string
	Note    *string
	Size    *float32
	Frame   *SharedFrame
	ByCount *SharedCount
	ByRatio *SharedRatio
}

// SharedThingUpdate is the Update body of the variant whose Update has an Update-only field.
type SharedThingUpdate struct {
	Id      *string
	Name    *string
	Note    *string
	Compact *bool
	Size    *float32
	Frame   *SharedFrame
	ByCount *SharedCount
	ByRatio *SharedRatio
}

type CreateSharedThingResponse struct {
	ChangedIds []string
	Thing      SharedThing
}

type GetSharedThingResponse struct {
	Thing SharedThing
}

type ReplaceSharedThingResponse struct {
	ChangedIds []string
	Thing      SharedThing
}

type SharedThingsServiceAPIService struct{}

type ApiSharedThingsServiceCreateSharedThingRequest struct {
	body SharedThing1
}
type ApiSharedThingsServiceGetSharedThingRequest struct {
	id string
}
type ApiSharedThingsServiceReplaceSharedThingRequest struct {
	body       SharedThing1
	updateBody *SharedThingUpdate
}
type ApiSharedThingsServiceDeleteSharedThingRequest struct{}

// Created and Replaced list the bodies that the calls received. Create returns the thing with
// the id CreatedID, and replace returns the thing it received. Get returns Stored.
var (
	Created   []SharedThing1
	Replaced  []SharedThing1
	CreatedID = "thing-1"
	Stored    SharedThing
)

func (*SharedThingsServiceAPIService) SharedThingsServiceCreateSharedThing(ctx context.Context) ApiSharedThingsServiceCreateSharedThingRequest {
	_ = ctx
	return ApiSharedThingsServiceCreateSharedThingRequest{}
}

func (*SharedThingsServiceAPIService) SharedThingsServiceGetSharedThing(ctx context.Context, id string) ApiSharedThingsServiceGetSharedThingRequest {
	_ = ctx
	return ApiSharedThingsServiceGetSharedThingRequest{id: id}
}

func (*SharedThingsServiceAPIService) SharedThingsServiceReplaceSharedThing(ctx context.Context) ApiSharedThingsServiceReplaceSharedThingRequest {
	_ = ctx
	return ApiSharedThingsServiceReplaceSharedThingRequest{}
}

func (*SharedThingsServiceAPIService) SharedThingsServiceDeleteSharedThing(ctx context.Context, id string) ApiSharedThingsServiceDeleteSharedThingRequest {
	_, _ = ctx, id
	return ApiSharedThingsServiceDeleteSharedThingRequest{}
}

func (r ApiSharedThingsServiceCreateSharedThingRequest) SharedThing1(sharedThing1 SharedThing1) ApiSharedThingsServiceCreateSharedThingRequest {
	r.body = sharedThing1
	return r
}

func (r ApiSharedThingsServiceReplaceSharedThingRequest) SharedThing1(sharedThing1 SharedThing1) ApiSharedThingsServiceReplaceSharedThingRequest {
	r.body = sharedThing1
	return r
}

func (r ApiSharedThingsServiceReplaceSharedThingRequest) SharedThingUpdate(sharedThingUpdate SharedThingUpdate) ApiSharedThingsServiceReplaceSharedThingRequest {
	r.updateBody = &sharedThingUpdate
	return r
}

func (r ApiSharedThingsServiceCreateSharedThingRequest) Execute() (*CreateSharedThingResponse, *http.Response, error) {
	Created = append(Created, r.body)
	id := CreatedID
	return &CreateSharedThingResponse{ChangedIds: []string{"other"}, Thing: SharedThing{Id: &id, Name: r.body.Name, Note: r.body.Note, Size: r.body.Size, Frame: r.body.Frame, ByCount: r.body.ByCount, ByRatio: r.body.ByRatio}}, nil, nil
}

func (r ApiSharedThingsServiceGetSharedThingRequest) Execute() (*GetSharedThingResponse, *http.Response, error) {
	_ = r.id
	return &GetSharedThingResponse{Thing: Stored}, nil, nil
}

func (r ApiSharedThingsServiceReplaceSharedThingRequest) Execute() (*ReplaceSharedThingResponse, *http.Response, error) {
	Replaced = append(Replaced, r.body)
	return &ReplaceSharedThingResponse{Thing: SharedThing{Id: r.body.Id, Name: r.body.Name, Note: r.body.Note, Size: r.body.Size, Frame: r.body.Frame, ByCount: r.body.ByCount, ByRatio: r.body.ByRatio}}, nil, nil
}

func (ApiSharedThingsServiceDeleteSharedThingRequest) Execute() (map[string]interface{}, *http.Response, error) {
	return nil, nil, nil
}
