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

package provider

import (
	"context"
	"fmt"
	"log"

	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	cxsdk "github.com/coralogix/coralogix-management-sdk/go"

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
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/encoding/protojson"
)

var (
	_ resource.ResourceWithConfigure   = &ArchiveMetricsResource{}
	_ resource.ResourceWithImportState = &ArchiveMetricsResource{}
)

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
	client *cxsdk.ArchiveMetricsClient
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

	createReq, diags := extractArchiveMetrics(ctx, *plan)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	log.Printf("[INFO] Creating new archive-metrics: %s", protojson.Format(createReq))
	_, err := r.client.ConfigureTenant(ctx, createReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError(
			"Error creating archive-metrics",
			utils.FormatRpcErrors(err, cxsdk.ArchiveMetricsConfigureTenantRPC, protojson.Format(createReq)),
		)
		return
	}
	log.Print("[INFO] Submitted new archive-metrics")

	readResp, err := r.client.Get(ctx)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError(
			"Error reading archive-metrics",
			utils.FormatRpcErrors(err, cxsdk.ArchiveMetricsGetTenantConfigRPC, ""),
		)
		return
	}
	log.Printf("[INFO] Received archiveMetrics: %s", protojson.Format(readResp))
	plan, diags = flattenArchiveMetrics(ctx, readResp.GetTenantConfig(), RESOURCE_ID_ARCHIVE_METRICS)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

func flattenArchiveMetrics(ctx context.Context, metricConfig *cxsdk.TenantConfigV2, id string) (*ArchiveMetricsResourceModel, diag.Diagnostics) {
	flattenedMetricsConfig := &ArchiveMetricsResourceModel{
		ID:       types.StringValue(id),
		TenantID: types.Int64Value(int64(metricConfig.GetTenantId())),
		Prefix:   types.StringValue(metricConfig.GetPrefix()),
	}

	flattenedMetricsConfig, diags := flattenStorageConfig(ctx, metricConfig, flattenedMetricsConfig)
	if diags.HasError() {
		return nil, diags
	}

	retentionPolicy, diags := flattenRetentionPolicy(ctx, metricConfig.GetRetentionPolicy())
	if diags.HasError() {
		return nil, diags
	}
	flattenedMetricsConfig.RetentionPolicy = retentionPolicy

	return flattenedMetricsConfig, nil
}

func flattenRetentionPolicy(ctx context.Context, policy *cxsdk.RetentionPolicyRequest) (types.Object, diag.Diagnostics) {
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

func flattenStorageConfig(ctx context.Context, metricConfig *cxsdk.TenantConfigV2, flattenedMetricsConfig *ArchiveMetricsResourceModel) (*ArchiveMetricsResourceModel, diag.Diagnostics) {
	switch storageConfig := metricConfig.GetStorageConfig().(type) {
	case *cxsdk.TenantConfigV2Ibm:
		ibmConfig := &IBMConfigModel{
			Endpoint: types.StringValue(storageConfig.Ibm.GetEndpoint()),
			Crn:      types.StringValue(storageConfig.Ibm.GetCrn()),
		}
		ibmConfigObject, diags := types.ObjectValueFrom(ctx, ibmConfigModelAttr(), ibmConfig)
		if diags.HasError() {
			return nil, diags
		}
		flattenedMetricsConfig.IBM = ibmConfigObject
		flattenedMetricsConfig.S3 = types.ObjectNull(s3ConfigModelAttr())
	case *cxsdk.TenantConfigV2S3:
		s3Config := &S3ConfigModel{
			Bucket: types.StringValue(storageConfig.S3.GetBucket()),
			Region: types.StringValue(storageConfig.S3.GetRegion()),
		}
		s3ConfigObject, diags := types.ObjectValueFrom(ctx, s3ConfigModelAttr(), s3Config)
		if diags.HasError() {
			return nil, diags
		}
		flattenedMetricsConfig.S3 = s3ConfigObject
		flattenedMetricsConfig.IBM = types.ObjectNull(ibmConfigModelAttr())
	}

	return flattenedMetricsConfig, nil
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

func extractArchiveMetrics(ctx context.Context, plan ArchiveMetricsResourceModel) (*cxsdk.ConfigureTenantRequest, diag.Diagnostics) {
	tenantConfig := cxsdk.ConfigureTenantRequest{}
	if !plan.IBM.IsNull() {
		var ibmConfig IBMConfigModel
		diags := plan.IBM.As(ctx, &ibmConfig, basetypes.ObjectAsOptions{})
		if diags.HasError() {
			return nil, diags
		}
		tenantConfig.StorageConfig = &cxsdk.ConfigureTenantRequestIbm{
			Ibm: &cxsdk.ArchiveIbmConfigV2{
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
		tenantConfig.StorageConfig = &cxsdk.ConfigureTenantRequestS3{
			S3: &cxsdk.ArchiveS3Config{
				Bucket: s3Config.Bucket.ValueString(),
				Region: s3Config.Region.ValueString(),
			},
		}
	}
	retentionPolicy, diags := extractRetentionPolicies(ctx, plan.RetentionPolicy)
	if diags.HasError() {
		return nil, diags
	}
	tenantConfig.RetentionPolicy = retentionPolicy

	return &tenantConfig, nil
}

func extractRetentionPolicies(ctx context.Context, policy types.Object) (*cxsdk.RetentionPolicyRequest, diag.Diagnostics) {
	if policy.IsNull() || policy.IsUnknown() {
		return nil, nil
	}

	var policyModel RetentionPolicyModel
	if diags := policy.As(ctx, &policyModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	return &cxsdk.RetentionPolicyRequest{
		RawResolution:         uint32(policyModel.RawResolution.ValueInt64()),
		FiveMinutesResolution: uint32(policyModel.FiveMinutesResolution.ValueInt64()),
		OneHourResolution:     uint32(policyModel.OneHourResolution.ValueInt64()),
	}, nil
}

func (r *ArchiveMetricsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state *ArchiveMetricsResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	//Get refreshed archiveMetrics value from Coralogix
	id := state.ID.ValueString()
	log.Printf("[INFO] Reading archiveMetrics: %s", id)
	getResp, err := r.client.Get(ctx)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if cxsdk.Code(err) == codes.NotFound {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("archiveMetrics %q is in state, but no longer exists in Coralogix backend", id),
				fmt.Sprintf("%s will be recreated when you apply", id),
			)
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError(
				"Error reading archive-metrics",
				utils.FormatRpcErrors(err, cxsdk.ArchiveMetricsGetTenantConfigRPC, ""),
			)
		}
		return
	}
	log.Printf("[INFO] Received archive-metrics: %s", protojson.Format(getResp))

	state, diags = flattenArchiveMetrics(ctx, getResp.GetTenantConfig(), id)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
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

	updateReq, diags := extractArchiveMetrics(ctx, *plan)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	log.Printf("[INFO] Updating new archiveMetrics: %s", protojson.Format(updateReq))
	_, err := r.client.ConfigureTenant(ctx, updateReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError(
			"Error updating archive-metrics",
			utils.FormatRpcErrors(err, cxsdk.E2MCreateRPC, protojson.Format(updateReq)),
		)
		return
	}
	log.Print("[INFO] Submitted updated archive-metrics")

	readResp, err := r.client.Get(ctx)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError(
			"Error reading archive-metrics",
			utils.FormatRpcErrors(err, cxsdk.ArchiveMetricsGetTenantConfigRPC, ""),
		)
		return
	}
	log.Printf("[INFO] Read updated archive-metrics %s", protojson.Format(readResp))
	plan, diags = flattenArchiveMetrics(ctx, readResp.GetTenantConfig(), plan.ID.ValueString())
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

// Safeguard against empty ID string, as using empty string causes problems when this provider is used in Pulumi via https://github.com/pulumi/pulumi-terraform-provider
const RESOURCE_ID_ARCHIVE_METRICS string = "archive-metrics-settings"
