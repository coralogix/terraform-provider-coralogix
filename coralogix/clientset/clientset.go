// Copyright 2024 Coralogix Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package clientset

import (
	cxsdk "github.com/coralogix/coralogix-management-sdk/go"
)

type ClientSet struct {
	actions             *cxsdk.ActionsClient
	alerts              *cxsdk.AlertsClient
	apikeys             *cxsdk.ApikeysClient
	ruleGroups          *RuleGroupsClient
	enrichments         *EnrichmentsClient
	dataSet             *DataSetClient
	dashboards          *DashboardsClient
	grafana             *GrafanaClient
	recordingRuleGroups *RecordingRulesGroupsSetsClient
	tcoPolicies         *TCOPoliciesClient
	webhooks            *WebhooksClient
	events2Metrics      *Events2MetricsClient
	archiveRetentions   *ArchiveRetentionsClient
	archiveMetrics      *ArchiveMetricsClient
	archiveLogs         *ArchiveLogsClient
	alertsSchedulers    *AlertsSchedulersClient
	teams               *TeamsClient
	slos                *SLOsClient
	dahboardsFolders    *DashboardsFoldersClient
	groups              *GroupsClient
	users               *UsersClient
	customRole          *RolesClient
	scopes              *ScopesClient
	integrations        *IntegrationsClient
}

func (c *ClientSet) RuleGroups() *RuleGroupsClient {
	return c.ruleGroups
}

func (c *ClientSet) Alerts() *cxsdk.AlertsClient {
	return c.alerts
}

func (c *ClientSet) APIKeys() *cxsdk.ApikeysClient {
	return c.apikeys
}

func (c *ClientSet) Actions() *cxsdk.ActionsClient {
	return c.actions
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

func (c *ClientSet) CustomRoles() *RolesClient {
	return c.customRole
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

func (c *ClientSet) Scopes() *ScopesClient {
	return c.scopes
}

func (c *ClientSet) Integrations() *IntegrationsClient {
	return c.integrations
}

func NewClientSet(targetUrl, apiKey string) *ClientSet {
	apikeyCPC := NewCallPropertiesCreator(targetUrl, apiKey)
	apiKeySdk := cxsdk.NewCallPropertiesCreator(targetUrl, cxsdk.NewAuthContext(apiKey, apiKey))

	return &ClientSet{
		apikeys:             cxsdk.NewAPIKeysClient(apiKeySdk),
		actions:             cxsdk.NewActionsClient(apiKeySdk),
		ruleGroups:          NewRuleGroupsClient(apikeyCPC),
		alerts:              cxsdk.NewAlertsClient(apiKeySdk),
		events2Metrics:      NewEvents2MetricsClient(apikeyCPC),
		enrichments:         NewEnrichmentClient(apikeyCPC),
		dataSet:             NewDataSetClient(apikeyCPC),
		dashboards:          NewDashboardsClient(apikeyCPC),
		grafana:             NewGrafanaClient(apikeyCPC),
		recordingRuleGroups: NewRecordingRuleGroupsClient(apikeyCPC),
		tcoPolicies:         NewTCOPoliciesClient(apikeyCPC),
		webhooks:            NewWebhooksClient(apikeyCPC),
		archiveRetentions:   NewArchiveRetentionsClient(apikeyCPC),
		archiveMetrics:      NewArchiveMetricsClient(apikeyCPC),
		archiveLogs:         NewArchiveLogsClient(apikeyCPC),
		alertsSchedulers:    NewAlertsSchedulersClient(apikeyCPC),
		teams:               NewTeamsClient(apikeyCPC),
		slos:                NewSLOsClient(apikeyCPC),
		dahboardsFolders:    NewDashboardsFoldersClient(apikeyCPC),
		groups:              NewGroupsClient(apikeyCPC),
		users:               NewUsersClient(apikeyCPC),
		customRole:          NewRolesClient(apikeyCPC),
		scopes:              NewScopesClient(apikeyCPC),
		integrations:        NewIntegrationsClient(apikeyCPC),
	}
}
