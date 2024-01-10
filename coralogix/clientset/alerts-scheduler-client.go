package clientset

import (
	"context"

	alertsSchedulers "terraform-provider-coralogix/coralogix/clientset/grpc/alerts-scheduler"
)

type AlertsSchedulersClient struct {
	callPropertiesCreator *CallPropertiesCreator
}

func (c AlertsSchedulersClient) CreateAlertScheduler(ctx context.Context, req *alertsSchedulers.CreateAlertSchedulerRuleRequest) (*alertsSchedulers.CreateAlertSchedulerRuleResponse, error) {
	callProperties, err := c.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := alertsSchedulers.NewAlertSchedulerRuleServiceClient(conn)

	return client.CreateAlertSchedulerRule(callProperties.Ctx, req, callProperties.CallOptions...)
}

func (c AlertsSchedulersClient) GetAlertScheduler(ctx context.Context, req *alertsSchedulers.GetAlertSchedulerRuleRequest) (*alertsSchedulers.GetAlertSchedulerRuleResponse, error) {
	callProperties, err := c.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := alertsSchedulers.NewAlertSchedulerRuleServiceClient(conn)

	return client.GetAlertSchedulerRule(callProperties.Ctx, req, callProperties.CallOptions...)
}

func (c AlertsSchedulersClient) UpdateAlertScheduler(ctx context.Context, req *alertsSchedulers.UpdateAlertSchedulerRuleRequest) (*alertsSchedulers.UpdateAlertSchedulerRuleResponse, error) {
	callProperties, err := c.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := alertsSchedulers.NewAlertSchedulerRuleServiceClient(conn)

	return client.UpdateAlertSchedulerRule(callProperties.Ctx, req, callProperties.CallOptions...)
}

func (c AlertsSchedulersClient) DeleteAlertScheduler(ctx context.Context, req *alertsSchedulers.DeleteAlertSchedulerRuleRequest) (*alertsSchedulers.DeleteAlertSchedulerRuleResponse, error) {
	callProperties, err := c.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := alertsSchedulers.NewAlertSchedulerRuleServiceClient(conn)

	return client.DeleteAlertSchedulerRule(callProperties.Ctx, req, callProperties.CallOptions...)
}

func NewAlertsSchedulersClient(c *CallPropertiesCreator) *AlertsSchedulersClient {
	return &AlertsSchedulersClient{callPropertiesCreator: c}
}
