package generator

import (
	"maps"
	"slices"

	iacoverrides "github.com/coralogix/coralogix-management-sdk/iac/overrides"
	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/overrides"
	"github.com/pb33f/libopenapi/datamodel/high/base"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
)

// skipListArmFields skips the fields that the list arm of a typedArms.asList oneOf has next to its
// list. The oneOf is shown as that list of strings, so a field that the API adds to the list arm has
// no place in the schema: expand leaves it unset, and flatten drops it. A field that has a line in
// the file, such as a skip line, keeps its line.
func skipListArmFields(doc *v3.Document, file *overrides.File) {
	if file == nil || doc.Components == nil || doc.Components.Schemas == nil {
		return
	}
	for _, name := range slices.Sorted(maps.Keys(file.Types)) {
		if line := file.Types[name].TypedArms; line == nil || !line.AsList {
			continue
		}
		arm, others := listArmOthers(doc, file, name)
		for _, field := range others {
			if _, ok := file.Types[arm].Fields[field]; ok {
				continue
			}
			t := file.Types[arm]
			if t.Fields == nil {
				t.Fields = map[string]overrides.Field{}
			}
			t.Fields[field] = overrides.Field{Field: iacoverrides.Field{Skip: true}}
			file.Types[arm] = t
		}
	}
}

// listArmOthers returns the component of the list arm of the oneOf component, and the names of its
// properties other than the list of strings that the file does not skip. It returns nothing for a
// oneOf that has no one list arm.
func listArmOthers(doc *v3.Document, file *overrides.File, oneOf string) (string, []string) {
	proxy := doc.Components.Schemas.GetOrZero(oneOf)
	if proxy == nil {
		return "", nil
	}
	schema, err := proxy.BuildSchema()
	if err != nil || schema == nil || schema.Properties == nil {
		return "", nil
	}
	var armName string
	var others []string
	for _, prop := range schema.Properties.FromOldest() {
		component := propertyComponent(prop)
		arm := doc.Components.Schemas.GetOrZero(component)
		if arm == nil {
			continue
		}
		armSchema, err := arm.BuildSchema()
		if err != nil || armSchema == nil || armSchema.Properties == nil {
			continue
		}
		lists, rest := splitStringLists(armSchema)
		// A list that the file skips is no list of the oneOf.
		lists = slices.DeleteFunc(lists, func(n string) bool { return file.Types[component].Fields[n].Skip })
		if len(lists) != 1 {
			continue
		}
		if armName != "" {
			return "", nil // two list arms: the oneOf is no list and an empty arm
		}
		armName, others = component, rest
	}
	return armName, others
}

// propertyComponent returns the component that a property refers to, directly or as the only entry
// of its allOf.
func propertyComponent(prop *base.SchemaProxy) string {
	if name := refComponent(prop); name != "" {
		return name
	}
	schema, err := prop.BuildSchema()
	if err != nil || schema == nil || len(schema.AllOf) != 1 {
		return ""
	}
	return refComponent(schema.AllOf[0])
}

// splitStringLists returns the names of the properties of the schema that are lists of strings, and
// the names of the other properties.
func splitStringLists(schema *base.Schema) (lists, rest []string) {
	for name, prop := range schema.Properties.FromOldest() {
		if p, err := prop.BuildSchema(); err == nil && p != nil && slices.Contains(p.Type, "array") && stringItems(p) {
			lists = append(lists, name)
			continue
		}
		rest = append(rest, name)
	}
	return lists, rest
}

// stringItems reports whether the items of the list schema are strings.
func stringItems(list *base.Schema) bool {
	if list.Items == nil || !list.Items.IsA() || list.Items.A == nil {
		return false
	}
	item, err := list.Items.A.BuildSchema()
	return err == nil && item != nil && slices.Contains(item.Type, "string")
}
