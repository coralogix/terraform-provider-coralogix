package action_things_service

import (
	"context"
	"net/http"
)

type ActionThing struct {
	Id     *string
	Name   *string
	Hidden *bool
	Note   *string
	Tags   []string
}

// The Create body is the resource without the field hidden, which the API refuses there.
type CreateActionThingBody struct {
	Name *string
	Note *string
	Tags []string
}

// The Replace body wraps the resource, with its id.
type ReplaceActionThingBody struct {
	Thing *ActionThing
}

type CreateActionThingResponse struct {
	Thing *ActionThing
}

type ReplaceActionThingResponse struct {
	Thing *ActionThing
}

type GetActionThingResponse struct {
	Thing *ActionThing
}

type ActionThingsServiceAPIService struct{}

type ApiActionThingsServiceCreateActionThingRequest struct {
	body CreateActionThingBody
}
type ApiActionThingsServiceGetActionThingRequest struct {
	id string
}
type ApiActionThingsServiceReplaceActionThingRequest struct {
	body ReplaceActionThingBody
}
type ApiActionThingsServiceDeleteActionThingRequest struct{}

// Created and Replaced list the bodies that the calls received. Create stores the thing without
// hidden, as the API does, and Replace stores the thing that it receives.
var (
	Created  []CreateActionThingBody
	Replaced []ReplaceActionThingBody
	Stored   ActionThing
	// GetErr makes Get fail, and DeleteErr makes Delete fail.
	GetErr, DeleteErr, ReplaceErr error
)

func (*ActionThingsServiceAPIService) ActionThingsServiceCreateActionThing(ctx context.Context) ApiActionThingsServiceCreateActionThingRequest {
	_ = ctx
	return ApiActionThingsServiceCreateActionThingRequest{}
}

func (*ActionThingsServiceAPIService) ActionThingsServiceGetActionThing(ctx context.Context, id string) ApiActionThingsServiceGetActionThingRequest {
	_ = ctx
	return ApiActionThingsServiceGetActionThingRequest{id: id}
}

func (*ActionThingsServiceAPIService) ActionThingsServiceReplaceActionThing(ctx context.Context) ApiActionThingsServiceReplaceActionThingRequest {
	_ = ctx
	return ApiActionThingsServiceReplaceActionThingRequest{}
}

func (*ActionThingsServiceAPIService) ActionThingsServiceDeleteActionThing(ctx context.Context, id string) ApiActionThingsServiceDeleteActionThingRequest {
	_, _ = ctx, id
	return ApiActionThingsServiceDeleteActionThingRequest{}
}

func (r ApiActionThingsServiceCreateActionThingRequest) CreateActionThingBody(createActionThingBody CreateActionThingBody) ApiActionThingsServiceCreateActionThingRequest {
	r.body = createActionThingBody
	return r
}

func (r ApiActionThingsServiceReplaceActionThingRequest) ReplaceActionThingBody(replaceActionThingBody ReplaceActionThingBody) ApiActionThingsServiceReplaceActionThingRequest {
	r.body = replaceActionThingBody
	return r
}

func (r ApiActionThingsServiceCreateActionThingRequest) Execute() (*CreateActionThingResponse, *http.Response, error) {
	Created = append(Created, r.body)
	id := "thing-1"
	hidden := false
	Stored = ActionThing{Id: &id, Name: r.body.Name, Hidden: &hidden, Note: r.body.Note, Tags: r.body.Tags}
	stored := Stored
	return &CreateActionThingResponse{Thing: &stored}, nil, nil
}

func (r ApiActionThingsServiceGetActionThingRequest) Execute() (*GetActionThingResponse, *http.Response, error) {
	if GetErr != nil {
		return nil, nil, GetErr
	}
	stored := Stored
	return &GetActionThingResponse{Thing: &stored}, nil, nil
}

func (r ApiActionThingsServiceReplaceActionThingRequest) Execute() (*ReplaceActionThingResponse, *http.Response, error) {
	Replaced = append(Replaced, r.body)
	if ReplaceErr != nil {
		return nil, nil, ReplaceErr
	}
	Stored = *r.body.Thing
	stored := Stored
	return &ReplaceActionThingResponse{Thing: &stored}, nil, nil
}

func (ApiActionThingsServiceDeleteActionThingRequest) Execute() (map[string]interface{}, *http.Response, error) {
	return nil, nil, DeleteErr
}
