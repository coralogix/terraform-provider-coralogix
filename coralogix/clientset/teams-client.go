package clientset

import (
	"context"

	teams "terraform-provider-coralogix/coralogix/clientset/grpc/teams"
)

type TeamsClient struct {
	callPropertiesCreator *CallPropertiesCreator
}

func (t TeamsClient) CreateTeam(ctx context.Context, req *teams.CreateTeamInOrgRequest) (*teams.CreateTeamInOrgResponse, error) {
	callProperties, err := t.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := teams.NewTeamServiceClient(conn)

	return client.CreateTeamInOrg(callProperties.Ctx, req, callProperties.CallOptions...)
}

func (t TeamsClient) GetTeamQuota(ctx context.Context, req *teams.GetTeamQuotaRequest) (*teams.GetTeamQuotaResponse, error) {
	callProperties, err := t.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := teams.NewTeamServiceClient(conn)

	return client.GetTeamQuota(callProperties.Ctx, req, callProperties.CallOptions...)
}

func (t TeamsClient) GetQuota(ctx context.Context, req *teams.GetTeamQuotaRequest) (*teams.GetTeamQuotaResponse, error) {
	callProperties, err := t.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := teams.NewTeamServiceClient(conn)

	return client.GetTeamQuota(callProperties.Ctx, req, callProperties.CallOptions...)
}

func (t TeamsClient) MoveQuota(ctx context.Context, req *teams.MoveQuotaRequest) (*teams.MoveQuotaResponse, error) {
	callProperties, err := t.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := teams.NewTeamServiceClient(conn)

	return client.MoveQuota(callProperties.Ctx, req, callProperties.CallOptions...)
}

func NewTeamsClient(c *CallPropertiesCreator) *TeamsClient {
	return &TeamsClient{callPropertiesCreator: c}
}
