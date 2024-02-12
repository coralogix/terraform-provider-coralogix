package clientset

type ClientSet struct {
	ruleGroups          *RuleGroupsClient
	alerts              *AlertsClient
	enrichments         *EnrichmentsClient
	dataSet             *DataSetClient
	dashboards          *DashboardsClient
	grafana             *GrafanaClient
	actions             *ActionsClient
	recordingRuleGroups *RecordingRulesGroupsSetsClient
	tcoPolicies         *TCOPoliciesClient
	webhooks            *WebhooksClient
	events2Metrics      *Events2MetricsClient
	slis                *SLIClient
	archiveRetentions   *ArchiveRetentionsClient
	archiveMetrics      *ArchiveMetricsClient
	archiveLogs         *ArchiveLogsClient
	alertsSchedulers    *AlertsSchedulersClient
	teams               *TeamsClient
	slos                *SLOsClient
	dahboardsFolders    *DashboardsFoldersClient
	apiKeys             *ApikeysClient
	groups              *GroupsClient
	users               *UsersClient
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

func (c *ClientSet) AlertSchedulers() *AlertsSchedulersClient {
	return c.alertsSchedulers
}

func (c *ClientSet) Teams() *TeamsClient {
	return c.teams
}

func (c *ClientSet) ApiKeys() *ApikeysClient {
	return c.apiKeys
}

func (c *ClientSet) SLOs() *SLOsClient {
	return c.slos
}

func (c *ClientSet) DashboardsFolders() *DashboardsFoldersClient {
	return c.dahboardsFolders
}

func (c *ClientSet) Groups() *GroupsClient {
	return c.groups
}

func (c *ClientSet) Users() *UsersClient {
	return c.users
}

func NewClientSet(targetUrl, apiKey, orgKey string) *ClientSet {
	apikeyCPC := NewCallPropertiesCreator(targetUrl, apiKey)
	teamsCPC := NewCallPropertiesCreator(targetUrl, orgKey)

	return &ClientSet{
		ruleGroups:          NewRuleGroupsClient(apikeyCPC),
		alerts:              NewAlertsClient(apikeyCPC),
		events2Metrics:      NewEvents2MetricsClient(apikeyCPC),
		enrichments:         NewEnrichmentClient(apikeyCPC),
		dataSet:             NewDataSetClient(apikeyCPC),
		dashboards:          NewDashboardsClient(apikeyCPC),
		grafana:             NewGrafanaClient(apikeyCPC),
		actions:             NewActionsClient(apikeyCPC),
		recordingRuleGroups: NewRecordingRuleGroupsClient(apikeyCPC),
		tcoPolicies:         NewTCOPoliciesClient(apikeyCPC),
		webhooks:            NewWebhooksClient(apikeyCPC),
		slis:                NewSLIsClient(apikeyCPC),
		archiveRetentions:   NewArchiveRetentionsClient(apikeyCPC),
		archiveMetrics:      NewArchiveMetricsClient(apikeyCPC),
		archiveLogs:         NewArchiveLogsClient(apikeyCPC),
		alertsSchedulers:    NewAlertsSchedulersClient(apikeyCPC),
		teams:               NewTeamsClient(teamsCPC),
		slos:                NewSLOsClient(apikeyCPC),
		dahboardsFolders:    NewDashboardsFoldersClient(apikeyCPC),
		apiKeys:             NewApiKeysClient(teamsCPC),
		groups:              NewGroupsClient(teamsCPC),
		users:               NewUsersClient(teamsCPC),
	}
}
