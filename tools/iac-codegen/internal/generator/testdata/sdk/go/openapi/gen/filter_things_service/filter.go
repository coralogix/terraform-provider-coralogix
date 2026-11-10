package filter_things_service

import (
	"context"
	"net/http"
)

type FilterOperator struct {
	Value *string
}

type FilterLogs struct {
	Field    *string
	Operator *FilterOperator
}

type FilterMetrics struct {
	Metric *string
}

// A oneOf item: one arm is set. Everything is an arm with no fields.
type Filter struct {
	Logs       *FilterLogs
	Metrics    *FilterMetrics
	Label      *string
	Everything map[string]interface{}
}

type FilterThing struct {
	Id      *string
	Name    *string
	Filters []Filter
}

type CreateFilterThingBody struct {
	Name    *string
	Filters []Filter
}

type ReplaceFilterThingBody struct {
	Name    *string
	Filters []Filter
}

type CreateFilterThingResponse struct {
	Thing *FilterThing
}

type ReplaceFilterThingResponse struct {
	Thing *FilterThing
}

type GetFilterThingResponse struct {
	Thing *FilterThing
}

type FilterThingsServiceAPIService struct{}

type ApiFilterThingsServiceCreateFilterThingRequest struct {
	body CreateFilterThingBody
}
type ApiFilterThingsServiceGetFilterThingRequest struct {
	id string
}
type ApiFilterThingsServiceReplaceFilterThingRequest struct {
	id   string
	body ReplaceFilterThingBody
}
type ApiFilterThingsServiceDeleteFilterThingRequest struct{}

// Created and Replaced list the bodies that the calls received. Stored is the thing that the
// calls return and that Get reads.
var (
	Created  []CreateFilterThingBody
	Replaced []ReplaceFilterThingBody
	Stored   FilterThing
)

func (*FilterThingsServiceAPIService) FilterThingsServiceCreateFilterThing(ctx context.Context) ApiFilterThingsServiceCreateFilterThingRequest {
	_ = ctx
	return ApiFilterThingsServiceCreateFilterThingRequest{}
}

func (*FilterThingsServiceAPIService) FilterThingsServiceGetFilterThing(ctx context.Context, thingId string) ApiFilterThingsServiceGetFilterThingRequest {
	_ = ctx
	return ApiFilterThingsServiceGetFilterThingRequest{id: thingId}
}

func (*FilterThingsServiceAPIService) FilterThingsServiceReplaceFilterThing(ctx context.Context, thingId string) ApiFilterThingsServiceReplaceFilterThingRequest {
	_ = ctx
	return ApiFilterThingsServiceReplaceFilterThingRequest{id: thingId}
}

func (*FilterThingsServiceAPIService) FilterThingsServiceDeleteFilterThing(ctx context.Context, thingId string) ApiFilterThingsServiceDeleteFilterThingRequest {
	_, _ = ctx, thingId
	return ApiFilterThingsServiceDeleteFilterThingRequest{}
}

func (r ApiFilterThingsServiceCreateFilterThingRequest) CreateFilterThingBody(createFilterThingBody CreateFilterThingBody) ApiFilterThingsServiceCreateFilterThingRequest {
	r.body = createFilterThingBody
	return r
}

func (r ApiFilterThingsServiceReplaceFilterThingRequest) ReplaceFilterThingBody(replaceFilterThingBody ReplaceFilterThingBody) ApiFilterThingsServiceReplaceFilterThingRequest {
	r.body = replaceFilterThingBody
	return r
}

func store(name *string, filters []Filter) *FilterThing {
	id := "thing-1"
	Stored = FilterThing{Id: &id, Name: name, Filters: filters}
	stored := Stored
	return &stored
}

func (r ApiFilterThingsServiceCreateFilterThingRequest) Execute() (*CreateFilterThingResponse, *http.Response, error) {
	Created = append(Created, r.body)
	return &CreateFilterThingResponse{Thing: store(r.body.Name, r.body.Filters)}, nil, nil
}

func (r ApiFilterThingsServiceGetFilterThingRequest) Execute() (*GetFilterThingResponse, *http.Response, error) {
	stored := Stored
	return &GetFilterThingResponse{Thing: &stored}, nil, nil
}

func (r ApiFilterThingsServiceReplaceFilterThingRequest) Execute() (*ReplaceFilterThingResponse, *http.Response, error) {
	Replaced = append(Replaced, r.body)
	return &ReplaceFilterThingResponse{Thing: store(r.body.Name, r.body.Filters)}, nil, nil
}

func (ApiFilterThingsServiceDeleteFilterThingRequest) Execute() (map[string]interface{}, *http.Response, error) {
	return nil, nil, nil
}
