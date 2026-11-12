package generator

import (
	"errors"
	"fmt"
	"maps"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/coralogix/coralogix-management-sdk/iac/model"
	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/overrides"
	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/version"
)

// tfResource is the template data for the Terraform schema and model files.
type tfResource struct {
	Package      string
	AfterFlatten bool
	// ModifyPlanHook: the Hooks have a ModifyPlan, that the generated ModifyPlan calls last.
	ModifyPlanHook bool
	// EqualityFile: the package has equality.go, with the document comparisons.
	EqualityFile bool
	// EqualityPlan: equality.go also defines ModifyPlan, for the equality lines of fields and the
	// enum aliases. An extra attribute with an equality line is only a plan modifier.
	EqualityPlan      bool
	VersionHeader     string
	Model             string // Go type of the resource model, for example "WidgetModel"
	Attributes        []*tfAttr
	ConfigValidators  []string // Go expressions of type resource.ConfigValidator
	Models            []*tfModel
	Conv              *convData // expand and flatten
	CRUD              *crudData // CRUD, import, and the provider data
	HasServerDefaults bool
	// SetServerDefaults: a server default is inside a set item, so its modifier reads the plan.
	SetServerDefaults bool
	// SetItemState: an object that keeps its state is inside a set item (types.<Type>.groups), so
	// its modifier takes the state item that the planned item matches.
	SetItemState       bool
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
	// SiblingRules are the exactlyOneOf and conflictsWith lines of the attribute that name only its
	// siblings in an object. The oneOfArmsValidator of the parent object checks them, so the cost does
	// not grow with the number of objects (see addArmValidators).
	SiblingRules []siblingRule
	// UpSteps are the path steps that each object level above the attribute takes, the nearest last:
	// 1 for a single object, and 2 for an item of a list, set, or map (the element and the name of
	// the collection). A "../" in a validator target goes up one level, so it removes that many steps.
	UpSteps []int
	// ElemValidators are the validators of one element of a collection of plain values, the
	// arguments of its Value<Kind>sAre validator. The acceptance test makes an element that passes.
	ElemValidators []string
	Validators     []string // Go expressions
	// GroupValidators are the oneOf group validators. They state the structure of the request, not a
	// limit, so they stay when the behavior-overrides file removes the inferred validators.
	GroupValidators []string
	// EnumValueSets are the values that an enum validator accepts, by the Go expression of the
	// validator, recorded where the validator is built. The description of an attribute lists
	// them (enumValuesInDescription), so the list is never read back from the text of a validator.
	// A validator that is no longer in Validators contributes nothing.
	EnumValueSets map[string][]string
	// OneOfGroup names the oneOf group of an arm: its arms, joined. "" when the attribute is no arm.
	// The acceptance test sets one arm of each group.
	OneOfGroup string
	// OneOfRequired says that the group needs one arm, so every config sets one.
	OneOfRequired bool
	// ArmConflicts are the attribute names of the arms that stay beside the inlined arm that this
	// field came from (inline line): the field cannot be set with any of them. The conflict is checked by
	// the validator on the parent object, as the oneOf groups are.
	ArmConflicts []string
	// ItemValidators are the validators of the object of each item of a list, set, or map of objects.
	ItemValidators []string
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
	// ObjectComponent is the API component of this object's value. It also identifies an
	// empty object, whose children cannot name the component.
	ObjectComponent string
	// PlainDescription: the docs text is Description, not MarkdownDescription.
	PlainDescription bool
	// VerbatimDescription: a description or markdownDescription line of the field states the text.
	VerbatimDescription bool
	DeprecationMessage  string
	// Sensitive: Terraform hides the value (a sensitive line).
	Sensitive bool
	Default   string // Go expression of a static default, or ""
	// DefaultRaw is the value of the default of the line, or of the extra attribute, as the file
	// states it. An object default compares it with the value that it gives the attribute.
	DefaultRaw any
	// StructDefault is a default that resolveStructDefaults builds after the field lines and the
	// extra attributes, because its Go expression holds the types of the attributes below it.
	StructDefault any
	// WriteOnly: Terraform sends the value and does not store it. Only extraAttributes set this.
	WriteOnly bool
	// TrueOnly: a bool that shows an object without fields (asBool). Only true is a valid value.
	TrueOnly bool
	// ItemModifiers are the plan modifiers of the object of each item of a list or set of objects
	// (types.<Type>.useStateForUnknown), not of the attribute that holds the list.
	ItemModifiers []string
	// FoldEnum is the enum component of the oneOf that the attribute shows as one string
	// (asString), "" for any other attribute.
	FoldEnum string
	// FoldArms are the Terraform values of a oneOf of arms without fields that the attribute shows
	// as one string (asString): the name of the arm that is set.
	FoldArms []string
	// TypedArm: the attribute is a field that a typedArms object merged from its arms. The
	// validators lines of an arm field run only when the type names the arm, so the attribute
	// itself takes none of them.
	TypedArm bool
	// Uint64: the attribute is a 64-bit number that the API sends as a string, signed or not,
	// shown as Int64.
	Uint64 bool
	// Extra: the attribute is from types.<Type>.extraAttributes, not the API. The generated
	// acceptance test leaves it out unless values or an upgrade case supplies the map: a made-up
	// map key is not a valid API field.
	Extra bool
	// ServerSetInRequest: the request requires the field, and Get requires it and marks it
	// read-only, as an id that a write sends back to name an item of an overwrite resource.
	ServerSetInRequest bool
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
	b := &tfBuilder{seen: map[string]bool{}, sent: sentFields(r, file), file: file, policy: r.Policy, replace: r.Replace, owners: map[string]string{}, itemModified: map[string]bool{}}
	out := &tfResource{Package: pkg, VersionHeader: version.Header, Model: modelTypeName(r.Name)}
	if file != nil {
		out.AfterFlatten = file.AfterFlatten
		out.ModifyPlanHook = file.ModifyPlan
	}
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
	if err := groups.check(r, out.Attributes); err != nil {
		return nil, err
	}
	if err := b.checkPlacesUsed(); err != nil {
		return nil, err
	}
	if err := b.checkNestedGroupsUsed(); err != nil {
		return nil, err
	}
	out.ConfigValidators = b.validators
	out.Models = b.models
	out.RequestReplaceKinds = b.replaceKinds
	out.DefaultPaths = b.defaultPaths
	out.SetServerDefaults = b.setDefaults
	out.SetItemState = b.setItemState
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
	addArmValidators(out.Attributes)
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

// HasTypedArms reports whether an attribute has the typedArmValidator of a typedArms object, which
// the schema file then defines.
func (r *tfResource) HasTypedArms() bool {
	var uses func(attrs []*tfAttr) bool
	uses = func(attrs []*tfAttr) bool {
		for _, a := range attrs {
			if slices.ContainsFunc(a.Validators, func(v string) bool { return strings.HasPrefix(v, typedArmValidatorName+"{") }) || uses(a.Attributes) {
				return true
			}
		}
		return false
	}
	return uses(r.Attributes)
}

// UsesAttr reports whether a static default of an attribute is an empty list or set, whose Go
// expression names attr.Value. The typedArmValidator also names it.
func (r *tfResource) UsesAttr() bool {
	if r.HasTypedArms() || r.HasArmValidators() {
		return true
	}
	var uses func(attrs []*tfAttr) bool
	uses = func(attrs []*tfAttr) bool {
		for _, a := range attrs {
			if strings.Contains(a.Default, "attr.Value") || uses(a.Attributes) {
				return true
			}
		}
		return false
	}
	return uses(r.Attributes)
}

// checkSingletonIDAttribute checks that the id attribute of a singleton with singletonID is still
// computed-only after the field lines: a line that makes it required, optional, or a configured
// default would let the user set a value that flatten replaces with the id of the server.
func checkSingletonIDAttribute(attrs []*tfAttr) error {
	for _, a := range attrs {
		if a.Name == "id" && (!computedOnly(a) || hasDefault(a)) {
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
func (b *tfBuilder) resourceAttribute(out *tfResource, r *model.Resource, f *model.ResourceField, groups []string) (*tfAttr, error) {
	a, err := b.attribute(rootPath(groups, terraformName(r.Policy, r.Name, f.Name)), r.Name, f.Name, f.Description, f.Type, fieldAttrs(f), responseOnly(f.Behavior))
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
		b.requiresReplace(a, rootPath(groups, a.Name).dotted())
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
	var walk func(attrs []*tfAttr, above [][]*tfAttr, steps []int) error
	walk = func(attrs []*tfAttr, above [][]*tfAttr, steps []int) error {
		for _, a := range attrs {
			a.UpSteps = steps
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
				if a.TypedArm {
					line.Validators = nil
				}
				if err := checkSiblingTargets(a, attrs, above, line.Validators); err != nil {
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
			if err := walk(a.Attributes, append(slices.Clone(above), attrs), append(slices.Clone(steps), levelSteps(a))); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(out.Attributes, nil, nil); err != nil {
		return err
	}
	// Field lines run after markComputed, so they can put validators and document
	// modifiers back on a computed-only copy of a component. Configuration is
	// always null there, so those checks never run.
	stripComputedConfig(out.Attributes)
	addEnumValues(out.Attributes, file.EnumValuesInDescription)
	return applyConfigChecks(out, file)
}

// addEnumValues appends the accepted values of the enum validator of an attribute to its
// description, in every attribute below attrs. The values are the ones that were recorded when the
// validator was built (EnumValueSets), so the list and the check cannot disagree: they are the
// Terraform spellings of the enums line, and the zero value only when the validator accepts it. A description that a field line states, one
// that already has "Valid values", and an attribute without an enum validator stay as they are.
// Without enumValuesInDescription (enabled false) it changes nothing.
func addEnumValues(attrs []*tfAttr, enabled bool) {
	if !enabled {
		return
	}
	for _, a := range attrs {
		if values, ok := enumValidatorValues(a); ok && !a.VerbatimDescription && !strings.Contains(a.Description, "Valid values") {
			list := fmt.Sprintf("Valid values: %q.", values)
			if a.Description != "" {
				list = " " + list
			}
			a.Description += list
		}
		addEnumValues(a.Attributes, true)
	}
}

// enumValidatorValues returns the values of the enum validators of an attribute of an enum (or a
// list or set of one). Every validator must pass, so with more than one, such as an inferred one
// and an enumExceptZero line, the accepted values are the ones that all of them accept.
func enumValidatorValues(a *tfAttr) ([]string, bool) {
	if a.EnumSchema == "" && a.FoldEnum == "" && len(a.FoldArms) == 0 {
		return nil, false
	}
	var values []string
	found := false
	for _, v := range a.Validators {
		list, ok := a.EnumValueSets[v]
		switch {
		case !ok:
		case !found:
			values, found = slices.Clone(list), true
		default:
			values = slices.DeleteFunc(values, func(value string) bool { return !slices.Contains(list, value) })
		}
	}
	return values, found && len(values) != 0
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
	if err := applyRequireOne(a, l); err != nil {
		return err
	}
	if err := applyMode(a, l); err != nil {
		return err
	}
	if l.Description != nil {
		a.Description, a.PlainDescription, a.VerbatimDescription = *l.Description, true, true
	}
	if l.MarkdownDescription != nil {
		a.Description, a.PlainDescription, a.VerbatimDescription = *l.MarkdownDescription, false, true
	}
	a.DeprecationMessage = l.Deprecation
	a.Sensitive = l.Sensitive
	if l.Computed != nil {
		a.Computed = *l.Computed
	}
	if err := serverSetInRequest(a, l, file); err != nil {
		return err
	}
	if err := applyModifiers(a, l); err != nil {
		return err
	}
	if err := applyDefault(a, l); err != nil {
		return err
	}
	return applyValidators(a, l, file)
}

// applyValidators adds the validators of a line to the attribute. A oneOf that shows as one string
// (asString) gets one validator made from the enum and the pattern of the line.
func applyValidators(a *tfAttr, l overrides.Field, file *overrides.File) error {
	if l.AsString && a.FoldEnum == "" && len(a.FoldArms) == 0 {
		return errors.New("asString needs a field that holds a oneOf")
	}
	enumOnly := false
	for _, v := range l.Validators {
		if l.AsString && v.Enum {
			// The validator of a oneOf that shows as one string is made below.
			enumOnly = true
			continue
		}
		if rule, ok := siblingRuleOf(a, v); ok {
			a.SiblingRules = append(a.SiblingRules, rule)
			continue
		}
		expr, err := validatorExpr(a, v, file)
		if err != nil {
			return err
		}
		a.Validators = append(a.Validators, expr)
		if (v.Enum || v.EnumExceptZero) && (a.ValueKind == "List" || a.ValueKind == "Set") {
			// The acceptance test picks an element from the element validators, so they name the
			// Terraform values of the enums line, not the API values of the contract.
			oneOf, err := enumOneOf(a, file, v.EnumExceptZero)
			if err != nil {
				return err
			}
			a.ElemValidators = append(slices.DeleteFunc(a.ElemValidators, func(e string) bool { return strings.HasPrefix(e, "stringvalidator.OneOf") }), oneOf)
		}
	}
	if l.AsString && len(a.FoldArms) != 0 {
		// The names of the arms are the only values: there is no free text arm.
		oneOf := "stringvalidator.OneOf(" + strings.Join(quoteAll(a.FoldArms), ", ") + ")"
		a.Validators = append(a.Validators, oneOf)
		a.recordEnumValues(oneOf, slices.Clone(a.FoldArms))
		return nil
	}
	if l.AsString {
		return foldStringValidators(a, file, enumOnly)
	}
	return nil
}

// quoteAll quotes each text as a Go string literal.
func quoteAll(values []string) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		out = append(out, strconv.Quote(v))
	}
	return out
}

// serverSetInRequest makes an item field of an overwrite resource computed-only when the write
// requires the value that Get returns as read-only, such as the id that names the item to change. The
// line states it with computed: true and useStateForUnknown: Update sends the value that the state
// keeps. Create has no state, so a BeforeWrite hook of the resource fills the request value. The
// request still requires the field.
func serverSetInRequest(a *tfAttr, l overrides.Field, file *overrides.File) error {
	if l.Computed == nil || !*l.Computed || !a.Required || !a.ServerSetInRequest {
		return nil
	}
	if file.API.Overwrite == nil {
		return errors.New("computed: true on a field that the request requires and Get returns as read-only needs api.overwrite")
	}
	if !l.UseStateForUnknown {
		return errors.New("computed: true on a field that the request requires and Get returns as read-only needs useStateForUnknown, so Update sends the value that the state keeps")
	}
	a.Required, a.Optional = false, false
	return nil
}

// applyRequireOne makes the arms of the oneOf that a requireOne line names exactly one of. On an
// arm of a top-level oneOf, the resource validator becomes ExactlyOneOf. Elsewhere, the line names
// the field that holds the oneOf.
func applyRequireOne(a *tfAttr, l overrides.Field) error {
	if !l.RequireOne {
		return nil
	}
	if !a.RootArm && !slices.ContainsFunc(a.Attributes, func(c *tfAttr) bool { return c.OneOfGroup != "" }) {
		return errors.New("requireOne needs a field that holds a oneOf, or an arm of a top-level oneOf")
	}
	if !a.RootArm {
		requireOneArm(a)
	}
	return nil
}

// applyMode applies the keys of a line that make the attribute required, optional, or computed:
// required, schemaOptional, and requestPosition.
func applyMode(a *tfAttr, l overrides.Field) error {
	if l.Required {
		a.Required, a.Optional = true, false
	}
	if l.ClientID != "" {
		// The client makes the id, so the user may leave it out although the request requires it.
		if a.Kind != "String" {
			return fmt.Errorf("clientID: uuid needs a string attribute, not %s", a.Kind)
		}
		a.Required, a.Optional = false, true
	}
	if l.SchemaOptional {
		if !a.Required {
			return errors.New("schemaOptional needs a field that the API request requires")
		}
		a.Required, a.Optional = false, true
	}
	if !l.RequestPosition {
		return nil
	}
	if a.Kind != "Int64" && a.Kind != "Int32" {
		return fmt.Errorf("requestPosition needs an integer attribute, not %s", a.Kind)
	}
	if a.Computed && !a.Optional && !a.Required || slices.ContainsFunc(a.Modifiers, func(m string) bool { return strings.Contains(m, "RequiresReplace") }) {
		return errors.New("requestPosition needs a field that Create and Update send, not a read-only or immutable one")
	}
	// The position in the list is the value, so the user does not set it.
	a.Required, a.Optional, a.Computed, a.Validators = false, false, true, nil
	return nil
}

// applyModifiers adds the plan modifiers of a line: useStateForUnknown,
// useNonNullStateForUnknown, and equality.
func applyModifiers(a *tfAttr, l overrides.Field) error {
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
	if l.DefaultFromSibling != "" {
		if a.ValueKind != "String" || !a.Computed {
			return fmt.Errorf("defaultFromSibling needs a computed string attribute, not %s", a.ValueKind)
		}
		a.Modifiers = append(a.Modifiers, fmt.Sprintf("siblingDefaultModifier{Sibling: %q}", l.DefaultFromSibling))
	}
	if l.Equality != "" {
		if a.ValueKind != "String" {
			return fmt.Errorf("equality needs a string attribute, not %s", a.ValueKind)
		}
		// First, so that RequiresReplace and the other modifiers see the state value of an equal document.
		a.Modifiers = append([]string{"equivalentDocument{equal: " + l.Equality + "Equal}"}, a.Modifiers...)
	}
	return nil
}

// applyDefault sets the static default of a line.
func applyDefault(a *tfAttr, l overrides.Field) error {
	if l.Default == nil {
		return nil
	}
	a.DefaultRaw = l.Default
	if deferredDefault(a, l.Default) {
		a.StructDefault = l.Default
	} else {
		expr, err := defaultExpr(a, l.Default)
		if err != nil {
			return err
		}
		a.Default = expr
	}
	if a.Required {
		// A released default on a field that the contract requires means that the contract is
		// wrong about the field. The line states the exception with optional: true.
		if !l.Optional {
			return errors.New("the line gives a default to a field that the contract requires: add optional: true, so the overrides file states that the released attribute is optional")
		}
		a.Required, a.Optional = false, true
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
	case hasDefault(a) && !a.Computed:
		return errors.New("the line gives the attribute a default, but the attribute is not computed: add computed: true")
	case !a.Required && !a.Optional && !a.Computed:
		return errors.New("the line leaves the attribute without a mode: computed: false needs a field that the user can set, and the contract makes this field server-owned")
	}
	return nil
}

// intDefaultExpr is the default of an integer value, and false when a is no number attribute.
func intDefaultExpr(a *tfAttr, v int) (string, bool, error) {
	switch a.Kind {
	case "Int64":
		return fmt.Sprintf("int64default.StaticInt64(%d)", v), true, nil
	case "Int32":
		if v < math.MinInt32 || v > math.MaxInt32 {
			return "", true, fmt.Errorf("the default %d does not fit an Int32 attribute", v)
		}
		return fmt.Sprintf("int32default.StaticInt32(%d)", v), true, nil
	case "Float64", "Float32":
		expr, err := floatDefaultExpr(a.Kind, float64(v))
		return expr, true, err
	}
	return "", false, nil
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
		if expr, ok, err := intDefaultExpr(a, v); ok || err != nil {
			return expr, err
		}
	case float64:
		if a.Kind == "Float64" || a.Kind == "Float32" {
			return floatDefaultExpr(a.Kind, v)
		}
	case []any:
		if (a.Kind == "Set" || a.Kind == "List") && a.ElementType != "" {
			return fmt.Sprintf("%sdefault.StaticValue(types.%sValueMust(%s, []attr.Value{}))", strings.ToLower(a.Kind), a.Kind, a.ElementType), nil
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
		expr := "stringvalidator.OneOf(" + strings.Join(quoted, ", ") + ")"
		a.recordEnumValues(expr, slices.Clone(v.OneOf))
		return expr, nil
	case v.SizeAtLeast != nil && (a.ValueKind == "List" || a.ValueKind == "Set" || a.ValueKind == "Map"):
		return fmt.Sprintf("%s.SizeAtLeast(%d)", pkg, *v.SizeAtLeast), nil
	case v.LengthAtLeast != nil && a.ValueKind == "String":
		return fmt.Sprintf("stringvalidator.LengthAtLeast(%d)", *v.LengthAtLeast), nil
	case v.LengthAtMost != nil && a.ValueKind == "String":
		return fmt.Sprintf("stringvalidator.LengthAtMost(%d)", *v.LengthAtMost), nil
	case len(v.Between) == 2 && slices.Contains([]string{"Float32", "Float64", "Int32", "Int64"}, a.ValueKind):
		return betweenExpr(a, v.Between)
	case len(v.ExactlyOneOf) != 0:
		return pkg + ".ExactlyOneOf(" + siblingPaths(a.UpSteps, v.ExactlyOneOf) + ")", nil
	case len(v.AtLeastOneOf) != 0:
		return pkg + ".AtLeastOneOf(" + siblingPaths(a.UpSteps, v.AtLeastOneOf) + ")", nil
	case len(v.AlsoRequires) != 0:
		return pkg + ".AlsoRequires(" + siblingPaths(a.UpSteps, v.AlsoRequires) + ")", nil
	case len(v.ConflictsWith) != 0:
		return pkg + ".ConflictsWith(" + siblingPaths(a.UpSteps, v.ConflictsWith) + ")", nil
	case v.Unsupported != "" && a.ValueKind == "Bool":
		return fmt.Sprintf("%s{Message: %q}", unsupportedBoolValidatorName, v.Unsupported), nil
	case v.Equals != nil && a.ValueKind == "Bool":
		return fmt.Sprintf("boolvalidator.Equals(%t)", *v.Equals), nil
	case v.AtLeast != nil && (a.ValueKind == "Int32" || a.ValueKind == "Int64"):
		if err := checkIntegerBound(a.ValueKind, "atLeast", *v.AtLeast); err != nil {
			return "", err
		}
		return fmt.Sprintf("%s.AtLeast(%s)", pkg, numberLiteral(a.ValueKind, *v.AtLeast)), nil
	case v.AtLeast != nil && a.ValueKind == "Float64":
		return fmt.Sprintf("float64validator.AtLeast(%s)", formatFloat(*v.AtLeast)), nil
	case v.RegexMatches != nil && a.ValueKind == "String":
		return regexValidator(v.RegexMatches.Pattern, v.RegexMatches.Message), nil
	case v.KeysAre != nil && a.ValueKind == "Map":
		return "mapvalidator.KeysAre(" + regexValidator(v.KeysAre.Pattern, v.KeysAre.Message) + ")", nil
	case v.RegexMatches != nil && (a.ValueKind == "List" || a.ValueKind == "Set") && a.ElementType == "types.StringType":
		// Each element of a list or set of strings must match.
		return pkg + ".ValueStringsAre(" + regexValidator(v.RegexMatches.Pattern, v.RegexMatches.Message) + ")", nil
	}
	return "", fmt.Errorf("the validator does not fit a %s attribute", a.ValueKind)
}

// enumValidatorExpr is the validator that accepts the Terraform values of the enum of the attribute.
// The values are the zero value and the Terraform value of each accepted value, in sorted order.
// They come from the enums line, so the validator and the conversion maps cannot disagree.
// exceptZero leaves the zero value out of the validator. The conversion maps still contain it.
func enumValidatorExpr(a *tfAttr, file *overrides.File, exceptZero bool) (string, error) {
	oneOf, values, err := enumChoice(a, file, exceptZero)
	if err != nil {
		return "", err
	}
	if a.ValueKind == "List" || a.ValueKind == "Set" {
		expr := strings.ToLower(a.ValueKind) + "validator.ValueStringsAre(" + oneOf + ")"
		a.recordEnumValues(expr, values)
		return expr, nil
	}
	a.recordEnumValues(oneOf, values)
	return oneOf, nil
}

// recordEnumValues states the values that the validator with the expression expr accepts.
func (a *tfAttr) recordEnumValues(expr string, values []string) {
	if a.EnumValueSets == nil {
		a.EnumValueSets = map[string][]string{}
	}
	a.EnumValueSets[expr] = values
}

// enumOneOf is the OneOf validator of one value of the enum of the attribute: the validator of a
// scalar, and the validator of each element of a list or set.
func enumOneOf(a *tfAttr, file *overrides.File, exceptZero bool) (string, error) {
	expr, _, err := enumChoice(a, file, exceptZero)
	return expr, err
}

// enumChoice is the OneOf validator of one value of the enum of the attribute, and the values that
// it accepts: the Terraform values of the enums line, sorted, and the zero value unless exceptZero.
// The expression is made from the values, so the two cannot disagree.
func enumChoice(a *tfAttr, file *overrides.File, exceptZero bool) (string, []string, error) {
	enum, ok := file.Enums[a.EnumSchema]
	collection := a.ValueKind == "List" || a.ValueKind == "Set"
	if a.EnumSchema == "" || !ok || a.ValueKind != "String" && !collection {
		return "", nil, errors.New("the enum validator needs a field of an enum type, or a list or set of one, that has a line under enums")
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
	if enum.CaseInsensitive {
		return "stringvalidator.OneOfCaseInsensitive(" + strings.Join(quoted, ", ") + ")", values, nil
	}
	return "stringvalidator.OneOf(" + strings.Join(quoted, ", ") + ")", values, nil
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
		for _, modifier := range append(slices.Clone(attr.Modifiers), attr.ItemModifiers...) {
			if strings.Contains(modifier, pkg+".") && !containsString(packages, pkg) {
				packages = append(packages, pkg)
			}
			if strings.HasPrefix(modifier, "objectplanmodifier.") && !containsString(packages, "objectplanmodifier") {
				packages = append(packages, "objectplanmodifier")
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

// markOneOfArms marks each arm of the oneOf groups of an object, so that the validator on the parent
// object can check the group: exactly one arm, or at most one when no arm is allowed. The check
// is on the parent and reads its attributes. A validator on each arm that names its siblings by a
// path expression walks the whole configuration for every sibling, which made the plan of a
// dashboard with many widgets slow.
func markOneOfArms(attrs []*tfAttr, groups []model.OneOfGroup, name func(string) string) {
	byName := map[string]*tfAttr{}
	for _, a := range attrs {
		byName[a.Name] = a
	}
	for _, g := range groups {
		for _, arm := range g.Arms {
			a := byName[name(arm)]
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
	setDefaults  bool            // SetServerDefaults
	setItemState bool            // SetItemState
	file         *overrides.File
	policy       model.Policy      // names the attributes that a name line renames
	replace      bool              // Update is a full replace, so a group can hold nested fields
	owners       map[string]string // model struct name to what makes it: the resource, a component, or a group
	grouped      map[string]bool   // components whose types.<Type>.groups the builder applied
	places       map[string]bool   // the places of types.<Type>.at lines that the builder found
	itemModified map[string]bool   // components whose items carry types.<Type>.useStateForUnknown
	// carried are the siblings that a carrySiblings arm holds, with the component whose lines they
	// take. flags are the fold flags that an arm holds.
	carried map[*model.Field]string
	flags   map[*model.Field]overrides.Fold
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

// requireOneArm makes the group of a oneOf that allows no arm need one arm. The contract allows
// none, and the released resource required one.
func requireOneArm(a *tfAttr) {
	for _, child := range a.Attributes {
		child.OneOfRequired = true
	}
}

// attrPath is a Terraform attribute path: "root", then the names. The step
// anyListItem is any element of a list, and anyMapValue any value of a map.
type attrPath []string

func countSetItems(p attrPath) int {
	n := 0
	for _, step := range p {
		if step == anySetItem {
			n++
		}
	}
	return n
}

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
	if sep := splitOf(b.file, component, name); sep != "" {
		if t.Kind != model.String {
			return nil, fmt.Errorf("%s.%s: split needs a string, the field is a %s", component, name, t.Kind)
		}
		if utf8.RuneCountInString(sep) != 1 {
			return nil, fmt.Errorf("%s.%s: split needs a separator of one character, not %q", component, name, sep)
		}
		a.Kind, a.ValueKind, a.ElementType = "Set", "Set", "types.StringType"
		// An element that holds the separator would read back as two elements, and a set of one
		// empty element would read back empty, so neither round-trips. Like a oneOf group validator,
		// it states the encoding, not a limit of the contract, so it stays when the overrides drop the
		// inferred validators.
		separator := "setvalidator.ValueStringsAre(" +
			regexValidator("^[^"+regexp.QuoteMeta(sep)+"]+$", fmt.Sprintf("must not be empty or contain %q", sep)) + ")"
		a.Validators = append(a.Validators, separator)
		a.GroupValidators = append(a.GroupValidators, separator)
		return a, nil
	}
	if err := b.setType(a, p, t, computed); err != nil {
		return nil, err
	}
	return a, nil
}

// enumKeyFor returns the key of the enums entry of a field: the contract enum, or its variant when
// the line of the field has enumAs.
func enumKeyFor(file *overrides.File, component, property, schema string) string {
	if file == nil || schema == "" {
		return schema
	}
	if variant := file.Types[component].Fields[property].EnumAs; variant != "" {
		return schema + "@" + variant
	}
	return schema
}

func durationOf(file *overrides.File, component, name string) string {
	if file == nil {
		return ""
	}
	return file.Types[component].Fields[name].DurationFormat
}

// splitOf is the separator of the split line of field name of component: the API holds the values
// of a Terraform set of strings joined by it. "" when the field has none.
func splitOf(file *overrides.File, component, name string) string {
	if file == nil {
		return ""
	}
	return file.Types[component].Fields[name].Split
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
		a.Uint64 = t.Kind == model.Integer && t.WireString && (t.Format == "uint64" || t.Format == "int64")
		if t.Kind == model.Enum {
			a.EnumSchema = enumKeyFor(b.file, a.Component, a.Property, t.Schema)
		}
		if sentTime(t, computed) {
			a.CustomType = rfc3339Type
			a.Validators = append(a.Validators, utcValidator)
		}
	case model.Set, model.List, model.Map:
		if sep := joinOf(b.file, a.Component, a.Property, t); sep != "" {
			if t.Kind != model.List || t.Elem == nil || t.Elem.Kind != model.String || isTime(t.Elem) {
				return fmt.Errorf("%s.%s: join needs a list of plain strings", a.Component, a.Property)
			}
			a.Kind, a.ValueKind = "String", "String"
			return nil
		}
		return b.collection(a, p, t)
	case model.Object, model.OneOf:
		if line := b.file.TypedArmsOf(t.Schema); line != nil && line.AsList {
			return b.asListAttribute(a, p, t)
		}
		if asString(b.file, a.Component, a.Property) {
			if arms := emptyArms(t); arms != nil {
				a.Kind, a.ValueKind = "String", "String"
				for _, arm := range arms {
					a.FoldArms = append(a.FoldArms, terraformName(b.policy, t.Schema, arm.Name))
				}
				return nil
			}
			enum, _, err := stringArms(t)
			if err != nil {
				return fmt.Errorf("%s.%s: %w", a.Component, a.Property, err)
			}
			a.Kind, a.ValueKind, a.FoldEnum = "String", "String", enum.Type.Schema
			return nil
		}
		a.Kind, a.ValueKind = "SingleNested", "Object"
		a.ObjectComponent = t.Schema
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
	case model.Object, model.OneOf:
		a.Kind = kind + "Nested"
		a.ObjectComponent = t.Elem.Schema
		if b.file != nil && b.file.Types[t.Elem.Schema].UseStateForUnknown {
			a.ItemModifiers = []string{"objectplanmodifier.UseStateForUnknown()"}
			b.itemModified[t.Elem.Schema] = true
		}
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
// of int32: Int64. The validators follow the type. On a uint64 that the API sends as a string, the
// attribute is Int64 already: the line states that the released resource showed it as Int64, so a
// contract that allows more digits than Int64 holds is accepted, and flatten reports a larger value.
// String keeps an integer as decimal text.
func applyTerraformType(a *tfAttr, l overrides.Field) error {
	switch l.TerraformType {
	case "":
		return nil
	case "Float64":
		return applyFloat64Type(a)
	case "Number":
		return applyNumberType(a)
	}
	if l.TerraformType == "String" {
		// The released attribute is the decimal text of an integer, such as the id or a team id. An
		// int32 is shown this way only when the server sets it.
		if a.Kind != "Int32" && a.Kind != "Int64" {
			return fmt.Errorf("terraformType String needs an integer attribute, not a %s", a.Kind)
		}
		if a.Kind == "Int32" && (!a.Computed || a.Optional || a.Required) {
			return fmt.Errorf("terraformType String needs a computed-only integer attribute for an int32")
		}
		// A validator over other attributes has the same arguments for every type. A validator over the
		// value (AtLeast, OneOf, ...) has no string counterpart with the same meaning.
		for i, v := range a.Validators {
			rest, ok := strings.CutPrefix(v, "int64validator.")
			if !ok {
				continue
			}
			if !strings.HasPrefix(rest, "ConflictsWith(") && !strings.HasPrefix(rest, "ExactlyOneOf(") && !strings.HasPrefix(rest, "AlsoRequires(") && !strings.HasPrefix(rest, "AtLeastOneOf(") {
				return fmt.Errorf("terraformType String cannot keep the validator %s of the contract: remove the limit with validators.inferred: false, or state it as a validators line", v)
			}
			a.Validators[i] = "stringvalidator." + rest
		}
		prefix := strings.ToLower(a.Kind) + "planmodifier."
		for i, m := range a.Modifiers {
			a.Modifiers[i] = strings.Replace(m, prefix, "stringplanmodifier.", 1)
		}
		a.Kind, a.ValueKind = "String", "String"
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
	case a.Uint64:
		return nil
	case a.Kind == "Int32":
		a.Kind, a.ValueKind = "Int64", "Int64"
	case (a.Kind == "Set" || a.Kind == "List") && a.ElementType == "types.Int32Type":
		a.ElementType = "types.Int64Type"
		a.ElemValidators = retype(a.ElemValidators)
	default:
		return fmt.Errorf("terraformType Int64 needs an int32 attribute, a 64-bit number that the API sends as a string, or a list or set of int32, not a %s", a.Kind)
	}
	a.Validators, a.GroupValidators = retype(a.Validators), retype(a.GroupValidators)
	return nil
}

// applyFloat64Type keeps the released Float64 type of a float32 attribute. The validators and the plan
// modifiers follow the type.
func applyFloat64Type(a *tfAttr) error {
	if a.Kind != "Float32" {
		return fmt.Errorf("terraformType Float64 needs a float32 attribute, not a %s", a.Kind)
	}
	retype := strings.NewReplacer("float32validator.", "float64validator.", "float32planmodifier.", "float64planmodifier.")
	for _, list := range []*[]string{&a.Validators, &a.GroupValidators, &a.Modifiers} {
		for i, v := range *list {
			(*list)[i] = retype.Replace(v)
		}
	}
	a.Kind, a.ValueKind = "Float64", "Float64"
	return nil
}

// applyNumberType keeps the released Number type of an int32 or a double attribute. A validator over
// the value (AtLeast, Between, ...) has no Number counterpart, so only a validator over other
// attributes stays.
func applyNumberType(a *tfAttr) error {
	if a.Kind != "Int32" && a.Kind != "Float64" {
		return fmt.Errorf("terraformType Number needs an int32 or a double attribute, not a %s", a.Kind)
	}
	valueKind := strings.ToLower(a.Kind)
	for _, list := range []*[]string{&a.Validators, &a.GroupValidators} {
		for i, v := range *list {
			rest, ok := strings.CutPrefix(v, valueKind+"validator.")
			if !ok {
				continue
			}
			if !crossAttributeValidator(rest) {
				return fmt.Errorf("terraformType Number cannot keep the validator %s of the contract: remove the limit with validators.inferred: false", v)
			}
			(*list)[i] = "numbervalidator." + rest
		}
	}
	for i, m := range a.Modifiers {
		a.Modifiers[i] = strings.Replace(m, valueKind+"planmodifier.", "numberplanmodifier.", 1)
	}
	a.Kind, a.ValueKind = "Number", "Number"
	return nil
}

// crossAttributeValidator reports whether the call of a validator, such as ConflictsWith(...), is
// over other attributes. It has the same arguments for every type.
func crossAttributeValidator(call string) bool {
	for _, name := range []string{"ConflictsWith(", "ExactlyOneOf(", "AlsoRequires(", "AtLeastOneOf("} {
		if strings.HasPrefix(call, name) {
			return true
		}
	}
	return false
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
	if line := b.file.TypedArmsOf(t.Schema); line != nil && !line.AsList {
		if t.Kind != model.OneOf && t.Kind != model.Object {
			return nil, fmt.Errorf("%s: typedArms needs a oneOf or an object, not a %s", t.Schema, t.Kind)
		}
		return b.typedAttributes(p, t, line)
	}
	if carry := carryField(b.file, t); carry != nil {
		return b.carryAttributes(p, t, carry)
	}
	var attrs []*tfAttr
	var fields []tfModelField
	if err := checkPlaceFields(b.file, t); err != nil {
		return nil, err
	}
	groups := newNestedGroups(b, t.Schema, slices.Contains(p, anySetItem))
	for _, f := range t.Fields {
		if fold, ok := b.flags[f]; ok {
			// The flag of a folded arm: true selects this arm, false the folded one.
			attrs = append(attrs, &tfAttr{Name: fold.Flag, Kind: "Bool", ValueKind: "Bool", Optional: true, Computed: true,
				Default: "booldefault.StaticBool(true)", Description: fold.MarkdownDescription})
			fields = append(fields, tfModelField{Name: camelize(fold.Flag), Type: "types.Bool", TFName: fold.Flag})
			continue
		}
		component := t.Schema
		if owner, ok := b.carried[f]; ok {
			component = owner
		}
		if requestValue(b.file, component, f.Name) {
			continue
		}
		group := groups.of(f.Name)
		tfName := terraformName(b.policy, component, f.Name)
		if placed, ok := placeFieldName(b.file, t, f.Name); ok {
			tfName = placed // the component has another name for the field at this place
		}
		ftype := placedType(b.file, f.Type, component+"."+f.Name)
		if ftype != f.Type {
			b.markPlace(component + "." + f.Name)
		}
		child := append(append(attrPath{}, p...), tfName)
		if group != "" {
			child = append(append(attrPath{}, p...), group, tfName)
		}
		only := b.sent.responseOnly(component, f)
		a, err := b.attribute(child, component, f.Name, f.Description, ftype, nestedAttrs(f), only)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", f.Name, err)
		}
		a.Name = tfName
		if f.Attrs.Computed && !a.Required {
			// A computed line, or an attribute of a keyedBy object, which the API can fill.
			a.Computed = true
		}
		a.ServerSetInRequest = a.Required && f.Attrs.Required && f.Attrs.ReadOnly
		switch f.Behavior {
		case model.Computed:
			markComputed(a)
		case model.Immutable:
			b.requiresReplace(a, child.dotted())
		}
		if err := b.nestedServerDefault(a, child, component, f); err != nil {
			return nil, fmt.Errorf("%s: %w", f.Name, err)
		}
		field := b.modelField(component, f.Name, ftype, f.Behavior, only, f.Attrs.Computed)
		field.TFName = tfName
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
	distributed, distributedFields, err := b.distributedAttributes(p, t)
	if err != nil {
		return nil, err
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
	b.keptArmConflicts(t, attrs)
	attrs, fields = append(attrs, distributed...), append(fields, distributedFields...)
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

// keptArmConflicts makes the fields of an inlined oneOf arm conflict with the arms that stay beside
// them in the object t. The arms exclude each other in the API, and an inline line moved one arm's
// fields up. The check is a group check: it states the structure of the request, not a limit. The
// validator on the parent object makes it.
func (b *tfBuilder) keptArmConflicts(t *model.Type, attrs []*tfAttr) {
	for _, a := range attrs {
		parent := b.file.NestedParent(t.Schema, a.Property)
		if parent == nil || parent.Kept == "" {
			continue
		}
		var names []string
		for _, kept := range parent.KeptArms() {
			for _, k := range attrs {
				if k.Property == kept {
					names = append(names, k.Name)
				}
			}
		}
		if len(names) == 0 {
			continue
		}
		a.ArmConflicts = names
	}
}

// carryField returns the field of t whose line has carrySiblings, or nil.
func carryField(file *overrides.File, t *model.Type) *model.Field {
	if file == nil || t.Kind != model.Object {
		return nil
	}
	for _, f := range t.Fields {
		if file.Types[t.Schema].Fields[f.Name].CarrySiblings != nil {
			return f
		}
	}
	return nil
}

// checkCarryField rejects a carrySiblings line that is not the only one of t, or that is not on a
// oneOf that Create and Update send.
func (b *tfBuilder) checkCarryField(t *model.Type, carry *model.Field) error {
	for _, f := range t.Fields {
		if f != carry && b.file.Types[t.Schema].Fields[f.Name].CarrySiblings != nil {
			return fmt.Errorf("%s: carrySiblings on %s and %s: an object shows one oneOf as its arms", t.Schema, carry.Name, f.Name)
		}
	}
	if carry.Type.Kind != model.OneOf || len(t.Groups) != 0 {
		return fmt.Errorf("%s.%s: carrySiblings needs a oneOf field in an object without other oneOf groups", t.Schema, carry.Name)
	}
	if carry.Behavior != model.Normal {
		return fmt.Errorf("%s.%s: carrySiblings needs a oneOf that Create and Update send, not a %s one: every arm is an optional attribute", t.Schema, carry.Name, carry.Behavior)
	}
	return nil
}

// carryAttributes are the attributes of an object t whose field carry holds a oneOf with a
// carrySiblings line: one attribute per arm, which holds the other fields of t, the fields of the
// arm, and the flag of an arm that a fold folds into it.
func (b *tfBuilder) carryAttributes(p attrPath, t *model.Type, carry *model.Field) ([]*tfAttr, error) {
	line := b.file.Types[t.Schema].Fields[carry.Name]
	if err := b.checkCarryField(t, carry); err != nil {
		return nil, err
	}
	oneOf := carry.Type
	if b.carried == nil {
		b.carried, b.flags = map[*model.Field]string{}, map[*model.Field]overrides.Fold{}
	}
	armComponent := model.ComponentOf(oneOf)
	arms := map[string]*model.Field{}
	for _, arm := range oneOf.Fields {
		arms[arm.Name] = arm
	}
	if err := checkCarry(t, carry, line.CarrySiblings, arms); err != nil {
		return nil, err
	}
	var attrs []*tfAttr
	var fields []tfModelField
	group := model.OneOfGroup{AllowNone: oneOf.AllowNone && !line.RequireOne}
	for _, arm := range oneOf.Fields {
		if _, folded := line.CarrySiblings.Fold[arm.Name]; folded {
			continue
		}
		if arm.Type.Kind != model.Object || arm.Type.Schema == "" {
			return nil, fmt.Errorf("%s.%s: carrySiblings needs arms that are object components; %s is not", t.Schema, carry.Name, arm.Name)
		}
		group.Arms = append(group.Arms, arm.Name)
		a, err := b.carryArm(p, t, carry, arm, line.CarrySiblings)
		if err != nil {
			return nil, err
		}
		attrs = append(attrs, a)
		fields = append(fields, b.modelField(armComponent, arm.Name, arm.Type, model.Normal, false, false))
	}
	markOneOfArms(attrs, []model.OneOfGroup{group}, func(arm string) string { return terraformName(b.policy, armComponent, arm) })
	name := typeModelName(t)
	if err := b.claimModel(name, "the component "+t.Schema); err != nil {
		return nil, err
	}
	if !b.seen[name] {
		b.seen[name] = true
		b.models = append(b.models, &tfModel{Name: name, Fields: fields})
	}
	return attrs, nil
}

// carryArm is the attribute of one arm of a carrySiblings oneOf: the siblings of carry in t, the
// fields of the arm, and the flags of the folds into it.
func (b *tfBuilder) carryArm(p attrPath, t *model.Type, carry, arm *model.Field, c *overrides.Carry) (*tfAttr, error) {
	held := *arm.Type
	held.Fields = nil
	for _, sibling := range t.Fields {
		if sibling == carry || c.RequestValues[arm.Name][sibling.Name] != "" {
			continue
		}
		b.carried[sibling] = t.Schema
		held.Fields = append(held.Fields, sibling)
	}
	held.Fields = append(held.Fields, arm.Type.Fields...)
	for _, folded := range sortedFoldsInto(c, arm.Name) {
		flag := &model.Field{Name: folded.Flag, Type: &model.Type{Kind: model.Bool}, Behavior: model.Normal}
		b.flags[flag] = folded
		held.Fields = append(held.Fields, flag)
	}
	// The arm holds fields of two components and the fold flags, which the name checks of each
	// component do not see together.
	byTF, byGo := map[string]string{}, map[string]string{}
	for _, f := range held.Fields {
		owner := arm.Type.Schema
		if _, sibling := b.carried[f]; sibling && !slices.Contains(arm.Type.Fields, f) {
			owner = t.Schema
		}
		tf := terraformName(b.policy, owner, f.Name)
		if _, flag := b.flags[f]; flag {
			tf = f.Name
		}
		if other, ok := byTF[tf]; ok {
			return nil, fmt.Errorf("%s: the carried arm has two fields named %s in Terraform: %s and %s", arm.Name, tf, other, f.Name)
		}
		if other, ok := byGo[camelize(tf)]; ok {
			return nil, fmt.Errorf("%s: the carried arm has two fields named %s in Go: %s and %s", arm.Name, camelize(tf), other, f.Name)
		}
		byTF[tf], byGo[camelize(tf)] = f.Name, f.Name
	}
	component := model.ComponentOf(carry.Type)
	name := terraformName(b.policy, component, arm.Name)
	a := &tfAttr{Name: name, Kind: "SingleNested", ValueKind: "Object", Optional: true, Description: arm.Description, Component: component, Property: arm.Name}
	nested, err := b.objectAttributes(append(append(attrPath{}, p...), name), &held)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", arm.Name, err)
	}
	a.Attributes = nested
	return a, nil
}

// checkCarry reports a carrySiblings line that names an arm or a sibling that t does not have, a
// fixed value for the carried field itself or for a field that the requests do not send, and a
// fold into an arm with other fields or into an arm that another fold uses.
func checkCarry(t *model.Type, carry *model.Field, c *overrides.Carry, arms map[string]*model.Field) error {
	siblings := map[string]*model.Field{}
	for _, f := range t.Fields {
		if f != carry {
			siblings[f.Name] = f
		}
	}
	for _, arm := range slices.Sorted(maps.Keys(c.RequestValues)) {
		if arms[arm] == nil {
			return fmt.Errorf("%s: carrySiblings.requestValues names %s, which is no arm", t.Schema, arm)
		}
		for _, sibling := range slices.Sorted(maps.Keys(c.RequestValues[arm])) {
			switch f := siblings[sibling]; {
			case sibling == carry.Name:
				return fmt.Errorf("%s: carrySiblings.requestValues.%s names %s, the field that holds the oneOf", t.Schema, arm, sibling)
			case f == nil:
				return fmt.Errorf("%s: carrySiblings.requestValues.%s names %s, which is no field of %s", t.Schema, arm, sibling, t.Schema)
			case f.Behavior != model.Normal:
				return fmt.Errorf("%s: carrySiblings.requestValues.%s names %s, which Create and Update do not send", t.Schema, arm, sibling)
			}
		}
	}
	folds := map[string]string{}
	for _, arm := range slices.Sorted(maps.Keys(c.Fold)) {
		target := arms[c.Fold[arm].Into]
		if arms[arm] == nil || target == nil {
			return fmt.Errorf("%s: carrySiblings.fold.%s names an arm that the oneOf does not have", t.Schema, arm)
		}
		if other, ok := folds[target.Name]; ok {
			return fmt.Errorf("%s: carrySiblings.fold.%s and fold.%s both fold into %s: an arm takes one fold", t.Schema, other, arm, target.Name)
		}
		folds[target.Name] = arm
		if _, folded := c.Fold[target.Name]; folded {
			return fmt.Errorf("%s: carrySiblings.fold.%s folds into %s, which is folded itself", t.Schema, arm, target.Name)
		}
		if !sameFieldNames(arms[arm].Type, target.Type) {
			return fmt.Errorf("%s: carrySiblings.fold.%s: the arms %s and %s need the same fields", t.Schema, arm, arm, target.Name)
		}
		if !maps.Equal(c.RequestValues[arm], c.RequestValues[target.Name]) {
			return fmt.Errorf("%s: carrySiblings.fold.%s: the arms %s and %s need the same requestValues", t.Schema, arm, arm, target.Name)
		}
	}
	return nil
}

// sameFieldNames reports whether two objects have fields of the same names and types, in order: a
// fold copies each field of one SDK struct into the other.
func sameFieldNames(a, b *model.Type) bool {
	if a == nil || b == nil || len(a.Fields) != len(b.Fields) {
		return false
	}
	for i := range a.Fields {
		if a.Fields[i].Name != b.Fields[i].Name || !sameWireType(a.Fields[i].Type, b.Fields[i].Type) {
			return false
		}
	}
	return true
}

// sameWireType reports whether two field types have the same SDK type: the same kind, format,
// component, and element.
func sameWireType(a, b *model.Type) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Kind == b.Kind && a.Format == b.Format && a.Schema == b.Schema && sameWireType(a.Elem, b.Elem)
}

// sortedFoldsInto returns the folds into arm, by folded arm.
func sortedFoldsInto(c *overrides.Carry, arm string) []overrides.Fold {
	var out []overrides.Fold
	for _, folded := range slices.Sorted(maps.Keys(c.Fold)) {
		if c.Fold[folded].Into == arm {
			out = append(out, c.Fold[folded])
		}
	}
	return out
}

// nestedServerDefault makes a an Optional and Computed attribute with the server-default plan
// modifier, when f is a nested field of component that the requests may omit and whose server
// default the response states. A field whose line says how an omitted value behaves keeps the
// released behavior that the line states. A line that only adds a text or a validator keeps the
// server default, as on a top-level field.
func (b *tfBuilder) nestedServerDefault(a *tfAttr, p attrPath, component string, f *model.Field) error {
	if b.file != nil {
		if line, ok := b.file.Types[component].Fields[f.Name]; ok && line.KeepsReleasedOmission() {
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
	if sets := countSetItems(p); sets > 1 {
		// The modifier asks whether the configuration sets the set that holds the field, and can
		// address only the outermost set: the key of a set element is a planned value.
		return fmt.Errorf("%s: a nested server default inside a set within another set is unsupported", p.dotted())
	}
	if slices.Contains(p, anySetItem) && !f.Attrs.Required {
		// Flatten cannot match a set element that the API returns without the default to the
		// configured element, so it could not keep the configured default. A response that
		// requires the field always returns the stored value, so the element needs no match.
		return fmt.Errorf("%s: a nested server default inside a set is unsupported unless the response requires it", p.dotted())
	}
	value, err := serverDefaultValue(f.Type, d)
	if err != nil {
		return fmt.Errorf("server default: %w", err)
	}
	a.Computed = true
	modifier := "serverDefaultModifier{value: " + value + "}"
	if slices.Contains(p, anySetItem) {
		// The framework gives a plan modifier inside a set item the state of the item at the same
		// index, which can be another item. Terraform proposes the value of the prior item that it
		// paired with the configured one, so the modifier reads the plan instead.
		modifier = "serverDefaultModifier{value: " + value + ", fromPlan: true}"
		b.setDefaults = true
	}
	a.Modifiers = append(a.Modifiers, modifier)
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
		if first[i].TFName != other[i].TFName {
			return fmt.Sprintf("one place has the field %s where another has %s", first[i].TFName, other[i].TFName)
		}
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
		if hasDefault(attrs[i]) && isZeroDefault(file.Types[r.Name].Fields[f.Name].Default) && !f.Update.Presence {
			// Removing the value plans the zero, but a field without presence does not send it,
			// and with no mask the server keeps the old value.
			return fmt.Errorf("%s: updateMask: omit cannot clear this attribute: its default is the zero value, which Update does not send without x-coralogix-presence", name)
		}
		if attrs[i].Required || hasDefault(attrs[i]) || keepsAnyState(attrs[i]) {
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
		if splitOf(b.file, component, name) != "" {
			goType = "types.Set"
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
		if b.file != nil && b.file.Types[component].Fields[name].TerraformType == "String" {
			goType = "types.String" // the released String id
		}
	case model.Set:
		goType = "types.Set"
	case model.List:
		goType = "types.List"
		if joinOf(b.file, component, name, t) != "" {
			goType = "types.String" // a list of strings that Terraform shows as one string
		}
	case model.Map:
		goType = "types.Map"
	case model.Object, model.OneOf:
		if line := b.file.TypedArmsOf(t.Schema); line != nil && line.AsList {
			goType = "types.List"
		} else if shown := b.scalarOneOf(component, name); shown != "" {
			goType = shown
		} else if behavior == model.Computed || computed {
			goType = "types.Object"
		} else {
			goType = "*" + typeModelName(t)
		}
	}
	if b.file != nil {
		switch b.file.Types[component].Fields[name].TerraformType {
		case "String":
			goType = "types.String"
		case "Float64":
			goType = "types.Float64"
		case "Number":
			goType = "types.Number"
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
			// A signed number that the API sends as a string has no range known here.
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

// sentFields returns the component fields that a request sends in some place of r. A list that a
// distributeTo line moved out of the resource is sent as before.
func sentFields(r *model.Resource, file *overrides.File) sentSet {
	sent := sentSet{}
	for _, f := range r.Fields {
		sent.add(f.Type)
	}
	if file != nil {
		for _, d := range file.Distributed {
			sent.add(d.Field.Type)
		}
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
// its groups, when behavior-overrides groups hold it.
func rootPath(groups []string, name string) attrPath {
	return append(append(attrPath{"root"}, groups...), name)
}

// tfGroups builds the groups of the behavior-overrides file: required objects that hold
// top-level fields and that the API does not have.
type tfGroups struct {
	file     *overrides.File
	resource string
	attrs    map[string]*tfAttr
	models   map[string]*tfModel
}

// addToRootGroup puts the attribute and the model field of a top-level field in its group, or in the
// group inside it. A group is made on its first field: the outer group goes into the resource, and a group
// inside it into its outer group.
func (b *tfBuilder) addToRootGroup(out *tfResource, root *tfModel, g *tfGroups, path []string, a *tfAttr, field tfModelField) error {
	outer := path[0]
	line := g.file.Groups[outer]
	if g.attrs[outer] == nil {
		attr := &tfAttr{Name: outer, Kind: "SingleNested", ValueKind: "Object", Required: !line.Optional, Optional: line.Optional,
			Computed: line.Computed, Description: line.MarkdownDescription}
		if line.UseStateForUnknown {
			attr.Modifiers = []string{"objectplanmodifier.UseStateForUnknown()"}
		}
		g.attrs[outer] = attr
		g.models[outer] = &tfModel{Name: groupModelName(g.resource, outer)}
		if err := b.claimModel(g.models[outer].Name, "groups."+outer); err != nil {
			return err
		}
		out.Attributes = append(out.Attributes, attr)
		goType := "*" + g.models[outer].Name
		if line.Computed {
			// The plan of an omitted computed object is unknown, which a pointer cannot hold.
			goType = "types.Object"
		}
		root.Fields = append(root.Fields, tfModelField{Name: camelize(outer), Type: goType, TFName: outer})
		b.models = append(b.models, g.models[outer])
	}
	if len(path) == 1 {
		g.attrs[outer].Attributes = append(g.attrs[outer].Attributes, a)
		g.models[outer].Fields = append(g.models[outer].Fields, field)
		return nil
	}
	key := innerKey(path[0], path[1])
	if g.attrs[key] == nil {
		inner := line.Groups[path[1]]
		attr := &tfAttr{Name: path[1], Kind: "SingleNested", ValueKind: "Object", Optional: true, Description: inner.MarkdownDescription}
		g.attrs[key] = attr
		g.models[key] = &tfModel{Name: groupModelName(g.resource, outer+"_"+path[1])}
		if err := b.claimModel(g.models[key].Name, "groups."+outer+".groups."+path[1]); err != nil {
			return err
		}
		g.attrs[outer].Attributes = append(g.attrs[outer].Attributes, attr)
		g.models[outer].Fields = append(g.models[outer].Fields, tfModelField{Name: camelize(path[1]), Type: "*" + g.models[key].Name, TFName: path[1]})
		b.models = append(b.models, g.models[key])
	}
	g.attrs[key].Attributes = append(g.attrs[key].Attributes, a)
	g.models[key].Fields = append(g.models[key].Fields, field)
	return nil
}

// innerKey is the key of a group inside a group in the maps of the groups.
func innerKey(outer, inner string) string { return outer + "." + inner }

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

// check reports a group field that is no top-level field of the resource, a group whose Terraform
// or Go name is also the name of another top-level attribute, and a group inside a group that is
// not the one arm of a oneOf. It then shows each group with a typedArms line as its type attribute.
func (g *tfGroups) check(r *model.Resource, root []*tfAttr) error {
	for _, name := range g.file.GroupNames() {
		line := g.file.Groups[name]
		for _, a := range root {
			if a != g.attrs[name] && (a.Name == name || camelize(a.Name) == camelize(name)) {
				return fmt.Errorf("groups.%s: the name is also the top-level attribute %s", name, a.Name)
			}
		}
		for _, field := range line.Fields {
			if !slices.ContainsFunc(g.attrs[name].attrsOrNil(), func(a *tfAttr) bool { return a.Property == field }) {
				return fmt.Errorf("groups.%s: %s is no top-level field of %s that Terraform shows", name, field, g.resource)
			}
		}
		if err := g.checkInner(r, name); err != nil {
			return err
		}
		if line.RequireOne && !oneOfArmsOnly(g.attrs[name].attrsOrNil()) {
			// Flatten checks that the response has one of the fields of the group, so every
			// field must be an arm of the same oneOf.
			return fmt.Errorf("groups.%s: requireOne needs the arms of one oneOf of the resource, and no other field", name)
		}
	}
	return g.typedArms(r)
}

// typedArms replaces the attributes of each group with a typedArms line by its type attribute. The
// group holds every arm of one oneOf of the resource, and each arm is an object without fields, so
// the type names the arm and nothing else is left to show. A default makes the type optional and
// computed.
func (g *tfGroups) typedArms(r *model.Resource) error {
	for _, name := range g.file.GroupNames() {
		line := g.file.Groups[name].TypedArms
		if line == nil {
			continue
		}
		arms, err := typedGroupArms(r, name, g.file.Groups[name])
		if err != nil {
			return err
		}
		values, err := typedValues(&model.Type{Schema: r.Name}, line, arms, func(arm string) string { return terraformName(r.Policy, r.Name, arm) })
		if err != nil {
			return fmt.Errorf("groups.%s: %w", name, err)
		}
		quoted := make([]string, 0, len(arms))
		for _, arm := range arms {
			quoted = append(quoted, fmt.Sprintf("%q", values[arm.Name]))
		}
		typeName := line.TypeField()
		oneOf := "stringvalidator.OneOf(" + strings.Join(quoted, ", ") + ")"
		a := &tfAttr{Name: typeName, Kind: "String", ValueKind: "String", Required: true, Property: typeName,
			Validators: []string{oneOf}, GroupValidators: []string{oneOf},
			Description: "Which arm of the oneOf the object holds. One of " + strings.Join(quoted, ", ") + "."}
		if line.Default != "" {
			if !slices.Contains(slices.Collect(maps.Values(values)), line.Default) {
				return fmt.Errorf("groups.%s: typedArms.default %q is no value of the type: %s", name, line.Default, strings.Join(quoted, ", "))
			}
			a.Required, a.Optional, a.Computed = false, true, true
			a.Default = fmt.Sprintf("stringdefault.StaticString(%q)", line.Default)
			a.Description += fmt.Sprintf(" The default is %q.", line.Default)
		}
		g.attrs[name].Attributes = []*tfAttr{a}
		g.models[name].Fields = []tfModelField{{Name: camelize(typeName), Type: "types.String", TFName: typeName}}
	}
	return nil
}

// typedGroupArms returns the arms of the oneOf that the group holds, for a typedArms line: the group
// must hold every arm of one oneOf of the resource and no other field, and each arm is an object
// without fields.
func typedGroupArms(r *model.Resource, name string, line overrides.Group) ([]*model.Field, error) {
	for _, o := range r.Groups {
		if !slices.Contains(line.Fields, o.Arms[0]) {
			continue
		}
		if len(o.Arms) != len(line.Fields) || slices.ContainsFunc(line.Fields, func(f string) bool { return !slices.Contains(o.Arms, f) }) {
			break
		}
		var arms []*model.Field
		for _, arm := range o.Arms {
			i := slices.IndexFunc(r.Fields, func(f *model.ResourceField) bool { return f.Name == arm })
			if i < 0 || !emptyObject(r.Fields[i].Type) {
				return nil, fmt.Errorf("groups.%s: typedArms needs arms that are objects without fields, and %s is not one", name, arm)
			}
			arms = append(arms, &model.Field{Name: arm, Type: r.Fields[i].Type})
		}
		return arms, nil
	}
	return nil, fmt.Errorf("groups.%s: typedArms needs the arms of one oneOf of the resource, and no other field", name)
}

// checkInner checks the groups inside the group name. A group inside a group stands for one arm of a
// oneOf whose other arms are fields of the outer group, so it is optional, and it holds that arm only.
func (g *tfGroups) checkInner(r *model.Resource, name string) error {
	for _, inner := range sortedGroupNames(g.file.Groups[name].Groups) {
		line := g.file.Groups[name].Groups[inner]
		at := "groups." + name + ".groups." + inner
		for _, a := range g.attrs[name].attrsOrNil() {
			if a != g.attrs[innerKey(name, inner)] && (a.Name == inner || camelize(a.Name) == camelize(inner)) {
				return fmt.Errorf("%s: the name is also the attribute %s of %s", at, a.Name, name)
			}
		}
		for _, field := range line.Fields {
			if !slices.ContainsFunc(g.attrs[innerKey(name, inner)].attrsOrNil(), func(a *tfAttr) bool { return a.Property == field }) {
				return fmt.Errorf("%s: %s is no top-level field of %s that Terraform shows", at, field, g.resource)
			}
			if len(line.Fields) != 1 || !slices.ContainsFunc(r.Groups, func(o model.OneOfGroup) bool { return slices.Contains(o.Arms, field) }) {
				return fmt.Errorf("%s: a group inside a group holds one arm of a oneOf and no other field", at)
			}
		}
		if !line.Optional {
			return fmt.Errorf("%s: the group holds an arm of a oneOf, so the other arms can be set instead: add optional: true", at)
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
// A name with "../" in front names an attribute of an object further up: above lists the attributes
// of each level above the siblings, the nearest last.
func checkSiblingTargets(a *tfAttr, siblings []*tfAttr, above [][]*tfAttr, validators []overrides.Validator) error {
	for _, v := range validators {
		for _, target := range slices.Concat(v.ExactlyOneOf, v.AtLeastOneOf, v.AlsoRequires, v.ConflictsWith) {
			ups, name := splitTarget(target)
			level := siblings
			if ups > 0 {
				if ups > len(above) {
					return fmt.Errorf("validator target %q goes up %d levels, and the attribute has only %d above its object", target, ups, len(above))
				}
				level = above[len(above)-ups]
			}
			found := (ups > 0 || name != a.Name) && slices.ContainsFunc(level, func(s *tfAttr) bool { return s.Name == name })
			if !found {
				return fmt.Errorf("validator target %q is not another attribute of the same object", target)
			}
		}
	}
	return nil
}

// splitTarget splits a validator target into the levels that it goes up and the attribute name.
func splitTarget(target string) (ups int, name string) {
	name = target
	for strings.HasPrefix(name, "../") {
		ups++
		name = strings.TrimPrefix(name, "../")
	}
	return ups, name
}

// levelSteps is the number of path steps that the object held by a goes up: its name for a single
// object, and its name and the element for each item of a list, set, or map of objects.
func levelSteps(a *tfAttr) int {
	switch a.Kind {
	case "ListNested", "SetNested", "MapNested":
		return 2
	}
	return 1
}

// siblingPaths are the path expressions of the targets of a validator. A sibling is one AtParent
// away, and each "../" goes up one object level: steps holds the path steps of each level above
// the attribute, the nearest last (tfAttr.UpSteps), because an item of a list is two steps.
func siblingPaths(steps []int, targets []string) string {
	paths := make([]string, 0, len(targets))
	for _, target := range targets {
		ups, name := splitTarget(target)
		parents := 1
		for i := 0; i < ups && i < len(steps); i++ {
			parents += steps[len(steps)-1-i]
		}
		paths = append(paths, fmt.Sprintf("path.MatchRelative()%s.AtName(%q)", strings.Repeat(".AtParent()", parents), name))
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
		if splitArms(file, r.Name, f.Name) {
			if err := b.splitAttributes(out, root, r, f, file); err != nil {
				return fmt.Errorf("%s: %w", f.Name, err)
			}
			continue
		}
		path := file.GroupPath(f.Name)
		group := ""
		if len(path) != 0 {
			group = path[0]
		}
		if err := checkGroupField(r, f, group); err != nil {
			return err
		}
		a, err := b.rootFieldAttribute(out, r, f, path)
		if err != nil {
			return fmt.Errorf("%s: %w", f.Name, err)
		}
		field := b.modelField(r.Name, f.Name, f.Type, f.Behavior, responseOnly(f.Behavior), r.Policy.ComputedLine(r.Name, f.Name))
		if group == "" {
			out.Attributes = append(out.Attributes, a)
			root.Fields = append(root.Fields, field)
			continue
		}
		if err := b.addToRootGroup(out, root, groups, path, a, field); err != nil {
			return err
		}
	}
	return checkSplitNames(out, root, r, file)
}

// rootFieldAttribute builds the attribute of a top-level field in its groups. A group with a typedArms
// line shows its arms as a type attribute, so the model struct of an arm that nothing else shows is
// not needed.
func (b *tfBuilder) rootFieldAttribute(out *tfResource, r *model.Resource, f *model.ResourceField, path []string) (*tfAttr, error) {
	typedArm := len(path) != 0 && b.file.Groups[path[0]].TypedArms != nil && emptyObject(f.Type)
	owned := typedArm && !b.seen[typeModelName(f.Type)]
	a, err := b.resourceAttribute(out, r, f, path)
	if err == nil && owned {
		b.dropModel(typeModelName(f.Type))
	}
	return a, err
}

// dropModel removes the model struct with the name, which the builder made and nothing shows.
func (b *tfBuilder) dropModel(name string) {
	delete(b.seen, name)
	delete(b.owners, name)
	b.models = slices.DeleteFunc(b.models, func(m *tfModel) bool { return m.Name == name })
}

// checkSplitNames rejects a split arm with the Terraform or Go name of another top-level attribute,
// also of an arm of another split field. The collision checks of a component do not see a split
// arm next to the top-level fields.
func checkSplitNames(out *tfResource, root *tfModel, r *model.Resource, file *overrides.File) error {
	split := map[string]bool{}
	for _, f := range r.Fields {
		if splitArms(file, r.Name, f.Name) {
			component := model.ComponentOf(f.Type.Elem)
			for _, arm := range f.Type.Elem.Fields {
				split[terraformName(r.Policy, component, arm.Name)] = true
				split[camelize(arm.Name)] = true
			}
		}
	}
	attrs := map[string]bool{}
	for _, a := range out.Attributes {
		if attrs[a.Name] && split[a.Name] {
			return fmt.Errorf("the split arm %s has the name of another top-level attribute", a.Name)
		}
		attrs[a.Name] = true
	}
	fields := map[string]bool{}
	for _, f := range root.Fields {
		if fields[f.Name] && split[f.Name] {
			return fmt.Errorf("the split arm %s has the Go name of another top-level field", f.Name)
		}
		fields[f.Name] = true
	}
	return nil
}

// scalarOneOf is the model type of an object field that Terraform shows as one scalar (asBool,
// asString), or "".
func (b *tfBuilder) scalarOneOf(component, name string) string {
	switch {
	case b.policy.AsBool(component, name):
		return "types.Bool"
	case asString(b.file, component, name):
		return "types.String"
	}
	return ""
}

// asString reports whether the field has an asString line.
func asString(file *overrides.File, component, field string) bool {
	return file != nil && file.Types[component].Fields[field].AsString
}

// listArms returns the arm without fields and the arm with one list of strings of a oneOf that
// typedArms.asList shows as that list.
func listArms(t *model.Type) (all, list *model.Field, err error) {
	if t.Kind != model.OneOf || len(t.Fields) != 2 || len(t.Wrappers) != 0 {
		return nil, nil, errors.New("typedArms.asList needs a oneOf with two arms: an object without fields and an object with one list")
	}
	for _, arm := range t.Fields {
		switch {
		case emptyObject(arm.Type) && all == nil:
			all = arm
		case arm.Type.Kind == model.Object && len(arm.Type.Fields) == 1 && arm.Type.Fields[0].Type.Kind == model.List && list == nil:
			list = arm
		}
	}
	if all == nil || list == nil {
		return nil, nil, errors.New("typedArms.asList needs a oneOf with two arms: an object without fields and an object with one list; skip the other fields of the list arm")
	}
	return all, list, nil
}

// asListAttribute makes a the list of strings of a oneOf that typedArms.asList shows as that list.
// The type of the list comes from the list arm. The model struct of that arm is not needed.
func (b *tfBuilder) asListAttribute(a *tfAttr, p attrPath, t *model.Type) error {
	_, list, err := listArms(t)
	if err != nil {
		return fmt.Errorf("%s: %w", t.Schema, err)
	}
	owned := !b.seen[typeModelName(list.Type)]
	inner, err := b.objectAttributes(p, list.Type)
	if err != nil {
		return err
	}
	if owned {
		// Nothing else shows the list arm as an object, so its model struct would be unused.
		b.dropModel(typeModelName(list.Type))
	}
	if len(inner) != 1 || inner[0].Kind != "List" || inner[0].ElementType != "types.StringType" {
		return fmt.Errorf("%s: typedArms.asList needs a list arm with one list of strings", t.Schema)
	}
	a.Kind, a.ValueKind, a.ElementType = "List", "List", inner[0].ElementType
	// [] selects the arm without fields, and expand never sends the list arm an empty list, so a
	// minimum size of the list arm is no limit of the shown list. Its other limits stay.
	a.Validators = append(a.Validators, slices.DeleteFunc(slices.Clone(inner[0].Validators), func(v string) bool {
		return strings.HasPrefix(v, "listvalidator.SizeAtLeast(")
	})...)
	a.ElemValidators = inner[0].ElemValidators
	return nil
}

// emptyArms returns the arms of a oneOf whose arms are all objects without fields, which asString
// shows as one string that names the arm that is set, or nil.
func emptyArms(t *model.Type) []*model.Field {
	if t.Kind != model.OneOf || len(t.Fields) == 0 || len(t.Wrappers) != 0 {
		return nil
	}
	for _, arm := range t.Fields {
		if !emptyObject(arm.Type) {
			return nil
		}
	}
	return t.Fields
}

// stringArms returns the enum arm and the string arm of a oneOf that asString shows as one
// string. The oneOf has exactly these two arms, and the string arm is a plain string.
func stringArms(t *model.Type) (enum, str *model.Field, err error) {
	if t.Kind != model.OneOf || len(t.Fields) != 2 || len(t.Wrappers) != 0 {
		return nil, nil, errors.New("asString needs a oneOf with two arms: an enum and a string")
	}
	for _, arm := range t.Fields {
		switch {
		case arm.Type.Kind == model.Enum && enum == nil:
			enum = arm
		case arm.Type.Kind == model.String && arm.Type.Format == "" && str == nil:
			str = arm
		}
	}
	if enum == nil || str == nil {
		return nil, nil, errors.New("asString needs a oneOf with two arms: an enum and a string")
	}
	return enum, str, nil
}

// foldStringValidators makes the validator of an attribute that shows a oneOf of an enum and a
// string as one string. A regexMatches validator of the line checks the string arm: the value is a
// Terraform spelling of the enum, or matches one of them. With enum: true and no pattern, only the
// spellings are valid. With neither, the attribute takes any string.
func foldStringValidators(a *tfAttr, file *overrides.File, enumOnly bool) error {
	oneOf, values, err := enumChoice(&tfAttr{EnumSchema: a.FoldEnum, ValueKind: "String"}, file, false)
	if err != nil {
		return err
	}
	var regexes []string
	a.Validators = slices.DeleteFunc(a.Validators, func(v string) bool {
		if strings.HasPrefix(v, "stringvalidator.RegexMatches(") {
			regexes = append(regexes, v)
			return true
		}
		return false
	})
	switch {
	case len(regexes) != 0:
		a.Validators = append(a.Validators, "stringvalidator.Any("+strings.Join(append([]string{oneOf}, regexes...), ", ")+")")
	case enumOnly:
		a.Validators = append(a.Validators, oneOf)
		a.recordEnumValues(oneOf, values)
	}
	return nil
}

// splitArms reports whether the field has a splitArms line.
func splitArms(file *overrides.File, component, field string) bool {
	return file != nil && file.Types[component].Fields[field].SplitArms
}

// checkSplitField rejects a splitArms line on a field whose items the arm collections cannot hold.
func checkSplitField(r *model.Resource, f *model.ResourceField, file *overrides.File) error {
	t := f.Type
	if (t.Kind != model.List && t.Kind != model.Set) || t.Elem == nil || t.Elem.Kind != model.OneOf {
		return errors.New("splitArms needs a list or set whose items are a oneOf")
	}
	if f.Behavior != model.Normal || file.GroupOf(f.Name) != "" {
		return errors.New("splitArms needs a field that Create and Update send, outside a group")
	}
	// The arm collections hold no order across arms: a list [a1, b1, a2] would come back as
	// [a1, a2, b1].
	if t.Kind != model.Set {
		return errors.New("splitArms needs a set (collection: set): a list has an order across arms that the arm collections cannot keep")
	}
	// Flatten writes an empty collection for an arm without values, so a null configuration
	// needs the computed empty default.
	if line := file.Types[r.Name].Fields[f.Name]; line.Computed == nil || !*line.Computed {
		return errors.New("splitArms needs computed: true and default: []: an arm without values reads as an empty collection")
	}
	if file.Validators.Inferred != nil && *file.Validators.Inferred && (t.MinItems != nil || t.MaxItems != nil) {
		return errors.New("splitArms cannot keep the minItems or maxItems of the collection: the limit counts the items of every arm together")
	}
	return nil
}

// splitAttributes adds one attribute per arm of the oneOf items of a top-level list or set
// (splitArms). Each attribute is a collection of the kind of f, of the value of its arm.
func (b *tfBuilder) splitAttributes(out *tfResource, root *tfModel, r *model.Resource, f *model.ResourceField, file *overrides.File) error {
	t := f.Type
	if err := checkSplitField(r, f, file); err != nil {
		return err
	}
	component := model.ComponentOf(t.Elem)
	for _, arm := range t.Elem.Fields {
		if arm.Type.Kind != model.String && arm.Type.Kind != model.Enum {
			return fmt.Errorf("splitArms: the arm %s is %s, want a string or an enum (unwrap a wrapper object)", arm.Name, arm.Type.Kind)
		}
		armType := &model.Type{Kind: t.Kind, Elem: arm.Type}
		name := terraformName(b.policy, component, arm.Name)
		a, err := b.attribute(rootPath(nil, name), component, arm.Name, arm.Description, armType, model.Attrs{}, false)
		if err != nil {
			return err
		}
		// The line of the list gives every arm attribute its mode.
		line := file.Types[r.Name].Fields[f.Name]
		if line.Computed != nil {
			a.Computed = *line.Computed
		}
		if line.Default != nil {
			if overrides.IsNullDefault(line.Default) {
				return errors.New("splitArms needs default: []: an arm without values reads as an empty collection, not null")
			}
			if a.Default, err = defaultExpr(a, line.Default); err != nil {
				return err
			}
		}
		out.Attributes = append(out.Attributes, a)
		root.Fields = append(root.Fields, b.modelField(component, arm.Name, armType, model.Normal, false, b.policy.ComputedLine(component, arm.Name)))
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
			if file.Groups[group].TypedArms != nil {
				continue // the type attribute is the one arm
			}
			// The arms are in one group, which is an object: they check each other inside it. An arm
			// in a group inside the group is checked as that group, the object that stands for it.
			markOneOfArms(groups.attrs[group].Attributes, []model.OneOfGroup{g}, func(arm string) string {
				if inner := file.Groups[group].InnerOf(arm); inner != "" {
					return inner
				}
				return terraformName(r.Policy, r.Name, arm)
			})
			if file.Groups[group].RequireOne {
				requireOneArm(groups.attrs[group])
			}
			continue
		}
		var arms []string
		for _, arm := range g.Arms {
			name := terraformName(r.Policy, r.Name, arm)
			arms = append(arms, rootPath(file.GroupPath(arm), name).expr())
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
	inSet     bool            // the component is the item of a set
}

func newNestedGroups(b *tfBuilder, component string, inSet bool) *nestedGroups {
	g := &nestedGroups{b: b, component: component, inSet: inSet, attrs: map[string]*tfAttr{}, models: map[string]*tfModel{}, fresh: map[string]bool{}}
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

// checkItemModifiersUsed reports a types.<Type>.useStateForUnknown of a type that no list or set of
// the resource holds, such as a misspelled type name or a single object: the modifier would be dropped
// without a word.
func (b *tfBuilder) checkItemModifiersUsed() error {
	if b.file == nil {
		return nil
	}
	for _, name := range slices.Sorted(maps.Keys(b.file.Types)) {
		if b.file.Types[name].UseStateForUnknown && !b.itemModified[name] {
			return fmt.Errorf("types.%s.useStateForUnknown: no list or set of the resource holds objects of %s", name, name)
		}
	}
	return nil
}

// checkNestedGroupsUsed reports types.<Type>.groups of a type that no object of the resource
// has, such as a misspelled type name: the groups would be dropped without a word. It also checks the
// useStateForUnknown lines of types.
func (b *tfBuilder) checkNestedGroupsUsed() error {
	if b.file == nil {
		return nil
	}
	for _, name := range slices.Sorted(maps.Keys(b.file.Types)) {
		if len(b.file.Types[name].Groups) != 0 && !b.grouped[name] {
			return fmt.Errorf("types.%s.groups: no object of the resource is %s", name, name)
		}
	}
	return b.checkItemModifiersUsed()
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
		line := g.lines[group]
		attr = &tfAttr{Name: group, Kind: "SingleNested", ValueKind: "Object", Required: !line.Optional, Optional: line.Optional,
			Computed: line.Computed, Component: g.component, Description: line.MarkdownDescription}
		attr.Modifiers = g.stateModifiers(line)
		g.attrs[group] = attr
		g.models[group] = &tfModel{Name: groupModelName(g.component, group)}
	}
	attr.Attributes = append(attr.Attributes, a)
	g.models[group].Fields = append(g.models[group].Fields, field)
	return nil
}

// stateModifiers are the plan modifiers of a group that keeps its state. Inside a set item the
// framework gives the built-in modifier the state of the item at the same index, which can be another
// item, so the group takes the state of the item that the planned item matches.
func (g *nestedGroups) stateModifiers(line overrides.Group) []string {
	switch {
	case !line.UseStateForUnknown:
		return nil
	case g.inSet:
		g.b.setItemState = true
		return []string{"setItemStateModifier{}"}
	}
	return []string{"objectplanmodifier.UseStateForUnknown()"}
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
	goType := "*" + model.Name
	if g.lines[group].Computed {
		// The plan of an omitted computed object is unknown, which a pointer cannot hold.
		goType = "types.Object"
	}
	return g.attrs[group], tfModelField{Name: camelize(group), Type: goType, TFName: group}, true, nil
}

// addValidators adds the oneOf validators: inside a group when the group holds every arm, else to
// the attributes of the component. A group cannot hold some of the arms only.
func (g *nestedGroups) addValidators(attrs []*tfAttr, oneOfs []model.OneOfGroup) error {
	name := func(arm string) string { return terraformName(g.b.policy, g.component, arm) }
	for _, o := range oneOfs {
		group := g.of(o.Arms[0])
		split := false
		for _, arm := range o.Arms[1:] {
			split = split || g.of(arm) != group
		}
		if split {
			// The arms are in different places: each group holds one arm of the oneOf, as the
			// object that stands for the arm. The arms then check each other at the level of the
			// component, where the group objects are attributes.
			if err := g.checkSingleArmGroups(o); err != nil {
				return err
			}
			markOneOfArms(attrs, []model.OneOfGroup{o}, func(arm string) string {
				if in := g.of(arm); in != "" {
					return in
				}
				return name(arm)
			})
			continue
		}
		if group == "" {
			markOneOfArms(attrs, []model.OneOfGroup{o}, name)
			continue
		}
		markOneOfArms(g.attrs[group].Attributes, []model.OneOfGroup{o}, name)
		if g.lines[group].RequireOne {
			requireOneArm(g.attrs[group])
		}
	}
	return nil
}

// checkSingleArmGroups checks the groups of a oneOf whose arms are in different places: a group
// holds one arm of the oneOf and no other field, and it is optional, because the other arms are
// not in it. The group object stands for the arm.
func (g *nestedGroups) checkSingleArmGroups(o model.OneOfGroup) error {
	for _, arm := range o.Arms {
		group := g.of(arm)
		if group == "" {
			continue
		}
		line := g.lines[group]
		if len(line.Fields) != 1 {
			return fmt.Errorf("types.%s.groups.%s: the oneOf arms %s are in different places, so a group holds one arm and no other field", g.component, group, strings.Join(o.Arms, ", "))
		}
		if !line.Optional {
			return fmt.Errorf("types.%s.groups.%s: the group holds the arm %s of a oneOf, so the other arms can be set instead: add optional: true", g.component, group, arm)
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
