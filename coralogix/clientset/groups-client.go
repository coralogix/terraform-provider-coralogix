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
	"encoding/json"
	"fmt"
	"log"
	"strings"

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
	ScopeID     string            `json:"nextGenScopeId"`
}

type SCIMGroupMember struct {
	Value string `json:"value"`
}

func (c GroupsClient) CreateGroup(ctx context.Context, groupReq *SCIMGroup) (*SCIMGroup, error) {
	body, err := json.Marshal(groupReq)
	if err != nil {
		return nil, err
	}
	log.Printf("[INFO] Creating Group: %s", body)

	bodyResp, err := c.client.Post(ctx, "", "application/json", string(body))
	if err != nil {
		return nil, err
	}
	log.Printf("[INFO] Received Group: %s", bodyResp)

	var groupResp SCIMGroup
	err = json.Unmarshal([]byte(bodyResp), &groupResp)
	if err != nil {
		return nil, err
	}

	return &groupResp, nil
}

func (c GroupsClient) GetGroup(ctx context.Context, groupID string) (*SCIMGroup, error) {
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

func (c GroupsClient) UpdateGroup(ctx context.Context, groupID string, groupReq *SCIMGroup) (*SCIMGroup, error) {
	body, err := json.Marshal(groupReq)
	if err != nil {
		return nil, err
	}

	bodyResp, err := c.client.Put(ctx, fmt.Sprintf("/%s", groupID), "application/json", string(body))
	if err != nil {
		return nil, err
	}
	log.Printf("[INFO] Received Group: %s", bodyResp)

	var groupResp SCIMGroup
	err = json.Unmarshal([]byte(bodyResp), &groupResp)
	if err != nil {
		return nil, err
	}

	return &groupResp, nil
}

func (c GroupsClient) DeleteGroup(ctx context.Context, groupID string) error {
	log.Printf("[INFO] Deleting Group: %s", groupID)

	_, err := c.client.Delete(ctx, fmt.Sprintf("/%s", groupID))
	return err
}

func NewGroupsClient(c *CallPropertiesCreator) *GroupsClient {
	targetUrl := "https://" + strings.Replace(c.targetUrl, "grpc", "http", 1) + "/scim/Groups"
	client := rest.NewRestClient(targetUrl, c.apiKey)
	return &GroupsClient{client: client, TargetUrl: targetUrl}
}
