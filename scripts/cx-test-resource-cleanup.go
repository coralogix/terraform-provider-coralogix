//go:build exclude

// Copyright 2025 Coralogix Ltd.
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
package main

import (
	"context"
	"log"
	"os"
	"strings"

	cxsdk "github.com/coralogix/coralogix-management-sdk/go"
	clientset "github.com/coralogix/terraform-provider-coralogix/internal/clientset"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

var envLongToShort = map[string]string{
	"APAC1":   "AP1",
	"APAC2":   "AP2",
	"APAC3":   "AP3",
	"EUROPE1": "EU1",
	"EUROPE2": "EU2",
	"USA1":    "US1",
	"USA2":    "US2",
}

func main() {
	apiKey := os.Getenv("CORALOGIX_API_KEY")
	region := os.Getenv("CORALOGIX_ENV")
	shortRegion, ok := envLongToShort[region]
	if ok {
		region = shortRegion
	}

	ctx := context.Background()
	region = strings.TrimSpace(strings.ToLower(region))
	log.Println("Cleaning up all resources in region:", region)

	cs := clientset.NewClientSet(region, apiKey, cxsdk.CoralogixGrpcEndpointFromRegion(region))
	// Dashboards
	dashboardClient := cs.Dashboards()
	dashboards, err := dashboardClient.List(ctx)

	if err == nil {
		log.Println("Deleting all dashboards")
		for _, d := range dashboards.GetItems() {
			dashboardClient.Delete(ctx, &cxsdk.DeleteDashboardRequest{DashboardId: d.GetId()})
		}
	} else {
		log.Print("Error listing Dashboards:", err)
	}

	// Alerts
	alertClient := cs.Alerts()
	alerts, _, err := alertClient.AlertDefsServiceListAlertDefs(ctx).Execute()
	if err == nil {
		log.Println("Deleting all alerts")

		for _, alert := range alerts.GetAlertDefs() {
			alertClient.AlertDefsServiceDeleteAlertDef(ctx, alert.GetId()).Execute()
		}
	} else {
		log.Print("Error listing Alerts:", err)
	}

	// Scopes
	scopesClient := cs.Scopes()
	scopes, _, err := scopesClient.ScopesServiceGetTeamScopes(ctx).Execute()
	if err == nil {
		log.Println("Deleting all Scopes")
		for _, scope := range scopes.GetScopes() {
			scopesClient.ScopesServiceDeleteScope(ctx, scope.GetId()).Execute()
		}
	} else {
		log.Print("Error listing Scopes:", err)
	}

	// Custom Roles
	rolesClients := cs.CustomRoles()
	roles, _, err := rolesClients.RoleManagementServiceListCustomRoles(ctx).Execute()
	if err == nil {
		log.Println("Deleting all custom roles")
		for _, role := range roles.GetRoles() {
			rolesClients.RoleManagementServiceDeleteRole(ctx, role.GetRoleId()).Execute()
		}
	} else {
		log.Print("Error listing custom roles:", err)
	}

	// Enrichments
	enrichmentClient := cs.Enrichments()
	enrichments, err := enrichmentClient.List(ctx, &cxsdk.GetEnrichmentsRequest{})
	if err == nil {
		log.Println("Deleting all Enrichments")
		ids := make([]*wrapperspb.UInt32Value, 0)
		for _, enrichment := range enrichments.GetEnrichments() {
			ids = append(ids, wrapperspb.UInt32(enrichment.GetId()))
		}
		enrichmentClient.Delete(ctx, &cxsdk.DeleteEnrichmentsRequest{EnrichmentIds: ids})
	} else {
		log.Print("Error listing Enrichments:", err)
	}

	// DataSets
	dataSetClient := cs.DataSet()
	dataSets, err := dataSetClient.List(ctx, &cxsdk.ListDataSetsRequest{})
	if err == nil {
		log.Println("Deleting all DataSets")
		for _, enrichment := range dataSets.GetCustomEnrichments() {
			dataSetClient.Delete(ctx, &cxsdk.DeleteDataSetRequest{CustomEnrichmentId: wrapperspb.UInt32(enrichment.GetId())})
		}
	} else {
		log.Print("Error listing DataSets:", err)
	}

	// Webhooks
	webhookClient := cs.Webhooks()
	webhooks, _, err := webhookClient.OutgoingWebhooksServiceListAllOutgoingWebhooks(ctx).Execute()
	if err == nil {
		log.Println("Deleting all webhooks")
		for _, webhook := range webhooks.GetDeployed() {
			webhookClient.OutgoingWebhooksServiceDeleteOutgoingWebhook(ctx, webhook.GetId()).Execute()
		}
	} else {
		log.Print("Error listing webhooks:", err)
	}

	// Recording Rules
	recordingRulesGroupsSetClient := cs.ParsingRuleGroups()
	recordingRulesGroupsSets, _, err := recordingRulesGroupsSetClient.RuleGroupsServiceListRuleGroups(ctx).Execute()
	if err == nil {
		log.Println("Deleting all parsing rules")
		groupIds := make([]string, 0)
		for _, recordingRulesGroupsSet := range recordingRulesGroupsSets.RuleGroups {
			groupIds = append(groupIds, *recordingRulesGroupsSet.Id)
		}
		_, _, err = recordingRulesGroupsSetClient.RuleGroupsServiceBulkDeleteRuleGroup(ctx).GroupIds(groupIds).Execute()
		if err != nil {
			log.Println("Error deleting all parsing rules:", err)
		}
	} else {
		log.Print("Error listing parsing rules:", err)
	}

	// Events2Metrics
	events2metricsClient := cs.Events2Metrics()
	events2metrics, err := events2metricsClient.List(ctx)
	if err == nil {
		log.Println("Deleting all events2metrics")
		for _, events2metric := range events2metrics.GetE2M() {
			events2metricsClient.Delete(ctx, &cxsdk.DeleteE2MRequest{Id: events2metric.GetId()})
		}
	} else {
		log.Print("Error listing events2metrics:", err)
	}

	// Dashboard folders
	dashboardsFolderClient := cs.DashboardsFolders()
	dashboardsFolders, _, err := dashboardsFolderClient.DashboardFoldersServiceListDashboardFolders(ctx).Execute()
	if err == nil {
		log.Println("Deleting all dashboard folders")
		for _, dashboardsFolder := range dashboardsFolders.GetFolder() {
			_, _, _ = dashboardsFolderClient.DashboardFoldersServiceDeleteDashboardFolder(ctx, *dashboardsFolder.Id).Execute()
		}
	} else {
		log.Print("Error listing dashboard folders:", err)
	}

	// TCO
	tcoPoliciesTracesClient := cs.TCOPolicies()
	tcoPolicies, _, err := tcoPoliciesTracesClient.PoliciesServiceGetCompanyPolicies(ctx).Execute()
	if err == nil {
		log.Println("Deleting all TCO Traces policies")

		for _, tcoPolicy := range tcoPolicies.GetPolicies() {
			if tcoPolicy.PolicyLogRules != nil {
				tcoPoliciesTracesClient.PoliciesServiceDeletePolicy(ctx, tcoPolicy.PolicyLogRules.GetId()).Execute()
			}
			if tcoPolicy.PolicySpanRules != nil {
				tcoPoliciesTracesClient.PoliciesServiceDeletePolicy(ctx, tcoPolicy.PolicySpanRules.GetId()).Execute()
			}
		}
	} else {
		log.Print("Error listing TCO policies:", err)
	}

	tcoPoliciesLogsClient := cs.TCOPolicies()
	tcoPolicies, _, err = tcoPoliciesLogsClient.PoliciesServiceGetCompanyPolicies(ctx).Execute()
	if err == nil {
		log.Println("Deleting all TCO Logs policies")
		for _, tcoPolicy := range tcoPolicies.GetPolicies() {
			if tcoPolicy.PolicyLogRules != nil {
				tcoPoliciesLogsClient.PoliciesServiceDeletePolicy(ctx, tcoPolicy.PolicyLogRules.GetId()).Execute()
			}
			if tcoPolicy.PolicySpanRules != nil {
				tcoPoliciesLogsClient.PoliciesServiceDeletePolicy(ctx, tcoPolicy.PolicySpanRules.GetId()).Execute()
			}
		}
	} else {
		log.Print("Error listing TCO Logs policies:", err)
	}
	// Groups
	groupClient := cxsdk.NewGroupsClient(cxsdk.NewSDKCallPropertiesCreator(region, cxsdk.NewAuthContext(apiKey, apiKey)))
	groups, err := groupClient.List(ctx, &cxsdk.GetTeamGroupsRequest{})

	if err == nil {
		log.Println("Deleting all groups")
		for _, group := range groups.GetGroups() {
			if !strings.HasPrefix(group.Name, "CI Group") {
				groupClient.Delete(ctx, &cxsdk.DeleteTeamGroupRequest{GroupId: group.GetGroupId()})
			}
		}
	} else {
		log.Print("Error listing groups:", err)
	}
	// Connectors
	connectors, globalRouters, presets := cs.GetNotifications()
	listConnectorsRes, _, err := connectors.ConnectorsServiceListConnectorSummaries(ctx).Execute()
	if err == nil {
		log.Println("Deleting all connectors")
		for _, connector := range listConnectorsRes.Connectors {
			connectors.ConnectorsServiceDeleteConnector(ctx, *connector.Id).Execute()
		}
	} else {
		log.Print("Error listing connectors:", err)
	}

	// Presets
	listPresetsRes, _, err := presets.PresetsServiceListPresetSummaries(ctx).Execute()
	if err == nil {
		log.Println("Deleting all presets")
		for _, preset := range listPresetsRes.PresetSummaries {
			presets.PresetsServiceDeleteCustomPreset(ctx, *preset.Id).Execute()
		}
	} else {
		log.Print("Error listing presets:", err)
	}

	// Global Routers
	listRoutersRes, _, err := globalRouters.GlobalRoutersServiceListGlobalRouters(ctx).Execute()
	if err == nil {
		log.Println("Deleting all global routers")
		for _, r := range listRoutersRes.Routers {
			globalRouters.GlobalRoutersServiceDeleteGlobalRouter(ctx, *r.Id).Execute()
		}
		globalRouters.GlobalRoutersServiceDeleteGlobalRouter(ctx, "router_default").Execute()
	} else {
		log.Print("Error listing global routers:", err)
	}

	// Users
	usersClient := cs.Users()
	users, err := usersClient.List(ctx)
	if err == nil {
		log.Println("Deleting all users")
		for _, user := range users {
			if user.ID != nil {
				usersClient.Delete(ctx, *user.ID)
			}
		}
	} else {
		log.Print("Error listing users:", err)
	}

	// Views
	viewsClient := cxsdk.NewViewsClient(cxsdk.NewSDKCallPropertiesCreator(region, cxsdk.NewAuthContext(apiKey, apiKey)))
	views, err := viewsClient.List(ctx, &cxsdk.ListViewsRequest{})
	if err == nil {
		log.Println("Deleting all views")
		for _, view := range views.Views {
			if view.Id != nil {
				viewsClient.Delete(ctx, &cxsdk.DeleteViewRequest{
					Id: view.Id,
				})
			}
		}
	} else {
		log.Print("Error listing users:", err)
	}

	// Views
	viewFoldersClient := cxsdk.NewViewFoldersClient(cxsdk.NewSDKCallPropertiesCreator(region, cxsdk.NewAuthContext(apiKey, apiKey)))
	viewFolders, err := viewFoldersClient.List(ctx, &cxsdk.ListViewFoldersRequest{})
	if err == nil {
		log.Println("Deleting all viewFolders")
		for _, f := range viewFolders.Folders {
			if f.Id != nil {
				viewFoldersClient.Delete(ctx, &cxsdk.DeleteViewFolderRequest{
					Id: f.Id,
				})
			}
		}
	} else {
		log.Print("Error listing views:", err)
	}

	// IPAccesss
	ipaccessClient := cs.IpAccess()
	ipaccess, _, err := ipaccessClient.IpAccessServiceGetCompanyIpAccessSettings(ctx).Execute()
	if err == nil {
		log.Println("Deleting all IpAccess")
		if ipaccess.Settings.IpAccess != nil {
			for _ = range *ipaccess.Settings.IpAccess {
				ipaccessClient.IpAccessServiceDeleteCompanyIpAccessSettings(ctx).Execute()
			}
		}
	} else {
		log.Print("Error listing IP Access:", err)
	}

	// SLOs
	sloClient := cs.SLOs()
	slos, _, err := sloClient.SlosServiceListSlos(ctx).Execute()
	if err == nil {
		log.Println("Deleting all SLOs")
		for _, f := range slos.Slos {
			var id string
			if f.SloRequestBasedMetricSli != nil {
				id = *f.SloRequestBasedMetricSli.Id
			} else if f.SloWindowBasedMetricSli != nil {
				id = *f.SloWindowBasedMetricSli.Id
			} else {
				continue
			}
			sloClient.SlosServiceDeleteSlo(ctx, id).Execute()
		}
	} else {
		log.Print("Error listing SLOs:", err)
	}
}
