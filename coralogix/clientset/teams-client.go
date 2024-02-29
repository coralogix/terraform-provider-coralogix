package clientset

import (
	"context"

	teams "terraform-provider-coralogix/coralogix/clientset/grpc/teams"
)

type TeamsClient struct {
	callPropertiesCreator *CallPropertiesCreator
}

func (c TeamsClient) CreateTeam(ctx context.Context, req *teams.CreateTeamInOrgRequest) (*teams.CreateTeamInOrgResponse, error) {
	callProperties, err := c.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := teams.NewTeamServiceClient(conn)

	return client.CreateTeamInOrg(callProperties.Ctx, req, callProperties.CallOptions...)
}

func (c TeamsClient) UpdateTeam(ctx context.Context, req *teams.UpdateTeamRequest) (*teams.UpdateTeamResponse, error) {
	callProperties, err := c.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := teams.NewTeamServiceClient(conn)

	return client.UpdateTeam(callProperties.Ctx, req, callProperties.CallOptions...)
}

func (c TeamsClient) GetTeam(ctx context.Context, req *teams.GetTeamRequest) (*teams.GetTeamResponse, error) {
	callProperties, err := c.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := teams.NewTeamServiceClient(conn)

	return client.GetTeam(callProperties.Ctx, req, callProperties.CallOptions...)
}

func (c TeamsClient) DeleteTeam(ctx context.Context, req *teams.DeleteTeamRequest) (*teams.DeleteTeamResponse, error) {
	callProperties, err := c.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := teams.NewTeamServiceClient(conn)

	return client.DeleteTeam(callProperties.Ctx, req, callProperties.CallOptions...)
}

func NewTeamsClient(c *CallPropertiesCreator) *TeamsClient {
	return &TeamsClient{callPropertiesCreator: c}
}
