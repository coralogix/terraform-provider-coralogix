// Package utils holds the schema copy of the provider utils package, for the generated data sources
// of the synthetic provider. The code is the code of the provider, so a generated data source
// compiles and behaves as in the provider.
package utils

import (
	"fmt"

	datasourceschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

func FrameworkDatasourceSchemaFromFrameworkResourceSchema(rs resourceschema.Schema) datasourceschema.Schema {
	attributes := ConvertAttributes(rs.Attributes)
	if idSchema, ok := rs.Attributes["id"]; ok {
		attributes["id"] = datasourceschema.StringAttribute{
			Required:            true,
			Description:         idSchema.GetDescription(),
			MarkdownDescription: idSchema.GetMarkdownDescription(),
		}
	}

	return datasourceschema.Schema{
		Attributes:          attributes,
		Description:         rs.Description,
		MarkdownDescription: rs.MarkdownDescription,
		DeprecationMessage:  rs.DeprecationMessage,
	}
}

func ConvertAttributes(attributes map[string]resourceschema.Attribute) map[string]datasourceschema.Attribute {
	result := make(map[string]datasourceschema.Attribute, len(attributes))
	for k, v := range attributes {
		result[k] = ConvertAttribute(v)
	}
	return result
}

func ConvertAttribute(resourceAttribute resourceschema.Attribute) datasourceschema.Attribute {
	if attr, ok := convertPrimitiveAttribute(resourceAttribute); ok {
		return attr
	}
	if attr, ok := convertCollectionAttribute(resourceAttribute); ok {
		return attr
	}
	if attr, ok := convertNestedAttribute(resourceAttribute); ok {
		return attr
	}
	panic(fmt.Sprintf("unknown resource attribute type: %T", resourceAttribute))
}

func convertPrimitiveAttribute(resourceAttribute resourceschema.Attribute) (datasourceschema.Attribute, bool) {
	switch attr := resourceAttribute.(type) {
	case resourceschema.BoolAttribute:
		return datasourceschema.BoolAttribute{
			Computed:            true,
			Description:         attr.Description,
			MarkdownDescription: attr.MarkdownDescription,
		}, true
	case resourceschema.Float32Attribute:
		return datasourceschema.Float32Attribute{
			Computed:            true,
			CustomType:          attr.CustomType,
			Description:         attr.Description,
			MarkdownDescription: attr.MarkdownDescription,
		}, true
	case resourceschema.Float64Attribute:
		return datasourceschema.Float64Attribute{
			Computed:            true,
			CustomType:          attr.CustomType,
			Description:         attr.Description,
			MarkdownDescription: attr.MarkdownDescription,
		}, true
	case resourceschema.Int64Attribute:
		return datasourceschema.Int64Attribute{
			Computed:            true,
			CustomType:          attr.CustomType,
			Description:         attr.Description,
			MarkdownDescription: attr.MarkdownDescription,
		}, true
	case resourceschema.Int32Attribute:
		return datasourceschema.Int32Attribute{
			Computed:            true,
			CustomType:          attr.CustomType,
			Description:         attr.Description,
			MarkdownDescription: attr.MarkdownDescription,
		}, true
	case resourceschema.NumberAttribute:
		return datasourceschema.NumberAttribute{
			Computed:            true,
			CustomType:          attr.CustomType,
			Description:         attr.Description,
			MarkdownDescription: attr.MarkdownDescription,
		}, true
	case resourceschema.StringAttribute:
		return datasourceschema.StringAttribute{
			Computed:            true,
			CustomType:          attr.CustomType,
			Description:         attr.Description,
			MarkdownDescription: attr.MarkdownDescription,
		}, true
	case resourceschema.DynamicAttribute:
		return datasourceschema.DynamicAttribute{
			Computed:            true,
			Description:         attr.Description,
			MarkdownDescription: attr.MarkdownDescription,
		}, true
	default:
		return nil, false
	}
}

func convertCollectionAttribute(resourceAttribute resourceschema.Attribute) (datasourceschema.Attribute, bool) {
	switch attr := resourceAttribute.(type) {
	case resourceschema.MapAttribute:
		return datasourceschema.MapAttribute{
			Computed:            true,
			Description:         attr.Description,
			MarkdownDescription: attr.MarkdownDescription,
			ElementType:         attr.ElementType,
		}, true
	case resourceschema.ObjectAttribute:
		return datasourceschema.ObjectAttribute{
			Computed:            true,
			Description:         attr.Description,
			MarkdownDescription: attr.MarkdownDescription,
			AttributeTypes:      attr.AttributeTypes,
		}, true
	case resourceschema.SetAttribute:
		return datasourceschema.SetAttribute{
			Computed:            true,
			Description:         attr.Description,
			MarkdownDescription: attr.MarkdownDescription,
			ElementType:         attr.ElementType,
		}, true
	case resourceschema.ListAttribute:
		return datasourceschema.ListAttribute{
			Computed:            true,
			Description:         attr.Description,
			MarkdownDescription: attr.MarkdownDescription,
			ElementType:         attr.ElementType,
		}, true
	default:
		return nil, false
	}
}

func convertNestedAttribute(resourceAttribute resourceschema.Attribute) (datasourceschema.Attribute, bool) {
	switch attr := resourceAttribute.(type) {
	case resourceschema.ListNestedAttribute:
		return datasourceschema.ListNestedAttribute{
			Computed:            true,
			Description:         attr.Description,
			MarkdownDescription: attr.MarkdownDescription,
			NestedObject: datasourceschema.NestedAttributeObject{
				Attributes: ConvertAttributes(attr.NestedObject.Attributes),
			},
		}, true
	case resourceschema.MapNestedAttribute:
		return datasourceschema.MapNestedAttribute{
			Computed:            true,
			Description:         attr.Description,
			MarkdownDescription: attr.MarkdownDescription,
			NestedObject: datasourceschema.NestedAttributeObject{
				Attributes: ConvertAttributes(attr.NestedObject.Attributes),
			},
		}, true
	case resourceschema.SetNestedAttribute:
		return datasourceschema.SetNestedAttribute{
			Computed:            true,
			Description:         attr.Description,
			MarkdownDescription: attr.MarkdownDescription,
			NestedObject: datasourceschema.NestedAttributeObject{
				Attributes: ConvertAttributes(attr.NestedObject.Attributes),
			},
		}, true
	case resourceschema.SingleNestedAttribute:
		return datasourceschema.SingleNestedAttribute{
			Computed:            true,
			Description:         attr.Description,
			MarkdownDescription: attr.MarkdownDescription,
			Attributes:          ConvertAttributes(attr.Attributes),
		}, true
	default:
		return nil, false
	}
}
