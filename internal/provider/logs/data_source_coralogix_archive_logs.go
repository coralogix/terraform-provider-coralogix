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
	"net/http"

	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	cxsdkOpenapi "github.com/coralogix/coralogix-management-sdk/go/openapi/cxsdk"
	archiveLogs "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/target_service"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSourceWithConfigure = &ArchiveLogsDataSource{}

func NewArchiveLogsDataSource() datasource.DataSource {
	return &ArchiveLogsDataSource{}
}

type ArchiveLogsDataSource struct {
	client *archiveLogs.TargetServiceAPIService
}

func (d *ArchiveLogsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_archive_logs"
}

func (d *ArchiveLogsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

	d.client = clientSet.ArchiveLogs()
}

func (d *ArchiveLogsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = utils.ConvertSchemaWithoutID(archiveLogsSchema())
}

func (d *ArchiveLogsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data *ArchiveLogsResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := data.ID.ValueString()
	rq := d.client.S3TargetServiceGetTarget(ctx)
	result, httpResponse, err := rq.Execute()
	if err != nil {
		if httpResponse != nil && httpResponse.StatusCode == http.StatusNotFound {
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
	data = flattenArchiveLogs(result.Target.S3, result.Target.ArchiveSpec, id)
	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// The data source keeps the schema and flatten of the released resource until it is migrated
// itself. The resource is generated (internal/provider/generated/archivelogs).

type ArchiveLogsResourceModel struct {
	ID                types.String `tfsdk:"id"`
	Active            types.Bool   `tfsdk:"active"`
	Bucket            types.String `tfsdk:"bucket"`
	ArchivingFormatId types.String `tfsdk:"archiving_format_id"`
	Region            types.String `tfsdk:"region"`
	EnableTags        types.Bool   `tfsdk:"enable_tags"`
}

// archiveLogsSchema is the schema of the released resource.
func archiveLogsSchema() schema.Schema {
	return schema.Schema{
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
		MarkdownDescription: "Coralogix archive-logs. Configures the S3 bucket Coralogix archives logs and traces into. For more info please review - https://coralogix.com/docs/user-guides/data-flow/s3-archive/connect-s3-archive/.",
	}
}

func flattenArchiveLogs(targetS3 *archiveLogs.S3TargetSpec, archiveSpec archiveLogs.ArchiveSpec, id string) *ArchiveLogsResourceModel {
	if targetS3 == nil {
		return nil
	}
	return &ArchiveLogsResourceModel{
		ID:                types.StringValue(id),
		Active:            types.BoolValue(archiveSpec.GetIsActive()),
		Bucket:            types.StringValue(targetS3.Bucket),
		Region:            types.StringPointerValue(targetS3.Region),
		ArchivingFormatId: types.StringValue(archiveSpec.GetArchivingFormatId()),
		EnableTags:        types.BoolValue(archiveSpec.GetEnableTags()),
	}
}
