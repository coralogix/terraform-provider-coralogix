package clientset

import (
	"context"

	slos "terraform-provider-coralogix/coralogix/clientset/grpc/slo"
)

type SLOsClient struct {
	callPropertiesCreator *CallPropertiesCreator
}

func (c SLOsClient) CreateSLO(ctx context.Context, req *slos.CreateServiceSloRequest) (*slos.CreateServiceSloResponse, error) {
	callProperties, err := c.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := slos.NewServiceSloServiceClient(conn)

	return client.CreateServiceSlo(callProperties.Ctx, req, callProperties.CallOptions...)
}

func (c SLOsClient) GetSLO(ctx context.Context, req *slos.GetServiceSloRequest) (*slos.GetServiceSloResponse, error) {
	callProperties, err := c.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := slos.NewServiceSloServiceClient(conn)

	return client.GetServiceSlo(callProperties.Ctx, req, callProperties.CallOptions...)
}

func (c SLOsClient) UpdateSLO(ctx context.Context, req *slos.ReplaceServiceSloRequest) (*slos.ReplaceServiceSloResponse, error) {
	callProperties, err := c.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := slos.NewServiceSloServiceClient(conn)

	return client.ReplaceServiceSlo(callProperties.Ctx, req, callProperties.CallOptions...)
}

func (c SLOsClient) DeleteSLO(ctx context.Context, req *slos.DeleteServiceSloRequest) (*slos.DeleteServiceSloResponse, error) {
	callProperties, err := c.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := slos.NewServiceSloServiceClient(conn)

	return client.DeleteServiceSlo(callProperties.Ctx, req, callProperties.CallOptions...)
}

func NewSLOsClient(c *CallPropertiesCreator) *SLOsClient {
	return &SLOsClient{callPropertiesCreator: c}
}
