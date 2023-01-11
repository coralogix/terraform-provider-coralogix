package clientset

type ClientSet struct {
	ruleGroups        *RuleGroupsClient
	alerts            *AlertsClient
	logs2Metrics      *Logs2MetricsClient
	enrichments       *EnrichmentsClient
	dataSet           *DataSetClient
	dashboards        *DashboardsClient
	grafanaDashboards *GrafanaDashboardClient
	actions           *ActionsClient
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

func NewClientSet(targetUrl, apiKey, teamsApiKey string) *ClientSet {
	apikeyCPC := NewCallPropertiesCreator(targetUrl, apiKey)
	_ = NewCallPropertiesCreator(targetUrl, teamsApiKey)

	ruleGroupsClient := NewRuleGroupsClient(apikeyCPC)
	alertsClient := NewAlertsClient(apikeyCPC)
	logs2MetricsClient := NewLogs2MetricsClient(apikeyCPC)
	enrichmentClient := NewEnrichmentClient(apikeyCPC)
	dataSetClient := NewDataSetClient(apikeyCPC)
	dashboards := NewDashboardsClient(apikeyCPC)
	grafanaDashboards := NewGrafanaClient(apikeyCPC)
	actions := NewActionsClient(apikeyCPC)

	return &ClientSet{
		ruleGroups:        ruleGroupsClient,
		alerts:            alertsClient,
		logs2Metrics:      logs2MetricsClient,
		enrichments:       enrichmentClient,
		dataSet:           dataSetClient,
		dashboards:        dashboards,
		grafanaDashboards: grafanaDashboards,
		actions:           actions,
	}
}
