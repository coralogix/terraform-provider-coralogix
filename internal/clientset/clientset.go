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
	"log/slog"
	"os"
	"strings"

	cxsdk "github.com/coralogix/coralogix-management-sdk/go"
	cxsdkOpenapi "github.com/coralogix/coralogix-management-sdk/go/openapi/cxsdk"
	actionss "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/actions_service"
	alerts "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/alert_definitions_service"
	apiKeys "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/api_keys_service"
	connectors "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/connectors_service"
	ess "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/enrichments_service"
	globalRouters "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/global_routers_service"
	integrations "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/integration_service"
	ipaccess "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/ip_access_service"
	ams "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/metrics_data_archive_service"
	webhhooks "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/outgoing_webhooks_service"
	tcoPolicys "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/policies_service"
	presets "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/presets_service"
	recRuless "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/recording_rules_service"
	retss "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/retentions_service"
	roless "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/role_management_service"
	prgs "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/rule_groups_service"
	scopess "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/scopes_service"
	archiveLogs "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/target_service"

	slos "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/slos_service"
)

type ClientSet struct {
	enrichments *cxsdk.EnrichmentsClient
	dataSet     *cxsdk.DataSetClient
	legacySlos  *cxsdk.LegacySLOsClient
	dashboards  *cxsdk.DashboardsClient
	// alertScheduler      *alertScheduler.AlertSchedulerRuleServiceAPIService
	alertScheduler   *cxsdk.AlertSchedulerClient
	dahboardsFolders *cxsdk.DashboardsFoldersClient
	ruleGroups       *cxsdk.RuleGroupsClient
	users            *cxsdk.UsersClient
	events2Metrics   *cxsdk.Events2MetricsClient
	groupGrpc        *cxsdk.GroupsClient
	teams            *cxsdk.TeamsClient

	dataEnrichments     *ess.EnrichmentsServiceAPIService
	parsingRuleGroups   *prgs.RuleGroupsServiceAPIService
	archiveMetrics      *ams.MetricsDataArchiveServiceAPIService
	archiveLogs         *archiveLogs.TargetServiceAPIService
	archiveRetentions   *retss.RetentionsServiceAPIService
	recordingRuleGroups *recRuless.RecordingRulesServiceAPIService
	tcoPolicies         *tcoPolicys.PoliciesServiceAPIService
	actions             *actionss.ActionsServiceAPIService
	alerts              *alerts.AlertDefinitionsServiceAPIService
	apikeys             *apiKeys.APIKeysServiceAPIService
	webhooks            *webhhooks.OutgoingWebhooksServiceAPIService
	slos                *slos.SlosServiceAPIService
	customRole          *roless.RoleManagementServiceAPIService
	scopes              *scopess.ScopesServiceAPIService
	connectors          *connectors.ConnectorsServiceAPIService
	presets             *presets.PresetsServiceAPIService
	globalRouters       *globalRouters.GlobalRoutersServiceAPIService
	ipaccess            *ipaccess.IPAccessServiceAPIService
	integrations        *integrations.IntegrationServiceAPIService
	grafana             *GrafanaClient
	groups              *GroupsClient
}

func (c *ClientSet) ParsingRuleGroups() *prgs.RuleGroupsServiceAPIService {
	return c.parsingRuleGroups
}

func (c *ClientSet) RuleGroups() *cxsdk.RuleGroupsClient {
	return c.ruleGroups
}

func (c *ClientSet) Alerts() *alerts.AlertDefinitionsServiceAPIService {
	return c.alerts
}

func (c *ClientSet) APIKeys() *apiKeys.APIKeysServiceAPIService {
	return c.apikeys
}

func (c *ClientSet) Actions() *actionss.ActionsServiceAPIService {
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

func (c *ClientSet) RecordingRuleGroupsSets() *recRuless.RecordingRulesServiceAPIService {
	return c.recordingRuleGroups
}

func (c *ClientSet) TCOPolicies() *tcoPolicys.PoliciesServiceAPIService {
	return c.tcoPolicies
}

func (c *ClientSet) Webhooks() *webhhooks.OutgoingWebhooksServiceAPIService {
	return c.webhooks
}

func (c *ClientSet) Events2Metrics() *cxsdk.Events2MetricsClient {
	return c.events2Metrics
}

func (c *ClientSet) ArchiveRetentions() *retss.RetentionsServiceAPIService {
	return c.archiveRetentions
}

func (c *ClientSet) ArchiveMetrics() *ams.MetricsDataArchiveServiceAPIService {
	return c.archiveMetrics
}

func (c *ClientSet) ArchiveLogs() *archiveLogs.TargetServiceAPIService {
	return c.archiveLogs
}

func (c *ClientSet) AlertSchedulers() *cxsdk.AlertSchedulerClient {
	return c.alertScheduler
}

func (c *ClientSet) CustomRoles() *roless.RoleManagementServiceAPIService {
	return c.customRole
}

func (c *ClientSet) SLOs() *slos.SlosServiceAPIService {
	return c.slos
}

func (c *ClientSet) DashboardsFolders() *cxsdk.DashboardsFoldersClient {
	return c.dahboardsFolders
}

func (c *ClientSet) Groups() *GroupsClient {
	return c.groups
}

func (c *ClientSet) Users() *cxsdk.UsersClient {
	return c.users
}

func (c *ClientSet) Scopes() *scopess.ScopesServiceAPIService {
	return c.scopes
}

func (c *ClientSet) Integrations() *integrations.IntegrationServiceAPIService {
	return c.integrations
}

func (c *ClientSet) IpAccess() *ipaccess.IPAccessServiceAPIService {
	return c.ipaccess
}

func (c *ClientSet) GroupGrpc() *cxsdk.GroupsClient {
	return c.groupGrpc
}

func (c *ClientSet) GetNotifications() (*connectors.ConnectorsServiceAPIService, *globalRouters.GlobalRoutersServiceAPIService, *presets.PresetsServiceAPIService) {
	return c.connectors, c.globalRouters, c.presets

}

func (c *ClientSet) LegacySLOs() *cxsdk.LegacySLOsClient {
	return c.legacySlos
}

func (c *ClientSet) Teams() *cxsdk.TeamsClient {
	return c.teams
}

func (c *ClientSet) DataEnrichments() *ess.EnrichmentsServiceAPIService {
	return c.dataEnrichments
}

func NewClientSet(region string, apiKey string, targetUrl string) *ClientSet {
	apiKeySdk := cxsdk.NewSDKCallPropertiesCreatorTerraform(strings.ToLower(region), cxsdk.NewAuthContext(apiKey, apiKey), TF_PROVIDER_VERSION)
	apikeyCPC := NewCallPropertiesCreator(targetUrl, apiKey)

	confBuilder := cxsdkOpenapi.NewConfigBuilder().
		WithTerraformVersion(TF_PROVIDER_VERSION).
		WithAPIKey(apiKey)

	_, found := cxsdkOpenapi.URLFromRegion(strings.ToLower(region))
	if !found {
		url := cxsdkOpenapi.URLFromDomain(region)
		confBuilder.WithURL(url)
	} else {
		confBuilder.WithRegion(strings.ToLower(region))
	}

	logLevel := slog.LevelInfo
	if strings.ToLower(os.Getenv("TF_LOG")) == "debug" {
		logLevel = slog.LevelDebug
	}

	confBuilder.WithLogger(slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel})))
	conf := confBuilder.Build()
	cs := cxsdkOpenapi.NewClientSet(conf)

	return &ClientSet{
		// deprecated
		dataSet:     cxsdk.NewDataSetClient(apiKeySdk),
		enrichments: cxsdk.NewEnrichmentClient(apiKeySdk),
		legacySlos:  cxsdk.NewLegacySLOsClient(apiKeySdk),
		ruleGroups:  cxsdk.NewRuleGroupsClient(apiKeySdk),
		teams:       cxsdk.NewTeamsClient(apiKeySdk),

		users: cxsdk.NewUsersClient(apiKeySdk),

		// TODO
		dashboards:       cxsdk.NewDashboardsClient(apiKeySdk),
		dahboardsFolders: cxsdk.NewDashboardsFoldersClient(apiKeySdk),
		events2Metrics:   cxsdk.NewEvents2MetricsClient(apiKeySdk),
		groupGrpc:        cxsdk.NewGroupsClient(apiKeySdk),
		alertScheduler:   cxsdk.NewAlertSchedulerClient(apiKeySdk),

		parsingRuleGroups:   cs.RuleGroups(),
		archiveMetrics:      cs.ArchiveMetrics(),
		alerts:              cs.Alerts(),
		archiveRetentions:   cs.ArchiveRetentions(),
		recordingRuleGroups: cs.RecordingRules(),
		archiveLogs:         cs.ArchiveLogs(),
		tcoPolicies:         cs.TCOPolicies(),
		actions:             cs.Actions(),
		customRole:          cs.CustomRoles(),
		scopes:              cs.Scopes(),
		presets:             cs.Presets(),
		connectors:          cs.Connectors(),
		globalRouters:       cs.GlobalRouters(),
		integrations:        cs.Integrations(),
		slos:                cs.SLOs(),
		apikeys:             cs.APIKeys(),
		webhooks:            cs.Webhooks(),
		ipaccess:            cs.IPAccess(),
		dataEnrichments:     cs.Enrichments(),
		// alertScheduler: cxsdkOpenapi.NewAlertSchedulerClient(oasTfCPC),
		grafana: NewGrafanaClient(apikeyCPC),
		groups:  NewGroupsClient(apikeyCPC),
	}
}
