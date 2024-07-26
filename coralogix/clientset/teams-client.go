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
