package echo_things_service

import (
	"context"
	"net/http"
)

type EchoThing struct {
	Id         string
	Name       string
	Kind       Kind
	CreateTime *string
	IsInternal *bool
	Quota      *Quota
	Items      []EchoItem
}

type Quota struct {
	Limit    int32
	Exceeded bool
}

type Kind string

const (
	KIND_KIND_UNSPECIFIED Kind = "KIND_UNSPECIFIED"
	KIND_KIND_LIMITED     Kind = "KIND_LIMITED"
	KIND_KIND_PLAIN       Kind = "KIND_PLAIN"
)

type Level string

const (
	LEVEL_LEVEL_UNSPECIFIED Level = "LEVEL_UNSPECIFIED"
	LEVEL_LEVEL_LOW         Level = "LEVEL_LOW"
	LEVEL_LEVEL_HIGH        Level = "LEVEL_HIGH"
)

type EchoItem struct {
	Key     string
	Values  []string
	Levels  []Level
	Enabled *bool
	None    map[string]interface{}
	Detail  *Detail
}

type Detail struct {
	Text    string
	Ratio   *float32
	Buckets []float32
}

type CreateEchoThingRequest struct {
	Name       string
	Kind       Kind
	LimitValue *int32
	Items      []EchoItem
}

type ReplaceEchoThingRequest struct {
	Id         string
	Name       string
	Kind       Kind
	CreateTime *string
	IsInternal *bool
	Quota      *Quota
	Items      []EchoItem
}

// The Create answer may leave out the resource; Get and Replace always have it.
type CreateEchoThingResponse struct {
	Thing *EchoThing
}

type GetEchoThingResponse struct {
	Thing EchoThing
}

type ReplaceEchoThingResponse struct {
	Thing EchoThing
}

// Replaced lists the Replace bodies that the client sent, and Answer is the resource that
// every call returns. Tests of the generated resource set and read them.
var (
	Created  []CreateEchoThingRequest
	Replaced []ReplaceEchoThingRequest
	Answer   EchoThing
)

type EchoThingsServiceAPIService struct{}

type ApiEchoThingsServiceCreateEchoThingRequest struct{ body CreateEchoThingRequest }
type ApiEchoThingsServiceGetEchoThingRequest struct{}
type ApiEchoThingsServiceReplaceEchoThingRequest struct{ body ReplaceEchoThingRequest }
type ApiEchoThingsServiceDeleteEchoThingRequest struct{}

func (*EchoThingsServiceAPIService) EchoThingsServiceCreateEchoThing(ctx context.Context) ApiEchoThingsServiceCreateEchoThingRequest {
	_ = ctx
	return ApiEchoThingsServiceCreateEchoThingRequest{}
}

func (*EchoThingsServiceAPIService) EchoThingsServiceGetEchoThing(ctx context.Context, id string) ApiEchoThingsServiceGetEchoThingRequest {
	_, _ = ctx, id
	return ApiEchoThingsServiceGetEchoThingRequest{}
}

func (*EchoThingsServiceAPIService) EchoThingsServiceReplaceEchoThing(ctx context.Context) ApiEchoThingsServiceReplaceEchoThingRequest {
	_ = ctx
	return ApiEchoThingsServiceReplaceEchoThingRequest{}
}

func (*EchoThingsServiceAPIService) EchoThingsServiceDeleteEchoThing(ctx context.Context, id string) ApiEchoThingsServiceDeleteEchoThingRequest {
	_, _ = ctx, id
	return ApiEchoThingsServiceDeleteEchoThingRequest{}
}

func (r ApiEchoThingsServiceCreateEchoThingRequest) CreateEchoThingRequest(createEchoThingRequest CreateEchoThingRequest) ApiEchoThingsServiceCreateEchoThingRequest {
	r.body = createEchoThingRequest
	return r
}

func (r ApiEchoThingsServiceReplaceEchoThingRequest) ReplaceEchoThingRequest(replaceEchoThingRequest ReplaceEchoThingRequest) ApiEchoThingsServiceReplaceEchoThingRequest {
	r.body = replaceEchoThingRequest
	return r
}

func (r ApiEchoThingsServiceCreateEchoThingRequest) Execute() (*CreateEchoThingResponse, *http.Response, error) {
	Created = append(Created, r.body)
	answer := Answer
	return &CreateEchoThingResponse{Thing: &answer}, nil, nil
}

func (ApiEchoThingsServiceGetEchoThingRequest) Execute() (*GetEchoThingResponse, *http.Response, error) {
	return &GetEchoThingResponse{Thing: Answer}, nil, nil
}

func (r ApiEchoThingsServiceReplaceEchoThingRequest) Execute() (*ReplaceEchoThingResponse, *http.Response, error) {
	Replaced = append(Replaced, r.body)
	return &ReplaceEchoThingResponse{Thing: Answer}, nil, nil
}

func (ApiEchoThingsServiceDeleteEchoThingRequest) Execute() (map[string]interface{}, *http.Response, error) {
	return nil, nil, nil
}
