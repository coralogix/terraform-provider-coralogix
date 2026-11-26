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

package integrations

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"

	integrationsSDK "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/integration_service"
)

// addWriteOnlyParameters adds the parameters of `parameters_wo` to the request, in key order with the
// others.
func addWriteOnlyParameters(ctx context.Context, config tfsdk.Config, body any) diag.Diagnostics {
	secrets, diags := writeOnlyParameters(ctx, config)
	if diags.HasError() || len(secrets) == 0 {
		return diags
	}
	metadata := requestMetadata(body)
	if metadata == nil {
		diags.AddError("Unexpected request body", fmt.Sprintf("The integration request body is %T.", body))
		return diags
	}
	list := append(metadata.IntegrationParameters.Parameters, secrets...)
	sort.Slice(list, func(i, j int) bool { return list[i].GetKey() < list[j].GetKey() })
	metadata.IntegrationParameters.Parameters = list
	return diags
}

// writeOnlyParameters returns the parameters of `parameters_wo` as API string parameters. The values
// live only in the configuration, because the framework keeps a write-only value out of the plan
// and the state.
func writeOnlyParameters(ctx context.Context, config tfsdk.Config) ([]integrationsSDK.Parameter, diag.Diagnostics) {
	var secrets types.Map
	diags := config.GetAttribute(ctx, path.Root("parameters_wo"), &secrets)
	if diags.HasError() || secrets.IsNull() || secrets.IsUnknown() {
		return nil, diags
	}
	values := map[string]string{}
	diags.Append(secrets.ElementsAs(ctx, &values, false)...)
	if diags.HasError() {
		return nil, diags
	}
	parameters := make([]integrationsSDK.Parameter, 0, len(values))
	for key, value := range values {
		parameters = append(parameters, integrationsSDK.Parameter{Key: &key, StringValue: &value})
	}
	return parameters, diags
}

// keepParameterVersions puts the versions of the prior value in the state. The API holds no
// version, and flatten leaves them null. The versions are what tells a read which keys are
// write-only, so the state has to keep them.
func keepParameterVersions(ctx context.Context, state *tfsdk.State, prior priorAttributes) diag.Diagnostics {
	var versions types.Map
	diags := prior.GetAttribute(ctx, path.Root("parameters_wo_versions"), &versions)
	if diags.HasError() {
		return diags
	}
	if versions.IsUnknown() {
		versions = types.MapNull(types.Int64Type)
	}
	diags.Append(state.SetAttribute(ctx, path.Root("parameters_wo_versions"), versions)...)
	return diags
}

type priorAttributes interface {
	GetAttribute(context.Context, path.Path, interface{}) diag.Diagnostics
}

// validateWriteOnlyParameters checks the rules that no attribute validator expresses:
//   - a key is in `parameters` or in `parameters_wo`, not in both
//   - every key of `parameters_wo` has a version, and every version has a key
//   - no value and no version is null
func validateWriteOnlyParameters(ctx context.Context, config tfsdk.Config) diag.Diagnostics {
	var secrets, versions types.Map
	var parameters types.Dynamic
	diags := config.GetAttribute(ctx, path.Root("parameters_wo"), &secrets)
	diags.Append(config.GetAttribute(ctx, path.Root("parameters_wo_versions"), &versions)...)
	diags.Append(config.GetAttribute(ctx, path.Root("parameters"), &parameters)...)
	if diags.HasError() || secrets.IsUnknown() || versions.IsUnknown() {
		return diags
	}
	names := mapElements(secrets)
	numbers := mapElements(versions)

	diags.Append(parameterNameChecks(names, numbers)...)
	if object, ok := parameters.UnderlyingValue().(types.Object); ok && !object.IsNull() && !object.IsUnknown() {
		var both []string
		for key := range object.Attributes() {
			if _, ok := names[key]; ok {
				both = append(both, key)
			}
		}
		sort.Strings(both)
		if len(both) > 0 {
			diags.AddAttributeError(path.Root("parameters_wo"), "Parameter set both ways",
				fmt.Sprintf("These keys appear in both `parameters` and `parameters_wo`: %s.\n\n"+
					"A parameter is set in one of them. Keep the ordinary value in `parameters`, or the secret in `parameters_wo`.", strings.Join(both, ", ")))
		}
	}
	return diags
}

func mapElements(m types.Map) map[string]attr.Value {
	if m.IsNull() || m.IsUnknown() {
		return map[string]attr.Value{}
	}
	return m.Elements()
}

func parameterNameChecks(names, numbers map[string]attr.Value) diag.Diagnostics {
	var diags diag.Diagnostics
	var orphans, nullSecrets, missing, nullVersions []string
	for key := range numbers {
		if _, ok := names[key]; !ok {
			orphans = append(orphans, key)
		}
	}
	for key, secret := range names {
		if secret.IsNull() {
			nullSecrets = append(nullSecrets, key)
		}
		version, ok := numbers[key]
		switch {
		case !ok:
			missing = append(missing, key)
		case version.IsNull():
			nullVersions = append(nullVersions, key)
		}
	}
	report := func(attribute, summary, detail string, keys []string) {
		if len(keys) == 0 {
			return
		}
		sort.Strings(keys)
		diags.AddAttributeError(path.Root(attribute), summary, fmt.Sprintf(detail, strings.Join(keys, ", ")))
	}
	report("parameters_wo_versions", "Version without a write-only parameter",
		"These `parameters_wo_versions` entries name a parameter that is not set in `parameters_wo`: %s.", orphans)
	report("parameters_wo", "Null write-only parameter value",
		"These `parameters_wo` entries are null: %s.\n\nA write-only parameter needs a value to send. Drop the entry, or set a value.", nullSecrets)
	report("parameters_wo_versions", "Missing write-only parameter version",
		"These `parameters_wo` entries have no matching `parameters_wo_versions` entry: %s.\n\n"+
			"Terraform cannot notice that a write-only value changed, so a version has to sit next to each one. Increment it to send a rotated secret.", missing)
	report("parameters_wo_versions", "Null write-only parameter version",
		"These `parameters_wo_versions` entries are null: %s.\n\nA version has to be a number.", nullVersions)
	return diags
}
