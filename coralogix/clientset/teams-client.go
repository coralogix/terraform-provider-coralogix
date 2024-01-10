package clientset

import (
	"context"

	teams "terraform-provider-coralogix/coralogix/clientset/grpc/teams"
)

type TeamsClient struct {
	callPropertiesCreator *CallPropertiesCreator
}

func (t TCOPoliciesClient) CreateTeam(ctx context.Context, req *teams.CreateTeamInOrgRequest) (*teams.CreateTeamInOrgResponse, error) {
	callProperties, err := t.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := teams.NewTeamServiceClient(conn)

	return client.CreateTeamInOrg(callProperties.Ctx, req, callProperties.CallOptions...)
}

func NewTeamsClient(c *CallPropertiesCreator) *TeamsClient {
	return &TeamsClient{callPropertiesCreator: c}
}
