package clientset

import (
	"context"
	"fmt"

	"terraform-provider-coralogix/coralogix/clientset/REST"
)

type WebhooksClient struct {
	client *REST.Client
}

func (w WebhooksClient) CreateWebhook(ctx context.Context, body interface{}) (map[string]interface{}, error) {
	return w.client.Post(ctx, "integrations", body)
}

func (w WebhooksClient) GetWebhook(ctx context.Context, webhookId string) (map[string]interface{}, error) {
	return w.client.Get(ctx, fmt.Sprintf("integrations/%s", webhookId))
}

func (w WebhooksClient) UpdateWebhook(ctx context.Context, body interface{}) (map[string]interface{}, error) {
	return w.client.Post(ctx, "integrations", body)
}

func (w WebhooksClient) DeleteWebhook(ctx context.Context, webhookId string) (map[string]interface{}, error) {
	return w.client.Delete(ctx, fmt.Sprintf("integrations/%s", webhookId))
}

func NewWebhooksClient(client *REST.Client) *WebhooksClient {
	return &WebhooksClient{client: client}
}
