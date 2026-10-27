// Package utils holds the schema copy of the provider utils package, for the generated data sources
// of the synthetic provider. The code is the code of the provider, so a generated data source
// compiles and behaves as in the provider.
package utils

import (
	"fmt"
	"strings"

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

// OverrideDatasourceDescriptions replaces the descriptions of a data source schema, which a data
// source copied from the resource. A key is the dotted Terraform path of an attribute, such as
// rules.allocation, and "" is the schema itself. The text is markdown and is also the plain
// description. A path that no attribute has is a programming error.
func OverrideDatasourceDescriptions(s *datasourceschema.Schema, descriptions map[string]string) {
	for at, text := range descriptions {
		if at == "" {
			s.Description, s.MarkdownDescription = text, text
			continue
		}
		if !overrideDatasourceDescription(s.Attributes, strings.Split(at, "."), text) {
			panic(fmt.Sprintf("the data source has no attribute %s", at))
		}
	}
}

func overrideDatasourceDescription(attributes map[string]datasourceschema.Attribute, path []string, text string) bool {
	current, ok := attributes[path[0]]
	if !ok {
		return false
	}
	if len(path) > 1 {
		switch attr := current.(type) {
		case datasourceschema.ListNestedAttribute:
			return overrideDatasourceDescription(attr.NestedObject.Attributes, path[1:], text)
		case datasourceschema.SetNestedAttribute:
			return overrideDatasourceDescription(attr.NestedObject.Attributes, path[1:], text)
		case datasourceschema.MapNestedAttribute:
			return overrideDatasourceDescription(attr.NestedObject.Attributes, path[1:], text)
		case datasourceschema.SingleNestedAttribute:
			return overrideDatasourceDescription(attr.Attributes, path[1:], text)
		}
		return false
	}
	switch attr := current.(type) {
	case datasourceschema.BoolAttribute:
		attr.Description, attr.MarkdownDescription = text, text
		current = attr
	case datasourceschema.Float32Attribute:
		attr.Description, attr.MarkdownDescription = text, text
		current = attr
	case datasourceschema.Float64Attribute:
		attr.Description, attr.MarkdownDescription = text, text
		current = attr
	case datasourceschema.Int32Attribute:
		attr.Description, attr.MarkdownDescription = text, text
		current = attr
	case datasourceschema.Int64Attribute:
		attr.Description, attr.MarkdownDescription = text, text
		current = attr
	case datasourceschema.NumberAttribute:
		attr.Description, attr.MarkdownDescription = text, text
		current = attr
	case datasourceschema.StringAttribute:
		attr.Description, attr.MarkdownDescription = text, text
		current = attr
	case datasourceschema.DynamicAttribute:
		attr.Description, attr.MarkdownDescription = text, text
		current = attr
	case datasourceschema.MapAttribute:
		attr.Description, attr.MarkdownDescription = text, text
		current = attr
	case datasourceschema.ObjectAttribute:
		attr.Description, attr.MarkdownDescription = text, text
		current = attr
	case datasourceschema.SetAttribute:
		attr.Description, attr.MarkdownDescription = text, text
		current = attr
	case datasourceschema.ListAttribute:
		attr.Description, attr.MarkdownDescription = text, text
		current = attr
	case datasourceschema.ListNestedAttribute:
		attr.Description, attr.MarkdownDescription = text, text
		current = attr
	case datasourceschema.SetNestedAttribute:
		attr.Description, attr.MarkdownDescription = text, text
		current = attr
	case datasourceschema.MapNestedAttribute:
		attr.Description, attr.MarkdownDescription = text, text
		current = attr
	case datasourceschema.SingleNestedAttribute:
		attr.Description, attr.MarkdownDescription = text, text
		current = attr
	default:
		panic(fmt.Sprintf("unknown data source attribute type: %T", current))
	}
	attributes[path[0]] = current
	return true
}
