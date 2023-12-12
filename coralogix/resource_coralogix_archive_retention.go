package coralogix

import (
	"context"
	"fmt"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"google.golang.org/protobuf/encoding/protojson"
	"terraform-provider-coralogix/coralogix/clientset"
	archiveRetention "terraform-provider-coralogix/coralogix/clientset/grpc/archive-retentions"
)

var (
	_                           resource.ResourceWithConfigure   = &ArchiveRetentionsResource{}
	_                           resource.ResourceWithImportState = &ArchiveRetentionsResource{}
	getArchiveRetentionsURL                                      = "com.coralogix.archive.v1.RetentionsService/GetRetentions"
	updateArchiveRetentionsURL                                   = "com.coralogix.archive.v1.RetentionsService/UpdateRetentions"
	activeArchiveRetentionsURL                                   = "com.coralogix.archive.v1.RetentionsService/ActivateRetentions"
	enablesArchiveRetentionsURL                                  = "com.coralogix.archive.v1.RetentionsService/GetRetentionsEnabled"
)

func NewArchiveRetentionsResource() resource.Resource {
	return &ArchiveRetentionsResource{}
}

type ArchiveRetentionsResource struct {
	client *clientset.ArchiveRetentionsClient
}

func (r *ArchiveRetentionsResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_archive_retentions"
}

func (r *ArchiveRetentionsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	//TODO implement me
}

func (r *ArchiveRetentionsResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Version: 0,
		Attributes: map[string]schema.Attribute{
			"retentions": schema.ListNestedAttribute{
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Computed:            true,
							Optional:            true,
							MarkdownDescription: "The retention id.",
						},
						"order": schema.Int64Attribute{
							Computed:            true,
							MarkdownDescription: "The retention order. Computed by the order of the retention in the retentions list definition.",
						},
						"name": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "The retention name.",
						},
						"editable": schema.BoolAttribute{
							Computed:            true,
							MarkdownDescription: "Is the retention editable.",
						},
					},
				},
				Required: true,
			},
		},
		MarkdownDescription: "Coralogix archive-retention. For more info please review - https://coralogix.com/docs/archive-setup-grpc-api/.",
	}
}

type ArchiveRetentionsResourceModel struct {
	Retentions types.List `tfsdk:"retentions"` //ArchiveRetentionResourceModel
}

type ArchiveRetentionResourceModel struct {
	Id       types.String `tfsdk:"id"`
	Order    types.Int64  `tfsdk:"order"`
	Name     types.String `tfsdk:"name"`
	Editable types.Bool   `tfsdk:"editable"`
}

func (r *ArchiveRetentionsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan *ArchiveRetentionsResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	createArchiveRetentions, diags := extractUpdateArchiveRetentions(ctx, plan)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	archiveRetentionsStr := protojson.Format(createArchiveRetentions)
	log.Printf("[INFO] Updating archive-retentions: %s", archiveRetentionsStr)
	updateResp, err := r.client.UpdateRetentions(ctx, createArchiveRetentions)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		diags.AddError(
			"Error creating archive-retentions",
			formatRpcErrors(err, updateArchiveRetentionsURL, archiveRetentionsStr),
		)
		return
	}
	log.Printf("[INFO] Submitted updated archive-retentions: %s", protojson.Format(updateResp))

	plan, diags = flattenArchiveRetentions(ctx, updateResp.GetRetentions())

	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

func flattenArchiveRetentions(ctx context.Context, retentions []*archiveRetention.Retention) (*ArchiveRetentionsResourceModel, diag.Diagnostics) {
	if len(retentions) == 0 {
		r, _ := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: archiveRetentionAttributes()}, []types.Object{})
		return &ArchiveRetentionsResourceModel{
			Retentions: r,
		}, nil
	}

	var diags diag.Diagnostics
	var retentionsObjects []types.Object
	for _, retention := range retentions {
		retentionModel := ArchiveRetentionResourceModel{
			Id:       wrapperspbStringToTypeString(retention.GetId()),
			Order:    wrapperspbInt32ToTypeInt64(retention.GetOrder()),
			Name:     wrapperspbStringToTypeString(retention.GetName()),
			Editable: wrapperspbBoolToTypeBool(retention.GetEditable()),
		}
		retentionObject, flattenDiags := types.ObjectValueFrom(ctx, archiveRetentionAttributes(), retentionModel)
		if flattenDiags.HasError() {
			diags.Append(flattenDiags...)
			continue
		}
		retentionsObjects = append(retentionsObjects, retentionObject)
	}
	if diags.HasError() {
		return nil, diags
	}

	flattenedRetentions, diags := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: archiveRetentionAttributes()}, retentionsObjects)
	if diags.HasError() {
		return nil, diags
	}

	return &ArchiveRetentionsResourceModel{
		Retentions: flattenedRetentions,
	}, nil
}

func archiveRetentionAttributes() map[string]attr.Type {
	return map[string]attr.Type{
		"id":       types.StringType,
		"order":    types.Int64Type,
		"name":     types.StringType,
		"editable": types.BoolType,
	}
}

func extractUpdateArchiveRetentions(ctx context.Context, plan *ArchiveRetentionsResourceModel) (*archiveRetention.UpdateRetentionsRequest, diag.Diagnostics) {
	var diags diag.Diagnostics
	var retentions []*archiveRetention.RetentionUpdateElement
	var retentionsObjects []types.Object
	plan.Retentions.ElementsAs(ctx, &retentionsObjects, true)
	for _, retentionObject := range retentionsObjects {
		var retentionModel ArchiveRetentionResourceModel
		if dg := retentionObject.As(ctx, &retentionModel, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}
		retentions = append(retentions, &archiveRetention.RetentionUpdateElement{
			Id:   typeStringToWrapperspbString(retentionModel.Id),
			Name: typeStringToWrapperspbString(retentionModel.Name),
		})
	}
	return &archiveRetention.UpdateRetentionsRequest{
		RetentionUpdateElements: retentions,
	}, diags
}

func (r *ArchiveRetentionsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state *ArchiveRetentionsResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	log.Print("[INFO] Reading archive-retentions")
	getArchiveRetentionsReq := &archiveRetention.GetRetentionsRequest{}
	getArchiveRetentionsResp, err := r.client.GetRetentions(ctx, getArchiveRetentionsReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		formatRpcErrors(err, getArchiveRetentionsURL, protojson.Format(getArchiveRetentionsReq))
		return
	}
	log.Printf("[INFO] Received archive-retentions: %s", protojson.Format(getArchiveRetentionsResp))

	state, diags = flattenArchiveRetentions(ctx, getArchiveRetentionsResp.GetRetentions())
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

func (r *ArchiveRetentionsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Retrieve values from plan
	var plan *ArchiveRetentionsResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	archiveRetentionsUpdateReq, diags := extractUpdateArchiveRetentions(ctx, plan)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	log.Printf("[INFO] Updating archive-retentions: %s", protojson.Format(archiveRetentionsUpdateReq))
	archiveRetentionsUpdateResp, err := r.client.UpdateRetentions(ctx, archiveRetentionsUpdateReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		resp.Diagnostics.AddError(
			"Error updating archive-retentions",
			formatRpcErrors(err, updateArchiveRetentionsURL, protojson.Format(archiveRetentionsUpdateReq)),
		)
		return
	}
	log.Printf("[INFO] Submitted updated archive-retentions: %s", protojson.Format(archiveRetentionsUpdateResp))

	// Get refreshed archive-retentions value from Coralogix
	getArchiveRetentionsReq := &archiveRetention.GetRetentionsRequest{}
	getArchiveRetentionsResp, err := r.client.GetRetentions(ctx, getArchiveRetentionsReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError(
			"Error reading archive-retentions",
			formatRpcErrors(err, getArchiveRetentionsURL, protojson.Format(getArchiveRetentionsReq)),
		)
		return
	}
	log.Printf("[INFO] Received archive-retentions: %s", protojson.Format(getArchiveRetentionsResp))

	plan, diags = flattenArchiveRetentions(ctx, getArchiveRetentionsResp.GetRetentions())
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

func (r *ArchiveRetentionsResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state ArchiveRetentionsResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	log.Print("[INFO] Deleting archive-retentions")
	deleteReq := &archiveRetention.UpdateRetentionsRequest{}
	if _, err := r.client.UpdateRetentions(ctx, deleteReq); err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting archive-retentions",
			formatRpcErrors(err, updateArchiveRetentionsURL, protojson.Format(deleteReq)),
		)
		return
	}
	log.Print("[INFO] archive-retentions were deleted")
}

func (r *ArchiveRetentionsResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

	r.client = clientSet.ArchiveRetentions()
}
