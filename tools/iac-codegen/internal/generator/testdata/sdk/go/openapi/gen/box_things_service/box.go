package box_things_service

import (
	"context"
	"net/http"
)

type BoxThing struct {
	Description *string
	Id          string
	Name        string
}

type BoxThingsServiceCreateBoxThingRequest struct {
	Description *string
	Name        *string
}

type BoxThingsServiceUpdateBoxThingRequest struct {
	Description *string
	Name        *string
	UpdateMask  *string
}

type CreateBoxThingResponse struct{ Item *BoxThing }
type UpdateBoxThingResponse struct{ Item *BoxThing }
type ListBoxThingsResponse struct{ Items []BoxThing }
type DeleteBoxThingResponse struct{ Id *string }

type BoxThingsServiceAPIService struct{}

type ApiBoxThingsServiceCreateBoxThingRequest struct {
	body BoxThingsServiceCreateBoxThingRequest
}
type ApiBoxThingsServiceListBoxThingsRequest struct{}
type ApiBoxThingsServiceUpdateBoxThingRequest struct {
	id   string
	body BoxThingsServiceUpdateBoxThingRequest
}
type ApiBoxThingsServiceDeleteBoxThingRequest struct{}

// Listed counts the calls of the List, and ListErr is the error that it returns. Stored is what the List returns, and Created and Updated
// list the bodies that Create and Update received, UpdatedIDs the ids of Update.
var (
	Listed     int
	ListErr    error
	Stored     []BoxThing
	Created    []BoxThingsServiceCreateBoxThingRequest
	Updated    []BoxThingsServiceUpdateBoxThingRequest
	UpdatedIDs []string
)

func (*BoxThingsServiceAPIService) BoxThingsServiceCreateBoxThing(ctx context.Context) ApiBoxThingsServiceCreateBoxThingRequest {
	_ = ctx
	return ApiBoxThingsServiceCreateBoxThingRequest{}
}

func (*BoxThingsServiceAPIService) BoxThingsServiceListBoxThings(ctx context.Context) ApiBoxThingsServiceListBoxThingsRequest {
	_ = ctx
	return ApiBoxThingsServiceListBoxThingsRequest{}
}

func (*BoxThingsServiceAPIService) BoxThingsServiceUpdateBoxThing(ctx context.Context, id string) ApiBoxThingsServiceUpdateBoxThingRequest {
	_ = ctx
	return ApiBoxThingsServiceUpdateBoxThingRequest{id: id}
}

func (*BoxThingsServiceAPIService) BoxThingsServiceDeleteBoxThing(ctx context.Context, id string) ApiBoxThingsServiceDeleteBoxThingRequest {
	_, _ = ctx, id
	return ApiBoxThingsServiceDeleteBoxThingRequest{}
}

func (r ApiBoxThingsServiceCreateBoxThingRequest) BoxThingsServiceCreateBoxThingRequest(boxThingsServiceCreateBoxThingRequest BoxThingsServiceCreateBoxThingRequest) ApiBoxThingsServiceCreateBoxThingRequest {
	r.body = boxThingsServiceCreateBoxThingRequest
	return r
}

func (r ApiBoxThingsServiceUpdateBoxThingRequest) BoxThingsServiceUpdateBoxThingRequest(boxThingsServiceUpdateBoxThingRequest BoxThingsServiceUpdateBoxThingRequest) ApiBoxThingsServiceUpdateBoxThingRequest {
	r.body = boxThingsServiceUpdateBoxThingRequest
	return r
}

func (r ApiBoxThingsServiceCreateBoxThingRequest) Execute() (*CreateBoxThingResponse, *http.Response, error) {
	Created = append(Created, r.body)
	created := BoxThing{Id: "box-1"}
	if r.body.Name != nil {
		created.Name = *r.body.Name
	}
	return &CreateBoxThingResponse{Item: &created}, nil, nil
}

func (ApiBoxThingsServiceListBoxThingsRequest) Execute() (*ListBoxThingsResponse, *http.Response, error) {
	Listed++
	if ListErr != nil {
		return nil, nil, ListErr
	}
	return &ListBoxThingsResponse{Items: append([]BoxThing(nil), Stored...)}, nil, nil
}

func (r ApiBoxThingsServiceUpdateBoxThingRequest) Execute() (*UpdateBoxThingResponse, *http.Response, error) {
	Updated = append(Updated, r.body)
	UpdatedIDs = append(UpdatedIDs, r.id)
	updated := BoxThing{Id: r.id}
	if r.body.Name != nil {
		updated.Name = *r.body.Name
	}
	return &UpdateBoxThingResponse{Item: &updated}, nil, nil
}

func (ApiBoxThingsServiceDeleteBoxThingRequest) Execute() (*DeleteBoxThingResponse, *http.Response, error) {
	return &DeleteBoxThingResponse{}, nil, nil
}
