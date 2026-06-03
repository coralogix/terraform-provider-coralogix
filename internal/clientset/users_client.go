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

	cxsdk "github.com/coralogix/coralogix-management-sdk/go"
	"github.com/coralogix/terraform-provider-coralogix/internal/clientset/rest"
)

// UsersClient calls the SCIM Users API with a region/domain-aware management base URL.
type UsersClient struct {
	client  *rest.Client
	baseURL string
}

// NewUsersClient builds a SCIM users client for provider env or domain configuration.
func NewUsersClient(regionOrDomain, apiKey string) *UsersClient {
	baseURL := ScimRestBaseURL(regionOrDomain) + "/scim/Users"
	return &UsersClient{
		client:  rest.NewRestClient(baseURL, apiKey),
		baseURL: baseURL,
	}
}

// BaseURL returns the SCIM Users collection URL.
func (c *UsersClient) BaseURL() string {
	return c.baseURL
}

// Create creates a new SCIM user.
func (c *UsersClient) Create(ctx context.Context, userReq *cxsdk.SCIMUser) (*cxsdk.SCIMUser, error) {
	body, err := json.Marshal(userReq)
	if err != nil {
		return nil, err
	}

	bodyResp, err := c.client.Post(ctx, "", "application/json", string(body))
	if err != nil {
		return nil, err
	}

	var userResp cxsdk.SCIMUser
	if err := json.Unmarshal([]byte(bodyResp), &userResp); err != nil {
		return nil, err
	}

	return &userResp, nil
}

// Get retrieves a SCIM user by ID.
func (c *UsersClient) Get(ctx context.Context, userID string) (*cxsdk.SCIMUser, error) {
	bodyResp, err := c.client.Get(ctx, fmt.Sprintf("/%s", userID))
	if err != nil {
		return nil, err
	}

	var userResp cxsdk.SCIMUser
	if err := json.Unmarshal([]byte(bodyResp), &userResp); err != nil {
		return nil, err
	}

	return &userResp, nil
}

// List retrieves all SCIM users.
func (c *UsersClient) List(ctx context.Context) ([]cxsdk.SCIMUser, error) {
	bodyResp, err := c.client.Get(ctx, "")
	if err != nil {
		return nil, err
	}

	var listResp struct {
		Resources []cxsdk.SCIMUser `json:"Resources"`
	}
	if err := json.Unmarshal([]byte(bodyResp), &listResp); err != nil {
		return nil, err
	}

	return listResp.Resources, nil
}

// Update updates a SCIM user by ID.
func (c *UsersClient) Update(ctx context.Context, userID string, userReq *cxsdk.SCIMUser) (*cxsdk.SCIMUser, error) {
	body, err := json.Marshal(userReq)
	if err != nil {
		return nil, err
	}

	bodyResp, err := c.client.Put(ctx, fmt.Sprintf("/%s", userID), "application/json", string(body))
	if err != nil {
		return nil, err
	}

	var userResp cxsdk.SCIMUser
	if err := json.Unmarshal([]byte(bodyResp), &userResp); err != nil {
		return nil, err
	}

	return &userResp, nil
}

// Delete deletes a SCIM user by ID.
func (c *UsersClient) Delete(ctx context.Context, userID string) error {
	_, err := c.client.Delete(ctx, fmt.Sprintf("/%s", userID))
	return err
}
