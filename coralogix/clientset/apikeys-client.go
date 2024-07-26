// Copyright 2024 Coralogix Ltd.
// 
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// 
//     https://www.apache.org/licenses/LICENSE-2.0
// 
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package clientset

import (
	"context"
	apikeys "terraform-provider-coralogix/coralogix/clientset/grpc/apikeys"
)

type ApikeysClient struct {
	callPropertiesCreator *CallPropertiesCreator
}

func (t ApikeysClient) CreateApiKey(ctx context.Context, req *apikeys.CreateApiKeyRequest) (*apikeys.CreateApiKeyResponse, error) {
	callProperties, err := t.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := apikeys.NewApiKeysServiceClient(conn)

	return client.CreateApiKey(callProperties.Ctx, req, callProperties.CallOptions...)
}

func (t ApikeysClient) GetApiKey(ctx context.Context, req *apikeys.GetApiKeyRequest) (*apikeys.GetApiKeyResponse, error) {
	callProperties, err := t.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := apikeys.NewApiKeysServiceClient(conn)

	return client.GetApiKey(callProperties.Ctx, req, callProperties.CallOptions...)
}

func (t ApikeysClient) UpdateApiKey(ctx context.Context, req *apikeys.UpdateApiKeyRequest) (*apikeys.UpdateApiKeyResponse, error) {
	callProperties, err := t.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := apikeys.NewApiKeysServiceClient(conn)

	return client.UpdateApiKey(callProperties.Ctx, req, callProperties.CallOptions...)
}

func (t ApikeysClient) DeleteApiKey(ctx context.Context, req *apikeys.DeleteApiKeyRequest) (*apikeys.DeleteApiKeyResponse, error) {
	callProperties, err := t.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := apikeys.NewApiKeysServiceClient(conn)

	return client.DeleteApiKey(callProperties.Ctx, req, callProperties.CallOptions...)
}

func NewApiKeysClient(c *CallPropertiesCreator) *ApikeysClient {
	return &ApikeysClient{callPropertiesCreator: c}
}
