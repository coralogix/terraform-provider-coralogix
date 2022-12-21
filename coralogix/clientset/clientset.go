package clientset

import (
	"terraform-provider-coralogix/coralogix/clientset/REST"
)

var (
	EnvToGrpcUrl = map[string]string{
		"APAC1":   "ng-api-grpc.app.coralogix.in:443",
		"APAC2":   "ng-api-grpc.coralogixsg.com:443",
		"EUROPE1": "ng-api-grpc.coralogix.com:443",
		"EUROPE2": "ng-api-grpc.eu2.coralogix.com:443",
		"USA1":    "ng-api-grpc.coralogix.us:443",
	}
	EnvToRESTUrl = map[string]string{
		"APAC1":   "https://api.app.coralogix.in/api/v1/external/",
		"APAC2":   "https://api.coralogixsg.com/api/v1/external/",
		"EUROPE1": "https://api.coralogix.com/api/v1/external/",
		"EUROPE2": "https://api.eu2.coralogix.com/api/v1/external/",
		"USA1":    "https://api.coralogix.us/api/v1/external/",
	}
)

type ClientSet struct {
	ruleGroups   *RuleGroupsClient
	alerts       *AlertsClient
	logs2Metrics *Logs2MetricsClient
	enrichments  *EnrichmentsClient
	dataSet      *DataSetClient
	webhooks     *WebhooksClient
}

func (c *ClientSet) RuleGroups() *RuleGroupsClient {
	return c.ruleGroups
}

func (c *ClientSet) Alerts() *AlertsClient {
	return c.alerts
}

func (c *ClientSet) Logs2Metrics() *Logs2MetricsClient {
	return c.logs2Metrics
}

func (c *ClientSet) Enrichments() *EnrichmentsClient {
	return c.enrichments
}

func (c *ClientSet) DataSet() *DataSetClient {
	return c.dataSet
}

func (c *ClientSet) Webhooks() *WebhooksClient {
	return c.webhooks
}

func NewClientSet(env, apiKey, teamsApiKey string) *ClientSet {
	targetUrl := EnvToGrpcUrl[env]
	targetRESTUrl := EnvToRESTUrl[env]

	apikeyCPC := NewCallPropertiesCreator(targetUrl, apiKey)
	restClient := REST.NewRESTClient(targetRESTUrl, apiKey)
	_ = NewCallPropertiesCreator(targetUrl, teamsApiKey)

	ruleGroupsClient := NewRuleGroupsClient(apikeyCPC)
	alertsClient := NewAlertsClient(apikeyCPC)
	logs2MetricsClient := NewLogs2MetricsClient(apikeyCPC)
	enrichmentClient := NewEnrichmentClient(apikeyCPC)
	dataSetClient := NewDataSetClient(apikeyCPC)
	webhooksClient := NewWebhooksClient(restClient)

	return &ClientSet{
		ruleGroups:   ruleGroupsClient,
		alerts:       alertsClient,
		logs2Metrics: logs2MetricsClient,
		enrichments:  enrichmentClient,
		dataSet:      dataSetClient,
		webhooks:     webhooksClient,
	}
}
