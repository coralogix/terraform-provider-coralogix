package generator

import (
	"errors"
	"fmt"
	"maps"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/coralogix/coralogix-management-sdk/iac/model"
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
	// RequestReplaceKinds are the Terraform value kinds of the immutable
	// attributes that hold values that the server sets. They use
	// requestReplaceModifier instead of RequiresReplace.
	RequestReplaceKinds []string
	PlanModifierPkgs    []string // lower-case Terraform value kinds with standard plan modifiers
	DefaultPkgs         []string // lower-case Terraform value kinds with static defaults
	// ComputedPaths are the paths of the computed attributes, with list and map
	// steps left out, as in Conv.OneOfArms.
	ComputedPaths []string
	// RequiredPaths are the paths of the required attributes, in the form of ComputedPaths.
	RequiredPaths []string
	// UniqueChecks, Float32Paths, and EnumAliases are the generic checks of a released
	// resource: uniqueBy lines, the float32 precision of the float attributes, and the
	// aliases of the enums lines. See applyConfigChecks.
	UniqueChecks []uniqueCheck
	Float32Paths []string
	EnumAliases  []enumAlias
	// ServerPaths are the paths of the computed-only attributes inside the
	// attributes that a request sends, in the form of ComputedPaths. The
	// server sets them, and Terraform plans them as unknown when the resource
	// changes, so the request checks leave them out.
	ServerPaths []string
	// DefaultPaths are the paths of the nested attributes with a server default, in the form
	// of ComputedPaths. A request does not send an unknown value of one: the server stores
	// the default.
	DefaultPaths []defaultPath
	// OmittedDefaults are the top-level attributes with a server default that Get does not
	// require, with that default. A read that omits one keeps a prior value equal to the
	// default: the API can leave out a value that equals its default.
	OmittedDefaults []defaultPath
	// SchemaVersion and ResourceMarkdownDescription are the released schema version and the
	// released description of the resource. Both come from the behavior-overrides file.
	SchemaVersion               int64
	ResourceMarkdownDescription string
	// DataSource is the data source of the package, or nil when the package has none.
	DataSource *dsData
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
	// CustomType is the Terraform type of a scalar attribute that is not the plain type of its
	// kind, for example rfc3339Type, or "".
	CustomType string
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
	// RootArm: the attribute is an arm of a top-level oneOf, which a resource validator checks.
	RootArm bool
	// KeyedObject: the attribute is the object of a keyedBy line, with an attribute per value of
	// the key. Its attributes take the state-keeping plan modifier of its line.
	KeyedObject bool
	// EnumSchema is the OpenAPI component of the enum of a scalar attribute, or of the elements
	// of a list or set, or "".
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
	// WriteOnly: Terraform sends the value and does not store it. Only extraAttributes set this.
	WriteOnly bool
	// TrueOnly: a bool that shows an object without fields (asBool). Only true is a valid value.
	TrueOnly bool
	// Extra: the attribute is from types.<Type>.extraAttributes, not the API. The generated
	// acceptance test leaves it out unless values or an upgrade case supplies the map: a made-up
	// map key is not a valid API field.
	Extra bool
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
	b := &tfBuilder{seen: map[string]bool{}, sent: sentFields(r), file: file, policy: r.Policy, replace: r.Replace, owners: map[string]string{}}
	out := &tfResource{Package: pkg, VersionHeader: version.Header, Model: modelTypeName(r.Name)}
	root := &tfModel{Name: out.Model}
	if err := b.claimModel(root.Name, "the resource "+r.Name); err != nil {
		return nil, err
	}
	b.models = append(b.models, root)
	if r.Singleton {
		if err := addSingletonID(out, root, r, file.IDFromAPI(), "SingletonID"); err != nil {
			return nil, err
		}
	} else if file != nil && file.SingletonID != nil {
		return nil, errors.New("singletonID is for a singleton: Get of this resource has a path parameter")
	}
	groups := &tfGroups{file: file, resource: r.Name, attrs: map[string]*tfAttr{}, models: map[string]*tfModel{}}
	if err := b.rootAttributes(out, root, r, file, groups); err != nil {
		return nil, err
	}
	if err := b.rootOneOfs(out, r, file, groups); err != nil {
		return nil, err
	}
	if err := groups.check(out.Attributes); err != nil {
		return nil, err
	}
	if err := b.checkNestedGroupsUsed(); err != nil {
		return nil, err
	}
	out.ConfigValidators = b.validators
	out.Models = b.models
	out.RequestReplaceKinds = b.replaceKinds
	out.DefaultPaths = b.defaultPaths
	for _, kind := range b.defaultKinds {
		out.HasServerDefaults = true
		if !containsString(out.ServerDefaultKinds, kind) {
			out.ServerDefaultKinds = append(out.ServerDefaultKinds, kind)
		}
	}
	if file != nil {
		if err := applyOverrides(out, file); err != nil {
			return nil, err
		}
		if err := checkImmutableValues(out.Attributes, ""); err != nil {
			return nil, err
		}
		if err := checkOmittedMaskClears(r, out.Attributes, file); err != nil {
			return nil, err
		}
		if file.IDFromAPI() {
			if err := checkSingletonIDAttribute(out.Attributes); err != nil {
				return nil, err
			}
		}
	}
	out.PlanModifierPkgs = planModifierPackages(out.Attributes)
	out.DefaultPkgs = defaultPackages(out.Attributes)
	out.ComputedPaths = computedPaths(nil, "", out.Attributes)
	out.RequiredPaths = requiredPaths(nil, "", out.Attributes)
	for _, a := range out.Attributes {
		if !computedOnly(a) {
			out.ServerPaths = serverPaths(out.ServerPaths, a.Name, a.Attributes)
		}
	}
	return out, nil
}

// serverPaths appends the paths of the computed-only attributes in attrs, in
// schema order, below the attribute at parent. The descendants of a
// computed-only attribute are computed too, so they add nothing.
func serverPaths(out []string, parent string, attrs []*tfAttr) []string {
	for _, a := range attrs {
		at := parent + "." + a.Name
		if computedOnly(a) {
			out = append(out, at)
			continue
		}
		out = serverPaths(out, at, a.Attributes)
	}
	return out
}

func computedOnly(a *tfAttr) bool { return a.Computed && !a.Optional && !a.Required }

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

// requiredPaths is computedPaths for the required attributes.
func requiredPaths(out []string, parent string, attrs []*tfAttr) []string {
	for _, a := range attrs {
		at := a.Name
		if parent != "" {
			at = parent + "." + a.Name
		}
		if a.Required {
			out = append(out, at)
		}
		out = requiredPaths(out, at, a.Attributes)
	}
	return out
}

// defaultPackages returns the default packages, such as stringdefault, that
// the static defaults of attrs and of their nested attributes use.
func defaultPackages(attrs []*tfAttr) []string {
	var packages []string
	for _, attr := range attrs {
		pkg := strings.ToLower(attr.ValueKind) + "default"
		if strings.HasPrefix(attr.Default, pkg+".") && !containsString(packages, pkg) {
			packages = append(packages, pkg)
		}
		for _, nested := range defaultPackages(attr.Attributes) {
			if !containsString(packages, nested) {
				packages = append(packages, nested)
			}
		}
	}
	return packages
}

// checkSingletonIDAttribute checks that the id attribute of a singleton with singletonID is still
// computed-only after the field lines: a line that makes it required, optional, or a configured
// default would let the user set a value that flatten replaces with the id of the server.
func checkSingletonIDAttribute(attrs []*tfAttr) error {
	for _, a := range attrs {
		if a.Name == "id" && (!computedOnly(a) || a.Default != "") {
			return errors.New("singletonID says that only the server sets the id: the id attribute must stay computed-only with no default, so its line cannot set required, optional, computed: false, or a default")
		}
	}
	return nil
}

// addSingletonID adds the fixed id attribute of a singleton (D18). With singletonID, the resource has
// an id field that the server sets, and the generic field pipeline builds the attribute.
func addSingletonID(out *tfResource, root *tfModel, r *model.Resource, singletonID bool, id string) error {
	for _, f := range r.Fields {
		if terraformName(r.Policy, r.Name, f.Name) != "id" {
			continue
		}
		if !singletonID {
			return fmt.Errorf("%s: a singleton with an id field is not supported: the id attribute is fixed", f.Name)
		}
		// The fixed fallback and the import are strings, and a request that sets the id would let the
		// user configure a value that flatten replaces.
		// The template reads and writes the model field Id.
		if f.Name != "id" {
			return fmt.Errorf("singletonID: the id property must be named exactly \"id\", not %q: the generated code reads it as the model field Id", f.Name)
		}
		if f.Type == nil || f.Type.Kind != model.String {
			return fmt.Errorf("singletonID: the id field %s must be a string, not %s", f.Name, typeName(f.Type))
		}
		if f.Behavior != model.Computed {
			return fmt.Errorf("singletonID says that only the server sets the id, but the Create or Update body of %s lets a caller set %q: add readOnly: true to its line, or mark it read-only in the contract", r.Name, f.Name)
		}
		return nil
	}
	if singletonID {
		return fmt.Errorf("singletonID says that %s has an id field, and it has none", r.Name)
	}
	// A static default puts the fixed id in the plan, so Create does not
	// show it as unknown. The user still cannot set it.
	out.Attributes = append(out.Attributes, &tfAttr{Name: "id", Kind: "String", ValueKind: "String", Computed: true,
		Description: "The fixed id of this singleton: there is one per company.",
		Default:     "stringdefault.StaticString(" + id + ")"})
	root.Fields = append(root.Fields, tfModelField{Name: "Id", Type: "types.String", TFName: "id"})
	return nil
}

// resourceAttribute builds the attribute of a top-level field: its type, its server default, a
// client-set id, and the plan modifiers of its behavior.
func (b *tfBuilder) resourceAttribute(out *tfResource, r *model.Resource, f *model.ResourceField, group string) (*tfAttr, error) {
	a, err := b.attribute(rootPath(group, terraformName(r.Policy, r.Name, f.Name)), r.Name, f.Name, f.Description, f.Type, fieldAttrs(f), responseOnly(f.Behavior))
	if err != nil {
		return nil, err
	}
	a.Name = terraformName(r.Policy, r.Name, f.Name)
	if serverDefault(r, f) {
		// The user may override this value. When it is omitted, the server
		// supplies the declared default, and Get returns it or omits it. The
		// plan modifier makes removal unknown until Update clears the value and
		// Get returns that default.
		value, err := serverDefaultValue(f.Type, *declaredDefault(f))
		if err != nil {
			return nil, fmt.Errorf("server default: %w", err)
		}
		a.Computed = true
		a.Modifiers = append(a.Modifiers, "serverDefaultModifier{value: "+value+"}")
		out.HasServerDefaults = true
		if !f.Get.Required {
			out.OmittedDefaults = append(out.OmittedDefaults, defaultPath{Path: terraformName(r.Policy, r.Name, f.Name), Value: value})
		}
		if !containsString(out.ServerDefaultKinds, a.ValueKind) {
			out.ServerDefaultKinds = append(out.ServerDefaultKinds, a.ValueKind)
		}
	}
	clientSetID := r.Policy.ClientSetID && f.Name == r.ID()
	if clientSetID {
		// The user can set the id. Without a value, the server makes one.
		// The id names the resource, so a new id replaces it: Update cannot
		// find a resource under the new id. UseStateForUnknown runs first, so
		// an id that the user removes keeps its value and replaces nothing.
		a.Required, a.Optional, a.Computed = false, true, true
		a.Modifiers = append(a.Modifiers, strings.ToLower(a.ValueKind)+"planmodifier.UseStateForUnknown()")
	}
	switch {
	case f.Behavior == model.Computed:
		markComputed(a)
		if f.Name == r.ID() {
			a.Modifiers = append(a.Modifiers, strings.ToLower(a.ValueKind)+"planmodifier.UseStateForUnknown()")
		}
	case f.Behavior == model.Immutable || clientSetID:
		// The path holds the group, as the server paths of the request do.
		b.requiresReplace(a, rootPath(group, a.Name).dotted())
	}
	return a, nil
}

// errStaleOverrideKey reports a key of a field line that the generator already applies.
var errStaleOverrideKey = errors.New("the generator already applies this key, so it changes nothing")

// applyOverrides applies the behavior-overrides file to the attributes. A field with a line
// keeps the released behavior that the line states.
func applyOverrides(out *tfResource, file *overrides.File) error {
	out.SchemaVersion = file.Schema.Version
	out.ResourceMarkdownDescription = file.MarkdownDescription
	// A component can hold a field that the user sets in one place and that the server owns in
	// another, for example a read-only copy of the request value. The line states how the user
	// sets the field. The server-owned copy takes only its text: a mode, a default, a
	// validator, or a plan modifier there would plan a value that the server does not set.
	settable := map[string]bool{}
	var collect func(attrs []*tfAttr)
	collect = func(attrs []*tfAttr) {
		for _, a := range attrs {
			if !serverOwned(a) {
				settable[a.Component+"."+a.Property] = true
			}
			collect(a.Attributes)
		}
	}
	collect(out.Attributes)
	var walk func(attrs []*tfAttr) error
	walk = func(attrs []*tfAttr) error {
		for _, a := range attrs {
			if file.Validators.Inferred != nil && !*file.Validators.Inferred {
				// No limit of the contract becomes a validator. A oneOf group validator is not a limit.
				a.Validators = slices.DeleteFunc(a.Validators, func(v string) bool { return !slices.Contains(a.GroupValidators, v) })
			} else if _, mapped := file.Enums[a.EnumSchema]; mapped && a.EnumSchema != "" && (a.ValueKind == "String" || a.ValueKind == "List" || a.ValueKind == "Set") {
				// The contract limit of an enum lists the API values, which the enums line renames
				// and filters. The validator takes the Terraform values of that line instead.
				expr, err := enumValidatorExpr(a, file, false)
				if err != nil {
					return err
				}
				inferred := "stringvalidator.OneOf("
				if a.ValueKind != "String" {
					inferred = strings.ToLower(a.ValueKind) + "validator.ValueStringsAre(stringvalidator.OneOf("
				}
				a.Validators = slices.DeleteFunc(a.Validators, func(v string) bool {
					return strings.HasPrefix(v, inferred) && !slices.Contains(a.GroupValidators, v)
				})
				a.Validators = append(a.Validators, expr)
				if a.ValueKind != "String" {
					// The acceptance test picks the value of an element from its validators, so they
					// name the Terraform values too.
					oneOf, err := enumOneOf(a, file, false)
					if err != nil {
						return err
					}
					a.ElemValidators = slices.DeleteFunc(a.ElemValidators, func(v string) bool { return strings.HasPrefix(v, "stringvalidator.OneOf(") })
					a.ElemValidators = append(a.ElemValidators, oneOf)
				}
			}
			if line, ok := file.Types[a.Component].Fields[a.Property]; ok {
				if serverOwned(a) && settable[a.Component+"."+a.Property] {
					line = overrides.Field{Description: line.Description, MarkdownDescription: line.MarkdownDescription, Deprecation: line.Deprecation}
				}
				if err := checkSiblingTargets(a, attrs, line.Validators); err != nil {
					return fmt.Errorf("%s.%s: %w", a.Component, a.Property, err)
				}
				if err := applyField(a, line, file); err != nil {
					return fmt.Errorf("%s.%s: %w", a.Component, a.Property, err)
				}
				if a.KeyedObject {
					// Each attribute is an item that the API can fill in, as the object is.
					for _, item := range a.Attributes {
						if line.UseStateForUnknown {
							item.Modifiers = append(item.Modifiers, "objectplanmodifier.UseStateForUnknown()")
						}
						if line.UseNonNullStateForUnknown {
							item.Modifiers = append(item.Modifiers, "objectplanmodifier.UseNonNullStateForUnknown()")
						}
					}
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
	if err := walk(out.Attributes); err != nil {
		return err
	}
	// Field lines run after markComputed, so they can put validators and document
	// modifiers back on a computed-only copy of a component. Configuration is
	// always null there, so those checks never run.
	stripComputedConfig(out.Attributes)
	return applyConfigChecks(out, file)
}

// stripComputedConfig removes configuration checks from a computed-only attribute
// and every descendant. UseStateForUnknown stays: a computed id keeps its value.
func stripComputedConfig(attrs []*tfAttr) {
	for _, a := range attrs {
		if computedOnly(a) {
			stripComputedNode(a)
			continue
		}
		stripComputedConfig(a.Attributes)
	}
}

func stripComputedNode(a *tfAttr) {
	a.Validators = nil
	a.GroupValidators = nil
	a.Modifiers = slices.DeleteFunc(a.Modifiers, func(m string) bool {
		return strings.HasPrefix(m, "equivalentDocument{")
	})
	for _, child := range a.Attributes {
		stripComputedNode(child)
	}
}

// requestValue reports whether the field is sent as a fixed API value and is not a Terraform attribute.
func requestValue(file *overrides.File, component, field string) bool {
	if file == nil {
		return false
	}
	return file.Types[component].Fields[field].Fixed()
}

func applyField(a *tfAttr, l overrides.Field, file *overrides.File) error {
	if err := applyValueNames(a, l); err != nil {
		return err
	}
	if err := applyTerraformType(a, l); err != nil {
		return err
	}
	if l.RequireOne {
		// On an arm of a top-level oneOf, the resource validator becomes ExactlyOneOf. Elsewhere,
		// the line names the field that holds the oneOf.
		if !a.RootArm && !slices.ContainsFunc(a.Attributes, func(c *tfAttr) bool { return c.OneOfGroup != "" }) {
			return errors.New("requireOne needs a field that holds a oneOf, or an arm of a top-level oneOf")
		}
		if !a.RootArm {
			requireOneArm(a)
		}
	}
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
		modifier := strings.ToLower(a.ValueKind) + "planmodifier.UseStateForUnknown()"
		if containsString(a.Modifiers, modifier) {
			// A client-set or computed id already has it. A key that changes nothing is an error.
			return fmt.Errorf("useStateForUnknown: %w", errStaleOverrideKey)
		}
		a.Modifiers = append(a.Modifiers, modifier)
	}
	if l.UseNonNullStateForUnknown {
		// A null state value stays unknown, so the server can fill it in. A known one stays.
		a.Modifiers = append(a.Modifiers, strings.ToLower(a.ValueKind)+"planmodifier.UseNonNullStateForUnknown()")
	}
	if l.Equality != "" {
		if a.ValueKind != "String" {
			return fmt.Errorf("equality needs a string attribute, not %s", a.ValueKind)
		}
		// First, so that RequiresReplace and the other modifiers see the state value of an equal document.
		a.Modifiers = append([]string{"equivalentDocument{equal: " + l.Equality + "Equal}"}, a.Modifiers...)
	}
	if l.Default != nil {
		expr, err := defaultExpr(a, l.Default)
		if err != nil {
			return err
		}
		a.Default = expr
		if a.Required {
			// A released default on a field that the contract requires means that the contract is
			// wrong about the field. The line states the exception with optional: true.
			if !l.Optional {
				return errors.New("the line gives a default to a field that the contract requires: add optional: true, so the overrides file states that the released attribute is optional")
			}
			a.Required, a.Optional = false, true
		}
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

// serverOwned reports whether only the server sets the attribute: it is computed and the user
// cannot set it.
func serverOwned(a *tfAttr) bool {
	return a.Computed && !a.Required && !a.Optional
}

// checkAttrMode rejects an attribute that the Framework cannot build: one that is neither required,
// optional, nor computed, or one that is required and computed. A line can cause it, for example
// computed: false on a field that the contract makes server-owned.
func checkAttrMode(a *tfAttr) error {
	switch {
	case a.Required && a.Computed:
		return errors.New("the line makes the attribute required and computed: required: true needs a field that the user sets")
	case a.Default != "" && !a.Computed:
		return errors.New("the line gives the attribute a default, but the attribute is not computed: add computed: true")
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
	case int:
		switch a.Kind {
		case "Int64":
			return fmt.Sprintf("int64default.StaticInt64(%d)", v), nil
		case "Int32":
			if v < math.MinInt32 || v > math.MaxInt32 {
				return "", fmt.Errorf("the default %d does not fit an Int32 attribute", v)
			}
			return fmt.Sprintf("int32default.StaticInt32(%d)", v), nil
		case "Float64", "Float32":
			return floatDefaultExpr(a.Kind, float64(v))
		}
	case float64:
		if a.Kind == "Float64" || a.Kind == "Float32" {
			return floatDefaultExpr(a.Kind, v)
		}
	}
	return "", fmt.Errorf("a default of type %T does not fit a %s attribute", value, a.Kind)
}

// floatDefaultExpr is the Go expression of a static default of a Float64 or Float32 attribute.
func floatDefaultExpr(kind string, v float64) (string, error) {
	bits := 64
	if kind == "Float32" {
		bits = 32
	}
	if math.IsNaN(v) || math.IsInf(v, 0) || bits == 32 && math.Abs(v) > math.MaxFloat32 {
		return "", fmt.Errorf("the default %v does not fit a %s attribute", v, kind)
	}
	// 'f' writes a Go literal, and the state text of the value, that the acceptance test compares.
	return fmt.Sprintf("%sdefault.Static%s(%s)", strings.ToLower(kind), kind, strconv.FormatFloat(v, 'f', -1, bits)), nil
}

// validatorExpr is the Go expression of a released validator.
func validatorExpr(a *tfAttr, v overrides.Validator, file *overrides.File) (string, error) {
	pkg := strings.ToLower(a.ValueKind) + "validator"
	switch {
	case v.Enum:
		return enumValidatorExpr(a, file, false)
	case v.EnumExceptZero:
		return enumValidatorExpr(a, file, true)
	case len(v.OneOf) != 0 && a.ValueKind == "String":
		quoted := make([]string, 0, len(v.OneOf))
		for _, one := range v.OneOf {
			quoted = append(quoted, strconv.Quote(one))
		}
		return "stringvalidator.OneOf(" + strings.Join(quoted, ", ") + ")", nil
	case v.SizeAtLeast != nil && (a.ValueKind == "List" || a.ValueKind == "Set" || a.ValueKind == "Map"):
		return fmt.Sprintf("%s.SizeAtLeast(%d)", pkg, *v.SizeAtLeast), nil
	case v.LengthAtLeast != nil && a.ValueKind == "String":
		return fmt.Sprintf("stringvalidator.LengthAtLeast(%d)", *v.LengthAtLeast), nil
	case len(v.Between) == 2 && slices.Contains([]string{"Float32", "Float64", "Int32", "Int64"}, a.ValueKind):
		return betweenExpr(a, v.Between)
	case len(v.ExactlyOneOf) != 0:
		return pkg + ".ExactlyOneOf(" + siblingPaths(v.ExactlyOneOf) + ")", nil
	case len(v.AtLeastOneOf) != 0:
		return pkg + ".AtLeastOneOf(" + siblingPaths(v.AtLeastOneOf) + ")", nil
	case v.AtLeast != nil && (a.ValueKind == "Int32" || a.ValueKind == "Int64"):
		if err := checkIntegerBound(a.ValueKind, "atLeast", *v.AtLeast); err != nil {
			return "", err
		}
		return fmt.Sprintf("%s.AtLeast(%s)", pkg, numberLiteral(a.ValueKind, *v.AtLeast)), nil
	case v.AtLeast != nil && a.ValueKind == "Float64":
		return fmt.Sprintf("float64validator.AtLeast(%s)", formatFloat(*v.AtLeast)), nil
	case v.RegexMatches != nil && a.ValueKind == "String":
		return regexValidator(v.RegexMatches.Pattern, v.RegexMatches.Message), nil
	}
	return "", fmt.Errorf("the validator does not fit a %s attribute", a.ValueKind)
}

// enumValidatorExpr is the validator that accepts the Terraform values of the enum of the attribute.
// The values are the zero value and the Terraform value of each accepted value, in sorted order.
// They come from the enums line, so the validator and the conversion maps cannot disagree.
// exceptZero leaves the zero value out of the validator. The conversion maps still contain it.
func enumValidatorExpr(a *tfAttr, file *overrides.File, exceptZero bool) (string, error) {
	oneOf, err := enumOneOf(a, file, exceptZero)
	if err != nil {
		return "", err
	}
	if a.ValueKind == "List" || a.ValueKind == "Set" {
		return strings.ToLower(a.ValueKind) + "validator.ValueStringsAre(" + oneOf + ")", nil
	}
	return oneOf, nil
}

// enumOneOf is the OneOf validator of one value of the enum of the attribute: the validator of a
// scalar, and the validator of each element of a list or set.
func enumOneOf(a *tfAttr, file *overrides.File, exceptZero bool) (string, error) {
	enum, ok := file.Enums[a.EnumSchema]
	collection := a.ValueKind == "List" || a.ValueKind == "Set"
	if a.EnumSchema == "" || !ok || a.ValueKind != "String" && !collection {
		return "", errors.New("the enum validator needs a field of an enum type, or a list or set of one, that has a line under enums")
	}
	var values []string
	if enum.Zero != "" && !exceptZero {
		values = append(values, enum.Zero)
	}
	for _, v := range enum.Values {
		values = append(values, enum.TFValue(v))
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
	d := declaredDefault(f)
	// Get returns the value. When it omits it, the state is null: validation
	// already requires a Get default, when there is one, to be the same.
	return d != nil && f.Get != nil && (f.Create == nil || !f.Create.Required)
}

// declaredDefault is the server default of a top-level field: the Create default,
// or the Update default of a field that only Update has. Create stores it.
func declaredDefault(f *model.ResourceField) *string {
	if f.Create != nil {
		return f.Create.Default
	}
	if f.Update != nil {
		return f.Update.Default
	}
	return nil
}

// updateOnly reports whether Create cannot send the managed field f. Create
// stores its default, and the generated Create follows with an Update when the
// configuration sets another value.
func updateOnly(f *model.ResourceField) bool {
	return f.Behavior == model.Normal && f.Create == nil && f.Update != nil
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
func addGroupValidators(attrs []*tfAttr, groups []model.OneOfGroup, name func(string) string) {
	byName := map[string]*tfAttr{}
	for _, a := range attrs {
		byName[a.Name] = a
	}
	for _, g := range groups {
		for _, arm := range g.Arms {
			a := byName[name(arm)]
			var others []string
			for _, o := range g.Arms {
				if o != arm {
					others = append(others, fmt.Sprintf("path.MatchRelative().AtParent().AtName(%q)", name(o)))
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
	models       []*tfModel
	seen         map[string]bool // model structs already added
	sent         sentSet         // component fields that a request sends in some place
	validators   []string        // resource config validators
	replaceKinds []string        // RequestReplaceKinds
	defaultKinds []string        // ServerDefaultKinds of nested attributes
	defaultPaths []defaultPath   // DefaultPaths
	file         *overrides.File
	policy       model.Policy      // names the attributes that a name line renames
	replace      bool              // Update is a full replace, so a group can hold nested fields
	owners       map[string]string // model struct name to what makes it: the resource, a component, or a group
	grouped      map[string]bool   // components whose types.<Type>.groups the builder applied
}

// claimModel records that owner makes the model struct name. Two owners of one name would share
// one struct, which holds the fields of only one of them, so the generator reports it.
func (b *tfBuilder) claimModel(name, owner string) error {
	if previous, ok := b.owners[name]; ok && previous != owner {
		return fmt.Errorf("the model struct %s is made by %s and by %s: rename the group", name, previous, owner)
	}
	b.owners[name] = owner
	return nil
}

// requireOneArm turns a oneOf that allows no arm into ExactlyOneOf. The contract
// allows none, and the released resource required one.
func requireOneArm(a *tfAttr) {
	for _, child := range a.Attributes {
		child.OneOfRequired = true
		for i, v := range child.Validators {
			child.Validators[i] = strings.Replace(v, ".ConflictsWith(", ".ExactlyOneOf(", 1)
		}
		for i, v := range child.GroupValidators {
			child.GroupValidators[i] = strings.Replace(v, ".ConflictsWith(", ".ExactlyOneOf(", 1)
		}
	}
}

// attrPath is a Terraform attribute path: "root", then the names. The step
// anyListItem is any element of a list, and anyMapValue any value of a map.
type attrPath []string

const (
	anyListItem = "[]"
	anyMapValue = "{}"
	// anySetItem is any element of a set. Paths render it as a list step.
	anySetItem = "[set]"
)

// dotted is the path in the form of tfResource.ComputedPaths: the names,
// joined by dots, without the list and map steps.
func (p attrPath) dotted() string {
	var names []string
	for _, n := range p[1:] {
		if n != anyListItem && n != anyMapValue && n != anySetItem {
			names = append(names, n)
		}
	}
	return strings.Join(names, ".")
}

func (p attrPath) expr() string {
	s := fmt.Sprintf("path.MatchRoot(%q)", p[1])
	for _, n := range p[2:] {
		switch n {
		case anyListItem, anySetItem:
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

func (b *tfBuilder) attribute(p attrPath, component, name, desc string, t *model.Type, attrs model.Attrs, computed bool) (*tfAttr, error) {
	a := &tfAttr{Name: terraformName(b.policy, component, name), Description: desc, Required: attrs.Required, Optional: !attrs.Required, Component: component, Property: name}
	if b.policy.AsBool(component, name) {
		if !emptyObject(t) {
			return nil, fmt.Errorf("%s.%s: asBool needs an object without fields", component, name)
		}
		a.Kind, a.ValueKind, a.TrueOnly = "Bool", "Bool", true
		return a, nil
	}
	if err := b.setType(a, p, t, computed); err != nil {
		return nil, err
	}
	return a, nil
}

// setType sets the kind, the type, and the validators of a. computed is true for a field that only
// the response has.
func (b *tfBuilder) setType(a *tfAttr, p attrPath, t *model.Type, computed bool) error {
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
		if sentTime(t, computed) {
			a.CustomType = rfc3339Type
			a.Validators = append(a.Validators, utcValidator)
		}
	case model.Set, model.List, model.Map:
		return b.collection(a, p, t)
	case model.Object, model.OneOf:
		a.Kind, a.ValueKind = "SingleNested", "Object"
		a.KeyedObject = t.Keyed != nil && t.Keyed.Object
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
	kind, step := "Set", anySetItem
	switch t.Kind {
	case model.List:
		kind, step = "List", anyListItem
	case model.Map:
		kind, step = "Map", anyMapValue
	}
	a.ValueKind = kind
	pkg := strings.ToLower(kind) + "validator"
	a.Validators = sizeValidator(pkg, t.MinItems, t.MaxItems)
	switch t.Elem.Kind {
	case model.Object:
		a.Kind = kind + "Nested"
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
		if t.Elem.Kind == model.Enum {
			a.EnumSchema = t.Elem.Schema
		}
		if isTime(t.Elem) {
			a.ElementType, a.ElemValidators = rfc3339Type, append(vals, utcValidator)
			vals = a.ElemValidators
		}
		if len(vals) > 0 {
			a.Validators = append(a.Validators, fmt.Sprintf("%s.Value%ssAre(%s)", pkg, elem, strings.Join(vals, ", ")))
		}
	default:
		return fmt.Errorf("%s of %s is not supported", t.Kind, t.Elem.Kind)
	}
	return nil
}

// applyTerraformType keeps the released Terraform type of an int32 attribute, or of a list or set
// of int32: Int64. The validators follow the type.
func applyTerraformType(a *tfAttr, l overrides.Field) error {
	if l.TerraformType == "" {
		return nil
	}
	retype := func(vs []string) []string {
		out := make([]string, 0, len(vs))
		for _, v := range vs {
			out = append(out, strings.NewReplacer("int32validator.", "int64validator.", "ValueInt32sAre(", "ValueInt64sAre(").Replace(v))
		}
		return out
	}
	switch {
	case a.Kind == "Int32":
		a.Kind, a.ValueKind = "Int64", "Int64"
	case (a.Kind == "Set" || a.Kind == "List") && a.ElementType == "types.Int32Type":
		a.ElementType = "types.Int64Type"
		a.ElemValidators = retype(a.ElemValidators)
	default:
		return fmt.Errorf("terraformType Int64 needs an int32 attribute, or a list or set of int32, not a %s", a.Kind)
	}
	a.Validators, a.GroupValidators = retype(a.Validators), retype(a.GroupValidators)
	return nil
}

// applyValueNames makes a list or set of integers a list or set of the names of its values. Only
// the names are valid, so the line also brings their OneOf validator.
func applyValueNames(a *tfAttr, l overrides.Field) error {
	if len(l.ValueNames) == 0 {
		return nil
	}
	if a.Kind != "Set" && a.Kind != "List" || a.ElementType != "types.Int32Type" && a.ElementType != "types.Int64Type" {
		return fmt.Errorf("valueNames needs a list or set of integers, not a %s of %s", a.Kind, a.ElementType)
	}
	names := make([]string, 0, len(l.ValueNames))
	for _, v := range l.SortedValueNames() {
		names = append(names, v.Name)
	}
	slices.Sort(names)
	quoted := make([]string, 0, len(names))
	for _, n := range names {
		quoted = append(quoted, strconv.Quote(n))
	}
	oneOf := "stringvalidator.OneOf(" + strings.Join(quoted, ", ") + ")"
	pkg := strings.ToLower(a.Kind) + "validator"
	a.Validators = slices.DeleteFunc(a.Validators, func(v string) bool { return strings.Contains(v, ".ValueInt") })
	a.ElementType, a.ElemValidators = "types.StringType", []string{oneOf}
	a.Validators = append(a.Validators, pkg+".ValueStringsAre("+oneOf+")")
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
	groups := newNestedGroups(b, t.Schema)
	for _, f := range t.Fields {
		if requestValue(b.file, t.Schema, f.Name) {
			continue
		}
		group := groups.of(f.Name)
		child := append(append(attrPath{}, p...), terraformName(b.policy, t.Schema, f.Name))
		if group != "" {
			child = append(append(attrPath{}, p...), group, terraformName(b.policy, t.Schema, f.Name))
		}
		only := b.sent.responseOnly(t.Schema, f)
		a, err := b.attribute(child, t.Schema, f.Name, f.Description, f.Type, nestedAttrs(f), only)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", f.Name, err)
		}
		if f.Attrs.Computed && !a.Required {
			// A computed line, or an attribute of a keyedBy object, which the API can fill.
			a.Computed = true
		}
		switch f.Behavior {
		case model.Computed:
			markComputed(a)
		case model.Immutable:
			b.requiresReplace(a, child.dotted())
		}
		if err := b.nestedServerDefault(a, child, t.Schema, f); err != nil {
			return nil, fmt.Errorf("%s: %w", f.Name, err)
		}
		field := b.modelField(t.Schema, f.Name, f.Type, f.Behavior, only, f.Attrs.Computed)
		if group != "" {
			if err := groups.add(group, f, a, field); err != nil {
				return nil, err
			}
			a, field, ok, err := groups.first(group)
			if err != nil {
				return nil, err
			}
			if ok {
				attrs = append(attrs, a)
				fields = append(fields, field)
			}
			continue
		}
		attrs = append(attrs, a)
		fields = append(fields, field)
	}
	oneOfs := t.Groups
	if t.Kind == model.OneOf {
		group := model.OneOfGroup{AllowNone: t.AllowNone}
		for _, f := range t.Fields {
			group.Arms = append(group.Arms, f.Name)
		}
		oneOfs = append(oneOfs, group)
	}
	if err := groups.addValidators(attrs, b.policy.WithDefaultArms(t.Schema, oneOfs)); err != nil {
		return nil, err
	}
	if err := groups.check(attrs); err != nil {
		return nil, err
	}
	name := typeModelName(t)
	if err := b.claimModel(name, "the component "+t.Schema); err != nil {
		return nil, err
	}
	if !b.seen[name] {
		b.seen[name] = true
		b.models = append(b.models, &tfModel{Name: name, Fields: fields})
		return attrs, nil
	}
	// One model struct serves every place of the component. A computed
	// object is a types.Object, so it must be computed in every place.
	for _, m := range b.models {
		if m.Name == name && !slices.Equal(m.Fields, fields) {
			return nil, fmt.Errorf("component %s needs another model struct in each place: %s", t.Schema, modelFieldDifference(m.Fields, fields))
		}
	}
	return attrs, nil
}

// nestedServerDefault makes a an Optional and Computed attribute with the server-default plan
// modifier, when f is a nested field of component that the requests may omit and whose server
// default the response states. A field with a line in the behavior-overrides file keeps the
// released behavior that the line states.
func (b *tfBuilder) nestedServerDefault(a *tfAttr, p attrPath, component string, f *model.Field) error {
	if b.file != nil {
		if _, line := b.file.Types[component].Fields[f.Name]; line {
			return nil
		}
	}
	if f.Behavior != model.Normal || f.Create == nil || f.Create.Required || f.Create.Default == nil {
		return nil
	}
	d := *f.Create.Default
	if !f.Attrs.Required && (f.Attrs.Default == nil || *f.Attrs.Default != d) {
		return nil
	}
	if slices.Contains(p, anySetItem) {
		// Flatten cannot match a set element that the API returns without the default to the
		// configured element, so it could not keep the configured default.
		return fmt.Errorf("%s: a nested server default inside a set is unsupported", p.dotted())
	}
	value, err := serverDefaultValue(f.Type, d)
	if err != nil {
		return fmt.Errorf("server default: %w", err)
	}
	a.Computed = true
	a.Modifiers = append(a.Modifiers, "serverDefaultModifier{value: "+value+"}")
	b.defaultPaths = append(b.defaultPaths, defaultPath{Path: p.dotted(), Value: value, Implicit: f.Type.ImplicitDefault})
	if !containsString(b.defaultKinds, a.ValueKind) {
		b.defaultKinds = append(b.defaultKinds, a.ValueKind)
	}
	return nil
}

// defaultPath is a nested attribute with a server default: its path in the form of
// ComputedPaths, and the Go expression of the default. Implicit is true for the zero value of a
// scalar without presence, which the contract does not declare.
type defaultPath struct {
	Path, Value string
	Implicit    bool
}

// DefaultObjects are the paths of the objects that hold a nested attribute with a declared server
// default, sorted. Only these objects read "" or a declared default as empty: the service fills
// such an object when a request omits it. An implicit zero default states no such thing.
func (r *tfResource) DefaultObjects() []string {
	var out []string
	for _, d := range r.DefaultPaths {
		if d.Implicit {
			continue
		}
		if i := strings.LastIndex(d.Path, "."); i > 0 && !slices.Contains(out, d.Path[:i]) {
			out = append(out, d.Path[:i])
		}
	}
	slices.Sort(out)
	return out
}

// RequestFilter reports whether requestValue has attributes to drop from a request value.
func (r *tfResource) RequestFilter() bool { return len(r.ServerPaths) != 0 || len(r.DefaultPaths) != 0 }

// modelFieldDifference names the first model field that differs between two
// places of a component, with its Go type in each place.
func modelFieldDifference(first, other []tfModelField) string {
	for i := range min(len(first), len(other)) {
		if first[i] != other[i] {
			return fmt.Sprintf("field %s is %s in one place and %s in another", first[i].TFName, first[i].Type, other[i].Type)
		}
	}
	return fmt.Sprintf("the places have %d and %d fields", len(first), len(other))
}

// requiresReplace makes a change of the immutable attribute a at path at
// replace the resource. The stock RequiresReplace compares whole values. When
// the resource changes, Terraform plans the attributes that the server sets as
// unknown, so a value that holds them would replace the resource on every
// change. requestReplaceModifier compares the values without them.
func (b *tfBuilder) requiresReplace(a *tfAttr, at string) {
	if !hasServerValue(a.Attributes) {
		a.Modifiers = append(a.Modifiers, strings.ToLower(a.ValueKind)+"planmodifier.RequiresReplace()")
		return
	}
	a.Modifiers = append(a.Modifiers, fmt.Sprintf("requestReplaceModifier{at: %q}", at))
	if !containsString(b.replaceKinds, a.ValueKind) {
		b.replaceKinds = append(b.replaceKinds, a.ValueKind)
	}
}

// checkImmutableValues rejects an immutable value that holds an optional and
// computed attribute. Only a behavior-overrides line makes a nested attribute
// optional and computed. When the user leaves it out and the resource
// changes, Terraform plans it as unknown. The replace check cannot tell that
// from a change of the user, so the resource would be replaced.
func checkImmutableValues(attrs []*tfAttr, parent string) error {
	for _, a := range attrs {
		at := a.Name
		if parent != "" {
			at = parent + "." + a.Name
		}
		if replaces(a) {
			// The attribute itself, not only a descendant. Optional+Computed plans an omitted
			// value as unknown, and the stock RequiresReplace treats that unknown as a change.
			// UseStateForUnknown runs first and plans the prior value instead, as for a
			// client-set id.
			if a.Optional && a.Computed && !keepsState(a) {
				return fmt.Errorf("%s: an optional and computed attribute cannot require replacement. Omitting it plans the value as unknown and replaces the resource when something else changes", at)
			}
			if child := optionalComputed(a.Attributes, at); child != "" {
				return fmt.Errorf("%s: the immutable value holds the optional and computed attribute %s. A change of another attribute would plan it as unknown and replace the resource", at, child)
			}
		}
		if err := checkImmutableValues(a.Attributes, at); err != nil {
			return err
		}
	}
	return nil
}

// checkOmittedMaskClears rejects updateMask: omit when a field that Update sends could be removed
// from the configuration: Update sends no mask, and the server keeps a field that the body omits,
// so removing it would not clear it. A required field, a field with a default, and a field that
// keeps its state value are always in the body.
func checkOmittedMaskClears(r *model.Resource, attrs []*tfAttr, file *overrides.File) error {
	if file == nil || file.UpdateMask != overrides.UpdateMaskOmit {
		return nil
	}
	for _, f := range r.Fields {
		if f.Update == nil || f.Behavior != model.Normal {
			continue
		}
		if file.Types[r.Name].Fields[f.Name].Fixed() {
			// A requestValue field has no attribute, and Update always sends it.
			continue
		}
		if file.Types[r.Name].Fields[f.Name].SendNullAsEmpty {
			// Expand sends "" for a removed value, so the server clears it.
			continue
		}
		// No field is in a groups object: a group needs a full-replace Update, and
		// updateMask: omit a PATCH.
		name := terraformName(r.Policy, r.Name, f.Name)
		i := slices.IndexFunc(attrs, func(a *tfAttr) bool { return a.Name == name })
		if i < 0 {
			return fmt.Errorf("%s: updateMask: omit found no attribute for the Update field", name)
		}
		if attrs[i].Default != "" && isZeroDefault(file.Types[r.Name].Fields[f.Name].Default) && !f.Update.Presence {
			// Removing the value plans the zero, but a field without presence does not send it,
			// and with no mask the server keeps the old value.
			return fmt.Errorf("%s: updateMask: omit cannot clear this attribute: its default is the zero value, which Update does not send without x-coralogix-presence", name)
		}
		if attrs[i].Required || attrs[i].Default != "" || keepsAnyState(attrs[i]) {
			continue
		}
		return fmt.Errorf("%s: updateMask: omit sends no mask, so removing this optional attribute would not clear it. Make it required or give it a default", name)
	}
	return nil
}

// isZeroDefault reports whether the default of a field line is the zero value of its kind.
func isZeroDefault(v any) bool {
	switch x := v.(type) {
	case bool:
		return !x
	case int:
		return x == 0
	case float64:
		return x == 0
	case string:
		return x == ""
	}
	return false
}

// keepsState reports whether an omitted value is planned as the prior state.
func keepsState(a *tfAttr) bool {
	return slices.ContainsFunc(a.Modifiers, func(m string) bool {
		return strings.HasSuffix(m, "planmodifier.UseStateForUnknown()")
	})
}

// keepsAnyState reports whether an omitted value is planned as the prior state, or as the prior
// non-null state. Either keeps the field in an Update without a mask: a null prior value has
// nothing to clear.
func keepsAnyState(a *tfAttr) bool {
	return keepsState(a) || slices.ContainsFunc(a.Modifiers, func(m string) bool {
		return strings.HasSuffix(m, "planmodifier.UseNonNullStateForUnknown()")
	})
}

// replaces reports whether a change of a replaces the resource.
func replaces(a *tfAttr) bool {
	return slices.ContainsFunc(a.Modifiers, func(m string) bool {
		return strings.HasSuffix(m, "planmodifier.RequiresReplace()") || strings.HasPrefix(m, "requestReplaceModifier{")
	})
}

// optionalComputed returns the path of the first optional and computed
// attribute in attrs or their descendants, or "".
func optionalComputed(attrs []*tfAttr, parent string) string {
	for _, a := range attrs {
		at := parent + "." + a.Name
		if a.Optional && a.Computed {
			return at
		}
		if child := optionalComputed(a.Attributes, at); child != "" {
			return child
		}
	}
	return ""
}

// hasServerValue reports whether attrs or their descendants have a
// computed-only attribute.
func hasServerValue(attrs []*tfAttr) bool {
	for _, a := range attrs {
		if computedOnly(a) || hasServerValue(a.Attributes) {
			return true
		}
	}
	return false
}

// nestedAttrs are the attributes that decide Required for a nested field.
// Create decides, as for a top-level field. A field that Create does not
// send has the attributes of the resource response.
func nestedAttrs(f *model.Field) model.Attrs {
	if f.Create != nil {
		return *f.Create
	}
	return f.Attrs
}

// modelField returns the model field of a field with behavior. responseOnly is true when no
// request sends the field.
func (b *tfBuilder) modelField(component, name string, t *model.Type, behavior model.Behavior, responseOnly, computed bool) tfModelField {
	var goType string
	switch t.Kind {
	case model.String, model.Enum:
		goType = "types.String"
		if sentTime(t, responseOnly) {
			goType = "timetypes.RFC3339"
		}
	case model.Bool:
		goType = "types.Bool"
	case model.Number:
		goType = "types.Float64"
		if t.Format == "float" {
			goType = "types.Float32"
		}
	case model.Integer:
		goType = "types.Int64"
		if t.Format == "int32" && terraformType(b.policy, component, name) == "" {
			goType = "types.Int32"
		}
	case model.Set:
		goType = "types.Set"
	case model.List:
		goType = "types.List"
	case model.Map:
		goType = "types.Map"
	case model.Object, model.OneOf:
		if b.policy.AsBool(component, name) {
			goType = "types.Bool"
		} else if behavior == model.Computed || computed {
			goType = "types.Object"
		} else {
			goType = "*" + typeModelName(t)
		}
	}
	return tfModelField{Name: camelize(name), Type: goType, TFName: terraformName(b.policy, component, name)}
}

// emptyObject reports whether t is an object without fields, which the SDK holds as a map.
func emptyObject(t *model.Type) bool {
	return (t.Kind == model.Object || t.Kind == model.OneOf) && len(t.Fields) == 0
}

// modelTypeName is the Terraform model struct of a component schema.
func modelTypeName(schema string) string { return model.GoName(schema) + "Model" }

// typeModelName is the Terraform model struct of an object type. A keyedBy object makes several
// objects of one component, which the variant tells apart.
func typeModelName(t *model.Type) string { return model.GoName(t.Schema) + t.Variant + "Model" }

// scalar returns the attribute kind and the validators of a scalar type.
func scalar(t *model.Type) (string, []string, error) {
	switch t.Kind {
	case model.String:
		var vals []string
		if v := lengthValidator(t.MinLength, t.MaxLength); v != "" {
			vals = append(vals, v)
		}
		if t.Pattern != "" {
			vals = append(vals, patternValidator(t.Pattern))
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

// rfc3339Type is the Terraform type of a time that a request sends. It checks the RFC 3339 format,
// and a value that differs only in fractional seconds plans no change.
const rfc3339Type = "timetypes.RFC3339Type{}"

// utcValidator requires a time in UTC. The API returns every time in UTC, so a configured offset
// such as +02:00 would read back as another text, and the apply would fail.
const utcValidator = "stringvalidator.RegexMatches(regexp.MustCompile(`(?i)z$`), \"must be a UTC time that ends in Z, for example 2030-01-01T00:00:00Z\")"

// isTime reports whether t is a date-time string.
func isTime(t *model.Type) bool { return t.Kind == model.String && t.Format == "date-time" }

// sentTime reports whether t is a time that a request sends. A time that only the response has stays
// a plain string, as the API returns it, with no validator.
func sentTime(t *model.Type, computed bool) bool { return isTime(t) && !computed }

// responseOnly reports whether only the response has the value of a field with behavior.
func responseOnly(behavior model.Behavior) bool {
	return behavior != model.Normal && behavior != model.Immutable
}

// sentSet holds the fields of components that a request sends in at least one place, as
// "Component.field".
type sentSet map[string]bool

// sentFields returns the component fields that a request sends in some place of r.
func sentFields(r *model.Resource) sentSet {
	sent := sentSet{}
	for _, f := range r.Fields {
		sent.add(f.Type)
	}
	return sent
}

func (s sentSet) add(t *model.Type) {
	if t == nil {
		return
	}
	s.add(t.Elem)
	for _, f := range t.Fields {
		if !responseOnly(f.Behavior) {
			s[t.Schema+"."+f.Name] = true
		}
		s.add(f.Type)
	}
}

// responseOnly reports whether no request sends field f of component in any place. One model
// struct serves every place of a component, so a time field needs one type in all of them. A
// field inside a computed value has no behavior of its own (""), so another place decides:
// a time that Create sends in one place is timetypes.RFC3339 also where only the response has it.
func (s sentSet) responseOnly(component string, f *model.Field) bool {
	return responseOnly(f.Behavior) && !s[component+"."+f.Name]
}

// patternValidator returns a validator that the value matches pattern. The
// pattern is a raw string literal when it can be, so it reads as in the API.
// An empty message makes the validator show the pattern.
func patternValidator(pattern string) string {
	return regexValidator(pattern, "")
}

// regexValidator returns a validator that the value matches pattern, with message.
func regexValidator(pattern, message string) string {
	literal := strconv.Quote(pattern)
	if !strings.Contains(pattern, "`") {
		literal = "`" + pattern + "`"
	}
	return fmt.Sprintf("stringvalidator.RegexMatches(regexp.MustCompile(%s), %s)", literal, strconv.Quote(message))
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
	return TerraformName(property)
}

// rootPath is the attribute path of a top-level attribute with the Terraform name name: below
// its group, when a behavior-overrides group holds it.
func rootPath(group, name string) attrPath {
	if group == "" {
		return attrPath{"root", name}
	}
	return attrPath{"root", group, name}
}

// tfGroups builds the groups of the behavior-overrides file: required objects that hold
// top-level fields and that the API does not have.
type tfGroups struct {
	file     *overrides.File
	resource string
	attrs    map[string]*tfAttr
	models   map[string]*tfModel
}

// add puts the attribute and the model field of a top-level field in its group. It reports
// whether the group is new.
func (g *tfGroups) add(group string, a *tfAttr, field tfModelField) bool {
	attr, found := g.attrs[group]
	if !found {
		attr = &tfAttr{Name: group, Kind: "SingleNested", ValueKind: "Object", Required: true,
			Description: g.file.Groups[group].MarkdownDescription}
		g.attrs[group] = attr
		g.models[group] = &tfModel{Name: groupModelName(g.resource, group)}
	}
	attr.Attributes = append(attr.Attributes, a)
	g.models[group].Fields = append(g.models[group].Fields, field)
	return !found
}

// oneOf returns the group that holds every arm of the top-level oneOf g, or "" when no group
// holds an arm. A group cannot hold some of the arms only.
func (g *tfGroups) oneOf(o model.OneOfGroup) (string, error) {
	group := g.file.GroupOf(o.Arms[0])
	for _, arm := range o.Arms[1:] {
		if g.file.GroupOf(arm) != group {
			return "", fmt.Errorf("groups: the oneOf arms %s must all be in one group, or in none", strings.Join(o.Arms, ", "))
		}
	}
	return group, nil
}

// check reports a group field that is no top-level field of the resource, and a group whose
// Terraform or Go name is also the name of another top-level attribute.
func (g *tfGroups) check(root []*tfAttr) error {
	for _, name := range g.file.GroupNames() {
		for _, a := range root {
			if a != g.attrs[name] && (a.Name == name || camelize(a.Name) == camelize(name)) {
				return fmt.Errorf("groups.%s: the name is also the top-level attribute %s", name, a.Name)
			}
		}
		if g.file.Groups[name].RequireOne && !oneOfArmsOnly(g.attrs[name].attrsOrNil()) {
			// Flatten checks that the response has one of the fields of the group, so every
			// field must be an arm of the same oneOf.
			return fmt.Errorf("groups.%s: requireOne needs the arms of one oneOf of the resource, and no other field", name)
		}
		for _, field := range g.file.Groups[name].Fields {
			if !slices.ContainsFunc(g.attrs[name].attrsOrNil(), func(a *tfAttr) bool { return a.Property == field }) {
				return fmt.Errorf("groups.%s: %s is no top-level field of %s that Terraform shows", name, field, g.resource)
			}
		}
	}
	return nil
}

// oneOfArmsOnly reports whether attrs are arms of one oneOf, and nothing else.
func oneOfArmsOnly(attrs []*tfAttr) bool {
	return len(attrs) != 0 && attrs[0].OneOfGroup != "" &&
		!slices.ContainsFunc(attrs, func(a *tfAttr) bool { return a.OneOfGroup != attrs[0].OneOfGroup })
}

func (a *tfAttr) attrsOrNil() []*tfAttr {
	if a == nil {
		return nil
	}
	return a.Attributes
}

// groupModelName is the Go type of the model of a group.
func groupModelName(resource, group string) string {
	return modelTypeName(resource + camelize(group))
}

// checkGroupField rejects a top-level field that a group cannot hold. A group holds fields that
// the user sets and that Update sends; the id names the resource, and an immutable field or a
// server default needs plan logic at the top level.
func checkGroupField(r *model.Resource, f *model.ResourceField, group string) error {
	switch {
	case group == "":
		return nil
	case !r.Replace:
		return fmt.Errorf("groups.%s: a group needs a full-replace Update (PUT): a PATCH mask names the API fields", group)
	case f.Name == r.ID():
		return fmt.Errorf("groups.%s: the resource id %s cannot be in a group", group, f.Name)
	case f.Behavior != model.Normal:
		return fmt.Errorf("groups.%s: %s is %s; a group holds only fields that the user sets and Update sends", group, f.Name, f.Behavior)
	case serverDefault(r, f):
		return fmt.Errorf("groups.%s: %s has a server default; a group cannot hold it", group, f.Name)
	}
	return nil
}

// betweenExpr is the Between validator of a number attribute.
func betweenExpr(a *tfAttr, bounds []float64) (string, error) {
	for _, bound := range bounds {
		if err := checkIntegerBound(a.ValueKind, "between bound", bound); err != nil {
			return "", err
		}
	}
	pkg := strings.ToLower(a.ValueKind) + "validator"
	return fmt.Sprintf("%s.Between(%s, %s)", pkg, numberLiteral(a.ValueKind, bounds[0]), numberLiteral(a.ValueKind, bounds[1])), nil
}

// maxExactInteger is the largest integer that a float64, which holds a YAML number of the
// behavior-overrides file, stores exactly.
const maxExactInteger = 1 << 53

// checkIntegerBound rejects a bound (between or atLeast, named by what) of an integer attribute
// that the validator would not state exactly: a fraction would be cut, and a number past 2^53 has
// already lost its last digits.
func checkIntegerBound(kind, what string, v float64) error {
	switch kind {
	case "Int32":
		if v != math.Trunc(v) || v < math.MinInt32 || v > math.MaxInt32 {
			return fmt.Errorf("%s %v is not an int32", what, v)
		}
	case "Int64":
		if v != math.Trunc(v) || math.Abs(v) > maxExactInteger {
			return fmt.Errorf("%s %v is not an integer of at most 2^53, which the file can state exactly", what, v)
		}
	}
	return nil
}

// numberLiteral is the Go literal of a validator bound of a number attribute of the kind.
func numberLiteral(kind string, v float64) string {
	if kind == "Int32" || kind == "Int64" {
		return strconv.FormatInt(int64(v), 10)
	}
	s := strconv.FormatFloat(v, 'f', -1, 64)
	if !strings.Contains(s, ".") {
		s += ".0"
	}
	return s
}

// siblingPaths are the path expressions of sibling attributes, by their Terraform names.
// checkSiblingTargets rejects an exactlyOneOf or atLeastOneOf name that is not another attribute
// of the same object. The validator renders the name into a path, so a typo would still compile
// and fail only when a configuration reaches it.
func checkSiblingTargets(a *tfAttr, siblings []*tfAttr, validators []overrides.Validator) error {
	for _, v := range validators {
		for _, name := range slices.Concat(v.ExactlyOneOf, v.AtLeastOneOf) {
			found := name != a.Name && slices.ContainsFunc(siblings, func(s *tfAttr) bool { return s.Name == name })
			if !found {
				return fmt.Errorf("validator target %q is not another attribute of the same object", name)
			}
		}
	}
	return nil
}

func siblingPaths(names []string) string {
	paths := make([]string, 0, len(names))
	for _, name := range names {
		paths = append(paths, fmt.Sprintf("path.MatchRelative().AtParent().AtName(%q)", name))
	}
	return strings.Join(paths, ", ")
}

// rootAttributes adds the attributes and the model fields of the top-level fields: at the top
// level, or in their group.
func (b *tfBuilder) rootAttributes(out *tfResource, root *tfModel, r *model.Resource, file *overrides.File, groups *tfGroups) error {
	for _, f := range r.Fields {
		if requestValue(file, r.Name, f.Name) {
			continue
		}
		group := file.GroupOf(f.Name)
		if err := checkGroupField(r, f, group); err != nil {
			return err
		}
		a, err := b.resourceAttribute(out, r, f, group)
		if err != nil {
			return fmt.Errorf("%s: %w", f.Name, err)
		}
		field := b.modelField(r.Name, f.Name, f.Type, f.Behavior, responseOnly(f.Behavior), r.Policy.ComputedLine(r.Name, f.Name))
		if group == "" {
			out.Attributes = append(out.Attributes, a)
			root.Fields = append(root.Fields, field)
			continue
		}
		if groups.add(group, a, field) {
			if err := b.claimModel(groups.models[group].Name, "groups."+group); err != nil {
				return err
			}
			out.Attributes = append(out.Attributes, groups.attrs[group])
			root.Fields = append(root.Fields, tfModelField{Name: camelize(group), Type: "*" + groups.models[group].Name, TFName: group})
			b.models = append(b.models, groups.models[group])
		}
	}
	return nil
}

// rootOneOfs adds the validators of the top-level oneOf groups: a resource validator, or arm
// validators when a behavior-overrides group holds the arms.
func (b *tfBuilder) rootOneOfs(out *tfResource, r *model.Resource, file *overrides.File, groups *tfGroups) error {
	for _, g := range r.Groups {
		if group, err := groups.oneOf(g); err != nil {
			return err
		} else if group != "" {
			// The arms are in one group, which is an object: they check each other inside it.
			addGroupValidators(groups.attrs[group].Attributes, []model.OneOfGroup{g}, func(arm string) string { return terraformName(r.Policy, r.Name, arm) })
			if file.Groups[group].RequireOne {
				requireOneArm(groups.attrs[group])
			}
			continue
		}
		var arms []string
		for _, arm := range g.Arms {
			name := terraformName(r.Policy, r.Name, arm)
			arms = append(arms, rootPath(file.GroupOf(arm), name).expr())
			if file.Types[r.Name].Fields[arm].RequireOne {
				// The released resource required one arm although the contract allows none.
				g.AllowNone = false
			}
		}
		for _, arm := range g.Arms {
			name := terraformName(r.Policy, r.Name, arm)
			for _, a := range out.Attributes {
				if a.Name == name {
					// The resource validator checks the arms; the acceptance test sets one of them.
					a.RootArm, a.OneOfGroup, a.OneOfRequired = true, strings.Join(g.Arms, ","), !g.AllowNone
				}
			}
		}
		b.validators = append(b.validators, groupValidator("resourcevalidator", g, true)+"(\n"+strings.Join(arms, ",\n")+",\n)")
	}
	return nil
}

// nestedGroups builds the groups of a nested component (types.<Type>.groups): required objects
// that hold fields of the component and that the API does not have.
type nestedGroups struct {
	b         *tfBuilder
	component string
	lines     map[string]overrides.Group
	attrs     map[string]*tfAttr
	models    map[string]*tfModel
	fresh     map[string]bool // the group attribute was made by the last add
}

func newNestedGroups(b *tfBuilder, component string) *nestedGroups {
	g := &nestedGroups{b: b, component: component, attrs: map[string]*tfAttr{}, models: map[string]*tfModel{}, fresh: map[string]bool{}}
	if b.file != nil {
		g.lines = b.file.Types[component].Groups
	}
	if len(g.lines) != 0 {
		if b.grouped == nil {
			b.grouped = map[string]bool{}
		}
		b.grouped[component] = true
	}
	return g
}

// checkNestedGroupsUsed reports types.<Type>.groups of a type that no object of the resource
// has, such as a misspelled type name: the groups would be dropped without a word.
func (b *tfBuilder) checkNestedGroupsUsed() error {
	if b.file == nil {
		return nil
	}
	for _, name := range slices.Sorted(maps.Keys(b.file.Types)) {
		if len(b.file.Types[name].Groups) != 0 && !b.grouped[name] {
			return fmt.Errorf("types.%s.groups: no object of the resource is %s", name, name)
		}
	}
	return nil
}

// of returns the group that holds field, or "".
func (g *nestedGroups) of(field string) string {
	for name, line := range g.lines {
		if slices.Contains(line.Fields, field) {
			return name
		}
	}
	return ""
}

// add puts the attribute and the model field of field f in its group. A group holds fields that
// the user sets and that a full replace sends, as a top-level group does.
func (g *nestedGroups) add(group string, f *model.Field, a *tfAttr, field tfModelField) error {
	switch {
	case !g.b.replace:
		return fmt.Errorf("types.%s.groups.%s: a group needs a full-replace Update (PUT): a PATCH mask names the API fields", g.component, group)
	case f.Behavior != model.Normal:
		return fmt.Errorf("types.%s.groups.%s: %s is %s; a group holds only fields that the user sets and Update sends", g.component, group, f.Name, f.Behavior)
	case slices.ContainsFunc(a.Modifiers, func(m string) bool { return strings.HasPrefix(m, "serverDefaultModifier") }):
		return fmt.Errorf("types.%s.groups.%s: %s has a server default; a group cannot hold it", g.component, group, f.Name)
	}
	attr, found := g.attrs[group]
	g.fresh[group] = !found
	if !found {
		attr = &tfAttr{Name: group, Kind: "SingleNested", ValueKind: "Object", Required: true, Component: g.component,
			Description: g.lines[group].MarkdownDescription}
		g.attrs[group] = attr
		g.models[group] = &tfModel{Name: groupModelName(g.component, group)}
	}
	attr.Attributes = append(attr.Attributes, a)
	g.models[group].Fields = append(g.models[group].Fields, field)
	return nil
}

// first returns the group attribute and its model field when the last add made the group, so the
// caller adds them once, in the place of the first field of the group.
func (g *nestedGroups) first(group string) (*tfAttr, tfModelField, bool, error) {
	if !g.fresh[group] {
		return nil, tfModelField{}, false, nil
	}
	model := g.models[group]
	if err := g.b.claimModel(model.Name, "types."+g.component+".groups."+group); err != nil {
		return nil, tfModelField{}, false, err
	}
	if !g.b.seen[model.Name] {
		g.b.seen[model.Name] = true
		g.b.models = append(g.b.models, model)
	}
	return g.attrs[group], tfModelField{Name: camelize(group), Type: "*" + model.Name, TFName: group}, true, nil
}

// addValidators adds the oneOf validators: inside a group when the group holds every arm, else to
// the attributes of the component. A group cannot hold some of the arms only.
func (g *nestedGroups) addValidators(attrs []*tfAttr, oneOfs []model.OneOfGroup) error {
	name := func(arm string) string { return terraformName(g.b.policy, g.component, arm) }
	for _, o := range oneOfs {
		group := g.of(o.Arms[0])
		for _, arm := range o.Arms[1:] {
			if g.of(arm) != group {
				return fmt.Errorf("types.%s.groups: the oneOf arms %s must all be in one group, or in none", g.component, strings.Join(o.Arms, ", "))
			}
		}
		if group == "" {
			addGroupValidators(attrs, []model.OneOfGroup{o}, name)
			continue
		}
		addGroupValidators(g.attrs[group].Attributes, []model.OneOfGroup{o}, name)
		if g.lines[group].RequireOne {
			requireOneArm(g.attrs[group])
		}
	}
	return nil
}

// check reports a group field that is no field of the component, and a group whose Terraform or
// Go name is also the name of another attribute.
func (g *nestedGroups) check(attrs []*tfAttr) error {
	for _, name := range sortedGroupNames(g.lines) {
		for _, a := range attrs {
			if a != g.attrs[name] && (a.Name == name || camelize(a.Name) == camelize(name)) {
				return fmt.Errorf("types.%s.groups.%s: the name is also the attribute %s", g.component, name, a.Name)
			}
		}
		for _, field := range g.lines[name].Fields {
			if !slices.ContainsFunc(g.attrs[name].attrsOrNil(), func(a *tfAttr) bool { return a.Property == field }) {
				return fmt.Errorf("types.%s.groups.%s: %s is no field of %s that Terraform shows", g.component, name, field, g.component)
			}
		}
		if g.lines[name].RequireOne && !oneOfArmsOnly(g.attrs[name].attrsOrNil()) {
			// Flatten checks that the response has one of the fields of the group, so every
			// field must be an arm of the same oneOf.
			return fmt.Errorf("types.%s.groups.%s: requireOne needs the arms of one oneOf of %s, and no other field", g.component, name, g.component)
		}
	}
	return nil
}

func sortedGroupNames(groups map[string]overrides.Group) []string {
	names := make([]string, 0, len(groups))
	for name := range groups {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}
