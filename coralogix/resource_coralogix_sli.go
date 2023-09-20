package coralogix

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"terraform-provider-coralogix/coralogix/clientset"
)

var (
	_ resource.ResourceWithConfigure   = &SLIResource{}
	_ resource.ResourceWithImportState = &SLIResource{}
)

func NewSLIResource() resource.Resource {
	return &SLIResource{}
}

type SLIResource struct {
	client *clientset.SLIClient
}

func (r SLIResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_sli"

}

func (r SLIResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

	r.client = clientSet.SLIs()
}

func (r SLIResource) ImportState(ctx context.Context, request resource.ImportStateRequest, response *resource.ImportStateResponse) {
	//TODO implement me
	panic("implement me")
}

func (r SLIResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Version: 0,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				MarkdownDescription: "SLI ID.",
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "SLI name.",
			},
			"service_name": schema.StringAttribute{
				Required: true,
			},
			"description": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Optional SLI description.",
			},
			"metric_name": schema.StringAttribute{},
			"metric_type": schema.StringAttribute{},
			"slo_percentage": schema.Int64Attribute{
				Required: true,
				Validators: []validator.Int64{
					int64validator.Between(0, 100),
				},
			},
			"slo_period_type": schema.StringAttribute{
				Required: true,
			},
			"threshold_symbol_type": schema.StringAttribute{},
			"threshold_value":       schema.Int64Attribute{},
			"filters": schema.ListAttribute{
				Elem: schema.StringAttribute{},
			},
			"slo_status_type":     schema.StringAttribute{},
			"error_budget":        schema.Int64Attribute{},
			"label_e2m_id":        schema.StringAttribute{},
			"total_e2m_id":        schema.StringAttribute{},
			"time_unit_type":      schema.StringAttribute{},
			"service_names_group": schema.ListAttribute{},
		},
	}
}

func (r SLIResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {

}

func (r SLIResource) Read(ctx context.Context, request resource.ReadRequest, response *resource.ReadResponse) {
	//TODO implement me
	panic("implement me")
}

func (r SLIResource) Update(ctx context.Context, request resource.UpdateRequest, response *resource.UpdateResponse) {
	//TODO implement me
	panic("implement me")
}

func (r SLIResource) Delete(ctx context.Context, request resource.DeleteRequest, response *resource.DeleteResponse) {
	//TODO implement me
	panic("implement me")
}
