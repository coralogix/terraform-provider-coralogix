package clientset

import (
	"context"

	"google.golang.org/protobuf/types/known/emptypb"
	metricsConfiguration "terraform-provider-coralogix/coralogix/clientset/grpc/metrics-configurator"
)

type MetricsConfigurationClient struct {
	callPropertiesCreator *CallPropertiesCreator
}

func (c MetricsConfigurationClient) UpdateMetricsConfiguration(ctx context.Context, req *metricsConfiguration.TenantConfigV2) (*emptypb.Empty, error) {
	callProperties, err := c.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := metricsConfiguration.NewMetricsConfiguratorPublicServiceClient(conn)

	return client.ConfigureTenant(callProperties.Ctx, req, callProperties.CallOptions...)
}

func (c MetricsConfigurationClient) GetMetricsConfiguration(ctx context.Context) (*metricsConfiguration.GetTenantConfigResponseV2, error) {
	callProperties, err := c.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := metricsConfiguration.NewMetricsConfiguratorPublicServiceClient(conn)

	return client.GetTenantConfig(callProperties.Ctx, &emptypb.Empty{}, callProperties.CallOptions...)
}

func NewMetricsConfiguration(c *CallPropertiesCreator) *MetricsConfigurationClient {
	return &MetricsConfigurationClient{callPropertiesCreator: c}
}
