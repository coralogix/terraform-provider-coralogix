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
