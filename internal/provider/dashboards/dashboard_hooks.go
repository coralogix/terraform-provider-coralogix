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

package dashboards

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/coralogix/coralogix-management-sdk/go/openapi/dashboardjson"
	dashboardservice "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/dashboard_service"
	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"
	dashboardschema "github.com/coralogix/terraform-provider-coralogix/internal/provider/dashboards/dashboard_schema"
	gendashboard "github.com/coralogix/terraform-provider-coralogix/internal/provider/generated/dashboard"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"
)

// The attributes of coralogix_dashboard that content_json leaves in the state: the others are null,
// because the JSON manages them.
var contentJSONKept = []string{"id", "folder", "access_policy", "content_json"}

const dashboardRequestIDPrefix = "terraform-provider-coralogix-dashboard"

// NewDashboardResource returns coralogix_dashboard, which is generated from the API contract
// (internal/provider/generated/dashboard). The hooks keep the released behavior that the contract
// cannot state: content_json, access_policy, and the plan-time check of the backend.
func NewDashboardResource() resource.Resource {
	check := &planCheck{}
	inner := gendashboard.NewResource(dashboardHooks(check)).(*gendashboard.Resource)
	return &dashboardResource{Resource: inner, check: check}
}

// NewDashboardDataSource returns the data source, which is generated too.
func NewDashboardDataSource() datasource.DataSource {
	return gendashboard.NewDataSource(dashboardHooks(nil))
}

// dashboardResource is the generated resource with the client of the plan-time check. The check
// needs the client in a hook, and a hook is made before the provider configures the resource.
type dashboardResource struct {
	*gendashboard.Resource
	check *planCheck
}

func (r *dashboardResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.Resource.Configure(ctx, req, resp)
	if cs, ok := req.ProviderData.(*clientset.ClientSet); ok {
		r.check.client = cs.Dashboards()
	}
}

func dashboardHooks(check *planCheck) gendashboard.Hooks {
	hooks := gendashboard.Hooks{
		BeforeWrite:      beforeDashboardWrite,
		AfterGet:         afterDashboardGet,
		AfterRead:        afterDashboardRead,
		ValidateConfig:   validateContentJSON,
		AllowUnsupported: allowUnsupportedWithContentJSON,
	}
	if check != nil {
		hooks.ModifyPlan = check.modifyPlan
	}
	return hooks
}

// allowUnsupportedWithContentJSON lets a dashboard that content_json manages hold widgets that the
// typed schema cannot show: the document keeps them, and the typed attributes are null.
func allowUnsupportedWithContentJSON(ctx context.Context, data any) bool {
	source, ok := data.(interface {
		GetAttribute(context.Context, path.Path, any) diag.Diagnostics
	})
	if !ok {
		return false
	}
	var contentJSON types.String
	return !source.GetAttribute(ctx, path.Root("content_json"), &contentJSON).HasError() && !contentJSON.IsNull() && !contentJSON.IsUnknown()
}

// beforeDashboardWrite puts what the generated request lacks in the body: a request id, the access
// policy that the configuration sets, and the dashboard that content_json holds.
func beforeDashboardWrite(ctx context.Context, config tfsdk.Config, body any) diag.Diagnostics {
	var diags diag.Diagnostics
	var dashboard *dashboardservice.Dashboard
	var accessPolicy **string
	switch b := body.(type) {
	case *dashboardservice.CreateDashboardRequestDataStructure:
		b.RequestId = newDashboardRequestID("Create")
		dashboard, accessPolicy = &b.Dashboard, &b.AccessPolicy
	case *dashboardservice.ReplaceDashboardRequestDataStructure:
		b.RequestId = newDashboardRequestID("Replace")
		if b.Dashboard == nil {
			b.Dashboard = &dashboardservice.Dashboard{}
		}
		dashboard, accessPolicy = b.Dashboard, &b.AccessPolicy
	default:
		diags.AddError("Unsupported dashboard request", fmt.Sprintf("The request body is %T.", body))
		return diags
	}

	var policy types.String
	diags.Append(config.GetAttribute(ctx, path.Root("access_policy"), &policy)...)
	// A null or blank policy sends nothing: Create keeps the policy that the API gives, and Update keeps
	// the stored one.
	if !policy.IsNull() && !policy.IsUnknown() && policy.ValueString() != "" {
		*accessPolicy = policy.ValueStringPointer()
	}

	var contentJSON types.String
	diags.Append(config.GetAttribute(ctx, path.Root("content_json"), &contentJSON)...)
	if diags.HasError() || contentJSON.IsNull() || contentJSON.IsUnknown() {
		return diags
	}
	parsed := new(dashboardservice.Dashboard)
	if err := dashboardjson.Unmarshal([]byte(contentJSON.ValueString()), parsed); err != nil {
		diags.AddError("Error unmarshalling dashboard content json", err.Error())
		return diags
	}
	var folder types.Object
	diags.Append(config.GetAttribute(ctx, path.Root("folder"), &folder)...)
	applyFolder(parsed, folder)
	*dashboard = *parsed
	return diags
}

// applyFolder sets the folder of the dashboard from the folder attribute: the id, or else the path.
// A folder that is null or unknown changes nothing.
func applyFolder(dashboard *dashboardservice.Dashboard, folder types.Object) {
	if folder.IsNull() || folder.IsUnknown() {
		return
	}
	attributes := folder.Attributes()
	id, _ := attributes["id"].(types.String)
	folderPath, _ := attributes["path"].(types.String)
	switch {
	case !id.IsNull() && !id.IsUnknown():
		dashboard.FolderId = &dashboardservice.UUID{Value: id.ValueStringPointer()}
	case !folderPath.IsNull() && !folderPath.IsUnknown():
		dashboard.FolderPath = &dashboardservice.FolderPath{Segments: strings.Split(folderPath.ValueString(), "/")}
	}
}

func newDashboardRequestID(operation string) string {
	return fmt.Sprintf("%s-%s-%s", dashboardRequestIDPrefix, operation, uuid.NewString())
}

// afterDashboardGet sets access_policy from the Get response. The state holds the plan or the prior
// state value, or null on import and in the data source.
func afterDashboardGet(ctx context.Context, state *tfsdk.State, response *dashboardservice.GetDashboardResponse) diag.Diagnostics {
	var current types.String
	diags := state.GetAttribute(ctx, path.Root("access_policy"), &current)
	if diags.HasError() {
		return diags
	}
	diags.Append(state.SetAttribute(ctx, path.Root("access_policy"), foldDashboardAccessPolicy(current, response.AccessPolicy))...)
	return diags
}

// foldDashboardAccessPolicy folds the access policy of the API into the state value.
//
// The API sometimes returns an empty string instead of omitting the field. Both mean that the
// dashboard has no policy, and a write never sends an empty one. A known blank value in the plan
// or the state is kept, so a configured or stored "" or whitespace reads back as the same text.
// A text that is JSON-equal to the API text is kept as written. Otherwise the policy is stored in
// the shape that jsonencode produces, so a configuration written that way holds the same text.
func foldDashboardAccessPolicy(current types.String, accessPolicy *string) types.String {
	known := !current.IsNull() && !current.IsUnknown()
	if accessPolicy == nil || strings.TrimSpace(*accessPolicy) == "" {
		if known && strings.TrimSpace(current.ValueString()) == "" {
			return current
		}
		return types.StringNull()
	}
	if known && utils.JSONStringsEqual(current.ValueString(), *accessPolicy) {
		return current
	}
	canonical, err := utils.CanonicalJSON(*accessPolicy)
	if err != nil {
		// The API returned something that is not JSON. Store it unchanged instead of failing the read.
		return types.StringValue(*accessPolicy)
	}
	return types.StringValue(canonical)
}

// afterDashboardRead leaves null every attribute that content_json manages: the JSON is the
// configuration, and the API values of the other attributes would differ from the configuration.
func afterDashboardRead(ctx context.Context, state *tfsdk.State, _ any) diag.Diagnostics {
	var contentJSON types.String
	diags := state.GetAttribute(ctx, path.Root("content_json"), &contentJSON)
	if diags.HasError() || contentJSON.IsNull() || contentJSON.IsUnknown() {
		return diags
	}
	diags.Append(nullExcept(state, contentJSONKept...)...)
	// A folder in the JSON belongs to the JSON. Setting it in the state too can make the framework fail.
	parsed := new(dashboardservice.Dashboard)
	if err := dashboardjson.Unmarshal([]byte(contentJSON.ValueString()), parsed); err != nil {
		diags.AddError("Error Unmarshal Dashboard", err.Error())
		return diags
	}
	if parsed.FolderId != nil || parsed.FolderPath != nil {
		var folder types.Object
		diags.Append(state.GetAttribute(ctx, path.Root("folder"), &folder)...)
		diags.Append(state.SetAttribute(ctx, path.Root("folder"), types.ObjectNull(folder.AttributeTypes(ctx)))...)
	}
	return diags
}

// nullExcept sets every top-level attribute of the state to null, except the named ones.
func nullExcept(state *tfsdk.State, keep ...string) diag.Diagnostics {
	var diags diag.Diagnostics
	object, ok := state.Raw.Type().(tftypes.Object)
	if !ok {
		diags.AddError("Unexpected dashboard state", fmt.Sprintf("The state has the type %s.", state.Raw.Type()))
		return diags
	}
	var values map[string]tftypes.Value
	if err := state.Raw.As(&values); err != nil {
		diags.AddError("Unexpected dashboard state", err.Error())
		return diags
	}
	for name, t := range object.AttributeTypes {
		if !containsName(keep, name) {
			values[name] = tftypes.NewValue(t, nil)
		}
	}
	state.Raw = tftypes.NewValue(object, values)
	return diags
}

func containsName(names []string, name string) bool {
	for _, n := range names {
		if n == name {
			return true
		}
	}
	return false
}

// validateContentJSON checks that content_json is a dashboard, and warns about the fields that the
// provider would drop.
func validateContentJSON(ctx context.Context, config tfsdk.Config) diag.Diagnostics {
	var contentJSON types.String
	diags := config.GetAttribute(ctx, path.Root("content_json"), &contentJSON)
	if diags.HasError() {
		return diags
	}
	resp := &validator.StringResponse{}
	dashboardschema.ContentJsonValidator{}.ValidateString(ctx, validator.StringRequest{
		Path: path.Root("content_json"), PathExpression: path.Root("content_json").Expression(), Config: config, ConfigValue: contentJSON,
	}, resp)
	diags.Append(resp.Diagnostics...)
	return diags
}
