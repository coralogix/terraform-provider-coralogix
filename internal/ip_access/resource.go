package ip_access

import (
	"context"
	"fmt"
	"terraform-provider-coralogix/coralogix/clientset"
	"terraform-provider-coralogix/coralogix/utils"

	ipaccess "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/ip_access_service"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
)

var (
	_ resource.ResourceWithConfigure   = &IpAccessResource{}
	_ resource.ResourceWithImportState = &IpAccessResource{}

	CustomerSupportAccessSchemaToApi = map[string]string{
		"unspecified": "CORALOGIX_CUSTOMER_SUPPORT_ACCESS_UNSPECIFIED",
		"disabled":    "CORALOGIX_CUSTOMER_SUPPORT_ACCESS_DISABLED",
		"enabled":     "CORALOGIX_CUSTOMER_SUPPORT_ACCESS_ENABLED",
	}
	CustomerSupportAccessApiToSchema       = utils.ReverseMap(CustomerSupportAccessSchemaToApi)
	ValidCustomerSupportAccessSchemaValues = utils.GetKeys(CustomerSupportAccessSchemaToApi)
)

type IpAccessResource struct {
	client *ipaccess.IPAccessServiceAPIService
}

func NewIpaccessResource() resource.Resource {
	return &IpAccessResource{}
}

type IpAccessCompanySettingsModel struct {
	Id                     types.String        `tfsdk:"id"`
	CoralogixSupportAccess types.String        `tfsdk:"enable_coralogix_customer_support_access"`
	Rules                  []IpAccessRuleModel `tfsdk:"rules"`
}

type IpAccessRuleModel struct {
	Name    types.String `tfsdk:"name"`
	IpRange types.String `tfsdk:"ip_range"`
	Enabled types.Bool   `tfsdk:"enabled"`
}

func (r *IpAccessResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ipaccess"
}

func (r *IpAccessResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The unique identifier for the company IP access settings. This is typically a company ID.",
			},
			"enable_coralogix_customer_support_access": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Validators: []validator.String{
					stringvalidator.OneOf(ValidCustomerSupportAccessSchemaValues...),
				},
				Default: stringdefault.StaticString("unspecified"),
			},
			"ip_access": schema.ListNestedAttribute{
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"enabled": schema.BoolAttribute{
							Required:            true,
							MarkdownDescription: "Whether this IP access entry is enabled.",
						},
						"ip_range": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "The IP range in CIDR notation.",
						},
						"name": schema.StringAttribute{
							Optional:            true,
							MarkdownDescription: "The name of the IP access entry.",
						},
					},
				},
				Optional: true,
				Computed: true,
			},
		},
	}
}

func (r *IpAccessResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data IpAccessCompanySettingsModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	fullObj := extractIpAccessCompanySettings(data)

	rq := ipaccess.CreateCompanyIPAccessSettingsRequest{
		EnableCoralogixCustomerSupportAccess: fullObj.EnableCoralogixCustomerSupportAccess,
		IpAccess:                             extractIpAccessRules(data.Rules),
	}
	result, _, err := r.client.
		IpAccessServiceCreateCompanyIpAccessSettings(ctx).
		CreateCompanyIPAccessSettingsRequest(rq).
		Execute()

	if err != nil {
		resp.Diagnostics.AddError("Error creating resource",
			utils.FormatOpenAPIErrors(err, "Create", rq),
		)
		return
	}
	state := flattenCreateResponse(result)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *IpAccessResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data IpAccessCompanySettingsModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	result, _, err := r.client.
		IpAccessServiceGetCompanyIpAccessSettings(ctx).
		Execute()

	if err != nil {
		resp.Diagnostics.AddError("Error read resource",
			utils.FormatOpenAPIErrors(err, "Read", nil),
		)
		return
	}
	state := flattenReadResponse(result)

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *IpAccessResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data IpAccessCompanySettingsModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}
	fullObj := extractIpAccessCompanySettings(data)

	rq := ipaccess.ReplaceCompanyIPAccessSettingsRequest{
		EnableCoralogixCustomerSupportAccess: fullObj.EnableCoralogixCustomerSupportAccess,
		IpAccess:                             extractIpAccessRules(data.Rules),
	}
	result, _, err := r.client.
		IpAccessServiceReplaceCompanyIpAccessSettings(ctx).
		ReplaceCompanyIPAccessSettingsRequest(rq).
		Execute()

	if err != nil {
		resp.Diagnostics.AddError("Error replacing resource",
			utils.FormatOpenAPIErrors(err, "Replace", rq),
		)
		return
	}
	state := flattenReplaceResponse(result)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *IpAccessResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data IpAccessCompanySettingsModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	_, _, err := r.client.
		IpAccessServiceDeleteCompanyIpAccessSettings(ctx).
		Execute()

	if err != nil {
		resp.Diagnostics.AddError("Error deleting resource",
			utils.FormatOpenAPIErrors(err, "Delete", nil),
		)
		return
	}

}
func (r *IpAccessResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	clientSet, ok := req.ProviderData.(*clientset.ClientSet)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *clientset.ClientSet, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = clientSet.IpAccess()
}

func (r *IpAccessResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func extractIpAccessCompanySettings(model IpAccessCompanySettingsModel) ipaccess.CompanyIPAccessSettings {
	return ipaccess.CompanyIPAccessSettings{
		EnableCoralogixCustomerSupportAccess: model.CoralogixSupportAccess.ValueStringPointer(),
		IpAccess:                             extractIpAccessRulesInner(model.Rules),
	}
}

func extractIpAccessRules(rules []IpAccessRuleModel) []ipaccess.IPAccess {
	mappedRules := make([]ipaccess.IPAccess, len(rules))
	for i, rule := range rules {
		mappedRules[i] = ipaccess.IPAccess{
			Name:    rule.Name.ValueStringPointer(),
			IpRange: rule.IpRange.ValueString(),
			Enabled: rule.Enabled.ValueBool(),
		}
	}
	return mappedRules
}

func extractIpAccessRulesInner(rules []IpAccessRuleModel) []ipaccess.CompanyIPAccessSettingsIpAccessInner {
	mappedRules := make([]ipaccess.CompanyIPAccessSettingsIpAccessInner, len(rules))
	for i, rule := range rules {
		mappedRules[i] = ipaccess.CompanyIPAccessSettingsIpAccessInner{
			Value: &ipaccess.IPAccess{
				Name:    rule.Name.ValueStringPointer(),
				IpRange: rule.IpRange.ValueString(),
				Enabled: rule.Enabled.ValueBool(),
			},
		}
	}
	return mappedRules
}

func flattenCreateResponse(resp *ipaccess.CreateCompanyIPAccessSettingsResponse) IpAccessCompanySettingsModel {
	rules := make([]IpAccessRuleModel, len(resp.Settings.IpAccess))
	for i, r := range *&resp.Settings.IpAccess {
		rules[i] = flattenIPAccess(r.Value)
	}
	return IpAccessCompanySettingsModel{
		Id:                     types.StringValue(*resp.Settings.Id),
		CoralogixSupportAccess: types.StringValue(CustomerSupportAccessApiToSchema[*resp.Settings.EnableCoralogixCustomerSupportAccess]),
		Rules:                  rules,
	}
}

func flattenReplaceResponse(resp *ipaccess.ReplaceCompanyIPAccessSettingsResponse) IpAccessCompanySettingsModel {
	rules := make([]IpAccessRuleModel, len(resp.Settings.IpAccess))
	for i, r := range *&resp.Settings.IpAccess {
		rules[i] = flattenIPAccess(r.Value)
	}
	return IpAccessCompanySettingsModel{
		Id:                     types.StringValue(*resp.Settings.Id),
		CoralogixSupportAccess: types.StringValue(CustomerSupportAccessApiToSchema[*resp.Settings.EnableCoralogixCustomerSupportAccess]),
		Rules:                  rules,
	}
}

func flattenReadResponse(resp *ipaccess.GetCompanyIPAccessSettingsResponse) IpAccessCompanySettingsModel {
	rules := make([]IpAccessRuleModel, len(resp.Settings.IpAccess))
	for i, r := range *&resp.Settings.IpAccess {
		rules[i] = flattenIPAccess(r.Value)
	}
	return IpAccessCompanySettingsModel{
		Id:                     types.StringValue(*resp.Settings.Id),
		CoralogixSupportAccess: types.StringValue(CustomerSupportAccessApiToSchema[*resp.Settings.EnableCoralogixCustomerSupportAccess]),
		Rules:                  rules,
	}
}

func flattenIPAccess(r *ipaccess.IPAccess) IpAccessRuleModel {
	return IpAccessRuleModel{
		Name:    types.StringValue(*r.Name),
		IpRange: types.StringValue(r.IpRange),
		Enabled: types.BoolValue(r.Enabled),
	}
}
