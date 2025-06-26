package coralogix

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strings"

	cxsdk "github.com/coralogix/coralogix-management-sdk/go"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/objectvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"google.golang.org/protobuf/encoding/protojson"
	"terraform-provider-coralogix/coralogix/clientset"
	"terraform-provider-coralogix/coralogix/utils"
)

var _ resource.Resource = (*ViewsResource)(nil)

func NewViewsResource() resource.Resource {
	return &ViewsResource{}
}

type ViewsResource struct {
	client *cxsdk.ViewsClient
}

func (r *ViewsResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_views"
}

func (r *ViewsResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *ViewsResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = ViewResourceSchema(ctx)
}

func (r *ViewsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
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
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func extractCreateView(ctx context.Context, data *ViewModel) (*cxsdk.CreateViewRequest, diag.Diagnostics) {
	return &cxsdk.CreateViewRequest{
		Name:          utils.TypeStringToWrapperspbString(data.Name),
		SearchQuery:   expandViewSearchQuery(ctx, data.SearchQuery),
		TimeSelection: expandViewTimeSelection(ctx, data.TimeSelection),
	}, nil
}

func expandViewSearchQuery(ctx context.Context, query SearchQueryValue) *cxsdk.SearchQuery {
	if query.IsNull() || query.IsUnknown() {
		return nil
	}

	return &cxsdk.SearchQuery{
		Query: utils.TypeStringToWrapperspbString(query.Query),
	}
}

func expandViewTimeSelection(ctx context.Context, selection TimeSelectionValue) *cxsdk.TimeSelection {
	if selection.IsNull() || selection.IsUnknown() {
		return nil
	}
	return nil
}

func (r *ViewsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data *ViewModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Read API call logic

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ViewsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data *ViewModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Update API call logic

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ViewsResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data *ViewModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Delete API call logic
}

func ViewResourceSchema(ctx context.Context) schema.Schema {
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
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
							CustomType: InnerFiltersType{
								ObjectType: types.ObjectType{
									AttrTypes: FiltersValue{}.AttributeTypes(ctx),
								},
							},
						},
						Optional: true,
						Computed: true,
						Validators: []validator.List{
							listvalidator.SizeAtLeast(1),
						},
					},
				},
				CustomType: FiltersType{
					ObjectType: types.ObjectType{
						AttrTypes: FiltersValue{}.AttributeTypes(ctx),
					},
				},
				Optional: true,
				Computed: true,
			},
			"folder_id": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				Description:         "Unique identifier for folders",
				MarkdownDescription: "Unique identifier for folders",
				Validators: []validator.String{
					stringvalidator.LengthBetween(36, 36),
					stringvalidator.RegexMatches(regexp.MustCompile("^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$"), ""),
				},
			},
			"id": schema.Int64Attribute{
				Computed:            true,
				Description:         "id",
				MarkdownDescription: "id",
			},
			"is_compact_mode": schema.BoolAttribute{
				Computed: true,
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
				CustomType: SearchQueryType{
					ObjectType: types.ObjectType{
						AttrTypes: SearchQueryValue{}.AttributeTypes(ctx),
					},
				},
				Optional: true,
				Computed: true,
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
						CustomType: CustomSelectionType{
							ObjectType: types.ObjectType{
								AttrTypes: CustomSelectionValue{}.AttributeTypes(ctx),
							},
						},
						Optional: true,
						Validators: []validator.Object{
							objectvalidator.ExactlyOneOf(path.MatchRelative().AtParent().AtName("quick_selection")),
						},
					},
					"quick_selection": schema.SingleNestedAttribute{
						Attributes: map[string]schema.Attribute{
							"caption": schema.StringAttribute{
								Optional:            true,
								Computed:            true,
								Description:         "Folder name",
								MarkdownDescription: "Folder name",
								Validators: []validator.String{
									stringvalidator.LengthAtLeast(1),
								},
							},
							"seconds": schema.Int64Attribute{
								Required:            true,
								Description:         "Folder name",
								MarkdownDescription: "Folder name",
							},
						},
						CustomType: QuickSelectionType{
							ObjectType: types.ObjectType{
								AttrTypes: QuickSelectionValue{}.AttributeTypes(ctx),
							},
						},
						Optional: true,
					},
				},
				CustomType: TimeSelectionType{
					ObjectType: types.ObjectType{
						AttrTypes: TimeSelectionValue{}.AttributeTypes(ctx),
					},
				},
				Required: true,
			},
		},
	}
}

type ViewModel struct {
	Filters       FiltersValue       `tfsdk:"filters"`
	FolderId      types.String       `tfsdk:"folder_id"`
	Id            types.Int64        `tfsdk:"id"`
	IsCompactMode types.Bool         `tfsdk:"is_compact_mode"`
	Name          types.String       `tfsdk:"name"`
	SearchQuery   SearchQueryValue   `tfsdk:"search_query"`
	TimeSelection TimeSelectionValue `tfsdk:"time_selection"`
}

var _ basetypes.ObjectTypable = FiltersType{}

type FiltersType struct {
	basetypes.ObjectType
}

func (t FiltersType) Equal(o attr.Type) bool {
	other, ok := o.(FiltersType)

	if !ok {
		return false
	}

	return t.ObjectType.Equal(other.ObjectType)
}

func (t FiltersType) String() string {
	return "FiltersType"
}

func (t FiltersType) ValueFromObject(ctx context.Context, in basetypes.ObjectValue) (basetypes.ObjectValuable, diag.Diagnostics) {
	var diags diag.Diagnostics

	attributes := in.Attributes()

	filtersAttribute, ok := attributes["filters"]

	if !ok {
		diags.AddError(
			"Attribute Missing",
			`filters is missing from object`)

		return nil, diags
	}

	filtersVal, ok := filtersAttribute.(basetypes.ListValue)

	if !ok {
		diags.AddError(
			"Attribute Wrong Type",
			fmt.Sprintf(`filters expected to be basetypes.ListValue, was: %T`, filtersAttribute))
	}

	if diags.HasError() {
		return nil, diags
	}

	return FiltersValue{
		Filters: filtersVal,
		state:   attr.ValueStateKnown,
	}, diags
}

func NewFiltersValueNull() FiltersValue {
	return FiltersValue{
		state: attr.ValueStateNull,
	}
}

func NewFiltersValueUnknown() FiltersValue {
	return FiltersValue{
		state: attr.ValueStateUnknown,
	}
}

func NewFiltersValue(attributeTypes map[string]attr.Type, attributes map[string]attr.Value) (FiltersValue, diag.Diagnostics) {
	var diags diag.Diagnostics

	// Reference: https://github.com/hashicorp/terraform-plugin-framework/issues/521
	ctx := context.Background()

	for name, attributeType := range attributeTypes {
		attribute, ok := attributes[name]

		if !ok {
			diags.AddError(
				"Missing FiltersValue Attribute Value",
				"While creating a FiltersValue value, a missing attribute value was detected. "+
					"A FiltersValue must contain values for all attributes, even if null or unknown. "+
					"This is always an issue with the provider and should be reported to the provider developers.\n\n"+
					fmt.Sprintf("FiltersValue Attribute Name (%s) Expected Type: %s", name, attributeType.String()),
			)

			continue
		}

		if !attributeType.Equal(attribute.Type(ctx)) {
			diags.AddError(
				"Invalid FiltersValue Attribute Type",
				"While creating a FiltersValue value, an invalid attribute value was detected. "+
					"A FiltersValue must use a matching attribute type for the value. "+
					"This is always an issue with the provider and should be reported to the provider developers.\n\n"+
					fmt.Sprintf("FiltersValue Attribute Name (%s) Expected Type: %s\n", name, attributeType.String())+
					fmt.Sprintf("FiltersValue Attribute Name (%s) Given Type: %s", name, attribute.Type(ctx)),
			)
		}
	}

	for name := range attributes {
		_, ok := attributeTypes[name]

		if !ok {
			diags.AddError(
				"Extra FiltersValue Attribute Value",
				"While creating a FiltersValue value, an extra attribute value was detected. "+
					"A FiltersValue must not contain values beyond the expected attribute types. "+
					"This is always an issue with the provider and should be reported to the provider developers.\n\n"+
					fmt.Sprintf("Extra FiltersValue Attribute Name: %s", name),
			)
		}
	}

	if diags.HasError() {
		return NewFiltersValueUnknown(), diags
	}

	filtersAttribute, ok := attributes["filters"]

	if !ok {
		diags.AddError(
			"Attribute Missing",
			`filters is missing from object`)

		return NewFiltersValueUnknown(), diags
	}

	filtersVal, ok := filtersAttribute.(basetypes.ListValue)

	if !ok {
		diags.AddError(
			"Attribute Wrong Type",
			fmt.Sprintf(`filters expected to be basetypes.ListValue, was: %T`, filtersAttribute))
	}

	if diags.HasError() {
		return NewFiltersValueUnknown(), diags
	}

	return FiltersValue{
		Filters: filtersVal,
		state:   attr.ValueStateKnown,
	}, diags
}

func NewFiltersValueMust(attributeTypes map[string]attr.Type, attributes map[string]attr.Value) FiltersValue {
	object, diags := NewFiltersValue(attributeTypes, attributes)

	if diags.HasError() {
		// This could potentially be added to the diag package.
		diagsStrings := make([]string, 0, len(diags))

		for _, diagnostic := range diags {
			diagsStrings = append(diagsStrings, fmt.Sprintf(
				"%s | %s | %s",
				diagnostic.Severity(),
				diagnostic.Summary(),
				diagnostic.Detail()))
		}

		panic("NewFiltersValueMust received error(s): " + strings.Join(diagsStrings, "\n"))
	}

	return object
}

func (t FiltersType) ValueFromTerraform(ctx context.Context, in tftypes.Value) (attr.Value, error) {
	if in.Type() == nil {
		return NewFiltersValueNull(), nil
	}

	if !in.Type().Equal(t.TerraformType(ctx)) {
		return nil, fmt.Errorf("expected %s, got %s", t.TerraformType(ctx), in.Type())
	}

	if !in.IsKnown() {
		return NewFiltersValueUnknown(), nil
	}

	if in.IsNull() {
		return NewFiltersValueNull(), nil
	}

	attributes := map[string]attr.Value{}

	val := map[string]tftypes.Value{}

	err := in.As(&val)

	if err != nil {
		return nil, err
	}

	for k, v := range val {
		a, err := t.AttrTypes[k].ValueFromTerraform(ctx, v)

		if err != nil {
			return nil, err
		}

		attributes[k] = a
	}

	return NewFiltersValueMust(FiltersValue{}.AttributeTypes(ctx), attributes), nil
}

func (t FiltersType) ValueType(ctx context.Context) attr.Value {
	return FiltersValue{}
}

var _ basetypes.ObjectValuable = FiltersValue{}

type FiltersValue struct {
	Filters basetypes.ListValue `tfsdk:"filters"`
	state   attr.ValueState
}

func (v FiltersValue) ToTerraformValue(ctx context.Context) (tftypes.Value, error) {
	attrTypes := make(map[string]tftypes.Type, 1)

	var val tftypes.Value
	var err error

	attrTypes["filters"] = basetypes.ListType{
		ElemType: FiltersValue{}.Type(ctx),
	}.TerraformType(ctx)

	objectType := tftypes.Object{AttributeTypes: attrTypes}

	switch v.state {
	case attr.ValueStateKnown:
		vals := make(map[string]tftypes.Value, 1)

		val, err = v.Filters.ToTerraformValue(ctx)

		if err != nil {
			return tftypes.NewValue(objectType, tftypes.UnknownValue), err
		}

		vals["filters"] = val

		if err := tftypes.ValidateValue(objectType, vals); err != nil {
			return tftypes.NewValue(objectType, tftypes.UnknownValue), err
		}

		return tftypes.NewValue(objectType, vals), nil
	case attr.ValueStateNull:
		return tftypes.NewValue(objectType, nil), nil
	case attr.ValueStateUnknown:
		return tftypes.NewValue(objectType, tftypes.UnknownValue), nil
	default:
		panic(fmt.Sprintf("unhandled Object state in ToTerraformValue: %s", v.state))
	}
}

func (v FiltersValue) IsNull() bool {
	return v.state == attr.ValueStateNull
}

func (v FiltersValue) IsUnknown() bool {
	return v.state == attr.ValueStateUnknown
}

func (v FiltersValue) String() string {
	return "FiltersValue"
}

func (v FiltersValue) ToObjectValue(ctx context.Context) (basetypes.ObjectValue, diag.Diagnostics) {
	var diags diag.Diagnostics

	filters := types.ListValueMust(
		FiltersType{
			basetypes.ObjectType{
				AttrTypes: FiltersValue{}.AttributeTypes(ctx),
			},
		},
		v.Filters.Elements(),
	)

	if v.Filters.IsNull() {
		filters = types.ListNull(
			FiltersType{
				basetypes.ObjectType{
					AttrTypes: FiltersValue{}.AttributeTypes(ctx),
				},
			},
		)
	}

	if v.Filters.IsUnknown() {
		filters = types.ListUnknown(
			FiltersType{
				basetypes.ObjectType{
					AttrTypes: FiltersValue{}.AttributeTypes(ctx),
				},
			},
		)
	}

	attributeTypes := map[string]attr.Type{
		"filters": basetypes.ListType{
			ElemType: FiltersValue{}.Type(ctx),
		},
	}

	if v.IsNull() {
		return types.ObjectNull(attributeTypes), diags
	}

	if v.IsUnknown() {
		return types.ObjectUnknown(attributeTypes), diags
	}

	objVal, diags := types.ObjectValue(
		attributeTypes,
		map[string]attr.Value{
			"filters": filters,
		})

	return objVal, diags
}

func (v FiltersValue) Equal(o attr.Value) bool {
	other, ok := o.(FiltersValue)

	if !ok {
		return false
	}

	if v.state != other.state {
		return false
	}

	if v.state != attr.ValueStateKnown {
		return true
	}

	if !v.Filters.Equal(other.Filters) {
		return false
	}

	return true
}

func (v FiltersValue) Type(ctx context.Context) attr.Type {
	return FiltersType{
		basetypes.ObjectType{
			AttrTypes: v.AttributeTypes(ctx),
		},
	}
}

func (v FiltersValue) AttributeTypes(ctx context.Context) map[string]attr.Type {
	return map[string]attr.Type{
		"filters": basetypes.ListType{
			ElemType: FiltersValue{}.Type(ctx),
		},
	}
}

var _ basetypes.ObjectTypable = FiltersType{}

type InnerFiltersType struct {
	basetypes.ObjectType
}

func (t InnerFiltersType) Equal(o attr.Type) bool {
	other, ok := o.(InnerFiltersType)

	if !ok {
		return false
	}

	return t.ObjectType.Equal(other.ObjectType)
}

func (t InnerFiltersType) String() string {
	return "InnerFiltersType"
}

func (t InnerFiltersType) ValueFromObject(ctx context.Context, in basetypes.ObjectValue) (basetypes.ObjectValuable, diag.Diagnostics) {
	var diags diag.Diagnostics

	attributes := in.Attributes()

	nameAttribute, ok := attributes["name"]

	if !ok {
		diags.AddError(
			"Attribute Missing",
			`name is missing from object`)

		return nil, diags
	}

	nameVal, ok := nameAttribute.(basetypes.StringValue)

	if !ok {
		diags.AddError(
			"Attribute Wrong Type",
			fmt.Sprintf(`name expected to be basetypes.StringValue, was: %T`, nameAttribute))
	}

	selectedValuesAttribute, ok := attributes["selected_values"]

	if !ok {
		diags.AddError(
			"Attribute Missing",
			`selected_values is missing from object`)

		return nil, diags
	}

	selectedValuesVal, ok := selectedValuesAttribute.(basetypes.MapValue)

	if !ok {
		diags.AddError(
			"Attribute Wrong Type",
			fmt.Sprintf(`selected_values expected to be basetypes.MapValue, was: %T`, selectedValuesAttribute))
	}

	if diags.HasError() {
		return nil, diags
	}

	return InnerFiltersValue{
		Name:           nameVal,
		SelectedValues: selectedValuesVal,
		state:          attr.ValueStateKnown,
	}, diags
}

func NewInnerFiltersValueNull() InnerFiltersValue {
	return InnerFiltersValue{
		state: attr.ValueStateNull,
	}
}

func NewInnerFiltersValueUnknown() InnerFiltersValue {
	return InnerFiltersValue{
		state: attr.ValueStateUnknown,
	}
}

func NewInnerFiltersValue(attributeTypes map[string]attr.Type, attributes map[string]attr.Value) (InnerFiltersValue, diag.Diagnostics) {
	var diags diag.Diagnostics

	// Reference: https://github.com/hashicorp/terraform-plugin-framework/issues/521
	ctx := context.Background()

	for name, attributeType := range attributeTypes {
		attribute, ok := attributes[name]

		if !ok {
			diags.AddError(
				"Missing FiltersValue Attribute Value",
				"While creating a FiltersValue value, a missing attribute value was detected. "+
					"A FiltersValue must contain values for all attributes, even if null or unknown. "+
					"This is always an issue with the provider and should be reported to the provider developers.\n\n"+
					fmt.Sprintf("FiltersValue Attribute Name (%s) Expected Type: %s", name, attributeType.String()),
			)

			continue
		}

		if !attributeType.Equal(attribute.Type(ctx)) {
			diags.AddError(
				"Invalid FiltersValue Attribute Type",
				"While creating a FiltersValue value, an invalid attribute value was detected. "+
					"A FiltersValue must use a matching attribute type for the value. "+
					"This is always an issue with the provider and should be reported to the provider developers.\n\n"+
					fmt.Sprintf("FiltersValue Attribute Name (%s) Expected Type: %s\n", name, attributeType.String())+
					fmt.Sprintf("FiltersValue Attribute Name (%s) Given Type: %s", name, attribute.Type(ctx)),
			)
		}
	}

	for name := range attributes {
		_, ok := attributeTypes[name]

		if !ok {
			diags.AddError(
				"Extra FiltersValue Attribute Value",
				"While creating a FiltersValue value, an extra attribute value was detected. "+
					"A FiltersValue must not contain values beyond the expected attribute types. "+
					"This is always an issue with the provider and should be reported to the provider developers.\n\n"+
					fmt.Sprintf("Extra FiltersValue Attribute Name: %s", name),
			)
		}
	}

	if diags.HasError() {
		return NewInnerFiltersValueUnknown(), diags
	}

	nameAttribute, ok := attributes["name"]

	if !ok {
		diags.AddError(
			"Attribute Missing",
			`name is missing from object`)

		return NewInnerFiltersValueUnknown(), diags
	}

	nameVal, ok := nameAttribute.(basetypes.StringValue)

	if !ok {
		diags.AddError(
			"Attribute Wrong Type",
			fmt.Sprintf(`name expected to be basetypes.StringValue, was: %T`, nameAttribute))
	}

	selectedValuesAttribute, ok := attributes["selected_values"]

	if !ok {
		diags.AddError(
			"Attribute Missing",
			`selected_values is missing from object`)

		return NewInnerFiltersValueUnknown(), diags
	}

	selectedValuesVal, ok := selectedValuesAttribute.(basetypes.MapValue)

	if !ok {
		diags.AddError(
			"Attribute Wrong Type",
			fmt.Sprintf(`selected_values expected to be basetypes.MapValue, was: %T`, selectedValuesAttribute))
	}

	if diags.HasError() {
		return NewInnerFiltersValueUnknown(), diags
	}

	return InnerFiltersValue{
		Name:           nameVal,
		SelectedValues: selectedValuesVal,
		state:          attr.ValueStateKnown,
	}, diags
}

func NewInnerFiltersValueMust(attributeTypes map[string]attr.Type, attributes map[string]attr.Value) InnerFiltersValue {
	object, diags := NewInnerFiltersValue(attributeTypes, attributes)

	if diags.HasError() {
		// This could potentially be added to the diag package.
		diagsStrings := make([]string, 0, len(diags))

		for _, diagnostic := range diags {
			diagsStrings = append(diagsStrings, fmt.Sprintf(
				"%s | %s | %s",
				diagnostic.Severity(),
				diagnostic.Summary(),
				diagnostic.Detail()))
		}

		panic("NewFiltersValueMust received error(s): " + strings.Join(diagsStrings, "\n"))
	}

	return object
}

func (t InnerFiltersType) ValueFromTerraform(ctx context.Context, in tftypes.Value) (attr.Value, error) {
	if in.Type() == nil {
		return NewInnerFiltersValueNull(), nil
	}

	if !in.Type().Equal(t.TerraformType(ctx)) {
		return nil, fmt.Errorf("expected %s, got %s", t.TerraformType(ctx), in.Type())
	}

	if !in.IsKnown() {
		return NewInnerFiltersValueUnknown(), nil
	}

	if in.IsNull() {
		return NewInnerFiltersValueNull(), nil
	}

	attributes := map[string]attr.Value{}

	val := map[string]tftypes.Value{}

	err := in.As(&val)

	if err != nil {
		return nil, err
	}

	for k, v := range val {
		a, err := t.AttrTypes[k].ValueFromTerraform(ctx, v)

		if err != nil {
			return nil, err
		}

		attributes[k] = a
	}

	return NewInnerFiltersValueMust(FiltersValue{}.AttributeTypes(ctx), attributes), nil
}

func (t InnerFiltersType) ValueType(ctx context.Context) attr.Value {
	return FiltersValue{}
}

var _ basetypes.ObjectValuable = InnerFiltersValue{}

type InnerFiltersValue struct {
	Name           basetypes.StringValue `tfsdk:"name"`
	SelectedValues basetypes.MapValue    `tfsdk:"selected_values"`
	state          attr.ValueState
}

func (v InnerFiltersValue) ToTerraformValue(ctx context.Context) (tftypes.Value, error) {
	attrTypes := make(map[string]tftypes.Type, 2)

	var val tftypes.Value
	var err error

	attrTypes["name"] = basetypes.StringType{}.TerraformType(ctx)
	attrTypes["selected_values"] = basetypes.MapType{
		ElemType: types.BoolType,
	}.TerraformType(ctx)

	objectType := tftypes.Object{AttributeTypes: attrTypes}

	switch v.state {
	case attr.ValueStateKnown:
		vals := make(map[string]tftypes.Value, 2)

		val, err = v.Name.ToTerraformValue(ctx)

		if err != nil {
			return tftypes.NewValue(objectType, tftypes.UnknownValue), err
		}

		vals["name"] = val

		val, err = v.SelectedValues.ToTerraformValue(ctx)

		if err != nil {
			return tftypes.NewValue(objectType, tftypes.UnknownValue), err
		}

		vals["selected_values"] = val

		if err := tftypes.ValidateValue(objectType, vals); err != nil {
			return tftypes.NewValue(objectType, tftypes.UnknownValue), err
		}

		return tftypes.NewValue(objectType, vals), nil
	case attr.ValueStateNull:
		return tftypes.NewValue(objectType, nil), nil
	case attr.ValueStateUnknown:
		return tftypes.NewValue(objectType, tftypes.UnknownValue), nil
	default:
		panic(fmt.Sprintf("unhandled Object state in ToTerraformValue: %s", v.state))
	}
}

func (v InnerFiltersValue) IsNull() bool {
	return v.state == attr.ValueStateNull
}

func (v InnerFiltersValue) IsUnknown() bool {
	return v.state == attr.ValueStateUnknown
}

func (v InnerFiltersValue) String() string {
	return "InnerFiltersValue"
}

func (v InnerFiltersValue) ToObjectValue(ctx context.Context) (basetypes.ObjectValue, diag.Diagnostics) {
	var diags diag.Diagnostics

	var selectedValuesVal basetypes.MapValue
	switch {
	case v.SelectedValues.IsUnknown():
		selectedValuesVal = types.MapUnknown(types.BoolType)
	case v.SelectedValues.IsNull():
		selectedValuesVal = types.MapNull(types.BoolType)
	default:
		var d diag.Diagnostics
		selectedValuesVal, d = types.MapValue(types.BoolType, v.SelectedValues.Elements())
		diags.Append(d...)
	}

	if diags.HasError() {
		return types.ObjectUnknown(map[string]attr.Type{
			"name": basetypes.StringType{},
			"selected_values": basetypes.MapType{
				ElemType: types.BoolType,
			},
		}), diags
	}

	attributeTypes := map[string]attr.Type{
		"name": basetypes.StringType{},
		"selected_values": basetypes.MapType{
			ElemType: types.BoolType,
		},
	}

	if v.IsNull() {
		return types.ObjectNull(attributeTypes), diags
	}

	if v.IsUnknown() {
		return types.ObjectUnknown(attributeTypes), diags
	}

	objVal, diags := types.ObjectValue(
		attributeTypes,
		map[string]attr.Value{
			"name":            v.Name,
			"selected_values": selectedValuesVal,
		})

	return objVal, diags
}

func (v InnerFiltersValue) Equal(o attr.Value) bool {
	other, ok := o.(InnerFiltersValue)

	if !ok {
		return false
	}

	if v.state != other.state {
		return false
	}

	if v.state != attr.ValueStateKnown {
		return true
	}

	if !v.Name.Equal(other.Name) {
		return false
	}

	if !v.SelectedValues.Equal(other.SelectedValues) {
		return false
	}

	return true
}

func (v InnerFiltersValue) Type(ctx context.Context) attr.Type {
	return InnerFiltersType{
		basetypes.ObjectType{
			AttrTypes: v.AttributeTypes(ctx),
		},
	}
}

func (v InnerFiltersValue) AttributeTypes(ctx context.Context) map[string]attr.Type {
	return map[string]attr.Type{
		"name": basetypes.StringType{},
		"selected_values": basetypes.MapType{
			ElemType: types.BoolType,
		},
	}
}

var _ basetypes.ObjectTypable = SearchQueryType{}

type SearchQueryType struct {
	basetypes.ObjectType
}

func (t SearchQueryType) Equal(o attr.Type) bool {
	other, ok := o.(SearchQueryType)

	if !ok {
		return false
	}

	return t.ObjectType.Equal(other.ObjectType)
}

func (t SearchQueryType) String() string {
	return "SearchQueryType"
}

func (t SearchQueryType) ValueFromObject(ctx context.Context, in basetypes.ObjectValue) (basetypes.ObjectValuable, diag.Diagnostics) {
	var diags diag.Diagnostics

	attributes := in.Attributes()

	queryAttribute, ok := attributes["query"]

	if !ok {
		diags.AddError(
			"Attribute Missing",
			`query is missing from object`)

		return nil, diags
	}

	queryVal, ok := queryAttribute.(basetypes.StringValue)

	if !ok {
		diags.AddError(
			"Attribute Wrong Type",
			fmt.Sprintf(`query expected to be basetypes.StringValue, was: %T`, queryAttribute))
	}

	if diags.HasError() {
		return nil, diags
	}

	return SearchQueryValue{
		Query: queryVal,
		state: attr.ValueStateKnown,
	}, diags
}

func NewSearchQueryValueNull() SearchQueryValue {
	return SearchQueryValue{
		state: attr.ValueStateNull,
	}
}

func NewSearchQueryValueUnknown() SearchQueryValue {
	return SearchQueryValue{
		state: attr.ValueStateUnknown,
	}
}

func NewSearchQueryValue(attributeTypes map[string]attr.Type, attributes map[string]attr.Value) (SearchQueryValue, diag.Diagnostics) {
	var diags diag.Diagnostics

	// Reference: https://github.com/hashicorp/terraform-plugin-framework/issues/521
	ctx := context.Background()

	for name, attributeType := range attributeTypes {
		attribute, ok := attributes[name]

		if !ok {
			diags.AddError(
				"Missing SearchQueryValue Attribute Value",
				"While creating a SearchQueryValue value, a missing attribute value was detected. "+
					"A SearchQueryValue must contain values for all attributes, even if null or unknown. "+
					"This is always an issue with the provider and should be reported to the provider developers.\n\n"+
					fmt.Sprintf("SearchQueryValue Attribute Name (%s) Expected Type: %s", name, attributeType.String()),
			)

			continue
		}

		if !attributeType.Equal(attribute.Type(ctx)) {
			diags.AddError(
				"Invalid SearchQueryValue Attribute Type",
				"While creating a SearchQueryValue value, an invalid attribute value was detected. "+
					"A SearchQueryValue must use a matching attribute type for the value. "+
					"This is always an issue with the provider and should be reported to the provider developers.\n\n"+
					fmt.Sprintf("SearchQueryValue Attribute Name (%s) Expected Type: %s\n", name, attributeType.String())+
					fmt.Sprintf("SearchQueryValue Attribute Name (%s) Given Type: %s", name, attribute.Type(ctx)),
			)
		}
	}

	for name := range attributes {
		_, ok := attributeTypes[name]

		if !ok {
			diags.AddError(
				"Extra SearchQueryValue Attribute Value",
				"While creating a SearchQueryValue value, an extra attribute value was detected. "+
					"A SearchQueryValue must not contain values beyond the expected attribute types. "+
					"This is always an issue with the provider and should be reported to the provider developers.\n\n"+
					fmt.Sprintf("Extra SearchQueryValue Attribute Name: %s", name),
			)
		}
	}

	if diags.HasError() {
		return NewSearchQueryValueUnknown(), diags
	}

	queryAttribute, ok := attributes["query"]

	if !ok {
		diags.AddError(
			"Attribute Missing",
			`query is missing from object`)

		return NewSearchQueryValueUnknown(), diags
	}

	queryVal, ok := queryAttribute.(basetypes.StringValue)

	if !ok {
		diags.AddError(
			"Attribute Wrong Type",
			fmt.Sprintf(`query expected to be basetypes.StringValue, was: %T`, queryAttribute))
	}

	if diags.HasError() {
		return NewSearchQueryValueUnknown(), diags
	}

	return SearchQueryValue{
		Query: queryVal,
		state: attr.ValueStateKnown,
	}, diags
}

func NewSearchQueryValueMust(attributeTypes map[string]attr.Type, attributes map[string]attr.Value) SearchQueryValue {
	object, diags := NewSearchQueryValue(attributeTypes, attributes)

	if diags.HasError() {
		// This could potentially be added to the diag package.
		diagsStrings := make([]string, 0, len(diags))

		for _, diagnostic := range diags {
			diagsStrings = append(diagsStrings, fmt.Sprintf(
				"%s | %s | %s",
				diagnostic.Severity(),
				diagnostic.Summary(),
				diagnostic.Detail()))
		}

		panic("NewSearchQueryValueMust received error(s): " + strings.Join(diagsStrings, "\n"))
	}

	return object
}

func (t SearchQueryType) ValueFromTerraform(ctx context.Context, in tftypes.Value) (attr.Value, error) {
	if in.Type() == nil {
		return NewSearchQueryValueNull(), nil
	}

	if !in.Type().Equal(t.TerraformType(ctx)) {
		return nil, fmt.Errorf("expected %s, got %s", t.TerraformType(ctx), in.Type())
	}

	if !in.IsKnown() {
		return NewSearchQueryValueUnknown(), nil
	}

	if in.IsNull() {
		return NewSearchQueryValueNull(), nil
	}

	attributes := map[string]attr.Value{}

	val := map[string]tftypes.Value{}

	err := in.As(&val)

	if err != nil {
		return nil, err
	}

	for k, v := range val {
		a, err := t.AttrTypes[k].ValueFromTerraform(ctx, v)

		if err != nil {
			return nil, err
		}

		attributes[k] = a
	}

	return NewSearchQueryValueMust(SearchQueryValue{}.AttributeTypes(ctx), attributes), nil
}

func (t SearchQueryType) ValueType(ctx context.Context) attr.Value {
	return SearchQueryValue{}
}

var _ basetypes.ObjectValuable = SearchQueryValue{}

type SearchQueryValue struct {
	Query basetypes.StringValue `tfsdk:"query"`
	state attr.ValueState
}

func (v SearchQueryValue) ToTerraformValue(ctx context.Context) (tftypes.Value, error) {
	attrTypes := make(map[string]tftypes.Type, 1)

	var val tftypes.Value
	var err error

	attrTypes["query"] = basetypes.StringType{}.TerraformType(ctx)

	objectType := tftypes.Object{AttributeTypes: attrTypes}

	switch v.state {
	case attr.ValueStateKnown:
		vals := make(map[string]tftypes.Value, 1)

		val, err = v.Query.ToTerraformValue(ctx)

		if err != nil {
			return tftypes.NewValue(objectType, tftypes.UnknownValue), err
		}

		vals["query"] = val

		if err := tftypes.ValidateValue(objectType, vals); err != nil {
			return tftypes.NewValue(objectType, tftypes.UnknownValue), err
		}

		return tftypes.NewValue(objectType, vals), nil
	case attr.ValueStateNull:
		return tftypes.NewValue(objectType, nil), nil
	case attr.ValueStateUnknown:
		return tftypes.NewValue(objectType, tftypes.UnknownValue), nil
	default:
		panic(fmt.Sprintf("unhandled Object state in ToTerraformValue: %s", v.state))
	}
}

func (v SearchQueryValue) IsNull() bool {
	return v.state == attr.ValueStateNull
}

func (v SearchQueryValue) IsUnknown() bool {
	return v.state == attr.ValueStateUnknown
}

func (v SearchQueryValue) String() string {
	return "SearchQueryValue"
}

func (v SearchQueryValue) ToObjectValue(ctx context.Context) (basetypes.ObjectValue, diag.Diagnostics) {
	var diags diag.Diagnostics

	attributeTypes := map[string]attr.Type{
		"query": basetypes.StringType{},
	}

	if v.IsNull() {
		return types.ObjectNull(attributeTypes), diags
	}

	if v.IsUnknown() {
		return types.ObjectUnknown(attributeTypes), diags
	}

	objVal, diags := types.ObjectValue(
		attributeTypes,
		map[string]attr.Value{
			"query": v.Query,
		})

	return objVal, diags
}

func (v SearchQueryValue) Equal(o attr.Value) bool {
	other, ok := o.(SearchQueryValue)

	if !ok {
		return false
	}

	if v.state != other.state {
		return false
	}

	if v.state != attr.ValueStateKnown {
		return true
	}

	if !v.Query.Equal(other.Query) {
		return false
	}

	return true
}

func (v SearchQueryValue) Type(ctx context.Context) attr.Type {
	return SearchQueryType{
		basetypes.ObjectType{
			AttrTypes: v.AttributeTypes(ctx),
		},
	}
}

func (v SearchQueryValue) AttributeTypes(ctx context.Context) map[string]attr.Type {
	return map[string]attr.Type{
		"query": basetypes.StringType{},
	}
}

var _ basetypes.ObjectTypable = TimeSelectionType{}

type TimeSelectionType struct {
	basetypes.ObjectType
}

func (t TimeSelectionType) Equal(o attr.Type) bool {
	other, ok := o.(TimeSelectionType)

	if !ok {
		return false
	}

	return t.ObjectType.Equal(other.ObjectType)
}

func (t TimeSelectionType) String() string {
	return "TimeSelectionType"
}

func (t TimeSelectionType) ValueFromObject(ctx context.Context, in basetypes.ObjectValue) (basetypes.ObjectValuable, diag.Diagnostics) {
	var diags diag.Diagnostics

	attributes := in.Attributes()

	customSelectionAttribute, ok := attributes["custom_selection"]

	if !ok {
		diags.AddError(
			"Attribute Missing",
			`custom_selection is missing from object`)

		return nil, diags
	}

	customSelectionVal, ok := customSelectionAttribute.(basetypes.ObjectValue)

	if !ok {
		diags.AddError(
			"Attribute Wrong Type",
			fmt.Sprintf(`custom_selection expected to be basetypes.ObjectValue, was: %T`, customSelectionAttribute))
	}

	quickSelectionAttribute, ok := attributes["quick_selection"]

	if !ok {
		diags.AddError(
			"Attribute Missing",
			`quick_selection is missing from object`)

		return nil, diags
	}

	quickSelectionVal, ok := quickSelectionAttribute.(basetypes.ObjectValue)

	if !ok {
		diags.AddError(
			"Attribute Wrong Type",
			fmt.Sprintf(`quick_selection expected to be basetypes.ObjectValue, was: %T`, quickSelectionAttribute))
	}

	if diags.HasError() {
		return nil, diags
	}

	return TimeSelectionValue{
		CustomSelection: customSelectionVal,
		QuickSelection:  quickSelectionVal,
		state:           attr.ValueStateKnown,
	}, diags
}

func NewTimeSelectionValueNull() TimeSelectionValue {
	return TimeSelectionValue{
		state: attr.ValueStateNull,
	}
}

func NewTimeSelectionValueUnknown() TimeSelectionValue {
	return TimeSelectionValue{
		state: attr.ValueStateUnknown,
	}
}

func NewTimeSelectionValue(attributeTypes map[string]attr.Type, attributes map[string]attr.Value) (TimeSelectionValue, diag.Diagnostics) {
	var diags diag.Diagnostics

	// Reference: https://github.com/hashicorp/terraform-plugin-framework/issues/521
	ctx := context.Background()

	for name, attributeType := range attributeTypes {
		attribute, ok := attributes[name]

		if !ok {
			diags.AddError(
				"Missing TimeSelectionValue Attribute Value",
				"While creating a TimeSelectionValue value, a missing attribute value was detected. "+
					"A TimeSelectionValue must contain values for all attributes, even if null or unknown. "+
					"This is always an issue with the provider and should be reported to the provider developers.\n\n"+
					fmt.Sprintf("TimeSelectionValue Attribute Name (%s) Expected Type: %s", name, attributeType.String()),
			)

			continue
		}

		if !attributeType.Equal(attribute.Type(ctx)) {
			diags.AddError(
				"Invalid TimeSelectionValue Attribute Type",
				"While creating a TimeSelectionValue value, an invalid attribute value was detected. "+
					"A TimeSelectionValue must use a matching attribute type for the value. "+
					"This is always an issue with the provider and should be reported to the provider developers.\n\n"+
					fmt.Sprintf("TimeSelectionValue Attribute Name (%s) Expected Type: %s\n", name, attributeType.String())+
					fmt.Sprintf("TimeSelectionValue Attribute Name (%s) Given Type: %s", name, attribute.Type(ctx)),
			)
		}
	}

	for name := range attributes {
		_, ok := attributeTypes[name]

		if !ok {
			diags.AddError(
				"Extra TimeSelectionValue Attribute Value",
				"While creating a TimeSelectionValue value, an extra attribute value was detected. "+
					"A TimeSelectionValue must not contain values beyond the expected attribute types. "+
					"This is always an issue with the provider and should be reported to the provider developers.\n\n"+
					fmt.Sprintf("Extra TimeSelectionValue Attribute Name: %s", name),
			)
		}
	}

	if diags.HasError() {
		return NewTimeSelectionValueUnknown(), diags
	}

	customSelectionAttribute, ok := attributes["custom_selection"]

	if !ok {
		diags.AddError(
			"Attribute Missing",
			`custom_selection is missing from object`)

		return NewTimeSelectionValueUnknown(), diags
	}

	customSelectionVal, ok := customSelectionAttribute.(basetypes.ObjectValue)

	if !ok {
		diags.AddError(
			"Attribute Wrong Type",
			fmt.Sprintf(`custom_selection expected to be basetypes.ObjectValue, was: %T`, customSelectionAttribute))
	}

	quickSelectionAttribute, ok := attributes["quick_selection"]

	if !ok {
		diags.AddError(
			"Attribute Missing",
			`quick_selection is missing from object`)

		return NewTimeSelectionValueUnknown(), diags
	}

	quickSelectionVal, ok := quickSelectionAttribute.(basetypes.ObjectValue)

	if !ok {
		diags.AddError(
			"Attribute Wrong Type",
			fmt.Sprintf(`quick_selection expected to be basetypes.ObjectValue, was: %T`, quickSelectionAttribute))
	}

	if diags.HasError() {
		return NewTimeSelectionValueUnknown(), diags
	}

	return TimeSelectionValue{
		CustomSelection: customSelectionVal,
		QuickSelection:  quickSelectionVal,
		state:           attr.ValueStateKnown,
	}, diags
}

func NewTimeSelectionValueMust(attributeTypes map[string]attr.Type, attributes map[string]attr.Value) TimeSelectionValue {
	object, diags := NewTimeSelectionValue(attributeTypes, attributes)

	if diags.HasError() {
		// This could potentially be added to the diag package.
		diagsStrings := make([]string, 0, len(diags))

		for _, diagnostic := range diags {
			diagsStrings = append(diagsStrings, fmt.Sprintf(
				"%s | %s | %s",
				diagnostic.Severity(),
				diagnostic.Summary(),
				diagnostic.Detail()))
		}

		panic("NewTimeSelectionValueMust received error(s): " + strings.Join(diagsStrings, "\n"))
	}

	return object
}

func (t TimeSelectionType) ValueFromTerraform(ctx context.Context, in tftypes.Value) (attr.Value, error) {
	if in.Type() == nil {
		return NewTimeSelectionValueNull(), nil
	}

	if !in.Type().Equal(t.TerraformType(ctx)) {
		return nil, fmt.Errorf("expected %s, got %s", t.TerraformType(ctx), in.Type())
	}

	if !in.IsKnown() {
		return NewTimeSelectionValueUnknown(), nil
	}

	if in.IsNull() {
		return NewTimeSelectionValueNull(), nil
	}

	attributes := map[string]attr.Value{}

	val := map[string]tftypes.Value{}

	err := in.As(&val)

	if err != nil {
		return nil, err
	}

	for k, v := range val {
		a, err := t.AttrTypes[k].ValueFromTerraform(ctx, v)

		if err != nil {
			return nil, err
		}

		attributes[k] = a
	}

	return NewTimeSelectionValueMust(TimeSelectionValue{}.AttributeTypes(ctx), attributes), nil
}

func (t TimeSelectionType) ValueType(ctx context.Context) attr.Value {
	return TimeSelectionValue{}
}

var _ basetypes.ObjectValuable = TimeSelectionValue{}

type TimeSelectionValue struct {
	CustomSelection basetypes.ObjectValue `tfsdk:"custom_selection"`
	QuickSelection  basetypes.ObjectValue `tfsdk:"quick_selection"`
	state           attr.ValueState
}

func (v TimeSelectionValue) ToTerraformValue(ctx context.Context) (tftypes.Value, error) {
	attrTypes := make(map[string]tftypes.Type, 2)

	var val tftypes.Value
	var err error

	attrTypes["custom_selection"] = basetypes.ObjectType{
		AttrTypes: CustomSelectionValue{}.AttributeTypes(ctx),
	}.TerraformType(ctx)
	attrTypes["quick_selection"] = basetypes.ObjectType{
		AttrTypes: QuickSelectionValue{}.AttributeTypes(ctx),
	}.TerraformType(ctx)

	objectType := tftypes.Object{AttributeTypes: attrTypes}

	switch v.state {
	case attr.ValueStateKnown:
		vals := make(map[string]tftypes.Value, 2)

		val, err = v.CustomSelection.ToTerraformValue(ctx)

		if err != nil {
			return tftypes.NewValue(objectType, tftypes.UnknownValue), err
		}

		vals["custom_selection"] = val

		val, err = v.QuickSelection.ToTerraformValue(ctx)

		if err != nil {
			return tftypes.NewValue(objectType, tftypes.UnknownValue), err
		}

		vals["quick_selection"] = val

		if err := tftypes.ValidateValue(objectType, vals); err != nil {
			return tftypes.NewValue(objectType, tftypes.UnknownValue), err
		}

		return tftypes.NewValue(objectType, vals), nil
	case attr.ValueStateNull:
		return tftypes.NewValue(objectType, nil), nil
	case attr.ValueStateUnknown:
		return tftypes.NewValue(objectType, tftypes.UnknownValue), nil
	default:
		panic(fmt.Sprintf("unhandled Object state in ToTerraformValue: %s", v.state))
	}
}

func (v TimeSelectionValue) IsNull() bool {
	return v.state == attr.ValueStateNull
}

func (v TimeSelectionValue) IsUnknown() bool {
	return v.state == attr.ValueStateUnknown
}

func (v TimeSelectionValue) String() string {
	return "TimeSelectionValue"
}

func (v TimeSelectionValue) ToObjectValue(ctx context.Context) (basetypes.ObjectValue, diag.Diagnostics) {
	var diags diag.Diagnostics

	var customSelection basetypes.ObjectValue

	if v.CustomSelection.IsNull() {
		customSelection = types.ObjectNull(
			CustomSelectionValue{}.AttributeTypes(ctx),
		)
	}

	if v.CustomSelection.IsUnknown() {
		customSelection = types.ObjectUnknown(
			CustomSelectionValue{}.AttributeTypes(ctx),
		)
	}

	if !v.CustomSelection.IsNull() && !v.CustomSelection.IsUnknown() {
		customSelection = types.ObjectValueMust(
			CustomSelectionValue{}.AttributeTypes(ctx),
			v.CustomSelection.Attributes(),
		)
	}

	var quickSelection basetypes.ObjectValue

	if v.QuickSelection.IsNull() {
		quickSelection = types.ObjectNull(
			QuickSelectionValue{}.AttributeTypes(ctx),
		)
	}

	if v.QuickSelection.IsUnknown() {
		quickSelection = types.ObjectUnknown(
			QuickSelectionValue{}.AttributeTypes(ctx),
		)
	}

	if !v.QuickSelection.IsNull() && !v.QuickSelection.IsUnknown() {
		quickSelection = types.ObjectValueMust(
			QuickSelectionValue{}.AttributeTypes(ctx),
			v.QuickSelection.Attributes(),
		)
	}

	attributeTypes := map[string]attr.Type{
		"custom_selection": basetypes.ObjectType{
			AttrTypes: CustomSelectionValue{}.AttributeTypes(ctx),
		},
		"quick_selection": basetypes.ObjectType{
			AttrTypes: QuickSelectionValue{}.AttributeTypes(ctx),
		},
	}

	if v.IsNull() {
		return types.ObjectNull(attributeTypes), diags
	}

	if v.IsUnknown() {
		return types.ObjectUnknown(attributeTypes), diags
	}

	objVal, diags := types.ObjectValue(
		attributeTypes,
		map[string]attr.Value{
			"custom_selection": customSelection,
			"quick_selection":  quickSelection,
		})

	return objVal, diags
}

func (v TimeSelectionValue) Equal(o attr.Value) bool {
	other, ok := o.(TimeSelectionValue)

	if !ok {
		return false
	}

	if v.state != other.state {
		return false
	}

	if v.state != attr.ValueStateKnown {
		return true
	}

	if !v.CustomSelection.Equal(other.CustomSelection) {
		return false
	}

	if !v.QuickSelection.Equal(other.QuickSelection) {
		return false
	}

	return true
}

func (v TimeSelectionValue) Type(ctx context.Context) attr.Type {
	return TimeSelectionType{
		basetypes.ObjectType{
			AttrTypes: v.AttributeTypes(ctx),
		},
	}
}

func (v TimeSelectionValue) AttributeTypes(ctx context.Context) map[string]attr.Type {
	return map[string]attr.Type{
		"custom_selection": basetypes.ObjectType{
			AttrTypes: CustomSelectionValue{}.AttributeTypes(ctx),
		},
		"quick_selection": basetypes.ObjectType{
			AttrTypes: QuickSelectionValue{}.AttributeTypes(ctx),
		},
	}
}

var _ basetypes.ObjectTypable = CustomSelectionType{}

type CustomSelectionType struct {
	basetypes.ObjectType
}

func (t CustomSelectionType) Equal(o attr.Type) bool {
	other, ok := o.(CustomSelectionType)

	if !ok {
		return false
	}

	return t.ObjectType.Equal(other.ObjectType)
}

func (t CustomSelectionType) String() string {
	return "CustomSelectionType"
}

func (t CustomSelectionType) ValueFromObject(ctx context.Context, in basetypes.ObjectValue) (basetypes.ObjectValuable, diag.Diagnostics) {
	var diags diag.Diagnostics

	attributes := in.Attributes()

	fromTimeAttribute, ok := attributes["from_time"]

	if !ok {
		diags.AddError(
			"Attribute Missing",
			`from_time is missing from object`)

		return nil, diags
	}

	fromTimeVal, ok := fromTimeAttribute.(basetypes.StringValue)

	if !ok {
		diags.AddError(
			"Attribute Wrong Type",
			fmt.Sprintf(`from_time expected to be basetypes.StringValue, was: %T`, fromTimeAttribute))
	}

	toTimeAttribute, ok := attributes["to_time"]

	if !ok {
		diags.AddError(
			"Attribute Missing",
			`to_time is missing from object`)

		return nil, diags
	}

	toTimeVal, ok := toTimeAttribute.(basetypes.StringValue)

	if !ok {
		diags.AddError(
			"Attribute Wrong Type",
			fmt.Sprintf(`to_time expected to be basetypes.StringValue, was: %T`, toTimeAttribute))
	}

	if diags.HasError() {
		return nil, diags
	}

	return CustomSelectionValue{
		FromTime: fromTimeVal,
		ToTime:   toTimeVal,
		state:    attr.ValueStateKnown,
	}, diags
}

func NewCustomSelectionValueNull() CustomSelectionValue {
	return CustomSelectionValue{
		state: attr.ValueStateNull,
	}
}

func NewCustomSelectionValueUnknown() CustomSelectionValue {
	return CustomSelectionValue{
		state: attr.ValueStateUnknown,
	}
}

func NewCustomSelectionValue(attributeTypes map[string]attr.Type, attributes map[string]attr.Value) (CustomSelectionValue, diag.Diagnostics) {
	var diags diag.Diagnostics

	// Reference: https://github.com/hashicorp/terraform-plugin-framework/issues/521
	ctx := context.Background()

	for name, attributeType := range attributeTypes {
		attribute, ok := attributes[name]

		if !ok {
			diags.AddError(
				"Missing CustomSelectionValue Attribute Value",
				"While creating a CustomSelectionValue value, a missing attribute value was detected. "+
					"A CustomSelectionValue must contain values for all attributes, even if null or unknown. "+
					"This is always an issue with the provider and should be reported to the provider developers.\n\n"+
					fmt.Sprintf("CustomSelectionValue Attribute Name (%s) Expected Type: %s", name, attributeType.String()),
			)

			continue
		}

		if !attributeType.Equal(attribute.Type(ctx)) {
			diags.AddError(
				"Invalid CustomSelectionValue Attribute Type",
				"While creating a CustomSelectionValue value, an invalid attribute value was detected. "+
					"A CustomSelectionValue must use a matching attribute type for the value. "+
					"This is always an issue with the provider and should be reported to the provider developers.\n\n"+
					fmt.Sprintf("CustomSelectionValue Attribute Name (%s) Expected Type: %s\n", name, attributeType.String())+
					fmt.Sprintf("CustomSelectionValue Attribute Name (%s) Given Type: %s", name, attribute.Type(ctx)),
			)
		}
	}

	for name := range attributes {
		_, ok := attributeTypes[name]

		if !ok {
			diags.AddError(
				"Extra CustomSelectionValue Attribute Value",
				"While creating a CustomSelectionValue value, an extra attribute value was detected. "+
					"A CustomSelectionValue must not contain values beyond the expected attribute types. "+
					"This is always an issue with the provider and should be reported to the provider developers.\n\n"+
					fmt.Sprintf("Extra CustomSelectionValue Attribute Name: %s", name),
			)
		}
	}

	if diags.HasError() {
		return NewCustomSelectionValueUnknown(), diags
	}

	fromTimeAttribute, ok := attributes["from_time"]

	if !ok {
		diags.AddError(
			"Attribute Missing",
			`from_time is missing from object`)

		return NewCustomSelectionValueUnknown(), diags
	}

	fromTimeVal, ok := fromTimeAttribute.(basetypes.StringValue)

	if !ok {
		diags.AddError(
			"Attribute Wrong Type",
			fmt.Sprintf(`from_time expected to be basetypes.StringValue, was: %T`, fromTimeAttribute))
	}

	toTimeAttribute, ok := attributes["to_time"]

	if !ok {
		diags.AddError(
			"Attribute Missing",
			`to_time is missing from object`)

		return NewCustomSelectionValueUnknown(), diags
	}

	toTimeVal, ok := toTimeAttribute.(basetypes.StringValue)

	if !ok {
		diags.AddError(
			"Attribute Wrong Type",
			fmt.Sprintf(`to_time expected to be basetypes.StringValue, was: %T`, toTimeAttribute))
	}

	if diags.HasError() {
		return NewCustomSelectionValueUnknown(), diags
	}

	return CustomSelectionValue{
		FromTime: fromTimeVal,
		ToTime:   toTimeVal,
		state:    attr.ValueStateKnown,
	}, diags
}

func NewCustomSelectionValueMust(attributeTypes map[string]attr.Type, attributes map[string]attr.Value) CustomSelectionValue {
	object, diags := NewCustomSelectionValue(attributeTypes, attributes)

	if diags.HasError() {
		// This could potentially be added to the diag package.
		diagsStrings := make([]string, 0, len(diags))

		for _, diagnostic := range diags {
			diagsStrings = append(diagsStrings, fmt.Sprintf(
				"%s | %s | %s",
				diagnostic.Severity(),
				diagnostic.Summary(),
				diagnostic.Detail()))
		}

		panic("NewCustomSelectionValueMust received error(s): " + strings.Join(diagsStrings, "\n"))
	}

	return object
}

func (t CustomSelectionType) ValueFromTerraform(ctx context.Context, in tftypes.Value) (attr.Value, error) {
	if in.Type() == nil {
		return NewCustomSelectionValueNull(), nil
	}

	if !in.Type().Equal(t.TerraformType(ctx)) {
		return nil, fmt.Errorf("expected %s, got %s", t.TerraformType(ctx), in.Type())
	}

	if !in.IsKnown() {
		return NewCustomSelectionValueUnknown(), nil
	}

	if in.IsNull() {
		return NewCustomSelectionValueNull(), nil
	}

	attributes := map[string]attr.Value{}

	val := map[string]tftypes.Value{}

	err := in.As(&val)

	if err != nil {
		return nil, err
	}

	for k, v := range val {
		a, err := t.AttrTypes[k].ValueFromTerraform(ctx, v)

		if err != nil {
			return nil, err
		}

		attributes[k] = a
	}

	return NewCustomSelectionValueMust(CustomSelectionValue{}.AttributeTypes(ctx), attributes), nil
}

func (t CustomSelectionType) ValueType(ctx context.Context) attr.Value {
	return CustomSelectionValue{}
}

var _ basetypes.ObjectValuable = CustomSelectionValue{}

type CustomSelectionValue struct {
	FromTime basetypes.StringValue `tfsdk:"from_time"`
	ToTime   basetypes.StringValue `tfsdk:"to_time"`
	state    attr.ValueState
}

func (v CustomSelectionValue) ToTerraformValue(ctx context.Context) (tftypes.Value, error) {
	attrTypes := make(map[string]tftypes.Type, 2)

	var val tftypes.Value
	var err error

	attrTypes["from_time"] = basetypes.StringType{}.TerraformType(ctx)
	attrTypes["to_time"] = basetypes.StringType{}.TerraformType(ctx)

	objectType := tftypes.Object{AttributeTypes: attrTypes}

	switch v.state {
	case attr.ValueStateKnown:
		vals := make(map[string]tftypes.Value, 2)

		val, err = v.FromTime.ToTerraformValue(ctx)

		if err != nil {
			return tftypes.NewValue(objectType, tftypes.UnknownValue), err
		}

		vals["from_time"] = val

		val, err = v.ToTime.ToTerraformValue(ctx)

		if err != nil {
			return tftypes.NewValue(objectType, tftypes.UnknownValue), err
		}

		vals["to_time"] = val

		if err := tftypes.ValidateValue(objectType, vals); err != nil {
			return tftypes.NewValue(objectType, tftypes.UnknownValue), err
		}

		return tftypes.NewValue(objectType, vals), nil
	case attr.ValueStateNull:
		return tftypes.NewValue(objectType, nil), nil
	case attr.ValueStateUnknown:
		return tftypes.NewValue(objectType, tftypes.UnknownValue), nil
	default:
		panic(fmt.Sprintf("unhandled Object state in ToTerraformValue: %s", v.state))
	}
}

func (v CustomSelectionValue) IsNull() bool {
	return v.state == attr.ValueStateNull
}

func (v CustomSelectionValue) IsUnknown() bool {
	return v.state == attr.ValueStateUnknown
}

func (v CustomSelectionValue) String() string {
	return "CustomSelectionValue"
}

func (v CustomSelectionValue) ToObjectValue(ctx context.Context) (basetypes.ObjectValue, diag.Diagnostics) {
	var diags diag.Diagnostics

	attributeTypes := map[string]attr.Type{
		"from_time": basetypes.StringType{},
		"to_time":   basetypes.StringType{},
	}

	if v.IsNull() {
		return types.ObjectNull(attributeTypes), diags
	}

	if v.IsUnknown() {
		return types.ObjectUnknown(attributeTypes), diags
	}

	objVal, diags := types.ObjectValue(
		attributeTypes,
		map[string]attr.Value{
			"from_time": v.FromTime,
			"to_time":   v.ToTime,
		})

	return objVal, diags
}

func (v CustomSelectionValue) Equal(o attr.Value) bool {
	other, ok := o.(CustomSelectionValue)

	if !ok {
		return false
	}

	if v.state != other.state {
		return false
	}

	if v.state != attr.ValueStateKnown {
		return true
	}

	if !v.FromTime.Equal(other.FromTime) {
		return false
	}

	if !v.ToTime.Equal(other.ToTime) {
		return false
	}

	return true
}

func (v CustomSelectionValue) Type(ctx context.Context) attr.Type {
	return CustomSelectionType{
		basetypes.ObjectType{
			AttrTypes: v.AttributeTypes(ctx),
		},
	}
}

func (v CustomSelectionValue) AttributeTypes(ctx context.Context) map[string]attr.Type {
	return map[string]attr.Type{
		"from_time": basetypes.StringType{},
		"to_time":   basetypes.StringType{},
	}
}

var _ basetypes.ObjectTypable = QuickSelectionType{}

type QuickSelectionType struct {
	basetypes.ObjectType
}

func (t QuickSelectionType) Equal(o attr.Type) bool {
	other, ok := o.(QuickSelectionType)

	if !ok {
		return false
	}

	return t.ObjectType.Equal(other.ObjectType)
}

func (t QuickSelectionType) String() string {
	return "QuickSelectionType"
}

func (t QuickSelectionType) ValueFromObject(ctx context.Context, in basetypes.ObjectValue) (basetypes.ObjectValuable, diag.Diagnostics) {
	var diags diag.Diagnostics

	attributes := in.Attributes()

	captionAttribute, ok := attributes["caption"]

	if !ok {
		diags.AddError(
			"Attribute Missing",
			`caption is missing from object`)

		return nil, diags
	}

	captionVal, ok := captionAttribute.(basetypes.StringValue)

	if !ok {
		diags.AddError(
			"Attribute Wrong Type",
			fmt.Sprintf(`caption expected to be basetypes.StringValue, was: %T`, captionAttribute))
	}

	secondsAttribute, ok := attributes["seconds"]

	if !ok {
		diags.AddError(
			"Attribute Missing",
			`seconds is missing from object`)

		return nil, diags
	}

	secondsVal, ok := secondsAttribute.(basetypes.Int64Value)

	if !ok {
		diags.AddError(
			"Attribute Wrong Type",
			fmt.Sprintf(`seconds expected to be basetypes.Int64Value, was: %T`, secondsAttribute))
	}

	if diags.HasError() {
		return nil, diags
	}

	return QuickSelectionValue{
		Caption: captionVal,
		Seconds: secondsVal,
		state:   attr.ValueStateKnown,
	}, diags
}

func NewQuickSelectionValueNull() QuickSelectionValue {
	return QuickSelectionValue{
		state: attr.ValueStateNull,
	}
}

func NewQuickSelectionValueUnknown() QuickSelectionValue {
	return QuickSelectionValue{
		state: attr.ValueStateUnknown,
	}
}

func NewQuickSelectionValue(attributeTypes map[string]attr.Type, attributes map[string]attr.Value) (QuickSelectionValue, diag.Diagnostics) {
	var diags diag.Diagnostics

	// Reference: https://github.com/hashicorp/terraform-plugin-framework/issues/521
	ctx := context.Background()

	for name, attributeType := range attributeTypes {
		attribute, ok := attributes[name]

		if !ok {
			diags.AddError(
				"Missing QuickSelectionValue Attribute Value",
				"While creating a QuickSelectionValue value, a missing attribute value was detected. "+
					"A QuickSelectionValue must contain values for all attributes, even if null or unknown. "+
					"This is always an issue with the provider and should be reported to the provider developers.\n\n"+
					fmt.Sprintf("QuickSelectionValue Attribute Name (%s) Expected Type: %s", name, attributeType.String()),
			)

			continue
		}

		if !attributeType.Equal(attribute.Type(ctx)) {
			diags.AddError(
				"Invalid QuickSelectionValue Attribute Type",
				"While creating a QuickSelectionValue value, an invalid attribute value was detected. "+
					"A QuickSelectionValue must use a matching attribute type for the value. "+
					"This is always an issue with the provider and should be reported to the provider developers.\n\n"+
					fmt.Sprintf("QuickSelectionValue Attribute Name (%s) Expected Type: %s\n", name, attributeType.String())+
					fmt.Sprintf("QuickSelectionValue Attribute Name (%s) Given Type: %s", name, attribute.Type(ctx)),
			)
		}
	}

	for name := range attributes {
		_, ok := attributeTypes[name]

		if !ok {
			diags.AddError(
				"Extra QuickSelectionValue Attribute Value",
				"While creating a QuickSelectionValue value, an extra attribute value was detected. "+
					"A QuickSelectionValue must not contain values beyond the expected attribute types. "+
					"This is always an issue with the provider and should be reported to the provider developers.\n\n"+
					fmt.Sprintf("Extra QuickSelectionValue Attribute Name: %s", name),
			)
		}
	}

	if diags.HasError() {
		return NewQuickSelectionValueUnknown(), diags
	}

	captionAttribute, ok := attributes["caption"]

	if !ok {
		diags.AddError(
			"Attribute Missing",
			`caption is missing from object`)

		return NewQuickSelectionValueUnknown(), diags
	}

	captionVal, ok := captionAttribute.(basetypes.StringValue)

	if !ok {
		diags.AddError(
			"Attribute Wrong Type",
			fmt.Sprintf(`caption expected to be basetypes.StringValue, was: %T`, captionAttribute))
	}

	secondsAttribute, ok := attributes["seconds"]

	if !ok {
		diags.AddError(
			"Attribute Missing",
			`seconds is missing from object`)

		return NewQuickSelectionValueUnknown(), diags
	}

	secondsVal, ok := secondsAttribute.(basetypes.Int64Value)

	if !ok {
		diags.AddError(
			"Attribute Wrong Type",
			fmt.Sprintf(`seconds expected to be basetypes.Int64Value, was: %T`, secondsAttribute))
	}

	if diags.HasError() {
		return NewQuickSelectionValueUnknown(), diags
	}

	return QuickSelectionValue{
		Caption: captionVal,
		Seconds: secondsVal,
		state:   attr.ValueStateKnown,
	}, diags
}

func NewQuickSelectionValueMust(attributeTypes map[string]attr.Type, attributes map[string]attr.Value) QuickSelectionValue {
	object, diags := NewQuickSelectionValue(attributeTypes, attributes)

	if diags.HasError() {
		// This could potentially be added to the diag package.
		diagsStrings := make([]string, 0, len(diags))

		for _, diagnostic := range diags {
			diagsStrings = append(diagsStrings, fmt.Sprintf(
				"%s | %s | %s",
				diagnostic.Severity(),
				diagnostic.Summary(),
				diagnostic.Detail()))
		}

		panic("NewQuickSelectionValueMust received error(s): " + strings.Join(diagsStrings, "\n"))
	}

	return object
}

func (t QuickSelectionType) ValueFromTerraform(ctx context.Context, in tftypes.Value) (attr.Value, error) {
	if in.Type() == nil {
		return NewQuickSelectionValueNull(), nil
	}

	if !in.Type().Equal(t.TerraformType(ctx)) {
		return nil, fmt.Errorf("expected %s, got %s", t.TerraformType(ctx), in.Type())
	}

	if !in.IsKnown() {
		return NewQuickSelectionValueUnknown(), nil
	}

	if in.IsNull() {
		return NewQuickSelectionValueNull(), nil
	}

	attributes := map[string]attr.Value{}

	val := map[string]tftypes.Value{}

	err := in.As(&val)

	if err != nil {
		return nil, err
	}

	for k, v := range val {
		a, err := t.AttrTypes[k].ValueFromTerraform(ctx, v)

		if err != nil {
			return nil, err
		}

		attributes[k] = a
	}

	return NewQuickSelectionValueMust(QuickSelectionValue{}.AttributeTypes(ctx), attributes), nil
}

func (t QuickSelectionType) ValueType(ctx context.Context) attr.Value {
	return QuickSelectionValue{}
}

var _ basetypes.ObjectValuable = QuickSelectionValue{}

type QuickSelectionValue struct {
	Caption basetypes.StringValue `tfsdk:"caption"`
	Seconds basetypes.Int64Value  `tfsdk:"seconds"`
	state   attr.ValueState
}

func (v QuickSelectionValue) ToTerraformValue(ctx context.Context) (tftypes.Value, error) {
	attrTypes := make(map[string]tftypes.Type, 2)

	var val tftypes.Value
	var err error

	attrTypes["caption"] = basetypes.StringType{}.TerraformType(ctx)
	attrTypes["seconds"] = basetypes.Int64Type{}.TerraformType(ctx)

	objectType := tftypes.Object{AttributeTypes: attrTypes}

	switch v.state {
	case attr.ValueStateKnown:
		vals := make(map[string]tftypes.Value, 2)

		val, err = v.Caption.ToTerraformValue(ctx)

		if err != nil {
			return tftypes.NewValue(objectType, tftypes.UnknownValue), err
		}

		vals["caption"] = val

		val, err = v.Seconds.ToTerraformValue(ctx)

		if err != nil {
			return tftypes.NewValue(objectType, tftypes.UnknownValue), err
		}

		vals["seconds"] = val

		if err := tftypes.ValidateValue(objectType, vals); err != nil {
			return tftypes.NewValue(objectType, tftypes.UnknownValue), err
		}

		return tftypes.NewValue(objectType, vals), nil
	case attr.ValueStateNull:
		return tftypes.NewValue(objectType, nil), nil
	case attr.ValueStateUnknown:
		return tftypes.NewValue(objectType, tftypes.UnknownValue), nil
	default:
		panic(fmt.Sprintf("unhandled Object state in ToTerraformValue: %s", v.state))
	}
}

func (v QuickSelectionValue) IsNull() bool {
	return v.state == attr.ValueStateNull
}

func (v QuickSelectionValue) IsUnknown() bool {
	return v.state == attr.ValueStateUnknown
}

func (v QuickSelectionValue) String() string {
	return "QuickSelectionValue"
}

func (v QuickSelectionValue) ToObjectValue(ctx context.Context) (basetypes.ObjectValue, diag.Diagnostics) {
	var diags diag.Diagnostics

	attributeTypes := map[string]attr.Type{
		"caption": basetypes.StringType{},
		"seconds": basetypes.Int64Type{},
	}

	if v.IsNull() {
		return types.ObjectNull(attributeTypes), diags
	}

	if v.IsUnknown() {
		return types.ObjectUnknown(attributeTypes), diags
	}

	objVal, diags := types.ObjectValue(
		attributeTypes,
		map[string]attr.Value{
			"caption": v.Caption,
			"seconds": v.Seconds,
		})

	return objVal, diags
}

func (v QuickSelectionValue) Equal(o attr.Value) bool {
	other, ok := o.(QuickSelectionValue)

	if !ok {
		return false
	}

	if v.state != other.state {
		return false
	}

	if v.state != attr.ValueStateKnown {
		return true
	}

	if !v.Caption.Equal(other.Caption) {
		return false
	}

	if !v.Seconds.Equal(other.Seconds) {
		return false
	}

	return true
}

func (v QuickSelectionValue) Type(ctx context.Context) attr.Type {
	return QuickSelectionType{
		basetypes.ObjectType{
			AttrTypes: v.AttributeTypes(ctx),
		},
	}
}

func (v QuickSelectionValue) AttributeTypes(ctx context.Context) map[string]attr.Type {
	return map[string]attr.Type{
		"caption": basetypes.StringType{},
		"seconds": basetypes.Int64Type{},
	}
}
