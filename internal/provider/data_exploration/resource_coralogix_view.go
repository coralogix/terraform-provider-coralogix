// Copyright 2025 Coralogix Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package data_exploration

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"time"

	views "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/views_service"
	"github.com/coralogix/terraform-provider-coralogix/coralogix/clientset"
	"github.com/coralogix/terraform-provider-coralogix/coralogix/utils"

	cxsdk "github.com/coralogix/coralogix-management-sdk/go"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/objectvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int32planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

var _ resource.Resource = (*ViewResource)(nil)

func NewViewResource() resource.Resource {
	return &ViewResource{}
}

type ViewResource struct {
	client *views.ViewsServiceAPIService
}

func (r *ViewResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_view"
}

func (r *ViewResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

	r.client = clientSet.Views()
}

func (r *ViewResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.Int32Attribute{
				Computed:            true,
				Description:         "id",
				MarkdownDescription: "id",
				PlanModifiers: []planmodifier.Int32{
					int32planmodifier.UseStateForUnknown(),
				},
			},
			"folder_id": schema.StringAttribute{
				Optional:            true,
				Description:         "Unique identifier for folders",
				MarkdownDescription: "Unique identifier for folders",
				Validators: []validator.String{
					stringvalidator.LengthBetween(36, 36),
					stringvalidator.RegexMatches(regexp.MustCompile("^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$"), ""),
				},
			},
			"name": schema.StringAttribute{
				Required:            true,
				Description:         "View name",
				MarkdownDescription: "View name",
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"search_query": schema.SingleNestedAttribute{
				Attributes: map[string]schema.Attribute{
					"query": schema.StringAttribute{
						Required: true,
						Validators: []validator.String{
							stringvalidator.LengthAtLeast(1),
						},
					},
				},
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.Object{
					objectplanmodifier.UseStateForUnknown(),
				},
			},
			"filters": schema.SingleNestedAttribute{
				Attributes: map[string]schema.Attribute{
					"filters": schema.ListNestedAttribute{
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"name": schema.StringAttribute{
									Required:            true,
									Description:         "Filter name",
									MarkdownDescription: "Filter name",
									Validators: []validator.String{
										stringvalidator.LengthAtLeast(1),
									},
								},
								"selected_values": schema.MapAttribute{
									ElementType:         types.BoolType,
									Required:            true,
									Description:         "Filter selected values",
									MarkdownDescription: "Filter selected values",
								},
							},
						},
						Optional: true,
						Computed: true,
						Validators: []validator.List{
							listvalidator.SizeAtLeast(1),
						},
						PlanModifiers: []planmodifier.List{
							listplanmodifier.UseStateForUnknown(),
						},
					},
				},
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.Object{
					objectplanmodifier.UseStateForUnknown(),
				},
			},
			"time_selection": schema.SingleNestedAttribute{
				Attributes: map[string]schema.Attribute{
					"custom_selection": schema.SingleNestedAttribute{
						Attributes: map[string]schema.Attribute{
							"from_time": schema.StringAttribute{
								Required: true,
								Validators: []validator.String{
									stringvalidator.LengthAtLeast(1),
								},
							},
							"to_time": schema.StringAttribute{
								Required: true,
								Validators: []validator.String{
									stringvalidator.LengthAtLeast(1),
								},
							},
						},
						Validators: []validator.Object{
							objectvalidator.ExactlyOneOf(
								path.MatchRoot("time_selection").AtName("quick_selection"),
							),
						},
						Optional: true,
					},
					"quick_selection": schema.SingleNestedAttribute{
						Attributes: map[string]schema.Attribute{
							"seconds": schema.Int64Attribute{
								Required:            true,
								Description:         "Folder name",
								MarkdownDescription: "Folder name",
							},
						},
						Optional: true,
					},
				},
				Required: true,
			},
		},
	}
}

func (r *ViewResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *ViewResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data *ViewModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Create API call logic
	createViewRequest, diags := extractCreateView(ctx, data)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	viewStr := protojson.Format(createViewRequest)
	log.Printf("[INFO] Creating new view: %s", viewStr)
	createViewResponse, err := r.client.Create(ctx, createViewRequest)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err)
		resp.Diagnostics.AddError("Error creating View",
			utils.FormatRpcErrors(err, cxsdk.CreateActionRPC, viewStr),
		)
		return
	}
	log.Printf("[INFO] View created successfully: %s", protojson.Format(createViewResponse.View))

	// Save data into Terraform state
	data, diags = flattenView(ctx, createViewResponse.View)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, data)...)
}

func flattenView(ctx context.Context, view *views.View) (*ViewModel, diag.Diagnostics) {
	filters, diags := flattenViewFilter(ctx, view.Filters)
	if diags.HasError() {
		return nil, diags
	}

	searchQuery, diags := flattenSearchQuery(ctx, view.SearchQuery)
	if diags.HasError() {
		return nil, diags
	}

	timeSelection, diags := flattenViewTimeSelection(ctx, &view.TimeSelection)
	if diags.HasError() {
		return nil, diags
	}

	return &ViewModel{
		Filters:       filters,
		FolderId:      utils.WrapperspbStringToTypeString(view.FolderId),
		Id:            types.Int32Value(view.Id),
		Name:          utils.WrapperspbStringToTypeString(view.Name),
		SearchQuery:   searchQuery,
		TimeSelection: timeSelection,
	}, nil
}

func flattenViewTimeSelection(ctx context.Context, selection *views.TimeSelection) (types.Object, diag.Diagnostics) {
	if selection == nil {
		return TimeSelectionModel{
			CustomSelection: types.ObjectNull(CustomSelectionModel{}.AttributeTypes(ctx)),
			QuickSelection:  types.ObjectNull(QuickSelectionModel{}.AttributeTypes(ctx)),
		}.ToObjectValue(ctx)
	}

	if quickSelection := selection.GetQuickSelection(); quickSelection != nil {
		qs, diags := flattenQuickSelection(ctx, quickSelection)
		if diags.HasError() {
			return types.ObjectNull(TimeSelectionModel{}.AttributeTypes(ctx)), diags
		}

		return TimeSelectionModel{
			QuickSelection:  qs,
			CustomSelection: types.ObjectNull(CustomSelectionModel{}.AttributeTypes(ctx)),
		}.ToObjectValue(ctx)
	}

	if customSelection := selection.GetCustomSelection(); customSelection != nil {
		cs, diags := flattenCustomSelection(ctx, customSelection)
		if diags.HasError() {
			return types.ObjectNull(TimeSelectionModel{}.AttributeTypes(ctx)), diags
		}
		return TimeSelectionModel{
			CustomSelection: cs,
			QuickSelection:  types.ObjectNull(QuickSelectionModel{}.AttributeTypes(ctx)),
		}.ToObjectValue(ctx)
	}

	return types.ObjectNull(TimeSelectionModel{}.AttributeTypes(ctx)), diag.Diagnostics{
		diag.NewErrorDiagnostic(
			"Invalid Time Selection",
			"Time selection must have either quick selection or custom selection defined.",
		),
	}
}

func flattenCustomSelection(ctx context.Context, selection *cxsdk.CustomTimeSelection) (types.Object, diag.Diagnostics) {
	if selection == nil {
		return types.ObjectNull(CustomSelectionModel{}.AttributeTypes(ctx)), nil
	}

	customSelectionModel := CustomSelectionModel{
		FromTime: types.StringValue(selection.FromTime.AsTime().Format(time.RFC3339)),
		ToTime:   types.StringValue(selection.ToTime.AsTime().Format(time.RFC3339)),
	}

	return customSelectionModel.ToObjectValue(ctx)
}

func flattenQuickSelection(ctx context.Context, selection *cxsdk.QuickTimeSelection) (types.Object, diag.Diagnostics) {
	if selection == nil {
		return types.ObjectNull(QuickSelectionModel{}.AttributeTypes(ctx)), nil
	}

	quickSelectionModel := QuickSelectionModel{
		Seconds: types.Int64Value(int64(selection.Seconds)),
	}

	return quickSelectionModel.ToObjectValue(ctx)
}

func flattenSearchQuery(ctx context.Context, query *views.SearchQuery) (types.Object, diag.Diagnostics) {
	if query == nil {
		return types.ObjectNull(SearchQueryModel{}.AttributeTypes(ctx)), nil
	}

	return SearchQueryModel{
		Query: utils.WrapperspbStringToTypeString(query.Query),
	}.ToObjectValue(ctx)
}

func flattenViewFilter(ctx context.Context, filters *views.SelectedFilters) (types.Object, diag.Diagnostics) {
	if filters == nil {
		return types.ObjectNull(FiltersModel{}.AttributeTypes()), nil
	}

	innerFilters, diags := flattenInnerViewFilters(ctx, filters.Filters)
	if diags.HasError() {
		return types.ObjectNull(FiltersModel{}.AttributeTypes()), diags
	}

	return FiltersModel{
		Filters: innerFilters,
	}.ToObjectValue(ctx)
}

func flattenInnerViewFilters(ctx context.Context, filters []views.ViewsV1Filter) (basetypes.ListValue, diag.Diagnostics) {
	if filters == nil {
		return types.ListNull(types.ObjectType{AttrTypes: InnerFiltersModel{}.AttributeTypes()}), nil
	}

	innerFilters := make([]InnerFiltersModel, 0, len(filters))
	var diags diag.Diagnostics
	for i := range filters {
		var selectedValues basetypes.MapValue
		if filters[i].SelectedValues == nil {
			selectedValues = types.MapNull(types.BoolType)
		} else {
			var dgs diag.Diagnostics
			selectedValues, dgs = types.MapValueFrom(ctx, types.BoolType, filters[i].SelectedValues)
			if dgs.HasError() {
				diags.Append(dgs...)
				continue
			}
		}

		innerFilters = append(innerFilters, InnerFiltersModel{
			Name:           utils.WrapperspbStringToTypeString(filters[i].Name),
			SelectedValues: selectedValues,
		})
	}

	if diags.HasError() {
		return types.ListNull(types.ObjectType{AttrTypes: InnerFiltersModel{}.AttributeTypes()}), diags
	}

	return types.ListValueFrom(ctx, types.ObjectType{AttrTypes: InnerFiltersModel{}.AttributeTypes()}, innerFilters)
}

func extractCreateView(ctx context.Context, data *ViewModel) (*cxsdk.CreateViewRequest, diag.Diagnostics) {
	filters, diags := expandSelectedFilters(ctx, data.Filters)
	if diags.HasError() {
		return nil, diags
	}

	timeSelection, diags := expandViewTimeSelection(ctx, data.TimeSelection)
	if diags.HasError() {
		return nil, diags
	}

	searchQuery, diags := expandViewSearchQuery(ctx, data.SearchQuery)
	if diags.HasError() {
		return nil, diags
	}

	return &cxsdk.CreateViewRequest{
		Name:          utils.TypeStringToWrapperspbString(data.Name),
		SearchQuery:   searchQuery,
		TimeSelection: timeSelection,
		Filters:       filters,
		FolderId:      utils.TypeStringToWrapperspbString(data.FolderId),
	}, nil
}

func extractUpdateView(ctx context.Context, data *ViewModel) (*cxsdk.ReplaceViewRequest, diag.Diagnostics) {
	filters, diags := expandSelectedFilters(ctx, data.Filters)
	if diags.HasError() {
		return nil, diags
	}

	timeSelection, diags := expandViewTimeSelection(ctx, data.TimeSelection)
	if diags.HasError() {
		return nil, diags
	}

	searchQuery, diags := expandViewSearchQuery(ctx, data.SearchQuery)
	if diags.HasError() {
		return nil, diags
	}

	return &cxsdk.ReplaceViewRequest{
		View: &cxsdk.View{
			Id:            wrapperspb.Int32(int32(data.Id.ValueInt32())),
			Name:          utils.TypeStringToWrapperspbString(data.Name),
			SearchQuery:   searchQuery,
			TimeSelection: timeSelection,
			Filters:       filters,
			FolderId:      utils.TypeStringToWrapperspbString(data.FolderId),
		},
	}, nil
}

func expandSelectedFilters(ctx context.Context, filtersObject types.Object) (*cxsdk.SelectedFilters, diag.Diagnostics) {
	if filtersObject.IsNull() || filtersObject.IsUnknown() {
		return nil, nil
	}

	ov, _ := filtersObject.ToObjectValue(ctx)
	var filters FiltersModel
	if dg := ov.As(ctx, &filters, basetypes.ObjectAsOptions{}); dg.HasError() {
		return nil, diag.Diagnostics{
			diag.NewErrorDiagnostic(
				"Invalid Filters Object",
				fmt.Sprintf("Expected FiltersModel, got: %T. Please report this issue to the provider developers.", filtersObject),
			),
		}
	}
	innerFilters, diags := expandViewFilters(ctx, filters.Filters)
	if diags.HasError() {
		return nil, diags
	}

	return &cxsdk.SelectedFilters{
		Filters: innerFilters,
	}, nil

}

func expandViewFilters(ctx context.Context, filters basetypes.ListValue) ([]*cxsdk.ViewFilter, diag.Diagnostics) {
	if filters.IsNull() || filters.IsUnknown() {
		return nil, nil
	}

	var diags diag.Diagnostics
	var filtersObjects []types.Object
	diags = filters.ElementsAs(ctx, &filtersObjects, true)
	innerFilters := make([]*cxsdk.ViewFilter, 0, len(filtersObjects))
	for _, fo := range filtersObjects {
		var innerFilterValue InnerFiltersModel
		if dg := fo.As(ctx, &innerFilterValue, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}

		var selectedValues map[string]bool
		if dg := innerFilterValue.SelectedValues.ElementsAs(ctx, &selectedValues, true); dg.HasError() {
			diags.Append(dg...)
			continue
		}
		innerFilters = append(innerFilters, &cxsdk.ViewFilter{
			Name:           utils.TypeStringToWrapperspbString(innerFilterValue.Name),
			SelectedValues: selectedValues,
		})
	}

	if diags.HasError() {
		return nil, diags
	}

	return innerFilters, nil
}

func expandViewSearchQuery(ctx context.Context, queryObject types.Object) (*cxsdk.SearchQuery, diag.Diagnostics) {
	if queryObject.IsNull() || queryObject.IsUnknown() {
		return nil, nil
	}

	ov, _ := queryObject.ToObjectValue(ctx)
	var query SearchQueryModel
	if dg := ov.As(ctx, &query, basetypes.ObjectAsOptions{}); dg.HasError() {
		return nil, diag.Diagnostics{
			diag.NewErrorDiagnostic(
				"Invalid Search Query Object",
				fmt.Sprintf("Expected SearchQueryModel, got: %T. Please report this issue to the provider developers.", queryObject),
			),
		}
	}

	return &cxsdk.SearchQuery{
		Query: utils.TypeStringToWrapperspbString(query.Query),
	}, nil
}

func expandViewTimeSelection(ctx context.Context, selectionObject types.Object) (*cxsdk.TimeSelection, diag.Diagnostics) {
	if selectionObject.IsNull() || selectionObject.IsUnknown() {
		return nil, nil
	}

	var selection TimeSelectionModel
	if dg := selectionObject.As(ctx, &selection, basetypes.ObjectAsOptions{}); dg.HasError() {
		return nil, diag.Diagnostics{
			diag.NewErrorDiagnostic(
				"Invalid Time Selection Object",
				fmt.Sprintf("Expected TimeSelectionModel, got: %T. Please report this issue to the provider developers.", selectionObject),
			),
		}
	}

	if quickSelection := selection.QuickSelection; !(quickSelection.IsNull() || quickSelection.IsUnknown()) {
		qs, diags := expandQuickSelection(ctx, selection.QuickSelection)
		if diags.HasError() {
			return nil, diags
		}
		return &cxsdk.TimeSelection{
			SelectionType: &cxsdk.ViewTimeSelectionQuick{
				QuickSelection: qs,
			},
		}, nil
	} else if customSelection := selection.CustomSelection; !(customSelection.IsNull() || customSelection.IsUnknown()) {
		cs, diags := expandCustomSelection(ctx, selection.CustomSelection)
		if diags.HasError() {
			return nil, diags
		}
		return &cxsdk.TimeSelection{
			SelectionType: &cxsdk.ViewTimeSelectionCustom{
				CustomSelection: cs,
			},
		}, nil
	}

	return nil, diag.Diagnostics{
		diag.NewErrorDiagnostic(
			"Invalid Time Selection",
			"Time selection must have either quick selection or custom selection defined.",
		),
	}
}

func expandCustomSelection(ctx context.Context, selection types.Object) (*cxsdk.CustomTimeSelection, diag.Diagnostics) {
	if selection.IsNull() || selection.IsUnknown() {
		return nil, nil
	}

	attributes := selection.Attributes()
	fromTimeAttr, ok := attributes["from_time"]
	if !ok {
		return nil, nil
	}
	toTimeAttr, ok := attributes["to_time"]
	if !ok {
		return nil, nil
	}

	fromTime, ok := fromTimeAttr.(types.String)
	if !ok || fromTime.IsNull() || fromTime.IsUnknown() {
		return nil, nil
	}
	toTime, ok := toTimeAttr.(types.String)
	if !ok || toTime.IsNull() || toTime.IsUnknown() {
		return nil, nil
	}

	ft, err := time.Parse(time.RFC3339, fromTime.ValueString())
	if err != nil {
		return nil, diag.Diagnostics{
			diag.NewErrorDiagnostic(
				"Invalid From Time Format",
				fmt.Sprintf("From time '%s' is not in RFC3339 format: %s", fromTime.ValueString(), err.Error()),
			),
		}
	}
	tt, err := time.Parse(time.RFC3339, toTime.ValueString())
	if err != nil {
		return nil, diag.Diagnostics{
			diag.NewErrorDiagnostic(
				"Invalid To Time Format",
				fmt.Sprintf("To time '%s' is not in RFC3339 format: %s", toTime.ValueString(), err.Error()),
			),
		}
	}

	return &cxsdk.CustomTimeSelection{
		FromTime: timestamppb.New(ft),
		ToTime:   timestamppb.New(tt),
	}, nil
}

func expandQuickSelection(ctx context.Context, selection types.Object) (*cxsdk.QuickTimeSelection, diag.Diagnostics) {
	if selection.IsNull() || selection.IsUnknown() {
		return nil, nil
	}

	attributes := selection.Attributes()
	secondsAttr, ok := attributes["seconds"]
	if !ok {
		return nil, nil
	}

	seconds, ok := secondsAttr.(types.Int64)
	if !ok || seconds.IsNull() || seconds.IsUnknown() {
		return nil, nil
	}

	if seconds.ValueInt64() < 0 {
		return nil, diag.Diagnostics{
			diag.NewErrorDiagnostic(
				"Invalid Seconds Value",
				fmt.Sprintf("Seconds value '%d' cannot be negative.", seconds.ValueInt64()),
			),
		}
	}

	return &cxsdk.QuickTimeSelection{
		Seconds: uint32(seconds.ValueInt64()),
	}, nil
}

func (r *ViewResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data *ViewModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	idStr := data.Id.ValueString()
	id, err := strconv.Atoi(idStr)
	if err != nil {
		resp.Diagnostics.AddError(
			"Invalid View ID",
			fmt.Sprintf("ID '%s' is not a valid integer: %s", idStr, err.Error()),
		)
		return
	}

	readReq := &cxsdk.GetViewRequest{
		Id: wrapperspb.Int32(int32(id)),
	}
	log.Printf("[INFO] Reading view with ID: %s", idStr)
	readResp, err := r.client.Get(ctx, readReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if cxsdk.Code(err) == codes.NotFound {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("View %q is in state, but no longer exists in Coralogix backend", idStr),
				fmt.Sprintf("%s will be recreated when you apply", idStr),
			)
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError(
				"Error reading view",
				utils.FormatRpcErrors(err, cxsdk.GetViewRPC, protojson.Format(readReq)),
			)
		}
		return
	}
	log.Printf("[INFO] View read successfully: %s", protojson.Format(readResp.View))

	// Flatten the response into the model
	data, diags := flattenView(ctx, readResp.View)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ViewResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data *ViewModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save updated data into Terraform state
	updateReq, diags := extractUpdateView(ctx, data)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	log.Printf("[INFO] Updating view in state: %s", protojson.Format(updateReq))
	updateResp, err := r.client.Replace(ctx, updateReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError(
			"Error updating view in state",
			utils.FormatRpcErrors(err, cxsdk.ReplaceViewRPC, protojson.Format(updateReq)),
		)
		return
	}
	log.Printf("[INFO] View updated in state successfully: %s", protojson.Format(updateResp.View))

	// Flatten the response into the model
	data, diags = flattenView(ctx, updateResp.View)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ViewResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data *ViewModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	id := data.Id
	rq := r.client.ViewsServiceDeleteView(ctx, id.ValueInt32())
	_, _, err := rq.Execute()
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if cxsdk.Code(err) == codes.NotFound {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("View %q is in state, but no longer exists in Coralogix backend", id),
				fmt.Sprintf("%d will be removed from state", id),
			)
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError(
				"Error deleting view",
				utils.FormatRpcErrors(err, cxsdk.DeleteViewRPC, fmt.Sprintf("ID: %d", id)),
			)
		}
		return
	}
	if resp.Diagnostics.HasError() {
		return
	}
}

type ViewModel struct {
	Filters       types.Object `tfsdk:"filters"` //FiltersModel
	FolderId      types.String `tfsdk:"folder_id"`
	Id            types.Int32  `tfsdk:"id"`
	Name          types.String `tfsdk:"name"`
	SearchQuery   types.Object `tfsdk:"search_query"`   //SearchQueryModel
	TimeSelection types.Object `tfsdk:"time_selection"` // TimeSelectionModel
}

type QuickSelectionModel struct {
	Seconds types.Int64 `tfsdk:"seconds"`
}

func (v QuickSelectionModel) ToObjectValue(ctx context.Context) (types.Object, diag.Diagnostics) {
	return types.ObjectValueFrom(ctx, v.AttributeTypes(ctx), v)
}

func (v QuickSelectionModel) AttributeTypes(ctx context.Context) map[string]attr.Type {
	return map[string]attr.Type{
		"seconds": basetypes.Int64Type{},
	}
}

type FiltersModel struct {
	Filters types.List `tfsdk:"filters"` // InnerFiltersModel
}

func (v FiltersModel) ToObjectValue(ctx context.Context) (types.Object, diag.Diagnostics) {
	return types.ObjectValueFrom(ctx, v.AttributeTypes(), v)
}

func (v FiltersModel) AttributeTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"filters": basetypes.ListType{
			ElemType: basetypes.ObjectType{
				AttrTypes: InnerFiltersModel{}.AttributeTypes(),
			},
		},
	}
}

type InnerFiltersModel struct {
	Name           types.String `tfsdk:"name"`
	SelectedValues types.Map    `tfsdk:"selected_values"`
}

func (v InnerFiltersModel) ToObjectValue(ctx context.Context) (types.Object, diag.Diagnostics) {
	return types.ObjectValueFrom(ctx, v.AttributeTypes(), v)
}

func (v InnerFiltersModel) AttributeTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"name": basetypes.StringType{},
		"selected_values": basetypes.MapType{
			ElemType: types.BoolType,
		},
	}
}

type SearchQueryModel struct {
	Query types.String `tfsdk:"query"`
}

func (v SearchQueryModel) ToObjectValue(ctx context.Context) (types.Object, diag.Diagnostics) {
	return types.ObjectValueFrom(ctx, v.AttributeTypes(ctx), v)
}

func (v SearchQueryModel) AttributeTypes(ctx context.Context) map[string]attr.Type {
	return map[string]attr.Type{
		"query": basetypes.StringType{},
	}
}

type TimeSelectionModel struct {
	CustomSelection types.Object `tfsdk:"custom_selection"` //CustomSelectionModel
	QuickSelection  types.Object `tfsdk:"quick_selection"`  //QuickSelectionModel
}

func (v TimeSelectionModel) ToObjectValue(ctx context.Context) (types.Object, diag.Diagnostics) {
	return types.ObjectValueFrom(ctx, v.AttributeTypes(ctx), v)
}

func (v TimeSelectionModel) AttributeTypes(ctx context.Context) map[string]attr.Type {
	return map[string]attr.Type{
		"custom_selection": basetypes.ObjectType{
			AttrTypes: CustomSelectionModel{}.AttributeTypes(ctx),
		},
		"quick_selection": basetypes.ObjectType{
			AttrTypes: QuickSelectionModel{}.AttributeTypes(ctx),
		},
	}
}

type CustomSelectionModel struct {
	FromTime types.String `tfsdk:"from_time"`
	ToTime   types.String `tfsdk:"to_time"`
}

func (v CustomSelectionModel) ToObjectValue(ctx context.Context) (types.Object, diag.Diagnostics) {
	return types.ObjectValueFrom(ctx, v.AttributeTypes(ctx), v)
}

func (v CustomSelectionModel) AttributeTypes(context.Context) map[string]attr.Type {
	return map[string]attr.Type{
		"from_time": basetypes.StringType{},
		"to_time":   basetypes.StringType{},
	}
}
