package rulebook_service

import (
	"context"
	"net/http"
)

type RulebookMode string

const (
	RULEBOOKMODE_RULEBOOK_MODE_UNSPECIFIED RulebookMode = "RULEBOOK_MODE_UNSPECIFIED"
	RULEBOOKMODE_RULEBOOK_MODE_ONE         RulebookMode = "RULEBOOK_MODE_ONE"
	RULEBOOKMODE_RULEBOOK_MODE_TWO         RulebookMode = "RULEBOOK_MODE_TWO"
)

type RulebookRule struct {
	EntityType string
	Allocation *float32
	Mode       *RulebookMode
}

type Rulebook struct {
	Id            *string
	RetentionDays *int32
	Rules         []RulebookRule
}

type CreateRulebookRequest struct {
	RetentionDays int32
	Rules         []RulebookRule
}

type UpdateRulebookRequest struct {
	RetentionDays *int32
	Rules         []RulebookRule
}

type RulebookServiceAPIService struct{}

type ApiRulebookServiceCreateRulebookRequest struct{}
type ApiRulebookServiceGetRulebookRequest struct{}
type ApiRulebookServiceUpdateRulebookRequest struct{}
type ApiRulebookServiceDeleteRulebookRequest struct{}

func (*RulebookServiceAPIService) RulebookServiceCreateRulebook(ctx context.Context) ApiRulebookServiceCreateRulebookRequest {
	_ = ctx
	return ApiRulebookServiceCreateRulebookRequest{}
}

func (*RulebookServiceAPIService) RulebookServiceGetRulebook(ctx context.Context) ApiRulebookServiceGetRulebookRequest {
	_ = ctx
	return ApiRulebookServiceGetRulebookRequest{}
}

func (*RulebookServiceAPIService) RulebookServiceUpdateRulebook(ctx context.Context) ApiRulebookServiceUpdateRulebookRequest {
	_ = ctx
	return ApiRulebookServiceUpdateRulebookRequest{}
}

func (*RulebookServiceAPIService) RulebookServiceDeleteRulebook(ctx context.Context) ApiRulebookServiceDeleteRulebookRequest {
	_ = ctx
	return ApiRulebookServiceDeleteRulebookRequest{}
}

func (r ApiRulebookServiceCreateRulebookRequest) CreateRulebookRequest(createRulebookRequest CreateRulebookRequest) ApiRulebookServiceCreateRulebookRequest {
	_ = createRulebookRequest
	return r
}

func (r ApiRulebookServiceUpdateRulebookRequest) UpdateRulebookRequest(updateRulebookRequest UpdateRulebookRequest) ApiRulebookServiceUpdateRulebookRequest {
	_ = updateRulebookRequest
	return r
}

func (r ApiRulebookServiceUpdateRulebookRequest) UpdateMask(updateMask string) ApiRulebookServiceUpdateRulebookRequest {
	_ = updateMask
	return r
}

func (ApiRulebookServiceCreateRulebookRequest) Execute() (*Rulebook, *http.Response, error) {
	return nil, nil, nil
}

func (ApiRulebookServiceGetRulebookRequest) Execute() (*Rulebook, *http.Response, error) {
	return nil, nil, nil
}

func (ApiRulebookServiceUpdateRulebookRequest) Execute() (*Rulebook, *http.Response, error) {
	return nil, nil, nil
}

func (ApiRulebookServiceDeleteRulebookRequest) Execute() (map[string]interface{}, *http.Response, error) {
	return nil, nil, nil
}
