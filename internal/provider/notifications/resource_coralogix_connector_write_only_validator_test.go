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

package notifications

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func mapOf(t *testing.T, elem attr.Type, values map[string]attr.Value) types.Map {
	t.Helper()
	value, diags := types.MapValue(elem, values)
	if diags.HasError() {
		t.Fatalf("build map: %v", diags)
	}
	return value
}

// validateWriteOnlyFields runs the validator over a connector_config holding
// just the two write-only maps, and returns the error summaries it produced.
func validateWriteOnlyFields(t *testing.T, secrets, versions types.Map) []string {
	t.Helper()
	ctx := context.Background()

	config, diags := types.ObjectValueFrom(ctx, connectorConfigAttr(), ConnectorConfigModel{
		ConnectorConfigFields: types.SetNull(types.ObjectType{AttrTypes: connectorConfigFieldAttrs()}),
		FieldValuesWO:         secrets,
		FieldValuesWOVersions: versions,
	})
	if diags.HasError() {
		t.Fatalf("build connector config: %v", diags)
	}

	resp := &validator.ObjectResponse{}
	connectorWriteOnlyFieldsValidator{}.ValidateObject(ctx, validator.ObjectRequest{
		Path:        path.Root("connector_config"),
		ConfigValue: config,
	}, resp)

	summaries := make([]string, 0, resp.Diagnostics.ErrorsCount())
	for _, err := range resp.Diagnostics.Errors() {
		summaries = append(summaries, err.Summary())
	}
	return summaries
}

func TestConnectorWriteOnlyVersionValidation(t *testing.T) {
	secret := mapOf(t, types.StringType, map[string]attr.Value{
		"additionalHeaders": types.StringValue("a secret"),
	})

	for name, tc := range map[string]struct {
		secrets  types.Map
		versions types.Map
		want     string
	}{
		"a real version is accepted": {
			secrets:  secret,
			versions: mapOf(t, types.Int64Type, map[string]attr.Value{"additionalHeaders": types.Int64Value(1)}),
		},
		// A null version passes a presence check while recording nothing, and a
		// change to the secret alone is invisible to planning, so the next
		// rotation would quietly not ship.
		"a null version is rejected": {
			secrets:  secret,
			versions: mapOf(t, types.Int64Type, map[string]attr.Value{"additionalHeaders": types.Int64Null()}),
			want:     "Null Write-Only Field Version",
		},
		// Validation runs again before apply with the value resolved, which is
		// where this can actually be judged.
		"an unknown version defers": {
			secrets:  secret,
			versions: mapOf(t, types.Int64Type, map[string]attr.Value{"additionalHeaders": types.Int64Unknown()}),
		},
		"a missing version is rejected": {
			secrets:  secret,
			versions: types.MapNull(types.Int64Type),
			want:     "Missing Write-Only Field Version",
		},
		"a version naming no secret is rejected": {
			secrets:  types.MapNull(types.StringType),
			versions: mapOf(t, types.Int64Type, map[string]attr.Value{"additionalHeaders": types.Int64Value(1)}),
			want:     "Version Without a Write-Only Field",
		},
		// A module passing a variable hits this on its default.
		"two empty maps are fine": {
			secrets:  mapOf(t, types.StringType, map[string]attr.Value{}),
			versions: mapOf(t, types.Int64Type, map[string]attr.Value{}),
		},
	} {
		t.Run(name, func(t *testing.T) {
			got := validateWriteOnlyFields(t, tc.secrets, tc.versions)
			if tc.want == "" {
				if len(got) > 0 {
					t.Errorf("expected no error, got %v", got)
				}
				return
			}
			if len(got) != 1 || got[0] != tc.want {
				t.Errorf("got %v, want exactly [%s]", got, tc.want)
			}
		})
	}
}

// The whole-map unknown case: neither side can be compared yet, and Elements()
// on an unknown map is indistinguishable from an empty one, so comparing here
// would invent a mismatch.
func TestConnectorWriteOnlyValidationDefersWholeMapUnknown(t *testing.T) {
	secret := mapOf(t, types.StringType, map[string]attr.Value{
		"additionalHeaders": types.StringValue("a secret"),
	})

	for name, tc := range map[string]struct {
		secrets  types.Map
		versions types.Map
	}{
		"versions unknown": {secrets: secret, versions: types.MapUnknown(types.Int64Type)},
		"secrets unknown":  {secrets: types.MapUnknown(types.StringType), versions: mapOf(t, types.Int64Type, map[string]attr.Value{"additionalHeaders": types.Int64Value(1)})},
		"both unknown":     {secrets: types.MapUnknown(types.StringType), versions: types.MapUnknown(types.Int64Type)},
	} {
		t.Run(name, func(t *testing.T) {
			if got := validateWriteOnlyFields(t, tc.secrets, tc.versions); len(got) > 0 {
				t.Errorf("expected the check to defer, got %v", got)
			}
		})
	}
}

func TestConnectorNullVersionErrorNamesTheField(t *testing.T) {
	ctx := context.Background()
	secrets := mapOf(t, types.StringType, map[string]attr.Value{
		"apiKey":            types.StringValue("a"),
		"additionalHeaders": types.StringValue("b"),
	})
	versions := mapOf(t, types.Int64Type, map[string]attr.Value{
		"apiKey":            types.Int64Value(1),
		"additionalHeaders": types.Int64Null(),
	})

	config, diags := types.ObjectValueFrom(ctx, connectorConfigAttr(), ConnectorConfigModel{
		ConnectorConfigFields: types.SetNull(types.ObjectType{AttrTypes: connectorConfigFieldAttrs()}),
		FieldValuesWO:         secrets,
		FieldValuesWOVersions: versions,
	})
	if diags.HasError() {
		t.Fatal(diags)
	}

	resp := &validator.ObjectResponse{}
	connectorWriteOnlyFieldsValidator{}.ValidateObject(ctx, validator.ObjectRequest{
		Path:        path.Root("connector_config"),
		ConfigValue: config,
	}, resp)

	if resp.Diagnostics.ErrorsCount() != 1 {
		t.Fatalf("expected 1 error, got %d: %v", resp.Diagnostics.ErrorsCount(), resp.Diagnostics.Errors())
	}
	detail := resp.Diagnostics.Errors()[0].Detail()
	if !strings.Contains(detail, "additionalHeaders") {
		t.Errorf("the error does not name the offending field: %s", detail)
	}
	if strings.Contains(detail, "apiKey") {
		t.Errorf("the error names a field that is fine: %s", detail)
	}
}
