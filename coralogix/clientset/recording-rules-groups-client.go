package clientset

import (
	"context"
	"strings"

	"terraform-provider-coralogix/coralogix/clientset/rest"
)

type RecordingRulesGroupsClient struct {
	client *rest.Client
}

func (r RecordingRulesGroupsClient) CreateRecordingRuleRules(ctx context.Context, yamlContent string) (string, error) {
	return r.client.Put(ctx, "/metrics/rule-groups", "application/yaml", yamlContent)
}

func (r RecordingRulesGroupsClient) GetRecordingRuleRules(ctx context.Context) (string, error) {
	return r.client.Get(ctx, "/metrics/rule-groups")
}

func (r RecordingRulesGroupsClient) UpdateRecordingRuleRules(ctx context.Context, yamlContent string) (string, error) {
	return r.CreateRecordingRuleRules(ctx, yamlContent)
}

func (r RecordingRulesGroupsClient) DeleteRecordingRuleRules(ctx context.Context) error {
	_, err := r.client.Put(ctx, "/metrics/rule-groups", "application/yaml", "groups: []")
	return err
}

func NewRecordingRulesGroupsClient(c *CallPropertiesCreator) *RecordingRulesGroupsClient {
	targetUrl := "https://" + strings.Replace(c.targetUrl, "grpc", "http", 1)
	client := rest.NewRestClient(targetUrl, c.apiKey)
	return &RecordingRulesGroupsClient{client: client}
}
