// Copyright 2026 Coralogix Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package provider

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

// A validator that names its siblings by a path expression walks the whole configuration for each
// sibling, so validate time grows with the number of objects. The generated schemas run one-of groups
// and sibling rules in a validator of the parent object. This budget counts the path expressions that
// are left, counted by line: each one is at the top level or runs only when its block is set. A new one in a list item is a
// regression: express the rule as a one-of group or a sibling rule (see the skill
// tfgen-sibling-rules-on-parent-validator).
func TestGeneratedSchemaPathValidatorBudget(t *testing.T) {
	budget := map[string]int{
		"dashboard":         12,
		"alert":             0,
		"slov2":             2,
		"tcopolicieslogs":   0,
		"tcopoliciesrum":    0,
		"tcopoliciestraces": 0,
	}
	var names []string
	for name := range budget {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		raw, err := os.ReadFile(filepath.Join("generated", name, "schema.go"))
		if err != nil {
			t.Fatal(err)
		}
		got := 0
		for _, line := range strings.Split(string(raw), "\n") {
			if strings.Contains(line, "path.MatchRelative") {
				got++
			}
		}
		if got > budget[name] {
			t.Errorf("generated/%s/schema.go has %d validators with a path.MatchRelative expression, want at most %d", name, got, budget[name])
		}
	}
}

// Validate must not cost much more than half of plan, as on the released resource. A one-of
// check that walks the whole configuration for each object made it grow faster than plan. The bound is
// 65% (the measured ratio is about 0.45, and 0.7 with a path validator on every widget).
func TestDashboardValidateIsCheaperThanPlan(t *testing.T) {
	if testing.Short() {
		t.Skip("timing test")
	}
	var widgets []any
	for _, b := range walkBases() {
		if b.top || len(widgets) == 8 {
			continue
		}
		cfg, _ := b.config(0, map[string]int{})
		layout, _ := cfg["layout"].(j)
		sections, _ := layout["sections"].([]any)
		rows, _ := sections[0].(j)["rows"].([]any)
		widgets = append(widgets, rows[0].(j)["widgets"].([]any)[0])
	}
	if len(widgets) < 8 {
		t.Fatalf("found %d widgets, want 8", len(widgets))
	}
	h := newDashboardHarness(t)
	cfg := h.config(j{"name": "validate speed", "layout": dashboardLayout(widgets...)})
	start := time.Now()
	h.validate(cfg)
	validate := time.Since(start)
	start = time.Now()
	h.plan(h.null(), cfg)
	plan := time.Since(start)
	t.Logf("validate %v, plan %v", validate, plan)
	if validate > plan*65/100 {
		t.Errorf("validate took %v, more than 65%% of plan, %v", validate, plan)
	}
}

// The same bound for the alert, whose schema has 15 alert types in one one-of group.
func TestAlertValidateIsCheaperThanPlan(t *testing.T) {
	if testing.Short() {
		t.Skip("timing test")
	}
	files, err := filepath.Glob(filepath.Join(alertGoldenDir, "scenario_*.json"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no scenario golden: %v", err)
	}
	sort.Strings(files)
	var validate, plan time.Duration
	used := 0
	for _, file := range files {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		steps, err := decodeAlertSteps(raw)
		if err != nil {
			t.Fatal(err)
		}
		for _, step := range steps {
			config, ok := step["config"].(map[string]any)
			if !ok || config == nil {
				continue
			}
			encoded, err := json.Marshal(config)
			if err != nil {
				t.Fatal(err)
			}
			var plain map[string]any
			if err := json.Unmarshal(encoded, &plain); err != nil {
				t.Fatal(err)
			}
			h := newAlertHarness(t)
			cfg := h.config(plain)
			start := time.Now()
			diags := h.validate(cfg)
			validate += time.Since(start)
			if hasError(diags) {
				break
			}
			start = time.Now()
			h.plan(h.null(), cfg)
			plan += time.Since(start)
			used++
			break
		}
		if used == 60 {
			break
		}
	}
	t.Logf("%d configs: validate %v, plan %v", used, validate, plan)
	if used < 20 {
		t.Fatalf("only %d configs validate, want at least 20", used)
	}
	if validate > plan*60/100 {
		t.Errorf("validate took %v, more than 60%% of plan, %v", validate, plan)
	}
}
