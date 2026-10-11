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
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	cfggroups "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/fleet_manager_configuration_groups"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func fleetCGClient(t *testing.T) *cfggroups.FleetManagerConfigurationGroupsAPIService {
	cs, err := testAccAlertClientSet()
	if err != nil {
		t.Fatal(err)
	}
	return cs.ConfigurationGroups()
}

// fleetCGCaptureID stores the group id in *id.
func fleetCGCaptureID(id *string) resource.TestCheckFunc {
	return resource.TestCheckResourceAttrWith(fleetCGName, "id", func(v string) error {
		*id = v
		return nil
	})
}

func fleetCGPresetChart(name, chartVersion, integrationVersion, extra string) string {
	c := fleetCGPreset(name, integrationVersion)
	c = strings.Replace(c, `chart_version = "0.0.353"`, fmt.Sprintf(`chart_version = %q`, chartVersion), 1)
	if extra != "" {
		c = strings.Replace(c, "  name = ", extra+"\n  name = ", 1)
	}
	return c
}

// Changing chart_version with integration_version omitted resolves it again.
func TestAccFleetCGX_PresetChartChange(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-fleet-cgx-chart")
	fleetCGTest(t,
		resource.TestStep{Config: fleetCGPresetChart(name, "0.0.353", "", ""),
			Check: resource.TestCheckResourceAttr(fleetCGName, "family.preset.integration_version", "0.9.0")},
		resource.TestStep{Config: fleetCGPresetChart(name, "0.0.300", "", ""),
			Check: resource.TestCheckResourceAttr(fleetCGName, "family.preset.integration_version", "0.8.0")},
		resource.TestStep{Config: fleetCGPresetChart(name, "0.0.300", "", ""), PlanOnly: true},
		resource.TestStep{Config: fleetCGPresetChart(name, "0.0.353", "", ""),
			Check: resource.TestCheckResourceAttr(fleetCGName, "family.preset.integration_version", "0.9.0")},
		resource.TestStep{Config: fleetCGPresetChart(name, "0.0.353", "", ""), PlanOnly: true},
	)
}

// An explicit integration_version that the new chart does not support is rejected.
func TestAccFleetCGX_PresetChartChangeExplicitIntegrationVersion(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-fleet-cgx-chartiv")
	fleetCGTest(t,
		resource.TestStep{Config: fleetCGPresetChart(name, "0.0.353", "0.9.0", "")},
		resource.TestStep{Config: fleetCGPresetChart(name, "0.0.300", "0.9.0", ""),
			ExpectError: regexp.MustCompile(`(?s)integration_version\s+0\.9\.0\s+is\s+incompatible\s+with\s+chart_version\s+0\.0\.300`)},
	)
}

// Preset metadata: change a value, drop a key.
func TestAccFleetCGX_PresetMetadata(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-fleet-cgx-pmeta")
	base := fleetCGPreset(name, "")
	changed := strings.Replace(base, `ClusterName         = "tf-acc"`, `ClusterName         = "tf-acc-2"`, 1)
	dropped := strings.Replace(changed, "\n        PrivateLinkEnabled  = \"false\"", "", 1)
	fleetCGTest(t,
		resource.TestStep{Config: base,
			Check: resource.TestCheckResourceAttrSet(fleetCGName, "family.preset.remote_configurations.0.raw_configuration")},
		resource.TestStep{Config: changed,
			Check: resource.TestCheckResourceAttr(fleetCGName, "family.preset.metadata.ClusterName", "tf-acc-2")},
		resource.TestStep{Config: changed, PlanOnly: true},
		resource.TestStep{Config: dropped,
			Check: resource.TestCheckNoResourceAttr(fleetCGName, "family.preset.metadata.PrivateLinkEnabled")},
		resource.TestStep{Config: dropped, PlanOnly: true},
	)
}

// Every raw remote configuration operation in turn.
func TestAccFleetCGX_RawRemoteLifecycle(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-fleet-cgx-raw")
	a := fleetCGRemote{name: "a", yaml: fleetCGYAMLInline, selector: `"cx.agent.type" = "agent"`}
	b := fleetCGRemote{name: "b", yaml: fleetCGYAMLInline, selector: `"cx.agent.type" = "cluster-collector"`}
	c := fleetCGRemote{name: "c", yaml: fleetCGYAMLExpanded, selector: `"cx.agent.type" = "gateway"`}
	bRenamed := fleetCGRemote{name: "b2", yaml: fleetCGYAMLInline, selector: `"cx.agent.type" = "cluster-collector"`}
	bSelector := fleetCGRemote{name: "b2", yaml: fleetCGYAMLInline, selector: `"cx.agent.type" = "cluster-collector", "k8s.cluster.name" = "prod"`}
	bNoSelector := fleetCGRemote{name: "b2", yaml: fleetCGYAMLInline}
	withMeta := func(cfg, meta string) string {
		return strings.Replace(cfg, "raw = {", "raw = {\n      metadata = { "+meta+" }", 1)
	}
	steps := []resource.TestStep{}
	add := func(cfg string, check resource.TestCheckFunc) {
		s := resource.TestStep{Config: cfg}
		if check != nil {
			s.Check = check
		}
		steps = append(steps, s, resource.TestStep{Config: cfg, PlanOnly: true})
	}
	add(fleetCGRaw(name, "", "", "", a, b), nil)
	add(fleetCGRaw(name, "", "", "", a, b, c), resource.TestCheckResourceAttr(fleetCGName, "family.raw.remote_configurations.#", "3"))
	add(fleetCGRaw(name, "", "", "", a, c), resource.TestCheckResourceAttr(fleetCGName, "family.raw.remote_configurations.1.name", "c"))
	add(fleetCGRaw(name, "", "", "", a, bRenamed), resource.TestCheckResourceAttr(fleetCGName, "family.raw.remote_configurations.1.name", "b2"))
	add(fleetCGRaw(name, "", "", "", a, bSelector), resource.TestCheckResourceAttr(fleetCGName, "family.raw.remote_configurations.1.agent_selector.attributes.k8s.cluster.name", "prod"))
	add(fleetCGRaw(name, "", "", "", a, bNoSelector), resource.TestCheckNoResourceAttr(fleetCGName, "family.raw.remote_configurations.1.agent_selector"))
	add(withMeta(fleetCGRaw(name, "", "", "", a, bNoSelector), `team = "fleet"`), resource.TestCheckResourceAttr(fleetCGName, "family.raw.metadata.team", "fleet"))
	add(withMeta(fleetCGRaw(name, "", "", "", a, bNoSelector), `team = "fleet2", env = "stg"`), resource.TestCheckResourceAttr(fleetCGName, "family.raw.metadata.env", "stg"))
	add(fleetCGRaw(name, "", "", "", a, bNoSelector), resource.TestCheckNoResourceAttr(fleetCGName, "family.raw.metadata"))
	steps = append(steps, resource.TestStep{ResourceName: fleetCGName, ImportState: true, ImportStateVerify: true, ImportStateVerifyIgnore: fleetCGImportIgnore})
	fleetCGTest(t, steps...)
}

// Tags: add, reorder, explicit empty, remove.
func TestAccFleetCGX_Tags(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-fleet-cgx-tags")
	one := fleetCGRemote{name: "default", yaml: fleetCGYAMLInline}
	tags := func(v string) string { return fleetCGRaw(name, "\n  tags = "+v, "", "", one) }
	fleetCGTest(t,
		resource.TestStep{Config: tags(`["a", "b"]`)},
		resource.TestStep{Config: tags(`["a", "b"]`), PlanOnly: true},
		resource.TestStep{Config: tags(`["b", "a", "c"]`),
			Check: resource.TestCheckResourceAttr(fleetCGName, "tags.0", "b")},
		resource.TestStep{Config: tags(`["b", "a", "c"]`), PlanOnly: true},
		resource.TestStep{Config: tags(`[]`)},
		resource.TestStep{Config: tags(`[]`), PlanOnly: true},
		resource.TestStep{Config: tags(`["x"]`)},
		resource.TestStep{Config: fleetCGRaw(name, "", "", "", one),
			Check: resource.TestCheckNoResourceAttr(fleetCGName, "tags")},
		resource.TestStep{Config: fleetCGRaw(name, "", "", "", one), PlanOnly: true},
	)
}

// Raw to preset is rejected by the API; the error must be clear.
func TestAccFleetCGX_SwitchRawToPreset(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-fleet-cgx-switch")
	one := fleetCGRemote{name: "default", yaml: fleetCGYAMLInline}
	fleetCGTest(t,
		resource.TestStep{Config: fleetCGRaw(name, "", "", "", one)},
		resource.TestStep{Config: fleetCGPreset(name, ""), ExpectError: regexp.MustCompile(`(?i)(400|preset|raw|kind|type)`)},
	)
}

// A change made outside Terraform is detected and reverted.
func TestAccFleetCGX_Drift(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-fleet-cgx-drift")
	one := fleetCGRemote{name: "default", yaml: fleetCGYAMLInline}
	var id string
	cfg := fleetCGRaw(name, "\n  priority_order = 3", "", "", one)
	fleetCGTest(t,
		resource.TestStep{Config: cfg, Check: fleetCGCaptureID(&id)},
		resource.TestStep{
			PreConfig: func() {
				body := cfggroups.ConfigurationGroupServiceUpdateConfigurationGroupRequest{}
				body.SetName(name + "-outside")
				body.SetPriorityOrder(7)
				_, _, err := fleetCGClient(t).ConfigurationGroupServiceUpdateConfigurationGroup(context.Background(), id).
					ConfigurationGroupServiceUpdateConfigurationGroupRequest(body).UpdateMask("name,priorityOrder").Execute()
				if err != nil {
					t.Fatalf("out-of-band update: %v", err)
				}
			},
			Config: cfg,
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction(fleetCGName, plancheck.ResourceActionUpdate)},
			},
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr(fleetCGName, "name", name),
				resource.TestCheckResourceAttr(fleetCGName, "priority_order", "3"),
			),
		},
		resource.TestStep{Config: cfg, PlanOnly: true},
	)
}

// A raw family changed outside Terraform (new family version) is detected and reverted.
func TestAccFleetCGX_DriftFamily(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-fleet-cgx-dfam")
	one := fleetCGRemote{name: "default", yaml: fleetCGYAMLInline}
	var id string
	cfg := fleetCGRaw(name, "", "", "", one)
	fleetCGTest(t,
		resource.TestStep{Config: cfg, Check: fleetCGCaptureID(&id)},
		resource.TestStep{
			PreConfig: func() {
				remote := cfggroups.NewRemoteConfigurationReplace("receivers: {}\n")
				remote.SetName("outside")
				raw := cfggroups.RawConfigurationFamilyUpdate{RemoteConfigurations: []cfggroups.RemoteConfigurationReplace{*remote}}
				family := cfggroups.ConfigurationFamilyUpdate{}
				family.SetRaw(raw)
				body := cfggroups.ConfigurationGroupServiceUpdateConfigurationGroupRequest{}
				body.SetFamily(family)
				_, _, err := fleetCGClient(t).ConfigurationGroupServiceUpdateConfigurationGroup(context.Background(), id).
					ConfigurationGroupServiceUpdateConfigurationGroupRequest(body).UpdateMask("family.raw").Execute()
				if err != nil {
					t.Fatalf("out-of-band update: %v", err)
				}
			},
			Config: cfg,
			Check:  resource.TestCheckResourceAttr(fleetCGName, "family.raw.remote_configurations.0.name", "default"),
		},
		resource.TestStep{Config: cfg, PlanOnly: true},
	)
}

// The group is archived outside Terraform: the read reports it gone and the plan
// creates it again. An archived group keeps its name, so the create conflicts.
func TestAccFleetCGX_ArchivedOutside(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-fleet-cgx-arch")
	one := fleetCGRemote{name: "default", yaml: fleetCGYAMLInline}
	var id string
	cfg := fleetCGRaw(name, "", "", "", one)
	fleetCGTest(t,
		resource.TestStep{Config: cfg, Check: fleetCGCaptureID(&id)},
		resource.TestStep{
			PreConfig: func() {
				if _, _, err := fleetCGClient(t).ConfigurationGroupServiceArchiveConfigurationGroup(context.Background(), id).Execute(); err != nil {
					t.Fatalf("out-of-band archive: %v", err)
				}
			},
			Config: cfg,
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction(fleetCGName, plancheck.ResourceActionCreate)},
			},
			ExpectError: regexp.MustCompile(`(?s)409.*name\s+already\s+exists`),
		},
	)
}

// Invalid input reaches the API and fails with a readable error.
func TestAccFleetCGX_InvalidYAML(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-fleet-cgx-badyaml")
	bad := fleetCGRemote{name: "default", yaml: "receivers: [unclosed\n"}
	fleetCGTest(t, resource.TestStep{Config: fleetCGRaw(name, "", "", "", bad), ExpectError: regexp.MustCompile(`(?i)(400|yaml|invalid)`)})
}

func TestAccFleetCGX_EmptyRemotes(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-fleet-cgx-empty")
	fleetCGTest(t, resource.TestStep{Config: fleetCGRaw(name, "", "", ""), ExpectError: regexp.MustCompile(`(?i)(remote|at least|400)`)})
}

func TestAccFleetCGX_InvalidChart(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-fleet-cgx-badchart")
	cfg := strings.Replace(fleetCGPreset(name, ""), "CHART_NAME_OTEL_INTEGRATION", "CHART_NAME_NOPE", 1)
	fleetCGTest(t, resource.TestStep{Config: cfg, ExpectError: regexp.MustCompile(`(?i)(chart|invalid|400)`)})
}

func TestAccFleetCGX_UnsupportedChartVersion(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-fleet-cgx-badver")
	fleetCGTest(t, resource.TestStep{Config: fleetCGPresetChart(name, "9.9.9", "", ""), ExpectError: regexp.MustCompile(`(?i)(version|unsupported|400)`)})
}

// Two remote configurations with the same name.
func TestAccFleetCGX_DuplicateRemoteNames(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-fleet-cgx-dupremote")
	r1 := fleetCGRemote{name: "same", yaml: fleetCGYAMLInline, selector: `"cx.agent.type" = "agent"`}
	r2 := fleetCGRemote{name: "same", yaml: fleetCGYAMLExpanded, selector: `"cx.agent.type" = "gateway"`}
	fleetCGTest(t,
		resource.TestStep{Config: fleetCGRaw(name, "", "", "", r1, r2)},
		resource.TestStep{Config: fleetCGRaw(name, "", "", "", r1, r2), PlanOnly: true},
	)
}

// Two groups with one name: the second create fails with a conflict.
func TestAccFleetCGX_DuplicateGroupName(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-fleet-cgx-dupname")
	one := fleetCGRemote{name: "default", yaml: fleetCGYAMLInline}
	first := fleetCGRaw(name, "", "", "", one)
	second := strings.Replace(first, `"test"`, `"second"`, 1)
	fleetCGTest(t, resource.TestStep{Config: first + second, ExpectError: regexp.MustCompile(`(?i)(409|already exists)`)})
}

// Values that are unknown during plan.
func TestAccFleetCGX_UnknownValues(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-fleet-cgx-unknown")
	cfg := fmt.Sprintf(`resource "terraform_data" "yaml" {
  input = %q
}

resource %q "test" {
  name = "%s-${terraform_data.yaml.id}"
  tags = [terraform_data.yaml.id]

  family = {
    raw = {
      remote_configurations = [
        {
          name              = terraform_data.yaml.id
          raw_configuration = terraform_data.yaml.output
        },
      ]
    }
  }
}
`, fleetCGYAMLInline, fleetCGType, name)
	fleetCGTest(t,
		resource.TestStep{Config: cfg},
		resource.TestStep{Config: cfg, PlanOnly: true},
	)
}

// Unicode and negative priority.
func TestAccFleetCGX_Values(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-fleet-cgx-vals")
	one := fleetCGRemote{name: "default", yaml: fleetCGYAMLInline}
	cfg := fleetCGRaw(name, "\n  description = \"Ünïcödé — 日本語 \\\"quoted\\\"\"\n  priority_order = -5", "", "", one)
	fleetCGTest(t,
		resource.TestStep{Config: cfg, Check: resource.TestCheckResourceAttr(fleetCGName, "priority_order", "-5")},
		resource.TestStep{Config: cfg, PlanOnly: true},
	)
}

// Real collector configurations read from files.
func TestAccFleetCGX_FromFiles(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-fleet-cgx-files")
	agent, _ := filepath.Abs("testdata/fleet_configuration_group/otel-agent.yaml")
	cluster, _ := filepath.Abs("testdata/fleet_configuration_group/otel-cluster-collector.yaml")
	cfg := fmt.Sprintf(`resource %q "test" {
  name = %q

  family = {
    active = false
    raw = {
      collector_version = "0.114.0"
      remote_configurations = [
        {
          name              = "otel-agent"
          raw_configuration = file(%q)
          agent_selector    = { attributes = { "cx.agent.type" = "agent", "service.version" = "0.114.0" } }
        },
        {
          name              = "otel-cluster-collector"
          raw_configuration = file(%q)
          agent_selector    = { attributes = { "cx.agent.type" = "cluster-collector", "service.version" = "0.114.0" } }
        },
      ]
    }
  }
}
`, fleetCGType, name, agent, cluster)
	fleetCGTest(t,
		resource.TestStep{Config: cfg},
		resource.TestStep{Config: cfg, PlanOnly: true},
		resource.TestStep{ResourceName: fleetCGName, ImportState: true, ImportStateVerify: true, ImportStateVerifyIgnore: fleetCGImportIgnore},
	)
}

// Data source on a preset group.
func TestAccFleetCGX_DataSourcePreset(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-fleet-cgx-dsp")
	cfg := fleetCGPreset(name, "") + fmt.Sprintf(`
data %q "test" {
  id = %s.id
}
`, fleetCGType, fleetCGName)
	fleetCGTest(t,
		resource.TestStep{Config: cfg,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttrPair("data."+fleetCGName, "family.preset.integration_version", fleetCGName, "family.preset.integration_version"),
				resource.TestCheckResourceAttrPair("data."+fleetCGName, "family.preset.remote_configurations.#", fleetCGName, "family.preset.remote_configurations.#"),
			)},
	)
}

// Destroy a group, then create one with the same name: the archived group keeps
// its name, so the create conflicts.
func TestAccFleetCGX_RecreateSameName(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-fleet-cgx-recreate")
	one := fleetCGRemote{name: "default", yaml: fleetCGYAMLInline}
	cfg := fleetCGRaw(name, "", "", "", one)
	fleetCGTest(t,
		resource.TestStep{Config: cfg},
		resource.TestStep{Config: `# nothing`},
		resource.TestStep{Config: cfg, ExpectError: regexp.MustCompile(`(?s)409.*name\s+already\s+exists`)},
	)
}
