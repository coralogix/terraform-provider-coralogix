package clientset

import (
	"context"

	alertsv1 "terraform-provider-coralogix/coralogix/clientset/grpc/com/coralogix/alerts/v1"
)

type AlertsClient struct {
	callPropertiesCreator *CallPropertiesCreator
}

func (a AlertsClient) CreateAlert(ctx context.Context, req *alertsv1.CreateAlertRequest) (*alertsv1.CreateAlertResponse, error) {
	callProperties, err := a.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := alertsv1.NewAlertServiceClient(conn)

	return client.CreateAlert(callProperties.Ctx, req, callProperties.CallOptions...)
}

func (a AlertsClient) GetAlert(ctx context.Context, req *alertsv1.GetAlertRequest) (*alertsv1.GetAlertResponse, error) {
	callProperties, err := a.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := alertsv1.NewAlertServiceClient(conn)

	return client.GetAlert(callProperties.Ctx, req, callProperties.CallOptions...)
}

func (a AlertsClient) UpdateAlert(ctx context.Context, req *alertsv1.UpdateAlertRequest) (*alertsv1.UpdateAlertResponse, error) {
	callProperties, err := a.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := alertsv1.NewAlertServiceClient(conn)

	return client.UpdateAlert(callProperties.Ctx, req, callProperties.CallOptions...)
}

func (a AlertsClient) DeleteAlert(ctx context.Context, req *alertsv1.DeleteAlertRequest) (*alertsv1.DeleteAlertResponse, error) {
	callProperties, err := a.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := alertsv1.NewAlertServiceClient(conn)

	return client.DeleteAlert(callProperties.Ctx, req, callProperties.CallOptions...)
}

func NewAlertsClient(c *CallPropertiesCreator) *AlertsClient {
	return &AlertsClient{callPropertiesCreator: c}
}
