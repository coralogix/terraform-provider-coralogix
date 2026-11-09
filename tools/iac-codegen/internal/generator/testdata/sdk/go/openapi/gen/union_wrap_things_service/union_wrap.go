package union_wrap_things_service

import (
	"context"
	"net/http"
)

type ArmA struct {
	Value string
}

type ArmB struct {
	Count *int32
}

type ArmC struct {
	Label *string
}

// Thing holds the common fields and the arms of the oneOf. The requests and the response hold it
// in a pointer, because the contract does not require it.
type Thing struct {
	Name        string
	Description *string
	ArmA        *ArmA
	ArmB        *ArmB
	ArmC        *ArmC
}

type UnionWrapThing struct {
	Id    *string
	Thing *Thing
}

type CreateUnionWrapThingRequest struct {
	Thing *Thing
}

type ReplaceUnionWrapThingRequest struct {
	Thing *Thing
	Id    *string
}

type CreateUnionWrapThingResponse struct {
	UnionWrapThing *UnionWrapThing
}

type GetUnionWrapThingResponse struct {
	UnionWrapThing *UnionWrapThing
}

type ReplaceUnionWrapThingResponse struct {
	UnionWrapThing *UnionWrapThing
}

type UnionWrapThingsServiceAPIService struct{}

type ApiUnionWrapThingsServiceCreateUnionWrapThingRequest struct {
	body CreateUnionWrapThingRequest
}
type ApiUnionWrapThingsServiceGetUnionWrapThingRequest struct{ id string }
type ApiUnionWrapThingsServiceReplaceUnionWrapThingRequest struct {
	body ReplaceUnionWrapThingRequest
}
type ApiUnionWrapThingsServiceDeleteUnionWrapThingRequest struct{}

// Created and Replaced list the bodies that the calls received. Create returns CreatedID, and Get
// returns Stored with that id; a Stored without a Thing is a response without the object.
var (
	Created   []CreateUnionWrapThingRequest
	Replaced  []ReplaceUnionWrapThingRequest
	CreatedID = "thing-1"
	Stored    UnionWrapThing
)

func (*UnionWrapThingsServiceAPIService) UnionWrapThingsServiceCreateUnionWrapThing(ctx context.Context) ApiUnionWrapThingsServiceCreateUnionWrapThingRequest {
	_ = ctx
	return ApiUnionWrapThingsServiceCreateUnionWrapThingRequest{}
}

func (*UnionWrapThingsServiceAPIService) UnionWrapThingsServiceGetUnionWrapThing(ctx context.Context, id string) ApiUnionWrapThingsServiceGetUnionWrapThingRequest {
	_ = ctx
	return ApiUnionWrapThingsServiceGetUnionWrapThingRequest{id: id}
}

func (*UnionWrapThingsServiceAPIService) UnionWrapThingsServiceReplaceUnionWrapThing(ctx context.Context) ApiUnionWrapThingsServiceReplaceUnionWrapThingRequest {
	_ = ctx
	return ApiUnionWrapThingsServiceReplaceUnionWrapThingRequest{}
}

func (*UnionWrapThingsServiceAPIService) UnionWrapThingsServiceDeleteUnionWrapThing(ctx context.Context, id string) ApiUnionWrapThingsServiceDeleteUnionWrapThingRequest {
	_, _ = ctx, id
	return ApiUnionWrapThingsServiceDeleteUnionWrapThingRequest{}
}

func (r ApiUnionWrapThingsServiceCreateUnionWrapThingRequest) CreateUnionWrapThingRequest(createUnionWrapThingRequest CreateUnionWrapThingRequest) ApiUnionWrapThingsServiceCreateUnionWrapThingRequest {
	r.body = createUnionWrapThingRequest
	return r
}

func (r ApiUnionWrapThingsServiceReplaceUnionWrapThingRequest) ReplaceUnionWrapThingRequest(replaceUnionWrapThingRequest ReplaceUnionWrapThingRequest) ApiUnionWrapThingsServiceReplaceUnionWrapThingRequest {
	r.body = replaceUnionWrapThingRequest
	return r
}

func (r ApiUnionWrapThingsServiceCreateUnionWrapThingRequest) Execute() (*CreateUnionWrapThingResponse, *http.Response, error) {
	Created = append(Created, r.body)
	id := CreatedID
	return &CreateUnionWrapThingResponse{UnionWrapThing: &UnionWrapThing{Id: &id, Thing: r.body.Thing}}, nil, nil
}

func (r ApiUnionWrapThingsServiceGetUnionWrapThingRequest) Execute() (*GetUnionWrapThingResponse, *http.Response, error) {
	stored := Stored
	stored.Id = &r.id
	return &GetUnionWrapThingResponse{UnionWrapThing: &stored}, nil, nil
}

func (r ApiUnionWrapThingsServiceReplaceUnionWrapThingRequest) Execute() (*ReplaceUnionWrapThingResponse, *http.Response, error) {
	Replaced = append(Replaced, r.body)
	return &ReplaceUnionWrapThingResponse{UnionWrapThing: &UnionWrapThing{Id: r.body.Id, Thing: r.body.Thing}}, nil, nil
}

func (ApiUnionWrapThingsServiceDeleteUnionWrapThingRequest) Execute() (map[string]interface{}, *http.Response, error) {
	return nil, nil, nil
}
