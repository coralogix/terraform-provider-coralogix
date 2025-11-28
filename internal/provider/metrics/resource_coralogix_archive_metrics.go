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

package metrics

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	cxsdkOpenapi "github.com/coralogix/coralogix-management-sdk/go/openapi/cxsdk"
	ams "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/metrics_data_archive_service"

	"github.com/hashicorp/terraform-plugin-framework-validators/objectvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

var (
	_ resource.ResourceWithConfigure   = &ArchiveMetricsResource{}
	_ resource.ResourceWithImportState = &ArchiveMetricsResource{}
)

// Safeguard against empty ID string, as using empty string causes problems when this provider is used in Pulumi via https://github.com/pulumi/pulumi-terraform-provider
const RESOURCE_ID_ARCHIVE_METRICS string = "archive-metrics-settings"

type ArchiveMetricsResourceModel struct {
	ID              types.String `tfsdk:"id"`
	TenantID        types.Int64  `tfsdk:"tenant_id"`
	Prefix          types.String `tfsdk:"prefix"`
	RetentionPolicy types.Object `tfsdk:"retention_policy"` //RetentionPolicyModel
	IBM             types.Object `tfsdk:"ibm"`              //IBMConfigModel
	S3              types.Object `tfsdk:"s3"`               //S3ConfigModel
}

func NewArchiveMetricsResource() resource.Resource {
	return &ArchiveMetricsResource{}
}

type ArchiveMetricsResource struct {
	client *ams.MetricsDataArchiveServiceAPIService
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

	r.client = clientSet.ArchiveMetrics()
}

type IBMConfigModel struct {
	Endpoint types.String `tfsdk:"endpoint"`
	Crn      types.String `tfsdk:"crn"`
}

type S3ConfigModel struct {
	Bucket types.String `tfsdk:"bucket"`
	Region types.String `tfsdk:"region"`
}

type RetentionPolicyModel struct {
	RawResolution         types.Int64 `tfsdk:"raw_resolution"`
	FiveMinutesResolution types.Int64 `tfsdk:"five_minutes_resolution"`
	OneHourResolution     types.Int64 `tfsdk:"one_hour_resolution"`
}

func (r *ArchiveMetricsResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_archive_metrics"
}

func (r *ArchiveMetricsResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
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
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"prefix": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"retention_policy": schema.SingleNestedAttribute{
				Computed: true,
				Optional: true,
				Attributes: map[string]schema.Attribute{
					"raw_resolution": schema.Int64Attribute{
						Required: true,
					},
					"five_minutes_resolution": schema.Int64Attribute{
						Required: true,
					},
					"one_hour_resolution": schema.Int64Attribute{
						Required: true,
					},
				},
				PlanModifiers: []planmodifier.Object{
					objectplanmodifier.UseStateForUnknown(),
				},
				MarkdownDescription: "The retention policy (in days) for the archived metrics. Having default values when not specified.",
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
						Required:            true,
						MarkdownDescription: "The bucket name to store the archived metrics in.",
					},
					"region": schema.StringAttribute{
						Required:            true,
						MarkdownDescription: "The bucket region. see - https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/Concepts.RegionsAndAvailabilityZones.html#Concepts.RegionsAndAvailabilityZones.Regions",
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

	rq, diags := extractArchiveMetrics(ctx, *plan)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	log.Printf("[INFO] Creating new coralogix_archive_metrics: %s", utils.FormatJSON(rq))
	_, httpResponse, err := r.client.
		MetricsConfiguratorPublicServiceConfigureTenant(ctx).
		MetricsConfiguratorPublicServiceConfigureTenantRequest(*rq).
		Execute()
	if err != nil {
		resp.Diagnostics.AddError("Error creating coralogix_archive_metrics",
			utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Create", rq),
		)
		return
	}
	result, httpResponse, err := r.client.
		MetricsConfiguratorPublicServiceGetTenantConfig(ctx).
		Execute()
	if err != nil {
		resp.Diagnostics.AddError("Error reading coralogix_archive_metrics",
			utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Read", rq),
		)
		return
	}
	log.Printf("[INFO] Created new coralogix_archive_metrics: %s", utils.FormatJSON(result))

	plan, diags = flattenArchiveMetrics(ctx, result.TenantConfig, RESOURCE_ID_ARCHIVE_METRICS)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

func (r *ArchiveMetricsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state *ArchiveMetricsResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := state.ID.ValueString()

	rq := r.client.
		MetricsConfiguratorPublicServiceGetTenantConfig(ctx)
	log.Printf("[INFO] Reading new coralogix_archive_metrics: %s", utils.FormatJSON(rq))

	result, httpResponse, err := rq.
		Execute()

	if err != nil {
		if httpResponse.StatusCode == http.StatusNotFound {
			resp.Diagnostics.AddWarning(
				"coralogix_archive_metrics is in state, but no longer exists in Coralogix backend",
				"coralogix_archive_metrics will be recreated when you apply",
			)
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError("Error reading coralogix_archive_metrics",
				utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Read", nil),
			)
		}
		return
	}

	log.Printf("[INFO] Created new coralogix_archive_metrics: %s", utils.FormatJSON(result))

	state, diags = flattenArchiveMetrics(ctx, result.TenantConfig, id)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

func (r *ArchiveMetricsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan *ArchiveMetricsResourceModel
	diags := req.Plan.Get(ctx, &plan)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	rq, diags := extractArchiveMetrics(ctx, *plan)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	log.Printf("[INFO] Updating new coralogix_archive_metrics: %s", utils.FormatJSON(rq))
	_, httpResponse, err := r.client.
		MetricsConfiguratorPublicServiceConfigureTenant(ctx).
		MetricsConfiguratorPublicServiceConfigureTenantRequest(*rq).
		Execute()
	if err != nil {
		resp.Diagnostics.AddError("Error update coralogix_archive_metrics",
			utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Create", rq),
		)
		return
	}
	result, httpResponse, err := r.client.
		MetricsConfiguratorPublicServiceGetTenantConfig(ctx).
		Execute()
	if err != nil {
		resp.Diagnostics.AddError("Error reading coralogix_archive_metrics",
			utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Read", rq),
		)
		return
	}
	log.Printf("[INFO] Created new coralogix_archive_metrics: %s", utils.FormatJSON(result))

	plan, diags = flattenArchiveMetrics(ctx, result.TenantConfig, plan.ID.ValueString())
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

func (r *ArchiveMetricsResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {

}

func flattenArchiveMetrics(ctx context.Context, metricConfig *ams.TenantConfigV2, id string) (*ArchiveMetricsResourceModel, diag.Diagnostics) {
	flattenedMetricsConfig, diags := flattenStorageConfig(ctx, metricConfig)
	if diags.HasError() {
		return nil, diags
	}
	flattenedMetricsConfig.ID = types.StringValue(id)
	return flattenedMetricsConfig, nil
}

func flattenRetentionPolicy(ctx context.Context, policy *ams.RetentionPolicyRequest) (types.Object, diag.Diagnostics) {
	if policy == nil {
		return types.ObjectNull(retentionPolicyModelAttr()), nil
	}

	flattenedPolicy := RetentionPolicyModel{
		RawResolution:         types.Int64Value(int64(policy.GetRawResolution())),
		FiveMinutesResolution: types.Int64Value(int64(policy.GetFiveMinutesResolution())),
		OneHourResolution:     types.Int64Value(int64(policy.GetOneHourResolution())),
	}

	return types.ObjectValueFrom(ctx, retentionPolicyModelAttr(), flattenedPolicy)
}

func flattenStorageConfig(ctx context.Context, metricConfig *ams.TenantConfigV2) (*ArchiveMetricsResourceModel, diag.Diagnostics) {
	if c := metricConfig.TenantConfigV2Ibm; c != nil {
		retentionPolicy, diags := flattenRetentionPolicy(ctx, c.RetentionPolicy)
		if diags.HasError() {
			return nil, diags
		}

		ibmConfig := &IBMConfigModel{
			Endpoint: types.StringPointerValue(c.Ibm.Endpoint),
			Crn:      types.StringPointerValue(c.Ibm.Crn),
		}
		ibmConfigObject, diags := types.ObjectValueFrom(ctx, ibmConfigModelAttr(), ibmConfig)
		if diags.HasError() {
			return nil, diags
		}

		return &ArchiveMetricsResourceModel{
			TenantID:        types.Int64PointerValue(c.TenantId),
			Prefix:          types.StringPointerValue(c.Prefix),
			RetentionPolicy: retentionPolicy,
			IBM:             ibmConfigObject,
			S3:              types.ObjectNull(s3ConfigModelAttr()),
		}, diags
	} else if c := metricConfig.TenantConfigV2S3; c != nil {
		retentionPolicy, diags := flattenRetentionPolicy(ctx, c.RetentionPolicy)
		if diags.HasError() {
			return nil, diags
		}

		s3Config := &S3ConfigModel{
			Bucket: types.StringPointerValue(c.S3.Bucket),
			Region: types.StringPointerValue(c.S3.Region),
		}
		s3ConfigObject, diags := types.ObjectValueFrom(ctx, s3ConfigModelAttr(), s3Config)
		if diags.HasError() {
			return nil, diags
		}
		return &ArchiveMetricsResourceModel{
			TenantID:        types.Int64PointerValue(c.TenantId),
			Prefix:          types.StringPointerValue(c.Prefix),
			S3:              s3ConfigObject,
			RetentionPolicy: retentionPolicy,
			IBM:             types.ObjectNull(ibmConfigModelAttr()),
		}, diags
	} else {
		return nil, nil
	}
}

func retentionPolicyModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"raw_resolution":          types.Int64Type,
		"five_minutes_resolution": types.Int64Type,
		"one_hour_resolution":     types.Int64Type,
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

func extractArchiveMetrics(ctx context.Context, plan ArchiveMetricsResourceModel) (*ams.MetricsConfiguratorPublicServiceConfigureTenantRequest, diag.Diagnostics) {
	tenantConfig := ams.MetricsConfiguratorPublicServiceConfigureTenantRequest{}
	retentionPolicy, diags := extractRetentionPolicies(ctx, plan.RetentionPolicy)
	if diags.HasError() {
		return nil, diags
	}
	if !plan.IBM.IsNull() {
		var ibmConfig IBMConfigModel
		diags := plan.IBM.As(ctx, &ibmConfig, basetypes.ObjectAsOptions{})
		if diags.HasError() {
			return nil, diags
		}
		tenantConfig.ConfigureTenantRequestIbm = &ams.ConfigureTenantRequestIbm{
			Ibm: &ams.IbmConfigV2{
				Endpoint: ibmConfig.Endpoint.ValueStringPointer(),
				Crn:      ibmConfig.Crn.ValueStringPointer(),
			},
		}
		tenantConfig.ConfigureTenantRequestIbm.RetentionPolicy = retentionPolicy
	} else if !plan.S3.IsNull() {
		var s3Config S3ConfigModel
		diags := plan.S3.As(ctx, &s3Config, basetypes.ObjectAsOptions{})
		if diags.HasError() {
			return nil, diags
		}
		log.Printf("HELLO %v", tenantConfig.ConfigureTenantRequestS3)
		tenantConfig.ConfigureTenantRequestS3 = &ams.ConfigureTenantRequestS3{
			S3: &ams.S3Config{
				Bucket: s3Config.Bucket.ValueStringPointer(),
				Region: s3Config.Region.ValueStringPointer(),
			},
		}
		tenantConfig.ConfigureTenantRequestS3.RetentionPolicy = retentionPolicy
	}
	return &tenantConfig, nil
}

func extractRetentionPolicies(ctx context.Context, policy types.Object) (*ams.RetentionPolicyRequest, diag.Diagnostics) {
	if policy.IsNull() || policy.IsUnknown() {
		return nil, nil
	}

	var policyModel RetentionPolicyModel
	if diags := policy.As(ctx, &policyModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	return &ams.RetentionPolicyRequest{
		RawResolution:         policyModel.RawResolution.ValueInt64Pointer(),
		FiveMinutesResolution: policyModel.FiveMinutesResolution.ValueInt64Pointer(),
		OneHourResolution:     policyModel.OneHourResolution.ValueInt64Pointer(),
	}, nil
}
