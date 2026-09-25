package main

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen-poc/internal/model"
)

// tfResource is the template data for the Terraform schema and model files.
type tfResource struct {
	Package          string
	Model            string // Go type of the resource model, for example "AiEvaluationModel"
	Attributes       []*tfAttr
	ConfigValidators []string // Go expressions of type resource.ConfigValidator
	Models           []*tfModel
	Conv             *convData // expand and flatten
	CRUD             *crudData // CRUD, import, and the provider data
	Acc              *accData  // the acceptance test; nil without a values file
}

// tfAttr is one Terraform schema attribute.
type tfAttr struct {
	Name        string // Terraform name, for example "join_limit"
	Kind        string // schema.<Kind>Attribute, for example "String", "SingleNested"
	ValueKind   string // validator.<ValueKind> and planmodifier.<ValueKind>, for example "String", "Object"
	Required    bool
	Optional    bool
	Computed    bool
	Description string
	ElementType string   // Set, List: the element type, for example "types.StringType"
	Default     string   // Go expression
	Validators  []string // Go expressions
	Modifiers   []string // plan modifiers, Go expressions
	Attributes  []*tfAttr
}

// tfModel is one Go struct of the Terraform model.
type tfModel struct {
	Name   string
	Fields []tfModelField
}

type tfModelField struct {
	Name   string // Go name
	Type   string // Go type
	TFName string
}

// buildTFResource maps the model to Terraform. Rules:
//   - Computed field → Computed. Only the id reuses the state value (D5).
//   - Immutable field → RequiresReplace (D9).
//   - Required in Create or in the object → Required. Otherwise Optional.
//   - "default" → Optional, Computed, and Default.
//   - oneOf → an object with one optional attribute per arm, and a resource
//     validator: ExactlyOneOf, or Conflicting when no arm is allowed.
//   - uint64 → Int64 (D7). Enum → String with the API values (D12).
func buildTFResource(r *model.Resource, pkg string) (*tfResource, error) {
	b := &tfBuilder{seen: map[string]bool{}}
	out := &tfResource{Package: pkg, Model: r.Name + "Model"}
	root := &tfModel{Name: out.Model}
	b.models = append(b.models, root)
	for _, f := range r.Fields {
		a, err := b.attribute(attrPath{"root", tfName(f.Name)}, f.Name, f.Description, f.Type, fieldAttrs(f))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", f.Name, err)
		}
		switch f.Behavior {
		case model.Computed:
			// Validators check the configuration, which is always null here.
			a.Required, a.Optional, a.Computed, a.Default, a.Validators = false, false, true, "", nil
			if f.Name == r.IDParam {
				a.Modifiers = append(a.Modifiers, strings.ToLower(a.ValueKind)+"planmodifier.UseStateForUnknown()")
			}
		case model.Immutable:
			a.Modifiers = append(a.Modifiers, strings.ToLower(a.ValueKind)+"planmodifier.RequiresReplace()")
		}
		out.Attributes = append(out.Attributes, a)
		root.Fields = append(root.Fields, b.modelField(f.Name, f.Type))
	}
	for _, g := range r.Groups {
		var arms []string
		for _, arm := range g.Arms {
			arms = append(arms, attrPath{"root", tfName(arm)}.expr())
		}
		b.validators = append(b.validators, groupValidator("resourcevalidator", g, true)+"(\n"+strings.Join(arms, ",\n")+",\n)")
	}
	out.ConfigValidators = b.validators
	out.Models = b.models
	return out, nil
}

// addGroupValidators adds a validator to each arm of the oneOf groups of an
// object: ExactlyOneOf, or ConflictsWith when no arm is allowed, with the
// other arms. On the arm, Terraform runs it only when the object is set. A
// resource validator would also run when the object is null, and then
// "exactly one" fails.
func addGroupValidators(attrs []*tfAttr, groups []model.OneOfGroup) {
	byName := map[string]*tfAttr{}
	for _, a := range attrs {
		byName[a.Name] = a
	}
	for _, g := range groups {
		for _, arm := range g.Arms {
			a := byName[tfName(arm)]
			var others []string
			for _, o := range g.Arms {
				if o != arm {
					others = append(others, fmt.Sprintf("path.MatchRelative().AtParent().AtName(%q)", tfName(o)))
				}
			}
			pkg := strings.ToLower(a.ValueKind) + "validator"
			a.Validators = append(a.Validators, groupValidator(pkg, g, false)+"("+strings.Join(others, ", ")+")")
		}
	}
}

// groupValidator is the validator function of a oneOf group: exactly one arm,
// or at most one when no arm is allowed. The resource validator is named
// Conflicting, the attribute validator ConflictsWith.
func groupValidator(pkg string, g model.OneOfGroup, resource bool) string {
	switch {
	case !g.AllowNone:
		return pkg + ".ExactlyOneOf"
	case resource:
		return pkg + ".Conflicting"
	}
	return pkg + ".ConflictsWith"
}

// fieldAttrs are the attributes that decide Required and Default. Create
// decides, because a Terraform resource is first created.
func fieldAttrs(f *model.ResourceField) model.Attrs {
	if f.Create != nil {
		return *f.Create
	}
	return model.Attrs{}
}

type tfBuilder struct {
	models     []*tfModel
	seen       map[string]bool // model structs already added
	validators []string        // resource config validators
}

// attrPath is a Terraform attribute path: "root", then the names. The step
// anyListItem is any element of a list, and anyMapValue any value of a map.
type attrPath []string

const (
	anyListItem = "[]"
	anyMapValue = "{}"
)

func (p attrPath) expr() string {
	s := fmt.Sprintf("path.MatchRoot(%q)", p[1])
	for _, n := range p[2:] {
		switch n {
		case anyListItem:
			s += ".AtAnyListIndex()"
			continue
		case anyMapValue:
			s += ".AtAnyMapKey()"
			continue
		}
		s += fmt.Sprintf(".AtName(%q)", n)
	}
	return s
}

func (b *tfBuilder) attribute(p attrPath, name, desc string, t *model.Type, attrs model.Attrs) (*tfAttr, error) {
	a := &tfAttr{Name: tfName(name), Description: desc, Required: attrs.Required, Optional: !attrs.Required}
	if err := b.setType(a, p, t); err != nil {
		return nil, err
	}
	if attrs.Default != nil {
		d, err := defaultExpr(a.Kind, *attrs.Default)
		if err != nil {
			return nil, err
		}
		a.Required, a.Optional, a.Computed, a.Default = false, true, true, d
	}
	return a, nil
}

func (b *tfBuilder) setType(a *tfAttr, p attrPath, t *model.Type) error {
	switch t.Kind {
	case model.String, model.Enum, model.Bool, model.Number, model.Integer:
		kind, vals, err := scalar(t)
		if err != nil {
			return err
		}
		a.Kind, a.ValueKind, a.Validators = kind, kind, vals
	case model.Set, model.List, model.Map:
		return b.collection(a, p, t)
	case model.Object, model.OneOf:
		a.Kind, a.ValueKind = "SingleNested", "Object"
		attrs, err := b.objectAttributes(p, t)
		if err != nil {
			return err
		}
		a.Attributes = attrs
	default:
		return fmt.Errorf("kind %s is not supported", t.Kind)
	}
	return nil
}

func (b *tfBuilder) collection(a *tfAttr, p attrPath, t *model.Type) error {
	kind, step := "Set", anyListItem
	switch t.Kind {
	case model.List:
		kind = "List"
	case model.Map:
		kind, step = "Map", anyMapValue
	}
	a.ValueKind = kind
	pkg := strings.ToLower(kind) + "validator"
	a.Validators = sizeValidator(pkg, t.MinItems, t.MaxItems)
	switch t.Elem.Kind {
	case model.Object:
		a.Kind = kind + "Nested"
		// buildConv rejects a set of objects, so a Set never gets here.
		attrs, err := b.objectAttributes(append(append(attrPath{}, p...), step), t.Elem)
		if err != nil {
			return err
		}
		a.Attributes = attrs
	case model.String, model.Enum, model.Bool, model.Number, model.Integer:
		elem, vals, err := scalar(t.Elem)
		if err != nil {
			return err
		}
		a.Kind, a.ElementType = kind, "types."+elem+"Type"
		if len(vals) > 0 {
			a.Validators = append(a.Validators, fmt.Sprintf("%s.Value%ssAre(%s)", pkg, elem, strings.Join(vals, ", ")))
		}
	default:
		return fmt.Errorf("%s of %s is not supported", t.Kind, t.Elem.Kind)
	}
	return nil
}

// objectAttributes returns the attributes of an object or of the arms of a
// oneOf, and adds its model struct.
func (b *tfBuilder) objectAttributes(p attrPath, t *model.Type) ([]*tfAttr, error) {
	if t.Schema == "" {
		return nil, fmt.Errorf("inline %s schema is not supported", t.Kind)
	}
	var attrs []*tfAttr
	var fields []tfModelField
	var arms []string
	for _, f := range t.Fields {
		child := append(append(attrPath{}, p...), tfName(f.Name))
		a, err := b.attribute(child, f.Name, f.Description, f.Type, f.Attrs)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", f.Name, err)
		}
		attrs = append(attrs, a)
		fields = append(fields, b.modelField(f.Name, f.Type))
		arms = append(arms, child.expr())
	}
	if t.Kind == model.OneOf {
		v := "resourcevalidator.ExactlyOneOf"
		if t.AllowNone {
			v = "resourcevalidator.Conflicting"
		}
		b.validators = append(b.validators, v+"(\n"+strings.Join(arms, ",\n")+",\n)")
	}
	addGroupValidators(attrs, t.Groups)
	if name := modelTypeName(t.Schema); !b.seen[name] {
		b.seen[name] = true
		b.models = append(b.models, &tfModel{Name: name, Fields: fields})
	}
	return attrs, nil
}

func (b *tfBuilder) modelField(name string, t *model.Type) tfModelField {
	var goType string
	switch t.Kind {
	case model.String, model.Enum:
		goType = "types.String"
	case model.Bool:
		goType = "types.Bool"
	case model.Number:
		goType = "types.Float64"
		if t.Format == "float" {
			goType = "types.Float32"
		}
	case model.Integer:
		goType = "types.Int64"
		if t.Format == "int32" {
			goType = "types.Int32"
		}
	case model.Set:
		goType = "types.Set"
	case model.List:
		goType = "types.List"
	case model.Map:
		goType = "types.Map"
	case model.Object, model.OneOf:
		goType = "*" + modelTypeName(t.Schema)
	}
	return tfModelField{Name: camelize(name), Type: goType, TFName: tfName(name)}
}

// modelTypeName is the Terraform model struct of a component schema.
func modelTypeName(schema string) string { return schema + "Model" }

// scalar returns the attribute kind and the validators of a scalar type.
func scalar(t *model.Type) (string, []string, error) {
	switch t.Kind {
	case model.String:
		var vals []string
		if v := lengthValidator(t.MinLength, t.MaxLength); v != "" {
			vals = append(vals, v)
		}
		return "String", vals, nil
	case model.Enum:
		quoted := make([]string, len(t.Values))
		for i, v := range t.Values {
			quoted[i] = strconv.Quote(v)
		}
		return "String", []string{"stringvalidator.OneOf(" + strings.Join(quoted, ", ") + ")"}, nil
	case model.Bool:
		return "Bool", nil, nil
	case model.Number:
		// float is a Float32 attribute. In a Float64 attribute, a float32
		// value would read back with other digits (0.1 → 0.10000000149).
		switch t.Format {
		case "double":
			return "Float64", rangeValidator("float64validator", t.Minimum, t.Maximum, formatFloat), nil
		case "float":
			return "Float32", rangeValidator("float32validator", t.Minimum, t.Maximum, formatFloat), nil
		}
		return "", nil, fmt.Errorf("number format %q is not supported", t.Format)
	case model.Integer:
		switch t.Format {
		case "uint64":
			// uint64 is sent as a string. Its length limit is on the string, so
			// it is not a range. Only the sign is known (F19).
			return "Int64", []string{"int64validator.AtLeast(0)"}, nil
		case "int64":
			return "Int64", rangeValidator("int64validator", t.Minimum, t.Maximum, formatInt), nil
		case "int32":
			return "Int32", rangeValidator("int32validator", t.Minimum, t.Maximum, formatInt), nil
		}
		return "", nil, fmt.Errorf("integer format %q is not supported", t.Format)
	}
	return "", nil, fmt.Errorf("kind %s is not a scalar", t.Kind)
}

func lengthValidator(minLen, maxLen *int64) string {
	switch {
	case minLen != nil && maxLen != nil:
		return fmt.Sprintf("stringvalidator.LengthBetween(%d, %d)", *minLen, *maxLen)
	case minLen != nil:
		return fmt.Sprintf("stringvalidator.LengthAtLeast(%d)", *minLen)
	case maxLen != nil:
		return fmt.Sprintf("stringvalidator.LengthAtMost(%d)", *maxLen)
	}
	return ""
}

// sizeValidator returns separate AtLeast and AtMost validators. A minimum of
// 0 is not a limit.
func sizeValidator(pkg string, minItems, maxItems *int64) []string {
	var vals []string
	if minItems != nil && *minItems > 0 {
		vals = append(vals, fmt.Sprintf("%s.SizeAtLeast(%d)", pkg, *minItems))
	}
	if maxItems != nil {
		vals = append(vals, fmt.Sprintf("%s.SizeAtMost(%d)", pkg, *maxItems))
	}
	return vals
}

func rangeValidator(pkg string, minimum, maximum *float64, format func(float64) string) []string {
	switch {
	case minimum != nil && maximum != nil:
		return []string{fmt.Sprintf("%s.Between(%s, %s)", pkg, format(*minimum), format(*maximum))}
	case minimum != nil:
		return []string{fmt.Sprintf("%s.AtLeast(%s)", pkg, format(*minimum))}
	case maximum != nil:
		return []string{fmt.Sprintf("%s.AtMost(%s)", pkg, format(*maximum))}
	}
	return nil
}

func formatFloat(f float64) string { return strconv.FormatFloat(f, 'g', -1, 64) }
func formatInt(f float64) string   { return strconv.FormatInt(int64(f), 10) }

// defaultExpr is the Go expression for a Terraform default. value is the
// YAML text of the OpenAPI default.
func defaultExpr(kind, value string) (string, error) {
	switch kind {
	case "Bool":
		v, err := strconv.ParseBool(value)
		if err != nil {
			return "", fmt.Errorf("default %q: %w", value, err)
		}
		return fmt.Sprintf("booldefault.StaticBool(%t)", v), nil
	case "String":
		return fmt.Sprintf("stringdefault.StaticString(%q)", value), nil
	case "Float64":
		v, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return "", fmt.Errorf("default %q: %w", value, err)
		}
		return fmt.Sprintf("float64default.StaticFloat64(%s)", formatFloat(v)), nil
	case "Float32":
		v, err := strconv.ParseFloat(value, 32)
		if err != nil {
			return "", fmt.Errorf("default %q: %w", value, err)
		}
		return fmt.Sprintf("float32default.StaticFloat32(%s)", formatFloat(v)), nil
	case "Int64", "Int32":
		v, err := strconv.ParseInt(value, 10, map[string]int{"Int64": 64, "Int32": 32}[kind])
		if err != nil {
			return "", fmt.Errorf("default %q: %w", value, err)
		}
		return fmt.Sprintf("%sdefault.Static%s(%d)", strings.ToLower(kind), kind, v), nil
	}
	return "", fmt.Errorf("default for %s is not supported", kind)
}

// tfName is the Terraform name for a property name: "sqlReadOnly" → "sql_read_only".
func tfName(property string) string {
	var b strings.Builder
	runes := []rune(property)
	for i, r := range runes {
		if unicode.IsUpper(r) && i > 0 {
			prevLower := !unicode.IsUpper(runes[i-1])
			nextLower := i+1 < len(runes) && unicode.IsLower(runes[i+1])
			if prevLower || nextLower {
				b.WriteByte('_')
			}
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}
