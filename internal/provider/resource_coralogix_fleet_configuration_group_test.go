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
	"regexp"
	"strings"
	"testing"

	"github.com/coralogix/terraform-provider-coralogix/internal/provider/fleet"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

var (
	fleetCGType = "coralogix_" + fleet.TypeName
	fleetCGName = fleetCGType + ".test"
)

const fleetCGYAMLInline = `receivers:
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

const fleetCGYAMLMultiline = `receivers:
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

// fleetCGYAMLExpanded and fleetCGYAMLMergeKey are the same document; the second
// shares the batch settings through an anchor and a merge key.
const fleetCGYAMLExpanded = `receivers:
  otlp:
    protocols:
      grpc: {}
processors:
  batch:
    timeout: 5s
    send_batch_size: 512
  batch/traces:
    timeout: 5s
    send_batch_size: 512
exporters:
  nop: {}
service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [batch/traces]
      exporters: [nop]
`

const fleetCGYAMLMergeKey = `receivers:
  otlp:
    protocols:
      grpc: {}
processors:
  batch: &batch
    timeout: 5s
    send_batch_size: 512
  batch/traces:
    <<: *batch
exporters:
  nop: {}
service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [batch/traces]
      exporters: [nop]
`

type fleetCGRemote struct {
	name, yaml string
	// selector is the HCL of agent_selector.attributes; empty leaves agent_selector out.
	selector string
}

// fleetCGRaw renders a raw family with only the given attributes set.
// group is extra HCL for the group, active and collectorVersion are left out when empty.
func fleetCGRaw(name, group, active, collectorVersion string, remotes ...fleetCGRemote) string {
	var b strings.Builder
	for _, r := range remotes {
		selector := ""
		if r.selector != "" {
			selector = "\n          agent_selector = { attributes = { " + r.selector + " } }"
		}
		fmt.Fprintf(&b, `
        {
          name              = %q
          raw_configuration = %q%s
        },`, r.name, r.yaml, selector)
	}
	activeLine, cvLine := "", ""
	if active != "" {
		activeLine = "\n    active = " + active
	}
	if collectorVersion != "" {
		cvLine = fmt.Sprintf("\n      collector_version = %q", collectorVersion)
	}
	return fmt.Sprintf(`resource %q "test" {
  name = %q%s

  family = {%s
    raw = {%s
      remote_configurations = [%s
      ]
    }
  }
}
`, fleetCGType, name, group, activeLine, cvLine, b.String())
}

const fleetCGGroupFull = `
  description    = "Acceptance test configuration group"
  tags           = ["tf-acc"]
  priority_order = 10`

func fleetCGPreset(name, integrationVersion string) string {
	return fleetCGPresetGroup(name, integrationVersion, "")
}

// fleetCGPresetGroup renders a preset family; groupFields is extra HCL for the group.
func fleetCGPresetGroup(name, integrationVersion, groupFields string) string {
	iv := ""
	if integrationVersion != "" {
		iv = fmt.Sprintf("\n      integration_version = %q", integrationVersion)
	}
	return fmt.Sprintf(`resource %q "test" {
  name = %q%s

  family = {
    preset = {
      chart_name    = "CHART_NAME_OTEL_INTEGRATION"
      chart_version = "0.0.353"%s
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
`, fleetCGType, name, groupFields, iv)
}

var fleetCGImportIgnore = []string{
	"family.raw.remote_configurations.0.raw_configuration",
	"family.raw.remote_configurations.1.raw_configuration",
	"family.preset.observability_features",
}

func fleetCGTest(t *testing.T, steps ...resource.TestStep) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps:                    steps,
	})
}

// Minimal raw family: no tags, metadata, selector, priority or active.
// Then reformat the YAML, set and change every group field, and import.
func TestAccFleetCGRawMinimal(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-fleet-cg")
	one := fleetCGRemote{name: "default", yaml: fleetCGYAMLInline}
	reformatted := fleetCGRemote{name: "default", yaml: fleetCGYAMLMultiline}
	fleetCGTest(t,
		resource.TestStep{
			Config: fleetCGRaw(name, "", "", "", one),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttrSet(fleetCGName, "family.id"),
				resource.TestCheckNoResourceAttr(fleetCGName, "tags"),
				resource.TestCheckNoResourceAttr(fleetCGName, "family.raw.metadata"),
				resource.TestCheckNoResourceAttr(fleetCGName, "family.raw.remote_configurations.0.agent_selector"),
			),
		},
		resource.TestStep{Config: fleetCGRaw(name, "", "", "", one), PlanOnly: true},
		resource.TestStep{Config: fleetCGRaw(name, "", "", "", reformatted), PlanOnly: true},
		resource.TestStep{Config: fleetCGRaw(name+"-renamed", fleetCGGroupFull, "", "", one)},
		resource.TestStep{Config: fleetCGRaw(name+"-renamed", fleetCGGroupFull, "", "", one), PlanOnly: true},
		resource.TestStep{ResourceName: fleetCGName, ImportState: true, ImportStateVerify: true, ImportStateVerifyIgnore: fleetCGImportIgnore},
	)
}

// tags and metadata omitted, then the YAML rewritten with a merge key: no changes.
func TestAccFleetCGYAMLMergeKey(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-fleet-cg-merge")
	expanded := fleetCGRemote{name: "default", yaml: fleetCGYAMLExpanded}
	merged := fleetCGRemote{name: "default", yaml: fleetCGYAMLMergeKey}
	fleetCGTest(t,
		resource.TestStep{Config: fleetCGRaw(name, "", "", "", expanded),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckNoResourceAttr(fleetCGName, "tags"),
				resource.TestCheckNoResourceAttr(fleetCGName, "family.raw.metadata"),
			)},
		resource.TestStep{RefreshState: true},
		resource.TestStep{Config: fleetCGRaw(name, "", "", "", expanded), PlanOnly: true},
		resource.TestStep{Config: fleetCGRaw(name, "", "", "", merged), PlanOnly: true},
		resource.TestStep{Config: fleetCGRaw(name, "", "", "", merged)},
		resource.TestStep{Config: fleetCGRaw(name, "", "", "", merged), PlanOnly: true},
	)
}

// collector_version without service.version in the selector: the API rejects it.
func TestAccFleetCGCollectorVersionRequiresServiceVersion(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-fleet-cg-cvreq")
	one := fleetCGRemote{name: "default", yaml: fleetCGYAMLInline, selector: `"cx.agent.type" = "agent"`}
	fleetCGTest(t,
		resource.TestStep{
			Config:      fleetCGRaw(name, "", "", "0.114.0", one),
			ExpectError: regexp.MustCompile(`service\.version`),
		},
	)
}

// collector_version with service.version: create, plan, change the version, remove it.
func TestAccFleetCGCollectorVersion(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-fleet-cg-cv")
	v114 := fleetCGRemote{name: "default", yaml: fleetCGYAMLInline, selector: `"cx.agent.type" = "agent", "service.version" = "0.114.0"`}
	v115 := fleetCGRemote{name: "default", yaml: fleetCGYAMLInline, selector: `"cx.agent.type" = "agent", "service.version" = "0.115.0"`}
	plain := fleetCGRemote{name: "default", yaml: fleetCGYAMLInline, selector: `"cx.agent.type" = "agent"`}
	fleetCGTest(t,
		resource.TestStep{Config: fleetCGRaw(name, "", "", "0.114.0", v114)},
		resource.TestStep{Config: fleetCGRaw(name, "", "", "0.114.0", v114), PlanOnly: true},
		resource.TestStep{ResourceName: fleetCGName, ImportState: true, ImportStateVerify: true, ImportStateVerifyIgnore: fleetCGImportIgnore},
		resource.TestStep{Config: fleetCGRaw(name, "", "", "0.115.0", v115)},
		resource.TestStep{Config: fleetCGRaw(name, "", "", "0.115.0", v115), PlanOnly: true},
		resource.TestStep{Config: fleetCGRaw(name, "", "", "", plain)},
		resource.TestStep{Config: fleetCGRaw(name, "", "", "", plain), PlanOnly: true},
	)
}

// Two remote configurations keep their order.
func TestAccFleetCGTwoRemotes(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-fleet-cg-two")
	remotes := []fleetCGRemote{
		{name: "otel-agent", yaml: fleetCGYAMLInline, selector: `"cx.agent.type" = "agent"`},
		{name: "otel-cluster-collector", yaml: fleetCGYAMLInline, selector: `"cx.agent.type" = "cluster-collector"`},
	}
	fleetCGTest(t,
		resource.TestStep{Config: fleetCGRaw(name, "", "", "", remotes...)},
		resource.TestStep{Config: fleetCGRaw(name, "", "", "", remotes...), PlanOnly: true},
		resource.TestStep{ResourceName: fleetCGName, ImportState: true, ImportStateVerify: true, ImportStateVerifyIgnore: fleetCGImportIgnore},
	)
}

// Reordering the remote configurations only: the API keeps its order (no new version).
func TestAccFleetCGReorderRemotes(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-fleet-cg-reorder")
	a := fleetCGRemote{name: "otel-agent", yaml: fleetCGYAMLInline, selector: `"cx.agent.type" = "agent"`}
	b := fleetCGRemote{name: "otel-cluster-collector", yaml: fleetCGYAMLInline, selector: `"cx.agent.type" = "cluster-collector"`}
	bChanged := fleetCGRemote{name: "otel-cluster-collector", yaml: fleetCGYAMLExpanded, selector: `"cx.agent.type" = "cluster-collector"`}
	fleetCGTest(t,
		resource.TestStep{Config: fleetCGRaw(name, "", "", "", a, b)},
		resource.TestStep{Config: fleetCGRaw(name, "", "", "", b, a)},
		resource.TestStep{Config: fleetCGRaw(name, "", "", "", b, a), PlanOnly: true},
		// Reorder back and change one item in the same apply: a new family version.
		resource.TestStep{Config: fleetCGRaw(name, "", "", "", a, bChanged)},
		resource.TestStep{Config: fleetCGRaw(name, "", "", "", a, bChanged), PlanOnly: true},
	)
}

// active false, true, false. The last step leaves it inactive so archive works.
func TestAccFleetCGActive(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-fleet-cg-active")
	one := fleetCGRemote{name: "default", yaml: fleetCGYAMLInline}
	fleetCGTest(t,
		resource.TestStep{Config: fleetCGRaw(name, "", "false", "", one),
			Check: resource.TestCheckResourceAttr(fleetCGName, "family.active", "false")},
		resource.TestStep{Config: fleetCGRaw(name, "", "false", "", one), PlanOnly: true},
		resource.TestStep{Config: fleetCGRaw(name, "", "true", "", one),
			Check: resource.TestCheckResourceAttr(fleetCGName, "family.active", "true")},
		resource.TestStep{Config: fleetCGRaw(name, "", "true", "", one), PlanOnly: true},
		resource.TestStep{Config: fleetCGRaw(name, "", "false", "", one)},
	)
}

// description and family.description set, changed, then removed: they read back absent.
func TestAccFleetCGDescriptionClear(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-fleet-cg-desc")
	one := fleetCGRemote{name: "default", yaml: fleetCGYAMLInline}
	withDesc := func(group, family string) string {
		c := fleetCGRaw(name, "\n  description = \""+group+"\"", "", "", one)
		return strings.Replace(c, "family = {", "family = {\n    description = \""+family+"\"", 1)
	}
	fleetCGTest(t,
		resource.TestStep{Config: withDesc("group one", "family one")},
		resource.TestStep{Config: withDesc("group two", "family two")},
		resource.TestStep{Config: fleetCGRaw(name, "", "", "", one),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckNoResourceAttr(fleetCGName, "description"),
				resource.TestCheckNoResourceAttr(fleetCGName, "family.description"),
			)},
		resource.TestStep{Config: fleetCGRaw(name, "", "", "", one), PlanOnly: true},
	)
}

// priority_order omitted reads 0; set to 5; removed resets to 0.
func TestAccFleetCGPriorityOrder(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-fleet-cg-prio")
	one := fleetCGRemote{name: "default", yaml: fleetCGYAMLInline}
	fleetCGTest(t,
		resource.TestStep{Config: fleetCGRaw(name, "", "", "", one),
			Check: resource.TestCheckResourceAttr(fleetCGName, "priority_order", "0")},
		resource.TestStep{Config: fleetCGRaw(name, "", "", "", one), PlanOnly: true},
		resource.TestStep{Config: fleetCGRaw(name, "\n  priority_order = 5", "", "", one),
			Check: resource.TestCheckResourceAttr(fleetCGName, "priority_order", "5")},
		resource.TestStep{Config: fleetCGRaw(name, "", "", "", one),
			Check: resource.TestCheckResourceAttr(fleetCGName, "priority_order", "0")},
	)
}

// Preset with integration_version set: create, plan, import, rename.
func TestAccFleetCGPresetExplicit(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-fleet-cg-presetx")
	fleetCGTest(t,
		resource.TestStep{Config: fleetCGPresetGroup(name, "0.9.0", "")},
		resource.TestStep{Config: fleetCGPresetGroup(name, "0.9.0", ""), PlanOnly: true},
		resource.TestStep{ResourceName: fleetCGName, ImportState: true, ImportStateVerify: true, ImportStateVerifyIgnore: fleetCGImportIgnore},
		resource.TestStep{Config: fleetCGPresetGroup(name+"-renamed", "0.9.0", "")},
	)
}

// Preset without integration_version: the server resolves it.
func TestAccFleetCGPresetDefaultIntegrationVersion(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-fleet-cg-preset")
	other := strings.Replace(fleetCGPreset(name, ""), "kubernetes_events  = true", "kubernetes_events  = false", 1)
	fleetCGTest(t,
		resource.TestStep{Config: fleetCGPreset(name, ""),
			Check: resource.TestCheckResourceAttr(fleetCGName, "family.preset.integration_version", "0.9.0")},
		resource.TestStep{Config: fleetCGPreset(name, ""), PlanOnly: true},
		// Another preset change with integration_version still omitted.
		resource.TestStep{Config: other,
			Check: resource.TestCheckResourceAttr(fleetCGName, "family.preset.integration_version", "0.9.0")},
		resource.TestStep{Config: other, PlanOnly: true},
		// A group-level change only.
		resource.TestStep{Config: strings.Replace(other, `name = "`, `name = "renamed-`, 1)},
		// Set explicitly, then remove the line again.
		resource.TestStep{Config: fleetCGPreset(name, "0.9.0")},
		resource.TestStep{Config: fleetCGPreset(name, "")},
		resource.TestStep{Config: fleetCGPreset(name, ""), PlanOnly: true},
	)
}

// observability_features written as a heredoc with another key order and spacing:
// the same JSON document plans no change.
func TestAccFleetCGPresetJSONReformat(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-fleet-cg-json")
	group := ""
	encoded := fleetCGPresetGroup(name, "0.9.0", group)
	start := strings.Index(encoded, "observability_features = jsonencode(")
	end := strings.Index(encoded[start:], "\n      })\n") + start + len("\n      })")
	heredoc := encoded[:start] + `observability_features = <<-EOT
        {
          "resource_catalog": false,
          "reduce_resources_attributes": true,
          "metrics": {"target_allocator": false, "statsd": false, "kubernetes_extra": {"scrape_all": false, "enabled": false},
                      "kubelet": false, "host": {"enabled": false}, "collector": false, "cluster": false},
          "logs": {"enabled": false},
          "kubernetes_events": true,
          "fleet_management": {"remote_config": false, "enabled": false},
          "coralogix_operator": false,
          "apm": {"span_metrics": {"transform_statements": [], "histogram_buckets": [], "enabled": true},
                  "sampling": {}, "profiling": {"enabled": false}, "ebpf": false, "enabled": true}
        }
      EOT` + encoded[end:]
	fleetCGTest(t,
		resource.TestStep{Config: encoded},
		resource.TestStep{Config: heredoc, PlanOnly: true},
		resource.TestStep{Config: heredoc},
		resource.TestStep{Config: encoded, PlanOnly: true},
	)
}

// Data source reads the group the resource created.
func TestAccFleetCGDataSource(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-fleet-cg-ds")
	one := fleetCGRemote{name: "default", yaml: fleetCGYAMLInline}
	config := fleetCGRaw(name, "", "", "", one) + fmt.Sprintf(`
data %q "test" {
  id = %s.id
}
`, fleetCGType, fleetCGName)
	fleetCGTest(t,
		resource.TestStep{
			Config: config,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttrPair("data."+fleetCGName, "name", fleetCGName, "name"),
				resource.TestCheckResourceAttrPair("data."+fleetCGName, "family.id", fleetCGName, "family.id"),
			),
		},
	)
}
