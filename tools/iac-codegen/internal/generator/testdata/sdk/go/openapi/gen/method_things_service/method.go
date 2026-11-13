package method_things_service

import (
	"context"
	"net/http"
)

// The Create request has a oneOf of the parent: the id or the name.
type CreateMethodThingRequest struct {
	Name        *string
	Description *string
	ParentId    *int64
	ParentName  *string
	Permissions []string
}

// Create returns only the id, as an integer.
type CreateMethodThingResponse struct {
	Id *int64
}

// A required field is a value, as openapi-generator writes it.
type MethodThing struct {
	ThingId     int64
	Name        string
	Description string
	ParentId    int64
	ParentName  string
	Permissions []string
}

// Get wraps the thing.
type GetMethodThingResponse struct {
	Thing *MethodThing
}

// The Update request holds the permissions in a one-property object.
type MethodThingPermissions struct {
	Permissions []string
}

type UpdateMethodThingRequest struct {
	NewName        *string
	NewDescription *string
	NewPermissions *MethodThingPermissions
}

type MethodThingsServiceAPIService struct{}

type ApiMethodThingsServiceCreateMethodThingRequest struct {
	body CreateMethodThingRequest
}
type ApiMethodThingsServiceGetMethodThingRequest struct {
	id int64
}
type ApiMethodThingsServiceUpdateMethodThingRequest struct {
	id   int64
	body UpdateMethodThingRequest
}
type ApiMethodThingsServiceDeleteMethodThingRequest struct{}

// Created and Updated list the bodies that the calls received. Create returns CreatedID, and Get
// returns Stored.
var (
	Created   []CreateMethodThingRequest
	Updated   []UpdateMethodThingRequest
	UpdatedID []int64
	CreatedID = int64(41)
	Stored    MethodThing
)

func (*MethodThingsServiceAPIService) MethodThingsServiceCreateMethodThing(ctx context.Context) ApiMethodThingsServiceCreateMethodThingRequest {
	_ = ctx
	return ApiMethodThingsServiceCreateMethodThingRequest{}
}

func (*MethodThingsServiceAPIService) MethodThingsServiceGetMethodThing(ctx context.Context, thingId int64) ApiMethodThingsServiceGetMethodThingRequest {
	_ = ctx
	return ApiMethodThingsServiceGetMethodThingRequest{id: thingId}
}

func (*MethodThingsServiceAPIService) MethodThingsServiceUpdateMethodThing(ctx context.Context, thingId int64) ApiMethodThingsServiceUpdateMethodThingRequest {
	_ = ctx
	return ApiMethodThingsServiceUpdateMethodThingRequest{id: thingId}
}

func (*MethodThingsServiceAPIService) MethodThingsServiceDeleteMethodThing(ctx context.Context, thingId int64) ApiMethodThingsServiceDeleteMethodThingRequest {
	_, _ = ctx, thingId
	return ApiMethodThingsServiceDeleteMethodThingRequest{}
}

func (r ApiMethodThingsServiceCreateMethodThingRequest) CreateMethodThingRequest(createMethodThingRequest CreateMethodThingRequest) ApiMethodThingsServiceCreateMethodThingRequest {
	r.body = createMethodThingRequest
	return r
}

func (r ApiMethodThingsServiceUpdateMethodThingRequest) UpdateMethodThingRequest(updateMethodThingRequest UpdateMethodThingRequest) ApiMethodThingsServiceUpdateMethodThingRequest {
	r.body = updateMethodThingRequest
	return r
}

func (r ApiMethodThingsServiceCreateMethodThingRequest) Execute() (*CreateMethodThingResponse, *http.Response, error) {
	Created = append(Created, r.body)
	id := CreatedID
	return &CreateMethodThingResponse{Id: &id}, nil, nil
}

func (r ApiMethodThingsServiceGetMethodThingRequest) Execute() (*GetMethodThingResponse, *http.Response, error) {
	stored := Stored
	return &GetMethodThingResponse{Thing: &stored}, nil, nil
}

// Update returns an empty object, as the SDK writes for a response with no fields.
func (r ApiMethodThingsServiceUpdateMethodThingRequest) Execute() (map[string]interface{}, *http.Response, error) {
	Updated = append(Updated, r.body)
	UpdatedID = append(UpdatedID, r.id)
	return map[string]interface{}{}, nil, nil
}

func (ApiMethodThingsServiceDeleteMethodThingRequest) Execute() (map[string]interface{}, *http.Response, error) {
	return nil, nil, nil
}
