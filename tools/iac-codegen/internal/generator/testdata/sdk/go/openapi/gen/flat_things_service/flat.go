package flat_things_service

import (
	"context"
	"net/http"
)

type FlatThingSample struct {
	Text   *string
	Weight *string
}

// The settings are inside Config in the response. The Create and Update bodies hold them flat.
type FlatThingConfig struct {
	Allowed      *string
	Denied       *string
	Instructions *string
	Kind         *string
	Samples      []FlatThingSample
	Verbose      *bool
}

type FlatThing struct {
	Config      *FlatThingConfig
	Description *string
	Id          *string
	LinkIds     []string
	Name        *string
}

type FlatThingsServiceCreateFlatThingRequest struct {
	Allowed      *string
	Denied       *string
	Description  *string
	Instructions *string
	Kind         *string
	LinkIds      []string
	Name         *string
	Samples      []FlatThingSample
	Verbose      *bool
}

type FlatThingsServiceUpdateFlatThingRequest struct {
	Allowed      *string
	Denied       *string
	Description  *string
	Instructions *string
	Kind         *string
	Name         *string
	Samples      []FlatThingSample
	UpdateMask   *string
	Verbose      *bool
}

// Create and Update wrap the thing in Item. Get returns it directly.
type CreateFlatThingResponse struct{ Item *FlatThing }
type UpdateFlatThingResponse struct{ Item *FlatThing }
type DeleteFlatThingResponse struct{ Id *string }

type FlatThingsServiceAPIService struct{}

type ApiFlatThingsServiceCreateFlatThingRequest struct {
	body FlatThingsServiceCreateFlatThingRequest
}
type ApiFlatThingsServiceGetFlatThingRequest struct{ id string }
type ApiFlatThingsServiceUpdateFlatThingRequest struct {
	id   string
	body FlatThingsServiceUpdateFlatThingRequest
}
type ApiFlatThingsServiceDeleteFlatThingRequest struct{}

// Created and Updated list the bodies that the calls received, and UpdatedIDs the ids. Create
// returns CreatedID. Get and Update return Stored.
var (
	Created    []FlatThingsServiceCreateFlatThingRequest
	Updated    []FlatThingsServiceUpdateFlatThingRequest
	UpdatedIDs []string
	CreatedID  = "flat-1"
	Stored     FlatThing
)

func (*FlatThingsServiceAPIService) FlatThingsServiceCreateFlatThing(ctx context.Context) ApiFlatThingsServiceCreateFlatThingRequest {
	_ = ctx
	return ApiFlatThingsServiceCreateFlatThingRequest{}
}

func (*FlatThingsServiceAPIService) FlatThingsServiceGetFlatThing(ctx context.Context, id string) ApiFlatThingsServiceGetFlatThingRequest {
	_ = ctx
	return ApiFlatThingsServiceGetFlatThingRequest{id: id}
}

func (*FlatThingsServiceAPIService) FlatThingsServiceUpdateFlatThing(ctx context.Context, id string) ApiFlatThingsServiceUpdateFlatThingRequest {
	_ = ctx
	return ApiFlatThingsServiceUpdateFlatThingRequest{id: id}
}

func (*FlatThingsServiceAPIService) FlatThingsServiceDeleteFlatThing(ctx context.Context, id string) ApiFlatThingsServiceDeleteFlatThingRequest {
	_, _ = ctx, id
	return ApiFlatThingsServiceDeleteFlatThingRequest{}
}

func (r ApiFlatThingsServiceCreateFlatThingRequest) FlatThingsServiceCreateFlatThingRequest(flatThingsServiceCreateFlatThingRequest FlatThingsServiceCreateFlatThingRequest) ApiFlatThingsServiceCreateFlatThingRequest {
	r.body = flatThingsServiceCreateFlatThingRequest
	return r
}

func (r ApiFlatThingsServiceUpdateFlatThingRequest) FlatThingsServiceUpdateFlatThingRequest(flatThingsServiceUpdateFlatThingRequest FlatThingsServiceUpdateFlatThingRequest) ApiFlatThingsServiceUpdateFlatThingRequest {
	r.body = flatThingsServiceUpdateFlatThingRequest
	return r
}

func (r ApiFlatThingsServiceCreateFlatThingRequest) Execute() (*CreateFlatThingResponse, *http.Response, error) {
	Created = append(Created, r.body)
	stored := Stored
	id := CreatedID
	stored.Id = &id
	return &CreateFlatThingResponse{Item: &stored}, nil, nil
}

func (r ApiFlatThingsServiceGetFlatThingRequest) Execute() (*FlatThing, *http.Response, error) {
	stored := Stored
	return &stored, nil, nil
}

func (r ApiFlatThingsServiceUpdateFlatThingRequest) Execute() (*UpdateFlatThingResponse, *http.Response, error) {
	Updated = append(Updated, r.body)
	UpdatedIDs = append(UpdatedIDs, r.id)
	stored := Stored
	return &UpdateFlatThingResponse{Item: &stored}, nil, nil
}

func (ApiFlatThingsServiceDeleteFlatThingRequest) Execute() (*DeleteFlatThingResponse, *http.Response, error) {
	return &DeleteFlatThingResponse{}, nil, nil
}
