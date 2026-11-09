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

package dataengine

import (
	"context"
	"fmt"

	"github.com/coralogix/coralogix-management-sdk/go/openapi/cxsdk"
	retss "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/retentions_service"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"
	"github.com/coralogix/terraform-provider-coralogix/internal/provider/generated/archiveretentions"
)

// retentionCount is the number of retention tags that the released resource manages.
const retentionCount = 4

// NewArchiveRetentionsResource returns the generated coralogix_archive_retentions with its
// handwritten hooks. The rename names each tag by its id, which only the server sets: Create reads
// the tags and sends their ids by position, and Update sends the ids of the state.
func NewArchiveRetentionsResource() resource.Resource {
	h := &archiveRetentionsHooks{}
	generated := archiveretentions.NewResource(h.hooks())
	return &archiveRetentionsResource{Resource: generated.(*archiveretentions.Resource), hooks: h}
}

// NewArchiveRetentionsDataSource returns coralogix_archive_retentions, the generated data source of
// the resource. It reads the retention tags of the company and takes no input.
func NewArchiveRetentionsDataSource() datasource.DataSource {
	return archiveretentions.NewDataSource(archiveretentions.Hooks{})
}

// archiveRetentionsResource keeps the client that the Create hook reads the tags with.
type archiveRetentionsResource struct {
	*archiveretentions.Resource
	hooks *archiveRetentionsHooks
}

func (r *archiveRetentionsResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.Resource.Configure(ctx, req, resp)
	if cs, ok := req.ProviderData.(*clientset.ClientSet); ok && !resp.Diagnostics.HasError() {
		r.hooks.client = cs.ArchiveRetentions()
	}
}

type archiveRetentionsHooks struct {
	client *retss.RetentionsServiceAPIService
}

func (h *archiveRetentionsHooks) hooks() archiveretentions.Hooks {
	return archiveretentions.Hooks{BeforeWrite: h.beforeWrite, ValidateConfig: validateRetentions}
}

// beforeWrite sends the ids of the tags and the name of the first one. The plan has no ids before
// the first write, so Create reads the tags and takes their ids by position. A name that the
// configuration leaves out arrives as "", which the API rejects, so it keeps the current name of the
// tag. The first tag, Default, cannot be renamed, and the configuration leaves its name out.
func (h *archiveRetentionsHooks) beforeWrite(ctx context.Context, _ tfsdk.Config, body any) diag.Diagnostics {
	var diags diag.Diagnostics
	req, ok := body.(*retss.UpdateRetentionsRequest)
	if !ok || len(req.RetentionUpdateElements) == 0 {
		return diags
	}
	items := req.RetentionUpdateElements
	if items[0].Id == "" || hasLeftOutName(items) {
		existing, httpResp, err := h.client.RetentionsServiceGetRetentions(ctx).Execute()
		if err != nil {
			diags.AddError("Error reading coralogix_archive_retentions", cxsdk.NewAPIError(httpResp, err).Error())
			return diags
		}
		if len(existing.Retentions) < len(items) {
			diags.AddError("Error creating coralogix_archive_retentions",
				fmt.Sprintf("the Coralogix backend returned %d archive retentions, but the configuration defines %d; "+
					"archive retentions are created by Coralogix and can only be renamed, so the configuration cannot "+
					"declare more retentions than the backend has", len(existing.Retentions), len(items)))
			return diags
		}
		for i := range items {
			if items[i].Id == "" {
				items[i].Id = existing.Retentions[i].Id
			}
			if items[i].Name == "" {
				items[i].Name = existing.Retentions[i].Name
			}
		}
	}
	items[0].Name = "Default"
	return diags
}

func hasLeftOutName(items []retss.RetentionUpdateElement) bool {
	for _, item := range items[1:] {
		if item.Name == "" {
			return true
		}
	}
	return false
}

// validateRetentions keeps the released checks: four retentions, and no name on the first one.
func validateRetentions(ctx context.Context, config tfsdk.Config) diag.Diagnostics {
	var diags diag.Diagnostics
	var retentions types.List
	diags.Append(config.GetAttribute(ctx, path.Root("retentions"), &retentions)...)
	if diags.HasError() || retentions.IsNull() || retentions.IsUnknown() {
		return diags
	}
	items := retentions.Elements()
	if len(items) != retentionCount {
		diags.AddAttributeError(path.Root("retentions"), "error on validating retentions",
			fmt.Sprintf("retentions list must have %d elements but got %d", retentionCount, len(items)))
		return diags
	}
	for i, item := range items {
		object, ok := item.(types.Object)
		if !ok {
			continue
		}
		name, ok := object.Attributes()["name"].(types.String)
		if !ok || name.IsNull() || name.IsUnknown() {
			continue
		}
		at := path.Root("retentions").AtListIndex(i).AtName("name")
		switch {
		case i == 0:
			diags.AddAttributeError(at, "error on validating retentions", "first retention's name can't be set")
		case name.ValueString() == "":
			// The API rejects an empty name. Leave the name out to keep the current one.
			diags.AddAttributeError(at, "error on validating retentions", "a retention name can't be empty")
		}
	}
	return diags
}
