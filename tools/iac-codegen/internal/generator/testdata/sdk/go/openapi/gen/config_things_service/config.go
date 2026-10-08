package config_things_service

import (
	"context"
	"net/http"
)

type ConfigRemote struct {
	Name             *string
	RawConfiguration *string
}

type ConfigThing struct {
	Id       string
	Name     *string
	Settings *string
	Template *string
	Version  *string
	Remotes  []ConfigRemote
	Labels   []string
	Metadata map[string]string
}

type CreateConfigThingRequest struct {
	Name     *string
	Settings *string
	Template *string
	Remotes  []ConfigRemote
	Labels   []string
	Metadata map[string]string
}

type ReplaceConfigThingRequest struct {
	Name     *string
	Settings *string
	Remotes  []ConfigRemote
	Labels   []string
	Metadata map[string]string
}

type ConfigThingsServiceAPIService struct{}

type ApiConfigThingsServiceCreateConfigThingRequest struct{}
type ApiConfigThingsServiceGetConfigThingRequest struct{}
type ApiConfigThingsServiceReplaceConfigThingRequest struct{}
type ApiConfigThingsServiceDeleteConfigThingRequest struct{}

func (*ConfigThingsServiceAPIService) ConfigThingsServiceCreateConfigThing(ctx context.Context) ApiConfigThingsServiceCreateConfigThingRequest {
	_ = ctx
	return ApiConfigThingsServiceCreateConfigThingRequest{}
}

func (*ConfigThingsServiceAPIService) ConfigThingsServiceGetConfigThing(ctx context.Context, id string) ApiConfigThingsServiceGetConfigThingRequest {
	_, _ = ctx, id
	return ApiConfigThingsServiceGetConfigThingRequest{}
}

func (*ConfigThingsServiceAPIService) ConfigThingsServiceReplaceConfigThing(ctx context.Context, id string) ApiConfigThingsServiceReplaceConfigThingRequest {
	_, _ = ctx, id
	return ApiConfigThingsServiceReplaceConfigThingRequest{}
}

func (*ConfigThingsServiceAPIService) ConfigThingsServiceDeleteConfigThing(ctx context.Context, id string) ApiConfigThingsServiceDeleteConfigThingRequest {
	_, _ = ctx, id
	return ApiConfigThingsServiceDeleteConfigThingRequest{}
}

func (r ApiConfigThingsServiceCreateConfigThingRequest) CreateConfigThingRequest(createConfigThingRequest CreateConfigThingRequest) ApiConfigThingsServiceCreateConfigThingRequest {
	_ = createConfigThingRequest
	return r
}

func (r ApiConfigThingsServiceReplaceConfigThingRequest) ReplaceConfigThingRequest(replaceConfigThingRequest ReplaceConfigThingRequest) ApiConfigThingsServiceReplaceConfigThingRequest {
	_ = replaceConfigThingRequest
	return r
}

func (ApiConfigThingsServiceCreateConfigThingRequest) Execute() (*ConfigThing, *http.Response, error) {
	return nil, nil, nil
}

func (ApiConfigThingsServiceGetConfigThingRequest) Execute() (*ConfigThing, *http.Response, error) {
	return nil, nil, nil
}

func (ApiConfigThingsServiceReplaceConfigThingRequest) Execute() (*ConfigThing, *http.Response, error) {
	return nil, nil, nil
}

func (ApiConfigThingsServiceDeleteConfigThingRequest) Execute() (map[string]interface{}, *http.Response, error) {
	return nil, nil, nil
}
