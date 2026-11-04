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
	"maps"

	"github.com/coralogix/terraform-provider-coralogix/internal/provider/generated/recordingrulesgroupsset"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	recRuless "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/recording_rules_service"

	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
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

// ModifyPlan keeps the labels of a rule that the configuration leaves out, as the released resource
// did: the API keeps no labels that a request omits. Terraform plans them as unknown when another
// value of their group changes, so they come from the state of the rule with the same group name and
// record. Matching by identity, and not by position, keeps a reorder from moving labels to another
// rule. A rule whose record changes takes the labels of the state rule whose record the group no
// longer has, when the group has as many new records as removed ones: they pair in order. A record
// or a group name that appears twice is left to the API. A rule or a group that is unknown is left
// out, and so is a group whose configured rules are unknown.
func (r *recordingRuleGroupSetResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() {
		return
	}
	var configGroups, planGroups, stateGroups types.Set
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("groups"), &configGroups)...)
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("groups"), &planGroups)...)
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("groups"), &stateGroups)...)
	if resp.Diagnostics.HasError() || !isKnown(configGroups) || !isKnown(planGroups) || !isKnown(stateGroups) {
		return
	}
	elems := planGroups.Elements()
	changed, d := keepLabels(ctx, elems, rulesByGroup(configGroups), rulesByGroup(stateGroups))
	if resp.Diagnostics.Append(d...); !changed || resp.Diagnostics.HasError() {
		return
	}
	set, d := types.SetValue(planGroups.ElementType(ctx), elems)
	if resp.Diagnostics.Append(d...); !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("groups"), set)...)
	}
}

// keepLabels replaces each known planned group whose rules keep labels, and reports whether any did.
func keepLabels(ctx context.Context, elems []attr.Value, configured, stored map[string][]types.Object) (bool, diag.Diagnostics) {
	changed := false
	for i, elem := range elems {
		group, ok := elem.(types.Object)
		if !ok {
			continue
		}
		name, ok := knownString(group, "name")
		if !ok {
			continue
		}
		config, inConfig := configured[name]
		prior, inState := stored[name]
		if !inConfig || !inState {
			continue
		}
		updated, keep, d := keepGroupLabels(ctx, group, config, prior)
		if d.HasError() {
			return false, d
		}
		if keep {
			elems[i], changed = updated, true
		}
	}
	return changed, nil
}

// keepGroupLabels sets the labels of every planned rule of the group whose labels are unknown, whose
// configuration leaves them out, and that has a prior rule.
func keepGroupLabels(ctx context.Context, group types.Object, config, prior []types.Object) (types.Object, bool, diag.Diagnostics) {
	rules, ok := group.Attributes()["rules"].(types.List)
	if !ok || !isKnown(rules) {
		return group, false, nil
	}
	configByRecord, configComplete := byRecord(config)
	priorByRecord, _ := byRecord(prior)
	planned := rules.Elements()
	renamed := renamedRules(planned, configByRecord, configComplete, prior)
	changed := false
	var diags diag.Diagnostics
	for j, elem := range planned {
		rule, ok := elem.(types.Object)
		if !ok {
			continue
		}
		labels, ok := priorLabels(rule, configByRecord, priorByRecord, renamed[j])
		if !ok {
			continue
		}
		attrs := maps.Clone(rule.Attributes())
		attrs["labels"] = labels
		value, d := types.ObjectValue(rule.AttributeTypes(ctx), attrs)
		if diags.Append(d...); diags.HasError() {
			return group, false, diags
		}
		planned[j], changed = value, true
	}
	if !changed {
		return group, false, diags
	}
	list, d := types.ListValue(rules.ElementType(ctx), planned)
	if diags.Append(d...); diags.HasError() {
		return group, false, diags
	}
	attrs := maps.Clone(group.Attributes())
	attrs["rules"] = list
	value, d := types.ObjectValue(group.AttributeTypes(ctx), attrs)
	diags.Append(d...)
	return value, !diags.HasError(), diags
}

// priorLabels returns the labels that a planned rule keeps: its labels are unknown, its configuration
// leaves them out, and it has a prior rule, with the same record or the renamed one.
func priorLabels(rule types.Object, config, prior map[string]types.Object, renamed types.Object) (attr.Value, bool) {
	if !isKnown(rule) || !rule.Attributes()["labels"].IsUnknown() {
		return nil, false
	}
	record, ok := knownString(rule, "record")
	if !ok {
		return nil, false
	}
	configRule, inConfig := config[record]
	if !inConfig || !configRule.Attributes()["labels"].IsNull() {
		return nil, false
	}
	priorRule, inState := prior[record]
	if !inState {
		priorRule = renamed
	}
	if !isKnown(priorRule) || priorRule.Attributes()["labels"].IsUnknown() {
		return nil, false
	}
	return priorRule.Attributes()["labels"], true
}

// renamedRules pairs, by index of the plan, each planned rule with a record that the prior rules do
// not have with a prior rule whose record the configuration no longer has. They pair in order, and
// only when both counts are the same and every configured record is known.
func renamedRules(planned []attr.Value, config map[string]types.Object, configComplete bool, prior []types.Object) map[int]types.Object {
	if !configComplete {
		return nil
	}
	priorRecords := map[string]bool{}
	var removed []types.Object
	for _, rule := range prior {
		record, ok := knownString(rule, "record")
		if !ok {
			return nil
		}
		priorRecords[record] = true
		if _, kept := config[record]; !kept {
			removed = append(removed, rule)
		}
	}
	var added []int
	for j, elem := range planned {
		rule, ok := elem.(types.Object)
		if !ok {
			return nil
		}
		record, ok := knownString(rule, "record")
		if !ok {
			return nil
		}
		if !priorRecords[record] {
			added = append(added, j)
		}
	}
	if len(added) != len(removed) {
		return nil
	}
	out := make(map[int]types.Object, len(added))
	for k, j := range added {
		out[j] = removed[k]
	}
	return out
}

// rulesByGroup maps the known groups by name to their known rules. A name that appears twice, or a
// group whose rules are unknown, is left out. An unknown rule is kept, so byRecord can tell that the
// records are incomplete.
func rulesByGroup(groups types.Set) map[string][]types.Object {
	out := map[string][]types.Object{}
	twice := map[string]bool{}
	for _, elem := range groups.Elements() {
		group, ok := elem.(types.Object)
		if !ok || !isKnown(group) {
			continue
		}
		name, ok := knownString(group, "name")
		if !ok {
			continue
		}
		if _, seen := out[name]; seen {
			twice[name] = true
		}
		var rules []types.Object
		if list, ok := group.Attributes()["rules"].(types.List); ok && isKnown(list) {
			for _, rule := range list.Elements() {
				if rule, ok := rule.(types.Object); ok {
					rules = append(rules, rule)
				}
			}
		} else {
			twice[name] = true
		}
		out[name] = rules
	}
	for name := range twice {
		delete(out, name)
	}
	return out
}

// byRecord maps the rules by record. A record that appears twice is left out, because its rules
// cannot be told apart. complete is false when a rule or its record is unknown.
func byRecord(rules []types.Object) (out map[string]types.Object, complete bool) {
	out, complete = map[string]types.Object{}, true
	twice := map[string]bool{}
	for _, rule := range rules {
		record, ok := knownString(rule, "record")
		if !ok {
			complete = complete && rule.IsNull()
			continue
		}
		if _, seen := out[record]; seen {
			twice[record] = true
		}
		out[record] = rule
	}
	for record := range twice {
		delete(out, record)
	}
	return out, complete
}

func knownString(object types.Object, name string) (string, bool) {
	if !isKnown(object) {
		return "", false
	}
	value, ok := object.Attributes()[name].(types.String)
	if !ok || !isKnown(value) {
		return "", false
	}
	return value.ValueString(), true
}

func isKnown(value attr.Value) bool {
	return !value.IsNull() && !value.IsUnknown()
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
