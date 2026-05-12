// Copyright 2026 Coralogix Ltd.
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

package dataplans

import (
	"context"
	"fmt"
	"net/http"
	"time"

	cxsdkOpenapi "github.com/coralogix/coralogix-management-sdk/go/openapi/cxsdk"
	quotaRules "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/quota_allocation_rule_set_service"
	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
)

var _ datasource.DataSourceWithConfigure = &QuotaAllocationRuleSetDataSource{}

func NewQuotaAllocationRuleSetDataSource() datasource.DataSource {
	return &QuotaAllocationRuleSetDataSource{}
}

type QuotaAllocationRuleSetDataSource struct {
	client *quotaRules.QuotaAllocationRuleSetServiceAPIService
}

func (d *QuotaAllocationRuleSetDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_quota_allocation_rule_set"
}

func (d *QuotaAllocationRuleSetDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	clientSet, ok := req.ProviderData.(*clientset.ClientSet)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *clientset.ClientSet, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	d.client = clientSet.QuotaAllocationRules()
}

func (d *QuotaAllocationRuleSetDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads the current account-level Coralogix quota allocation rule set. Requires `team-quota-rules:Read` permission.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The backend identifier for the quota allocation rule set.",
			},
			"rules": schema.SetNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Current quota allocation rules returned by the backend.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"entity_type": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Entity type covered by the rule.",
						},
						"allocation": schema.Float64Attribute{
							Computed:            true,
							MarkdownDescription: "Quota allocation percentage for this entity type.",
						},
						"enabled": schema.BoolAttribute{
							Computed:            true,
							MarkdownDescription: "Whether the quota allocation rule is enabled.",
						},
						"can_overflow": schema.BoolAttribute{
							Computed:            true,
							MarkdownDescription: "Whether this entity type can overflow beyond its allocation.",
						},
					},
				},
			},
		},
	}
}

func (d *QuotaAllocationRuleSetDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	result, httpResponse, err := getQuotaAllocationRuleSet(ctx, d.client, "")
	if err != nil {
		if responseStatus(httpResponse) == http.StatusNotFound {
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError("Error reading coralogix_quota_allocation_rule_set",
			utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Read", nil),
		)
		return
	}

	state, diags := flattenGetQuotaAllocationRuleSetResponse(result)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}
