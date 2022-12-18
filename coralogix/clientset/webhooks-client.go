package clientset

import (
	"terraform-provider-coralogix/coralogix/clientset/REST"
)

type WebhooksClient struct {
	client *REST.Client
}

func NewWebhooksClient(client *REST.Client) *WebhooksClient {
	return &WebhooksClient{client: client}
}
