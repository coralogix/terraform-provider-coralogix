package clientset

import (
	"context"
	"fmt"

	"google.golang.org/protobuf/types/known/wrapperspb"
	enrichmentv1 "terraform-provider-coralogix/coralogix/clientset/grpc/com/coralogix/enrichment/v1"
)

type EnrichmentsClient struct {
	callPropertiesCreator *CallPropertiesCreator
}

func (e EnrichmentsClient) CreateEnrichment(ctx context.Context, req *enrichmentv1.EnrichmentRequestModel) (*enrichmentv1.Enrichment, error) {
	callProperties, err := e.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := enrichmentv1.NewEnrichmentServiceClient(conn)

	addReq := &enrichmentv1.AddEnrichmentsRequest{RequestEnrichments: []*enrichmentv1.EnrichmentRequestModel{req}}
	resp, err := client.AddEnrichments(callProperties.Ctx, addReq, callProperties.CallOptions...)
	if err != nil {
		return nil, err
	}

	enrichments := resp.GetEnrichments()
	return enrichments[len(enrichments)-1], nil
}

func (e EnrichmentsClient) GetEnrichment(ctx context.Context, id uint32) (*enrichmentv1.Enrichment, error) {
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

	enrichment := getEnrichment(resp.GetEnrichments(), id)
	if enrichment == nil {
		return nil, fmt.Errorf("couldn't find enrichment with id %d", id)
	}

	return enrichment, nil
}

func getEnrichment(enrichments []*enrichmentv1.Enrichment, id uint32) *enrichmentv1.Enrichment {
	for _, e := range enrichments {
		if e.GetId() == id {
			return e
		}
	}
	return nil
}

func (e EnrichmentsClient) UpdateEnrichment(ctx context.Context, id uint32, req *enrichmentv1.EnrichmentRequestModel) (*enrichmentv1.Enrichment, error) {
	err := e.DeleteEnrichment(ctx, id)
	if err != nil {
		return nil, err
	}
	return e.CreateEnrichment(ctx, req)
}

func (e EnrichmentsClient) DeleteEnrichment(ctx context.Context, id uint32) error {
	callProperties, err := e.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return err
	}

	conn := callProperties.Connection
	defer conn.Close()

	client := enrichmentv1.NewEnrichmentServiceClient(conn)

	req := &enrichmentv1.RemoveEnrichmentsRequest{
		EnrichmentIds: []*wrapperspb.UInt32Value{wrapperspb.UInt32(id)},
	}

	_, err = client.RemoveEnrichments(callProperties.Ctx, req, callProperties.CallOptions...)
	return err
}

func NewEnrichmentClient(c *CallPropertiesCreator) *EnrichmentsClient {
	return &EnrichmentsClient{callPropertiesCreator: c}
}
