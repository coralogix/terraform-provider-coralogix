package typed_things_service

import (
	"context"
	"net/http"
)

type TypedMetadataField string

const (
	TYPEDMETADATAFIELD_METADATA_FIELD_UNSPECIFIED      TypedMetadataField = "METADATA_FIELD_UNSPECIFIED"
	TYPEDMETADATAFIELD_METADATA_FIELD_SERVICE_NAME     TypedMetadataField = "METADATA_FIELD_SERVICE_NAME"
	TYPEDMETADATAFIELD_METADATA_FIELD_APPLICATION_NAME TypedMetadataField = "METADATA_FIELD_APPLICATION_NAME"
)

type TypedBucket struct {
	Size *string
}

type TypedAverage struct {
	Field *string
}

type TypedPercentile struct {
	Field   *string
	Percent *float64
	Bucket  *TypedBucket
}

// A oneOf: one arm is set. Count is an arm with no fields.
type TypedAggregation struct {
	Count      map[string]interface{}
	Average    *TypedAverage
	Percentile *TypedPercentile
}

// A oneOf whose arms are scalars: an enum and two strings.
type TypedSpanField struct {
	MetadataField   *TypedMetadataField
	TagField        *string
	ProcessTagField *string
}

type TypedThing struct {
	Id          *string
	Name        *string
	Aggregation *TypedAggregation
	Fields      []TypedSpanField
}

type CreateTypedThingBody struct {
	Name        *string
	Aggregation *TypedAggregation
	Fields      []TypedSpanField
}

type ReplaceTypedThingBody struct {
	Name        *string
	Aggregation *TypedAggregation
	Fields      []TypedSpanField
}

type CreateTypedThingResponse struct {
	Thing *TypedThing
}

type ReplaceTypedThingResponse struct {
	Thing *TypedThing
}

type GetTypedThingResponse struct {
	Thing *TypedThing
}

type TypedThingsServiceAPIService struct{}

type ApiTypedThingsServiceCreateTypedThingRequest struct {
	body CreateTypedThingBody
}
type ApiTypedThingsServiceGetTypedThingRequest struct {
	id string
}
type ApiTypedThingsServiceReplaceTypedThingRequest struct {
	id   string
	body ReplaceTypedThingBody
}
type ApiTypedThingsServiceDeleteTypedThingRequest struct{}

// Created and Replaced list the bodies that the calls received. Stored is the thing that the
// calls return and that Get reads.
var (
	Created  []CreateTypedThingBody
	Replaced []ReplaceTypedThingBody
	Stored   TypedThing
)

func (*TypedThingsServiceAPIService) TypedThingsServiceCreateTypedThing(ctx context.Context) ApiTypedThingsServiceCreateTypedThingRequest {
	_ = ctx
	return ApiTypedThingsServiceCreateTypedThingRequest{}
}

func (*TypedThingsServiceAPIService) TypedThingsServiceGetTypedThing(ctx context.Context, thingId string) ApiTypedThingsServiceGetTypedThingRequest {
	_ = ctx
	return ApiTypedThingsServiceGetTypedThingRequest{id: thingId}
}

func (*TypedThingsServiceAPIService) TypedThingsServiceReplaceTypedThing(ctx context.Context, thingId string) ApiTypedThingsServiceReplaceTypedThingRequest {
	_ = ctx
	return ApiTypedThingsServiceReplaceTypedThingRequest{id: thingId}
}

func (*TypedThingsServiceAPIService) TypedThingsServiceDeleteTypedThing(ctx context.Context, thingId string) ApiTypedThingsServiceDeleteTypedThingRequest {
	_, _ = ctx, thingId
	return ApiTypedThingsServiceDeleteTypedThingRequest{}
}

func (r ApiTypedThingsServiceCreateTypedThingRequest) CreateTypedThingBody(createTypedThingBody CreateTypedThingBody) ApiTypedThingsServiceCreateTypedThingRequest {
	r.body = createTypedThingBody
	return r
}

func (r ApiTypedThingsServiceReplaceTypedThingRequest) ReplaceTypedThingBody(replaceTypedThingBody ReplaceTypedThingBody) ApiTypedThingsServiceReplaceTypedThingRequest {
	r.body = replaceTypedThingBody
	return r
}

func store(name *string, aggregation *TypedAggregation, fields []TypedSpanField) *TypedThing {
	id := "thing-1"
	Stored = TypedThing{Id: &id, Name: name, Aggregation: aggregation, Fields: fields}
	stored := Stored
	return &stored
}

func (r ApiTypedThingsServiceCreateTypedThingRequest) Execute() (*CreateTypedThingResponse, *http.Response, error) {
	Created = append(Created, r.body)
	return &CreateTypedThingResponse{Thing: store(r.body.Name, r.body.Aggregation, r.body.Fields)}, nil, nil
}

func (r ApiTypedThingsServiceGetTypedThingRequest) Execute() (*GetTypedThingResponse, *http.Response, error) {
	stored := Stored
	return &GetTypedThingResponse{Thing: &stored}, nil, nil
}

func (r ApiTypedThingsServiceReplaceTypedThingRequest) Execute() (*ReplaceTypedThingResponse, *http.Response, error) {
	Replaced = append(Replaced, r.body)
	return &ReplaceTypedThingResponse{Thing: store(r.body.Name, r.body.Aggregation, r.body.Fields)}, nil, nil
}

func (ApiTypedThingsServiceDeleteTypedThingRequest) Execute() (map[string]interface{}, *http.Response, error) {
	return nil, nil, nil
}
