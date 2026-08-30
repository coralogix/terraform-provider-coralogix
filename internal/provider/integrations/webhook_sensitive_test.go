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

package integrations

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

// The credential-bearing webhook attributes must stay sensitive so their values
// are redacted from plan and apply output.
func TestWebhookCredentialAttributesAreSensitive(t *testing.T) {
	var r WebhookResource
	var resp resource.SchemaResponse
	r.Schema(context.Background(), resource.SchemaRequest{}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("schema: %v", resp.Diagnostics)
	}

	for _, tc := range []struct{ block, attr string }{
		{"custom", "headers"},
		{"pager_duty", "service_key"},
		{"jira", "api_token"},
	} {
		t.Run(tc.block+"."+tc.attr, func(t *testing.T) {
			nested, ok := resp.Schema.Attributes[tc.block].(schema.SingleNestedAttribute)
			if !ok {
				t.Fatalf("%s is not a single nested attribute", tc.block)
			}
			attr, ok := nested.Attributes[tc.attr]
			if !ok {
				t.Fatalf("%s.%s not found", tc.block, tc.attr)
			}
			if !attr.IsSensitive() {
				t.Errorf("%s.%s must be sensitive", tc.block, tc.attr)
			}
		})
	}
}
