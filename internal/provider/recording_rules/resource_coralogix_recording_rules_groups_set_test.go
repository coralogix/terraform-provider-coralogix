// Copyright 2024 Coralogix Ltd.
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
package recording_rules

import (
	"fmt"
	"testing"
)

// yaml_content is decoded straight into the generated SDK struct, which carries JSON tags but no
// YAML tags, so gopkg.in/yaml.v3 derives each key from the lowercased Go field name. For a
// multi-word field that means the only spelling that binds is the all-lowercase, unseparated one
// (evaluationdelayms), and yaml.v3 drops unrecognized keys silently. This locks the contract that
// the yaml_content description documents; if a future SDK adds YAML tags, the documented key
// changes and this test fails.
func TestDecodeYAMLContentEvaluationDelayKey(t *testing.T) {
	yamlFor := func(key string) string {
		return fmt.Sprintf(`name: Example
groups:
  - name: Foo
    interval: 180
    rules:
      - record: job:http_requests_total:sum
        expr: sum(rate(http_requests_total[5m])) by (job)
        %s: 60000
`, key)
	}

	for _, tc := range []struct {
		key       string
		wantBound bool
	}{
		{"evaluationdelayms", true},
		{"evaluationDelayMs", false},
		{"evaluation_delay_ms", false},
		{"EvaluationDelayMs", false},
		{"EVALUATIONDELAYMS", false},
	} {
		t.Run(tc.key, func(t *testing.T) {
			result, diags := decodeYAMLContent(yamlFor(tc.key))
			if diags.HasError() {
				t.Fatalf("unmarshal failed: %s", diags)
			}
			if len(result.Groups) != 1 || len(result.Groups[0].Rules) != 1 {
				t.Fatalf("got %d groups, want 1 with 1 rule", len(result.Groups))
			}
			got, ok := result.Groups[0].Rules[0].GetEvaluationDelayMsOk()
			if tc.wantBound {
				if !ok || got == nil {
					t.Fatalf("key %q: expected evaluationDelayMs to bind, but it was absent", tc.key)
				}
				if *got != 60000 {
					t.Errorf("key %q: evaluationDelayMs = %d, want 60000", tc.key, *got)
				}
			} else if ok {
				t.Errorf("key %q: expected the key to be dropped silently, but evaluationDelayMs bound to %d", tc.key, *got)
			}
		})
	}
}
