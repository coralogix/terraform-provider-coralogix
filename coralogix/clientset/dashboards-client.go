package clientset

import (
	"context"

	dashboard "terraform-provider-coralogix/coralogix/clientset/grpc/com/coralogix/coralogix-dashboards"
)

type DashboardsClient struct {
	callPropertiesCreator *CallPropertiesCreator
}

func (d DashboardsClient) CreateDashboard(ctx context.Context, req *dashboard.CreateDashboardRequest) (*dashboard.CreateDashboardResponse, error) {
	callProperties, err := d.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := dashboard.NewDashboardsServiceClient(conn)

	return client.CreateDashboard(callProperties.Ctx, req, callProperties.CallOptions...)
}

func (d DashboardsClient) GetDashboard(ctx context.Context, req *dashboard.GetDashboardRequest) (*dashboard.GetDashboardResponse, error) {
	callProperties, err := d.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := dashboard.NewDashboardsServiceClient(conn)

	return client.GetDashboard(callProperties.Ctx, req, callProperties.CallOptions...)
}

func (d DashboardsClient) UpdateDashboard(ctx context.Context, req *dashboard.ReplaceDashboardRequest) (*dashboard.ReplaceDashboardResponse, error) {
	callProperties, err := d.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := dashboard.NewDashboardsServiceClient(conn)

	return client.ReplaceDashboard(callProperties.Ctx, req, callProperties.CallOptions...)
}

func (d DashboardsClient) DeleteDashboard(ctx context.Context, req *dashboard.DeleteDashboardRequest) (*dashboard.DeleteDashboardResponse, error) {
	callProperties, err := d.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := dashboard.NewDashboardsServiceClient(conn)

	return client.DeleteDashboard(callProperties.Ctx, req, callProperties.CallOptions...)
}

func NewDashboardsClient(c *CallPropertiesCreator) *DashboardsClient {
	return &DashboardsClient{callPropertiesCreator: c}
}
