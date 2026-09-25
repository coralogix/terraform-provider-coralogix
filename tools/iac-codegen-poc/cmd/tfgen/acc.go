package main

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v4"

	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen-poc/internal/model"
)

// accValues is the handwritten values file of the acceptance test (D15–D17).
// It uses the API names and value shapes, not the Terraform ones.
type accValues struct {
	Create map[string]any    `yaml:"create"`
	Update map[string]any    `yaml:"update"`
	Skip   map[string]string `yaml:"skip"`
}

// accData is the template data of the acceptance test.
type accData struct {
	TestName     string   // for example "TestAccAiEvaluation"
	Placeholders []string // the ${name} values that the environment hook fills
	Steps        []accStep
}

// accStep is one Terraform test step.
type accStep struct {
	Kind   string // "create", "import", "update", or "clear"
	Field  string // update, clear: the Terraform attribute that the step changes
	Config string // create, update, clear: the HCL body of the resource
	Checks []accCheck
}

// accCheck is a check of one top-level attribute after a step. It checks a
// scalar value, or that the attribute is not set. The empty-plan check after
// each step and ImportStateVerify cover the nested values.
type accCheck struct {
	Attr   string
	Value  string // the expected state value; "" with Absent
	Absent bool
}

var placeholder = regexp.MustCompile(`\$\{([a-zA-Z_][a-zA-Z0-9_]*)\}`)

// loadAccValues reads the values file. Unknown top-level keys are an error.
func loadAccValues(path string) (*accValues, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var v accValues
	dec := yaml.NewDecoder(strings.NewReader(string(data)))
	dec.KnownFields(true)
	if err := dec.Decode(&v); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &v, nil
}

// buildAcc checks the values against the model and plans the steps:
// create, import, one update step for each Update field, one clear step for
// each field that can be cleared, then Terraform destroys the resource.
func buildAcc(r *model.Resource, v *accValues) (*accData, error) {
	if err := checkAccKeys(r, v); err != nil {
		return nil, err
	}
	current := map[string]string{} // Terraform attribute → HCL value
	values := map[string]any{}     // Terraform attribute → API value, for checks
	for _, f := range r.Fields {
		val, ok := v.Create[f.Name]
		if !ok {
			continue
		}
		hcl, err := hclValue(f.Name, f.Type, val)
		if err != nil {
			return nil, fmt.Errorf("create.%w", err)
		}
		current[tfName(f.Name)], values[tfName(f.Name)] = hcl, val
	}
	data := &accData{TestName: "TestAcc" + camelize(r.Name), Placeholders: placeholders(v)}
	data.Steps = append(data.Steps,
		accStep{Kind: "create", Config: hclBody(r, current), Checks: scalarChecks(r, values)},
		accStep{Kind: "import"})

	for _, f := range r.Fields {
		val, ok := v.Update[f.Name]
		if !ok {
			continue
		}
		hcl, err := hclValue(f.Name, f.Type, val)
		if err != nil {
			return nil, fmt.Errorf("update.%w", err)
		}
		name := tfName(f.Name)
		current[name], values[name] = hcl, val
		data.Steps = append(data.Steps, accStep{Kind: "update", Field: name, Config: hclBody(r, current),
			Checks: scalarChecks(r, map[string]any{name: val})})
	}
	for _, f := range r.Fields {
		name := tfName(f.Name)
		if _, set := current[name]; !set || !clearable(f) || v.Skip[f.Name] != "" {
			continue
		}
		delete(current, name)
		data.Steps = append(data.Steps, accStep{Kind: "clear", Field: name, Config: hclBody(r, current),
			Checks: []accCheck{{Attr: name, Absent: true}}})
	}
	return data, nil
}

// checkAccKeys checks the keys of the values file against the model: create
// has only Create fields and every required one, update has only Update
// fields, and each Update field has an update value or a skip reason.
func checkAccKeys(r *model.Resource, v *accValues) error {
	byName := map[string]*model.ResourceField{}
	for _, f := range r.Fields {
		byName[f.Name] = f
	}
	isCreate := func(k string) bool { return byName[k] != nil && byName[k].Create != nil }
	isUpdate := func(k string) bool { return byName[k] != nil && byName[k].Update != nil }
	errs := slices.Concat(
		checkSection("create", sortedKeys(v.Create), isCreate, "not a Create field"),
		checkSection("update", sortedKeys(v.Update), isUpdate, "not an Update field"),
		checkSection("skip", sortedKeys(v.Skip), func(k string) bool { return isUpdate(k) && v.Skip[k] != "" },
			"not an Update field, or no reason"),
	)
	for _, f := range r.Fields {
		errs = append(errs, checkCovered(f, v)...)
	}
	if err := checkGroups("create", r.Groups, v.Create); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

// checkSection returns an error for each key that ok rejects.
func checkSection(section string, keys []string, ok func(string) bool, msg string) []error {
	var errs []error
	for _, k := range keys {
		if !ok(k) {
			errs = append(errs, fmt.Errorf("%s.%s: %s", section, k, msg))
		}
	}
	return errs
}

// checkCovered checks that a required Create field has a value, and that an
// Update field has exactly one of an update value and a skip reason.
func checkCovered(f *model.ResourceField, v *accValues) []error {
	_, inCreate := v.Create[f.Name]
	_, inUpdate := v.Update[f.Name]
	_, inSkip := v.Skip[f.Name]
	switch {
	case f.Create != nil && f.Create.Required && !inCreate:
		return []error{fmt.Errorf("create.%s: missing, the field is required", f.Name)}
	case f.Update != nil && inUpdate == inSkip:
		return []error{fmt.Errorf("%s: an Update field needs exactly one of update and skip", f.Name)}
	}
	return nil
}

// clearable reports whether the test can clear the field: it is an Update
// field with presence, optional everywhere, and with no default.
func clearable(f *model.ResourceField) bool {
	return f.Update != nil && f.Update.Presence && !f.Update.Required && f.Update.Default == nil &&
		(f.Create == nil || (!f.Create.Required && f.Create.Default == nil))
}

// hclValue converts an API value of the field path, of type t, to HCL.
func hclValue(path string, t *model.Type, v any) (string, error) {
	switch t.Kind {
	case model.Object, model.OneOf:
		return hclObject(path, t, v)
	case model.List, model.Set:
		return hclList(path, t, v)
	case model.Map:
		return hclMap(path, t, v)
	}
	return hclScalar(path, t, v)
}

// hclScalar converts a string, enum, bool, number, or integer.
func hclScalar(path string, t *model.Type, v any) (string, error) {
	fail := func(want string) (string, error) {
		return "", fmt.Errorf("%s: %v (%T) is not %s", path, v, v, want)
	}
	switch t.Kind {
	case model.String, model.Enum:
		s, ok := v.(string)
		if !ok {
			return fail("a string")
		}
		if t.Kind == model.Enum && !placeholder.MatchString(s) && !slices.Contains(t.Values, s) {
			return fail(fmt.Sprintf("one of %v", t.Values))
		}
		return strconv.Quote(s), nil
	case model.Bool:
		if b, ok := v.(bool); ok {
			return strconv.FormatBool(b), nil
		}
		return fail("a bool")
	case model.Number:
		switch n := v.(type) {
		case int:
			return strconv.Itoa(n), nil
		case float64:
			return strconv.FormatFloat(n, 'g', -1, 64), nil
		}
		return fail("a number")
	case model.Integer:
		return hclInteger(path, t, v)
	}
	return "", fmt.Errorf("%s: kind %s is not supported", path, t.Kind)
}

// hclInteger converts an integer. A uint64 is a decimal string in the API
// (D7); int32 and int64 are JSON numbers.
func hclInteger(path string, t *model.Type, v any) (string, error) {
	if t.WireString {
		s, ok := v.(string)
		if _, err := strconv.ParseUint(s, 10, 63); !ok || err != nil {
			return "", fmt.Errorf("%s: %v (%T) is not a decimal string up to the Int64 maximum", path, v, v)
		}
		return s, nil
	}
	n, ok := v.(int)
	bits := map[string]int{"int32": 32, "int64": 64}[t.Format]
	if !ok || bits == 0 || int64(n) != int64(n)<<(64-bits)>>(64-bits) {
		return "", fmt.Errorf("%s: %v (%T) is not an %s number", path, v, v, t.Format)
	}
	return strconv.Itoa(n), nil
}

func hclList(path string, t *model.Type, v any) (string, error) {
	items, ok := v.([]any)
	if !ok {
		return "", fmt.Errorf("%s: %v (%T) is not a list", path, v, v)
	}
	out := make([]string, 0, len(items))
	for i, item := range items {
		s, err := hclValue(fmt.Sprintf("%s[%d]", path, i), t.Elem, item)
		if err != nil {
			return "", err
		}
		out = append(out, s)
	}
	return "[" + strings.Join(out, ", ") + "]", nil
}

func hclMap(path string, t *model.Type, v any) (string, error) {
	m, ok := v.(map[string]any)
	if !ok {
		return "", fmt.Errorf("%s: %v (%T) is not a map", path, v, v)
	}
	out := make([]string, 0, len(m))
	for _, k := range sortedKeys(m) {
		s, err := hclValue(path+"."+k, t.Elem, m[k])
		if err != nil {
			return "", err
		}
		out = append(out, strconv.Quote(k)+" = "+s)
	}
	return "{ " + strings.Join(out, ", ") + " }", nil
}

// hclObject converts an API object or oneOf to HCL, with the fields in spec
// order. A oneOf must set exactly one arm.
func hclObject(path string, t *model.Type, v any) (string, error) {
	m, ok := v.(map[string]any)
	if !ok {
		return "", fmt.Errorf("%s: %v (%T) is not an object", path, v, v)
	}
	var out []string
	known := map[string]bool{}
	for _, f := range t.Fields {
		known[f.Name] = true
		val, ok := m[f.Name]
		if !ok {
			if f.Attrs.Required {
				return "", fmt.Errorf("%s.%s: missing, the field is required", path, f.Name)
			}
			continue
		}
		s, err := hclValue(path+"."+f.Name, f.Type, val)
		if err != nil {
			return "", err
		}
		out = append(out, tfName(f.Name)+" = "+s)
	}
	for _, k := range sortedKeys(m) {
		if !known[k] {
			return "", fmt.Errorf("%s.%s: no such field", path, k)
		}
	}
	if t.Kind == model.OneOf && len(out) != 1 {
		return "", fmt.Errorf("%s: a oneOf needs exactly one arm, it has %d", path, len(out))
	}
	if err := checkGroups(path, t.Groups, m); err != nil {
		return "", err
	}
	if len(out) == 0 {
		return "{}", nil
	}
	return "{ " + strings.Join(out, ", ") + " }", nil
}

// checkGroups checks that the values set at most one arm of each oneOf group,
// and one when the group allows no "no arm".
func checkGroups(path string, groups []model.OneOfGroup, values map[string]any) error {
	for _, g := range groups {
		var set []string
		for _, arm := range g.Arms {
			if _, ok := values[arm]; ok {
				set = append(set, arm)
			}
		}
		if len(set) > 1 || (len(set) == 0 && !g.AllowNone) {
			return fmt.Errorf("%s: oneOf %v needs %s arm, it has %v", path, g.Arms, map[bool]string{true: "at most one", false: "exactly one"}[g.AllowNone], set)
		}
	}
	return nil
}

// hclBody writes the attributes in spec order, one per line.
func hclBody(r *model.Resource, attrs map[string]string) string {
	var b strings.Builder
	for _, f := range r.Fields {
		if v, ok := attrs[tfName(f.Name)]; ok {
			fmt.Fprintf(&b, "  %s = %s\n", tfName(f.Name), v)
		}
	}
	return b.String()
}

// scalarChecks returns a value check for each top-level string, bool, or
// number in values.
func scalarChecks(r *model.Resource, values map[string]any) []accCheck {
	var out []accCheck
	for _, f := range r.Fields {
		v, ok := values[tfName(f.Name)]
		if !ok {
			continue
		}
		switch f.Type.Kind {
		case model.String, model.Enum, model.Bool, model.Number, model.Integer:
			s, err := hclValue(f.Name, f.Type, v)
			if err != nil {
				continue // hclValue already accepted v for the config
			}
			if uq, err := strconv.Unquote(s); err == nil {
				s = uq
			}
			out = append(out, accCheck{Attr: tfName(f.Name), Value: s})
		}
	}
	return out
}

// placeholders returns the sorted ${name} names in the values file.
func placeholders(v *accValues) []string {
	seen := map[string]bool{}
	var walk func(x any)
	walk = func(x any) {
		switch x := x.(type) {
		case string:
			for _, m := range placeholder.FindAllStringSubmatch(x, -1) {
				seen[m[1]] = true
			}
		case []any:
			for _, e := range x {
				walk(e)
			}
		case map[string]any:
			for _, e := range x {
				walk(e)
			}
		}
	}
	walk(v.Create)
	walk(v.Update)
	return sortedKeys(seen)
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
