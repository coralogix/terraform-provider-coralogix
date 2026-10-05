package generator

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/model"
	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/overrides"
	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/version"
)

// tfResource is the template data for the Terraform schema and model files.
type tfResource struct {
	Package            string
	VersionHeader      string
	Model              string // Go type of the resource model, for example "WidgetModel"
	Attributes         []*tfAttr
	ConfigValidators   []string // Go expressions of type resource.ConfigValidator
	Models             []*tfModel
	Conv               *convData // expand and flatten
	CRUD               *crudData // CRUD, import, and the provider data
	HasServerDefaults  bool
	ServerDefaultKinds []string // Terraform scalar kinds that need reset planning
	PlanModifierPkgs   []string // lower-case Terraform value kinds with standard plan modifiers
	DefaultPkgs        []string // lower-case Terraform value kinds with static defaults
	// ComputedPaths are the paths of the computed attributes, with list and map
	// steps left out, as in Conv.OneOfArms.
	ComputedPaths []string
	// SchemaVersion and ResourceMarkdownDescription are the released schema version and the
	// released description of the resource. Both come from the behavior-overrides file.
	SchemaVersion               int64
	ResourceMarkdownDescription string
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
	ElementType string // Set, List: the element type, for example "types.StringType"
	// ElemValidators are the validators of one element of a collection of plain values, the
	// arguments of its Value<Kind>sAre validator. The acceptance test makes an element that passes.
	ElemValidators []string
	Validators     []string // Go expressions
	// GroupValidators are the oneOf group validators. They state the structure of the request, not a
	// limit, so they stay when the behavior-overrides file removes the inferred validators.
	GroupValidators []string
	// OneOfGroup names the oneOf group of an arm: its arms, joined. "" when the attribute is no arm.
	// The acceptance test sets one arm of each group.
	OneOfGroup string
	// OneOfRequired says that the group needs one arm (ExactlyOneOf), so every config sets one.
	OneOfRequired bool
	// EnumSchema is the OpenAPI component of the enum of a scalar attribute, or "".
	EnumSchema string
	Modifiers  []string // plan modifiers, Go expressions
	Attributes []*tfAttr
	// Component and Property name the API field of the attribute: the OpenAPI component
	// of its parent object, and the property. The behavior-overrides file uses them.
	Component, Property string
	// PlainDescription: the docs text is Description, not MarkdownDescription.
	PlainDescription   bool
	DeprecationMessage string
	Default            string // Go expression of a static default, or ""
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
//   - oneOf → an object with one optional attribute per arm. Each arm checks
//     the other arms only when its parent object is present.
//   - uint64 → Int64 (D7). Enum → String with the API values (D12).
func buildTFResource(r *model.Resource, pkg string) (*tfResource, error) {
	return buildTFResourceWith(r, pkg, nil)
}

// buildTFResourceWith is buildTFResource for a resource with a behavior-overrides file
// (nil for a new resource).
func buildTFResourceWith(r *model.Resource, pkg string, file *overrides.File) (*tfResource, error) {
	b := &tfBuilder{seen: map[string]bool{}}
	out := &tfResource{Package: pkg, VersionHeader: version.Header, Model: r.Name + "Model"}
	root := &tfModel{Name: out.Model}
	b.models = append(b.models, root)
	if r.Singleton {
		if err := addSingletonID(out, root, r); err != nil {
			return nil, err
		}
	}
	for _, f := range r.Fields {
		a, err := b.resourceAttribute(out, r, f)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", f.Name, err)
		}
		out.Attributes = append(out.Attributes, a)
		root.Fields = append(root.Fields, b.modelField(f.Name, f.Type, f.Behavior == model.Computed))
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
	if file != nil {
		if err := applyOverrides(out, file); err != nil {
			return nil, err
		}
	}
	out.PlanModifierPkgs = planModifierPackages(out.Attributes)
	out.DefaultPkgs = defaultPackages(out.Attributes)
	out.ComputedPaths = computedPaths(nil, "", out.Attributes)
	return out, nil
}

// computedPaths appends the paths of the computed attributes in attrs, in
// schema order. A nested attribute adds its name to the path. A list, set, or
// map step adds nothing.
func computedPaths(out []string, parent string, attrs []*tfAttr) []string {
	for _, a := range attrs {
		at := a.Name
		if parent != "" {
			at = parent + "." + a.Name
		}
		if a.Computed {
			out = append(out, at)
		}
		out = computedPaths(out, at, a.Attributes)
	}
	return out
}

// defaultPackages returns the default packages, such as stringdefault, that
// the static defaults of attrs use.
func defaultPackages(attrs []*tfAttr) []string {
	var packages []string
	for _, attr := range attrs {
		pkg := strings.ToLower(attr.ValueKind) + "default"
		if strings.HasPrefix(attr.Default, pkg+".") && !containsString(packages, pkg) {
			packages = append(packages, pkg)
		}
	}
	return packages
}

// addSingletonID adds the fixed id attribute of a singleton (D18).
func addSingletonID(out *tfResource, root *tfModel, r *model.Resource) error {
	for _, f := range r.Fields {
		if tfName(f.Name) == "id" {
			return fmt.Errorf("%s: a singleton with an id field is not supported: the id attribute is fixed", f.Name)
		}
	}
	// A static default puts the fixed id in the plan, so Create does not
	// show it as unknown. The user still cannot set it.
	out.Attributes = append(out.Attributes, &tfAttr{Name: "id", Kind: "String", ValueKind: "String", Computed: true,
		Description: "The fixed id of this singleton: there is one per company.",
		Default:     "stringdefault.StaticString(TypeName)"})
	root.Fields = append(root.Fields, tfModelField{Name: "Id", Type: "types.String", TFName: "id"})
	return nil
}

// resourceAttribute builds the attribute of a top-level field: its type, its server default, a
// client-set id, and the plan modifiers of its behavior.
func (b *tfBuilder) resourceAttribute(out *tfResource, r *model.Resource, f *model.ResourceField) (*tfAttr, error) {
	a, err := b.attribute(attrPath{"root", tfName(f.Name)}, r.Name, f.Name, f.Description, f.Type, fieldAttrs(f))
	if err != nil {
		return nil, err
	}
	if serverDefault(r, f) {
		// The user may override this value. When it is omitted, the server
		// supplies the declared default and Get always returns it. The plan
		// modifier makes removal unknown until Update clears the value and
		// Get returns that default.
		value, err := serverDefaultValue(f.Type, *f.Create.Default)
		if err != nil {
			return nil, fmt.Errorf("server default: %w", err)
		}
		a.Computed = true
		a.Modifiers = append(a.Modifiers, "serverDefaultModifier{value: "+value+"}")
		out.HasServerDefaults = true
		if !containsString(out.ServerDefaultKinds, a.ValueKind) {
			out.ServerDefaultKinds = append(out.ServerDefaultKinds, a.ValueKind)
		}
	}
	if r.Policy.ClientSetID && f.Name == r.IDParam {
		// The user can set the id. Without a value, the server makes one.
		a.Required, a.Optional, a.Computed = false, true, true
	}
	switch f.Behavior {
	case model.Computed:
		markComputed(a)
		if f.Name == r.IDParam {
			a.Modifiers = append(a.Modifiers, strings.ToLower(a.ValueKind)+"planmodifier.UseStateForUnknown()")
		}
	case model.Immutable:
		a.Modifiers = append(a.Modifiers, strings.ToLower(a.ValueKind)+"planmodifier.RequiresReplace()")
	}
	return a, nil
}

// applyOverrides applies the behavior-overrides file to the attributes. A field with a line
// keeps the released behavior that the line states.
func applyOverrides(out *tfResource, file *overrides.File) error {
	out.SchemaVersion = file.Schema.Version
	out.ResourceMarkdownDescription = file.MarkdownDescription
	var walk func(attrs []*tfAttr) error
	walk = func(attrs []*tfAttr) error {
		for _, a := range attrs {
			if file.Validators.Inferred != nil && !*file.Validators.Inferred {
				// No limit of the contract becomes a validator. A oneOf group validator is not a limit.
				a.Validators = slices.DeleteFunc(a.Validators, func(v string) bool { return !slices.Contains(a.GroupValidators, v) })
			}
			if line, ok := file.Types[a.Component].Fields[a.Property]; ok {
				if err := applyField(a, line, file); err != nil {
					return fmt.Errorf("%s.%s: %w", a.Component, a.Property, err)
				}
				if err := checkAttrMode(a); err != nil {
					return fmt.Errorf("%s.%s: %w", a.Component, a.Property, err)
				}
			}
			if err := walk(a.Attributes); err != nil {
				return err
			}
		}
		return nil
	}
	return walk(out.Attributes)
}

func applyField(a *tfAttr, l overrides.Field, file *overrides.File) error {
	if l.Required {
		a.Required, a.Optional = true, false
	}
	if l.Description != nil {
		a.Description, a.PlainDescription = *l.Description, true
	}
	if l.MarkdownDescription != nil {
		a.Description, a.PlainDescription = *l.MarkdownDescription, false
	}
	a.DeprecationMessage = l.Deprecation
	if l.Computed != nil {
		a.Computed = *l.Computed
	}
	if l.UseStateForUnknown {
		a.Modifiers = append(a.Modifiers, strings.ToLower(a.ValueKind)+"planmodifier.UseStateForUnknown()")
	}
	if l.Default != nil {
		expr, err := defaultExpr(a, l.Default)
		if err != nil {
			return err
		}
		a.Default = expr
	}
	for _, v := range l.Validators {
		expr, err := validatorExpr(a, v, file)
		if err != nil {
			return err
		}
		a.Validators = append(a.Validators, expr)
	}
	return nil
}

// checkAttrMode rejects an attribute that the Framework cannot build: one that is neither required,
// optional, nor computed, or one that is required and computed. A line can cause it, for example
// computed: false on a field that the contract makes server-owned.
func checkAttrMode(a *tfAttr) error {
	switch {
	case a.Required && a.Computed:
		return errors.New("the line makes the attribute required and computed: required: true needs a field that the user sets")
	case !a.Required && !a.Optional && !a.Computed:
		return errors.New("the line leaves the attribute without a mode: computed: false needs a field that the user can set, and the contract makes this field server-owned")
	}
	return nil
}

// defaultExpr is the Go expression of a static default of the attribute.
func defaultExpr(a *tfAttr, value any) (string, error) {
	switch v := value.(type) {
	case string:
		if a.Kind == "String" {
			return fmt.Sprintf("stringdefault.StaticString(%q)", v), nil
		}
	case bool:
		if a.Kind == "Bool" {
			return fmt.Sprintf("booldefault.StaticBool(%t)", v), nil
		}
	}
	return "", fmt.Errorf("a default of type %T does not fit a %s attribute", value, a.Kind)
}

// validatorExpr is the Go expression of a released validator.
func validatorExpr(a *tfAttr, v overrides.Validator, file *overrides.File) (string, error) {
	pkg := strings.ToLower(a.ValueKind) + "validator"
	switch {
	case v.Enum:
		return enumValidatorExpr(a, file)
	case len(v.OneOf) != 0 && a.ValueKind == "String":
		quoted := make([]string, 0, len(v.OneOf))
		for _, one := range v.OneOf {
			quoted = append(quoted, strconv.Quote(one))
		}
		return "stringvalidator.OneOf(" + strings.Join(quoted, ", ") + ")", nil
	case v.SizeAtLeast != nil && (a.ValueKind == "List" || a.ValueKind == "Set" || a.ValueKind == "Map"):
		return fmt.Sprintf("%s.SizeAtLeast(%d)", pkg, *v.SizeAtLeast), nil
	}
	return "", fmt.Errorf("the validator does not fit a %s attribute", a.ValueKind)
}

// enumValidatorExpr is the validator that accepts the Terraform values of the enum of the attribute.
// The values are the zero value and the lower case of each accepted value, in sorted order. They
// come from the enums line, so the validator and the conversion maps cannot disagree.
func enumValidatorExpr(a *tfAttr, file *overrides.File) (string, error) {
	enum, ok := file.Enums[a.EnumSchema]
	if a.EnumSchema == "" || a.ValueKind != "String" || !ok {
		return "", errors.New("the enum validator needs a field of an enum type that has a line under enums")
	}
	var values []string
	if enum.Zero != "" {
		values = append(values, enum.Zero)
	}
	for _, v := range enum.Values {
		values = append(values, strings.ToLower(v))
	}
	slices.Sort(values)
	quoted := make([]string, 0, len(values))
	for _, v := range values {
		quoted = append(quoted, strconv.Quote(v))
	}
	return "stringvalidator.OneOf(" + strings.Join(quoted, ", ") + ")", nil
}

// markComputed makes a server-owned attribute and every nested attribute
// state-only. Validators inspect configuration, which is always null here.
func markComputed(a *tfAttr) {
	a.Required, a.Optional, a.Computed, a.Validators = false, false, true, nil
	for _, child := range a.Attributes {
		markComputed(child)
	}
}

func planModifierPackages(attrs []*tfAttr) []string {
	var packages []string
	for _, attr := range attrs {
		pkg := strings.ToLower(attr.ValueKind) + "planmodifier"
		for _, modifier := range attr.Modifiers {
			if strings.Contains(modifier, pkg+".") && !containsString(packages, pkg) {
				packages = append(packages, pkg)
			}
		}
		for _, nested := range planModifierPackages(attr.Attributes) {
			if !containsString(packages, nested) {
				packages = append(packages, nested)
			}
		}
	}
	return packages
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func hasServerDefault(f *model.ResourceField) bool {
	return f.Create != nil && !f.Create.Required && f.Create.Default != nil && f.Get != nil && f.Get.Required
}

// serverDefaultValue returns the typed Terraform value used only to detect
// whether state already equals the declared server default. It is not a
// Terraform static default and is never inserted into an API request.
func serverDefaultValue(t *model.Type, value string) (string, error) {
	switch t.Kind {
	case model.String:
		return fmt.Sprintf("types.StringValue(%q)", value), nil
	case model.Enum:
		for _, candidate := range t.Values {
			if candidate == value {
				return fmt.Sprintf("types.StringValue(%q)", value), nil
			}
		}
		return "", fmt.Errorf("%q is not one of %v", value, t.Values)
	case model.Bool:
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return "", fmt.Errorf("%q is not a boolean", value)
		}
		return fmt.Sprintf("types.BoolValue(%t)", parsed), nil
	case model.Number:
		bits := 64
		kind := "Float64"
		if t.Format == "float" {
			bits, kind = 32, "Float32"
		}
		parsed, err := strconv.ParseFloat(value, bits)
		if err != nil {
			return "", fmt.Errorf("%q is not a %s", value, t.Format)
		}
		return fmt.Sprintf("types.%sValue(%s)", kind, strconv.FormatFloat(parsed, 'g', -1, bits)), nil
	case model.Integer:
		bits := 64
		kind := "Int64"
		if t.Format == "int32" {
			bits, kind = 32, "Int32"
		}
		parsed, err := strconv.ParseInt(value, 10, bits)
		if err != nil {
			return "", fmt.Errorf("%q is not a supported %s", value, t.Format)
		}
		return fmt.Sprintf("types.%sValue(%d)", kind, parsed), nil
	default:
		return "", fmt.Errorf("%s defaults are not supported", t.Kind)
	}
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
			validator := groupValidator(pkg, g, false) + "(" + strings.Join(others, ", ") + ")"
			a.Validators = append(a.Validators, validator)
			a.GroupValidators = append(a.GroupValidators, validator)
			a.OneOfGroup = strings.Join(g.Arms, ",")
			a.OneOfRequired = !g.AllowNone
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

// fieldAttrs are the attributes that decide Required. Create decides, because
// a Terraform resource is first created.
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

func (b *tfBuilder) attribute(p attrPath, component, name, desc string, t *model.Type, attrs model.Attrs) (*tfAttr, error) {
	a := &tfAttr{Name: tfName(name), Description: desc, Required: attrs.Required, Optional: !attrs.Required, Component: component, Property: name}
	if err := b.setType(a, p, t); err != nil {
		return nil, err
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
		if t.Kind == model.Enum {
			a.EnumSchema = t.Schema
		}
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
		a.Kind, a.ElementType, a.ElemValidators = kind, "types."+elem+"Type", vals
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
	for _, f := range t.Fields {
		child := append(append(attrPath{}, p...), tfName(f.Name))
		a, err := b.attribute(child, t.Schema, f.Name, f.Description, f.Type, f.Attrs)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", f.Name, err)
		}
		attrs = append(attrs, a)
		fields = append(fields, b.modelField(f.Name, f.Type, false))
	}
	groups := t.Groups
	if t.Kind == model.OneOf {
		group := model.OneOfGroup{AllowNone: t.AllowNone}
		for _, f := range t.Fields {
			group.Arms = append(group.Arms, f.Name)
		}
		groups = append(groups, group)
	}
	addGroupValidators(attrs, groups)
	if name := modelTypeName(t.Schema); !b.seen[name] {
		b.seen[name] = true
		b.models = append(b.models, &tfModel{Name: name, Fields: fields})
	}
	return attrs, nil
}

func (b *tfBuilder) modelField(name string, t *model.Type, computed bool) tfModelField {
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
		if computed {
			goType = "types.Object"
		} else {
			goType = "*" + modelTypeName(t.Schema)
		}
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

// tfName is the Terraform name for a property name: "sqlReadOnly" → "sql_read_only".
func tfName(property string) string {
	return model.TerraformName(property)
}
