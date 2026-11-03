// Copyright 2024 Coralogix Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package recording_rules

import (
	"context"
	"fmt"

	"github.com/coralogix/terraform-provider-coralogix/internal/provider/generated/recordingrulesgroupsset"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	recRuless "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/recording_rules_service"

	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"gopkg.in/yaml.v3"
)

// NewRecordingRuleGroupSetResource returns coralogix_recording_rules_groups_set, which is generated
// from the API contract (internal/provider/generated/recordingrulesgroupsset). yaml_content is the
// groups as a YAML document: a hook turns it into the request, and this file adds its validators and
// its plan modifier.
func NewRecordingRuleGroupSetResource() resource.Resource {
	return &recordingRuleGroupSetResource{Resource: recordingrulesgroupsset.NewResource(recordingRulesHooks).(*recordingrulesgroupsset.Resource)}
}

type recordingRuleGroupSetResource struct {
	*recordingrulesgroupsset.Resource
}

func (r *recordingRuleGroupSetResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	s := recordingrulesgroupsset.Schema()
	groups, ok := s.Attributes["groups"].(schema.SetNestedAttribute)
	if !ok {
		resp.Diagnostics.AddError("Unexpected recording rules schema", "groups is not a set nested attribute.")
		return
	}
	groups.Validators = append(groups.Validators, setvalidator.ExactlyOneOf(path.MatchRelative().AtParent().AtName("yaml_content")))
	s.Attributes["groups"] = groups
	yamlContent, ok := s.Attributes["yaml_content"].(schema.StringAttribute)
	if !ok {
		resp.Diagnostics.AddError("Unexpected recording rules schema", "yaml_content is not a string attribute.")
		return
	}
	yamlContent.Validators = []validator.String{
		stringvalidator.ConflictsWith(path.MatchRelative().AtParent().AtName("groups")),
		yamlContentValidator{},
	}
	// The released resource replaced the set on every change of the document.
	yamlContent.PlanModifiers = []planmodifier.String{
		stringplanmodifier.RequiresReplaceIf(utils.JSONStringsEqualPlanModifier, "", ""),
	}
	s.Attributes["yaml_content"] = yamlContent
	resp.Schema = s
}

// recordingRulesHooks send the groups of yaml_content, and its name when the configuration sets no
// name. The data source shares them: it writes no request.
var recordingRulesHooks = recordingrulesgroupsset.Hooks{
	BeforeWrite: func(ctx context.Context, config tfsdk.Config, body any) diag.Diagnostics {
		var document, name types.String
		diags := config.GetAttribute(ctx, path.Root("yaml_content"), &document)
		diags.Append(config.GetAttribute(ctx, path.Root("name"), &name)...)
		if diags.HasError() || document.ValueString() == "" {
			return diags
		}
		set, d := decodeYAMLContent(document.ValueString())
		if diags.Append(d...); diags.HasError() {
			return diags
		}
		switch b := body.(type) {
		case *recRuless.CreateRuleGroupSet:
			b.Groups = set.Groups
			if name.IsNull() {
				b.Name = set.Name
			}
		case *recRuless.UpdateRuleGroupSet:
			b.Groups = set.Groups
			if name.IsNull() && b.Name == nil {
				b.Name = set.Name
			}
		default:
			diags.AddError("Unexpected request", fmt.Sprintf("The request body is %T.", body))
		}
		return diags
	},
}

// decodeYAMLContent decodes the document into the Create request of the SDK. The SDK types carry no
// YAML tags, so a key is the lowercased Go field name (evaluationdelayms), and yaml.v3 drops an
// unknown key.
func decodeYAMLContent(document string) (*recRuless.CreateRuleGroupSet, diag.Diagnostics) {
	var set recRuless.CreateRuleGroupSet
	if err := yaml.Unmarshal([]byte(document), &set); err != nil {
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Error on unmarshal yaml_content", err.Error())}
	}
	return &set, nil
}

type yamlContentValidator struct{}

func (v yamlContentValidator) Description(context.Context) string {
	return "validate yaml_content"
}

func (v yamlContentValidator) MarkdownDescription(context.Context) string {
	return "validate yaml_content"
}

func (v yamlContentValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	var set recRuless.CreateRuleGroupSet
	if err := yaml.Unmarshal([]byte(req.ConfigValue.ValueString()), &set); err != nil {
		resp.Diagnostics.AddError("error on validating yaml_content", err.Error())
	}
	for i, group := range set.Groups {
		if group.Name == "" {
			resp.Diagnostics.AddError("error on validating yaml_content", fmt.Sprintf("groups[%d] name can not be empty", i))
		}
	}
}
