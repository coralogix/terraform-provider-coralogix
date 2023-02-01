package clientset

type ClientSet struct {
	ruleGroups          *RuleGroupsClient
	alerts              *AlertsClient
	logs2Metrics        *Logs2MetricsClient
	enrichments         *EnrichmentsClient
	dataSet             *DataSetClient
	dashboards          *DashboardsClient
	grafanaDashboards   *GrafanaDashboardClient
	actions             *ActionsClient
	recordingRuleGroups *RecordingRulesGroupsClient
	tcoPolicies         *TCOPolicies
	webhooks            *WebhooksClient
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

func (c *ClientSet) Dashboards() *DashboardsClient {
	return c.dashboards
}

func (c *ClientSet) GrafanaDashboards() *GrafanaDashboardClient {
	return c.grafanaDashboards
}

func (c *ClientSet) Actions() *ActionsClient {
	return c.actions
}

func (c *ClientSet) RecordingRuleGroups() *RecordingRulesGroupsClient {
	return c.recordingRuleGroups
}

func (c *ClientSet) TCOPolicies() *TCOPolicies {
	return c.tcoPolicies
}

func (c *ClientSet) Webhooks() *WebhooksClient {
	return c.webhooks
}

func NewClientSet(targetUrl, apiKey, teamsApiKey string) *ClientSet {
	apikeyCPC := NewCallPropertiesCreator(targetUrl, apiKey)
	_ = NewCallPropertiesCreator(targetUrl, teamsApiKey)

	return &ClientSet{
		ruleGroups:          NewRuleGroupsClient(apikeyCPC),
		alerts:              NewAlertsClient(apikeyCPC),
		logs2Metrics:        NewLogs2MetricsClient(apikeyCPC),
		enrichments:         NewEnrichmentClient(apikeyCPC),
		dataSet:             NewDataSetClient(apikeyCPC),
		dashboards:          NewDashboardsClient(apikeyCPC),
		grafanaDashboards:   NewGrafanaClient(apikeyCPC),
		actions:             NewActionsClient(apikeyCPC),
		recordingRuleGroups: NewRecordingRuleGroupsClient(apikeyCPC),
		tcoPolicies:         NewTCOPoliciesClient(apikeyCPC),
		webhooks:            NewWebhooksClient(apikeyCPC),
	}
}
