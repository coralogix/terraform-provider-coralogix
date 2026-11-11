package generator

import (
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"

	"github.com/coralogix/coralogix-management-sdk/iac/issue"
	"github.com/coralogix/coralogix-management-sdk/iac/model"
	"github.com/coralogix/coralogix-management-sdk/iac/sdkcheck"
	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/overrides"
	"go.yaml.in/yaml/v4"
)

// placeVariant is the variant of the model struct of a component at a place: the parts of the API
// path of the holder, so that two places have two model structs.
func placeVariant(place string) string {
	var out strings.Builder
	out.WriteString("At")
	for _, part := range strings.Split(place, ".") {
		out.WriteString(camelize(part))
	}
	return out.String()
}

// placedType returns t with the variant of its place when a types.<Type>.at line names the holder:
// the component and field that hold t, written Component.field. A list, set, or map of such objects
// gets the variant on its items. The component then has its own model struct and its own names at
// that place.
func placedType(file *overrides.File, t *model.Type, holder string) *model.Type {
	if file == nil || t == nil {
		return t
	}
	switch t.Kind {
	case model.List, model.Set, model.Map:
		if elem := placedType(file, t.Elem, holder); elem != t.Elem {
			c := *t
			c.Elem = elem
			return &c
		}
	case model.Object, model.OneOf:
		if _, ok := file.Types[t.Schema].At[holder]; ok && t.Variant == "" {
			c := *t
			c.Variant = placeVariant(holder)
			return &c
		}
	}
	return t
}

// placeFieldName returns the name that a types.<Type>.at line gives the field of the object t at
// its place, and whether there is one.
func placeFieldName(file *overrides.File, t *model.Type, field string) (string, bool) {
	if file == nil || t == nil || t.Variant == "" {
		return "", false
	}
	for place, line := range file.Types[t.Schema].At {
		if placeVariant(place) != t.Variant {
			continue
		}
		if f, ok := line.Fields[field]; ok {
			return f.Name, true
		}
	}
	return "", false
}

// markPlace records that the builder found the place of a types.<Type>.at line.
func (b *tfBuilder) markPlace(place string) {
	if b.places == nil {
		b.places = map[string]bool{}
	}
	b.places[place] = true
}

// checkPlacesUsed reports a types.<Type>.at line whose holder the resource does not have, such as a
// misspelled component or field: the names of the line would be dropped without a word.
func (b *tfBuilder) checkPlacesUsed() error {
	if b.file == nil {
		return nil
	}
	for _, name := range slices.Sorted(maps.Keys(b.file.Types)) {
		for _, place := range slices.Sorted(maps.Keys(b.file.Types[name].At)) {
			if !b.places[place] {
				return fmt.Errorf("types.%s.at.%s: no field of that holder holds %s", name, place, name)
			}
		}
	}
	return nil
}

// checkPlaceFields reports a field of an at line that the object t does not have, for t at a place.
func checkPlaceFields(file *overrides.File, t *model.Type) error {
	if file == nil || t.Variant == "" {
		return nil
	}
	for place, line := range file.Types[t.Schema].At {
		if placeVariant(place) != t.Variant {
			continue
		}
		for field := range line.Fields {
			if !slices.ContainsFunc(t.Fields, func(f *model.Field) bool { return f.Name == field }) {
				return fmt.Errorf("types.%s.at.%s.fields.%s: %s has no field %s", t.Schema, place, field, t.Schema, field)
			}
		}
	}
	return nil
}

// joinOf returns the separator of the join line of the field, or "". A line on the list inside a
// wrapper that collapses is the line of the field that holds the wrapper, as for collection.
func joinOf(file *overrides.File, component, field string, t *model.Type) string {
	if file == nil {
		return ""
	}
	if sep := file.Types[component].Fields[field].Join; sep != "" {
		return sep
	}
	if t != nil && len(t.Wrappers) != 0 {
		inner := t.Wrappers[len(t.Wrappers)-1]
		return file.Types[inner.Schema].Fields[inner.Field].Join
	}
	return ""
}

// withPlaceClones returns the file with a copy of each component at a place whose at line inlines a
// field. An inline line inlines a field of a component in every place that holds it, because the
// generator changes the component of the contract. The copy gives the place a component of its own
// (PlaceClone), with the lines of the original: the at line replaces the line of each field that it
// names. The original keeps every other place. The file is unchanged when no at line inlines.
func withPlaceClones(file *overrides.File) (*overrides.File, error) {
	if file == nil {
		return nil, nil
	}
	var clones []*overrides.PlaceClone
	for _, component := range slices.Sorted(maps.Keys(file.Types)) {
		for _, holder := range slices.Sorted(maps.Keys(file.Types[component].At)) {
			if place := file.Types[component].At[holder]; place.NeedsCopy() {
				clones = append(clones, &overrides.PlaceClone{Component: component, Holder: holder, Name: component + placeVariant(holder), Fields: place.Fields})
			}
		}
	}
	if len(clones) == 0 {
		return file, nil
	}
	out := *file
	out.Types = maps.Clone(file.Types)
	out.Unwrap = slices.Clone(file.Unwrap)
	out.PlaceClones = clones
	for _, c := range clones {
		if _, ok := file.Types[c.Name]; ok {
			return nil, fmt.Errorf("types.%s.at.%s: the copy of %s at this place would have the name %s, which the file has a line for already", c.Component, c.Holder, c.Component, c.Name)
		}
		copied, err := placeCopy(file.Types[c.Component], c)
		if err != nil {
			return nil, err
		}
		out.Types[c.Name] = copied
		if slices.Contains(file.Unwrap, c.Component) {
			out.Unwrap = append(out.Unwrap, c.Name)
		}
		// The original no longer has the place: the copy holds it.
		original := out.Types[c.Component]
		original.At = maps.Clone(original.At)
		delete(original.At, c.Holder)
		out.Types[c.Component] = original
	}
	return &out, nil
}

// placeCopy returns the lines of the copy of a component at a place: the lines of the component,
// with the lines of the place laid over them, and an inline line for each inlined field. An inlined
// field has no attribute, so its line is the inline line only.
func placeCopy(original overrides.Type, c *overrides.PlaceClone) (overrides.Type, error) {
	copied := original
	copied.At = nil
	copied.Fields = maps.Clone(original.Fields)
	if copied.Fields == nil {
		copied.Fields = map[string]overrides.Field{}
	}
	for _, field := range slices.Sorted(maps.Keys(c.Fields)) {
		line := c.Fields[field]
		switch {
		case line.Inline && original.Fields[field].Inline:
			return copied, fmt.Errorf("types.%s.at.%s.fields.%s: the line of %s inlines the field in every place already", c.Component, c.Holder, field, c.Component)
		case line.Inline:
			// The other keys of an inline line are the keys of the field of the object with the name of the field.
			copied.Fields[field] = line
		default:
			copied.Fields[field] = layField(copied.Fields[field], line)
		}
	}
	return copied, nil
}

// layField returns the line base with every key that over sets laid over it. A key that over sets
// to its zero value is not a key: a place line cannot clear a key of the line of the component, but
// optional: true or another key can say the opposite.
func layField(base, over overrides.Field) overrides.Field {
	out := reflect.ValueOf(&base).Elem()
	from := reflect.ValueOf(over)
	for i := 0; i < from.NumField(); i++ {
		if !from.Field(i).IsZero() && out.Field(i).CanSet() {
			out.Field(i).Set(from.Field(i))
		}
	}
	// optional and required say the opposite: the key of the place wins.
	switch {
	case over.Optional && !over.Required:
		base.Required = false
	case over.Required && !over.Optional:
		base.Optional = false
	}
	return base
}

// placeClones gives each place that inlines a field a copy of its component in the contract. The
// field of the holder that holds the component refers to the copy, so the nested inline lines of
// the copy change that place only.
func (h inliner) placeClones() issue.Report {
	var report issue.Report
	for _, c := range h.file.PlaceClones {
		if err := h.placeClone(c); err != nil {
			report = append(report, issue.Issue{
				Code: "INLINE_UNSUPPORTED", Location: fmt.Sprintf("%s:types.%s.at.%s", overrides.FileName, c.Component, c.Holder),
				Message:     err.Error(),
				Remediation: "Name the component and the field of the contract that hold the component, and fields that the component has.",
			})
		}
	}
	return report
}

func (h inliner) placeClone(c *overrides.PlaceClone) error {
	at := strings.LastIndex(c.Holder, ".")
	holder, field := c.Holder[:at], c.Holder[at+1:]
	schema := mapValue(h.schemas, holder)
	if schema == nil {
		return fmt.Errorf("the contract has no component %s", holder)
	}
	prop := mapValue(mapValue(schema, "properties"), field)
	if prop == nil {
		return fmt.Errorf("%s has no field %s", holder, field)
	}
	ref := refOf(prop, c.Component)
	if ref == nil {
		return fmt.Errorf("the field %s of %s does not hold %s", field, holder, c.Component)
	}
	original := mapValue(h.schemas, c.Component)
	if original == nil {
		return fmt.Errorf("the contract has no component %s", c.Component)
	}
	for _, name := range slices.Sorted(maps.Keys(c.Fields)) {
		if mapValue(mapValue(original, "properties"), name) == nil {
			return fmt.Errorf("%s has no field %s", c.Component, name)
		}
	}
	if mapValue(h.schemas, c.Name) != nil {
		return fmt.Errorf("the contract has a component %s already", c.Name)
	}
	setMap(h.schemas, c.Name, cloneNode(original))
	mapValue(ref, "$ref").Value = "#/components/schemas/" + c.Name
	return nil
}

// refOf returns the node that refers to the component in the schema of a field: the field itself,
// the one entry of its allOf, or the same in the items or the values of a list or map.
func refOf(prop *yaml.Node, component string) *yaml.Node {
	if refName(prop) == component {
		return prop
	}
	if all := mapValue(prop, "allOf"); all != nil && len(all.Content) == 1 && refName(all.Content[0]) == component {
		return all.Content[0]
	}
	for _, key := range []string{"items", "additionalProperties"} {
		if inner := mapValue(prop, key); inner != nil {
			if ref := refOf(inner, component); ref != nil {
				return ref
			}
		}
	}
	return nil
}

// cloneNode returns a deep copy of a YAML node.
func cloneNode(n *yaml.Node) *yaml.Node {
	out := *n
	out.Content = make([]*yaml.Node, len(n.Content))
	for i, child := range n.Content {
		out.Content[i] = cloneNode(child)
	}
	return &out
}

// placeSDKNames gives the SDK names of the copy of a component at a place the names of the
// component: the SDK has one type for the component, and the copy only differs in the contract.
func placeSDKNames(refs []sdkcheck.Ref, file *overrides.File) []sdkcheck.Ref {
	if file == nil || len(file.PlaceClones) == 0 {
		return refs
	}
	names := map[string]string{}
	for _, c := range file.PlaceClones {
		names[model.GoName(c.Name)] = model.GoName(c.Component)
	}
	out := slices.Clone(refs)
	for i := range out {
		if out[i].Kind == sdkcheck.KindType {
			out[i].Name = renamedType(out[i].Name, names)
		}
		out[i].Owner = renamedType(out[i].Owner, names)
		out[i].Want, out[i].WantValue = renamedType(out[i].Want, names), renamedType(out[i].WantValue, names)
	}
	return out
}

// renamedType replaces the name of the type in the Go type s, which can be a pointer, a list, or a map.
func renamedType(s string, names map[string]string) string {
	prefix := ""
	for {
		switch {
		case strings.HasPrefix(s, "*"):
			prefix, s = prefix+"*", s[1:]
		case strings.HasPrefix(s, "[]"):
			prefix, s = prefix+"[]", s[2:]
		case strings.HasPrefix(s, "map[string]"):
			prefix, s = prefix+"map[string]", s[len("map[string]"):]
		default:
			if to, ok := names[s]; ok {
				s = to
			}
			return prefix + s
		}
	}
}
