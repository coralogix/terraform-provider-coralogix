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

type TypedMode string

const (
	TYPEDMODE_MODE_UNSPECIFIED TypedMode = "MODE_UNSPECIFIED"
	TYPEDMODE_MODE_FAST        TypedMode = "MODE_FAST"
	TYPEDMODE_MODE_EXACT       TypedMode = "MODE_EXACT"
)

// Mode is an enum here and free text in TypedPercentile: the two arms share the attribute.
type TypedAverage struct {
	Field  *string
	Mode   *TypedMode
	Weight *float64
	// AverageUnit and PercentileUnit are one attribute, unit.
	AverageUnit *string
}

type TypedPercentile struct {
	Field          *string
	Mode           *string
	Weight         *float64
	Percent        *float64
	Bucket         *TypedBucket
	PercentileUnit *string
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

type TypedList struct {
	Values []string
	Note   *string
}

// A oneOf: All has no fields, List has the values.
type TypedSelection struct {
	All  map[string]interface{}
	List *TypedList
}

type TypedNotList struct {
	Values []string
}

// A object with one field, as the selection of a notEquals operator is: only a list.
type TypedNotSelection struct {
	List *TypedNotList
}

type TypedEquals struct {
	Selection *TypedSelection
}

type TypedNotEquals struct {
	Selection *TypedNotSelection
}

// A oneOf of two objects that each hold one selection.
type TypedOperator struct {
	Equals    *TypedEquals
	NotEquals *TypedNotEquals
}

type TypedPickList struct {
	Values []string
	Labels []string
}

// A oneOf: All has no fields, List has a list of values.
type TypedPick struct {
	All  map[string]interface{}
	List *TypedPickList
}

type TypedThing struct {
	Pick        *TypedPick
	Id          *string
	Name        *string
	Aggregation *TypedAggregation
	Selection   *TypedSelection
	Operator    *TypedOperator
	Fields      []TypedSpanField
}

type CreateTypedThingBody struct {
	Pick        *TypedPick
	Name        *string
	Aggregation *TypedAggregation
	Selection   *TypedSelection
	Operator    *TypedOperator
	Fields      []TypedSpanField
}

type ReplaceTypedThingBody struct {
	Pick        *TypedPick
	Name        *string
	Aggregation *TypedAggregation
	Selection   *TypedSelection
	Operator    *TypedOperator
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

func store(name *string, aggregation *TypedAggregation, selection *TypedSelection, pick *TypedPick, operator *TypedOperator, fields []TypedSpanField) *TypedThing {
	id := "thing-1"
	Stored = TypedThing{Id: &id, Name: name, Aggregation: aggregation, Selection: selection, Pick: pick, Operator: operator, Fields: fields}
	stored := Stored
	return &stored
}

func (r ApiTypedThingsServiceCreateTypedThingRequest) Execute() (*CreateTypedThingResponse, *http.Response, error) {
	Created = append(Created, r.body)
	return &CreateTypedThingResponse{Thing: store(r.body.Name, r.body.Aggregation, r.body.Selection, r.body.Pick, r.body.Operator, r.body.Fields)}, nil, nil
}

func (r ApiTypedThingsServiceGetTypedThingRequest) Execute() (*GetTypedThingResponse, *http.Response, error) {
	stored := Stored
	return &GetTypedThingResponse{Thing: &stored}, nil, nil
}

func (r ApiTypedThingsServiceReplaceTypedThingRequest) Execute() (*ReplaceTypedThingResponse, *http.Response, error) {
	Replaced = append(Replaced, r.body)
	return &ReplaceTypedThingResponse{Thing: store(r.body.Name, r.body.Aggregation, r.body.Selection, r.body.Pick, r.body.Operator, r.body.Fields)}, nil, nil
}

func (ApiTypedThingsServiceDeleteTypedThingRequest) Execute() (map[string]interface{}, *http.Response, error) {
	return nil, nil, nil
}
