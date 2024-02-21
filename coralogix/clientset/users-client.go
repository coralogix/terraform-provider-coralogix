package clientset

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"terraform-provider-coralogix/coralogix/clientset/rest"
)

type UsersClient struct {
	client    *rest.Client
	TargetUrl string
}

type SCIMUser struct {
	Schemas  []string        `json:"schemas"`
	ID       *string         `json:"id,omitempty"`
	UserName string          `json:"userName"`
	Active   bool            `json:"active"`
	Name     *SCIMUserName   `json:"name,omitempty"`
	Groups   []SCIMUserGroup `json:"groups,omitempty"`
	Emails   []SCIMUserEmail `json:"emails,omitempty"`
}

type SCIMUserName struct {
	GivenName  string `json:"givenName"`
	FamilyName string `json:"familyName"`
}

type SCIMUserEmail struct {
	Value   string `json:"value"`
	Primary bool   `json:"primary"`
	Type    string `json:"type"`
}

type SCIMUserGroup struct {
	Value string `json:"value"`
}

func (c UsersClient) CreateUser(ctx context.Context, userReq *SCIMUser) (*SCIMUser, error) {
	body, err := json.Marshal(userReq)
	if err != nil {
		return nil, err
	}

	bodyResp, err := c.client.Post(ctx, "", "application/json", string(body))
	if err != nil {
		return nil, err
	}

	var UserResp SCIMUser
	err = json.Unmarshal([]byte(bodyResp), &UserResp)
	if err != nil {
		return nil, err
	}

	return &UserResp, nil
}

func (c UsersClient) GetUser(ctx context.Context, userID string) (*SCIMUser, error) {
	bodyResp, err := c.client.Get(ctx, fmt.Sprintf("/%s", userID))
	if err != nil {
		return nil, err
	}

	var UserResp SCIMUser
	err = json.Unmarshal([]byte(bodyResp), &UserResp)
	if err != nil {
		return nil, err
	}

	return &UserResp, nil
}

func (c UsersClient) UpdateUser(ctx context.Context, userID string, userReq *SCIMUser) (*SCIMUser, error) {
	body, err := json.Marshal(userReq)
	if err != nil {
		return nil, err
	}

	bodyResp, err := c.client.Put(ctx, fmt.Sprintf("/%s", userID), "application/json", string(body))
	if err != nil {
		return nil, err
	}

	var UserResp SCIMUser
	err = json.Unmarshal([]byte(bodyResp), &UserResp)
	if err != nil {
		return nil, err
	}

	return &UserResp, nil
}

func (c UsersClient) DeleteUser(ctx context.Context, userID string) error {
	_, err := c.client.Delete(ctx, fmt.Sprintf("/%s", userID))
	return err

}

func NewUsersClient(c *CallPropertiesCreator) *UsersClient {
	targetUrl := "https://" + strings.Replace(c.targetUrl, "grpc", "http", 1) + "/scim/Users"
	client := rest.NewRestClient(targetUrl, c.apiKey)
	return &UsersClient{client: client, TargetUrl: targetUrl}
}
