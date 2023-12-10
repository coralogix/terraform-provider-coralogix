package clientset

import (
	"context"

	rrg "terraform-provider-coralogix/coralogix/clientset/grpc/recording-rules-groups-sets/v1"

	"google.golang.org/protobuf/types/known/emptypb"
)

type RecordingRulesGroupsSetsClient struct {
	callPropertiesCreator *CallPropertiesCreator
}

func (r RecordingRulesGroupsSetsClient) CreateRecordingRuleGroupsSet(ctx context.Context, req *rrg.CreateRuleGroupSet) (*rrg.CreateRuleGroupSetResult, error) {
	callProperties, err := r.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := rrg.NewRuleGroupSetsClient(conn)

	ctx = createAuthContext(ctx, r.callPropertiesCreator.apiKey)
	return client.Create(callProperties.Ctx, req, callProperties.CallOptions...)
}

func (r RecordingRulesGroupsSetsClient) GetRecordingRuleGroupsSet(ctx context.Context, req *rrg.FetchRuleGroupSet) (*rrg.OutRuleGroupSet, error) {
	callProperties, err := r.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := rrg.NewRuleGroupSetsClient(conn)

	return client.Fetch(callProperties.Ctx, req, callProperties.CallOptions...)
}

func (r RecordingRulesGroupsSetsClient) UpdateRecordingRuleGroupsSet(ctx context.Context, req *rrg.UpdateRuleGroupSet) (*emptypb.Empty, error) {
	callProperties, err := r.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := rrg.NewRuleGroupSetsClient(conn)

	return client.Update(callProperties.Ctx, req, callProperties.CallOptions...)
}

func (r RecordingRulesGroupsSetsClient) DeleteRecordingRuleGroupsSet(ctx context.Context, req *rrg.DeleteRuleGroupSet) (*emptypb.Empty, error) {
	callProperties, err := r.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := rrg.NewRuleGroupSetsClient(conn)

	return client.Delete(callProperties.Ctx, req, callProperties.CallOptions...)
}

func (r RecordingRulesGroupsSetsClient) ListRecordingRuleGroupsSets(ctx context.Context) (*rrg.RuleGroupSetListing, error) {
	callProperties, err := r.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := rrg.NewRuleGroupSetsClient(conn)

	ctx = createAuthContext(ctx, r.callPropertiesCreator.apiKey)
	return client.List(callProperties.Ctx, &emptypb.Empty{}, callProperties.CallOptions...)
}

func NewRecordingRuleGroupsClient(c *CallPropertiesCreator) *RecordingRulesGroupsSetsClient {
	return &RecordingRulesGroupsSetsClient{callPropertiesCreator: c}
}
