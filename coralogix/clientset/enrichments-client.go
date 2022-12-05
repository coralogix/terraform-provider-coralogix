package clientset

import (
	"context"

	"google.golang.org/protobuf/types/known/wrapperspb"
	enrichmentv1 "terraform-provider-coralogix/coralogix/clientset/grpc/com/coralogix/enrichment/v1"
)

type EnrichmentsClient struct {
	callPropertiesCreator *CallPropertiesCreator
}

func (e EnrichmentsClient) CreateEnrichments(ctx context.Context, req []*enrichmentv1.EnrichmentRequestModel) ([]*enrichmentv1.Enrichment, error) {
	callProperties, err := e.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := enrichmentv1.NewEnrichmentServiceClient(conn)

	addReq := &enrichmentv1.AddEnrichmentsRequest{RequestEnrichments: req}
	resp, err := client.AddEnrichments(callProperties.Ctx, addReq, callProperties.CallOptions...)
	if err != nil {
		return nil, err
	}

	enrichments := resp.GetEnrichments()
	from := len(enrichments) - len(req)
	to := len(enrichments)
	return enrichments[from:to], nil
}

func (e EnrichmentsClient) GetEnrichmentsByType(ctx context.Context, enrichmentType string) ([]*enrichmentv1.Enrichment, error) {
	callProperties, err := e.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := enrichmentv1.NewEnrichmentServiceClient(conn)

	resp, err := client.GetEnrichments(callProperties.Ctx, &enrichmentv1.GetEnrichmentsRequest{}, callProperties.CallOptions...)
	if err != nil {
		return nil, err
	}

	result := make([]*enrichmentv1.Enrichment, 0)
	for _, enrichment := range resp.GetEnrichments() {
		if enrichment.GetEnrichmentType().String() == enrichmentType+":{}" {
			result = append(result, enrichment)
		}
	}

	return result, nil
}

func (e EnrichmentsClient) GetCustomEnrichments(ctx context.Context, customEnrichmentId uint32) ([]*enrichmentv1.Enrichment, error) {
	callProperties, err := e.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := enrichmentv1.NewEnrichmentServiceClient(conn)

	resp, err := client.GetEnrichments(callProperties.Ctx, &enrichmentv1.GetEnrichmentsRequest{}, callProperties.CallOptions...)
	if err != nil {
		return nil, err
	}

	result := make([]*enrichmentv1.Enrichment, 0)
	for _, enrichment := range resp.GetEnrichments() {
		if customEnrichment := enrichment.GetEnrichmentType().GetCustomEnrichment(); customEnrichment != nil && customEnrichment.GetId().GetValue() == customEnrichmentId {
			result = append(result, enrichment)
		}
	}

	return result, nil
}

func (e EnrichmentsClient) UpdateEnrichments(ctx context.Context, ids []uint32, req []*enrichmentv1.EnrichmentRequestModel) ([]*enrichmentv1.Enrichment, error) {
	err := e.DeleteEnrichments(ctx, ids)
	if err != nil {
		return nil, err
	}
	return e.CreateEnrichments(ctx, req)
}

func (e EnrichmentsClient) DeleteEnrichments(ctx context.Context, ids []uint32) error {
	callProperties, err := e.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return err
	}

	conn := callProperties.Connection
	defer conn.Close()

	client := enrichmentv1.NewEnrichmentServiceClient(conn)

	enrichmentIds := make([]*wrapperspb.UInt32Value, 0, len(ids))
	for _, id := range ids {
		enrichmentIds = append(enrichmentIds, wrapperspb.UInt32(id))
	}

	req := &enrichmentv1.RemoveEnrichmentsRequest{
		EnrichmentIds: enrichmentIds,
	}

	_, err = client.RemoveEnrichments(callProperties.Ctx, req, callProperties.CallOptions...)
	return err
}

func (e EnrichmentsClient) DeleteEnrichmentsByType(ctx context.Context, enrichmentType string) error {
	enrichmentsToDelete, err := e.GetEnrichmentsByType(ctx, enrichmentType)
	if err != nil {
		return err
	}

	callProperties, err := e.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return err
	}

	conn := callProperties.Connection
	defer conn.Close()

	client := enrichmentv1.NewEnrichmentServiceClient(conn)

	enrichmentIds := make([]*wrapperspb.UInt32Value, 0, len(enrichmentsToDelete))
	for _, enrichment := range enrichmentsToDelete {
		enrichmentIds = append(enrichmentIds, wrapperspb.UInt32(enrichment.GetId()))
	}

	req := &enrichmentv1.RemoveEnrichmentsRequest{
		EnrichmentIds: enrichmentIds,
	}

	_, err = client.RemoveEnrichments(callProperties.Ctx, req, callProperties.CallOptions...)
	return err
}

func NewEnrichmentClient(c *CallPropertiesCreator) *EnrichmentsClient {
	return &EnrichmentsClient{callPropertiesCreator: c}
}
