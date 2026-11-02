package storagething

import (
	"context"
	"net/http"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"example.com/iac-test-sdk/go/openapi/gen/storage_service"
)

var unknown = struct{}{}

// value converts v to a value of typ: nil is null, unknown is unknown, and a map is an object.
func value(typ tftypes.Type, v any) tftypes.Value {
	switch v := v.(type) {
	case nil:
		return tftypes.NewValue(typ, nil)
	case map[string]any:
		object := typ.(tftypes.Object)
		attrs := map[string]tftypes.Value{}
		for name, at := range object.AttributeTypes {
			attrs[name] = value(at, v[name])
		}
		return tftypes.NewValue(object, attrs)
	}
	if v == unknown {
		return tftypes.NewValue(typ, tftypes.UnknownValue)
	}
	return tftypes.NewValue(typ, v)
}

func setup(t *testing.T, stored *storage_service.StorageConfig, getErr error) (*Resource, frameworkresource.SchemaResponse) {
	t.Helper()
	storage_service.Stored, storage_service.GetErr, storage_service.SetErr, storage_service.Sets = stored, getErr, nil, nil
	r := &Resource{client: &storage_service.StorageServiceAPIService{}}
	var s frameworkresource.SchemaResponse
	r.Schema(context.Background(), frameworkresource.SchemaRequest{}, &s)
	return r, s
}

func create(t *testing.T, r *Resource, s frameworkresource.SchemaResponse) *frameworkresource.CreateResponse {
	t.Helper()
	typ := s.Schema.Type().TerraformType(context.Background())
	config := value(typ, map[string]any{"s3": map[string]any{"bucket": "b"}})
	plan := value(typ, map[string]any{"id": "storage-thing-settings", "s3": map[string]any{"bucket": "b"}, "prefix": unknown})
	resp := &frameworkresource.CreateResponse{State: tfsdk.State{Schema: s.Schema, Raw: value(typ, nil)}}
	r.Create(context.Background(), frameworkresource.CreateRequest{Config: tfsdk.Config{Schema: s.Schema, Raw: config}, Plan: tfsdk.Plan{Schema: s.Schema, Raw: plan}}, resp)
	return resp
}

func state(t *testing.T, st tfsdk.State) StorageConfigModel {
	t.Helper()
	var out StorageConfigModel
	if diags := st.Get(context.Background(), &out); diags.HasError() {
		t.Fatal(diags)
	}
	return out
}

// The Set call returns an empty object, so Create reads the singleton with Get.
func TestCreateReadsBackAfterAnEmptySet(t *testing.T) {
	r, s := setup(t, nil, nil)
	resp := create(t, r, s)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	if len(storage_service.Sets) != 1 || storage_service.Sets[0].S3 == nil || storage_service.Sets[0].S3.Bucket != "b" {
		t.Fatalf("Set calls = %+v, want one with the bucket", storage_service.Sets)
	}
	got := state(t, resp.State)
	if got.Id.ValueString() != "storage-thing-settings" || got.Prefix.ValueString() != "cx_metrics/" || got.S3 == nil || got.S3.Bucket.ValueString() != "b" {
		t.Fatalf("state = %+v, want the fixed id, the bucket, and the prefix that Get returns", got)
	}
}

// When the read after the Set call fails, the singleton is set, so Create keeps the plan with the
// fixed id: the configured storage, and the computed values still unknown. Terraform marks the
// resource as tainted.
func TestCreateKeepsThePlanWhenTheReadFails(t *testing.T) {
	r, s := setup(t, nil, storage_service.APIError{Status: http.StatusInternalServerError, Text: "Internal: unavailable"})
	resp := create(t, r, s)
	if !resp.Diagnostics.HasError() {
		t.Fatal("want an error when the read after the Set call fails")
	}
	if len(storage_service.Sets) != 1 {
		t.Fatalf("Set calls = %d, want 1", len(storage_service.Sets))
	}
	got := state(t, resp.State)
	if got.Id.ValueString() != "storage-thing-settings" {
		t.Fatalf("state id = %q, want the fixed id", got.Id.ValueString())
	}
	if got.S3 == nil || got.S3.Bucket.ValueString() != "b" || got.Ibm != nil {
		t.Fatalf("state = %+v, want the configured S3 storage", got)
	}
	if !got.Prefix.IsUnknown() {
		t.Fatalf("state prefix = %v, want unknown from the plan", got.Prefix)
	}
}

// When the Set call fails, nothing is set, so Create keeps no state.
func TestCreateKeepsNoStateWhenTheSetFails(t *testing.T) {
	r, s := setup(t, nil, nil)
	storage_service.SetErr = storage_service.APIError{Status: http.StatusBadRequest, Text: "Bad Request: Invalid bucket: Cannot write file"}
	resp := create(t, r, s)
	if !resp.Diagnostics.HasError() {
		t.Fatal("want an error when the Set call fails")
	}
	if !resp.State.Raw.IsNull() {
		t.Fatalf("state = %v, want none", resp.State.Raw)
	}
	if storage_service.Stored != nil {
		t.Fatalf("stored = %+v, want nothing set", storage_service.Stored)
	}
}

// A response with the gcs arm, which the resource does not support, is an error. Read keeps the
// resource in the state: the singleton exists.
func TestReadReportsTheUnsupportedArm(t *testing.T) {
	r, s := setup(t, &storage_service.StorageConfig{Gcs: &storage_service.Gcs{Bucket: "g"}}, nil)
	typ := s.Schema.Type().TerraformType(context.Background())
	prior := tfsdk.State{Schema: s.Schema, Raw: value(typ, map[string]any{"id": "storage-thing-settings", "s3": map[string]any{"bucket": "b"}, "prefix": "cx_metrics/"})}
	resp := &frameworkresource.ReadResponse{State: prior}
	r.Read(context.Background(), frameworkresource.ReadRequest{State: prior}, resp)
	if !resp.Diagnostics.HasError() || resp.Diagnostics[0].Summary() != "Unsupported storage" {
		t.Fatalf("diagnostics = %v, want Unsupported storage", resp.Diagnostics)
	}
	if resp.State.Raw.IsNull() {
		t.Fatal("Read removed the resource; want it kept with the error")
	}
}

// A singleton that the API does not find is gone: Read removes it.
func TestReadRemovesAMissingSingleton(t *testing.T) {
	r, s := setup(t, nil, nil)
	typ := s.Schema.Type().TerraformType(context.Background())
	prior := tfsdk.State{Schema: s.Schema, Raw: value(typ, map[string]any{"id": "storage-thing-settings", "s3": map[string]any{"bucket": "b"}, "prefix": "cx_metrics/"})}
	resp := &frameworkresource.ReadResponse{State: prior}
	r.Read(context.Background(), frameworkresource.ReadRequest{State: prior}, resp)
	if resp.Diagnostics.HasError() || !resp.State.Raw.IsNull() {
		t.Fatalf("diagnostics = %v, null state = %t; want the resource removed", resp.Diagnostics, resp.State.Raw.IsNull())
	}
}

// The data source reports the unsupported arm too.
func TestDataSourceReportsTheUnsupportedArm(t *testing.T) {
	setup(t, &storage_service.StorageConfig{Gcs: &storage_service.Gcs{Bucket: "g"}}, nil)
	ctx := context.Background()
	d := &DataSource{resource: Resource{client: &storage_service.StorageServiceAPIService{}}}
	var schemaResp datasource.SchemaResponse
	d.Schema(ctx, datasource.SchemaRequest{}, &schemaResp)
	config := value(schemaResp.Schema.Type().TerraformType(ctx), map[string]any{})
	resp := &datasource.ReadResponse{State: tfsdk.State{Schema: schemaResp.Schema, Raw: config}}
	d.Read(ctx, datasource.ReadRequest{Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: config}}, resp)
	if !resp.Diagnostics.HasError() || resp.Diagnostics[0].Summary() != "Unsupported storage" {
		t.Fatalf("diagnostics = %v, want Unsupported storage", resp.Diagnostics)
	}
}
