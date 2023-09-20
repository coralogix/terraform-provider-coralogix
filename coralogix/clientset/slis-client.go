package clientset

import (
	"context"

	slis "terraform-provider-coralogix/coralogix/clientset/grpc/sli"
)

type SLIClient struct {
	callPropertiesCreator *CallPropertiesCreator
}

func (c SLIClient) CreateSLI(ctx context.Context, req *slis.CreateSliRequest) (*slis.CreateSliResponse, error) {
	callProperties, err := c.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := slis.NewSliServiceClient(conn)

	return client.CreateSli(callProperties.Ctx, req, callProperties.CallOptions...)
}

func (c SLIClient) GetSLIs(ctx context.Context, req *slis.GetSlisRequest) (*slis.GetSlisResponse, error) {
	callProperties, err := c.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := slis.NewSliServiceClient(conn)

	return client.GetSlis(callProperties.Ctx, req, callProperties.CallOptions...)
}

func (c SLIClient) UpdateSLI(ctx context.Context, req *slis.UpdateSliRequest) (*slis.UpdateSliResponse, error) {
	callProperties, err := c.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := slis.NewSliServiceClient(conn)

	return client.UpdateSli(callProperties.Ctx, req, callProperties.CallOptions...)
}

func (c SLIClient) DeleteSLI(ctx context.Context, req *slis.DeleteSliRequest) (*slis.DeleteSliResponse, error) {
	callProperties, err := c.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := slis.NewSliServiceClient(conn)

	return client.DeleteSli(callProperties.Ctx, req, callProperties.CallOptions...)
}

func NewSLIsClient(c *CallPropertiesCreator) *SLIClient {
	return &SLIClient{callPropertiesCreator: c}
}
