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

package ai

import (
	"context"
	"fmt"
	"slices"
	"sort"

	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	cxsdkOpenapi "github.com/coralogix/coralogix-management-sdk/go/openapi/cxsdk"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// The applications of a custom evaluation are named by application and subsystem in Terraform, and
// linked by id in the API. The ids are the stable identity of a link.

type aiApplicationReference struct {
	ID        string
	Name      string
	Subsystem string
}

type aiApplicationSelectorKey struct {
	Application string
	Subsystem   string
}

type aiCustomEvaluationApplicationModel struct {
	Application types.String `tfsdk:"application"`
	Subsystem   types.String `tfsdk:"subsystem"`
}

func aiCustomEvaluationApplicationAttributeTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"application": types.StringType,
		"subsystem":   types.StringType,
	}
}

// resolveApplicationsForUpdate resolves the planned selectors. When they did not change, the
// applications are the ones that the state links to.
func (r *aiCustomEvaluationResource) resolveApplicationsForUpdate(ctx context.Context, plan, state, stateIDs types.Set) ([]aiApplicationReference, diag.Diagnostics) {
	var diags diag.Diagnostics

	planApplications, planDiags := applicationSelectorsFromSet(ctx, plan)
	diags.Append(planDiags...)
	stateApplications, stateDiags := applicationSelectorsFromSet(ctx, state)
	diags.Append(stateDiags...)
	if diags.HasError() {
		return nil, diags
	}

	if applicationSelectorsEqual(planApplications, stateApplications) && !stateIDs.IsNull() && !stateIDs.IsUnknown() {
		return r.resolveApplicationIDs(ctx, stringSliceFromSet(ctx, stateIDs))
	}

	return r.resolveApplicationSelectors(ctx, plan)
}

func (r *aiCustomEvaluationResource) resolveApplicationSelectors(ctx context.Context, applications types.Set) ([]aiApplicationReference, diag.Diagnostics) {
	var diags diag.Diagnostics
	selectors, selectorDiags := applicationSelectorsFromSet(ctx, applications)
	diags.Append(selectorDiags...)
	if diags.HasError() {
		return nil, diags
	}
	if len(selectors) == 0 {
		return []aiApplicationReference{}, diags
	}

	allApplications, err := r.listAIApplications(ctx)
	if err != nil {
		diags.AddError("Error resolving AI applications", err.Error())
		return nil, diags
	}

	applicationsBySelector := make(map[aiApplicationSelectorKey][]aiApplicationReference, len(allApplications))
	for _, application := range allApplications {
		if application.Name == "" || application.ID == "" {
			continue
		}
		key := aiApplicationSelectorKey{
			Application: application.Name,
			Subsystem:   application.Subsystem,
		}
		applicationsBySelector[key] = append(applicationsBySelector[key], application)
	}

	resolved := make([]aiApplicationReference, 0, len(selectors))
	for _, selector := range selectors {
		name := selector.Application.ValueString()
		subsystem := selector.Subsystem.ValueString()
		matches := applicationsBySelector[aiApplicationSelectorKey{
			Application: name,
			Subsystem:   subsystem,
		}]
		if len(matches) == 0 {
			diags.AddError("AI application not found", fmt.Sprintf("No AI application named %q with subsystem %q was found.", name, subsystem))
			continue
		}
		if len(matches) > 1 {
			diags.AddError("Ambiguous AI application selector", fmt.Sprintf("Found %d AI applications named %q with subsystem %q.", len(matches), name, subsystem))
			continue
		}
		resolved = append(resolved, matches[0])
	}
	if diags.HasError() {
		return nil, diags
	}

	sortApplications(resolved)
	return resolved, diags
}

func (r *aiCustomEvaluationResource) resolveApplicationIDs(ctx context.Context, ids []string) ([]aiApplicationReference, diag.Diagnostics) {
	var diags diag.Diagnostics
	if len(ids) == 0 {
		return []aiApplicationReference{}, diags
	}

	allApplications, err := r.listAIApplications(ctx)
	if err != nil {
		diags.AddError("Error resolving AI applications", err.Error())
		return nil, diags
	}

	applicationsByID := make(map[string]aiApplicationReference, len(allApplications))
	for _, application := range allApplications {
		if application.ID == "" {
			continue
		}
		applicationsByID[application.ID] = application
	}

	resolved := make([]aiApplicationReference, 0, len(ids))
	for _, id := range ids {
		application, ok := applicationsByID[id]
		if !ok {
			diags.AddError("AI application not found", fmt.Sprintf("Linked AI application ID %q was not found.", id))
			continue
		}
		resolved = append(resolved, application)
	}
	if diags.HasError() {
		return nil, diags
	}

	sortApplications(resolved)
	return resolved, diags
}

// listAIApplications lists the applications of the team in pages of 200.
func (r *aiCustomEvaluationResource) listAIApplications(ctx context.Context) ([]aiApplicationReference, error) {
	const pageSize = int32(200)
	var applications []aiApplicationReference
	for pageOffset := int64(0); ; pageOffset++ {
		resp, httpResponse, err := r.aiApplicationsClient.
			AiApplicationsServiceListAiApplications(ctx).
			PageSize(pageSize).
			PageOffset(pageOffset).
			Execute()
		if err != nil {
			return nil, fmt.Errorf(
				"%s",
				utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "List", nil),
			)
		}

		page := resp.GetAiApplications()
		for _, application := range page {
			applications = append(applications, aiApplicationReference{
				ID:        application.GetId(),
				Name:      application.GetApplication(),
				Subsystem: application.GetSubsystem(),
			})
		}
		if len(page) < int(pageSize) {
			break
		}
	}

	return applications, nil
}

// reconcileApplicationLinks links the applications that are new, then unlinks the ones that are no
// longer wanted, each in the order of their ids.
func (r *aiCustomEvaluationResource) reconcileApplicationLinks(ctx context.Context, customEvaluationID string, currentApplicationIDs []string, desiredApplicationIDs []string) error {
	current := stringSet(currentApplicationIDs)
	desired := stringSet(desiredApplicationIDs)

	toLink := make([]string, 0)
	for id := range desired {
		if _, ok := current[id]; !ok {
			toLink = append(toLink, id)
		}
	}
	sort.Strings(toLink)

	toUnlink := make([]string, 0)
	for id := range current {
		if _, ok := desired[id]; !ok {
			toUnlink = append(toUnlink, id)
		}
	}
	sort.Strings(toUnlink)

	for _, id := range toLink {
		_, httpResponse, err := r.aiEvaluationsClient.
			AiEvaluationsServiceLinkCustomEvaluation(ctx, id, customEvaluationID).
			Execute()
		if err != nil {
			return fmt.Errorf("failed to link AI application %q: %s", id, utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Link", id))
		}
	}

	for _, id := range toUnlink {
		_, httpResponse, err := r.aiEvaluationsClient.
			AiEvaluationsServiceUnlinkCustomEvaluationFromApp(ctx, id, customEvaluationID).
			Execute()
		if err != nil {
			return fmt.Errorf("failed to unlink AI application %q: %s", id, utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Unlink", id))
		}
	}

	return nil
}

func applicationIDs(applications []aiApplicationReference) []string {
	ids := make([]string, 0, len(applications))
	for _, application := range applications {
		ids = append(ids, application.ID)
	}
	sort.Strings(ids)
	return ids
}

func sortApplications(applications []aiApplicationReference) {
	sort.Slice(applications, func(i, j int) bool {
		if applications[i].ID == applications[j].ID {
			if applications[i].Name == applications[j].Name {
				return applications[i].Subsystem < applications[j].Subsystem
			}
			return applications[i].Name < applications[j].Name
		}
		return applications[i].ID < applications[j].ID
	})
}

func applicationSelectorsValueFromReferences(ctx context.Context, applications []aiApplicationReference) (types.Set, diag.Diagnostics) {
	selectors := make([]aiCustomEvaluationApplicationModel, 0, len(applications))
	for _, application := range applications {
		selectors = append(selectors, aiCustomEvaluationApplicationModel{
			Application: types.StringValue(application.Name),
			Subsystem:   types.StringValue(application.Subsystem),
		})
	}

	return types.SetValueFrom(ctx, types.ObjectType{AttrTypes: aiCustomEvaluationApplicationAttributeTypes()}, selectors)
}

func applicationSelectorsFromSet(ctx context.Context, set types.Set) ([]aiCustomEvaluationApplicationModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	if set.IsNull() || set.IsUnknown() {
		return nil, diags
	}

	var selectors []aiCustomEvaluationApplicationModel
	diags.Append(set.ElementsAs(ctx, &selectors, false)...)
	if diags.HasError() {
		return nil, diags
	}

	sort.Slice(selectors, func(i, j int) bool {
		return applicationSelectorLess(selectors[i], selectors[j])
	})
	return selectors, diags
}

func applicationSelectorsEqual(left []aiCustomEvaluationApplicationModel, right []aiCustomEvaluationApplicationModel) bool {
	if len(left) != len(right) {
		return false
	}

	leftCopy := slices.Clone(left)
	rightCopy := slices.Clone(right)
	sort.Slice(leftCopy, func(i, j int) bool {
		return applicationSelectorLess(leftCopy[i], leftCopy[j])
	})
	sort.Slice(rightCopy, func(i, j int) bool {
		return applicationSelectorLess(rightCopy[i], rightCopy[j])
	})

	for i := range leftCopy {
		if leftCopy[i].Application.ValueString() != rightCopy[i].Application.ValueString() ||
			leftCopy[i].Subsystem.ValueString() != rightCopy[i].Subsystem.ValueString() {
			return false
		}
	}
	return true
}

func applicationSelectorLess(left aiCustomEvaluationApplicationModel, right aiCustomEvaluationApplicationModel) bool {
	if left.Application.ValueString() == right.Application.ValueString() {
		return left.Subsystem.ValueString() < right.Subsystem.ValueString()
	}
	return left.Application.ValueString() < right.Application.ValueString()
}

// stringSliceFromSet is the sorted strings of a set, or nil for a null or unknown set.
func stringSliceFromSet(ctx context.Context, set types.Set) []string {
	if set.IsNull() || set.IsUnknown() {
		return nil
	}

	var values []string
	if diags := set.ElementsAs(ctx, &values, false); diags.HasError() {
		return nil
	}
	sort.Strings(values)
	return values
}

func stringSlicesEqual(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}

	leftCopy := slices.Clone(left)
	rightCopy := slices.Clone(right)
	sort.Strings(leftCopy)
	sort.Strings(rightCopy)
	return slices.Equal(leftCopy, rightCopy)
}

func stringSet(values []string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}
