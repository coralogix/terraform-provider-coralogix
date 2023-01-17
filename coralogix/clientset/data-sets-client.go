package clientset

import (
	"context"

	"terraform-provider-coralogix/coralogix/clientset/grpc/enrichment/v1"
)

type DataSetClient struct {
	callPropertiesCreator *CallPropertiesCreator
}

func (d DataSetClient) CreatDataSet(ctx context.Context, req *__.CreateCustomEnrichmentRequest) (*__.CreateCustomEnrichmentResponse, error) {
	callProperties, err := d.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()

	client := __.NewCustomEnrichmentServiceClient(conn)

	return client.CreateCustomEnrichment(callProperties.Ctx, req, callProperties.CallOptions...)
}

func (d DataSetClient) GetDataSet(ctx context.Context, req *__.GetCustomEnrichmentRequest) (*__.GetCustomEnrichmentResponse, error) {
	callProperties, err := d.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()

	client := __.NewCustomEnrichmentServiceClient(conn)

	return client.GetCustomEnrichment(callProperties.Ctx, req, callProperties.CallOptions...)
}

func (d DataSetClient) UpdateDataSet(ctx context.Context, req *__.UpdateCustomEnrichmentRequest) (*__.UpdateCustomEnrichmentResponse, error) {
	callProperties, err := d.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()

	client := __.NewCustomEnrichmentServiceClient(conn)

	return client.UpdateCustomEnrichment(callProperties.Ctx, req, callProperties.CallOptions...)
}

func (d DataSetClient) DeleteDataSet(ctx context.Context, req *__.DeleteCustomEnrichmentRequest) (*__.DeleteCustomEnrichmentResponse, error) {
	callProperties, err := d.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()

	client := __.NewCustomEnrichmentServiceClient(conn)

	return client.DeleteCustomEnrichment(callProperties.Ctx, req, callProperties.CallOptions...)
}

func NewDataSetClient(c *CallPropertiesCreator) *DataSetClient {
	return &DataSetClient{callPropertiesCreator: c}
}
