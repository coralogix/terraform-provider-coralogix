package clientset

import (
	"context"

	rulesv1 "terraform-provider-coralogix/coralogix/clientset/grpc/com/coralogix/rules/v1"
)

type RuleGroupsClient struct {
	callPropertiesCreator *CallPropertiesCreator
}

func (r RuleGroupsClient) CreateRuleGroup(ctx context.Context, req *rulesv1.CreateRuleGroupRequest) (*rulesv1.CreateRuleGroupResponse, error) {
	callProperties, err := r.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := rulesv1.NewRuleGroupsServiceClient(conn)

	return client.CreateRuleGroup(callProperties.Ctx, req, callProperties.CallOptions...)
}

func (r RuleGroupsClient) GetRuleGroup(ctx context.Context, req *rulesv1.GetRuleGroupRequest) (*rulesv1.GetRuleGroupResponse, error) {
	callProperties, err := r.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := rulesv1.NewRuleGroupsServiceClient(conn)

	return client.GetRuleGroup(callProperties.Ctx, req, callProperties.CallOptions...)
}

func (r RuleGroupsClient) UpdateRuleGroup(ctx context.Context, req *rulesv1.UpdateRuleGroupRequest) (*rulesv1.UpdateRuleGroupResponse, error) {
	callProperties, err := r.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()

	client := rulesv1.NewRuleGroupsServiceClient(conn)

	return client.UpdateRuleGroup(callProperties.Ctx, req, callProperties.CallOptions...)
}

func (r RuleGroupsClient) DeleteRuleGroup(ctx context.Context, req *rulesv1.DeleteRuleGroupRequest) (*rulesv1.DeleteRuleGroupResponse, error) {
	callProperties, err := r.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()

	client := rulesv1.NewRuleGroupsServiceClient(conn)

	return client.DeleteRuleGroup(callProperties.Ctx, req, callProperties.CallOptions...)
}

func NewRuleGroupsClient(c *CallPropertiesCreator) *RuleGroupsClient {
	return &RuleGroupsClient{callPropertiesCreator: c}
}
