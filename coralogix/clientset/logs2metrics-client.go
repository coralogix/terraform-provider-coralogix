package clientset

import (
	"context"

	logs2metrics "terraform-provider-coralogix/coralogix/clientset/grpc/logs2metrics/v2"

	"google.golang.org/protobuf/types/known/emptypb"
)

type Logs2MetricsClient struct {
	callPropertiesCreator *CallPropertiesCreator
}

func (l Logs2MetricsClient) CreateLogs2Metric(ctx context.Context, req *logs2metrics.CreateL2MRequest) (*logs2metrics.L2M, error) {
	callProperties, err := l.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := logs2metrics.NewLogs2MetricServiceClient(conn)

	return client.CreateL2M(callProperties.Ctx, req, callProperties.CallOptions...)
}

func (l Logs2MetricsClient) GetLogs2Metric(ctx context.Context, req *logs2metrics.GetL2MRequest) (*logs2metrics.L2M, error) {
	callProperties, err := l.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := logs2metrics.NewLogs2MetricServiceClient(conn)

	return client.GetL2M(callProperties.Ctx, req, callProperties.CallOptions...)
}

func (l Logs2MetricsClient) UpdateLogs2Metric(ctx context.Context, req *logs2metrics.ReplaceL2MRequest) (*logs2metrics.L2M, error) {
	callProperties, err := l.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()

	client := logs2metrics.NewLogs2MetricServiceClient(conn)

	return client.ReplaceL2M(callProperties.Ctx, req, callProperties.CallOptions...)
}

func (l Logs2MetricsClient) DeleteLogs2Metric(ctx context.Context, req *logs2metrics.DeleteL2MRequest) (*emptypb.Empty, error) {
	callProperties, err := l.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()

	client := logs2metrics.NewLogs2MetricServiceClient(conn)

	return client.DeleteL2M(callProperties.Ctx, req, callProperties.CallOptions...)
}

func NewLogs2MetricsClient(c *CallPropertiesCreator) *Logs2MetricsClient {
	return &Logs2MetricsClient{callPropertiesCreator: c}
}
