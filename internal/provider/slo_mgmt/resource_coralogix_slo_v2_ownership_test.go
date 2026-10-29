// Copyright 2024 Coralogix Ltd.
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

package slo_mgmt

import (
	"context"
	"encoding/json"
	"testing"

	slos "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/slos_service"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
)

// A request with no ownership tags sends an empty object, which clears the tags of an APM SLO. The
// API keeps them when the field is left out.
func TestClearOwnershipOnReplaceSendsAnEmptyObject(t *testing.T) {
	body := &slos.Slo1{}
	if diags := clearOwnershipOnReplace(context.Background(), tfsdk.Config{}, body); diags.HasError() {
		t.Fatal(diags)
	}
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	tags, ok := got["ownershipTags"].(map[string]any)
	if !ok || len(tags) != 0 {
		t.Fatalf("ownershipTags = %v, want an empty object in %s", got["ownershipTags"], raw)
	}
}

// Configured tags are not touched.
func TestClearOwnershipOnReplaceKeepsConfiguredTags(t *testing.T) {
	tag := slos.SloOwnershipTag{StaticValues: []string{"payments"}}
	body := &slos.Slo1{OwnershipTags: &slos.SloOwnershipTags{Team: &tag}}
	if diags := clearOwnershipOnReplace(context.Background(), tfsdk.Config{}, body); diags.HasError() {
		t.Fatal(diags)
	}
	if body.OwnershipTags.Team == nil || len(body.OwnershipTags.Team.StaticValues) != 1 {
		t.Fatalf("ownership tags = %+v, want the configured team", body.OwnershipTags)
	}
}
