package coralogix

import (
	"context"
	"fmt"
	"log"

	"github.com/hashicorp/terraform-plugin-framework-validators/objectvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"terraform-provider-coralogix/coralogix/clientset"
	ArchiveMetrics "terraform-provider-coralogix/coralogix/clientset/grpc/metrics-configurator"
)

var (
	_                       resource.ResourceWithConfigure   = &ArchiveMetricsResource{}
	_                       resource.ResourceWithImportState = &ArchiveMetricsResource{}
	updateArchiveMetricsURL                                  = "com.coralogix.metrics.metrics_configurator.MetricsConfiguratorPublicService/ConfigureTenant"
	getArchiveMetricsURL                                     = "com.coralogix.metrics.metrics_configurator.MetricsConfiguratorPublicService/GetTenantConfig"
)

type ArchiveMetricsResourceModel struct {
	ID                 types.String `tfsdk:"id"`
	TenantID           types.Int64  `tfsdk:"tenant_id"`
	Prefix             types.String `tfsdk:"prefix"`
	RetentionsPolicies types.List   `tfsdk:"retentions_policies"` //RetentionsPolicyModel
	IBM                types.Object `tfsdk:"ibm"`                 //IBMConfigModel
	S3                 types.Object `tfsdk:"s3"`                  //S3ConfigModel
}

func NewArchiveMetricsResource() resource.Resource {
	return &ArchiveMetricsResource{}
}

type ArchiveMetricsResource struct {
	client *clientset.MetricsConfigurationClient
}

func (r *ArchiveMetricsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *ArchiveMetricsResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

	r.client = clientSet.MetricsConfiguration()
}

type IBMConfigModel struct {
	Endpoint types.String `tfsdk:"endpoint"`
	Crn      types.String `tfsdk:"crn"`
}

type S3ConfigModel struct {
	Bucket types.String `tfsdk:"bucket"`
	Region types.String `tfsdk:"region"`
}

type RetentionsPolicyModel struct {
	Resolution    types.Int64 `tfsdk:"resolution"`
	RetentionDays types.Int64 `tfsdk:"retention_days"`
}

func (r *ArchiveMetricsResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_archive_metrics"
}

func (r ArchiveMetricsResource) Schema(_ context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Version: 0,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"tenant_id": schema.Int64Attribute{
				Computed: true,
			},
			"prefix": schema.StringAttribute{
				Computed: true,
			},
			"retentions_policies": schema.ListNestedAttribute{
				Optional: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"resolution": schema.Int64Attribute{
							Required: true,
						},
						"retention_days": schema.Int64Attribute{
							Required: true,
						},
					},
				},
			},
			"ibm": schema.SingleNestedAttribute{
				Optional: true,
				Attributes: map[string]schema.Attribute{
					"endpoint": schema.StringAttribute{
						Required: true,
					},
					"crn": schema.StringAttribute{
						Required: true,
					},
				},
				Validators: []validator.Object{
					objectvalidator.ExactlyOneOf(
						path.MatchRelative().AtParent().AtName("s3"),
					),
				},
			},
			"s3": schema.SingleNestedAttribute{
				Optional: true,
				Attributes: map[string]schema.Attribute{
					"bucket": schema.StringAttribute{
						Required: true,
					},
					"region": schema.StringAttribute{
						Required: true,
					},
				},
				Validators: []validator.Object{
					objectvalidator.ExactlyOneOf(
						path.MatchRelative().AtParent().AtName("ibm"),
					),
				},
			},
		},
	}
}

func (r *ArchiveMetricsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// Retrieve values from plan
	var plan *ArchiveMetricsResourceModel
	diags := req.Plan.Get(ctx, &plan)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	createReq, diags := extractArchiveMetrics(ctx, *plan)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	log.Printf("[INFO] Creating new ArchiveMetrics: %s", protojson.Format(createReq))
	_, err := r.client.UpdateMetricsConfiguration(ctx, createReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		resp.Diagnostics.AddError(
			"Error creating ArchiveMetrics",
			formatRpcErrors(err, updateArchiveMetricsURL, protojson.Format(createReq)),
		)
		return
	}
	log.Print("[INFO] Submitted new ArchiveMetrics")

	readResp, err := r.client.GetMetricsConfiguration(ctx)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		resp.Diagnostics.AddError(
			"Error reading ArchiveMetrics",
			formatRpcErrors(err, getArchiveMetricsURL, ""),
		)
		return
	}
	log.Printf("[INFO] Received ArchiveMetrics: %s", protojson.Format(readResp))
	plan, diags = flattenArchiveMetrics(ctx, readResp.GetTenantConfig())
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

func flattenArchiveMetrics(ctx context.Context, metricConfig *ArchiveMetrics.TenantConfigV2) (*ArchiveMetricsResourceModel, diag.Diagnostics) {
	flattenedMetricsConfig := &ArchiveMetricsResourceModel{
		ID:       types.StringValue(""),
		TenantID: types.Int64Value(int64(metricConfig.GetTenantId())),
		Prefix:   types.StringValue(metricConfig.GetPrefix()),
	}

	flattenedMetricsConfig, diags := flattenStorageConfig(ctx, metricConfig, flattenedMetricsConfig)
	if diags.HasError() {
		return nil, diags
	}

	retentionsPolicies, diags := flattenRetentionPolicies(ctx, metricConfig.GetRetentionPolicy())
	if diags.HasError() {
		return nil, diags
	}
	flattenedMetricsConfig.RetentionsPolicies = retentionsPolicies

	return flattenedMetricsConfig, nil
}

func flattenRetentionPolicies(ctx context.Context, retentionPolicies []*ArchiveMetrics.RetentionPolicy) (types.List, diag.Diagnostics) {
	if retentionPolicies == nil {
		return types.ListNull(types.ObjectType{AttrTypes: retentionsPolicyModelAttr()}), nil
	}

	var policies []RetentionsPolicyModel
	for _, policy := range retentionPolicies {
		policies = append(policies, RetentionsPolicyModel{
			Resolution:    types.Int64Value(int64(policy.GetResolution())),
			RetentionDays: types.Int64Value(int64(policy.GetRetentionDays())),
		})
	}

	return types.ListValueFrom(ctx, types.ObjectType{AttrTypes: retentionsPolicyModelAttr()}, policies)
}

func flattenStorageConfig(ctx context.Context, metricConfig *ArchiveMetrics.TenantConfigV2, flattenedMetricsConfig *ArchiveMetricsResourceModel) (*ArchiveMetricsResourceModel, diag.Diagnostics) {
	switch metricConfig.GetStorageConfig().(type) {
	case *ArchiveMetrics.TenantConfigV2_Ibm:
		var ibmConfig *IBMConfigModel
		ibmConfigObject, diags := types.ObjectValueFrom(ctx, ibmConfigModelAttr(), ibmConfig)
		if diags.HasError() {
			return nil, diags
		}
		flattenedMetricsConfig.IBM = ibmConfigObject
		flattenedMetricsConfig.S3 = types.ObjectNull(s3ConfigModelAttr())
	case *ArchiveMetrics.TenantConfigV2_S3:
		var s3Config *S3ConfigModel
		s3ConfigObject, diags := types.ObjectValueFrom(ctx, s3ConfigModelAttr(), s3Config)
		if diags.HasError() {
			return nil, diags
		}
		flattenedMetricsConfig.S3 = s3ConfigObject
		flattenedMetricsConfig.IBM = types.ObjectNull(ibmConfigModelAttr())
	}

	return flattenedMetricsConfig, nil
}

func retentionsPolicyModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"resolution":     types.Int64Type,
		"retention_days": types.Int64Type,
	}
}

func ibmConfigModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"endpoint": types.StringType,
		"crn":      types.StringType,
	}
}

func s3ConfigModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"bucket": types.StringType,
		"region": types.StringType,
	}
}

func extractArchiveMetrics(ctx context.Context, plan ArchiveMetricsResourceModel) (*ArchiveMetrics.TenantConfigV2, diag.Diagnostics) {
	tenantConfig := ArchiveMetrics.TenantConfigV2{}
	if !plan.IBM.IsNull() {
		var ibmConfig IBMConfigModel
		diags := plan.IBM.As(ctx, &ibmConfig, basetypes.ObjectAsOptions{})
		if diags.HasError() {
			return nil, diags
		}
		tenantConfig.StorageConfig = &ArchiveMetrics.TenantConfigV2_Ibm{
			Ibm: &ArchiveMetrics.IbmConfigV2{
				Endpoint: ibmConfig.Endpoint.ValueString(),
				Crn:      ibmConfig.Crn.ValueString(),
			},
		}
	} else if !plan.S3.IsNull() {
		var s3Config S3ConfigModel
		diags := plan.S3.As(ctx, &s3Config, basetypes.ObjectAsOptions{})
		if diags.HasError() {
			return nil, diags
		}
		tenantConfig.StorageConfig = &ArchiveMetrics.TenantConfigV2_S3{
			S3: &ArchiveMetrics.S3Config{
				Bucket: s3Config.Bucket.ValueString(),
				Region: s3Config.Region.ValueString(),
			},
		}
	}
	retentionPolicy, diags := extractRetentionPolicies(ctx, plan.RetentionsPolicies)
	if diags.HasError() {
		return nil, diags
	}
	tenantConfig.RetentionPolicy = retentionPolicy

	return &tenantConfig, nil
}

func extractRetentionPolicies(ctx context.Context, policies types.List) ([]*ArchiveMetrics.RetentionPolicy, diag.Diagnostics) {
	var diags diag.Diagnostics
	var policiesObjects []types.Object
	var expandedPolicies []*ArchiveMetrics.RetentionPolicy
	policies.ElementsAs(ctx, &policiesObjects, true)

	for _, po := range policiesObjects {
		var policy RetentionsPolicyModel
		if dg := po.As(ctx, &policy, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}
		expandedPolicies = append(expandedPolicies, &ArchiveMetrics.RetentionPolicy{
			Resolution:    int32(policy.Resolution.ValueInt64()),
			RetentionDays: int32(policy.RetentionDays.ValueInt64()),
		})
	}
	if diags.HasError() {
		return nil, diags

	}
	return expandedPolicies, nil
}

func (r *ArchiveMetricsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state *ArchiveMetricsResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	//Get refreshed ArchiveMetrics value from Coralogix
	id := state.ID.ValueString()
	log.Printf("[INFO] Reading ArchiveMetrics: %s", id)
	getResp, err := r.client.GetMetricsConfiguration(ctx)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		if status.Code(err) == codes.NotFound {
			state.ID = types.StringNull()
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("ArchiveMetrics %q is in state, but no longer exists in Coralogix backend", id),
				fmt.Sprintf("%s will be recreated when you apply", id),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error reading ArchiveMetrics",
				formatRpcErrors(err, getArchiveMetricsURL, ""),
			)
		}
		return
	}
	log.Printf("[INFO] Received ArchiveMetrics: %s", protojson.Format(getResp))

	state, diags = flattenArchiveMetrics(ctx, getResp.GetTenantConfig())
	//
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

func (r *ArchiveMetricsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Retrieve values from plan
	var plan *ArchiveMetricsResourceModel
	diags := req.Plan.Get(ctx, &plan)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	createReq, diags := extractArchiveMetrics(ctx, *plan)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	log.Printf("[INFO] Updating new ArchiveMetrics: %s", protojson.Format(createReq))
	_, err := r.client.UpdateMetricsConfiguration(ctx, createReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		resp.Diagnostics.AddError(
			"Error updating ArchiveMetrics",
			formatRpcErrors(err, createEvents2MetricURL, protojson.Format(createReq)),
		)
		return
	}
	log.Print("[INFO] Submitted updated ArchiveMetrics")

	readResp, err := r.client.GetMetricsConfiguration(ctx)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		resp.Diagnostics.AddError(
			"Error reading ArchiveMetrics",
			formatRpcErrors(err, getEvents2MetricURL, ""),
		)
		return
	}
	plan, diags = flattenArchiveMetrics(ctx, readResp.GetTenantConfig())
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

func (r *ArchiveMetricsResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {

}
