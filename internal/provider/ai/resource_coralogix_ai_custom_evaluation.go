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

	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"
	"github.com/coralogix/terraform-provider-coralogix/internal/provider/generated/aicustomevaluation"

	aiapplications "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/ai_applications_service"
	aievaluations "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/ai_evaluations_service"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// NewAICustomEvaluationResource returns coralogix_ai_custom_evaluation. The generated resource
// (internal/provider/generated/aicustomevaluation) does the CRUD calls. This type adds what the API
// cannot state:
//   - criteria is shown as flags and examples, and sent as the fields safe and violates and one list
//     of examples that carry a score;
//   - applications names AI applications by application and subsystem, and the API links them by id:
//     Create sends the ids, and Update links and unlinks them with calls of their own.
type aiCustomEvaluationResource struct {
	*aicustomevaluation.Resource
	aiApplicationsClient *aiapplications.AIApplicationsServiceAPIService
	aiEvaluationsClient  *aievaluations.AIEvaluationsServiceAPIService
}

var (
	_ resource.Resource                = &aiCustomEvaluationResource{}
	_ resource.ResourceWithConfigure   = &aiCustomEvaluationResource{}
	_ resource.ResourceWithImportState = &aiCustomEvaluationResource{}
)

func NewAICustomEvaluationResource() resource.Resource {
	r := &aiCustomEvaluationResource{}
	r.Resource = aicustomevaluation.NewResource(aicustomevaluation.Hooks{
		BeforeWrite:   r.beforeWrite,
		BeforeFlatten: r.beforeFlatten,
		AfterFlatten:  r.afterFlatten,
	}).(*aicustomevaluation.Resource)
	return r
}

func (r *aiCustomEvaluationResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.Resource.Configure(ctx, req, resp)
	if resp.Diagnostics.HasError() || req.ProviderData == nil {
		return
	}
	if clientSet, ok := req.ProviderData.(*clientset.ClientSet); ok {
		r.aiApplicationsClient = clientSet.AIApplications()
		r.aiEvaluationsClient = clientSet.AIEvaluations()
	}
}

// applicationsHolder carries the application ids between the steps of one call. The hooks of the
// generated resource get the context of the call, and this is what they share.
type applicationsHolder struct {
	// known says that knownIDs are the ids that the applications attribute of the plan or the state
	// stands for, so that a read that returns the same ids keeps the selectors without a lookup.
	known    bool
	knownIDs []string
	// finalIDs, when set, replaces the application ids of the API value before it is flattened.
	// The answer of an Update holds no links, and the links are made after it.
	finalIDs *[]string
}

type applicationsHolderKey struct{}

func withApplicationsHolder(ctx context.Context, h *applicationsHolder) context.Context {
	return context.WithValue(ctx, applicationsHolderKey{}, h)
}

func applicationsHolderOf(ctx context.Context) *applicationsHolder {
	h, _ := ctx.Value(applicationsHolderKey{}).(*applicationsHolder)
	return h
}

func (r *aiCustomEvaluationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	r.Resource.Create(withApplicationsHolder(ctx, &applicationsHolder{}), req, resp)
}

// Read remembers the application ids of the state, so that the selectors of the state are kept
// when the API returns the same ids.
func (r *aiCustomEvaluationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	h := &applicationsHolder{}
	var ids, selectors types.Set
	if !req.State.GetAttribute(ctx, path.Root("application_ids"), &ids).HasError() &&
		!req.State.GetAttribute(ctx, path.Root("applications"), &selectors).HasError() &&
		!ids.IsNull() && !ids.IsUnknown() && !selectors.IsNull() && !selectors.IsUnknown() {
		h.known, h.knownIDs = true, stringSliceFromSet(ctx, ids)
	}
	r.Resource.Read(withApplicationsHolder(ctx, h), req, resp)
}

// Update resolves the application selectors before any write, then lets the generated resource
// update the fields, then links and unlinks the applications. When a link fails, the fields have
// changed already, and the state keeps the old values until the next refresh.
func (r *aiCustomEvaluationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var planApplications, stateApplications, stateIDs types.Set
	var id types.String
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("applications"), &planApplications)...)
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("applications"), &stateApplications)...)
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("application_ids"), &stateIDs)...)
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("id"), &id)...)
	if resp.Diagnostics.HasError() {
		return
	}

	applications, diags := r.resolveApplicationsForUpdate(ctx, planApplications, stateApplications, stateIDs)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	desired := applicationIDs(applications)
	h := &applicationsHolder{known: true, knownIDs: desired, finalIDs: &desired}
	r.Resource.Update(withApplicationsHolder(ctx, h), req, resp)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.reconcileApplicationLinks(ctx, id.ValueString(), stringSliceFromSet(ctx, stateIDs), desired); err != nil {
		resp.Diagnostics.AddError(
			"Error updating coralogix_ai_custom_evaluation application links",
			fmt.Sprintf("%s. The custom evaluation fields may already have been updated; run Terraform again after resolving the link error.", err.Error()),
		)
		resp.State.Raw = req.State.Raw
	}
}

// beforeWrite sends criteria through the fields safe, violates, and examples, and the applications
// of a Create as ids.
func (r *aiCustomEvaluationResource) beforeWrite(ctx context.Context, config tfsdk.Config, body any) diag.Diagnostics {
	var diags diag.Diagnostics
	var criteria types.Object
	diags.Append(config.GetAttribute(ctx, path.Root("criteria"), &criteria)...)
	if diags.HasError() {
		return diags
	}
	examples, safe, violates, criteriaDiags := extractAICustomEvaluationCriteria(ctx, criteria)
	diags.Append(criteriaDiags...)
	if diags.HasError() {
		return diags
	}

	switch b := body.(type) {
	case *aievaluations.AiEvaluationsServiceCreateCustomEvaluationRequest:
		b.Safe, b.Violates, b.Examples = aievaluations.PtrString(safe), aievaluations.PtrString(violates), examples
		var selectors types.Set
		diags.Append(config.GetAttribute(ctx, path.Root("applications"), &selectors)...)
		if diags.HasError() {
			return diags
		}
		applications, applicationDiags := r.resolveApplicationSelectors(ctx, selectors)
		diags.Append(applicationDiags...)
		if diags.HasError() {
			return diags
		}
		b.ApplicationIds = applicationIDs(applications)
		if h := applicationsHolderOf(ctx); h != nil {
			h.known, h.knownIDs = true, b.ApplicationIds
		}
	case *aievaluations.AiEvaluationsServiceUpdateCustomEvaluationRequest:
		b.Safe, b.Violates, b.Examples = aievaluations.PtrString(safe), aievaluations.PtrString(violates), examples
	}
	return diags
}

// beforeFlatten gives the API value of an Update the links that the Update makes after it.
func (r *aiCustomEvaluationResource) beforeFlatten(ctx context.Context, v *aievaluations.CustomEvaluation) diag.Diagnostics {
	if h := applicationsHolderOf(ctx); h != nil && h.finalIDs != nil {
		v.ApplicationIds = *h.finalIDs
	}
	return nil
}

// afterFlatten shows criteria and applications from the API value. The selectors of the plan or the
// state are kept when the ids that they stand for are the ids of the API; otherwise the ids are
// looked up.
func (r *aiCustomEvaluationResource) afterFlatten(ctx context.Context, m *aicustomevaluation.CustomEvaluationModel, v *aievaluations.CustomEvaluation) diag.Diagnostics {
	var diags diag.Diagnostics
	config := v.GetConfig()

	criteria, criteriaDiags := flattenAICustomEvaluationCriteria(ctx, config)
	diags.Append(criteriaDiags...)
	if diags.HasError() {
		return diags
	}
	m.Criteria = criteria

	ids := v.GetApplicationIds()
	if h := applicationsHolderOf(ctx); h != nil && h.known && !m.Applications.IsNull() && !m.Applications.IsUnknown() && stringSlicesEqual(h.knownIDs, ids) {
		return diags
	}
	applications, applicationDiags := r.resolveApplicationIDs(ctx, ids)
	diags.Append(applicationDiags...)
	if diags.HasError() {
		return diags
	}
	selectors, selectorDiags := applicationSelectorsValueFromReferences(ctx, applications)
	diags.Append(selectorDiags...)
	m.Applications = selectors
	return diags
}
