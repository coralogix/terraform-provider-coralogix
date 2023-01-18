package clientset

import (
	"context"

	enrichment "terraform-provider-coralogix/coralogix/clientset/grpc/enrichment/v1"
)

type DataSetClient struct {
	callPropertiesCreator *CallPropertiesCreator
}

func (d DataSetClient) CreatDataSet(ctx context.Context, req *enrichment.CreateCustomEnrichmentRequest) (*enrichment.CreateCustomEnrichmentResponse, error) {
	callProperties, err := d.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()

	client := enrichment.NewCustomEnrichmentServiceClient(conn)

	return client.CreateCustomEnrichment(callProperties.Ctx, req, callProperties.CallOptions...)
}

func (d DataSetClient) GetDataSet(ctx context.Context, req *enrichment.GetCustomEnrichmentRequest) (*enrichment.GetCustomEnrichmentResponse, error) {
	callProperties, err := d.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()

	client := enrichment.NewCustomEnrichmentServiceClient(conn)

	return client.GetCustomEnrichment(callProperties.Ctx, req, callProperties.CallOptions...)
}

func (d DataSetClient) UpdateDataSet(ctx context.Context, req *enrichment.UpdateCustomEnrichmentRequest) (*enrichment.UpdateCustomEnrichmentResponse, error) {
	callProperties, err := d.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()

	client := enrichment.NewCustomEnrichmentServiceClient(conn)

	return client.UpdateCustomEnrichment(callProperties.Ctx, req, callProperties.CallOptions...)
}

func (d DataSetClient) DeleteDataSet(ctx context.Context, req *enrichment.DeleteCustomEnrichmentRequest) (*enrichment.DeleteCustomEnrichmentResponse, error) {
	callProperties, err := d.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()

	client := enrichment.NewCustomEnrichmentServiceClient(conn)

	return client.DeleteCustomEnrichment(callProperties.Ctx, req, callProperties.CallOptions...)
}

func NewDataSetClient(c *CallPropertiesCreator) *DataSetClient {
	return &DataSetClient{callPropertiesCreator: c}
}
