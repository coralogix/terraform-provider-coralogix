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

package coralogix

import (
	"context"
	"fmt"
	"log"

	"terraform-provider-coralogix/coralogix/clientset"

	cxsdk "github.com/coralogix/coralogix-management-sdk/go"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
)

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
	client *cxsdk.ArchiveLogsClient
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

	createReq := extractArchiveLogs(*plan)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	log.Printf("[INFO] Creating new archive-logs: %s", protojson.Format(createReq))
	createResp, err := r.client.Update(ctx, createReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError(
			"Error creating archive-logs",
			formatRpcErrors(err, cxsdk.ArchiveLogsSetTargetRPC, protojson.Format(createReq)),
		)
		return
	}
	log.Printf("[INFO] Submitted new archive-logs: %s", protojson.Format(createResp))

	plan = flattenArchiveLogs(createResp.GetTarget())
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

func flattenArchiveLogs(target *cxsdk.Target) *ArchiveLogsResourceModel {
	if target == nil {
		return nil
	}
	s3Target, ok := target.GetTargetSpec().(*cxsdk.TargetS3)
	if !ok {
		return nil
	}

	return &ArchiveLogsResourceModel{
		ID:                types.StringValue(""),
		Active:            types.BoolValue(target.ArchiveSpec.GetIsActive()),
		Bucket:            types.StringValue(s3Target.S3.GetBucket()),
		Region:            types.StringValue(s3Target.S3.GetRegion()),
		ArchivingFormatId: types.StringValue(target.ArchiveSpec.GetArchivingFormatId()),
		EnableTags:        types.BoolValue(target.ArchiveSpec.GetEnableTags()),
	}
}

func extractArchiveLogs(plan ArchiveLogsResourceModel) *cxsdk.SetTargetRequest {
	return &cxsdk.SetTargetRequest{
		IsActive: plan.Active.ValueBool(),
		TargetSpec: &cxsdk.SetTargetRequestS3{
			S3: &cxsdk.S3TargetSpec{
				Bucket: plan.Bucket.ValueString(),
				Region: plan.Region.ValueStringPointer(),
			},
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
	log.Printf("[INFO] Reading archive-logs: %s", id)
	getResp, err := r.client.Get(ctx)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if status.Code(err) == codes.NotFound {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("archive-logs %q is in state, but no longer exists in Coralogix backend", id),
				fmt.Sprintf("%s will be recreated when you apply", id),
			)
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError(
				"Error reading archive-logs",
				formatRpcErrors(err, cxsdk.ArchiveLogsGetTargetRPC, ""),
			)
		}
		return
	}
	log.Printf("[INFO] Received archive-logs: %s", protojson.Format(getResp))

	state = flattenArchiveLogs(getResp.GetTarget())
	//
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

func (r *ArchiveLogsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Retrieve values from plan
	var plan *ArchiveLogsResourceModel
	diags := req.Plan.Get(ctx, &plan)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	updateReq := extractArchiveLogs(*plan)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	log.Printf("[INFO] Updating archive-logs: %s", protojson.Format(updateReq))
	updateResp, err := r.client.Update(ctx, updateReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError(
			"Error updating archive-logs",
			formatRpcErrors(err, cxsdk.CreateE2MRequest, protojson.Format(updateReq)),
		)
		return
	}
	log.Printf("[INFO] Submitted updated archive-logs %s", protojson.Format(updateResp))

	readResp, err := r.client.Get(ctx)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError(
			"Error reading archive-logs",
			formatRpcErrors(err, cxsdk.ArchiveLogsGetTargetRPC, ""),
		)
		return
	}
	plan = flattenArchiveLogs(readResp.GetTarget())
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

func (r *ArchiveLogsResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {

}
