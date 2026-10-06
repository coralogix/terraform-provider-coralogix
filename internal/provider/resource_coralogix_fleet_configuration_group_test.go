// Copyright 2026 Coralogix Ltd.
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
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

var fleetConfigurationGroupResourceName = "coralogix_fleet_configuration_group.test"

func TestAccCoralogixResourceFleetConfigurationGroup(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-fleet-cg")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceFleetConfigurationGroup(name, fleetAccRawConfigInlineList),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(fleetConfigurationGroupResourceName, "id"),
					resource.TestCheckResourceAttr(fleetConfigurationGroupResourceName, "name", name),
					resource.TestCheckResourceAttr(fleetConfigurationGroupResourceName, "family.raw.remote_configuration.0.name", "default"),
					resource.TestCheckResourceAttrSet(fleetConfigurationGroupResourceName, "family.id"),
					resource.TestCheckResourceAttrSet(fleetConfigurationGroupResourceName, "family.raw.remote_configuration.0.hash"),
				),
			},
			{
				Config:             testAccCoralogixResourceFleetConfigurationGroup(name, fleetAccRawConfigInlineList),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
			{
				Config:             testAccCoralogixResourceFleetConfigurationGroup(name, fleetAccRawConfigMultilineList),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
			{
				// Removing collector_version clears it.
				Config:             testAccCoralogixResourceFleetConfigurationGroupOmitCollectorVersion(name, fleetAccRawConfigInlineList),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
			{
				ResourceName:            fleetConfigurationGroupResourceName,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"family.raw.remote_configuration.0.raw_configuration"},
			},
		},
	})
}

const fleetAccRawConfigInlineList = `receivers:
  otlp:
    protocols:
      grpc: {}
processors:
  batch: {}
exporters:
  nop: {}
service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [batch]
      exporters: [nop]
`

const fleetAccRawConfigMultilineList = `receivers:
  otlp:
    protocols:
      grpc: {}
processors:
  batch: {}
exporters:
  nop: {}
service:
  pipelines:
    traces:
      receivers:
        - otlp
      processors:
        - batch
      exporters:
        - nop
`

func TestAccCoralogixResourceFleetConfigurationGroupFromFile(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-fleet-cg-file")
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	exampleDir := filepath.Join(filepath.Dir(filepath.Dir(wd)), "examples", "resources", "coralogix_fleet_configuration_group")
	agentYAML := filepath.Join(exampleDir, "otel-agent.yaml")
	clusterYAML := filepath.Join(exampleDir, "otel-cluster-collector.yaml")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceFleetConfigurationGroupFromFile(name, agentYAML, clusterYAML),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(fleetConfigurationGroupResourceName, "id"),
					resource.TestCheckResourceAttr(fleetConfigurationGroupResourceName, "name", name),
					resource.TestCheckResourceAttr(fleetConfigurationGroupResourceName, "family.raw.remote_configuration.#", "2"),
					resource.TestCheckResourceAttr(fleetConfigurationGroupResourceName, "family.raw.remote_configuration.0.name", "otel-agent"),
					resource.TestCheckResourceAttr(fleetConfigurationGroupResourceName, "family.raw.remote_configuration.0.agent_selector.cx.agent.type", "agent"),
					resource.TestCheckResourceAttr(fleetConfigurationGroupResourceName, "family.raw.remote_configuration.1.name", "otel-cluster-collector"),
					resource.TestCheckResourceAttr(fleetConfigurationGroupResourceName, "family.raw.remote_configuration.1.agent_selector.cx.agent.type", "cluster-collector"),
					resource.TestCheckResourceAttrSet(fleetConfigurationGroupResourceName, "family.id"),
					resource.TestCheckResourceAttrSet(fleetConfigurationGroupResourceName, "family.raw.remote_configuration.0.hash"),
					resource.TestCheckResourceAttrSet(fleetConfigurationGroupResourceName, "family.raw.remote_configuration.1.hash"),
				),
			},
			{
				Config:             testAccCoralogixResourceFleetConfigurationGroupFromFile(name, agentYAML, clusterYAML),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
			{
				ResourceName:      fleetConfigurationGroupResourceName,
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateVerifyIgnore: []string{
					"family.raw.remote_configuration.0.raw_configuration",
					"family.raw.remote_configuration.1.raw_configuration",
				},
			},
		},
	})
}

func testAccCoralogixResourceFleetConfigurationGroup(name, rawConfiguration string) string {
	return fmt.Sprintf(`resource "coralogix_fleet_configuration_group" "test" {
  name           = %q
  description    = "Acceptance test configuration group"
  tags           = ["tf-acc"]
  priority_order = 10

  family = {
    active = true
    raw = {
      collector_version = "0.114.0"
      remote_configuration = [
        {
          name              = "default"
          raw_configuration = %q
          agent_selector = {
            "cx.agent.type" = "agent"
          }
        }
      ]
    }
  }
}
`, name, rawConfiguration)
}

func testAccCoralogixResourceFleetConfigurationGroupOmitCollectorVersion(name, rawConfiguration string) string {
	return fmt.Sprintf(`resource "coralogix_fleet_configuration_group" "test" {
  name           = %q
  description    = "Acceptance test configuration group"
  tags           = ["tf-acc"]
  priority_order = 10

  family = {
    active = true
    raw = {
      remote_configuration = [
        {
          name              = "default"
          raw_configuration = %q
          agent_selector = {
            "cx.agent.type" = "agent"
          }
        }
      ]
    }
  }
}
`, name, rawConfiguration)
}

func testAccCoralogixResourceFleetConfigurationGroupFromFile(name, agentYAML, clusterYAML string) string {
	return fmt.Sprintf(`resource "coralogix_fleet_configuration_group" "test" {
  name           = %q
  description    = "Acceptance test configuration group from file"
  tags           = ["tf-acc"]
  priority_order = 10

  family = {
    active = true
    raw = {
      collector_version = "0.114.0"
      remote_configuration = [
        {
          name              = "otel-agent"
          raw_configuration = file(%q)
          agent_selector = {
            "cx.agent.type" = "agent"
          }
        },
        {
          name              = "otel-cluster-collector"
          raw_configuration = file(%q)
          agent_selector = {
            "cx.agent.type" = "cluster-collector"
          }
        }
      ]
    }
  }
}
`, name, agentYAML, clusterYAML)
}

func TestAccCoralogixResourceFleetConfigurationGroupPreset(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-fleet-cg-preset")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceFleetConfigurationGroupPreset(name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(fleetConfigurationGroupResourceName, "id"),
					resource.TestCheckResourceAttrSet(fleetConfigurationGroupResourceName, "family.id"),
					resource.TestCheckResourceAttr(fleetConfigurationGroupResourceName, "family.preset.chart_name", "otel_integration"),
					resource.TestCheckResourceAttrSet(fleetConfigurationGroupResourceName, "family.preset.integration_version"),
					resource.TestCheckResourceAttr(fleetConfigurationGroupResourceName, "family.preset.remote_configuration.#", "2"),
					resource.TestCheckResourceAttrSet(fleetConfigurationGroupResourceName, "family.preset.remote_configuration.0.raw_configuration"),
					resource.TestCheckNoResourceAttr(fleetConfigurationGroupResourceName, "family.raw"),
				),
			},
			{
				Config:             testAccCoralogixResourceFleetConfigurationGroupPreset(name),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
			{
				ResourceName:            fleetConfigurationGroupResourceName,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"family.preset.observability_features"},
			},
			{
				// The API cannot switch a family's type in place, so this replaces the group.
				Config: testAccCoralogixResourceFleetConfigurationGroup(name, fleetAccRawConfigInlineList),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(fleetConfigurationGroupResourceName, plancheck.ResourceActionDestroyBeforeCreate),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(fleetConfigurationGroupResourceName, "family.raw.remote_configuration.0.name", "default"),
					resource.TestCheckNoResourceAttr(fleetConfigurationGroupResourceName, "family.preset"),
				),
			},
		},
	})
}

// Known-good otel-integration preset. Bump chart_version when fleet-manager
// stops serving it.
func testAccCoralogixResourceFleetConfigurationGroupPreset(name string) string {
	return fmt.Sprintf(`resource "coralogix_fleet_configuration_group" "test" {
  name = %q

  family = {
    preset = {
      chart_name    = "otel_integration"
      chart_version = "0.0.353"
      metadata = {
        ClusterName         = "tf-acc"
        KubernetesRunningOn = "openshift"
        PrivateLinkEnabled  = "false"
      }
      observability_features = jsonencode({
        apm = {
          enabled   = true
          ebpf      = false
          profiling = { enabled = false }
          sampling  = {}
          span_metrics = {
            enabled              = true
            histogram_buckets    = []
            transform_statements = []
          }
        }
        coralogix_operator = false
        fleet_management   = { enabled = false, remote_config = false }
        kubernetes_events  = true
        logs               = { enabled = false }
        metrics = {
          cluster          = false
          collector        = false
          host             = { enabled = false }
          kubelet          = false
          kubernetes_extra = { enabled = false, scrape_all = false }
          statsd           = false
          target_allocator = false
        }
        reduce_resources_attributes = true
        resource_catalog            = false
      })
    }
  }
}
`, name)
}

func TestAccCoralogixResourceFleetConfigurationGroupFamilyTypeIsExclusive(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-fleet-cg-type")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccCoralogixResourceFleetConfigurationGroupFamilyType(name, true, true),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`(?s)Invalid Attribute Combination`),
			},
			{
				Config:      testAccCoralogixResourceFleetConfigurationGroupFamilyType(name, false, false),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`(?s)Invalid Attribute Combination`),
			},
		},
	})
}

func testAccCoralogixResourceFleetConfigurationGroupFamilyType(name string, withPreset, withRaw bool) string {
	preset, raw := "", ""
	if withPreset {
		preset = `
    preset = {
      chart_name             = "otel_integration"
      chart_version          = "0.0.353"
      observability_features = jsonencode({})
    }`
	}
	if withRaw {
		raw = fmt.Sprintf(`
    raw = {
      remote_configuration = [
        {
          name              = "default"
          raw_configuration = %q
        }
      ]
    }`, fleetAccRawConfigInlineList)
	}
	return fmt.Sprintf(`resource "coralogix_fleet_configuration_group" "test" {
  name = %q

  family = {%s%s
  }
}
`, name, preset, raw)
}
