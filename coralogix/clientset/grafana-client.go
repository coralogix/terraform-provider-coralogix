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
	"encoding/json"
	"fmt"
	"strings"

	"terraform-provider-coralogix/coralogix/clientset/rest"

	gapi "github.com/grafana/grafana-api-golang-client"
)

type GrafanaClient struct {
	targetUrl string
	client    *rest.Client
}

func (g GrafanaClient) CreateGrafanaDashboard(ctx context.Context, dashboard gapi.Dashboard) (*gapi.DashboardSaveResponse, error) {
	body, err := json.Marshal(dashboard)
	if err != nil {
		return nil, err
	}

	bodyResp, err := g.client.Post(ctx, "/grafana/api/dashboards/db", "application/json", string(body))
	if err != nil {
		return nil, err
	}

	var dashboardResp gapi.DashboardSaveResponse
	err = json.Unmarshal([]byte(bodyResp), &dashboardResp)
	if err != nil {
		return nil, err
	}

	return &dashboardResp, nil
}

func (g GrafanaClient) GetGrafanaDashboard(ctx context.Context, uid string) (*gapi.Dashboard, error) {
	bodyResp, err := g.client.Get(ctx, fmt.Sprintf("/grafana/api/dashboards/uid/%s", uid))
	if err != nil {
		return nil, err
	}

	var dashboardResp gapi.Dashboard
	err = json.Unmarshal([]byte(bodyResp), &dashboardResp)
	if err != nil {
		return nil, err
	}

	return &dashboardResp, nil
}

func (g GrafanaClient) UpdateGrafanaDashboard(ctx context.Context, dashboard gapi.Dashboard) (*gapi.DashboardSaveResponse, error) {
	dashboard.Overwrite = true
	return g.CreateGrafanaDashboard(ctx, dashboard)
}

func (g GrafanaClient) DeleteGrafanaDashboard(ctx context.Context, uid string) error {
	_, err := g.client.Delete(ctx, fmt.Sprintf("/grafana/api/dashboards/uid/%s", uid))
	return err

}

func (g GrafanaClient) CreateGrafanaFolder(ctx context.Context, folder gapi.Folder) (*gapi.Folder, error) {
	body, err := json.Marshal(folder)
	if err != nil {
		return nil, err
	}

	bodyResp, err := g.client.Post(ctx, "/grafana/api/folders", "application/json", string(body))
	if err != nil {
		return nil, err
	}

	var folderResp gapi.Folder
	err = json.Unmarshal([]byte(bodyResp), &folderResp)
	if err != nil {
		return nil, err
	}

	return &folderResp, nil
}

func (g GrafanaClient) GetGrafanaFolder(ctx context.Context, uid string) (*gapi.Folder, error) {
	bodyResp, err := g.client.Get(ctx, fmt.Sprintf("/grafana/api/folders/id/%s", uid))
	if err != nil {
		return nil, err
	}

	var folderResp gapi.Folder
	err = json.Unmarshal([]byte(bodyResp), &folderResp)
	if err != nil {
		return nil, err
	}

	return &folderResp, nil
}

func (g GrafanaClient) UpdateGrafanaFolder(ctx context.Context, folder gapi.FolderPayload) (*gapi.Folder, error) {
	body, err := json.Marshal(folder)
	if err != nil {
		return nil, err
	}

	bodyResp, err := g.client.Put(ctx, fmt.Sprintf("/grafana/api/folders/%s", folder.UID), "application/json", string(body))
	if err != nil {
		return nil, err
	}

	var folderResp gapi.Folder
	err = json.Unmarshal([]byte(bodyResp), &folderResp)
	if err != nil {
		return nil, err
	}

	return &folderResp, nil
}

func (g GrafanaClient) DeleteGrafanaFolder(ctx context.Context, uid string) error {
	_, err := g.client.Delete(ctx, fmt.Sprintf("/grafana/api/folders/%s", uid))
	return err
}

func (g GrafanaClient) GetTargetURL() string {
	return g.targetUrl

}

func NewGrafanaClient(c *CallPropertiesCreator) *GrafanaClient {
	targetUrl := "https://" + strings.Replace(c.targetUrl, "grpc", "http", 1)
	client := rest.NewRestClient(targetUrl, c.apiKey)
	return &GrafanaClient{client: client, targetUrl: targetUrl}
}
