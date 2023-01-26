package clientset

import (
	"context"

	"google.golang.org/protobuf/types/known/emptypb"
	recordingrules "terraform-provider-coralogix/coralogix/clientset/grpc/recording-rules-groups/v1"
)

type RecordingRulesGroupsClient struct {
	callPropertiesCreator *CallPropertiesCreator
}

func (r RecordingRulesGroupsClient) CreateRecordingRuleGroup(ctx context.Context, req *recordingrules.RecordingRuleGroup) (*emptypb.Empty, error) {
	callProperties, err := r.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := recordingrules.NewRuleGroupsClient(conn)

	ctx = createAuthContext(ctx, r.callPropertiesCreator.apiKey)
	return client.Save(callProperties.Ctx, req, callProperties.CallOptions...)
}

func (r RecordingRulesGroupsClient) GetRecordingRuleGroup(ctx context.Context, req *recordingrules.FetchRuleGroup) (*recordingrules.FetchRuleGroupResult, error) {
	callProperties, err := r.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := recordingrules.NewRuleGroupsClient(conn)

	ctx = createAuthContext(ctx, r.callPropertiesCreator.apiKey)
	return client.Fetch(ctx, req, callProperties.CallOptions...)
}

func (r RecordingRulesGroupsClient) UpdateRecordingRuleGroup(ctx context.Context, req *recordingrules.RecordingRuleGroup) (*emptypb.Empty, error) {
	return r.CreateRecordingRuleGroup(ctx, req)
}

func (r RecordingRulesGroupsClient) DeleteRecordingRuleGroup(ctx context.Context, req *recordingrules.DeleteRuleGroup) (*emptypb.Empty, error) {
	callProperties, err := r.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := recordingrules.NewRuleGroupsClient(conn)

	ctx = createAuthContext(ctx, r.callPropertiesCreator.apiKey)
	return client.Delete(ctx, req, callProperties.CallOptions...)
}

func (r RecordingRulesGroupsClient) ListRecordingRuleGroup(ctx context.Context) (*recordingrules.RuleGroupListing, error) {
	callProperties, err := r.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := recordingrules.NewRuleGroupsClient(conn)

	ctx = createAuthContext(ctx, r.callPropertiesCreator.apiKey)
	return client.List(ctx, &emptypb.Empty{}, callProperties.CallOptions...)
}

func NewRecordingRuleGroupsClient(c *CallPropertiesCreator) *RecordingRulesGroupsClient {
	return &RecordingRulesGroupsClient{callPropertiesCreator: c}
}
