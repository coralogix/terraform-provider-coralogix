package enrichment_rules

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	cxsdkOpenapi "github.com/coralogix/coralogix-management-sdk/go/openapi/cxsdk"
	cess "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/custom_enrichments_service"
	ess "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/enrichments_service"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

var (
	_ resource.ResourceWithConfigure   = &DataEnrichmentsResource{}
	_ resource.ResourceWithImportState = &DataEnrichmentsResource{}
)

var _ datasource.DataSourceWithConfigure = &DataEnrichmentDataSource{}

func NewDataEnrichmentDataSource() datasource.DataSource {
	return &DataEnrichmentDataSource{}
}

type DataEnrichmentDataSource struct {
	client                    *ess.EnrichmentsServiceAPIService
	custom_enrichments_client *cess.CustomEnrichmentsServiceAPIService
}

func (d *DataEnrichmentDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_data_enrichments"
}

func (d *DataEnrichmentDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

	d.client, d.custom_enrichments_client = clientSet.DataEnrichments()
}

func (d *DataEnrichmentDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	var r DataEnrichmentsResource
	var resourceResp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &resourceResp)

	resp.Schema = utils.FrameworkDatasourceSchemaFromFrameworkResourceSchema(resourceResp.Schema)

	if idAttr, ok := resp.Schema.Attributes[CUSTOM_TYPE].(schema.StringAttribute); ok {
		idAttr.Required = false
		idAttr.Optional = true
		resp.Schema.Attributes[CUSTOM_TYPE] = idAttr
	}

}

func (d *DataEnrichmentDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data *DataEnrichmentsModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := data.ID.ValueString()
	types := strings.Split(id, ",")

	customEnrichmentId := getCustomEnrichmentId(data)
	if len(types) == 0 && customEnrichmentId == nil {
		resp.Diagnostics.AddError("Error reading coralogix_data_enrichments",
			"No ids found",
		)
		return
	}

	var customEnrichment *cess.CustomEnrichment = nil
	if customEnrichmentId != nil {
		result, httpResponse, err := d.custom_enrichments_client.
			CustomEnrichmentServiceGetCustomEnrichment(ctx, *customEnrichmentId).
			Execute()
		if err != nil {
			if httpResponse.StatusCode == http.StatusNotFound {
				resp.Diagnostics.AddWarning(
					"coralogix_data_enrichments is in state, but no longer exists in Coralogix backend",
					"coralogix_data_enrichments will be recreated when you apply",
				)
				resp.State.RemoveResource(ctx)
			} else {
				resp.Diagnostics.AddError("Error reading coralogix_data_enrichments",
					utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Read", nil),
				)
			}
			return
		}
		customEnrichment = &result.CustomEnrichment
	}
	var enrichments []ess.Enrichment
	if len(types) > 0 {
		result, httpResponse, err := d.client.
			EnrichmentServiceGetEnrichments(ctx).
			Execute()
		if err != nil {
			if httpResponse.StatusCode == http.StatusNotFound {
				resp.Diagnostics.AddWarning(
					"coralogix_data_enrichments is in state, but no longer exists in Coralogix backend",
					"coralogix_data_enrichments will be recreated when you apply",
				)
				resp.State.RemoveResource(ctx)
			} else {
				resp.Diagnostics.AddError("Error reading coralogix_data_enrichments",
					utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Read", nil),
				)
			}
			return
		}
		for _, t := range types {
			enrichments = append(enrichments, FilterEnrichmentByTypes(result.Enrichments, t)...)
		}
	}

	var content *string = nil
	if customEnrichmentId != nil {
		content = data.Custom.CustomEnrichmentDataModel.Contents.ValueStringPointer()
	}
	data = flattenDataEnrichments(enrichments,
		customEnrichment,
		content)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
