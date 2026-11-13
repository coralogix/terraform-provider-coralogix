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

package alerts

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// An alert type is an arm of a oneOf of the API. The overrides file lists the arms by hand in two places,
// and the generator does not notice a new arm that one of them misses:
//   - groups.type_definition.fields: without it the generator stops with an error (RENDERER_SHAPE_UNSUPPORTED);
//   - types.AlertDef.fields.type.requestValueBy: without it the generator passes, and a request of the new
//     type is sent without a type, which the server reads as logs_immediate.
// The enum AlertDefType needs no line for a new value: the generator accepts every value of the contract.
// This test fails for both with the edit to make, so the last one cannot be missed.

type alertOverridesFile struct {
	Groups map[string]struct {
		Fields []string `yaml:"fields"`
	} `yaml:"groups"`
	Types map[string]struct {
		Fields map[string]struct {
			RequestValueBy map[string]string `yaml:"requestValueBy"`
		} `yaml:"fields"`
	} `yaml:"types"`
	Enums map[string]struct {
		Rejected []string `yaml:"rejected"`
	} `yaml:"enums"`
}

var camelBoundary = regexp.MustCompile(`([a-z0-9])([A-Z])`)

// snake turns an API field name into its Terraform name: logsRatioThreshold -> logs_ratio_threshold.
func snake(name string) string {
	return strings.ToLower(camelBoundary.ReplaceAllString(name, "${1}_${2}"))
}

func readAlertOverrides(t *testing.T) alertOverridesFile {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "generated", "alert", "behavior-overrides.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var file alertOverridesFile
	if err := yaml.Unmarshal(raw, &file); err != nil {
		t.Fatal(err)
	}
	return file
}

func TestEveryAlertTypeIsInTheOverrides(t *testing.T) {
	file := readAlertOverrides(t)
	arms := schemaArms(t)

	grouped := map[string]string{} // Terraform name -> API name
	for _, name := range file.Groups["type_definition"].Fields {
		grouped[snake(name)] = name
	}
	byType := file.Types["AlertDef"].Fields["type"].RequestValueBy
	typeValue := map[string]string{} // Terraform name -> enum value
	for name, value := range byType {
		typeValue[snake(name)] = value
	}
	enum := file.Enums["AlertDefType"]

	for arm := range arms {
		if _, ok := grouped[arm]; !ok {
			t.Errorf("alert type %q is not in groups.type_definition.fields of the overrides: add the API name of the arm", arm)
		}
		value, ok := typeValue[arm]
		if !ok {
			t.Errorf("alert type %q has no value in types.AlertDef.fields.type.requestValueBy of the overrides: add `%s: <the AlertDefType value>`. Without it, a request of this type is sent without a type, and the server reads it as logs_immediate", arm, arm)
			continue
		}
		if slices.Contains(enum.Rejected, value) {
			t.Errorf("alert type %q sends %s, which enums.AlertDefType rejects: remove it from rejected", arm, value)
		}
	}
	for arm := range grouped {
		if _, ok := arms[arm]; !ok {
			t.Errorf("groups.type_definition.fields lists %q, which is no alert type of the schema: remove it", grouped[arm])
		}
	}
	for arm := range typeValue {
		if _, ok := arms[arm]; !ok {
			t.Errorf("types.AlertDef.fields.type.requestValueBy lists %q, which is no alert type of the schema: remove it", arm)
		}
	}
	// Two arms must not send the same type.
	seen := map[string]string{}
	for arm, value := range typeValue {
		if other, ok := seen[value]; ok {
			t.Errorf("alert types %q and %q send the same type %s", other, arm, value)
		}
		seen[value] = arm
	}
}
