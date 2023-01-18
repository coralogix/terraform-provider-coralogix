package clientset

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"terraform-provider-coralogix/coralogix/clientset/rest"

	gapi "github.com/grafana/grafana-api-golang-client"
)

type GrafanaDashboardClient struct {
	targetUrl string
	client    *rest.Client
}

func (g GrafanaDashboardClient) CreateGrafanaDashboard(ctx context.Context, dashboard gapi.Dashboard) (*gapi.DashboardSaveResponse, error) {
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

func (g GrafanaDashboardClient) GetGrafanaDashboard(ctx context.Context, uid string) (*gapi.Dashboard, error) {
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

func (g GrafanaDashboardClient) UpdateGrafanaDashboard(ctx context.Context, dashboard gapi.Dashboard) (*gapi.DashboardSaveResponse, error) {
	dashboard.Overwrite = true
	return g.CreateGrafanaDashboard(ctx, dashboard)
}

func (g GrafanaDashboardClient) DeleteGrafanaDashboard(ctx context.Context, uid string) error {
	_, err := g.client.Delete(ctx, fmt.Sprintf("/grafana/api/dashboards/uid/%s", uid))
	return err

}

func (g GrafanaDashboardClient) GetTargetURL() string {
	return g.targetUrl

}

func NewGrafanaClient(c *CallPropertiesCreator) *GrafanaDashboardClient {
	targetUrl := "https://" + strings.Replace(c.targetUrl, "grpc", "http", 1)
	client := rest.NewRestClient(targetUrl, c.apiKey)
	return &GrafanaDashboardClient{client: client, targetUrl: targetUrl}
}
