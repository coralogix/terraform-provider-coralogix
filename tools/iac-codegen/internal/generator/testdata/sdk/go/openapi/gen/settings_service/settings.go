package settings_service

import (
	"context"
	"net/http"
)

type Settings struct {
	RetentionDays *int32
}

type CreateSettingsRequest struct {
	RetentionDays int32
}

type UpdateSettingsRequest struct {
	RetentionDays *int32
}

type SettingsServiceAPIService struct{}

type ApiSettingsServiceCreateSettingsRequest struct{}
type ApiSettingsServiceGetSettingsRequest struct{}
type ApiSettingsServiceUpdateSettingsRequest struct{}
type ApiSettingsServiceDeleteSettingsRequest struct{}

func (*SettingsServiceAPIService) SettingsServiceCreateSettings(ctx context.Context) ApiSettingsServiceCreateSettingsRequest {
	_ = ctx
	return ApiSettingsServiceCreateSettingsRequest{}
}

func (*SettingsServiceAPIService) SettingsServiceGetSettings(ctx context.Context) ApiSettingsServiceGetSettingsRequest {
	_ = ctx
	return ApiSettingsServiceGetSettingsRequest{}
}

func (*SettingsServiceAPIService) SettingsServiceUpdateSettings(ctx context.Context) ApiSettingsServiceUpdateSettingsRequest {
	_ = ctx
	return ApiSettingsServiceUpdateSettingsRequest{}
}

func (*SettingsServiceAPIService) SettingsServiceDeleteSettings(ctx context.Context) ApiSettingsServiceDeleteSettingsRequest {
	_ = ctx
	return ApiSettingsServiceDeleteSettingsRequest{}
}

func (r ApiSettingsServiceCreateSettingsRequest) CreateSettingsRequest(createSettingsRequest CreateSettingsRequest) ApiSettingsServiceCreateSettingsRequest {
	_ = createSettingsRequest
	return r
}

func (r ApiSettingsServiceUpdateSettingsRequest) UpdateSettingsRequest(updateSettingsRequest UpdateSettingsRequest) ApiSettingsServiceUpdateSettingsRequest {
	_ = updateSettingsRequest
	return r
}

func (r ApiSettingsServiceUpdateSettingsRequest) UpdateMask(updateMask string) ApiSettingsServiceUpdateSettingsRequest {
	_ = updateMask
	return r
}

func (ApiSettingsServiceCreateSettingsRequest) Execute() (*Settings, *http.Response, error) {
	return nil, nil, nil
}

func (ApiSettingsServiceGetSettingsRequest) Execute() (*Settings, *http.Response, error) {
	return nil, nil, nil
}

func (ApiSettingsServiceUpdateSettingsRequest) Execute() (*Settings, *http.Response, error) {
	return nil, nil, nil
}

func (ApiSettingsServiceDeleteSettingsRequest) Execute() (map[string]interface{}, *http.Response, error) {
	return nil, nil, nil
}
