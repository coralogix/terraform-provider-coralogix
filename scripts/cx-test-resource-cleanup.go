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
	"github.com/coralogix/coralogix-management-sdk/go/internal/coralogixapis/views/v1/services"
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
	region = strings.TrimSpace(strings.ToLower(region))
	log.Println("Cleaning up all resources in region:", region)
	// Dashboards
	dashboardClient := cxsdk.NewDashboardsClient(cxsdk.NewCallPropertiesCreator(region, cxsdk.NewAuthContext(apiKey, apiKey)))
	dashboards, err := dashboardClient.List(context.Background())

	if err == nil {
		log.Println("Deleting all dashboards")
		for _, d := range dashboards.GetItems() {
			dashboardClient.Delete(context.Background(), &cxsdk.DeleteDashboardRequest{DashboardId: d.GetId()})
		}
	} else {
		log.Fatal("Error listing Dashboards:", err)
	}

	// Alerts
	alertClient := cxsdk.NewAlertsClient(cxsdk.NewCallPropertiesCreator(region, cxsdk.NewAuthContext(apiKey, apiKey)))
	alerts, err := alertClient.List(context.Background(), &cxsdk.ListAlertDefsRequest{})
	if err == nil {
		log.Println("Deleting all alerts")

		for _, alert := range alerts.GetAlertDefs() {
			alertClient.Delete(context.Background(), &cxsdk.DeleteAlertDefRequest{Id: alert.GetId()})
		}
	} else {
		log.Fatal("Error listing Alerts:", err)
	}

	// Scopes
	scopesClient := cxsdk.NewScopesClient(cxsdk.NewCallPropertiesCreator(region, cxsdk.NewAuthContext(apiKey, apiKey)))
	scopes, err := scopesClient.List(context.Background(), &cxsdk.GetTeamScopesRequest{})
	if err == nil {
		log.Println("Deleting all Scopes")
		for _, scope := range scopes.GetScopes() {
			scopesClient.Delete(context.Background(), &cxsdk.DeleteScopeRequest{Id: scope.GetId()})
		}
	} else {
		log.Fatal("Error listing Scopes:", err)
	}

	// Custom Roles
	rolesClients := cxsdk.NewRolesClient(cxsdk.NewCallPropertiesCreator(region, cxsdk.NewAuthContext(apiKey, apiKey)))
	roles, err := rolesClients.List(context.Background(), &cxsdk.ListCustomRolesRequest{})
	if err == nil {
		log.Println("Deleting all custom roles")
		for _, role := range roles.GetRoles() {
			rolesClients.Delete(context.Background(), &cxsdk.DeleteRoleRequest{RoleId: role.GetRoleId()})
		}
	} else {
		log.Fatal("Error listing custom roles:", err)
	}

	// Enrichments
	enrichmentClient := cxsdk.NewEnrichmentClient(cxsdk.NewCallPropertiesCreator(region, cxsdk.NewAuthContext(apiKey, apiKey)))
	enrichments, err := enrichmentClient.List(context.Background(), &cxsdk.GetEnrichmentsRequest{})
	if err == nil {
		log.Println("Deleting all Enrichments")
		ids := make([]*wrapperspb.UInt32Value, 0)
		for _, enrichment := range enrichments.GetEnrichments() {
			ids = append(ids, wrapperspb.UInt32(enrichment.GetId()))
		}
		enrichmentClient.Delete(context.Background(), &cxsdk.DeleteEnrichmentsRequest{EnrichmentIds: ids})
	} else {
		log.Fatal("Error listing Enrichments:", err)
	}

	// DataSets
	dataSetClient := cxsdk.NewDataSetClient(cxsdk.NewCallPropertiesCreator(region, cxsdk.NewAuthContext(apiKey, apiKey)))
	dataSets, err := dataSetClient.List(context.Background(), &cxsdk.ListDataSetsRequest{})
	if err == nil {
		log.Println("Deleting all DataSets")
		for _, enrichment := range dataSets.GetCustomEnrichments() {
			dataSetClient.Delete(context.Background(), &cxsdk.DeleteDataSetRequest{CustomEnrichmentId: wrapperspb.UInt32(enrichment.GetId())})
		}
	} else {
		log.Fatal("Error listing DataSets:", err)
	}

	// Webhooks
	webhookClient := cxsdk.NewWebhooksClient(cxsdk.NewCallPropertiesCreator(region, cxsdk.NewAuthContext(apiKey, apiKey)))
	webhooks, err := webhookClient.List(context.Background(), &cxsdk.ListAllOutgoingWebhooksRequest{})
	if err == nil {
		log.Println("Deleting all webhooks")
		for _, webhook := range webhooks.GetDeployed() {
			webhookClient.Delete(context.Background(), &cxsdk.DeleteOutgoingWebhookRequest{Id: webhook.GetId()})
		}
	} else {
		log.Fatal("Error listing webhooks:", err)
	}

	// Recording Rules
	recordingRulesGroupsSetClient := cxsdk.NewRecordingRuleGroupSetsClient(cxsdk.NewCallPropertiesCreator(region, cxsdk.NewAuthContext(apiKey, apiKey)))
	recordingRulesGroupsSets, err := recordingRulesGroupsSetClient.List(context.Background())
	if err == nil {
		log.Println("Deleting all recording rules")
		for _, recordingRulesGroupsSet := range recordingRulesGroupsSets.GetSets() {
			recordingRulesGroupsSetClient.Delete(context.Background(), &cxsdk.DeleteRuleGroupSetRequest{Id: recordingRulesGroupsSet.GetId()})
		}
	} else {
		log.Fatal("Error listing recording rules:", err)
	}

	// Events2Metrics
	events2metricsClient := cxsdk.NewEvents2MetricsClient(cxsdk.NewCallPropertiesCreator(region, cxsdk.NewAuthContext(apiKey, apiKey)))
	events2metrics, err := events2metricsClient.List(context.Background())
	if err == nil {
		log.Println("Deleting all events2metrics")
		for _, events2metric := range events2metrics.GetE2M() {
			events2metricsClient.Delete(context.Background(), &cxsdk.DeleteE2MRequest{Id: events2metric.GetId()})
		}
	} else {
		log.Fatal("Error listing events2metrics:", err)
	}

	// Dashboard folders
	dashboardsFolderClient := cxsdk.NewDashboardsFoldersClient(cxsdk.NewCallPropertiesCreator(region, cxsdk.NewAuthContext(apiKey, apiKey)))
	dashboardsFolders, err := dashboardsFolderClient.List(context.Background())
	if err == nil {
		log.Println("Deleting all dashboard folders")
		for _, dashboardsFolder := range dashboardsFolders.GetFolder() {
			dashboardsFolderClient.Delete(context.Background(), &cxsdk.DeleteDashboardFolderRequest{FolderId: dashboardsFolder.GetId()})
		}
	} else {
		log.Fatal("Error listing dashboard folders:", err)
	}

	// TCO
	tcoPoliciesTracesClient := cxsdk.NewTCOPoliciesClient(cxsdk.NewCallPropertiesCreator(region, cxsdk.NewAuthContext(apiKey, apiKey)))
	tcoPolicies, err := tcoPoliciesTracesClient.List(context.Background(), &cxsdk.GetCompanyPoliciesRequest{})
	if err == nil {
		log.Println("Deleting all TCO Traces policies")

		for _, tcoPolicy := range tcoPolicies.GetPolicies() {
			tcoPoliciesTracesClient.Delete(context.Background(), &cxsdk.DeletePolicyRequest{Id: tcoPolicy.GetId()})
		}
	} else {
		log.Fatal("Error listing TCO policies:", err)
	}

	tcoPoliciesLogsClient := cxsdk.NewTCOPoliciesClient(cxsdk.NewCallPropertiesCreator(region, cxsdk.NewAuthContext(apiKey, apiKey)))
	tcoPolicies, err = tcoPoliciesLogsClient.List(context.Background(), &cxsdk.GetCompanyPoliciesRequest{})
	if err == nil {
		log.Println("Deleting all TCO Logs policies")

		for _, tcoPolicy := range tcoPolicies.GetPolicies() {
			tcoPoliciesLogsClient.Delete(context.Background(), &cxsdk.DeletePolicyRequest{Id: tcoPolicy.GetId()})
		}
	} else {
		log.Fatal("Error listing TCO Logs policies:", err)
	}
	// Groups
	groupClient := cxsdk.NewGroupsClient(cxsdk.NewCallPropertiesCreator(region, cxsdk.NewAuthContext(apiKey, apiKey)))
	groups, err := groupClient.List(context.Background(), &cxsdk.GetTeamGroupsRequest{})

	if err == nil {
		log.Println("Deleting all groups")

		for _, group := range groups.GetGroups() {
			groupClient.Delete(context.Background(), &cxsdk.DeleteTeamGroupRequest{GroupId: group.GetGroupId()})
		}
	} else {
		log.Fatal("Error listing groups:", err)
	}
	// Connectors
	notificationClient := cxsdk.NewNotificationsClient(cxsdk.NewCallPropertiesCreator(region, cxsdk.NewAuthContext(apiKey, apiKey)))
	listConnectorsRes, err := notificationClient.ListConnectors(context.Background(), &cxsdk.ListConnectorsRequest{})
	if err == nil {
		log.Println("Deleting all connectors")
		for _, connector := range listConnectorsRes.Connectors {
			notificationClient.DeleteConnector(context.Background(), &cxsdk.DeleteConnectorRequest{Id: connector.GetId()})
		}
	} else {
		log.Fatal("Error listing connectors:", err)
	}

	// Presets
	listPresetsRes, err := notificationClient.ListPresetSummaries(context.Background(),
		&cxsdk.ListPresetSummariesRequest{EntityType: cxsdk.EntityTypeAlerts})
	if err == nil {
		log.Println("Deleting all presets")
		for _, preset := range listPresetsRes.PresetSummaries {
			notificationClient.DeleteCustomPreset(context.Background(), &cxsdk.DeleteCustomPresetRequest{Id: preset.GetId()})
		}
	} else {
		log.Fatal("Error listing presets:", err)
	}

	// Global Routers
	listRoutersRes, err := notificationClient.ListGlobalRouters(context.Background(), &cxsdk.ListGlobalRoutersRequest{})
	if err == nil {
		log.Println("Deleting all global routers")
		for _, router := range listRoutersRes.Routers {
			notificationClient.DeleteGlobalRouter(context.Background(), &cxsdk.DeleteGlobalRouterRequest{Id: router.GetId()})
		}
	} else {
		log.Fatal("Error listing global routers:", err)
	}

	// Users
	usersClient := cxsdk.NewUsersClient(cxsdk.NewCallPropertiesCreator(region, cxsdk.NewAuthContext(apiKey, apiKey)))
	users, err := usersClient.List(context.Background())
	if err == nil {
		log.Println("Deleting all users")
		for _, user := range users {
			if user.ID != nil {
				usersClient.Delete(context.Background(), *user.ID)
			}
		}
	} else {
		log.Fatal("Error listing users:", err)
	}

	// Views
	viewsClient := cxsdk.NewViewsClient(cxsdk.NewCallPropertiesCreator(region, cxsdk.NewAuthContext(apiKey, apiKey)))
	views, err := viewsClient.List(context.Background(), &services.ListViewsRequest{})
	if err == nil {
		log.Println("Deleting all views")
		for _, view := range views.Views {
			if view.Id != nil {
				viewsClient.Delete(context.Background(), &cxsdk.DeleteViewRequest{
					Id: view.Id,
				})
			}
		}
	} else {
		log.Fatal("Error listing users:", err)
	}

	// Views
	viewFoldersClient := cxsdk.NewViewFoldersClient(cxsdk.NewCallPropertiesCreator(region, cxsdk.NewAuthContext(apiKey, apiKey)))
	viewFolders, err := viewFoldersClient.List(context.Background())
	if err == nil {
		log.Println("Deleting all viewFolders")
		for _, f := range viewFolders.Folders {
			if f.Id != nil {
				viewFoldersClient.Delete(context.Background(), &cxsdk.DeleteViewFolderRequest{
					Id: f.Id,
				})
			}
		}
	} else {
		log.Fatal("Error listing users:", err)
	}
}
