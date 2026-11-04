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
