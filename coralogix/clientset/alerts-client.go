package clientset

import (
	"context"

	alerts "terraform-provider-coralogix/coralogix/clientset/grpc/alerts/v2"
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
	client := alerts.NewAlertServiceClient(conn)

	return client.CreateAlert(callProperties.Ctx, req, callProperties.CallOptions...)
}

func (a AlertsClient) GetAlert(ctx context.Context, req *alerts.GetAlertByUniqueIdRequest) (*alerts.GetAlertByUniqueIdResponse, error) {
	callProperties, err := a.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := alerts.NewAlertServiceClient(conn)

	return client.GetAlertByUniqueId(callProperties.Ctx, req, callProperties.CallOptions...)
}

func (a AlertsClient) UpdateAlert(ctx context.Context, req *alerts.UpdateAlertByUniqueIdRequest) (*alerts.UpdateAlertByUniqueIdResponse, error) {
	callProperties, err := a.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := alerts.NewAlertServiceClient(conn)

	return client.UpdateAlertByUniqueId(callProperties.Ctx, req, callProperties.CallOptions...)
}

func (a AlertsClient) DeleteAlert(ctx context.Context, req *alerts.DeleteAlertByUniqueIdRequest) (*alerts.DeleteAlertByUniqueIdResponse, error) {
	callProperties, err := a.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := alerts.NewAlertServiceClient(conn)

	return client.DeleteAlertByUniqueId(callProperties.Ctx, req, callProperties.CallOptions...)
}

func NewAlertsClient(c *CallPropertiesCreator) *AlertsClient {
	return &AlertsClient{callPropertiesCreator: c}
}
