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

package aaa

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	teamGroups "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/team_groups_management_service"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// A group deleted after the last refresh must fail Update with an error. Update must not
// remove state: the Framework then fails with "Missing Resource State After Update".
func TestGroupUpdateErrorsWhenGroupIsGone(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		updateStatus int
	}{
		"update call returns 404":       {updateStatus: http.StatusNotFound},
		"read after update returns 404": {updateStatus: http.StatusOK},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// A missing group returns a specific message. A bare "Not Found: Not Found" means a
			// missing route, which cxsdk.IsNotFound does not treat as a missing resource.
			const groupNotFound = `{"code":404,"message":"Not Found: team group 123 not found"}`
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.Method != http.MethodGet && tc.updateStatus == http.StatusOK {
					_, _ = w.Write([]byte(`{}`))
					return
				}
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(groupNotFound))
			}))
			defer server.Close()

			cfg := teamGroups.NewConfiguration()
			cfg.Servers = teamGroups.ServerConfigurations{{URL: server.URL}}
			r := &GroupResource{client: teamGroups.NewAPIClient(cfg).TeamGroupsManagementServiceAPI}

			ctx := context.Background()
			var schemaResp resource.SchemaResponse
			r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)

			model := &GroupResourceModel{
				ID:          types.StringValue("123"),
				DisplayName: types.StringValue("group"),
				Members:     types.SetNull(types.StringType),
				Role:        types.StringValue("Read-Only User"),
				ScopeID:     types.StringNull(),
			}
			plan := tfsdk.Plan{Schema: schemaResp.Schema}
			if diags := plan.Set(ctx, model); diags.HasError() {
				t.Fatalf("plan.Set: %v", diags)
			}
			config := tfsdk.Config{Schema: schemaResp.Schema, Raw: plan.Raw}
			priorState := tfsdk.State{Schema: schemaResp.Schema, Raw: plan.Raw}

			resp := &resource.UpdateResponse{State: priorState}
			r.Update(ctx, resource.UpdateRequest{Plan: plan, Config: config, State: priorState}, resp)

			if !resp.Diagnostics.HasError() {
				t.Fatalf("want an error, got diagnostics %v", resp.Diagnostics)
			}
			if got := resp.Diagnostics.Errors()[0].Summary(); got != "Group no longer exists" {
				t.Errorf("error summary = %q: %s", got, resp.Diagnostics.Errors()[0].Detail())
			}
			if resp.State.Raw.Equal(tftypes.NewValue(resp.State.Schema.Type().TerraformType(ctx), nil)) {
				t.Error("Update removed the resource from state")
			}
		})
	}
}

// The same race on coralogix_group_attachment: the group is gone when Update reads its
// members, or when Update writes them.
func TestGroupAttachmentUpdateErrorsWhenGroupIsGone(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		getFinds bool
	}{
		"member read returns 404":  {getFinds: false},
		"member write returns 404": {getFinds: true},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			const groupNotFound = `{"code":404,"message":"Not Found: team group 123 not found"}`
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.Method == http.MethodGet && tc.getFinds {
					_, _ = w.Write([]byte(`{"group":{"name":"group","userIds":["u1"]}}`))
					return
				}
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(groupNotFound))
			}))
			defer server.Close()

			cfg := teamGroups.NewConfiguration()
			cfg.Servers = teamGroups.ServerConfigurations{{URL: server.URL}}
			r := &GroupAttachmentResource{client: teamGroups.NewAPIClient(cfg).TeamGroupsManagementServiceAPI}

			ctx := context.Background()
			var schemaResp resource.SchemaResponse
			r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)

			priorState := tfsdk.State{Schema: schemaResp.Schema}
			if diags := priorState.Set(ctx, &GroupAttachmentResourceModel{GroupID: "123", UserIDs: []string{"u1"}}); diags.HasError() {
				t.Fatalf("state.Set: %v", diags)
			}
			plan := tfsdk.Plan{Schema: schemaResp.Schema}
			if diags := plan.Set(ctx, &GroupAttachmentResourceModel{GroupID: "123", UserIDs: []string{"u1", "u2"}}); diags.HasError() {
				t.Fatalf("plan.Set: %v", diags)
			}

			resp := &resource.UpdateResponse{State: priorState}
			r.Update(ctx, resource.UpdateRequest{Plan: plan, Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: plan.Raw}, State: priorState}, resp)

			if !resp.Diagnostics.HasError() {
				t.Fatalf("want an error, got diagnostics %v", resp.Diagnostics)
			}
			if got := resp.Diagnostics.Errors()[0].Summary(); got != "Group no longer exists" {
				t.Errorf("error summary = %q: %s", got, resp.Diagnostics.Errors()[0].Detail())
			}
			if !resp.State.Raw.Equal(priorState.Raw) {
				t.Error("Update changed the attachment state")
			}
		})
	}
}

// Delete treats a missing group as success, also when the group is deleted between the
// member read and the member write.
func TestGroupAttachmentDeleteSucceedsWhenGroupIsGone(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		getFinds bool
	}{
		"member read returns 404":  {getFinds: false},
		"member write returns 404": {getFinds: true},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			const groupNotFound = `{"code":404,"message":"Not Found: team group 123 not found"}`
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.Method == http.MethodGet && tc.getFinds {
					_, _ = w.Write([]byte(`{"group":{"name":"group","userIds":["u1"]}}`))
					return
				}
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(groupNotFound))
			}))
			defer server.Close()

			cfg := teamGroups.NewConfiguration()
			cfg.Servers = teamGroups.ServerConfigurations{{URL: server.URL}}
			r := &GroupAttachmentResource{client: teamGroups.NewAPIClient(cfg).TeamGroupsManagementServiceAPI}

			ctx := context.Background()
			var schemaResp resource.SchemaResponse
			r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)

			state := tfsdk.State{Schema: schemaResp.Schema}
			if diags := state.Set(ctx, &GroupAttachmentResourceModel{GroupID: "123", UserIDs: []string{"u1"}}); diags.HasError() {
				t.Fatalf("state.Set: %v", diags)
			}

			resp := &resource.DeleteResponse{State: state}
			r.Delete(ctx, resource.DeleteRequest{State: state}, resp)

			if resp.Diagnostics.HasError() {
				t.Errorf("want no error, got %v", resp.Diagnostics)
			}
		})
	}
}
