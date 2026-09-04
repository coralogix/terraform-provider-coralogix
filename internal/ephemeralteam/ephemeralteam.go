// Copyright 2026 Coralogix Ltd.
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

// Package ephemeralteam provisions a disposable Coralogix team for acceptance
// tests that mutate team-wide singleton state (archive settings, TCO policies,
// archive retentions, quota allocation rules, IP access, data enrichments).
//
// Running those tests against the shared CI team means concurrent workflow
// runs clobber each other's state. This harness gives each test its own team:
//
//  1. Create a team in the organization of the CORALOGIX_ORG_API_KEY key.
//  2. Mint a team-scoped API key for the new team.
//  3. Return a `provider "coralogix" {}` HCL block pinned to that key, which
//     the test prepends to its configuration. The plugin-framework provider
//     gives configuration precedence over the CORALOGIX_API_KEY environment
//     variable, so the test runs fully inside the ephemeral team.
//  4. On test success the team is deleted (its daily quota returns to the
//     organization). On failure the team is kept and its ID is logged so the
//     state can be inspected; stale teams are recognizable by the
//     TeamNamePrefix and can be swept by a scheduled cleanup.
//
// The harness is opt-in: when CORALOGIX_ORG_API_KEY is unset, ProviderConfig
// returns an empty string and the test falls back to the shared team exactly
// as before. The org key must carry org-teams:Manage, org-teams:ReadConfig,
// and the ability to create team-level API keys.
//
// Limitation: this only isolates resources served by the plugin-framework
// provider. The legacy SDKv2 provider resolves CORALOGIX_API_KEY before the
// provider block, so SDKv2 resources (e.g. coralogix_enrichment) ignore the
// injected key.
package ephemeralteam

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	cxsdk "github.com/coralogix/coralogix-management-sdk/go"
	cxsdkOpenapi "github.com/coralogix/coralogix-management-sdk/go/openapi/cxsdk"
	apikeysservice "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/api_keys_service"
	teamsservice "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/teams_service"

	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"
)

// OrgAPIKeyEnvVar holds the org-scoped API key that is allowed to create and
// delete teams. The harness is a no-op when it is unset.
const OrgAPIKeyEnvVar = "CORALOGIX_ORG_API_KEY"

// TeamNamePrefix marks teams created by this harness so leaked teams (kept
// after a failure, or orphaned by a killed run) can be found and swept.
const TeamNamePrefix = "tf-acc-ephemeral"

var envAliasToSDKRegion = map[string]string{
	"APAC1":   "AP1",
	"APAC2":   "AP2",
	"APAC3":   "AP3",
	"EUROPE1": "EU1",
	"EUROPE2": "EU2",
	"USA1":    "US1",
	"USA2":    "US2",
	"USA3":    "US3",
}

// DefaultTeamKeyPermissions is granted to the minted team key. It is the
// documented "Legacy Api Key" permission set (see docs/guides/api-keys.md),
// which covers the team-wide configuration surfaces the singleton acceptance
// tests exercise.
var DefaultTeamKeyPermissions = []string{
	"alerts:ReadConfig",
	"alerts:UpdateConfig",
	"cloud-metadata-enrichment:ReadConfig",
	"cloud-metadata-enrichment:UpdateConfig",
	"data-usage:Read",
	"geo-enrichment:ReadConfig",
	"geo-enrichment:UpdateConfig",
	"grafana:Read",
	"grafana:Update",
	"livetail:Read",
	"logs.alerts:ReadConfig",
	"logs.alerts:UpdateConfig",
	"logs.data-setup#low:ReadConfig",
	"logs.data-setup#low:UpdateConfig",
	"logs.events2metrics:ReadConfig",
	"logs.events2metrics:UpdateConfig",
	"logs.tco:ReadPolicies",
	"logs.tco:UpdatePolicies",
	"metrics.alerts:ReadConfig",
	"metrics.alerts:UpdateConfig",
	"metrics.data-analytics#high:Read",
	"metrics.data-analytics#low:Read",
	"metrics.data-setup#high:ReadConfig",
	"metrics.data-setup#high:UpdateConfig",
	"metrics.data-setup#low:ReadConfig",
	"metrics.data-setup#low:UpdateConfig",
	"metrics.recording-rules:ReadConfig",
	"metrics.recording-rules:UpdateConfig",
	"metrics.tco:ReadPolicies",
	"metrics.tco:UpdatePolicies",
	"outbound-webhooks:ReadConfig",
	"outbound-webhooks:UpdateConfig",
	"parsing-rules:ReadConfig",
	"parsing-rules:UpdateConfig",
	"security-enrichment:ReadConfig",
	"security-enrichment:UpdateConfig",
	"serverless:Read",
	"service-catalog:Read",
	"service-catalog:ReadApdexConfig",
	"service-catalog:ReadDimensionsConfig",
	"service-catalog:ReadSLIConfig",
	"service-catalog:Update",
	"service-catalog:UpdateApdexConfig",
	"service-catalog:UpdateDimensionsConfig",
	"service-catalog:UpdateSLIConfig",
	"service-map:Read",
	"source-mapping:UploadMapping",
	"spans.alerts:ReadConfig",
	"spans.alerts:UpdateConfig",
	"spans.data-api#high:ReadData",
	"spans.data-api#low:ReadData",
	"spans.data-setup#low:ReadConfig",
	"spans.data-setup#low:UpdateConfig",
	"spans.events2metrics:ReadConfig",
	"spans.events2metrics:UpdateConfig",
	"spans.tco:ReadPolicies",
	"spans.tco:UpdatePolicies",
	"team-actions:ReadConfig",
	"team-actions:UpdateConfig",
	"team-api-keys-security-settings:Manage",
	"team-api-keys-security-settings:ReadConfig",
	"team-api-keys:Manage",
	"team-api-keys:ReadConfig",
	"team-custom-enrichment:ReadConfig",
	"team-custom-enrichment:ReadData",
	"team-custom-enrichment:UpdateConfig",
	"team-custom-enrichment:UpdateData",
	"team-dashboards:Read",
	"team-dashboards:Update",
	// Beyond the documented legacy set: quota allocation rules and IP access
	// (names verified against the SDK permission catalog).
	"team-ip-access:Manage",
	"team-ip-access:ReadConfig",
	"team-quota-rules:Manage",
	"team-quota-rules:Read",
	"team-quota:Manage",
	"team-quota:Read",
	"user-actions:ReadConfig",
	"user-actions:UpdateConfig",
	"user-dashboards:Read",
	"user-dashboards:Update",
	"version-benchmark-tags:Read",
	"version-benchmark-tags:Update",
}

// Team is a disposable Coralogix team owned by a single acceptance test.
type Team struct {
	ID     int64
	Name   string
	APIKey string
}

// ProviderConfig provisions an ephemeral team for the test and returns the
// provider override block to prepend to every TestStep configuration. It
// returns "" when CORALOGIX_ORG_API_KEY is unset, in which case the test runs
// against the shared team from CORALOGIX_API_KEY as before.
func ProviderConfig(t *testing.T) string {
	t.Helper()
	team := Acquire(t)
	if team == nil {
		return ""
	}
	return team.ProviderConfig()
}

// Acquire creates an ephemeral team plus a team-scoped API key and registers
// a cleanup that deletes the team only when the test passed. It returns nil
// when CORALOGIX_ORG_API_KEY is unset.
func Acquire(t *testing.T) *Team {
	t.Helper()
	// resource.Test skips without TF_ACC, but this helper runs before it —
	// don't provision a team for a test that is about to be skipped.
	if os.Getenv("TF_ACC") == "" {
		return nil
	}
	orgKey := os.Getenv(OrgAPIKeyEnvVar)
	if orgKey == "" {
		return nil
	}

	cs, err := newOrgClientSet(orgKey)
	if err != nil {
		t.Fatalf("ephemeralteam: %s", err)
	}
	ctx := context.Background()

	name := fmt.Sprintf("%s-%d", TeamNamePrefix, time.Now().UnixNano())
	createResp, httpResp, err := cs.Teams().
		TeamServiceCreateTeamInOrg(ctx).
		TeamServiceCreateTeamInOrgRequest(teamsservice.TeamServiceCreateTeamInOrgRequest{
			TeamName: name,
		}).
		Execute()
	if err != nil {
		t.Fatalf("ephemeralteam: creating team %q: %s", name,
			utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResp, err), "Create", nil))
	}
	teamID := createResp.TeamId.GetId()
	t.Logf("ephemeralteam: created team %d (%s)", teamID, name)

	t.Cleanup(func() {
		if t.Failed() {
			t.Logf("ephemeralteam: test failed; keeping team %d (%s) for inspection", teamID, name)
			return
		}
		// Deletion is best-effort: the backend refuses DeleteTeam while the
		// org's quota ledger is being rebalanced (e.g. "DailyQuota must be
		// greater than 0.01" when the org's default team is over its quota).
		// A leaked, prefix-named team is sweepable later; failing a green
		// test over it is not, so retry and then warn instead of erroring.
		var lastErr string
		for attempt := 1; attempt <= 3; attempt++ {
			_, httpResp, err := cs.Teams().TeamServiceDeleteTeam(context.Background(), teamID).Execute()
			if err == nil {
				t.Logf("ephemeralteam: deleted team %d (%s)", teamID, name)
				return
			}
			lastErr = utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResp, err), "Delete", nil)
			time.Sleep(time.Duration(attempt) * 2 * time.Second)
		}
		t.Logf("ephemeralteam: WARNING: could not delete team %d (%s); leaving it for the sweeper: %s",
			teamID, name, lastErr)
	})

	keyName := name + "-key"
	hashed := false
	keyResp, httpResp, err := cs.APIKeys().
		ApiKeysServiceCreateApiKey(ctx).
		CreateApiKeyRequest(apikeysservice.CreateApiKeyRequest{
			Name:   &keyName,
			Hashed: &hashed,
			Owner:  &apikeysservice.Owner{TeamId: &teamID},
			KeyPermissions: &apikeysservice.CreateApiKeyRequestKeyPermissions{
				Permissions: DefaultTeamKeyPermissions,
			},
		}).
		Execute()
	if err != nil {
		t.Fatalf("ephemeralteam: creating API key for team %d: %s", teamID,
			utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResp, err), "Create", nil))
	}
	if keyResp.GetValue() == "" {
		t.Fatalf("ephemeralteam: backend returned an empty API key value for team %d", teamID)
	}

	return &Team{ID: teamID, Name: name, APIKey: keyResp.GetValue()}
}

// ProviderConfig renders the provider override block that pins the Coralogix
// provider to this team's API key.
func (tm *Team) ProviderConfig() string {
	if domain := os.Getenv("CORALOGIX_DOMAIN"); domain != "" {
		return fmt.Sprintf(`provider "coralogix" {
  domain  = %q
  api_key = %q
}

`, domain, tm.APIKey)
	}
	return fmt.Sprintf(`provider "coralogix" {
  env     = %q
  api_key = %q
}

`, strings.ToUpper(os.Getenv("CORALOGIX_ENV")), tm.APIKey)
}

func newOrgClientSet(orgKey string) (*clientset.ClientSet, error) {
	if domain := os.Getenv("CORALOGIX_DOMAIN"); domain != "" {
		grpcTarget, err := clientset.GrpcTargetFromDomain(domain)
		if err != nil {
			return nil, fmt.Errorf("resolving gRPC target for domain %q: %w", domain, err)
		}
		return clientset.NewClientSet(domain, orgKey, grpcTarget), nil
	}
	region := strings.ToUpper(os.Getenv("CORALOGIX_ENV"))
	if region == "" {
		return nil, fmt.Errorf("CORALOGIX_ENV or CORALOGIX_DOMAIN must be set")
	}
	if short, ok := envAliasToSDKRegion[region]; ok {
		region = short
	}
	region = strings.ToLower(region)
	return clientset.NewClientSet(region, orgKey, cxsdk.CoralogixGrpcEndpointFromRegion(region)), nil
}
