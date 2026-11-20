package generator

import (
	"fmt"
	"math/big"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/coralogix/coralogix-management-sdk/iac/issue"
	"github.com/coralogix/coralogix-management-sdk/iac/model"
	"go.yaml.in/yaml/v4"
)

// A schema property of the contract can carry a valid test value: `examples` (the proto option
// `example` of protoc-gen-openapiv3 renders as this array) or `example`. The model of the iac module
// does not keep it, so the generator reads the text of the contract, as it does for deprecations.
// The acceptance test of a new resource uses the value in place of a made-up one. An example is the
// way to state a rule that the schema cannot express: a name that must exist, a range that must be
// ordered.
//
//	examples: ["PLATFORM"]               -> field = "PLATFORM" in every config
//	examples: ["PLATFORM", "BROWSER"]    -> "PLATFORM" in the full config, "BROWSER" in the updated one
//	examples: [["a", "b"]]               -> a list property takes the elements "a" and "b"
//	items: {examples: ["a"]}             -> a list property takes the element "a"

// exampleValue is one scalar of an example: the text of the contract and its YAML tag.
type exampleValue struct{ Text, Tag string }

// exampleItem is one entry of `examples`. A scalar property reads one value. A list or set property
// reads Values as its elements: an array entry gives all of them, a scalar entry gives one.
type exampleItem struct {
	Values []exampleValue
	Array  bool
}

// contractExample is what the contract states for one property, in order.
type contractExample struct {
	Items []exampleItem
}

// base is the example that the full config uses.
func (e *contractExample) base() exampleItem { return e.Items[0] }

// updated is the example that the updated config uses: the second one, or the first when the
// contract has only one.
func (e *contractExample) updated() exampleItem {
	if len(e.Items) > 1 {
		return e.Items[1]
	}
	return e.Items[0]
}

// readExamples returns the examples of the properties of the component schemas, keyed
// "<component>.<property>" like contractDeprecations. A property that has no `examples` or `example`
// takes the ones of its `items`. An example that holds an object, or an array of objects, is left out:
// the generator makes no value from it.
func readExamples(spec []byte) map[string]*contractExample {
	var root yaml.Node
	if err := yaml.Unmarshal(spec, &root); err != nil || len(root.Content) == 0 {
		return nil
	}
	out := map[string]*contractExample{}
	schemas := mappingValue(mappingValue(root.Content[0], "components"), "schemas")
	for i := 0; schemas != nil && i+1 < len(schemas.Content); i += 2 {
		props := mappingValue(schemas.Content[i+1], "properties")
		for j := 0; props != nil && j+1 < len(props.Content); j += 2 {
			prop := props.Content[j+1]
			example := exampleOf(prop)
			if example == nil {
				example = exampleOf(mappingValue(prop, "items"))
			}
			if example != nil {
				out[schemas.Content[i].Value+"."+props.Content[j].Value] = example
			}
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// exampleOf reads the `examples` or the `example` of a schema node, or returns nil.
func exampleOf(schema *yaml.Node) *contractExample {
	nodes := []*yaml.Node(nil)
	if list := mappingValue(schema, "examples"); list != nil && list.Kind == yaml.SequenceNode {
		nodes = list.Content
	} else if one := mappingValue(schema, "example"); one != nil {
		nodes = []*yaml.Node{one}
	}
	var items []exampleItem
	for _, n := range nodes {
		item, ok := exampleItemOf(n)
		if !ok {
			return nil
		}
		items = append(items, item)
	}
	if len(items) == 0 {
		return nil
	}
	return &contractExample{Items: items}
}

func exampleItemOf(n *yaml.Node) (exampleItem, bool) {
	switch n.Kind {
	case yaml.ScalarNode:
		return exampleItem{Values: []exampleValue{{Text: n.Value, Tag: n.ShortTag()}}}, true
	case yaml.SequenceNode:
		item := exampleItem{Array: true}
		for _, c := range n.Content {
			if c.Kind != yaml.ScalarNode {
				return exampleItem{}, false
			}
			item.Values = append(item.Values, exampleValue{Text: c.Value, Tag: c.ShortTag()})
		}
		return item, true
	}
	return exampleItem{}, false
}

// exampleHit is an example that the contract states for a field of the resource.
type exampleHit struct {
	Key    string // "<component>.<property>" of the field in the resource response: the key of its attribute
	Source string // "<component>.<property>" that states the example: a request component can state it
	Type   *model.Type
	Value  *contractExample
}

// findExamples returns the examples of the fields of the resource, in the order of the walk. A field
// takes the example of its component in the resource response, or else of the one in the Create
// or Update request.
func findExamples(r *model.Resource, raw map[string]*contractExample) []exampleHit {
	if len(raw) == 0 {
		return nil
	}
	var hits []exampleHit
	seen := map[*model.Type]bool{}
	var walk func(t *model.Type)
	visit := func(components []string, name string, typ *model.Type) {
		if components[0] == "" {
			return
		}
		for _, c := range components {
			if value, ok := raw[c+"."+name]; ok && c != "" {
				hits = append(hits, exampleHit{Key: components[0] + "." + name, Source: c + "." + name, Type: typ, Value: value})
				return
			}
		}
	}
	walk = func(t *model.Type) {
		if t == nil || seen[t] {
			return
		}
		seen[t] = true
		walk(t.Elem)
		for _, f := range t.Fields {
			visit([]string{t.Schema, t.CreateSchema, t.UpdateSchema}, f.Name, f.Type)
			walk(f.Type)
		}
	}
	for _, f := range r.Fields {
		visit([]string{r.Name, r.Create.RequestComponent, r.Update.RequestComponent}, f.Name, f.Type)
		walk(f.Type)
	}
	return hits
}

// exampleMap keys the hits by the field of the resource response.
func exampleMap(hits []exampleHit) map[string]*contractExample {
	if len(hits) == 0 {
		return nil
	}
	out := map[string]*contractExample{}
	for _, h := range hits {
		out[h.Key] = h.Value
	}
	return out
}

// exampleIssues reports each example that the schema of its own property rules out: a value that
// does not match the pattern, lies outside the number or length range, is no value of the enum, or
// has the wrong JSON type. The test would send it, and the API would refuse it.
func exampleIssues(hits []exampleHit) issue.Report {
	var report issue.Report
	for _, h := range hits {
		component, property, _ := strings.Cut(h.Source, ".")
		if problem := exampleProblem(h.Type, h.Value); problem != "" {
			report = append(report, issue.Issue{
				Code:        "EXAMPLE_INVALID",
				Location:    fmt.Sprintf("components.schemas.%s.properties.%s", component, property),
				Message:     fmt.Sprintf("The example of %s does not pass the schema of the property: %s.", h.Source, problem),
				Remediation: "Change the example to a value that the schema of the property accepts, or delete the example.",
			})
		}
	}
	return report
}

// exampleProblem returns what is wrong with the example for the type, or "". An example of a
// property that is no scalar and no list of scalars has no meaning for the generator: it is not checked.
func exampleProblem(t *model.Type, e *contractExample) string {
	switch t.Kind {
	case model.List, model.Set:
		if t.Elem == nil || !exampleScalarKind(t.Elem.Kind) {
			return ""
		}
		for _, item := range e.Items {
			if n := int64(len(item.Values)); t.MinItems != nil && n < *t.MinItems || t.MaxItems != nil && n > *t.MaxItems {
				return fmt.Sprintf("it has %d elements, outside the item limits of the schema", n)
			}
			for _, v := range item.Values {
				if p := scalarProblem(t.Elem, v); p != "" {
					return p
				}
			}
		}
	case model.String, model.Enum, model.Bool, model.Number, model.Integer:
		for _, item := range e.Items {
			if item.Array {
				return fmt.Sprintf("the property is a %s, and the example is an array", t.Kind)
			}
			if p := scalarProblem(t, item.Values[0]); p != "" {
				return p
			}
		}
	}
	return ""
}

func exampleScalarKind(k model.Kind) bool {
	return slices.Contains([]model.Kind{model.String, model.Enum, model.Bool, model.Number, model.Integer}, k)
}

// scalarProblem checks one value against the schema of a scalar.
func scalarProblem(t *model.Type, v exampleValue) string {
	wrong := func(want string) string {
		return fmt.Sprintf("%s has the JSON type %s, and the property needs %s", strconv.Quote(v.Text), jsonType(v.Tag), want)
	}
	switch t.Kind {
	case model.String, model.Enum:
		if v.Tag != "!!str" && v.Tag != "!!timestamp" {
			return wrong("a string")
		}
		return stringProblem(t, v.Text)
	case model.Bool:
		if v.Tag != "!!bool" {
			return wrong("a boolean")
		}
	case model.Integer:
		if v.Tag != "!!int" && !(t.WireString && v.Tag == "!!str") {
			return wrong("an integer")
		}
		n, ok := new(big.Int).SetString(v.Text, 10)
		if !ok {
			return fmt.Sprintf("%s is no integer", strconv.Quote(v.Text))
		}
		return rangeProblem(t, new(big.Rat).SetInt(n), v.Text)
	case model.Number:
		if v.Tag != "!!int" && v.Tag != "!!float" {
			return wrong("a number")
		}
		r, ok := new(big.Rat).SetString(v.Text)
		if !ok {
			return fmt.Sprintf("%s is no number", strconv.Quote(v.Text))
		}
		return rangeProblem(t, r, v.Text)
	}
	return ""
}

func jsonType(tag string) string {
	switch tag {
	case "!!str", "!!timestamp":
		return "string"
	case "!!int", "!!float":
		return "number"
	case "!!bool":
		return "boolean"
	}
	return "other"
}

func stringProblem(t *model.Type, text string) string {
	if t.Kind == model.Enum {
		if !slices.Contains(t.Values, text) && text != t.EnumZero {
			return fmt.Sprintf("%s is no value of the enum (%s)", strconv.Quote(text), strings.Join(t.Values, ", "))
		}
		return ""
	}
	n := int64(utf8.RuneCountInString(text))
	if t.MinLength != nil && n < *t.MinLength {
		return fmt.Sprintf("%s is shorter than minLength %d", strconv.Quote(text), *t.MinLength)
	}
	if t.MaxLength != nil && n > *t.MaxLength {
		return fmt.Sprintf("%s is longer than maxLength %d", strconv.Quote(text), *t.MaxLength)
	}
	if t.Pattern != "" {
		if re, err := regexp.Compile(t.Pattern); err == nil && !re.MatchString(text) {
			return fmt.Sprintf("%s does not match the pattern %s", strconv.Quote(text), strconv.Quote(t.Pattern))
		}
	}
	return ""
}

// limitRat returns a schema limit as an exact decimal. The model holds the limit as a float64, so
// the shortest text of that float is read: the limit 0.1 gives 1/10, the same value as the example 0.1.
func limitRat(limit float64) *big.Rat {
	r, _ := new(big.Rat).SetString(strconv.FormatFloat(limit, 'g', -1, 64))
	return r
}

func rangeProblem(t *model.Type, v *big.Rat, text string) string {
	if t.Minimum != nil {
		if low := limitRat(*t.Minimum); low != nil && v.Cmp(low) < 0 {
			return fmt.Sprintf("%s is below the minimum %v", text, *t.Minimum)
		}
	}
	if t.Maximum != nil {
		if high := limitRat(*t.Maximum); high != nil && v.Cmp(high) > 0 {
			return fmt.Sprintf("%s is above the maximum %v", text, *t.Maximum)
		}
	}
	return ""
}

// markExamples gives each attribute that the contract states an example for the example. Only a
// scalar, or a list or set of scalars, takes one.
func markExamples(attrs []*tfAttr, examples map[string]*contractExample) {
	if len(examples) == 0 {
		return
	}
	for _, a := range attrs {
		if e, ok := examples[contractField(a, nil)]; ok && !a.Extra {
			a.Example = e
		}
		markExamples(a.Attributes, examples)
	}
}

// hasExample reports whether an attribute at or below the list has an example.
func hasExample(attrs []*tfAttr) bool {
	return slices.ContainsFunc(attrs, func(a *tfAttr) bool { return a.Example != nil || hasExample(a.Attributes) })
}

// exampleItemFor returns the example that the config of the mode uses, and false when the attribute
// takes none: it has no example, it is an enum (an example never replaces an enum value), it is a
// time, or the updated config of a resource that would not change otherwise keeps the made-up values.
func (s *accSynth) exampleItemFor(a *tfAttr, mode accMode) (exampleItem, bool) {
	if a.Example == nil || a.CustomType != "" || len(choices(a)) != 0 || len(enumValues(a)) != 0 || len(a.Example.Items) == 0 {
		return exampleItem{}, false
	}
	if mode == accUpdated {
		if s.noExamples {
			return exampleItem{}, false
		}
		return a.Example.updated(), true
	}
	return a.Example.base(), true
}

// exampleLiteral returns the HCL and the state text of an example value for a scalar kind.
func exampleLiteral(kind string, v exampleValue) (hcl, state string, ok bool) {
	switch kind {
	case "String":
		if v.Tag != "!!str" && v.Tag != "!!timestamp" {
			return "", "", false
		}
		// HCL starts a template at ${ and %{: doubling the $ or the % writes them as they are.
		text := strconv.Quote(v.Text)
		text = strings.ReplaceAll(strings.ReplaceAll(text, "${", "$${"), "%{", "%%{")
		return text, v.Text, true
	case "Bool":
		if v.Text != "true" && v.Text != "false" {
			return "", "", false
		}
		return v.Text, v.Text, true
	case "Int64", "Int32":
		n, ok := new(big.Int).SetString(v.Text, 10)
		if !ok {
			return "", "", false
		}
		return n.String(), n.String(), true
	case "Float64", "Float32", "Number":
		f, err := strconv.ParseFloat(v.Text, 64)
		if err != nil {
			return "", "", false
		}
		// Terraform shows a number in fixed-point notation, so the check text has no exponent.
		text := strconv.FormatFloat(f, 'f', -1, 64)
		return text, text, true
	}
	return "", "", false
}

// exampleScalar returns the HCL and the state text of the example of a scalar attribute.
func (s *accSynth) exampleScalar(a *tfAttr, mode accMode) (hcl, state string, ok bool) {
	item, ok := s.exampleItemFor(a, mode)
	if !ok || item.Array || len(item.Values) != 1 {
		return "", "", false
	}
	return exampleLiteral(a.Kind, item.Values[0])
}

// exampleElements returns the HCL and the state text of the elements of the example of a list or set.
// A set drops an element that repeats.
func (s *accSynth) exampleElements(a *tfAttr, kind string, mode accMode) (hcl, state []string, ok bool) {
	item, ok := s.exampleItemFor(a, mode)
	// An empty array is a valid example: it gives the empty collection.
	if !ok || a.ElementType == rfc3339Type || len(item.Values) == 0 && !item.Array {
		return nil, nil, false
	}
	for _, v := range item.Values {
		h, st, ok := exampleLiteral(kind, v)
		if !ok {
			return nil, nil, false
		}
		if a.Kind == "Set" && slices.Contains(state, st) {
			continue
		}
		hcl, state = append(hcl, h), append(state, st)
	}
	return hcl, state, true
}
