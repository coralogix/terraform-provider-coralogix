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

	webhooks "terraform-provider-coralogix/coralogix/clientset/grpc/webhooks"
)

type WebhooksClient struct {
	callPropertiesCreator *CallPropertiesCreator
}

func (c WebhooksClient) CreateWebhook(ctx context.Context, req *webhooks.CreateOutgoingWebhookRequest) (*webhooks.CreateOutgoingWebhookResponse, error) {
	callProperties, err := c.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := webhooks.NewOutgoingWebhooksServiceClient(conn)

	return client.CreateOutgoingWebhook(callProperties.Ctx, req, callProperties.CallOptions...)
}

func (c WebhooksClient) GetWebhook(ctx context.Context, req *webhooks.GetOutgoingWebhookRequest) (*webhooks.GetOutgoingWebhookResponse, error) {
	callProperties, err := c.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := webhooks.NewOutgoingWebhooksServiceClient(conn)

	return client.GetOutgoingWebhook(callProperties.Ctx, req, callProperties.CallOptions...)
}

func (c WebhooksClient) UpdateWebhook(ctx context.Context, req *webhooks.UpdateOutgoingWebhookRequest) (*webhooks.UpdateOutgoingWebhookResponse, error) {
	callProperties, err := c.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := webhooks.NewOutgoingWebhooksServiceClient(conn)

	return client.UpdateOutgoingWebhook(callProperties.Ctx, req, callProperties.CallOptions...)
}

func (c WebhooksClient) DeleteWebhook(ctx context.Context, req *webhooks.DeleteOutgoingWebhookRequest) (*webhooks.DeleteOutgoingWebhookResponse, error) {
	callProperties, err := c.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := webhooks.NewOutgoingWebhooksServiceClient(conn)

	return client.DeleteOutgoingWebhook(callProperties.Ctx, req, callProperties.CallOptions...)
}

func (c WebhooksClient) ListWebhooks(ctx context.Context, req *webhooks.ListAllOutgoingWebhooksRequest) (*webhooks.ListAllOutgoingWebhooksResponse, error) {
	callProperties, err := c.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := webhooks.NewOutgoingWebhooksServiceClient(conn)

	return client.ListAllOutgoingWebhooks(callProperties.Ctx, req, callProperties.CallOptions...)
}

func NewWebhooksClient(c *CallPropertiesCreator) *WebhooksClient {
	return &WebhooksClient{callPropertiesCreator: c}
}
