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

package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.yaml.in/yaml/v4"
)

func TestGoldenOutputBatchGet(t *testing.T) {
	input, loadDir := syntheticInput(t)
	input.OpenAPI = []byte(mustRead(t, filepath.Join("testdata", "batchthing.yaml")))
	out := filepath.Join(t.TempDir(), "batchthing")
	options := Options{Resource: "LegacyThing", OutputDir: out, OverridesPath: filepath.Join("testdata", "batchthing-overrides.yaml")}
	if err := generateFromInput(options, input, loadDir); err != nil {
		t.Fatal(err)
	}
	golden := filepath.Join("testdata", "golden", "batchthing")
	if *updateGolden {
		if err := os.RemoveAll(golden); err != nil {
			t.Fatal(err)
		}
		if err := copyDirectory(out, golden); err != nil {
			t.Fatal(err)
		}
	}
	if diff := compareDirectories(golden, out); diff != "" {
		t.Fatalf("batch golden differs; run with -update:\n%s", diff)
	}
	const semantics = `package batchthing
import (
 "context"
 "testing"
 "slices"
 "example.com/iac-test-sdk/go/openapi/gen/legacy_things_service"
 "github.com/hashicorp/terraform-plugin-framework/diag"
 "github.com/hashicorp/terraform-plugin-framework/path"
 "github.com/hashicorp/terraform-plugin-framework/resource"
 "github.com/hashicorp/terraform-plugin-framework/tfsdk"
 "github.com/hashicorp/terraform-plugin-framework/types"
 "github.com/hashicorp/terraform-plugin-go/tftypes"
)
func TestBatchReadSendsOnlyTheStateID(t *testing.T) {
 first, second := "first", "second"
 client := &legacy_things_service.LegacyThingsServiceAPIService{BatchResponse:&legacy_things_service.GetLegacyThingResponse{Things:[]legacy_things_service.LegacyThing{{Id:&first},{Id:&second}}}}
 r := &Resource{client:client}
 got, err := r.get(context.Background(),"state-id")
 if err != nil || got == nil || *got.Id != first { t.Fatalf("batch read = %v, %v",got,err) }
 if !slices.Equal(client.BatchIDs,[]string{"state-id"}) { t.Fatalf("ids = %v",client.BatchIDs) }
 client.BatchResponse = &legacy_things_service.GetLegacyThingResponse{}
 got, err = r.get(context.Background(),"state-id")
 if err != nil || got != nil { t.Fatalf("empty read = %v, %v",got,err) }
 client.BatchResponse = nil
 got, err = r.get(context.Background(),"state-id")
 if err != nil || got != nil { t.Fatalf("nil read = %v, %v",got,err) }
}
func TestReleasedIntegerString(t *testing.T) {
 id := "example"
 team := int32(1234)
 got, diags := flatten(context.Background(), &legacy_things_service.LegacyThing{Id:&id, TeamId:&team}, nil)
 if diags.HasError() { t.Fatal(diags) }
 if got.TeamId.ValueString() != "1234" { t.Fatalf("team_id = %v",got.TeamId) }
 got, diags = flatten(context.Background(), &legacy_things_service.LegacyThing{Id:&id}, nil)
 if diags.HasError() || got.TeamId.ValueString() != "0" { t.Fatalf("absent team_id = %v, %v",got,diags) }
}
func TestCreateRecoveryRequiresUsableID(t *testing.T) {
 ctx := context.Background()
 empty, valid := "", "created"
 for _, tc := range []struct {
  name string
  value *legacy_things_service.LegacyThing
  failFlatten bool
  recoverID string
 }{
  {name:"null resource"},
  {name:"missing id", value:&legacy_things_service.LegacyThing{}},
  {name:"empty id", value:&legacy_things_service.LegacyThing{Id:&empty}},
  {name:"valid id with failed flatten", value:&legacy_things_service.LegacyThing{Id:&valid}, failFlatten:true, recoverID:valid},
 } {
  t.Run(tc.name,func(t *testing.T) {
   schema := Schema()
   typ := schema.Type().TerraformType(ctx)
   values := map[string]tftypes.Value{}
   for key, attrType := range typ.(tftypes.Object).AttributeTypes { values[key] = tftypes.NewValue(attrType,nil) }
   values["name"] = tftypes.NewValue(tftypes.String,"example")
   raw := tftypes.NewValue(typ,values)
   plan := tfsdk.Plan{Schema:schema,Raw:raw}
   client := &legacy_things_service.LegacyThingsServiceAPIService{CreateResponse:&legacy_things_service.CreateLegacyThingResponse{Thing:tc.value}}
   r := &Resource{client:client}
   if tc.failFlatten { r.hooks.BeforeFlatten = func(context.Context,*legacy_things_service.LegacyThing) diag.Diagnostics { return diag.Diagnostics{diag.NewErrorDiagnostic("Invalid response","test response cannot be flattened")} } }
   response := resource.CreateResponse{State:tfsdk.State{Schema:schema,Raw:tftypes.NewValue(typ,nil)}}
   r.Create(ctx,resource.CreateRequest{Config:tfsdk.Config{Schema:schema,Raw:raw},Plan:plan},&response)
   if client.CreateCalls != 1 || !response.Diagnostics.HasError() { t.Fatalf("create calls=%d, diagnostics=%v",client.CreateCalls,response.Diagnostics) }
   if tc.recoverID == "" {
    if !response.State.Raw.IsNull() { t.Fatalf("failed create wrote state: %s",response.State.Raw) }
    return
   }
   var id types.String
   if d := response.State.GetAttribute(ctx,path.Root("id"),&id); d.HasError() || id.ValueString() != tc.recoverID { t.Fatalf("recovered id=%v, diagnostics=%v",id,d) }
  })
 }
}
func TestEmptyStringIDRejected(t *testing.T) {
 id := ""
 _, diags := flatten(context.Background(), &legacy_things_service.LegacyThing{Id:&id}, nil)
 if !diags.HasError() { t.Fatal("accepted an empty resource id") }
}`
	if err := os.WriteFile(filepath.Join(out, "semantics_test.go"), []byte(semantics), 0o644); err != nil {
		t.Fatal(err)
	}
	compileGenerated(t, out, input)
}

func TestStringTerraformTypeRejectsManagedInteger(t *testing.T) {
	input, _ := syntheticInput(t)
	var doc map[string]any
	if err := yaml.Unmarshal(mustRead(t, filepath.Join("testdata", "batchthing.yaml")), &doc); err != nil {
		t.Fatal(err)
	}
	schemas := doc["components"].(map[string]any)["schemas"].(map[string]any)
	team := schemas["LegacyThing"].(map[string]any)["properties"].(map[string]any)["teamId"].(map[string]any)
	team["readOnly"] = false
	var err error
	input.OpenAPI, err = yaml.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	file, err := readOverrides(filepath.Join("testdata", "batchthing-overrides.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = validateOpenAPIWith(input.OpenAPI, "LegacyThing", Options{}.OperationIDs, input.SDKModule, input.Module, file)
	if err == nil || !strings.Contains(err.Error(), "computed-only integer") {
		t.Fatalf("got %v, want a computed-only integer error", err)
	}
}

// A released required field can be optional in Create, with either Update requiredness:
// Terraform still supplies it on every write.
func TestRequiredOverrideSupportsOptionalCreate(t *testing.T) {
	for name, requiredUpdate := range map[string]bool{"optional-update": false, "required-update": true} {
		t.Run(name, func(t *testing.T) {
			input, loadDir := copiedSyntheticInput(t)
			original := input.OpenAPI
			spec := strings.Replace(string(input.OpenAPI), "required: [name, kind, config]", "required: [kind, config]", 1)
			if requiredUpdate {
				spec = strings.Replace(spec, "title: UpdateThingRequest\n              type: object\n              required: []", "title: UpdateThingRequest\n              type: object\n              required: [name]", 1)
			}
			input.OpenAPI = []byte(spec)
			sdkFile := filepath.Join(input.SDKDir, "go", "openapi", "gen", "things_service", "things.go")
			sdk := mustRead(t, sdkFile)
			text := strings.Replace(string(sdk), "type CreateThingRequest struct {\n\tName         string", "type CreateThingRequest struct {\n\tName         *string", 1)
			if err := os.WriteFile(sdkFile, []byte(text), 0o644); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), "overrides.yaml")
			if err := os.WriteFile(path, []byte("resource: Thing\nmode: existing\nvalidators: {inferred: false}\ntypes: {Thing: {fields: {name: {required: true}}}}\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			file, err := readOverrides(path)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := validateOpenAPIWith(original, "Thing", Options{}.OperationIDs, input.SDKModule, input.Module, file); err == nil || !strings.Contains(err.Error(), "OVERRIDE_UNUSED") {
				t.Fatalf("an already-required Create must still reject the redundant override: %v", err)
			}
			without := filepath.Join(t.TempDir(), "without-required.yaml")
			if err := os.WriteFile(without, []byte("resource: Thing\nmode: existing\nvalidators: {inferred: false}\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			noRequired, err := readOverrides(without)
			if err != nil {
				t.Fatal(err)
			}
			wantCode := "FIELD_SERVER_DEFAULT_UNDECLARED"
			if requiredUpdate {
				wantCode = "FIELD_REQUIREDNESS_UNSUPPORTED"
			}
			if _, err := validateOpenAPIWith(input.OpenAPI, "Thing", Options{}.OperationIDs, input.SDKModule, input.Module, noRequired); err == nil || !strings.Contains(err.Error(), wantCode) {
				t.Fatalf("optional Create needs the released required override: %v", err)
			}
			out := filepath.Join(t.TempDir(), "requiredthing")
			if err := generateFromInput(Options{Resource: "Thing", OutputDir: out, OverridesPath: path}, input, loadDir); err != nil {
				t.Fatal(err)
			}
			golden := filepath.Join("testdata", "golden", "requiredthing", "schema.go")
			schema := mustRead(t, filepath.Join(out, "schema.go"))
			if *updateGolden {
				if err := os.MkdirAll(filepath.Dir(golden), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(golden, schema, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if string(mustRead(t, golden)) != string(schema) {
				t.Fatal("required schema golden differs; run with -update")
			}
			if err := os.WriteFile(filepath.Join(out, "semantics_test.go"), []byte(`package requiredthing
		import "testing"
		func TestNameRemainsRequired(t *testing.T) {
		 if !Schema().Attributes["name"].IsRequired() { t.Fatal("released required name became optional") }
		}`), 0o644); err != nil {
				t.Fatal(err)
			}
			compileGenerated(t, out, input)
		})
	}
}
