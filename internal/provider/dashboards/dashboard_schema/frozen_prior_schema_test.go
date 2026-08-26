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

package dashboard_schema

import (
	"crypto/sha256"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

// V1 to V3 exist only to decode stored state, so their shape must stay frozen.
// Widening them is easy to do by accident, because several widget schemas come
// from helpers that V4 shares. These fingerprints fail on any change to a prior
// schema's type, including one made through a shared helper.
//
// Update a fingerprint only when a prior schema is deliberately corrected. Print
// the current value with:
//
//	go test ./internal/provider/dashboards/dashboard_schema/ -run TestPriorSchemasStayFrozen -v
func TestPriorSchemasStayFrozen(t *testing.T) {
	for _, tc := range []struct {
		name        string
		schema      schema.Schema
		fingerprint string
	}{
		{"V1", V1(), "659e17d9e7890a963ddb9b2a2935710034ae1766362f992ffe7afbb730d08f97"},
		{"V2", V2(), "fd28c9bb1d61fc109d535e0073766bc03ffc5bc1d0515a251b3547952e0fd7ec"},
		{"V3", V3(), "44e8b3bfc7c5b210ee47e9aff7c15a0e80dbd756b955ecdf800ae8e5aa7cb24c"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := fmt.Sprintf("%x", sha256.Sum256([]byte(tc.schema.Type().TerraformType(t.Context()).String())))
			t.Logf("%s fingerprint = %s", tc.name, got)
			if got != tc.fingerprint {
				t.Fatalf("schema %s changed: fingerprint = %s, want %s", tc.name, got, tc.fingerprint)
			}
		})
	}
}
