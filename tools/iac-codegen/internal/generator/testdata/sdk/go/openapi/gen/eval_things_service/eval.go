package eval_things_service

import (
	"context"
	"net/http"
)

type EvalTarget string

const (
	EVALTARGET_PROMPT       EvalTarget = "PROMPT"
	EVALTARGET_RESPONSE     EvalTarget = "RESPONSE"
	EVALTARGET_CONVERSATION EvalTarget = "CONVERSATION"
)

type TopicsEvalConfig struct {
	Topics  []string
	Context *string
}

// LegacyEvalConfig is the arm the resource does not configure. Flatten only reports it.
type LegacyEvalConfig struct {
	Limit *string
}

type EvalConfig struct {
	Topics   *TopicsEvalConfig
	Toxicity map[string]interface{}
	Legacy   *LegacyEvalConfig
}

type EvalThing struct {
	Id      *string
	Name    *string
	Target  *EvalTarget
	Config  *EvalConfig
	Enabled *bool
}

type CreateEvalThingRequest struct {
	Name    *string
	Target  *EvalTarget
	Config  *EvalConfig
	Enabled *bool
}

// UpdateEvalThingRequest has an UpdateMask, which the released resource never sent.
type UpdateEvalThingRequest struct {
	Name       *string
	Target     *EvalTarget
	Config     *EvalConfig
	Enabled    *bool
	UpdateMask *string
}

type CreateEvalThingResponse struct {
	Thing *EvalThing
}

type GetEvalThingResponse struct {
	Thing *EvalThing
}

type UpdateEvalThingResponse struct {
	Thing *EvalThing
}

type EvalThingsServiceAPIService struct{}

type ApiEvalThingsServiceCreateEvalThingRequest struct {
	body CreateEvalThingRequest
}
type ApiEvalThingsServiceGetEvalThingRequest struct {
	id string
}
type ApiEvalThingsServiceUpdateEvalThingRequest struct {
	id   string
	body UpdateEvalThingRequest
}
type ApiEvalThingsServiceDeleteEvalThingRequest struct{}

// Created and Updated list the bodies that the calls received. Get returns Stored.
var (
	Created []CreateEvalThingRequest
	Updated []UpdateEvalThingRequest
	Stored  EvalThing
)

func (*EvalThingsServiceAPIService) EvalThingsServiceCreateEvalThing(ctx context.Context) ApiEvalThingsServiceCreateEvalThingRequest {
	_ = ctx
	return ApiEvalThingsServiceCreateEvalThingRequest{}
}

func (*EvalThingsServiceAPIService) EvalThingsServiceGetEvalThing(ctx context.Context, id string) ApiEvalThingsServiceGetEvalThingRequest {
	_ = ctx
	return ApiEvalThingsServiceGetEvalThingRequest{id: id}
}

func (*EvalThingsServiceAPIService) EvalThingsServiceUpdateEvalThing(ctx context.Context, id string) ApiEvalThingsServiceUpdateEvalThingRequest {
	_ = ctx
	return ApiEvalThingsServiceUpdateEvalThingRequest{id: id}
}

func (*EvalThingsServiceAPIService) EvalThingsServiceDeleteEvalThing(ctx context.Context, id string) ApiEvalThingsServiceDeleteEvalThingRequest {
	_, _ = ctx, id
	return ApiEvalThingsServiceDeleteEvalThingRequest{}
}

func (r ApiEvalThingsServiceCreateEvalThingRequest) CreateEvalThingRequest(createEvalThingRequest CreateEvalThingRequest) ApiEvalThingsServiceCreateEvalThingRequest {
	r.body = createEvalThingRequest
	return r
}

func (r ApiEvalThingsServiceUpdateEvalThingRequest) UpdateEvalThingRequest(updateEvalThingRequest UpdateEvalThingRequest) ApiEvalThingsServiceUpdateEvalThingRequest {
	r.body = updateEvalThingRequest
	return r
}

func (r ApiEvalThingsServiceCreateEvalThingRequest) Execute() (*CreateEvalThingResponse, *http.Response, error) {
	Created = append(Created, r.body)
	id := "thing-1"
	Stored = EvalThing{Id: &id, Name: r.body.Name, Target: r.body.Target, Config: r.body.Config, Enabled: r.body.Enabled}
	return &CreateEvalThingResponse{Thing: &Stored}, nil, nil
}

func (r ApiEvalThingsServiceGetEvalThingRequest) Execute() (*GetEvalThingResponse, *http.Response, error) {
	_ = r.id
	return &GetEvalThingResponse{Thing: &Stored}, nil, nil
}

func (r ApiEvalThingsServiceUpdateEvalThingRequest) Execute() (*UpdateEvalThingResponse, *http.Response, error) {
	Updated = append(Updated, r.body)
	if r.body.Config != nil {
		Stored.Config = r.body.Config
	}
	if r.body.Enabled != nil {
		Stored.Enabled = r.body.Enabled
	}
	return &UpdateEvalThingResponse{Thing: &Stored}, nil, nil
}

func (ApiEvalThingsServiceDeleteEvalThingRequest) Execute() (map[string]interface{}, *http.Response, error) {
	return nil, nil, nil
}
