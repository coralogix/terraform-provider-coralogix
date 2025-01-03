package main

import (
	"context"
	"log"
	"os"

	cxsdk "github.com/coralogix/coralogix-management-sdk/go"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

var envToGrpcUrl_ = map[string]string{
	"APAC1":   "ng-api-grpc.app.coralogix.in:443",
	"AP1":     "ng-api-grpc.app.coralogix.in:443",
	"APAC2":   "ng-api-grpc.coralogixsg.com:443",
	"AP2":     "ng-api-grpc.coralogixsg.com:443",
	"APAC3":   "ng-api-grpc.ap3.coralogix.com:443",
	"AP3":     "ng-api-grpc.ap3.coralogix.com:443",
	"EUROPE1": "ng-api-grpc.coralogix.com:443",
	"EU1":     "ng-api-grpc.coralogix.com:443",
	"EUROPE2": "ng-api-grpc.eu2.coralogix.com:443",
	"EU2":     "ng-api-grpc.eu2.coralogix.com:443",
	"USA1":    "ng-api-grpc.coralogix.us:443",
	"US1":     "ng-api-grpc.coralogix.us:443",
	"USA2":    "ng-api-grpc.cx498.coralogix.com:443",
	"US2":     "ng-api-grpc.cx498.coralogix.com:443",
}

func main() {
	apiKey := os.Getenv("CORALOGIX_API_KEY")
	region := os.Getenv("CORALOGIX_ENV")
	url := envToGrpcUrl_[region]

	// Dashboards
	dashboardClient := cxsdk.NewDashboardsClient(cxsdk.NewCallPropertiesCreator(url, cxsdk.NewAuthContext(apiKey, apiKey)))
	dashboards, err := dashboardClient.List(context.Background())

	if err == nil {
		log.Println("Deleting all dashboards")
		for _, d := range dashboards.GetItems() {
			dashboardClient.Delete(context.Background(), &cxsdk.DeleteDashboardRequest{DashboardId: d.GetId()})
		}
	} else {
		log.Println("Error listing Dashboards: %v\n", err)
	}

	// Alerts
	alertClient := cxsdk.NewAlertsClient(cxsdk.NewCallPropertiesCreator(url, cxsdk.NewAuthContext(apiKey, apiKey)))
	alerts, err := alertClient.List(context.Background(), &cxsdk.ListAlertDefsRequest{})
	if err == nil {
		log.Println("Deleting all alerts")

		for _, alert := range alerts.GetAlertDefs() {
			alertClient.Delete(context.Background(), &cxsdk.DeleteAlertDefRequest{Id: alert.GetId()})
		}
	} else {
		log.Println("Error listing Alerts: %v\n", err)
	}

	// Scopes
	scopesClient := cxsdk.NewScopesClient(cxsdk.NewCallPropertiesCreator(url, cxsdk.NewAuthContext(apiKey, apiKey)))
	scopes, err := scopesClient.List(context.Background(), &cxsdk.GetTeamScopesRequest{})
	if err == nil {
		log.Println("Deleting all Scopes")
		for _, scope := range scopes.GetScopes() {
			scopesClient.Delete(context.Background(), &cxsdk.DeleteScopeRequest{Id: scope.GetId()})
		}
	} else {
		log.Println("Error listing Scopes: %v\n", err)
	}

	// Custom Roles
	rolesClients := cxsdk.NewRolesClient(cxsdk.NewCallPropertiesCreator(url, cxsdk.NewAuthContext(apiKey, apiKey)))
	roles, err := rolesClients.List(context.Background(), &cxsdk.ListCustomRolesRequest{})
	if err == nil {
		log.Println("Deleting all custom roles")
		for _, role := range roles.GetRoles() {
			rolesClients.Delete(context.Background(), &cxsdk.DeleteRoleRequest{RoleId: role.GetRoleId()})
		}
	} else {
		log.Println("Error listing custom roles: %v\n", err)
	}

	// Enrichments
	enrichmentClient := cxsdk.NewEnrichmentClient(cxsdk.NewCallPropertiesCreator(url, cxsdk.NewAuthContext(apiKey, apiKey)))
	enrichments, err := enrichmentClient.List(context.Background(), &cxsdk.GetEnrichmentsRequest{})
	if err == nil {
		log.Println("Deleting all Enrichments")
		ids := make([]*wrapperspb.UInt32Value, 0)
		for _, enrichment := range enrichments.GetEnrichments() {
			ids = append(ids, wrapperspb.UInt32(enrichment.GetId()))
		}
		enrichmentClient.Delete(context.Background(), &cxsdk.DeleteEnrichmentsRequest{EnrichmentIds: ids})
	} else {
		log.Println("Error listing Enrichments: %v\n", err)
	}

	// DataSets
	dataSetClient := cxsdk.NewDataSetClient(cxsdk.NewCallPropertiesCreator(url, cxsdk.NewAuthContext(apiKey, apiKey)))
	dataSets, err := dataSetClient.List(context.Background(), &cxsdk.ListDataSetsRequest{})
	if err == nil {
		log.Println("Deleting all DataSets")
		for _, enrichment := range dataSets.GetCustomEnrichments() {
			dataSetClient.Delete(context.Background(), &cxsdk.DeleteDataSetRequest{CustomEnrichmentId: wrapperspb.UInt32(enrichment.GetId())})
		}
	} else {
		log.Println("Error listing DataSets: %v\n", err)
	}

	// Webhooks
	webhookClient := cxsdk.NewWebhooksClient(cxsdk.NewCallPropertiesCreator(url, cxsdk.NewAuthContext(apiKey, apiKey)))
	webhooks, err := webhookClient.List(context.Background(), &cxsdk.ListAllOutgoingWebhooksRequest{})
	if err == nil {
		log.Println("Deleting all webhooks")
		for _, webhook := range webhooks.GetDeployed() {
			webhookClient.Delete(context.Background(), &cxsdk.DeleteOutgoingWebhookRequest{Id: webhook.GetId()})
		}
	} else {
		log.Println("Error listing webhooks: %v\n", err)
	}

	// Recording Rules
	recordingRulesGroupsSetClient := cxsdk.NewRecordingRuleGroupSetsClient(cxsdk.NewCallPropertiesCreator(url, cxsdk.NewAuthContext(apiKey, apiKey)))
	recordingRulesGroupsSets, err := recordingRulesGroupsSetClient.List(context.Background())
	if err == nil {
		log.Println("Deleting all recording rules")
		for _, recordingRulesGroupsSet := range recordingRulesGroupsSets.GetSets() {
			recordingRulesGroupsSetClient.Delete(context.Background(), &cxsdk.DeleteRuleGroupSetRequest{Id: recordingRulesGroupsSet.GetId()})
		}
	} else {
		log.Println("Error listing recording rules: %v\n", err)
	}

	// Events2Metrics
	events2metricsClient := cxsdk.NewEvents2MetricsClient(cxsdk.NewCallPropertiesCreator(url, cxsdk.NewAuthContext(apiKey, apiKey)))
	events2metrics, err := events2metricsClient.List(context.Background())
	if err == nil {
		log.Println("Deleting all events2metrics")
		for _, events2metric := range events2metrics.GetE2M() {
			events2metricsClient.Delete(context.Background(), &cxsdk.DeleteE2MRequest{Id: events2metric.GetId()})
		}
	} else {
		log.Println("Error listing events2metrics: %v\n", err)
	}

	// Dashboard folders
	dashboardsFolderClient := cxsdk.NewDashboardsFoldersClient(cxsdk.NewCallPropertiesCreator(url, cxsdk.NewAuthContext(apiKey, apiKey)))
	dashboardsFolders, err := dashboardsFolderClient.List(context.Background())
	if err == nil {
		log.Println("Deleting all dashboard folders")
		for _, dashboardsFolder := range dashboardsFolders.GetFolder() {
			dashboardsFolderClient.Delete(context.Background(), &cxsdk.DeleteDashboardFolderRequest{FolderId: dashboardsFolder.GetId()})
		}
	} else {
		log.Println("Error listing dashboard folders: %v\n", err)
	}

	// TCO
	tcoPoliciesTracesClient := cxsdk.NewTCOPoliciesClient(cxsdk.NewCallPropertiesCreator(url, cxsdk.NewAuthContext(apiKey, apiKey)))
	tcoPolicies, err := tcoPoliciesTracesClient.List(context.Background(), &cxsdk.GetCompanyPoliciesRequest{})
	if err == nil {
		log.Println("Deleting all TCO Traces policies")

		for _, tcoPolicy := range tcoPolicies.GetPolicies() {
			tcoPoliciesTracesClient.Delete(context.Background(), &cxsdk.DeletePolicyRequest{Id: tcoPolicy.GetId()})
		}
	} else {
		log.Println("Error listing TCO policies: %v\n", err)
	}

	tcoPoliciesLogsClient := cxsdk.NewTCOPoliciesClient(cxsdk.NewCallPropertiesCreator(url, cxsdk.NewAuthContext(apiKey, apiKey)))
	tcoPolicies, err = tcoPoliciesLogsClient.List(context.Background(), &cxsdk.GetCompanyPoliciesRequest{})
	if err == nil {
		log.Println("Deleting all TCO Logs policies")

		for _, tcoPolicy := range tcoPolicies.GetPolicies() {
			tcoPoliciesLogsClient.Delete(context.Background(), &cxsdk.DeletePolicyRequest{Id: tcoPolicy.GetId()})
		}
	} else {
		log.Println("Error listing TCO Logs policies: %v\n", err)
	}
	// Groups
	groupClient := cxsdk.NewGroupsClient(cxsdk.NewCallPropertiesCreator(url, cxsdk.NewAuthContext(apiKey, apiKey)))
	groups, err := groupClient.List(context.Background(), &cxsdk.GetTeamGroupsRequest{})

	if err == nil {
		log.Println("Deleting all groups")

		for _, group := range groups.GetGroups() {
			groupClient.Delete(context.Background(), &cxsdk.DeleteTeamGroupRequest{GroupId: group.GetGroupId()})
		}
	} else {
		log.Println("Error listing groups: %v\n", err)
	}
}
