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
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
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
					resource.TestCheckResourceAttr(fleetConfigurationGroupResourceName, "family.remote_configuration.0.name", "default"),
					resource.TestCheckResourceAttrSet(fleetConfigurationGroupResourceName, "family.id"),
					resource.TestCheckResourceAttrSet(fleetConfigurationGroupResourceName, "family.remote_configuration.0.hash"),
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
				Config:             testAccCoralogixResourceFleetConfigurationGroupOmitCollectorVersion(name, fleetAccRawConfigInlineList),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
			{
				ResourceName:            fleetConfigurationGroupResourceName,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"family.remote_configuration.0.raw_configuration"},
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
					resource.TestCheckResourceAttr(fleetConfigurationGroupResourceName, "family.remote_configuration.#", "2"),
					resource.TestCheckResourceAttr(fleetConfigurationGroupResourceName, "family.remote_configuration.0.name", "otel-agent"),
					resource.TestCheckResourceAttr(fleetConfigurationGroupResourceName, "family.remote_configuration.0.agent_selector.cx.agent.type", "agent"),
					resource.TestCheckResourceAttr(fleetConfigurationGroupResourceName, "family.remote_configuration.1.name", "otel-cluster-collector"),
					resource.TestCheckResourceAttr(fleetConfigurationGroupResourceName, "family.remote_configuration.1.agent_selector.cx.agent.type", "cluster-collector"),
					resource.TestCheckResourceAttrSet(fleetConfigurationGroupResourceName, "family.id"),
					resource.TestCheckResourceAttrSet(fleetConfigurationGroupResourceName, "family.remote_configuration.0.hash"),
					resource.TestCheckResourceAttrSet(fleetConfigurationGroupResourceName, "family.remote_configuration.1.hash"),
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
					"family.remote_configuration.0.raw_configuration",
					"family.remote_configuration.1.raw_configuration",
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
    active            = true
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
`, name, rawConfiguration)
}

func testAccCoralogixResourceFleetConfigurationGroupFromFile(name, agentYAML, clusterYAML string) string {
	return fmt.Sprintf(`resource "coralogix_fleet_configuration_group" "test" {
  name           = %q
  description    = "Acceptance test configuration group from file"
  tags           = ["tf-acc"]
  priority_order = 10

  family = {
    active            = true
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
`, name, agentYAML, clusterYAML)
}
