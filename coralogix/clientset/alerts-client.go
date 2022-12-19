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

func (a AlertsClient) GetAlert(ctx context.Context, req *alertsv1.GetAlertByUniqueIdRequest) (*alertsv1.GetAlertByUniqueIdResponse, error) {
	callProperties, err := a.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := alertsv1.NewAlertServiceClient(conn)

	return client.GetAlertByUniqueId(callProperties.Ctx, req, callProperties.CallOptions...)
}

func (a AlertsClient) UpdateAlert(ctx context.Context, req *alertsv1.UpdateAlertByUniqueIdRequest) (*alertsv1.UpdateAlertByUniqueIdResponse, error) {
	callProperties, err := a.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := alertsv1.NewAlertServiceClient(conn)

	return client.UpdateAlertByUniqueId(callProperties.Ctx, req, callProperties.CallOptions...)
}

func (a AlertsClient) DeleteAlert(ctx context.Context, req *alertsv1.DeleteAlertByUniqueIdRequest) (*alertsv1.DeleteAlertByUniqueIdResponse, error) {
	callProperties, err := a.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := alertsv1.NewAlertServiceClient(conn)

	return client.DeleteAlertByUniqueId(callProperties.Ctx, req, callProperties.CallOptions...)
}

func NewAlertsClient(c *CallPropertiesCreator) *AlertsClient {
	return &AlertsClient{callPropertiesCreator: c}
}
