package clientset

import (
	"context"
	"fmt"
	"strings"

	"terraform-provider-coralogix/coralogix/clientset/rest"
)

type TCOPoliciesOverrides struct {
	client *rest.Client
}

func (t TCOPoliciesOverrides) CreateTCOPolicyOverride(ctx context.Context, jsonContent string) (string, error) {
	return t.client.Post(ctx, "/overrides", "application/json", jsonContent)
}

func (t TCOPoliciesOverrides) GetTCOPolicyOverride(ctx context.Context, id string) (string, error) {
	return t.client.Get(ctx, fmt.Sprintf("/overrides/%s", id))
}

func (t TCOPoliciesOverrides) UpdateTCOPolicyOverride(ctx context.Context, id string, jsonContent string) (string, error) {
	return t.client.Put(ctx, fmt.Sprintf("/overrides/%s", id), "application/json", jsonContent)
}

func (t TCOPoliciesOverrides) DeleteTCOPolicyOverride(ctx context.Context, id string) error {
	_, err := t.client.Delete(ctx, fmt.Sprintf("/overrides/%s", id))
	return err
}

func NewTCOPoliciesOverridesClient(c *CallPropertiesCreator) *TCOPoliciesOverrides {
	targetUrl := "https://" + strings.Replace(c.targetUrl, "ng-api-grpc", "api", 1) + "/api/v1/external/tco"
	client := rest.NewRestClient(targetUrl, c.apiKey)
	return &TCOPoliciesOverrides{client: client}
}
