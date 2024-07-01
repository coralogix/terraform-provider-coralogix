package clientset

import (
	"context"

	alerts "terraform-provider-coralogix/coralogix/clientset/grpc/alerts/v3"
)

type AlertsClient struct {
	callPropertiesCreator *CallPropertiesCreator
}

func (a AlertsClient) CreateAlert(ctx context.Context, req *alerts.CreateAlertDefRequest) (*alerts.CreateAlertDefResponse, error) {
	callProperties, err := a.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := alerts.NewAlertDefsServiceClient(conn)

	return client.CreateAlertDef(callProperties.Ctx, req, callProperties.CallOptions...)
}

func (a AlertsClient) GetAlert(ctx context.Context, req *alerts.GetAlertDefRequest) (*alerts.GetAlertDefResponse, error) {
	callProperties, err := a.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := alerts.NewAlertDefsServiceClient(conn)

	return client.GetAlertDef(callProperties.Ctx, req, callProperties.CallOptions...)
}

func (a AlertsClient) UpdateAlert(ctx context.Context, req *alerts.ReplaceAlertDefRequest) (*alerts.ReplaceAlertDefResponse, error) {
	callProperties, err := a.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := alerts.NewAlertDefsServiceClient(conn)

	return client.ReplaceAlertDef(callProperties.Ctx, req, callProperties.CallOptions...)
}

func (a AlertsClient) DeleteAlert(ctx context.Context, req *alerts.DeleteAlertDefRequest) (*alerts.DeleteAlertDefResponse, error) {
	callProperties, err := a.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := alerts.NewAlertDefsServiceClient(conn)

	return client.DeleteAlertDef(callProperties.Ctx, req, callProperties.CallOptions...)
}

func NewAlertsClient(c *CallPropertiesCreator) *AlertsClient {
	return &AlertsClient{callPropertiesCreator: c}
}
