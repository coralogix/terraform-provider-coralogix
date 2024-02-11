package clientset

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"google.golang.org/grpc/metadata"
	"terraform-provider-coralogix/coralogix/clientset/rest"
)

type GroupsClient struct {
	client    *rest.Client
	TargetUrl string
}

type SCIMGroup struct {
	ID          string            `json:"id"`
	DisplayName string            `json:"displayName"`
	Members     []SCIMGroupMember `json:"members"`
	Role        string            `json:"role"`
}

type SCIMGroupMember struct {
	Value string `json:"value"`
}

func (c GroupsClient) CreateGroup(ctx context.Context, teamID string, groupReq *SCIMGroup) (*SCIMGroup, error) {
	ctx = metadata.AppendToOutgoingContext(ctx, "cgx-team-id", teamID)
	body, err := json.Marshal(groupReq)
	if err != nil {
		return nil, err
	}

	bodyResp, err := c.client.Post(ctx, "", "application/json", string(body))
	if err != nil {
		return nil, err
	}

	var groupResp SCIMGroup
	err = json.Unmarshal([]byte(bodyResp), &groupResp)
	if err != nil {
		return nil, err
	}

	return &groupResp, nil
}

func (c GroupsClient) GetGroup(ctx context.Context, teamID, groupID string) (*SCIMGroup, error) {
	ctx = metadata.AppendToOutgoingContext(ctx, "cgx-team-id", teamID)
	bodyResp, err := c.client.Get(ctx, fmt.Sprintf("/%s", groupID))
	if err != nil {
		return nil, err
	}

	var groupResp SCIMGroup
	err = json.Unmarshal([]byte(bodyResp), &groupResp)
	if err != nil {
		return nil, err
	}

	return &groupResp, nil
}

func (c GroupsClient) UpdateGroup(ctx context.Context, teamID string, groupReq *SCIMGroup) (*SCIMGroup, error) {
	ctx = metadata.AppendToOutgoingContext(ctx, "cgx-team-id", teamID)
	body, err := json.Marshal(groupReq)
	if err != nil {
		return nil, err
	}

	bodyResp, err := c.client.Put(ctx, fmt.Sprintf("/%s", groupReq.ID), "application/json", string(body))
	if err != nil {
		return nil, err
	}

	var groupResp SCIMGroup
	err = json.Unmarshal([]byte(bodyResp), &groupResp)
	if err != nil {
		return nil, err
	}

	return &groupResp, nil
}

func (c GroupsClient) DeleteGroup(ctx context.Context, teamID, groupID string) error {
	ctx = metadata.AppendToOutgoingContext(ctx, "cgx-team-id", teamID)
	_, err := c.client.Delete(ctx, fmt.Sprintf("/%s", groupID))
	return err

}

func NewGroupsClient(c *CallPropertiesCreator) *GroupsClient {
	targetUrl := "https://" + strings.Replace(c.targetUrl, "grpc", "http", 1) + "/scim/Groups"
	client := rest.NewRestClient(targetUrl, c.apiKey)
	return &GroupsClient{client: client, TargetUrl: targetUrl}
}
