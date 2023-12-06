package clientset

import (
	"context"

	enrichment "terraform-provider-coralogix/coralogix/clientset/grpc/enrichment/v1"
)

type EnrichmentsClient struct {
	callPropertiesCreator *CallPropertiesCreator
}

func (e EnrichmentsClient) CreateEnrichments(ctx context.Context, req *enrichment.AddEnrichmentsRequest) ([]*enrichment.Enrichment, error) {
	callProperties, err := e.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := enrichment.NewEnrichmentServiceClient(conn)

	resp, err := client.AddEnrichments(callProperties.Ctx, req, callProperties.CallOptions...)
	if err != nil {
		return nil, err
	}

	enrichments := resp.GetEnrichments()
	from := len(enrichments) - len(req.GetRequestEnrichments())
	to := len(enrichments)
	return enrichments[from:to], nil
}

func (e EnrichmentsClient) GetEnrichmentsByType(ctx context.Context, enrichmentType string) ([]*enrichment.Enrichment, error) {
	callProperties, err := e.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := enrichment.NewEnrichmentServiceClient(conn)

	resp, err := client.GetEnrichments(callProperties.Ctx, &enrichment.GetEnrichmentsRequest{}, callProperties.CallOptions...)
	if err != nil {
		return nil, err
	}

	result := make([]*enrichment.Enrichment, 0)
	for _, enrichment := range resp.GetEnrichments() {
		if enrichment.GetEnrichmentType().String() == enrichmentType+":{}" {
			result = append(result, enrichment)
		}
	}

	return result, nil
}

func (e EnrichmentsClient) GetCustomEnrichments(ctx context.Context, customEnrichmentId uint32) ([]*enrichment.Enrichment, error) {
	callProperties, err := e.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := enrichment.NewEnrichmentServiceClient(conn)

	resp, err := client.GetEnrichments(callProperties.Ctx, &enrichment.GetEnrichmentsRequest{}, callProperties.CallOptions...)
	if err != nil {
		return nil, err
	}

	result := make([]*enrichment.Enrichment, 0)
	for _, enrichment := range resp.GetEnrichments() {
		if customEnrichment := enrichment.GetEnrichmentType().GetCustomEnrichment(); customEnrichment != nil && customEnrichment.GetId().GetValue() == customEnrichmentId {
			result = append(result, enrichment)
		}
	}

	return result, nil
}

func (e EnrichmentsClient) DeleteEnrichments(ctx context.Context, req *enrichment.RemoveEnrichmentsRequest) error {
	callProperties, err := e.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return err
	}

	conn := callProperties.Connection
	defer conn.Close()

	client := enrichment.NewEnrichmentServiceClient(conn)

	_, err = client.RemoveEnrichments(callProperties.Ctx, req, callProperties.CallOptions...)
	return err
}

//func (e EnrichmentsClient) DeleteEnrichmentsByType(ctx context.Context, enrichmentType string) error {
//	enrichmentsToDelete, err := e.GetEnrichmentsByType(ctx, enrichmentType)
//	if err != nil {
//		return err
//	}
//
//	callProperties, err := e.callPropertiesCreator.GetCallProperties(ctx)
//	if err != nil {
//		return err
//	}
//
//	conn := callProperties.Connection
//	defer conn.Close()
//
//	client := enrichment.NewEnrichmentServiceClient(conn)
//
//	enrichmentIds := make([]*wrapperspb.UInt32Value, 0, len(enrichmentsToDelete))
//	for _, enrichment := range enrichmentsToDelete {
//		enrichmentIds = append(enrichmentIds, wrapperspb.UInt32(enrichment.GetId()))
//	}
//
//	req := &enrichment.RemoveEnrichmentsRequest{
//		EnrichmentIds: enrichmentIds,
//	}
//
//	_, err = client.RemoveEnrichments(callProperties.Ctx, req, callProperties.CallOptions...)
//	return err
//}

func NewEnrichmentClient(c *CallPropertiesCreator) *EnrichmentsClient {
	return &EnrichmentsClient{callPropertiesCreator: c}
}
