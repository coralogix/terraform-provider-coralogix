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
	cxsdk "github.com/coralogix/coralogix-management-sdk/go"
)

type ClientSet struct {
	actions           *cxsdk.ActionsClient
	alerts            *cxsdk.AlertsClient
	apikeys           *cxsdk.ApikeysClient
	integrations      *cxsdk.IntegrationsClient
	enrichments       *cxsdk.EnrichmentsClient
	dataSet           *cxsdk.DataSetClient
	webhooks          *cxsdk.WebhooksClient
	slos              *cxsdk.SLOsClient
	teams             *cxsdk.TeamsClient
	scopes            *cxsdk.ScopesClient
	dashboards        *cxsdk.DashboardsClient
	archiveLogs       *cxsdk.ArchiveLogsClient
	archiveMetrics    *cxsdk.ArchiveMetricsClient
	archiveRetentions *cxsdk.ArchiveRetentionsClient
	tcoPolicies       *cxsdk.TCOPoliciesClient
	alertScheduler    *cxsdk.AlertSchedulerClient

	ruleGroups          *RuleGroupsClient
	grafana             *GrafanaClient
	recordingRuleGroups *RecordingRulesGroupsSetsClient
	events2Metrics      *Events2MetricsClient
	dahboardsFolders    *DashboardsFoldersClient
	groups              *GroupsClient
	users               *UsersClient
	customRole          *RolesClient
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
func (c *ClientSet) Enrichments() *cxsdk.EnrichmentsClient {
	return c.enrichments
}

func (c *ClientSet) DataSet() *cxsdk.DataSetClient {
	return c.dataSet
}

func (c *ClientSet) Dashboards() *cxsdk.DashboardsClient {
	return c.dashboards
}

func (c *ClientSet) Grafana() *GrafanaClient {
	return c.grafana
}

func (c *ClientSet) RecordingRuleGroupsSets() *RecordingRulesGroupsSetsClient {
	return c.recordingRuleGroups
}

func (c *ClientSet) TCOPolicies() *cxsdk.TCOPoliciesClient {
	return c.tcoPolicies
}

func (c *ClientSet) Webhooks() *cxsdk.WebhooksClient {
	return c.webhooks
}

func (c *ClientSet) Events2Metrics() *Events2MetricsClient {
	return c.events2Metrics
}

func (c *ClientSet) ArchiveRetentions() *cxsdk.ArchiveRetentionsClient {
	return c.archiveRetentions
}

func (c *ClientSet) ArchiveMetrics() *cxsdk.ArchiveMetricsClient {
	return c.archiveMetrics
}

func (c *ClientSet) ArchiveLogs() *cxsdk.ArchiveLogsClient {
	return c.archiveLogs
}

func (c *ClientSet) AlertSchedulers() *cxsdk.AlertSchedulerClient {
	return c.alertScheduler
}

func (c *ClientSet) Teams() *cxsdk.TeamsClient {
	return c.teams
}

func (c *ClientSet) CustomRoles() *RolesClient {
	return c.customRole
}

func (c *ClientSet) SLOs() *cxsdk.SLOsClient {
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

func (c *ClientSet) Scopes() *cxsdk.ScopesClient {
	return c.scopes
}

func (c *ClientSet) Integrations() *cxsdk.IntegrationsClient {
	return c.integrations
}

func NewClientSet(targetUrl, apiKey string) *ClientSet {
	apikeyCPC := NewCallPropertiesCreator(targetUrl, apiKey)
	apiKeySdk := cxsdk.NewCallPropertiesCreator(targetUrl, cxsdk.NewAuthContext(apiKey, apiKey))

	return &ClientSet{
		apikeys:           cxsdk.NewAPIKeysClient(apiKeySdk),
		actions:           cxsdk.NewActionsClient(apiKeySdk),
		integrations:      cxsdk.NewIntegrationsClient(apiKeySdk),
		enrichments:       cxsdk.NewEnrichmentClient(apiKeySdk),
		alerts:            cxsdk.NewAlertsClient(apiKeySdk),
		dataSet:           cxsdk.NewDataSetClient(apiKeySdk),
		webhooks:          cxsdk.NewWebhooksClient(apiKeySdk),
		slos:              cxsdk.NewSLOsClient(apiKeySdk),
		teams:             cxsdk.NewTeamsClient(apiKeySdk),
		scopes:            cxsdk.NewScopesClient(apiKeySdk),
		dashboards:        cxsdk.NewDashboardsClient(apiKeySdk),
		archiveLogs:       cxsdk.NewArchiveLogsClient(apiKeySdk),
		archiveMetrics:    cxsdk.NewArchiveMetricsClient(apiKeySdk),
		archiveRetentions: cxsdk.NewArchiveRetentionsClient(apiKeySdk),
		tcoPolicies:       cxsdk.NewTCOPoliciesClient(apiKeySdk),
		alertScheduler:    cxsdk.NewAlertSchedulerClient(apiKeySdk),

		ruleGroups:          NewRuleGroupsClient(apikeyCPC),
		events2Metrics:      NewEvents2MetricsClient(apikeyCPC),
		grafana:             NewGrafanaClient(apikeyCPC),
		recordingRuleGroups: NewRecordingRuleGroupsClient(apikeyCPC),
		dahboardsFolders:    NewDashboardsFoldersClient(apikeyCPC),
		groups:              NewGroupsClient(apikeyCPC),
		users:               NewUsersClient(apikeyCPC),
		customRole:          NewRolesClient(apikeyCPC),
	}
}
