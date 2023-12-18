package clientset

type ClientSet struct {
	ruleGroups           *RuleGroupsClient
	alerts               *AlertsClient
	enrichments          *EnrichmentsClient
	dataSet              *DataSetClient
	dashboards           *DashboardsClient
	grafana              *GrafanaClient
	actions              *ActionsClient
	recordingRuleGroups  *RecordingRulesGroupsSetsClient
	tcoPolicies          *TCOPoliciesClient
	tcoPoliciesOverrides *TCOPoliciesOverrides
	webhooks             *WebhooksClient
	events2Metrics       *Events2MetricsClient
	slis                 *SLIClient
	archiveRetentions    *ArchiveRetentionsClient
	archiveMetrics       *ArchiveMetricsClient
	archiveLogs          *ArchiveLogsClient
}

func (c *ClientSet) RuleGroups() *RuleGroupsClient {
	return c.ruleGroups
}

func (c *ClientSet) Alerts() *AlertsClient {
	return c.alerts
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

func (c *ClientSet) Grafana() *GrafanaClient {
	return c.grafana
}

func (c *ClientSet) Actions() *ActionsClient {
	return c.actions
}

func (c *ClientSet) RecordingRuleGroupsSets() *RecordingRulesGroupsSetsClient {
	return c.recordingRuleGroups
}

func (c *ClientSet) TCOPolicies() *TCOPoliciesClient {
	return c.tcoPolicies
}

func (c *ClientSet) TCOPoliciesOverrides() *TCOPoliciesOverrides {
	return c.tcoPoliciesOverrides
}

func (c *ClientSet) Webhooks() *WebhooksClient {
	return c.webhooks
}

func (c *ClientSet) Events2Metrics() *Events2MetricsClient {
	return c.events2Metrics
}

func (c *ClientSet) SLIs() *SLIClient {
	return c.slis
}

func (c *ClientSet) ArchiveRetentions() *ArchiveRetentionsClient {
	return c.archiveRetentions

}

func (c *ClientSet) ArchiveMetrics() *ArchiveMetricsClient {
	return c.archiveMetrics
}

func (c *ClientSet) ArchiveLogs() *ArchiveLogsClient {
	return c.archiveLogs
}

func NewClientSet(targetUrl, apiKey, teamsApiKey string) *ClientSet {
	apikeyCPC := NewCallPropertiesCreator(targetUrl, apiKey)
	_ = NewCallPropertiesCreator(targetUrl, teamsApiKey)

	return &ClientSet{
		ruleGroups:           NewRuleGroupsClient(apikeyCPC),
		alerts:               NewAlertsClient(apikeyCPC),
		events2Metrics:       NewEvents2MetricsClient(apikeyCPC),
		enrichments:          NewEnrichmentClient(apikeyCPC),
		dataSet:              NewDataSetClient(apikeyCPC),
		dashboards:           NewDashboardsClient(apikeyCPC),
		grafana:              NewGrafanaClient(apikeyCPC),
		actions:              NewActionsClient(apikeyCPC),
		recordingRuleGroups:  NewRecordingRuleGroupsClient(apikeyCPC),
		tcoPolicies:          NewTCOPoliciesClient(apikeyCPC),
		tcoPoliciesOverrides: NewTCOPoliciesOverridesClient(apikeyCPC),
		webhooks:             NewWebhooksClient(apikeyCPC),
		slis:                 NewSLIsClient(apikeyCPC),
		archiveRetentions:    NewArchiveRetentionsClient(apikeyCPC),
		archiveMetrics:       NewArchiveMetricsClient(apikeyCPC),
		archiveLogs:          NewArchiveLogsClient(apikeyCPC),
	}
}
