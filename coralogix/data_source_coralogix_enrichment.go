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
	"terraform-provider-coralogix/coralogix/utils"

	cxsdk "github.com/coralogix/coralogix-management-sdk/go"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"google.golang.org/grpc/codes"
)

var _ datasource.DataSourceWithConfigure = &EnrichmentDataSource{}

func NewEnrichmentDataSource() datasource.DataSource {
	return &EnrichmentDataSource{}
}

type EnrichmentDataSource struct {
	client *cxsdk.EnrichmentsClient
}

func (d *EnrichmentDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_enrichment"
}

func (d *EnrichmentDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

	d.client = clientSet.Enrichments()
}

type EnrichmentReadableModel struct {
	ID types.String `tfsdk:"id"`
}

func (d *EnrichmentDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	var r EnrichmentResource
	var resourceResp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &resourceResp)

	resp.Schema = utils.FrameworkDatasourceSchemaFromFrameworkResourceSchema(resourceResp.Schema)
}

func (d *EnrichmentDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data EnrichmentReadableModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	//Get refreshed Enrichment value from Coralogix
	id := data.ID.ValueString()

	log.Printf("[INFO] Reading Enrichment: %s", id)
	var err error
	var enrichments []*cxsdk.Enrichment
	if id == AWS_TYPE || id == GEOIP_TYPE || id == SUSIP_TYPE {
		enrichments, err = EnrichmentsByType(ctx, d.client, id)
	} else {
		enrichments, err = EnrichmentsByID(ctx, d.client, utils.StrToUint32(id))
	}

	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if cxsdk.Code(err) == codes.NotFound {
			resp.Diagnostics.AddWarning(err.Error(),
				fmt.Sprintf("Enrichment %q is in state, but no longer exists in Coralogix backend", id))
		} else {
			resp.Diagnostics.AddError(
				"Error reading Enrichment",
				utils.FormatRpcErrors(err, cxsdk.GetEnrichmentsRPC, id),
			)
		}
		return
	}

	state := flattenEnrichments(enrichments)

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}
