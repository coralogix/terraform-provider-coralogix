package folder_things_service

import (
	"context"
	"net/http"
)

type FolderThing struct {
	Id       *string
	Name     *string
	ParentId *string
}

// The Create and Replace requests wrap the resource and have an idempotency key that the
// resource does not send.
type CreateFolderThingBody struct {
	RequestId *string
	Thing     *FolderThing
}

type ReplaceFolderThingBody struct {
	RequestId *string
	Thing     *FolderThing
}

// The Create response holds only the id.
type CreateFolderThingResponse struct {
	ThingId *string
}

type GetFolderThingResponse struct {
	Thing *FolderThing
}

type FolderThingsServiceAPIService struct{}

type ApiFolderThingsServiceCreateFolderThingRequest struct {
	body CreateFolderThingBody
}
type ApiFolderThingsServiceGetFolderThingRequest struct {
	id string
}
type ApiFolderThingsServiceReplaceFolderThingRequest struct {
	body ReplaceFolderThingBody
}
type ApiFolderThingsServiceDeleteFolderThingRequest struct{}

// Created and Replaced list the bodies that the calls received. Create returns CreatedID. Get
// returns Stored, and GetIDs lists the ids that it was asked for.
var (
	Created   []CreateFolderThingBody
	Replaced  []ReplaceFolderThingBody
	CreatedID = "folder-1"
	Stored    FolderThing
	GetIDs    []string
)

// The path parameter is thing_id in the contract, which openapi-generator writes as thingId.
func (*FolderThingsServiceAPIService) FolderThingsServiceCreateFolderThing(ctx context.Context) ApiFolderThingsServiceCreateFolderThingRequest {
	_ = ctx
	return ApiFolderThingsServiceCreateFolderThingRequest{}
}

func (*FolderThingsServiceAPIService) FolderThingsServiceGetFolderThing(ctx context.Context, thingId string) ApiFolderThingsServiceGetFolderThingRequest {
	_ = ctx
	return ApiFolderThingsServiceGetFolderThingRequest{id: thingId}
}

func (*FolderThingsServiceAPIService) FolderThingsServiceReplaceFolderThing(ctx context.Context) ApiFolderThingsServiceReplaceFolderThingRequest {
	_ = ctx
	return ApiFolderThingsServiceReplaceFolderThingRequest{}
}

func (*FolderThingsServiceAPIService) FolderThingsServiceDeleteFolderThing(ctx context.Context, thingId string) ApiFolderThingsServiceDeleteFolderThingRequest {
	_, _ = ctx, thingId
	return ApiFolderThingsServiceDeleteFolderThingRequest{}
}

func (r ApiFolderThingsServiceCreateFolderThingRequest) CreateFolderThingBody(createFolderThingBody CreateFolderThingBody) ApiFolderThingsServiceCreateFolderThingRequest {
	r.body = createFolderThingBody
	return r
}

func (r ApiFolderThingsServiceReplaceFolderThingRequest) ReplaceFolderThingBody(replaceFolderThingBody ReplaceFolderThingBody) ApiFolderThingsServiceReplaceFolderThingRequest {
	r.body = replaceFolderThingBody
	return r
}

func (r ApiFolderThingsServiceCreateFolderThingRequest) Execute() (*CreateFolderThingResponse, *http.Response, error) {
	Created = append(Created, r.body)
	id := CreatedID
	return &CreateFolderThingResponse{ThingId: &id}, nil, nil
}

func (r ApiFolderThingsServiceGetFolderThingRequest) Execute() (*GetFolderThingResponse, *http.Response, error) {
	GetIDs = append(GetIDs, r.id)
	stored := Stored
	return &GetFolderThingResponse{Thing: &stored}, nil, nil
}

// Replace returns an empty object, as the SDK writes for a response with no fields.
func (r ApiFolderThingsServiceReplaceFolderThingRequest) Execute() (map[string]interface{}, *http.Response, error) {
	Replaced = append(Replaced, r.body)
	return map[string]interface{}{}, nil, nil
}

func (ApiFolderThingsServiceDeleteFolderThingRequest) Execute() (map[string]interface{}, *http.Response, error) {
	return nil, nil, nil
}
