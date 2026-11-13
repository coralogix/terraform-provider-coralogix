package generator

import (
	"strings"

	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/overrides"
	"go.yaml.in/yaml/v4"
)

// contractDeprecations returns the API fields that the contract marks deprecated: true, keyed
// "<component>.<property>", the way an attribute names its API field (tfAttr.Component and
// Property). The flag counts on the property itself: inline, next to a $ref, or on an allOf
// wrapper of a property. The flag of the component that a $ref names does not count: it is the
// deprecation of the component, not of this field. The function reads the text of the contract,
// because a parsed schema of a $ref has the target's flags, not those of the property.
func contractDeprecations(spec []byte) map[string]bool {
	var root yaml.Node
	if err := yaml.Unmarshal(spec, &root); err != nil || len(root.Content) == 0 {
		return nil
	}
	out := map[string]bool{}
	schemas := mappingValue(mappingValue(root.Content[0], "components"), "schemas")
	for i := 0; schemas != nil && i+1 < len(schemas.Content); i += 2 {
		props := mappingValue(schemas.Content[i+1], "properties")
		for j := 0; props != nil && j+1 < len(props.Content); j += 2 {
			if value := mappingValue(props.Content[j+1], "deprecated"); value != nil && value.Value == "true" {
				out[schemas.Content[i].Value+"."+props.Content[j].Value] = true
			}
		}
	}
	return out
}

// mappingValue returns the value of a key of a mapping node, or nil.
func mappingValue(node *yaml.Node, key string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}

// markDeprecations gives each attribute that the contract deprecates the warning text of the
// generator. An attribute that has a message already (a deprecation line) keeps it. A Required
// attribute gets no warning, because a user cannot leave it out, and an attribute that only the
// server sets gets none either, because a user never writes it: its description still says
// deprecated. A nested attribute and an arm of a oneOf count like any other.
func markDeprecations(attrs []*tfAttr, deprecated map[string]bool, file *overrides.File) {
	for _, a := range attrs {
		if a.DeprecationMessage == "" && !a.Extra && a.Optional && !a.Required && deprecated[contractField(a, file)] {
			a.DeprecationMessage = overrides.DeprecationText
		}
		markDeprecations(a.Attributes, deprecated, file)
	}
}

// contractField is the contract name of the API field of an attribute: a copy of a component for a
// place (an at line) names the component that it copies.
func contractField(a *tfAttr, file *overrides.File) string {
	component := a.Component
	if file != nil {
		if place := file.PlaceCloneOf(component); place != nil {
			component = place.Component
		}
	}
	component, _, _ = strings.Cut(component, "@")
	return component + "." + a.Property
}
