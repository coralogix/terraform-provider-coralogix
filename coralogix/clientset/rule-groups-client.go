package clientset

import (
	"context"

	rulesgroups "terraform-provider-coralogix/coralogix/clientset/grpc/rules-groups/v1"
)

type RuleGroupsClient struct {
	callPropertiesCreator *CallPropertiesCreator
}

func (r RuleGroupsClient) CreateRuleGroup(ctx context.Context, req *rulesgroups.CreateRuleGroupRequest) (*rulesgroups.CreateRuleGroupResponse, error) {
	callProperties, err := r.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := rulesgroups.NewRuleGroupsServiceClient(conn)

	return client.CreateRuleGroup(callProperties.Ctx, req, callProperties.CallOptions...)
}

func (r RuleGroupsClient) GetRuleGroup(ctx context.Context, req *rulesgroups.GetRuleGroupRequest) (*rulesgroups.GetRuleGroupResponse, error) {
	callProperties, err := r.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := rulesgroups.NewRuleGroupsServiceClient(conn)

	return client.GetRuleGroup(callProperties.Ctx, req, callProperties.CallOptions...)
}

func (r RuleGroupsClient) UpdateRuleGroup(ctx context.Context, req *rulesgroups.UpdateRuleGroupRequest) (*rulesgroups.UpdateRuleGroupResponse, error) {
	callProperties, err := r.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()

	client := rulesgroups.NewRuleGroupsServiceClient(conn)

	return client.UpdateRuleGroup(callProperties.Ctx, req, callProperties.CallOptions...)
}

func (r RuleGroupsClient) DeleteRuleGroup(ctx context.Context, req *rulesgroups.DeleteRuleGroupRequest) (*rulesgroups.DeleteRuleGroupResponse, error) {
	callProperties, err := r.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()

	client := rulesgroups.NewRuleGroupsServiceClient(conn)

	return client.DeleteRuleGroup(callProperties.Ctx, req, callProperties.CallOptions...)
}

func NewRuleGroupsClient(c *CallPropertiesCreator) *RuleGroupsClient {
	return &RuleGroupsClient{callPropertiesCreator: c}
}
