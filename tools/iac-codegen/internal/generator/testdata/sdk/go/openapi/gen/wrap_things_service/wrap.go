package wrap_things_service

import (
	"context"
	"net/http"
)

type WrapPriority string

const (
	WRAPPRIORITY_WRAP_PRIORITY_UNSPECIFIED WrapPriority = "WRAP_PRIORITY_UNSPECIFIED"
	WRAPPRIORITY_WRAP_PRIORITY_P1          WrapPriority = "WRAP_PRIORITY_P1"
	WRAPPRIORITY_WRAP_PRIORITY_P2          WrapPriority = "WRAP_PRIORITY_P2"
)

type LuceneQuery struct {
	Value *string
}

type PriorityValue struct {
	Value *WrapPriority
}

// Threshold requires its value, so the SDK holds it as a value, not a pointer.
type Threshold struct {
	Value float64
}

type UUID struct {
	Value *string
}

type FilterHolder struct {
	Filter *SimpleFilter
}

type SimpleFilter struct {
	Text  *string
	Limit *int32
}

type Selection struct {
	List *ListSelection
}

type ListSelection struct {
	Values []string
}

type Label struct {
	Value *string
}

// WrapThing requires query, so the SDK holds it as a value, not a pointer.
type WrapThing struct {
	Id         string
	Name       *string
	Query      LuceneQuery
	RawQuery   *LuceneQuery
	Priority   *PriorityValue
	Threshold  *Threshold
	WidgetIds  []UUID
	Filter     *FilterHolder
	Selection  *Selection
	MainLabel  *Label
	ExtraLabel *Label
}

type CreateWrapThingRequest struct {
	Name       *string
	Query      LuceneQuery
	RawQuery   *LuceneQuery
	Priority   *PriorityValue
	Threshold  *Threshold
	WidgetIds  []UUID
	Filter     *FilterHolder
	Selection  *Selection
	MainLabel  *Label
	ExtraLabel *Label
}

type ReplaceWrapThingRequest struct {
	Name       *string
	Query      LuceneQuery
	RawQuery   *LuceneQuery
	Priority   *PriorityValue
	Threshold  *Threshold
	WidgetIds  []UUID
	Filter     *FilterHolder
	Selection  *Selection
	MainLabel  *Label
	ExtraLabel *Label
}

type WrapThingsServiceAPIService struct{}

type ApiWrapThingsServiceCreateWrapThingRequest struct{}
type ApiWrapThingsServiceGetWrapThingRequest struct{}
type ApiWrapThingsServiceReplaceWrapThingRequest struct{}
type ApiWrapThingsServiceDeleteWrapThingRequest struct{}

func (*WrapThingsServiceAPIService) WrapThingsServiceCreateWrapThing(ctx context.Context) ApiWrapThingsServiceCreateWrapThingRequest {
	_ = ctx
	return ApiWrapThingsServiceCreateWrapThingRequest{}
}

func (*WrapThingsServiceAPIService) WrapThingsServiceGetWrapThing(ctx context.Context, id string) ApiWrapThingsServiceGetWrapThingRequest {
	_, _ = ctx, id
	return ApiWrapThingsServiceGetWrapThingRequest{}
}

func (*WrapThingsServiceAPIService) WrapThingsServiceReplaceWrapThing(ctx context.Context, id string) ApiWrapThingsServiceReplaceWrapThingRequest {
	_, _ = ctx, id
	return ApiWrapThingsServiceReplaceWrapThingRequest{}
}

func (*WrapThingsServiceAPIService) WrapThingsServiceDeleteWrapThing(ctx context.Context, id string) ApiWrapThingsServiceDeleteWrapThingRequest {
	_, _ = ctx, id
	return ApiWrapThingsServiceDeleteWrapThingRequest{}
}

func (r ApiWrapThingsServiceCreateWrapThingRequest) CreateWrapThingRequest(createWrapThingRequest CreateWrapThingRequest) ApiWrapThingsServiceCreateWrapThingRequest {
	_ = createWrapThingRequest
	return r
}

func (r ApiWrapThingsServiceReplaceWrapThingRequest) ReplaceWrapThingRequest(replaceWrapThingRequest ReplaceWrapThingRequest) ApiWrapThingsServiceReplaceWrapThingRequest {
	_ = replaceWrapThingRequest
	return r
}

func (ApiWrapThingsServiceCreateWrapThingRequest) Execute() (*WrapThing, *http.Response, error) {
	return nil, nil, nil
}

func (ApiWrapThingsServiceGetWrapThingRequest) Execute() (*WrapThing, *http.Response, error) {
	return nil, nil, nil
}

func (ApiWrapThingsServiceReplaceWrapThingRequest) Execute() (*WrapThing, *http.Response, error) {
	return nil, nil, nil
}

func (ApiWrapThingsServiceDeleteWrapThingRequest) Execute() (map[string]interface{}, *http.Response, error) {
	return nil, nil, nil
}
