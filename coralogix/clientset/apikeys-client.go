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

func NewApiKeysClient(c *CallPropertiesCreator) *ApikeysClient {
	return &ApikeysClient{callPropertiesCreator: c}
}
