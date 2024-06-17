package clientset

import (
	"context"

	alerts "terraform-provider-coralogix/coralogix/clientset/grpc/alerts/v3"
)

type AlertsClient struct {
	callPropertiesCreator *CallPropertiesCreator
}

func (a AlertsClient) CreateAlert(ctx context.Context, req *alerts.CreateAlertRequest) (*alerts.CreateAlertResponse, error) {
	callProperties, err := a.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := alerts.NewAlertsServiceClient(conn)

	return client.CreateAlert(callProperties.Ctx, req, callProperties.CallOptions...)
}

func (a AlertsClient) GetAlert(ctx context.Context, req *alerts.GetAlertRequest) (*alerts.GetAlertResponse, error) {
	callProperties, err := a.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := alerts.NewAlertsServiceClient(conn)

	return client.GetAlert(callProperties.Ctx, req, callProperties.CallOptions...)
}

func (a AlertsClient) UpdateAlert(ctx context.Context, req *alerts.ReplaceAlertRequest) (*alerts.ReplaceAlertResponse, error) {
	callProperties, err := a.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := alerts.NewAlertsServiceClient(conn)

	return client.ReplaceAlert(callProperties.Ctx, req, callProperties.CallOptions...)
}

func (a AlertsClient) DeleteAlert(ctx context.Context, req *alerts.DeleteAlertRequest) (*alerts.DeleteAlertResponse, error) {
	callProperties, err := a.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := alerts.NewAlertsServiceClient(conn)

	return client.DeleteAlert(callProperties.Ctx, req, callProperties.CallOptions...)
}

func NewAlertsClient(c *CallPropertiesCreator) *AlertsClient {
	return &AlertsClient{callPropertiesCreator: c}
}
