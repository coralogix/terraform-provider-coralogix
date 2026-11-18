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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	ams "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/metrics_data_archive_service"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/coralogix/terraform-provider-coralogix/internal/provider/generated/archivemetrics"
)

// A Get response holds no storage at all: Read fails and keeps the state, as the released resource
// did. The data source runs the same BeforeFlatten hook.
func TestArchiveMetricsReadWithoutStorageFails(t *testing.T) {
	ctx := context.Background()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tenantConfig":{"prefix":"cx_metrics/","tenantId":42,"disabled":false}}`))
	}))
	defer srv.Close()
	cfg := ams.NewConfiguration()
	cfg.Servers = ams.ServerConfigurations{{URL: srv.URL}}
	cfg.HTTPClient = srv.Client()
	client := ams.NewAPIClient(cfg).MetricsDataArchiveServiceAPI

	r := archivemetrics.NewResourceWithClient(client, ArchiveMetricsHooks())
	var schemaResp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)
	typ := schemaResp.Schema.Type().TerraformType(ctx).(tftypes.Object)
	attrs := map[string]tftypes.Value{}
	for name, at := range typ.AttributeTypes {
		attrs[name] = tftypes.NewValue(at, nil)
	}
	attrs["id"] = tftypes.NewValue(tftypes.String, archivemetrics.SingletonID)
	prior := tfsdk.State{Schema: schemaResp.Schema, Raw: tftypes.NewValue(typ, attrs)}
	resp := &resource.ReadResponse{State: prior}
	r.Read(ctx, resource.ReadRequest{State: prior}, resp)
	if !resp.Diagnostics.HasError() || resp.Diagnostics[0].Summary() != "Error reading coralogix_archive_metrics" {
		t.Fatalf("resource diagnostics = %v, want the unsupported storage error", resp.Diagnostics)
	}
	if resp.State.Raw.IsNull() {
		t.Fatal("Read removed the resource; want it kept with the error")
	}

}

// archiveMetricsFake is a one-company metrics archive: POST stores what it gets, GET returns it.
type archiveMetricsFake struct {
	posted map[string]any
}

func (f *archiveMetricsFake) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method == http.MethodPost {
		_ = json.NewDecoder(r.Body).Decode(&f.posted)
		_, _ = w.Write([]byte(`{}`))
		return
	}
	cfg := map[string]any{"prefix": "cx_metrics/", "tenantId": 42, "disabled": false}
	for k, v := range f.posted {
		cfg[k] = v
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"tenantConfig": cfg})
}

func archiveMetricsResource(t *testing.T, url string, client *http.Client) (resource.Resource, schemaFixture) {
	t.Helper()
	ctx := context.Background()
	cfg := ams.NewConfiguration()
	cfg.Servers = ams.ServerConfigurations{{URL: url}}
	cfg.HTTPClient = client
	r := archivemetrics.NewResourceWithClient(ams.NewAPIClient(cfg).MetricsDataArchiveServiceAPI, ArchiveMetricsHooks())
	var schemaResp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)
	return r, schemaFixture{schema: schemaResp.Schema, typ: schemaResp.Schema.Type().TerraformType(ctx).(tftypes.Object)}
}

type schemaFixture struct {
	schema schema.Schema
	typ    tftypes.Object
}

// value builds an object value of the resource: every attribute is null unless set.
func (s schemaFixture) value(set map[string]tftypes.Value) tftypes.Value {
	attrs := map[string]tftypes.Value{}
	for name, at := range s.typ.AttributeTypes {
		attrs[name] = tftypes.NewValue(at, nil)
	}
	for k, v := range set {
		attrs[k] = v
	}
	return tftypes.NewValue(s.typ, attrs)
}

func (s schemaFixture) block(name string, fields map[string]string) tftypes.Value {
	at := s.typ.AttributeTypes[name].(tftypes.Object)
	vals := map[string]tftypes.Value{}
	for k, at2 := range at.AttributeTypes {
		vals[k] = tftypes.NewValue(at2, nil)
	}
	for k, v := range fields {
		vals[k] = tftypes.NewValue(tftypes.String, v)
	}
	return tftypes.NewValue(at, vals)
}

// A GCS destination is sent as the gcs arm only, and a read shows it back with s3 and ibm null.
func TestArchiveMetricsGcsCreateAndRead(t *testing.T) {
	ctx := context.Background()
	fake := &archiveMetricsFake{}
	srv := httptest.NewServer(fake)
	defer srv.Close()
	r, s := archiveMetricsResource(t, srv.URL, srv.Client())

	gcs := s.block("gcs", map[string]string{"bucket": "my-gcs-bucket"})
	plan := tfsdk.Plan{Schema: s.schema, Raw: s.value(map[string]tftypes.Value{
		"id":        tftypes.NewValue(tftypes.String, archivemetrics.SingletonID),
		"gcs":       gcs,
		"prefix":    tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
		"tenant_id": tftypes.NewValue(tftypes.Number, tftypes.UnknownValue),
	})}
	config := tfsdk.Config{Schema: s.schema, Raw: s.value(map[string]tftypes.Value{"gcs": gcs})}
	resp := &resource.CreateResponse{State: tfsdk.State{Schema: s.schema, Raw: s.value(nil)}}
	r.Create(ctx, resource.CreateRequest{Config: config, Plan: plan}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("Create: %v", resp.Diagnostics)
	}

	wantBody := map[string]any{"gcs": map[string]any{"bucket": "my-gcs-bucket"}}
	if !reflect.DeepEqual(fake.posted, wantBody) {
		t.Fatalf("request body = %v, want %v", fake.posted, wantBody)
	}
	var bucket types.String
	resp.Diagnostics.Append(resp.State.GetAttribute(ctx, path.Root("gcs").AtName("bucket"), &bucket)...)
	if bucket.ValueString() != "my-gcs-bucket" {
		t.Fatalf("state gcs.bucket = %q, want my-gcs-bucket", bucket.ValueString())
	}
	for _, arm := range []string{"s3", "ibm"} {
		var obj types.Object
		resp.Diagnostics.Append(resp.State.GetAttribute(ctx, path.Root(arm), &obj)...)
		if !obj.IsNull() {
			t.Fatalf("state %s = %v, want null", arm, obj)
		}
	}

	// A second read of the same archive keeps the same state: no drift.
	readResp := &resource.ReadResponse{State: resp.State}
	r.Read(ctx, resource.ReadRequest{State: resp.State}, readResp)
	if readResp.Diagnostics.HasError() {
		t.Fatalf("Read: %v", readResp.Diagnostics)
	}
	if !readResp.State.Raw.Equal(resp.State.Raw) {
		t.Fatalf("state changed on read:\n got %v\nwant %v", readResp.State.Raw, resp.State.Raw)
	}
}

// gcs stays exclusive with s3 and ibm, and one of the three is required.
func TestArchiveMetricsGcsIsExclusive(t *testing.T) {
	ctx := context.Background()
	r, s := archiveMetricsResource(t, "http://unused.invalid", http.DefaultClient)
	v, ok := r.(resource.ResourceWithValidateConfig)
	if !ok {
		t.Fatal("resource has no ValidateConfig")
	}
	cv := r.(resource.ResourceWithConfigValidators).ConfigValidators(ctx)

	check := func(name string, set map[string]tftypes.Value, wantErr bool) {
		t.Helper()
		req := resource.ValidateConfigRequest{Config: tfsdk.Config{Schema: s.schema, Raw: s.value(set)}}
		resp := &resource.ValidateConfigResponse{}
		v.ValidateConfig(ctx, req, resp)
		for _, c := range cv {
			cr := &resource.ValidateConfigResponse{}
			c.ValidateResource(ctx, req, cr)
			resp.Diagnostics.Append(cr.Diagnostics...)
		}
		if resp.Diagnostics.HasError() != wantErr {
			t.Fatalf("%s: HasError = %v, want %v: %v", name, resp.Diagnostics.HasError(), wantErr, resp.Diagnostics)
		}
	}
	gcs := s.block("gcs", map[string]string{"bucket": "b"})
	s3 := s.block("s3", map[string]string{"bucket": "b", "region": "eu-north-1"})
	check("gcs only", map[string]tftypes.Value{"gcs": gcs}, false)
	check("gcs and s3", map[string]tftypes.Value{"gcs": gcs, "s3": s3}, true)
	check("none", nil, true)
}
