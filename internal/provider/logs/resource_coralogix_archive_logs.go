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

package logs

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	cxsdkOpenapi "github.com/coralogix/coralogix-management-sdk/go/openapi/cxsdk"
	archiveLogs "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/target_service"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Safeguard against empty ID string, as using empty string causes problems when this provider is used in Pulumi via https://github.com/pulumi/pulumi-terraform-provider
const RESOURCE_ID_ARCHIVE_LOGS string = "archive-logs-settings"

var (
	_ resource.ResourceWithConfigure   = &ArchiveLogsResource{}
	_ resource.ResourceWithImportState = &ArchiveLogsResource{}
)

type ArchiveLogsResourceModel struct {
	ID                types.String `tfsdk:"id"`
	Active            types.Bool   `tfsdk:"active"`
	Bucket            types.String `tfsdk:"bucket"`
	ArchivingFormatId types.String `tfsdk:"archiving_format_id"`
	Region            types.String `tfsdk:"region"`
	EnableTags        types.Bool   `tfsdk:"enable_tags"`
}

func NewArchiveLogsResource() resource.Resource {
	return &ArchiveLogsResource{}
}

type ArchiveLogsResource struct {
	client *archiveLogs.TargetServiceAPIService
}

func (r *ArchiveLogsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *ArchiveLogsResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

	r.client = clientSet.ArchiveLogs()
}

func (r *ArchiveLogsResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_archive_logs"
}

func (r ArchiveLogsResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Version: 0,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"bucket": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The bucket name to store the archived logs in.",
			},
			"active": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(true),
			},
			"archiving_format_id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"enable_tags": schema.BoolAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"region": schema.StringAttribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				MarkdownDescription: "The bucket region. see - https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/Concepts.RegionsAndAvailabilityZones.html#Concepts.RegionsAndAvailabilityZones.Regions",
			},
		},
	}
}

func (r *ArchiveLogsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// Retrieve values from plan
	var plan *ArchiveLogsResourceModel
	diags := req.Plan.Get(ctx, &plan)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	rq := extractArchiveLogs(*plan)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	result, httpResponse, err := r.client.
		S3TargetServiceSetTarget(ctx).
		SetTargetResponse(rq).
		Execute()

	if err != nil {
		resp.Diagnostics.AddError("Error replacing coralogix_archive_logs", utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Create", rq))
		return
	}

	plan = flattenArchiveLogs(result.Target.TargetS3, RESOURCE_ID_ARCHIVE_LOGS)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

func flattenArchiveLogs(targetS3 *archiveLogs.TargetS3, id string) *ArchiveLogsResourceModel {
	if targetS3 == nil {
		return nil
	}
	return &ArchiveLogsResourceModel{
		ID:                types.StringValue(id),
		Active:            types.BoolValue(targetS3.ArchiveSpec.GetIsActive()),
		Bucket:            types.StringValue(targetS3.GetS3().Bucket),
		Region:            types.StringPointerValue(targetS3.GetS3().Region),
		ArchivingFormatId: types.StringValue(targetS3.ArchiveSpec.GetArchivingFormatId()),
		EnableTags:        types.BoolValue(targetS3.ArchiveSpec.GetEnableTags()),
	}
}

func extractArchiveLogs(plan ArchiveLogsResourceModel) archiveLogs.SetTargetResponse {
	return archiveLogs.SetTargetResponse{
		IsActive: plan.Active.ValueBool(),
		S3: &archiveLogs.S3TargetSpec{
			Bucket: plan.Bucket.ValueString(),
			Region: utils.TypeStringToStringPointer(plan.Region),
		},
	}
}

func (r *ArchiveLogsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state *ArchiveLogsResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	//Get refreshed ArchiveLogs value from Coralogix
	id := state.ID.ValueString()
	rq := r.client.S3TargetServiceGetTarget(ctx)
	result, httpResponse, err := rq.Execute()
	if err != nil {
		if httpResponse.StatusCode == http.StatusNotFound {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("coralogix_archive_logs %q is in state, but no longer exists in Coralogix backend", id),
				fmt.Sprintf("%s will be recreated when you apply", id),
			)
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError("Error reading coralogix_archive_logs",
				utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Read", nil),
			)
		}
		return
	}

	state = flattenArchiveLogs(result.Target.TargetS3, id)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *ArchiveLogsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Retrieve values from plan
	var plan *ArchiveLogsResourceModel
	diags := req.Plan.Get(ctx, &plan)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	rq := extractArchiveLogs(*plan)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	result, httpResponse, err := r.client.
		S3TargetServiceSetTarget(ctx).
		SetTargetResponse(rq).
		Execute()

	if err != nil {
		if httpResponse.StatusCode == http.StatusNotFound {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("coralogix_archive_logs %v is in state, but no longer exists in Coralogix backend", rq),
				fmt.Sprintf("%v will be recreated when you apply", rq),
			)
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError("Error replacing coralogix_archive_logs", utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Replace", rq))
		}
		return
	}

	plan = flattenArchiveLogs(result.Target.TargetS3, plan.ID.ValueString())
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

func (r *ArchiveLogsResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// This API doesn't support deletion :(
	log.Printf("[INFO] coralogix_archive_logs cannot be deleted")
}
