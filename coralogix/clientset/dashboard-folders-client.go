// Copyright 2024 Coralogix Ltd.
// 
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// 
//     https://www.apache.org/licenses/LICENSE-2.0
// 
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
