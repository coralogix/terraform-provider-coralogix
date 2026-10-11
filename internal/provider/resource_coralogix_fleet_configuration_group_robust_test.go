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
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	cfggroups "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/fleet_manager_configuration_groups"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// fleetCGBigYAML returns a valid collector configuration with n batch processors.
func fleetCGBigYAML(n int) string {
	var b strings.Builder
	b.WriteString("receivers:\n  otlp:\n    protocols:\n      grpc: {}\nprocessors:\n")
	names := make([]string, 0, n)
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "  batch/p%04d:\n    timeout: %ds\n    send_batch_size: %d\n", i, i%60+1, 512+i)
		names = append(names, fmt.Sprintf("batch/p%04d", i))
	}
	b.WriteString("exporters:\n  nop: {}\nservice:\n  pipelines:\n    traces:\n      receivers: [otlp]\n      processors: [" + strings.Join(names, ", ") + "]\n      exporters: [nop]\n")
	return b.String()
}

func fleetCGRemotes(n int, yaml string) []fleetCGRemote {
	out := make([]fleetCGRemote, n)
	for i := range out {
		out[i] = fleetCGRemote{name: fmt.Sprintf("remote-%02d", i), yaml: yaml, selector: fmt.Sprintf(`"cx.agent.type" = "agent-%02d"`, i)}
	}
	return out
}

// Twelve remote configurations: create, change one, drop half, reverse the rest.
func TestAccFleetCGR_ManyRemotes(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-fleet-cgr-many")
	all := fleetCGRemotes(12, fleetCGYAMLInline)
	changed := append([]fleetCGRemote{}, all...)
	changed[5].yaml = fleetCGYAMLExpanded
	half := changed[:6]
	reversed := []fleetCGRemote{half[5], half[4], half[3], half[2], half[1], half[0]}
	fleetCGTest(t,
		resource.TestStep{Config: fleetCGRaw(name, "", "", "", all...),
			Check: resource.TestCheckResourceAttr(fleetCGName, "family.raw.remote_configurations.#", "12")},
		resource.TestStep{Config: fleetCGRaw(name, "", "", "", all...), PlanOnly: true},
		resource.TestStep{ResourceName: fleetCGName, ImportState: true, ImportStateVerify: true,
			ImportStateVerifyIgnore: []string{"family.raw.remote_configurations"}},
		resource.TestStep{Config: fleetCGRaw(name, "", "", "", changed...)},
		resource.TestStep{Config: fleetCGRaw(name, "", "", "", changed...), PlanOnly: true},
		resource.TestStep{Config: fleetCGRaw(name, "", "", "", half...),
			Check: resource.TestCheckResourceAttr(fleetCGName, "family.raw.remote_configurations.#", "6")},
		resource.TestStep{Config: fleetCGRaw(name, "", "", "", reversed...),
			Check: resource.TestCheckResourceAttr(fleetCGName, "family.raw.remote_configurations.0.name", "remote-05")},
		resource.TestStep{Config: fleetCGRaw(name, "", "", "", reversed...), PlanOnly: true},
	)
}

// A large collector configuration (about 300 KB), then a one-line change in it.
func TestAccFleetCGR_LargeYAML(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-fleet-cgr-large")
	big := fleetCGBigYAML(4000)
	if len(big) < 250_000 {
		t.Fatalf("yaml is only %d bytes", len(big))
	}
	bigChanged := strings.Replace(big, "send_batch_size: 512\n", "send_batch_size: 513\n", 1)
	r := fleetCGRemote{name: "big", yaml: big}
	r2 := fleetCGRemote{name: "big", yaml: bigChanged}
	fleetCGTest(t,
		resource.TestStep{Config: fleetCGRaw(name, "", "", "", r)},
		resource.TestStep{Config: fleetCGRaw(name, "", "", "", r), PlanOnly: true},
		resource.TestStep{Config: fleetCGRaw(name, "", "", "", r2)},
		resource.TestStep{Config: fleetCGRaw(name, "", "", "", r2), PlanOnly: true},
	)
}

// YAML features the API may normalize: comments, anchors, block strings, quoting,
// unicode, YAML 1.1 booleans, numbers.
func TestAccFleetCGR_YAMLFeatures(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-fleet-cgr-yamlf")
	yaml := `# collector for tests
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: "0.0.0.0:4317" # quoted
      http: &http
        endpoint: 0.0.0.0:4318
processors:
  attributes/unicode:
    actions:
      - key: "team.名前"
        value: "café — ñ"
        action: upsert
      - key: on_flag
        value: "on"
        action: insert
  batch:
    timeout: 1.5s
    send_batch_size: 1024
  transform:
    log_statements:
      - context: log
        statements:
          - |
            set(attributes["multi"], "line one")
          - 'set(attributes["q"], "it''s")'
exporters:
  nop: {}
service:
  pipelines:
    logs:
      receivers: [otlp]
      processors: [attributes/unicode, batch, transform]
      exporters: [nop]
`
	r := fleetCGRemote{name: "features", yaml: yaml}
	fleetCGTest(t,
		resource.TestStep{Config: fleetCGRaw(name, "", "", "", r)},
		resource.TestStep{Config: fleetCGRaw(name, "", "", "", r), PlanOnly: true},
		resource.TestStep{RefreshState: true},
		resource.TestStep{Config: fleetCGRaw(name, "", "", "", r), PlanOnly: true},
	)
}

// Size limits from the contract: at the limit works.
func TestAccFleetCGR_LimitsAtMax(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-fleet-cgr-lim")
	one := fleetCGRemote{name: "default", yaml: fleetCGYAMLInline}
	cfg := fleetCGRaw(name, fleetCGTags(64)+fleetCGDesc(4096)+"\n  priority_order = 2147483647", "", "", one)
	fleetCGTest(t,
		resource.TestStep{Config: cfg, Check: resource.TestCheckResourceAttr(fleetCGName, "tags.#", "64")},
		resource.TestStep{Config: cfg, PlanOnly: true},
		resource.TestStep{Config: fleetCGRaw(name, "\n  priority_order = -2147483648", "", "", one)},
		resource.TestStep{Config: fleetCGWithMeta(fleetCGRaw(name, "", "", "", one), 64),
			Check: resource.TestCheckResourceAttr(fleetCGName, "family.raw.metadata.%", "64")},
		resource.TestStep{Config: fleetCGWithMeta(fleetCGRaw(name, "", "", "", one), 64), PlanOnly: true},
	)
}

func TestAccFleetCGR_LimitTags65(t *testing.T) {
	t.Skip("the API accepts values over the documented limit")
	name := acctest.RandomWithPrefix("tf-acc-fleet-cgr-l65t")
	fleetCGTest(t, resource.TestStep{Config: fleetCGRaw(name, fleetCGTags(65), "", "", fleetCGRemote{name: "d", yaml: fleetCGYAMLInline}),
		ExpectError: regexp.MustCompile(`(?i)(tags|64|400)`)})
}

func TestAccFleetCGR_LimitMetadata65(t *testing.T) {
	t.Skip("the API accepts values over the documented limit")
	name := acctest.RandomWithPrefix("tf-acc-fleet-cgr-l65m")
	fleetCGTest(t, resource.TestStep{Config: fleetCGWithMeta(fleetCGRaw(name, "", "", "", fleetCGRemote{name: "d", yaml: fleetCGYAMLInline}), 65),
		ExpectError: regexp.MustCompile(`(?i)(metadata|64|400)`)})
}

func TestAccFleetCGR_LimitDescription4097(t *testing.T) {
	t.Skip("the API accepts values over the documented limit")
	name := acctest.RandomWithPrefix("tf-acc-fleet-cgr-l4097")
	fleetCGTest(t, resource.TestStep{Config: fleetCGRaw(name, fleetCGDesc(4097), "", "", fleetCGRemote{name: "d", yaml: fleetCGYAMLInline}),
		ExpectError: regexp.MustCompile(`(?i)(description|4096|400)`)})
}

func TestAccFleetCGR_LimitName257(t *testing.T) {
	t.Skip("the API accepts values over the documented limit")
	fleetCGTest(t, resource.TestStep{Config: fleetCGRaw(strings.Repeat("n", 257), "", "", "", fleetCGRemote{name: "d", yaml: fleetCGYAMLInline}),
		ExpectError: regexp.MustCompile(`(?i)(name|256|400)`)})
}

func fleetCGTags(n int) string {
	v := make([]string, n)
	for i := range v {
		v[i] = fmt.Sprintf("%q", fmt.Sprintf("t%02d", i))
	}
	return "\n  tags = [" + strings.Join(v, ", ") + "]"
}

func fleetCGDesc(n int) string { return "\n  description = \"" + strings.Repeat("d", n) + "\"" }

func fleetCGWithMeta(cfg string, n int) string {
	v := make([]string, n)
	for i := range v {
		v[i] = fmt.Sprintf("k%02d = \"v\"", i)
	}
	return strings.Replace(cfg, "raw = {", "raw = {\n      metadata = { "+strings.Join(v, ", ")+" }", 1)
}

// Special characters in names, tags, selector keys and metadata.
func TestAccFleetCGR_SpecialCharacters(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-fleet-cgr-spec") + " — ünïcode 🚀 \\\"q\\\""
	r := fleetCGRemote{name: "remote with spaces/and-slashes.dots", yaml: fleetCGYAMLInline,
		selector: `"k8s.pod.label/app.kubernetes.io~name" = "my app", "cx.agent.type" = "agent"`}
	cfg := strings.Replace(fleetCGRaw(name, "\n  tags = [\"a b\", \"ü\", \"x:y=z\", \"🚀\"]", "", "", r),
		"raw = {", "raw = {\n      metadata = { \"key with space\" = \"värde\", \"a.b/c\" = \"{json:1}\" }", 1)
	fleetCGTest(t,
		resource.TestStep{Config: cfg},
		resource.TestStep{Config: cfg, PlanOnly: true},
		resource.TestStep{ResourceName: fleetCGName, ImportState: true, ImportStateVerify: true, ImportStateVerifyIgnore: fleetCGImportIgnore},
	)
}

// Renaming to a name another group has fails, and state stays consistent.
func TestAccFleetCGR_RenameConflict(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-fleet-cgr-ren")
	one := fleetCGRemote{name: "default", yaml: fleetCGYAMLInline}
	a := fleetCGRaw(name+"-a", "", "", "", one)
	b := strings.Replace(fleetCGRaw(name+"-b", "", "", "", one), `"test"`, `"other"`, 1)
	bConflict := strings.Replace(fleetCGRaw(name+"-a", "", "", "", one), `"test"`, `"other"`, 1)
	fleetCGTest(t,
		resource.TestStep{Config: a + b},
		resource.TestStep{Config: a + bConflict, ExpectError: regexp.MustCompile(`(?i)(409|already exists)`)},
		resource.TestStep{Config: a + b, PlanOnly: true},
	)
}

// Several groups in one apply with count, then a change to all of them.
func TestAccFleetCGR_CountMany(t *testing.T) {
	t.Skip("archiving several groups at once can return 500 from the API")
	name := acctest.RandomWithPrefix("tf-acc-fleet-cgr-multi")
	cfg := func(desc string) string {
		return fmt.Sprintf(`resource %[1]q "counted" {
  count       = 6
  name        = "%[2]s-c${count.index}"
  description = %[3]q
  family = {
    raw = {
      remote_configurations = [
        { name = "r${count.index}", raw_configuration = %[4]q },
      ]
    }
  }
}

locals {
  shapes = [{ yaml = %[4]q, prio = 1 }, { yaml = %[5]q, prio = 2 }]
}

resource %[1]q "shaped" {
  count          = length(local.shapes)
  name           = "%[2]s-s${count.index}"
  description    = %[3]q
  priority_order = local.shapes[count.index].prio
  family = {
    raw = {
      remote_configurations = [
        { name = "s${count.index}", raw_configuration = local.shapes[count.index].yaml },
      ]
    }
  }
}
`, fleetCGType, name, desc, fleetCGYAMLInline, fleetCGYAMLExpanded)
	}
	fleetCGTest(t,
		resource.TestStep{Config: cfg("one")},
		resource.TestStep{Config: cfg("one"), PlanOnly: true},
		resource.TestStep{Config: cfg("two"),
			Check: resource.TestCheckResourceAttr(fleetCGType+".counted.5", "description", "two")},
		resource.TestStep{Config: cfg("two"), PlanOnly: true},
	)
}

// ignore_changes keeps an out-of-band YAML change out of the plan.
func TestAccFleetCGR_IgnoreChanges(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-fleet-cgr-ign")
	one := fleetCGRemote{name: "default", yaml: fleetCGYAMLInline}
	cfg := strings.Replace(fleetCGRaw(name, "", "", "", one), "\n  family = {", "\n  lifecycle {\n    ignore_changes = [family.raw.remote_configurations]\n  }\n\n  family = {", 1)
	var id string
	fleetCGTest(t,
		resource.TestStep{Config: cfg, Check: fleetCGCaptureID(&id)},
		resource.TestStep{
			PreConfig: func() {
				remote := cfggroups.NewRemoteConfigurationReplace("receivers: {}\n")
				remote.SetName("default")
				raw := cfggroups.RawConfigurationFamilyUpdate{RemoteConfigurations: []cfggroups.RemoteConfigurationReplace{*remote}}
				family := cfggroups.ConfigurationFamilyUpdate{}
				family.SetRaw(raw)
				body := cfggroups.ConfigurationGroupServiceUpdateConfigurationGroupRequest{}
				body.SetFamily(family)
				if _, _, err := fleetCGClient(t).ConfigurationGroupServiceUpdateConfigurationGroup(context.Background(), id).
					ConfigurationGroupServiceUpdateConfigurationGroupRequest(body).UpdateMask("family.raw").Execute(); err != nil {
					t.Fatalf("out-of-band update: %v", err)
				}
			},
			Config:   cfg,
			PlanOnly: true,
		},
	)
}

// The same YAML from a heredoc and from file() plans no change.
func TestAccFleetCGR_HeredocVersusFile(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-fleet-cgr-file")
	path, _ := filepath.Abs("testdata/fleet_configuration_group/otel-agent.yaml")
	asFile := fmt.Sprintf(`resource %q "test" {
  name = %q
  family = {
    raw = {
      remote_configurations = [{ name = "agent", raw_configuration = file(%q) }]
    }
  }
}
`, fleetCGType, name, path)
	asHeredoc := strings.Replace(asFile, fmt.Sprintf("file(%q)", path), "<<-EOT\n"+indent(readTestFile(t, path), "        ")+"        EOT\n", 1)
	fleetCGTest(t,
		resource.TestStep{Config: asFile},
		resource.TestStep{Config: asHeredoc, PlanOnly: true},
	)
}

// The data source reads the state after an update in the same configuration.
func TestAccFleetCGR_DataSourceAfterUpdate(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-fleet-cgr-dsu")
	cfg := func(desc string) string {
		return fleetCGRaw(name, "\n  description = \""+desc+"\"", "", "", fleetCGRemote{name: "default", yaml: fleetCGYAMLInline}) + fmt.Sprintf(`
data %q "test" {
  id         = %s.id
  depends_on = [%s]
}
`, fleetCGType, fleetCGName, fleetCGName)
	}
	fleetCGTest(t,
		resource.TestStep{Config: cfg("first"), Check: resource.TestCheckResourceAttr("data."+fleetCGName, "description", "first")},
		resource.TestStep{Config: cfg("second"), Check: resource.TestCheckResourceAttr("data."+fleetCGName, "description", "second")},
	)
}

// A preset changed outside Terraform is detected and reverted.
func TestAccFleetCGR_PresetDrift(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-fleet-cgr-pdrift")
	cfg := fleetCGPreset(name, "")
	var id string
	fleetCGTest(t,
		resource.TestStep{Config: cfg, Check: fleetCGCaptureID(&id)},
		resource.TestStep{
			PreConfig: func() {
				client := fleetCGClient(t)
				got, _, err := client.ConfigurationGroupServiceGetConfigurationGroup(context.Background(), id).Execute()
				if err != nil {
					t.Fatalf("get: %v", err)
				}
				p := got.Family.Preset
				meta := p.GetMetadata()
				meta["ClusterName"] = "changed-outside"
				up := cfggroups.NewPresetConfigurationFamilyUpdate(*p.ChartName, p.ChartVersion, p.ObservabilityFeatures)
				up.SetMetadata(meta)
				family := cfggroups.ConfigurationFamilyUpdate{}
				family.SetPreset(*up)
				body := cfggroups.ConfigurationGroupServiceUpdateConfigurationGroupRequest{}
				body.SetFamily(family)
				if _, _, err := client.ConfigurationGroupServiceUpdateConfigurationGroup(context.Background(), id).
					ConfigurationGroupServiceUpdateConfigurationGroupRequest(body).UpdateMask("family.preset").Execute(); err != nil {
					t.Fatalf("out-of-band update: %v", err)
				}
			},
			Config: cfg,
			Check:  resource.TestCheckResourceAttr(fleetCGName, "family.preset.metadata.ClusterName", "tf-acc"),
		},
		resource.TestStep{Config: cfg, PlanOnly: true},
	)
}

// Explicit integration_version for the old chart, then removed: resolved again.
func TestAccFleetCGR_PresetIntegrationVersionLifecycle(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-fleet-cgr-iv")
	fleetCGTest(t,
		resource.TestStep{Config: fleetCGPresetChart(name, "0.0.300", "0.8.0", ""),
			Check: resource.TestCheckResourceAttr(fleetCGName, "family.preset.integration_version", "0.8.0")},
		resource.TestStep{Config: fleetCGPresetChart(name, "0.0.353", "0.9.0", ""),
			Check: resource.TestCheckResourceAttr(fleetCGName, "family.preset.integration_version", "0.9.0")},
		resource.TestStep{Config: fleetCGPresetChart(name, "0.0.300", "", ""),
			Check: resource.TestCheckResourceAttr(fleetCGName, "family.preset.integration_version", "0.8.0")},
		resource.TestStep{Config: fleetCGPresetChart(name, "0.0.300", "", ""), PlanOnly: true},
	)
}

// Several kinds of change in one apply.
func TestAccFleetCGR_CombinedChange(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-fleet-cgr-comb")
	a := fleetCGRemote{name: "a", yaml: fleetCGYAMLInline, selector: `"cx.agent.type" = "agent"`}
	b := fleetCGRemote{name: "b", yaml: fleetCGYAMLExpanded}
	aChanged := fleetCGRemote{name: "a", yaml: fleetCGYAMLMultiline, selector: `"cx.agent.type" = "gateway"`}
	start := fleetCGRaw(name, "\n  description = \"one\"\n  tags = [\"x\"]", "true", "", a, b)
	end := strings.Replace(fleetCGRaw(name+"-r", "\n  description = \"two\"\n  tags = [\"y\", \"x\"]\n  priority_order = 9", "true", "", b, aChanged),
		"family = {", "family = {\n    description = \"family two\"", 1)
	final := strings.Replace(end, "active = true", "active = false", 1)
	fleetCGTest(t,
		resource.TestStep{Config: start},
		resource.TestStep{Config: end},
		resource.TestStep{Config: end, PlanOnly: true},
		resource.TestStep{Config: final},
	)
}

// Explicit empty values that the API leaves out on read.
func TestAccFleetCGR_ExplicitEmptyValues(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-fleet-cgr-empty")
	r := fleetCGRemote{name: "default", yaml: fleetCGYAMLInline}
	emptySelector := strings.Replace(fleetCGRaw(name, "", "", "", r), "raw_configuration = ", "agent_selector = { attributes = {} }\n          raw_configuration = ", 1)
	emptyMeta := strings.Replace(fleetCGRaw(name, "", "", "", r), "raw = {", "raw = {\n      metadata = {}", 1)
	fleetCGTest(t,
		resource.TestStep{Config: emptySelector},
		resource.TestStep{Config: emptySelector, PlanOnly: true},
		resource.TestStep{Config: emptyMeta},
		resource.TestStep{Config: emptyMeta, PlanOnly: true},
	)
}

func readTestFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func indent(s, prefix string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	for i, l := range lines {
		lines[i] = prefix + l
	}
	return strings.Join(lines, "\n") + "\n"
}

// fleetCGPresetAny renders a preset family for any chart.
func fleetCGPresetAny(name, chart, chartVersion, integrationVersion, metadata, features string) string {
	iv := ""
	if integrationVersion != "" {
		iv = fmt.Sprintf("\n      integration_version = %q", integrationVersion)
	}
	return fmt.Sprintf(`resource %q "test" {
  name = %q
  family = {
    preset = {
      chart_name             = %q
      chart_version          = %q%s
      metadata               = %s
      observability_features = %q
    }
  }
}
`, fleetCGType, name, chart, chartVersion, iv, metadata, features)
}

const (
	fleetCGStandaloneMeta = `{ ApplicationName = "tf-acc", SubsystemName = "tf-acc", HostRunningOn = "on_premise" }`
	fleetCGECSMeta        = `{ ClusterName = "tf-acc" }`
	fleetCGECSMinimal     = `{"apm":{"transactions":{"enabled":false}},"logs_and_events":{"logs":{"enabled":false}}}`
	fleetCGECSTypical     = `{"apm":{"transactions":{"enabled":true,"wait_duration_seconds":10}},"logs_and_events":{"logs":{"enabled":true}},"metrics":{"enabled":true,"host":{"enabled":true}}}`
)

// ECS on EC2: chart versions on both sides of the integration range, and a features change.
func TestAccFleetCGR_PresetECS(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-fleet-cgr-ecs")
	fleetCGTest(t,
		resource.TestStep{Config: fleetCGPresetAny(name, "CHART_NAME_OTEL_ECS_EC2", "0.0.30", "", fleetCGECSMeta, fleetCGECSMinimal),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr(fleetCGName, "family.preset.integration_version", "0.7.0"),
				resource.TestCheckResourceAttrSet(fleetCGName, "family.preset.collector_version"),
				resource.TestCheckResourceAttrSet(fleetCGName, "family.preset.remote_configurations.0.raw_configuration"),
			)},
		resource.TestStep{Config: fleetCGPresetAny(name, "CHART_NAME_OTEL_ECS_EC2", "0.0.30", "", fleetCGECSMeta, fleetCGECSMinimal), PlanOnly: true},
		resource.TestStep{Config: fleetCGPresetAny(name, "CHART_NAME_OTEL_ECS_EC2", "0.0.40", "", fleetCGECSMeta, fleetCGECSTypical),
			Check: resource.TestCheckResourceAttr(fleetCGName, "family.preset.integration_version", "0.9.0")},
		resource.TestStep{Config: fleetCGPresetAny(name, "CHART_NAME_OTEL_ECS_EC2", "0.0.40", "", fleetCGECSMeta, fleetCGECSTypical), PlanOnly: true},
		resource.TestStep{ResourceName: fleetCGName, ImportState: true, ImportStateVerify: true, ImportStateVerifyIgnore: fleetCGImportIgnore},
	)
}

// Host charts: default and explicit integration versions, chart upgrade.
func TestAccFleetCGR_PresetLinux(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-fleet-cgr-linux")
	c := "CHART_NAME_OTEL_LINUX_STANDALONE"
	fleetCGTest(t,
		resource.TestStep{Config: fleetCGPresetAny(name, c, "0.0.10", "", fleetCGStandaloneMeta, "{}"),
			Check: resource.TestCheckResourceAttr(fleetCGName, "family.preset.integration_version", "0.1.0")},
		resource.TestStep{Config: fleetCGPresetAny(name, c, "0.0.10", "", fleetCGStandaloneMeta, "{}"), PlanOnly: true},
		resource.TestStep{Config: fleetCGPresetAny(name, c, "0.0.20", "0.3.0", fleetCGStandaloneMeta, "{}"),
			Check: resource.TestCheckResourceAttr(fleetCGName, "family.preset.integration_version", "0.3.0")},
		resource.TestStep{Config: fleetCGPresetAny(name, c, "0.0.20", "0.3.0", fleetCGStandaloneMeta, "{}"), PlanOnly: true},
	)
}

func TestAccFleetCGR_PresetWindows(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-fleet-cgr-win")
	c := "CHART_NAME_OTEL_WINDOWS_STANDALONE"
	fleetCGTest(t,
		resource.TestStep{Config: fleetCGPresetAny(name, c, "0.0.10", "", fleetCGStandaloneMeta, "{}"),
			Check: resource.TestCheckResourceAttr(fleetCGName, "family.preset.integration_version", "0.3.0")},
		resource.TestStep{Config: fleetCGPresetAny(name, c, "0.0.10", "0.4.0", fleetCGStandaloneMeta, "{}"),
			Check: resource.TestCheckResourceAttr(fleetCGName, "family.preset.integration_version", "0.4.0")},
		resource.TestStep{Config: fleetCGPresetAny(name, c, "0.0.10", "0.4.0", fleetCGStandaloneMeta, "{}"), PlanOnly: true},
	)
}

func TestAccFleetCGR_PresetMacOS(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-fleet-cgr-mac")
	c := "CHART_NAME_OTEL_MACOS_STANDALONE"
	fleetCGTest(t,
		resource.TestStep{Config: fleetCGPresetAny(name, c, "0.0.25", "", fleetCGStandaloneMeta, "{}"),
			Check: resource.TestCheckResourceAttr(fleetCGName, "family.preset.integration_version", "0.4.0")},
		resource.TestStep{Config: fleetCGPresetAny(name, c, "0.0.25", "", fleetCGStandaloneMeta, "{}"), PlanOnly: true},
		resource.TestStep{Config: fleetCGPresetAny(name, c, "0.0.40", "0.5.0", fleetCGStandaloneMeta, "{}"),
			Check: resource.TestCheckResourceAttr(fleetCGName, "family.preset.integration_version", "0.5.0")},
	)
}

// Missing required metadata for the chart fails with the API's message.
func TestAccFleetCGR_PresetMissingMetadata(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-fleet-cgr-nometa")
	fleetCGTest(t, resource.TestStep{
		Config:      fleetCGPresetAny(name, "CHART_NAME_OTEL_LINUX_STANDALONE", "0.0.10", "", `{ ApplicationName = "x" }`, "{}"),
		ExpectError: regexp.MustCompile(`(?s)missing\s+required\s+metadata`),
	})
}
