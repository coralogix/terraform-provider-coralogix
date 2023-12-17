package clientset

import (
	"context"

	archiveRetention "terraform-provider-coralogix/coralogix/clientset/grpc/archive-retentions"
)

type ArchiveRetentionsClient struct {
	callPropertiesCreator *CallPropertiesCreator
}

func (c ArchiveRetentionsClient) GetRetentions(ctx context.Context, req *archiveRetention.GetRetentionsRequest) (*archiveRetention.GetRetentionsResponse, error) {
	callProperties, err := c.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := archiveRetention.NewRetentionsServiceClient(conn)

	return client.GetRetentions(callProperties.Ctx, req, callProperties.CallOptions...)
}

func (c ArchiveRetentionsClient) UpdateRetentions(ctx context.Context, req *archiveRetention.UpdateRetentionsRequest) (*archiveRetention.UpdateRetentionsResponse, error) {
	callProperties, err := c.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := archiveRetention.NewRetentionsServiceClient(conn)

	return client.UpdateRetentions(callProperties.Ctx, req, callProperties.CallOptions...)
}

func (c ArchiveRetentionsClient) ActivateRetentions(ctx context.Context, req *archiveRetention.ActivateRetentionsRequest) (*archiveRetention.ActivateRetentionsResponse, error) {
	callProperties, err := c.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := archiveRetention.NewRetentionsServiceClient(conn)

	return client.ActivateRetentions(callProperties.Ctx, req, callProperties.CallOptions...)
}

func (c ArchiveRetentionsClient) GetRetentionsEnabled(ctx context.Context, req *archiveRetention.GetRetentionsEnabledRequest) (*archiveRetention.GetRetentionsEnabledResponse, error) {
	callProperties, err := c.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := archiveRetention.NewRetentionsServiceClient(conn)

	return client.GetRetentionsEnabled(callProperties.Ctx, req, callProperties.CallOptions...)
}

func NewArchiveRetentionsClient(c *CallPropertiesCreator) *ArchiveRetentionsClient {
	return &ArchiveRetentionsClient{callPropertiesCreator: c}
}
