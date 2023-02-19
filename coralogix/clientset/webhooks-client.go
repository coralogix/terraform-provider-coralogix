package clientset

import (
	"context"
	"fmt"
	"strings"

	"terraform-provider-coralogix/coralogix/clientset/rest"
)

type WebhooksClient struct {
	client *rest.Client
}

func (w WebhooksClient) CreateWebhook(ctx context.Context, body string) (string, error) {
	return w.client.Post(ctx, "/api/v1/external/integrations", "application/json", body)
}

func (w WebhooksClient) GetWebhook(ctx context.Context, webhookId string) (string, error) {
	return w.client.Get(ctx, fmt.Sprintf("/api/v1/external/integrations/%s", webhookId))
}

func (w WebhooksClient) UpdateWebhook(ctx context.Context, body string) (string, error) {
	return w.client.Post(ctx, "/api/v1/external/integrations", "application/json", body)
}

func (w WebhooksClient) DeleteWebhook(ctx context.Context, webhookId string) (string, error) {
	return w.client.Delete(ctx, fmt.Sprintf("/api/v1/external/integrations/%s", webhookId))
}

func NewWebhooksClient(c *CallPropertiesCreator) *WebhooksClient {
	targetUrl := "https://" + strings.Replace(c.targetUrl, "grpc", "http", 1)
	client := rest.NewRestClient(targetUrl, c.apiKey)
	return &WebhooksClient{client: client}
}
