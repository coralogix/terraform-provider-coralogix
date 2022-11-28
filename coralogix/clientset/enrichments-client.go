package clientset

import (
	"context"

	enrichmentv1 "terraform-provider-coralogix/coralogix/clientset/grpc/com/coralogix/enrichment/v1"
)

type EnrichmentsClient struct {
	callPropertiesCreator *CallPropertiesCreator
}

func (e EnrichmentsClient) CreateEnrichment(ctx context.Context, req *enrichmentv1.AddEnrichmentsRequest) (*enrichmentv1.AddEnrichmentsResponse, error) {
	callProperties, err := e.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := enrichmentv1.NewEnrichmentServiceClient(conn)

	return client.AddEnrichments(callProperties.Ctx, req, callProperties.CallOptions...)
}

func (e EnrichmentsClient) GetEnrichments(ctx context.Context, req *enrichmentv1.GetEnrichmentsRequest) (*enrichmentv1.GetEnrichmentsResponse, error) {
	callProperties, err := e.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := enrichmentv1.NewEnrichmentServiceClient(conn)

	return client.GetEnrichments(callProperties.Ctx, req, callProperties.CallOptions...)
}

func (e EnrichmentsClient) DeleteEnrichment(ctx context.Context, req *enrichmentv1.RemoveEnrichmentsRequest) (*enrichmentv1.RemoveEnrichmentsResponse, error) {
	callProperties, err := e.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()

	client := enrichmentv1.NewEnrichmentServiceClient(conn)

	return client.RemoveEnrichments(callProperties.Ctx, req, callProperties.CallOptions...)
}

func NewEnrichmentClient(c *CallPropertiesCreator) *EnrichmentsClient {
	return &EnrichmentsClient{callPropertiesCreator: c}
}
