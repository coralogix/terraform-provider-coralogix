package clientset

import (
	"context"

	e2m "github.com/coralogix/coralogix-sdk-demo/events2metrics/v2"
)

type Events2MetricsClient struct {
	callPropertiesCreator *CallPropertiesCreator
}

func (e Events2MetricsClient) CreateEvents2Metric(ctx context.Context, req *e2m.CreateE2MRequest) (*e2m.CreateE2MResponse, error) {
	callProperties, err := e.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := e2m.NewEvents2MetricServiceClient(conn)

	return client.CreateE2M(callProperties.Ctx, req, callProperties.CallOptions...)
}

func (e Events2MetricsClient) GetEvents2Metric(ctx context.Context, req *e2m.GetE2MRequest) (*e2m.GetE2MResponse, error) {
	callProperties, err := e.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := e2m.NewEvents2MetricServiceClient(conn)

	return client.GetE2M(callProperties.Ctx, req, callProperties.CallOptions...)
}

func (e Events2MetricsClient) UpdateEvents2Metric(ctx context.Context, req *e2m.ReplaceE2MRequest) (*e2m.ReplaceE2MResponse, error) {
	callProperties, err := e.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := e2m.NewEvents2MetricServiceClient(conn)

	return client.ReplaceE2M(callProperties.Ctx, req, callProperties.CallOptions...)
}

func (e Events2MetricsClient) DeleteEvents2Metric(ctx context.Context, req *e2m.DeleteE2MRequest) (*e2m.DeleteE2MResponse, error) {
	callProperties, err := e.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := e2m.NewEvents2MetricServiceClient(conn)

	return client.DeleteE2M(callProperties.Ctx, req, callProperties.CallOptions...)
}

func NewEvents2MetricsClient(c *CallPropertiesCreator) *Events2MetricsClient {
	return &Events2MetricsClient{callPropertiesCreator: c}
}
