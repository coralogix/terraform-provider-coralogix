package clientset

import (
	"context"

	dashboards "terraform-provider-coralogix/coralogix/clientset/grpc/dashboards"
)

type DashboardsFoldersClient struct {
	callPropertiesCreator *CallPropertiesCreator
}

func (c DashboardsFoldersClient) CreateDashboardsFolder(ctx context.Context, req *dashboards.CreateDashboardFolderRequest) (*dashboards.CreateDashboardFolderResponse, error) {
	callProperties, err := c.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := dashboards.NewDashboardFoldersServiceClient(conn)

	return client.CreateDashboardFolder(callProperties.Ctx, req, callProperties.CallOptions...)
}

func (c DashboardsFoldersClient) GetDashboardsFolders(ctx context.Context, req *dashboards.ListDashboardFoldersRequest) (*dashboards.ListDashboardFoldersResponse, error) {
	callProperties, err := c.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := dashboards.NewDashboardFoldersServiceClient(conn)

	return client.ListDashboardFolders(callProperties.Ctx, req, callProperties.CallOptions...)
}

func (c DashboardsFoldersClient) UpdateDashboardsFolder(ctx context.Context, req *dashboards.ReplaceDashboardFolderRequest) (*dashboards.ReplaceDashboardFolderResponse, error) {
	callProperties, err := c.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := dashboards.NewDashboardFoldersServiceClient(conn)

	return client.ReplaceDashboardFolder(callProperties.Ctx, req, callProperties.CallOptions...)
}

func (c DashboardsFoldersClient) DeleteDashboardsFolder(ctx context.Context, req *dashboards.DeleteDashboardFolderRequest) (*dashboards.DeleteDashboardFolderResponse, error) {
	callProperties, err := c.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := dashboards.NewDashboardFoldersServiceClient(conn)

	return client.DeleteDashboardFolder(callProperties.Ctx, req, callProperties.CallOptions...)
}

func NewDashboardsFoldersClient(c *CallPropertiesCreator) *DashboardsFoldersClient {
	return &DashboardsFoldersClient{callPropertiesCreator: c}
}
