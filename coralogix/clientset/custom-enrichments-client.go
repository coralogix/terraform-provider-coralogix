package clientset

import (
	"context"

	enrichmentv1 "terraform-provider-coralogix/coralogix/clientset/grpc/com/coralogix/enrichment/v1"
)

type EnrichmentDataClient struct {
	callPropertiesCreator *CallPropertiesCreator
}

func (e EnrichmentDataClient) CreatEnrichmentData(ctx context.Context, req *enrichmentv1.CreateCustomEnrichmentRequest) (*enrichmentv1.CreateCustomEnrichmentResponse, error) {
	callProperties, err := e.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()

	client := enrichmentv1.NewCustomEnrichmentServiceClient(conn)

	return client.CreateCustomEnrichment(callProperties.Ctx, req, callProperties.CallOptions...)
}

func (e EnrichmentDataClient) GetEnrichmentData(ctx context.Context, req *enrichmentv1.GetCustomEnrichmentRequest) (*enrichmentv1.GetCustomEnrichmentResponse, error) {
	callProperties, err := e.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()

	client := enrichmentv1.NewCustomEnrichmentServiceClient(conn)

	return client.GetCustomEnrichment(callProperties.Ctx, req, callProperties.CallOptions...)
}

func (e EnrichmentDataClient) UpdateEnrichmentData(ctx context.Context, req *enrichmentv1.UpdateCustomEnrichmentRequest) (*enrichmentv1.UpdateCustomEnrichmentResponse, error) {
	callProperties, err := e.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()

	client := enrichmentv1.NewCustomEnrichmentServiceClient(conn)

	return client.UpdateCustomEnrichment(callProperties.Ctx, req, callProperties.CallOptions...)
}

func (e EnrichmentDataClient) DeleteEnrichmentData(ctx context.Context, req *enrichmentv1.DeleteCustomEnrichmentRequest) (*enrichmentv1.DeleteCustomEnrichmentResponse, error) {
	callProperties, err := e.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()

	client := enrichmentv1.NewCustomEnrichmentServiceClient(conn)

	return client.DeleteCustomEnrichment(callProperties.Ctx, req, callProperties.CallOptions...)
}

func NewEnrichmentDataClient(c *CallPropertiesCreator) *EnrichmentDataClient {
	return &EnrichmentDataClient{callPropertiesCreator: c}
}
