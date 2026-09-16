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

package metrics

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	ams "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/metrics_data_archive_service"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func strPtr(s string) *string {
	return &s
}

func int64Ptr(i int64) *int64 {
	return &i
}

// TestFlattenStorageConfigErrors covers the defensive error paths in
// flattenStorageConfig. Previously a nil metricConfig or a config with neither
// IBM nor S3 set caused a nil-pointer dereference / nil return that later
// panicked; both must now surface an error diagnostic instead.
func TestFlattenStorageConfigErrors(t *testing.T) {
	ctx := context.Background()

	t.Run("nil_metric_config_returns_error_diagnostic", func(t *testing.T) {
		model, diags := flattenStorageConfig(ctx, nil)
		if !diags.HasError() {
			t.Fatalf("expected an error diagnostic for nil metricConfig, got none")
		}
		if model != nil {
			t.Fatalf("expected nil model on error, got %+v", model)
		}
	})

	t.Run("no_ibm_or_s3_returns_error_diagnostic", func(t *testing.T) {
		// A tenant config with neither IBM nor S3 storage set.
		cfg := &ams.TenantConfigV2{
			TenantId: int64Ptr(42),
			Prefix:   strPtr("some-prefix"),
		}
		model, diags := flattenStorageConfig(ctx, cfg)
		if !diags.HasError() {
			t.Fatalf("expected an error diagnostic for unsupported backend, got none")
		}
		if model != nil {
			t.Fatalf("expected nil model on error, got %+v", model)
		}
	})
}

// TestFlattenStorageConfigHappyPath verifies that the flatten logic for the
// supported IBM and S3 backends is unchanged: it produces a populated model,
// no error diagnostics, and leaves the opposite backend's object null.
func TestFlattenStorageConfigHappyPath(t *testing.T) {
	ctx := context.Background()

	t.Run("ibm", func(t *testing.T) {
		cfg := &ams.TenantConfigV2{
			TenantId: int64Ptr(7),
			Prefix:   strPtr("ibm-prefix"),
			Ibm: &ams.IbmConfigV2{
				Endpoint: strPtr("https://ibm.example.com"),
				Crn:      strPtr("crn:v1:bluemix:public:cos"),
			},
		}
		model, diags := flattenStorageConfig(ctx, cfg)
		if diags.HasError() {
			t.Fatalf("unexpected error diagnostics: %v", diags.Errors())
		}
		if model == nil {
			t.Fatal("expected a populated model, got nil")
		}
		if model.IBM.IsNull() {
			t.Error("expected IBM object to be set")
		}
		if !model.S3.IsNull() {
			t.Error("expected S3 object to be null for an IBM config")
		}
		if model.Prefix.ValueString() != "ibm-prefix" {
			t.Errorf("prefix = %q, want %q", model.Prefix.ValueString(), "ibm-prefix")
		}
		if model.TenantID.ValueInt64() != 7 {
			t.Errorf("tenant id = %d, want 7", model.TenantID.ValueInt64())
		}
	})

	t.Run("s3", func(t *testing.T) {
		cfg := &ams.TenantConfigV2{
			TenantId: int64Ptr(9),
			Prefix:   strPtr("s3-prefix"),
			S3: &ams.S3Config{
				Bucket: strPtr("my-bucket"),
				Region: strPtr("us-east-1"),
			},
		}
		model, diags := flattenStorageConfig(ctx, cfg)
		if diags.HasError() {
			t.Fatalf("unexpected error diagnostics: %v", diags.Errors())
		}
		if model == nil {
			t.Fatal("expected a populated model, got nil")
		}
		if model.S3.IsNull() {
			t.Error("expected S3 object to be set")
		}
		if !model.IBM.IsNull() {
			t.Error("expected IBM object to be null for an S3 config")
		}
		if model.Prefix.ValueString() != "s3-prefix" {
			t.Errorf("prefix = %q, want %q", model.Prefix.ValueString(), "s3-prefix")
		}
		if model.TenantID.ValueInt64() != 9 {
			t.Errorf("tenant id = %d, want 9", model.TenantID.ValueInt64())
		}
	})
}

// TestArchiveMetricsCreateRetainsIDOnFailedGet verifies that when the initial
// ConfigureTenant mutation succeeds (POST 200) but the follow-up
// GetTenantConfig read fails (GET 500), Create still persists a partial state
// carrying the resource ID. Otherwise Terraform would orphan a resource that
// now exists in the backend, and a subsequent apply would try to recreate it.
func TestArchiveMetricsCreateRetainsIDOnFailedGet(t *testing.T) {
	ctx := context.Background()

	// The mutation (POST) succeeds; the follow-up read (GET) fails with 500.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"message":"boom"}`))
		default:
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		}
	}))
	defer srv.Close()

	cfg := ams.NewConfiguration()
	cfg.Servers = ams.ServerConfigurations{{URL: srv.URL}}
	cfg.HTTPClient = srv.Client()
	client := ams.NewAPIClient(cfg).MetricsDataArchiveServiceAPI

	r := &ArchiveMetricsResource{client: client}

	// Build the resource schema so we can construct a valid plan/state.
	schemaResp := &resource.SchemaResponse{}
	r.Schema(ctx, resource.SchemaRequest{}, schemaResp)

	// Seed a plan with an S3 backend so extractArchiveMetrics produces a valid
	// mutation request.
	plan := tfsdk.Plan{Schema: schemaResp.Schema}
	s3Object, diags := types.ObjectValueFrom(ctx, s3ConfigModelAttr(), &S3ConfigModel{
		Bucket: types.StringValue("my-bucket"),
		Region: types.StringValue("us-east-1"),
	})
	if diags.HasError() {
		t.Fatalf("failed to build s3 object: %v", diags.Errors())
	}
	model := &ArchiveMetricsResourceModel{
		ID:              types.StringUnknown(),
		TenantID:        types.Int64Unknown(),
		Prefix:          types.StringUnknown(),
		RetentionPolicy: types.ObjectNull(retentionPolicyModelAttr()),
		IBM:             types.ObjectNull(ibmConfigModelAttr()),
		S3:              s3Object,
	}
	if diags := plan.Set(ctx, model); diags.HasError() {
		t.Fatalf("failed to seed plan: %v", diags.Errors())
	}

	req := resource.CreateRequest{Plan: plan}
	resp := &resource.CreateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	r.Create(ctx, req, resp)

	// The GET failing must surface an error...
	if !resp.Diagnostics.HasError() {
		t.Fatalf("expected an error diagnostic from the failed GET, got none")
	}

	// ...but the state must NOT be null: the partial state carrying the ID must
	// be retained so the resource is tracked.
	if resp.State.Raw.IsNull() {
		t.Fatal("expected non-null state carrying the resource ID, got null state")
	}

	var got ArchiveMetricsResourceModel
	if diags := resp.State.Get(ctx, &got); diags.HasError() {
		t.Fatalf("failed to read back state: %v", diags.Errors())
	}
	if got.ID.ValueString() != RESOURCE_ID_ARCHIVE_METRICS {
		t.Fatalf("state ID = %q, want %q", got.ID.ValueString(), RESOURCE_ID_ARCHIVE_METRICS)
	}
}
