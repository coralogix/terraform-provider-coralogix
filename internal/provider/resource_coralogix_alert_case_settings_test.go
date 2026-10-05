// Copyright 2025 Coralogix Ltd.
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

package provider

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"

	alerts "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/alert_definitions_service"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

const caseSettingsSystemPresetId = "preset_system_generic_https_cases_empty"

func TestAccCoralogixResourceAlert_caseSettings(t *testing.T) {
	connectorPrefix := "tf-acc " + uuid.NewString()[:8]
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAlertDestroy(t),
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceAlertCaseSettings(connectorPrefix, "case settings alert", `{
    auto_resolve_mode  = "disabled"
    enrichment_queries = [{ query = "source logs | limit 1" }]
    destinations = [
      {
        connector_id = coralogix_connector.cases_b.id
        condition    = "true"
      },
      {
        connector_id = coralogix_connector.cases_a.id
        condition    = "caseMetadata.notificationReason == 'caseResolved'"
        preset_id    = "`+caseSettingsSystemPresetId+`"
      },
    ]
  }`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(alertResourceName, "case_settings.auto_resolve_mode", "disabled"),
					resource.TestCheckResourceAttr(alertResourceName, "case_settings.enrichment_queries.#", "1"),
					resource.TestCheckResourceAttr(alertResourceName, "case_settings.enrichment_queries.0.query", "source logs | limit 1"),
					resource.TestCheckResourceAttr(alertResourceName, "case_settings.enrichment_queries.0.type", "dataprime"),
					resource.TestCheckResourceAttr(alertResourceName, "case_settings.destinations.#", "2"),
					resource.TestCheckResourceAttrPair(alertResourceName, "case_settings.destinations.0.connector_id", "coralogix_connector.cases_b", "id"),
					resource.TestCheckResourceAttr(alertResourceName, "case_settings.destinations.0.condition", "true"),
					resource.TestCheckNoResourceAttr(alertResourceName, "case_settings.destinations.0.preset_id"),
					resource.TestCheckResourceAttrPair(alertResourceName, "case_settings.destinations.1.connector_id", "coralogix_connector.cases_a", "id"),
					resource.TestCheckResourceAttr(alertResourceName, "case_settings.destinations.1.preset_id", caseSettingsSystemPresetId),
					testAccCheckAlertCaseAutoResolveMode(alertResourceName, alerts.ALERTDEFCASEAUTORESOLVEMODE_ALERT_DEF_CASE_AUTO_RESOLVE_MODE_DISABLED),
				),
			},
			{
				ResourceName:      alertResourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Updating an unrelated field must keep the case settings: a
			// replace that drops them resets auto-resolve to enabled.
			{
				Config: testAccCoralogixResourceAlertCaseSettings(connectorPrefix, "case settings alert renamed", `{
    auto_resolve_mode  = "disabled"
    enrichment_queries = [{ query = "source logs | limit 1" }]
    destinations = [
      {
        connector_id = coralogix_connector.cases_b.id
        condition    = "true"
      },
      {
        connector_id = coralogix_connector.cases_a.id
        condition    = "caseMetadata.notificationReason == 'caseResolved'"
        preset_id    = "`+caseSettingsSystemPresetId+`"
      },
    ]
  }`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(alertResourceName, "name", "case settings alert renamed"),
					resource.TestCheckResourceAttr(alertResourceName, "case_settings.auto_resolve_mode", "disabled"),
					testAccCheckAlertCaseAutoResolveMode(alertResourceName, alerts.ALERTDEFCASEAUTORESOLVEMODE_ALERT_DEF_CASE_AUTO_RESOLVE_MODE_DISABLED),
				),
			},
			{
				Config: testAccCoralogixResourceAlertCaseSettings(connectorPrefix, "case settings alert renamed", `{
    auto_resolve_mode = "enabled"
    destinations = [
      {
        connector_id = coralogix_connector.cases_a.id
        condition    = "caseMetadata.notificationReason == 'caseResolved'"
        preset_id    = "`+caseSettingsSystemPresetId+`"
      },
      {
        connector_id = coralogix_connector.cases_b.id
        condition    = "true"
      },
    ]
  }`),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(alertResourceName, plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(alertResourceName, "case_settings.auto_resolve_mode", "enabled"),
					resource.TestCheckResourceAttr(alertResourceName, "case_settings.enrichment_queries.#", "0"),
					resource.TestCheckResourceAttr(alertResourceName, "case_settings.destinations.#", "2"),
					resource.TestCheckResourceAttrPair(alertResourceName, "case_settings.destinations.0.connector_id", "coralogix_connector.cases_a", "id"),
					resource.TestCheckResourceAttrPair(alertResourceName, "case_settings.destinations.1.connector_id", "coralogix_connector.cases_b", "id"),
					testAccCheckAlertCaseAutoResolveMode(alertResourceName, alerts.ALERTDEFCASEAUTORESOLVEMODE_ALERT_DEF_CASE_AUTO_RESOLVE_MODE_ENABLED),
				),
			},
			// The API stores an object that only holds defaults as absent; an
			// empty block must still converge.
			{
				Config: testAccCoralogixResourceAlertCaseSettings(connectorPrefix, "case settings alert renamed", `{}`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(alertResourceName, "case_settings.auto_resolve_mode", "enabled"),
					resource.TestCheckResourceAttr(alertResourceName, "case_settings.enrichment_queries.#", "0"),
					resource.TestCheckResourceAttr(alertResourceName, "case_settings.destinations.#", "0"),
					testAccCheckAlertCaseAutoResolveMode(alertResourceName, alerts.ALERTDEFCASEAUTORESOLVEMODE_ALERT_DEF_CASE_AUTO_RESOLVE_MODE_ENABLED),
				),
			},
			{
				Config: testAccCoralogixResourceAlertCaseSettings(connectorPrefix, "case settings alert renamed", ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr(alertResourceName, "case_settings.auto_resolve_mode"),
					testAccCheckAlertCaseSettingsAbsent(alertResourceName),
				),
			},
			{
				Config: testAccCoralogixResourceAlertCaseSettings(connectorPrefix, "case settings alert renamed", `{
    auto_resolve_mode = "disabled"
  }`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(alertResourceName, "case_settings.auto_resolve_mode", "disabled"),
					resource.TestCheckResourceAttr(alertResourceName, "case_settings.destinations.#", "0"),
					testAccCheckAlertCaseAutoResolveMode(alertResourceName, alerts.ALERTDEFCASEAUTORESOLVEMODE_ALERT_DEF_CASE_AUTO_RESOLVE_MODE_DISABLED),
				),
			},
		},
	})
}

func TestAccCoralogixDataSourceAlert_caseSettings(t *testing.T) {
	connectorPrefix := "tf-acc " + uuid.NewString()[:8]
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAlertDestroy(t),
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceAlertCaseSettings(connectorPrefix, "case settings data source alert", `{
    auto_resolve_mode = "disabled"
    destinations = [{
      connector_id = coralogix_connector.cases_a.id
      condition    = "true"
      preset_id    = "`+caseSettingsSystemPresetId+`"
    }]
  }`) + `
data "coralogix_alert" "test" {
  id = coralogix_alert.test.id
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(alertDataSourceName, "case_settings.auto_resolve_mode", "disabled"),
					resource.TestCheckResourceAttr(alertDataSourceName, "case_settings.enrichment_queries.#", "0"),
					resource.TestCheckResourceAttr(alertDataSourceName, "case_settings.destinations.#", "1"),
					resource.TestCheckResourceAttrPair(alertDataSourceName, "case_settings.destinations.0.connector_id", "coralogix_connector.cases_a", "id"),
					resource.TestCheckResourceAttr(alertDataSourceName, "case_settings.destinations.0.preset_id", caseSettingsSystemPresetId),
				),
			},
		},
	})
}

func testAccCheckAlertCaseAutoResolveMode(resourceName string, expected alerts.AlertDefCaseAutoResolveMode) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		caseSettings, err := testAccGetAlertCaseSettings(s, resourceName)
		if err != nil {
			return err
		}
		if caseSettings == nil {
			return fmt.Errorf("alert %q has no case settings, want auto-resolve mode %s", resourceName, expected)
		}
		if got := caseSettings.GetAutoResolveMode(); got != expected {
			return fmt.Errorf("alert %q auto-resolve mode = %s, want %s", resourceName, got, expected)
		}
		return nil
	}
}

func testAccCheckAlertCaseSettingsAbsent(resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		caseSettings, err := testAccGetAlertCaseSettings(s, resourceName)
		if err != nil {
			return err
		}
		if caseSettings != nil {
			return fmt.Errorf("alert %q case settings = %+v, want none", resourceName, *caseSettings)
		}
		return nil
	}
}

func testAccGetAlertCaseSettings(s *terraform.State, resourceName string) (*alerts.AlertDefCaseSettings, error) {
	rs, ok := s.RootModule().Resources[resourceName]
	if !ok {
		return nil, fmt.Errorf("resource %q not found in state", resourceName)
	}
	if rs.Primary == nil || rs.Primary.ID == "" {
		return nil, fmt.Errorf("resource %q has no ID in state", resourceName)
	}

	clientSet, err := testAccAlertClientSet()
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, _, err := clientSet.Alerts().AlertDefsServiceGetAlertDef(ctx, rs.Primary.ID).Execute()
	if err != nil {
		return nil, fmt.Errorf("read alert %q: %w", resourceName, err)
	}
	alertDef := resp.GetAlertDef()
	return alertDef.GetAlertDefProperties().CaseSettings, nil
}

// testAccCoralogixResourceAlertCaseSettings renders a logs threshold alert with
// two connectors that support the cases entity type. connectorPrefix keeps the
// connector names unique across parallel tests. An empty caseSettings omits
// the case_settings attribute.
func testAccCoralogixResourceAlertCaseSettings(connectorPrefix, name, caseSettings string) string {
	caseSettingsAttribute := ""
	if caseSettings != "" {
		caseSettingsAttribute = "case_settings = " + caseSettings
	}
	return fmt.Sprintf(`resource "coralogix_connector" "cases_a" {
  type        = "generic_https"
  name        = "%[1]s cases connector a"
  description = "Connector for alert case settings acceptance test"
  connector_config = {
    fields = [
      { field_name = "url", value = "https://webhook.example.com/cases-a" },
      { field_name = "method", value = "POST" },
    ]
  }
  config_overrides = [{
    entity_type = "cases"
    fields      = []
  }]
}

resource "coralogix_connector" "cases_b" {
  type        = "generic_https"
  name        = "%[1]s cases connector b"
  description = "Connector for alert case settings acceptance test"
  connector_config = {
    fields = [
      { field_name = "url", value = "https://webhook.example.com/cases-b" },
      { field_name = "method", value = "POST" },
    ]
  }
  config_overrides = [{
    entity_type = "cases"
    fields      = []
  }]
}

resource "coralogix_alert" "test" {
  name        = %[2]q
  description = "Alert with case settings from terraform"
  enabled     = false
  priority    = "P3"

  type_definition = {
    logs_threshold = {
      logs_filter = {
        simple_filter = {
          lucene_query = "level:ERROR"
        }
      }
      rules = [{
        condition = {
          threshold      = 10
          time_window    = "10_MINUTES"
          condition_type = "MORE_THAN"
        }
        override = {
          priority = "P3"
        }
      }]
    }
  }

  %[3]s
}
`, connectorPrefix, name, caseSettingsAttribute)
}
