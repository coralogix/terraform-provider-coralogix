// Package overrides reads behavior-overrides.yaml. The file lists where a
// resource that users already have differs from the API contract. A new API
// field never needs a line.
//
// The reader is strict. An unknown key is an error. The generator also fails
// when a line matches nothing in the API contract, so a stale line cannot stay.
package overrides

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/coralogix/coralogix-management-sdk/iac/model"
	iacoverrides "github.com/coralogix/coralogix-management-sdk/iac/overrides"
	"go.yaml.in/yaml/v4"
)

// FileName is the name of the file. It sits in the output directory of the resource.
const FileName = "behavior-overrides.yaml"

// ModeExisting is the only mode. The file exists for a resource that users already have.
const ModeExisting = "existing"

// File is the content of behavior-overrides.yaml.
type File struct {
	// AfterFlatten gives a released resource a hook that can project API fields into
	// Terraform attributes after the generated conversion. The data source runs it too.
	AfterFlatten bool `yaml:"afterFlatten"`
	// ModifyPlan gives a released resource a hook that runs at the end of the plan, for a planned
	// value that no plan modifier of one attribute can state: one that reads another attribute, or
	// that changes values inside the items of a set.
	ModifyPlan bool `yaml:"modifyPlan"`
	// GetHooks gives a released resource two hooks around its Get call: Get replaces the call, to
	// add a query parameter or to retry it, and AfterGet receives the whole Get response, with the
	// values that sit next to the resource. The data source runs them too. A singleton has no
	// such hooks.
	GetHooks bool `yaml:"getHooks"`
	// EnumValuesInDescription appends the accepted Terraform values of an enum attribute to its
	// description: ` Valid values: ["A" "B"].`, as the released handwritten resources wrote them.
	// The list is the one of the validator. A description that a field line states, or that
	// already says "Valid values", stays as it is.
	EnumValuesInDescription bool `yaml:"enumValuesInDescription"`
	// Resource is the OpenAPI component of the resource. It must match the
	// generator's --resource flag, so a file cannot be used for the wrong resource.
	Resource string `yaml:"resource"`
	// Mode is "existing".
	Mode string `yaml:"mode"`
	// MarkdownDescription is the released description of the resource.
	MarkdownDescription string `yaml:"markdownDescription"`
	// Schema is the Terraform schema version and how old state is upgraded.
	Schema Schema `yaml:"schema"`
	// API describes the parts of the HTTP contract that differ from a new resource.
	API API `yaml:"api"`
	// Validators says which validators the generated resource has.
	Validators Validators `yaml:"validators"`
	// Types has one entry per OpenAPI component.
	Types map[string]Type `yaml:"types"`
	// Enums has one entry per OpenAPI enum component.
	Enums map[string]Enum `yaml:"enums"`
	// Unwrap lists the components with one property that the released resource shows as the
	// value of that property, for example LuceneQuery in {"luceneQuery": {"value": "error"}}. It
	// applies in every place that has no unwrap key on its field line.
	Unwrap []string `yaml:"unwrap"`
	// Inlined is what the generator derives from the upsert and inline lines. It is not a key of
	// the file.
	Inlined *Inlined `yaml:"-"`
	// TypeName is the released resource type name without the provider prefix, for example
	// "slo_v2" for coralogix_slo_v2. "" names the resource after its OpenAPI component.
	TypeName string `yaml:"typeName"`
	// SingletonID says where the id attribute of a singleton (Get has no path parameter) comes
	// from: fixed, the released fixed value, or fromAPI, the id field that only the server sets.
	// Without the key, the id is the resource type name.
	SingletonID *SingletonID `yaml:"singletonID"`
	// Groups are required Terraform objects that the API does not have. Each holds top-level
	// fields of the resource, for example sli = {request_based_metric_sli = ...} for the
	// requestBasedMetricSli field of the API.
	Groups map[string]Group `yaml:"groups"`
	// UpdateMask: "omit" sends no updateMask in the Update body (api.updateMaskInBody), as a
	// released resource that never sent one did. The server then updates every field present in
	// the body, so removing an optional attribute from the configuration cannot clear it. The key
	// also states that the server replaces the whole value of each field in the body, nested
	// objects included: a removed nested attribute is cleared. A server that merged nested
	// objects would keep it, and the generator cannot check that.
	UpdateMask string `yaml:"updateMask"`
	// DataSource is the data source of the resource. Without it, a released resource gets no
	// generated data source.
	DataSource *DataSource `yaml:"dataSource"`
}

// UpdateMaskOmit is the updateMask value that sends no update mask.
const UpdateMaskOmit = "omit"

// API is the api section of the file: the API keys of the iac module.
type API struct {
	iacoverrides.API `yaml:",inline"`
}

// OperationNone is the api.operations.delete value of an API that cannot remove the resource.
const OperationNone = "none"

// Check checks the API keys. api.operations with delete: none is a singleton that one call sets,
// for example GET and POST on /logs/data-setup/v2: create and update name that call, and no other
// key names an operation or the id.
func (a API) Check() error {
	if err := a.API.Check(); err != nil {
		return err
	}
	o := a.Operations
	if o == nil || o.Delete != OperationNone {
		return nil
	}
	if o.Create != o.Update {
		return errors.New("api.operations.delete: none is for a singleton that one call sets: create and update must name the same operation")
	}
	if a.RequestWrapper != "" || a.UpdateIDInBody || a.ClientSetID || a.ServerSetID || a.IDProperty != "" || a.CustomMethods || a.UpdateMaskInBody {
		return errors.New("api.operations with delete: none is a singleton that one call sets: it cannot have requestWrapper, updateIDInBody, clientSetID, serverSetID, idProperty, customMethods, or updateMaskInBody")
	}
	return nil
}

// Inlined is, for each inlined top-level field, the API object that holds it in the Create body,
// the Update body, and the resource response. The generator reads the contract with each inlined
// object replaced by its fields, so the model sees top-level fields; the renderer reads and writes
// them through the object.
type Inlined struct {
	Create, Update, Get map[string]Parent
	// Nested is, for each component other than the resource whose object field is inlined, the
	// object that holds each moved field: component, then field. A request component that holds the
	// same object under the same name is inlined too, so it has its own entry.
	Nested map[string]map[string]Parent
}

// Parent is the object field of the API that holds an inlined top-level field.
type Parent struct {
	Field  string // API property of the object, for example "s3"
	Schema string // component of the object
	// Arm: the object is an arm of a oneOf group of the resource response, and the file skips the
	// other arms. When Get returns no such object, the resource does not exist.
	Arm bool
}

// Upserts reports whether the resource is a singleton that one call sets: api.operations with
// delete: none.
func (f *File) Upserts() bool {
	return f != nil && f.API.Operations != nil && f.API.Operations.Delete == OperationNone
}

// InlinedFields returns the fields of the resource that inline lines name, in order.
func (f *File) InlinedFields() []string {
	if f == nil {
		return nil
	}
	var out []string
	for _, field := range sortedKeys(f.Types[f.Resource].Fields) {
		if f.Types[f.Resource].Fields[field].Inline {
			out = append(out, field)
		}
	}
	return out
}

// NestedInlined returns the inlined fields of each component other than the resource, in order.
func (f *File) NestedInlined() map[string][]string {
	if f == nil {
		return nil
	}
	out := map[string][]string{}
	for _, name := range sortedKeys(f.Types) {
		if name == f.Resource {
			continue
		}
		for _, field := range sortedKeys(f.Types[name].Fields) {
			if f.Types[name].Fields[field].Inline {
				out[name] = append(out[name], field)
			}
		}
	}
	return out
}

// NestedParent returns the object that holds field of the component schema, when an inline line
// moved it there, or nil.
func (f *File) NestedParent(schema, field string) *Parent {
	if f == nil || f.Inlined == nil {
		return nil
	}
	parent, ok := f.Inlined.Nested[schema][field]
	if !ok {
		return nil
	}
	return &parent
}

// ParentOf returns the object that holds field in root ("create", "update", or "get"), or nil.
func (f *File) ParentOf(root, field string) *Parent {
	if f == nil || f.Inlined == nil {
		return nil
	}
	m := map[string]map[string]Parent{"create": f.Inlined.Create, "update": f.Inlined.Update, "get": f.Inlined.Get}[root]
	parent, ok := m[field]
	if !ok {
		return nil
	}
	return &parent
}

// SingletonID is where the id attribute of a singleton comes from. Exactly one key is set.
type SingletonID struct {
	// Fixed is the released fixed id, for example "archive-logs-settings".
	Fixed string `yaml:"fixed"`
	// FromAPI: the API resource has an id field that only the server sets. The id attribute then
	// holds that value, and Create, Get, Update, and Delete still take no id. Mark the field
	// readOnly. The id must be a string.
	FromAPI bool `yaml:"fromAPI"`
}

// IDFromAPI reports whether the id attribute of the singleton is the id that the server sets.
func (f *File) IDFromAPI() bool { return f != nil && f.SingletonID != nil && f.SingletonID.FromAPI }

// FixedID returns the released fixed id of the singleton, or "" for the resource type name.
func (f *File) FixedID() string {
	if f == nil || f.SingletonID == nil {
		return ""
	}
	return f.SingletonID.Fixed
}

// Group is a required Terraform object that holds top-level fields of the resource.
type Group struct {
	// Fields are the API names of the top-level fields in the object.
	Fields []string `yaml:"fields"`
	// MarkdownDescription is the released description of the object.
	MarkdownDescription string `yaml:"markdownDescription"`
	// RequireOne: the fields are the arms of a oneOf of the resource, and the user must set one.
	// The arms then check each other with ExactlyOneOf instead of ConflictsWith.
	RequireOne bool `yaml:"requireOne"`
	// Optional makes the object optional, for a released attribute that the user may leave out and
	// that the API has as plain fields. Expand sends none of its fields when it is null, and
	// flatten reads it as null when the response has none of them, so every field must be one that
	// the SDK holds by pointer, or a list or a map. The fields inside stay as their lines say.
	Optional bool `yaml:"optional"`
	// Computed, with optional, makes the object Optional and Computed: the plan is unknown when
	// the configuration leaves it out, and the model holds it as a types.Object.
	Computed bool `yaml:"computed"`
	// UseStateForUnknown, with computed, keeps the state value of the object when the
	// configuration leaves it out.
	UseStateForUnknown bool `yaml:"useStateForUnknown"`
}

// check checks the keys of a group that change its mode.
func (g Group) check() error {
	switch {
	case g.Computed && !g.Optional:
		return errors.New("computed needs optional: true: a required object cannot be computed")
	case g.UseStateForUnknown && !g.Computed:
		return errors.New("useStateForUnknown needs computed: true: only a computed object is unknown in a plan")
	case g.Optional && g.RequireOne:
		return errors.New("requireOne needs an object that the user sets, so it cannot be optional")
	}
	return nil
}

var tfNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// GroupOf returns the group that holds the top-level field, or "".
func (f *File) GroupOf(field string) string {
	if f == nil {
		return ""
	}
	for name, g := range f.Groups {
		if slices.Contains(g.Fields, field) {
			return name
		}
	}
	return ""
}

// GroupNames returns the group names in order.
func (f *File) GroupNames() []string {
	if f == nil {
		return nil
	}
	return sortedKeys(f.Groups)
}

func (f *File) checkGroups() error {
	if f.TypeName != "" && !tfNamePattern.MatchString(f.TypeName) {
		return fmt.Errorf("typeName %q is not a Terraform name: lower case letters, digits, and underscores", f.TypeName)
	}
	if err := checkGroupMap("groups", f.Groups); err != nil {
		return err
	}
	for _, name := range sortedKeys(f.Groups) {
		if g := f.Groups[name]; g.Optional || g.Computed || g.UseStateForUnknown {
			return fmt.Errorf("groups.%s: optional, computed, and useStateForUnknown are keys of types.<Type>.groups, not of a group of the resource", name)
		}
	}
	for _, name := range sortedKeys(f.Types) {
		if err := checkGroupMap("types."+name+".groups", f.Types[name].Groups); err != nil {
			return err
		}
	}
	return nil
}

// checkGroupMap checks the groups at prefix: Terraform names, and each field in one group.
func checkGroupMap(prefix string, groups map[string]Group) error {
	seen := map[string]string{}
	for _, name := range sortedKeys(groups) {
		g := groups[name]
		if !tfNamePattern.MatchString(name) {
			return fmt.Errorf("%s.%s: the name is not a Terraform name: lower case letters, digits, and underscores", prefix, name)
		}
		if len(g.Fields) == 0 {
			return fmt.Errorf("%s.%s: fields is empty", prefix, name)
		}
		if err := g.check(); err != nil {
			return fmt.Errorf("%s.%s: %w", prefix, name, err)
		}
		for _, field := range g.Fields {
			if other, ok := seen[field]; ok {
				return fmt.Errorf("%s.%s: field %s is also in %s.%s", prefix, name, field, prefix, other)
			}
			seen[field] = name
		}
	}
	return nil
}

// Schema is the Terraform schema version of a released resource.
type Schema struct {
	// Version is the current schema version.
	Version int64 `yaml:"version"`
	// Upgrade has one entry per older version that users can still have in their state.
	Upgrade map[int64]Upgrade `yaml:"upgrade"`
}

// Upgrade is how state of an older schema version becomes current.
type Upgrade struct {
	// PriorSchema is the Go function that returns the frozen schema of that version:
	// the import path of its package, a dot, and the function name, for example
	// "github.com/acme/provider/internal/thing/schema.V0".
	PriorSchema string `yaml:"priorSchema"`
	// Strategy is "refresh": the upgrader reads the resource from the API, so one
	// upgrader serves every attribute that changed shape.
	Strategy string `yaml:"strategy"`
}

// PriorSchemaFunc splits PriorSchema into the import path and the function name.
func (u Upgrade) PriorSchemaFunc() (importPath, function string, ok bool) {
	i := strings.LastIndex(u.PriorSchema, ".")
	if i <= 0 || i == len(u.PriorSchema)-1 || !strings.Contains(u.PriorSchema[:i], "/") {
		return "", "", false
	}
	return u.PriorSchema[:i], u.PriorSchema[i+1:], true
}

// Validators is the validator policy of the resource.
type Validators struct {
	// Inferred: false means that no limit of the contract (minLength, maxLength,
	// minItems, ...) becomes a validator. A released provider did not check these
	// limits, and a new validator would reject configs that work today. The validators
	// of the released resource are written as field lines. true keeps the limits of the
	// contract as validators: for a resource without a released version, or one whose
	// released validators are exactly the limits of the contract. The key is required.
	Inferred *bool `yaml:"inferred"`
	// RejectUnknown: true stops Create and Update before the API call when a request value is
	// still unknown, as a new resource does. A released resource left an unknown computed
	// value out of the request, so only a resource without a released version sets it.
	RejectUnknown bool `yaml:"rejectUnknown"`
}

// Type overrides one OpenAPI object component.
type Type struct {
	// Required: [] says that the object declares no required list and means
	// "no field is required". Only the empty list is allowed.
	Required *[]string `yaml:"required"`
	// Fields overrides single fields of the object.
	Fields map[string]Field `yaml:"fields"`
	// ExtraAttributes are Terraform attributes that the API contract does not
	// have. The generator adds them to the schema and the model of this type.
	// Expand does not send them. Flatten writes a typed null. A handwritten
	// overlay reads them from configuration.
	ExtraAttributes map[string]ExtraAttribute `yaml:"extraAttributes"`
	// Groups are required Terraform objects that the API does not have, as the top-level groups,
	// each holding fields of this object, for example frequency = {daily = ...} for the daily
	// field of a recurring schedule.
	Groups map[string]Group `yaml:"groups"`
	// UnsupportedArms are oneOf arms of this object that the released resource does not
	// configure. They stay out of the schema, and flatten reports a response that selects one
	// with UnsupportedSummary and UnsupportedDetail. The three keys are set together.
	UnsupportedArms    []string `yaml:"unsupportedArms"`
	UnsupportedSummary string   `yaml:"unsupportedSummary"`
	UnsupportedDetail  string   `yaml:"unsupportedDetail"`
	// RequestWrapper is the property of the request objects of this nested component that holds
	// its fields, for a request item {"thing": {"name": ...}, "rules": ...} of a read item
	// {"name": ..., "rules": ...}. The requestAs lines of its fields are relative to it.
	RequestWrapper string `yaml:"requestWrapper"`
	// TypedArms shows this oneOf object as a type attribute plus the merged fields of its arms.
	TypedArms *TypedArms `yaml:"typedArms"`
	// UseStateForUnknown puts objectplanmodifier.UseStateForUnknown() on the object of each list
	// or set that holds this component, for a released schema that has it there. A modifier on
	// the item, unlike one on an attribute inside it, is not an attribute of the item.
	UseStateForUnknown bool `yaml:"useStateForUnknown"`
}

// isEmpty reports whether the type line sets nothing.
func (t Type) isEmpty() bool {
	return t.Required == nil && len(t.Fields) == 0 && len(t.ExtraAttributes) == 0 && len(t.Groups) == 0 && len(t.UnsupportedArms) == 0 &&
		t.UnsupportedSummary == "" && t.UnsupportedDetail == "" && t.RequestWrapper == "" && !t.UseStateForUnknown && t.TypedArms == nil
}

// TypedArms shows a oneOf object as one object. A required string attribute (Field) names the arm.
// The other attributes are the fields of all arms, merged by name, and all optional. The API
// spelling `aggregation: {percentile: {field: duration, percent: 95}}` is
// `{type = "percentile", field = "duration", percent = 95}` in Terraform.
type TypedArms struct {
	// Field is the Terraform name of the attribute that names the arm. The default is "type".
	Field string `yaml:"field"`
	// Names maps an arm (its API name) to the value of Field, for a released spelling that is not
	// the Terraform name of the arm, such as `average: avg`. Every arm has a value.
	Names map[string]string `yaml:"names"`
	// Value is the Terraform name of the attribute that holds an arm that is a scalar, such as a
	// string or an enum. The default is "value". Two scalar arms share it, so they have one type.
	Value string `yaml:"value"`
	// Infer makes the type attribute optional and computed, for a released resource whose users
	// leave it out. The plan then picks the first arm that has a field the configuration sets (a
	// list counts as set when it is not empty), and else the arm without fields. For a oneOf of
	// `all` (no fields) and `list` (values), no values mean `all`, and some values mean `list`.
	Infer bool `yaml:"infer"`
}

// check checks the names of a typedArms line and that no two arms have one value.
func (t *TypedArms) check() error {
	for _, name := range []struct{ key, value string }{{"field", t.Field}, {"value", t.Value}} {
		if name.value != "" && !tfNamePattern.MatchString(name.value) {
			return fmt.Errorf("typedArms.%s %q must be lower case letters, digits, and _", name.key, name.value)
		}
	}
	if t.TypeField() == t.ValueField() {
		return fmt.Errorf("typedArms: field and value are both %q", t.TypeField())
	}
	seen := map[string]string{}
	for _, arm := range sortedKeys(t.Names) {
		value := t.Names[arm]
		if value == "" {
			return fmt.Errorf("typedArms.names.%s is empty", arm)
		}
		if other, ok := seen[value]; ok {
			return fmt.Errorf("typedArms.names: %s and %s are both %q", other, arm, value)
		}
		seen[value] = arm
	}
	return nil
}

// TypeField returns the Terraform name of the attribute that names the arm.
func (t *TypedArms) TypeField() string {
	if t.Field == "" {
		return "type"
	}
	return t.Field
}

// ValueField returns the Terraform name of the attribute that holds a scalar arm.
func (t *TypedArms) ValueField() string {
	if t.Value == "" {
		return "value"
	}
	return t.Value
}

// GroupOf returns the group of the type that holds field, or "".
func (t Type) GroupOf(field string) string {
	for name, g := range t.Groups {
		if slices.Contains(g.Fields, field) {
			return name
		}
	}
	return ""
}

// ExtraAttribute is one Terraform-only attribute of a generated object.
type ExtraAttribute struct {
	// Type is "string", "int64", or "bool" for a scalar, "set" or "list" for a collection of strings,
	// or "object" for an object with the attributes below. Without it, the attribute is a map. A
	// string of the resource that is not read-only or writeOnly keeps the value of the plan or the
	// prior state, as a dynamic one does, for a text that a handwritten hook turns into request
	// values, such as a document of the whole resource. So does an object of the resource.
	Type string `yaml:"type"`
	// ElementType is the map or set value type. A scalar or dynamic attribute has none.
	ElementType string `yaml:"elementType"`
	// Dynamic: true makes the attribute a DynamicAttribute (types.Dynamic) in place of a map,
	// for a value of any Terraform type that a handwritten hook turns into request values.
	Dynamic bool `yaml:"dynamic"`
	// Required: true makes a Terraform-only attribute Required.
	Required bool `yaml:"required"`
	// Computed makes a Terraform-only attribute read-only. A handwritten AfterFlatten hook
	// can set it from an API value of another type or at another path.
	Computed bool `yaml:"computed"`
	// InUpdate: true says that a handwritten BeforeWrite hook sends this attribute, so a change
	// of it makes an Update. Without it, a change of only this attribute plans an update that
	// sends no request. It needs a resource whose Update replaces the resource (PUT).
	InUpdate bool `yaml:"inUpdate"`
	// WriteOnly: Terraform sends the value and does not store it.
	WriteOnly bool `yaml:"writeOnly"`
	// MarkdownDescription is the docs text of the attribute.
	MarkdownDescription string `yaml:"markdownDescription"`
	// Validators keep released checks on Terraform-only attributes.
	Validators []Validator `yaml:"validators"`
	// ElementValues limits the strings of a Terraform-only set.
	ElementValues []string `yaml:"elementValues"`
	// UseStateForUnknown keeps a computed value during a plan.
	UseStateForUnknown bool `yaml:"useStateForUnknown"`
	// Optional: true, with computed: true, makes the attribute Optional and Computed, for a value
	// that the user may set and that a handwritten hook fills in otherwise. Without computed, an
	// extra attribute is optional already.
	Optional bool `yaml:"optional"`
	// Default is the static default of an Optional and Computed string, bool, or int64.
	Default any `yaml:"default"`
	// Equality is "json" or "yaml" for a string: the plan keeps the state value when the configured
	// document is equal to it, as the equality line of a field does.
	Equality string `yaml:"equality"`
	// Attributes are the attributes of an object (type: object), by Terraform name. They take the
	// keys of an extra attribute, except dynamic, writeOnly, and inUpdate, and they are optional
	// unless they say required: true.
	Attributes map[string]ExtraAttribute `yaml:"attributes"`
}

// Field overrides one field of an object. A field with a line keeps the released
// behavior that the line states. A field without a line follows the contract.
type Field struct {
	// Field has the keys that describe the API: skip, readOnly, optional, createAs, collection,
	// keyedBy, and unwrap.
	iacoverrides.Field `yaml:",inline"`
	// ValueNames shows a list or set of integers as the names of its values, for example the
	// weekdays 1 to 7 as Sunday to Saturday. Terraform holds strings; the API holds the numbers.
	ValueNames map[int64]string `yaml:"valueNames"`
	// Inline: true shows the fields of this object field of the resource as top-level
	// attributes, as the released resource did. The API sends and returns them inside the object.
	// The lines of those fields are lines of the resource component. With required: true, the
	// line says that the API always sends and returns the object, when the contract has no
	// required list that can say so; the line then has no attribute to make required.
	Inline bool `yaml:"inline"`
	// AsBool shows an object without fields as a bool, for example a oneOf arm alwaysActive: {}.
	// true sends the empty object, false is an error, and a response that has the object reads
	// as true; one without it reads as null.
	AsBool bool `yaml:"asBool"`
	// AsString, on a field that holds a oneOf of an enum and a string, shows the oneOf as one
	// string: a Terraform spelling of the enum selects the enum arm, and any other value is the
	// string arm. Flatten reads the arm that the response has, so a released resource that took
	// "5_MINUTES" or "1h30m" in one attribute keeps it. A regexMatches validator of the line is
	// the check of the string arm: the validator of the attribute accepts a spelling of the enum
	// or a match of the pattern. enum: true alone accepts only the spellings of the enum, and a
	// line with neither accepts any string.
	AsString bool `yaml:"asString"`
	// TerraformType keeps the released Terraform type of an int32 field, or of a list or set of
	// int32: Int64. Expand converts the value and reports one outside the int32 range. On a
	// digit string (^[0-9]+$ or ^-?[0-9]+$) it reads the 64-bit number as an Int64. String keeps a
	// computed integer as decimal text, including a released int64 resource id: the state holds
	// the decimal text, and the calls parse it.
	TerraformType string `yaml:"terraformType"`
	// StoredAs: float32 on a double field says that the API stores the value as a float32 and
	// returns the float64 of that float32 (0.3 reads back as 0.30000001192092896). Flatten reads
	// the shortest decimal of the float32, so 0.3 reads back as 0.3.
	StoredAs string `yaml:"storedAs"`
	// DefaultArm: when the configuration sets no arm of the oneOf group of this arm, Create and
	// Update send this arm empty. The group then allows no arm in Terraform, for an API that
	// requires an arm and reads an empty one as "all", for example alertUniqueIds: {}. The arm
	// must be a list of strings in wrapper objects with readEmptyAs: "null".
	DefaultArm bool `yaml:"defaultArm"`
	// Description and MarkdownDescription are the released texts of the docs. An empty
	// string means that the released field had no description.
	Description         *string `yaml:"description"`
	MarkdownDescription *string `yaml:"markdownDescription"`
	// Required: true makes the Terraform attribute Required, although the contract does not
	// require the field. The SDK type of the field does not change. Delete this key when the
	// contract requires the field.
	Required bool `yaml:"required"`
	// SchemaOptional keeps a released Terraform attribute optional while the API request
	// remains required. An omitted value is sent as the SDK zero value.
	SchemaOptional bool `yaml:"schemaOptional"`
	// Name is the Terraform attribute name of the field, when the released resource does not
	// use the name derived from the JSON name (applications for applicationnameFilters).
	Name string `yaml:"name"`
	// Deprecation is the released deprecation message.
	Deprecation string `yaml:"deprecation"`
	// Sensitive marks the Terraform attribute sensitive, as the released attribute was: Terraform
	// hides its value in plans and output. It is a Terraform fact that the contract does not state.
	Sensitive bool `yaml:"sensitive"`
	// Computed: true makes an optional field Optional+Computed. false makes it Optional.
	Computed *bool `yaml:"computed"`
	// UseStateForUnknown keeps the value in the state when the config has none.
	UseStateForUnknown bool `yaml:"useStateForUnknown"`
	// UseNonNullStateForUnknown keeps a non-null value of the state when the config has none.
	// A null state value stays unknown, so the server can fill it in.
	UseNonNullStateForUnknown bool `yaml:"useNonNullStateForUnknown"`
	// Default is a static default in the schema: a string, a bool, a number, [] (an empty list or
	// set), null (NullDefault: a list, set, map, or object that plans null when omitted), or a map
	// ({} or {child: value, ...}: an object whose attributes get these values, the others null).
	Default any `yaml:"default"`
	// ReadEmptyAs: "null" reads an empty list or object from the API as null.
	ReadEmptyAs string `yaml:"readEmptyAs"`
	// ReadNullAs: "empty" reads a missing string as "", and a missing list or set of objects as an
	// empty collection.
	ReadNullAs string `yaml:"readNullAs"`
	// CarrySiblings, on an object field that holds a oneOf, shows the object as the arms of the
	// oneOf: each arm attribute holds the other fields of the object (the siblings) next to the
	// fields of the arm, as a released resource that showed rule = {parse = {name, source_field,
	// regular_expression}} for a rule with a name, a source field, and parameters did. The lines
	// of the object give the siblings their behavior, in every arm.
	CarrySiblings *Carry `yaml:"carrySiblings"`
	// RequestPosition makes an integer field of a list item computed-only, and Create and Update
	// send the 1-based position of the item in its list, as a released resource that computed the
	// order of each rule did. The read shows the value of the API.
	RequestPosition bool `yaml:"requestPosition"`
	// ShowInverted shows a bool field as its opposite, as a released resource that showed
	// keep_source_field for the API flag deleteSource did. Expand sends the inverted value, and
	// flatten reads it inverted. A default on the line is a Terraform value. It differs from the
	// iac key negate, which sends the opposite of a read field under another request name.
	ShowInverted bool `yaml:"showInverted"`
	// SplitArms shows a top-level list or set whose items are each one arm of a oneOf as one
	// attribute per arm, holding the values of that arm, as a released resource that showed
	// applications, subsystems, and severities for a list of rule matchers did. Each arm must be a
	// string or an enum, also in a wrapper object (unwrap). computed and default on this line
	// apply to every arm attribute, because an arm of a oneOf has no lifecycle of its own. The
	// lines of the arms name the attributes and give their texts and validators. Create and
	// Update send one item per value, the arms in the order of the contract.
	SplitArms bool `yaml:"splitArms"`
	// SendUnknownAsZero sends the zero value of a scalar when its planned value is unknown, as a
	// released resource that converted the plan with ValueStringPointer and the like did. The API
	// stores that zero value (false, 0, ""), not the value that it gives an omitted field. It
	// needs computed: true, because only a computed attribute is unknown in a plan.
	SendUnknownAsZero bool `yaml:"sendUnknownAsZero"`
	// SendNullAsEmpty sends "" for a null string, for an API that keeps the stored value when a
	// request omits the field. Removing the value from the configuration then clears it. It needs
	// readEmptyAs: "null", so that the stored "" reads back as null.
	SendNullAsEmpty bool `yaml:"sendNullAsEmpty"`
	// RequireOne makes a oneOf that allows no arm use ExactlyOneOf. The released
	// resource required one arm even though the contract allows none.
	RequireOne bool `yaml:"requireOne"`
	// RequestValue is an API value that Create and Update always send. The field
	// is not a Terraform attribute.
	RequestValue string `yaml:"requestValue"`
	// RequestValueBy maps an object field of the same component to the API value that Create
	// and Update send when that field is set, for a released resource that derives the value
	// (type: logsQuery → E2M_TYPE_LOGS2METRICS). The field is not a Terraform attribute.
	RequestValueBy map[string]string `yaml:"requestValueBy"`
	// KeepPriorOrder returns the list items in the order of the plan or state, because
	// the API does not keep the order.
	KeepPriorOrder bool `yaml:"keepPriorOrder"`
	// Validators are the released validators of the field.
	Validators []Validator `yaml:"validators"`
	// Equality: "yaml" or "json" compares a string field as a YAML or JSON document. The API
	// returns the document normalized, so a change of format alone must not plan a change.
	Equality string `yaml:"equality"`
	// RequiresReplace: a change of this top-level field replaces the resource, although Update
	// accepts it, as the released resource did. A PATCH Update does not send the field. A PUT
	// Update still sends it, so an update of another field does not clear it.
	RequiresReplace bool `yaml:"requiresReplace"`
	// UniqueBy names a property of the items of a top-level list or set of objects (entityType)
	// that must differ between the items. The generated resource reports a duplicate before the
	// request, for an API that rejects it.
	UniqueBy string `yaml:"uniqueBy"`
	// Split shows a string that the API holds as values joined by this separator, "a,b", as a set
	// of strings, as the released resource did. Expand joins the values; flatten splits the text.
	Split string `yaml:"split"`
}

// Carry is a carrySiblings line.
type Carry struct {
	// RequestValues leave a sibling out of an arm, and Create and Update send the API value in
	// that arm instead: arm → sibling → value, for a released arm that had no source field and
	// always sent "text".
	RequestValues map[string]map[string]string `yaml:"requestValues"`
	// Fold shows an arm as another arm with the same fields, chosen by a bool: folded arm →
	// fold. The flag is true for the arm Into, its default, and false for the folded arm.
	Fold map[string]Fold `yaml:"fold"`
}

// Fold is one arm of a carrySiblings oneOf that Terraform shows as the arm Into with Flag false.
type Fold struct {
	Into                string `yaml:"into"`
	Flag                string `yaml:"flag"`
	MarkdownDescription string `yaml:"markdownDescription"`
}

// check reports a fold without its arm, flag, or text, and a request value without a value.
func (c *Carry) check() error {
	for _, arm := range sortedKeys(c.Fold) {
		f := c.Fold[arm]
		if f.Into == "" || f.Flag == "" || f.MarkdownDescription == "" || f.Into == arm {
			return fmt.Errorf("carrySiblings.fold.%s needs into (another arm), flag, and markdownDescription", arm)
		}
		if !validAttrName.MatchString(f.Flag) {
			return fmt.Errorf("carrySiblings.fold.%s: flag %q is not a Terraform attribute name", arm, f.Flag)
		}
		if _, folded := c.Fold[f.Into]; folded {
			return fmt.Errorf("carrySiblings.fold.%s: the arm %s is folded itself", arm, f.Into)
		}
	}
	for _, arm := range sortedKeys(c.RequestValues) {
		for _, sibling := range sortedKeys(c.RequestValues[arm]) {
			if c.RequestValues[arm][sibling] == "" {
				return fmt.Errorf("carrySiblings.requestValues.%s.%s is empty", arm, sibling)
			}
		}
	}
	return nil
}

// Equality values of a field line.
const (
	EqualityYAML = "yaml"
	EqualityJSON = "json"
)

// Validator is one released validator. Exactly one key is set.
type Validator struct {
	OneOf       []string `yaml:"oneOf"`
	SizeAtLeast *int     `yaml:"sizeAtLeast"`
	// LengthAtLeast and LengthAtMost are a released stringvalidator.LengthAtLeast and
	// stringvalidator.LengthAtMost.
	LengthAtLeast *int `yaml:"lengthAtLeast"`
	LengthAtMost  *int `yaml:"lengthAtMost"`
	// Enum: true accepts the Terraform values of the enum of the field, as the enums line of
	// the enum states them, including the zero value. The list of values has one source, so it
	// cannot drift.
	Enum bool `yaml:"enum"`
	// EnumExceptZero accepts the same values as Enum, except the zero value. The conversion
	// maps still contain the zero value, so a response that has it still converts. A user
	// cannot configure it.
	EnumExceptZero bool `yaml:"enumExceptZero"`
	// Between is a released numeric Between validator: the minimum and the maximum.
	Between []float64 `yaml:"between"`
	// ExactlyOneOf and AtLeastOneOf name sibling attributes, by their Terraform names, for a
	// released ExactlyOneOf or AtLeastOneOf validator.
	ExactlyOneOf  []string `yaml:"exactlyOneOf"`
	AtLeastOneOf  []string `yaml:"atLeastOneOf"`
	AlsoRequires  []string `yaml:"alsoRequires"`
	ConflictsWith []string `yaml:"conflictsWith"`
	// AtLeast is a released AtLeast validator of a number.
	AtLeast *float64 `yaml:"atLeast"`
	// RegexMatches is a released stringvalidator.RegexMatches with its message.
	RegexMatches *RegexMatches `yaml:"regexMatches"`
	// KeysAre is a released mapvalidator.KeysAre(stringvalidator.RegexMatches) of a map: each key
	// must match the pattern.
	KeysAre *RegexMatches `yaml:"keysAre"`
}

// RegexMatches is a released pattern validator of a string.
type RegexMatches struct {
	Pattern string `yaml:"pattern"`
	Message string `yaml:"message"`
}

// RejectedTFValue is the Terraform value that a rejected value reads back as: its API spelling,
// or the lower case with readRejected: lower.
func (e Enum) RejectedTFValue(v string) string {
	if e.ReadRejected == ReadRejectedLower {
		return strings.ToLower(v)
	}
	return v
}

// checkUnsupported checks the unsupported arm keys: the three keys are set together, no arm is
// listed twice, and no unsupported arm has a field line, because the arm is not in the schema.
func (t Type) checkUnsupported() error {
	if len(t.UnsupportedArms) == 0 && t.UnsupportedSummary == "" && t.UnsupportedDetail == "" {
		return nil
	}
	if len(t.UnsupportedArms) == 0 || t.UnsupportedSummary == "" || t.UnsupportedDetail == "" {
		return errors.New("unsupportedArms, unsupportedSummary, and unsupportedDetail are set together")
	}
	for i, arm := range t.UnsupportedArms {
		if arm == "" || slices.Contains(t.UnsupportedArms[:i], arm) {
			return fmt.Errorf("unsupportedArms names %q twice or empty", arm)
		}
		if _, ok := t.Fields[arm]; ok {
			return fmt.Errorf("unsupported arm %q also has a field line, which changes nothing", arm)
		}
	}
	return nil
}

// TFValue is the Terraform value of the API value v of the enum: its names entry, or the value
// without trimPrefix, in lower case unless the line is verbatim.
// checkAliases checks that each alias is an accepted Terraform value and has another accepted value
// as its target. The zero value is no target: the plan would replace a value with the API's
// unspecified one.
func (e Enum) checkAliases(name string) error {
	accepted := map[string]bool{}
	targets := map[string]bool{}
	if e.Zero != "" {
		accepted[e.Zero] = true
	}
	for _, v := range e.Values {
		accepted[e.TFValue(v)] = true
		targets[e.TFValue(v)] = true
	}
	for _, alias := range sortedKeys(e.Aliases) {
		target := e.Aliases[alias]
		switch {
		case !accepted[alias]:
			return fmt.Errorf("enums.%s.aliases: %q is not the zero value or a value of the enum, so no config can set it", name, alias)
		case !targets[target]:
			return fmt.Errorf("enums.%s.aliases: the target %q of %q is not a value of the enum", name, target, alias)
		case target == alias:
			return fmt.Errorf("enums.%s.aliases: %q maps to itself", name, alias)
		case e.Aliases[target] != "":
			return fmt.Errorf("enums.%s.aliases: the target %q of %q is an alias itself", name, target, alias)
		}
	}
	return nil
}

func (e Enum) TFValue(v string) string {
	if tf, ok := e.Names[v]; ok {
		return tf
	}
	return e.defaultTFValue(v)
}

// defaultTFValue is the Terraform value of v without its names entry.
func (e Enum) defaultTFValue(v string) string {
	v = strings.TrimPrefix(v, e.TrimPrefix)
	if e.Verbatim {
		return v
	}
	return strings.ToLower(v)
}

// Enum overrides one enum component.
type Enum struct {
	// Zero is the Terraform value of the protobuf zero value (ENTITY_TYPE_UNSPECIFIED).
	// A new resource uses null for it. A released resource kept a value in its state.
	Zero string `yaml:"zero"`
	// ZeroNull: the released resource reads the zero value as null and does not accept it, as a new
	// resource does. It excludes Zero.
	ZeroNull bool `yaml:"zeroNull"`
	// Values are the enum values that the resource accepts. The Terraform value is the
	// lower case of the API value, unless Verbatim is set.
	Values []string `yaml:"values"`
	// Verbatim keeps the API spelling as the Terraform value. Attachment policies stay
	// AUTO, ENABLED, and DISABLED.
	Verbatim bool `yaml:"verbatim"`
	// TrimPrefix is removed from each value before the Terraform value is made, for example
	// SLO_TIME_FRAME_ for SLO_TIME_FRAME_7_DAYS, which is 7_days in Terraform.
	TrimPrefix string `yaml:"trimPrefix"`
	// Names gives an accepted API value its Terraform value, for a released resource that
	// spells it in another way (SEVERITY_DEBUG is Debug). It overrides the lower case and
	// Verbatim.
	Names map[string]string `yaml:"names"`
	// Aliases maps a Terraform value that the resource accepts to the Terraform value that the
	// plan replaces it with, for an alias that the API stores as another value (unspecified is
	// percentage). The alias is the zero value or one of the values, and its target is one of
	// the values.
	Aliases map[string]string `yaml:"aliases"`
	// CaseInsensitive accepts a Terraform value in any case, as a released resource whose
	// validator was OneOfCaseInsensitive did. Expand matches the value in any case, and flatten
	// keeps the spelling of the plan or the state when the API value differs only in case. A
	// scalar field of the enum only: a list or set has no prior value to keep per element.
	CaseInsensitive bool `yaml:"caseInsensitive"`
	// Rejected are the values of the contract that the resource does not accept. The generator
	// reports a value of the contract that is in neither list, so a new API value needs a decision.
	Rejected []string `yaml:"rejected"`
	// ReadRejected: a rejected value still reads back, as the API spelling (true) or in lower
	// case (lower, as a released resource that lowercased an unknown API value did). The write
	// map and the schema validator do not accept it. Without this key, a rejected value fails
	// the read.
	ReadRejected ReadRejected `yaml:"readRejected"`
}

// ReadRejected is how a rejected enum value reads back: not at all (""), as the API spelling
// (ReadRejectedAPI, from true), or in lower case (ReadRejectedLower, from lower).
type ReadRejected string

const (
	ReadRejectedAPI   ReadRejected = "api"
	ReadRejectedLower ReadRejected = "lower"
)

// On reports whether a rejected value reads back.
func (r ReadRejected) On() bool { return r != "" }

// UnmarshalYAML accepts true, false, or lower.
func (r *ReadRejected) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind == yaml.ScalarNode {
		switch n.Value {
		case "true":
			*r = ReadRejectedAPI
			return nil
		case "false":
			*r = ""
			return nil
		case "lower":
			*r = ReadRejectedLower
			return nil
		}
	}
	return fmt.Errorf("line %d: readRejected is %q, want true, false, or lower", n.Line, n.Value)
}

// Parse reads the file. An unknown key or a wrong value is an error.
func Parse(data []byte) (*File, error) {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	var f File
	if err := dec.Decode(&f); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("%s is empty", FileName)
		}
		return nil, fmt.Errorf("%s: %w", FileName, err)
	}
	if err := markNullDefaults(data, &f); err != nil {
		return nil, fmt.Errorf("%s: %w", FileName, err)
	}
	if err := f.check(); err != nil {
		return nil, fmt.Errorf("%s: %w", FileName, err)
	}
	return &f, nil
}

func (f *File) check() error {
	if f.Resource == "" {
		return errors.New("resource is required")
	}
	if f.Mode != ModeExisting {
		return fmt.Errorf("mode is %q, want %q", f.Mode, ModeExisting)
	}
	if err := f.Schema.checkUpgrades(); err != nil {
		return err
	}
	if f.Validators.Inferred == nil {
		return errors.New("validators.inferred is required: false for a released resource, which does not turn contract limits into validators, and true for a resource without a released version")
	}
	if err := f.checkEnums(); err != nil {
		return err
	}
	if err := iacoverrides.CheckUnwrap(f.Unwrap); err != nil {
		return err
	}
	if err := f.checkGroups(); err != nil {
		return err
	}
	if err := f.API.Check(); err != nil {
		return err
	}
	if f.UpdateMask != "" && f.UpdateMask != UpdateMaskOmit {
		return fmt.Errorf("updateMask is %q, want %q", f.UpdateMask, UpdateMaskOmit)
	}
	if f.UpdateMask != "" && !f.API.UpdateMaskInBody {
		return errors.New("updateMask: omit needs api.updateMaskInBody: only a body mask can be left out")
	}
	if s := f.SingletonID; s != nil && (s.Fixed == "") == !s.FromAPI {
		return errors.New("singletonID needs exactly one of fixed (the released fixed id) or fromAPI: true (the id that the server sets)")
	}
	if f.SingletonID != nil && (f.API.ClientSetID || f.API.ServerSetID || f.API.UpdateIDInBody) {
		return errors.New("singletonID is for a singleton, which has no id in a path: it cannot be combined with api.clientSetID, api.serverSetID, or api.updateIDInBody")
	}
	if f.DataSource != nil && f.DataSource.QueryID && (f.SingletonID == nil || !f.SingletonID.FromAPI) {
		return errors.New("dataSource.queryID needs singletonID: {fromAPI: true}: the query id must be distinct from the singleton's server-set resource id")
	}
	return f.checkSections()
}

// checkSections checks the dataSource section and the types section.
func (f *File) checkSections() error {
	if err := f.DataSource.check(); err != nil {
		return err
	}
	if err := f.checkTypes(); err != nil {
		return err
	}
	return nil
}

// checkTypes checks the type lines: the required list, the field lines, and the extra attributes.
func (f *File) checkTypes() error {
	for name, t := range f.Types {
		if t.Required != nil && len(*t.Required) != 0 {
			return fmt.Errorf("types.%s.required: only the empty list [] is allowed", name)
		}
		if t.isEmpty() {
			return fmt.Errorf("types.%s has no override", name)
		}
		if err := t.checkUnsupported(); err != nil {
			return fmt.Errorf("types.%s: %w", name, err)
		}
		if t.TypedArms != nil {
			if err := t.TypedArms.check(); err != nil {
				return fmt.Errorf("types.%s.%w", name, err)
			}
		}
		if t.RequestWrapper != "" && name == f.Resource {
			return fmt.Errorf("types.%s.requestWrapper: the resource component uses api.requestWrapper", name)
		}
		names := map[string]string{}
		for _, field := range sortedKeys(t.Fields) {
			line := t.Fields[field]
			if err := line.check(); err != nil {
				return fmt.Errorf("types.%s.fields.%s: %w", name, field, err)
			}
			if line.Split != "" && (line.Collection != "" || line.KeyedBy != nil) {
				return fmt.Errorf("types.%s.fields.%s: split shows a string as a set, so collection and keyedBy cannot be set", name, field)
			}
			if len(line.CreateAs) != 0 && name != f.Resource {
				// Only the Create body of the resource has top-level fields to fold.
				return fmt.Errorf("types.%s.fields.%s: createAs works on a field of the resource component %s only", name, field, f.Resource)
			}
			if line.Name == "" {
				continue
			}
			if other, ok := names[line.Name]; ok {
				return fmt.Errorf("types.%s.fields.%s: name %q is also the name of %s", name, field, line.Name, other)
			}
			names[line.Name] = field
		}
		for field, extra := range t.ExtraAttributes {
			if err := extra.check(field); err != nil {
				return fmt.Errorf("types.%s.extraAttributes.%s: %w", name, field, err)
			}
			// A skipped API field is no attribute, so an extra attribute can take its name.
			if line, ok := t.Fields[field]; ok && !line.Skip {
				return fmt.Errorf("types.%s.extraAttributes.%s: the type already has a field override of that name", name, field)
			}
		}
	}
	return nil
}

// checkUpgrades checks that each upgrade reads an older version through a prior schema function.
func (s Schema) checkUpgrades() error {
	for version, u := range s.Upgrade {
		if version >= s.Version {
			return fmt.Errorf("schema.upgrade.%d: the version must be older than schema.version %d", version, s.Version)
		}
		if _, _, ok := u.PriorSchemaFunc(); !ok || u.Strategy != "refresh" {
			return fmt.Errorf("schema.upgrade.%d: priorSchema must be <import path>.<Func>, and strategy must be \"refresh\"", version)
		}
	}
	return nil
}

// checkEnums checks that no value is accepted and rejected at the same time, and that names
// gives accepted values distinct Terraform values.
func (f *File) checkEnums() error {
	for _, name := range sortedKeys(f.Enums) {
		e := f.Enums[name]
		for _, v := range e.Rejected {
			if slices.Contains(e.Values, v) {
				return fmt.Errorf("enums.%s: %q is in values and in rejected", name, v)
			}
		}
		if e.TrimPrefix != "" {
			for _, v := range e.Values {
				if !strings.HasPrefix(v, e.TrimPrefix) || v == e.TrimPrefix {
					return fmt.Errorf("enums.%s: trimPrefix %q is not a prefix of %q", name, e.TrimPrefix, v)
				}
			}
		}
		for _, v := range sortedKeys(e.Names) {
			if !slices.Contains(e.Values, v) {
				return fmt.Errorf("enums.%s.names: %q is not in values", name, v)
			}
			if e.Names[v] == "" {
				return fmt.Errorf("enums.%s.names: %q has an empty name", name, v)
			}
			if e.Names[v] == e.defaultTFValue(v) {
				return fmt.Errorf("enums.%s.names: %q is already the Terraform value of %q, so the entry changes nothing", name, e.Names[v], v)
			}
		}
		if e.ZeroNull && e.Zero != "" {
			return fmt.Errorf("enums.%s: zeroNull reads the zero value as null, so it has no Terraform value: remove zero", name)
		}
		seen := map[string]string{}
		if e.Zero != "" {
			seen[e.Zero] = "zero"
		}
		if err := e.checkAliases(name); err != nil {
			return err
		}
		if e.ReadRejected == ReadRejectedLower && e.Verbatim {
			return fmt.Errorf("enums.%s: readRejected: lower excludes verbatim", name)
		}
		if e.ReadRejected.On() {
			// A rejected value reads back as its read spelling, so no accepted value can have it.
			for _, v := range e.Rejected {
				tf := e.RejectedTFValue(v)
				if previous, ok := seen[tf]; ok {
					return fmt.Errorf("enums.%s: %s and the rejected %q have the same Terraform value %q", name, previous, v, tf)
				}
				seen[tf] = "the rejected " + strconv.Quote(v)
			}
		}
		for _, v := range e.Values {
			tf := e.TFValue(v)
			if previous, ok := seen[tf]; ok {
				return fmt.Errorf("enums.%s: %q and %s have the same Terraform value %q", name, v, previous, tf)
			}
			seen[tf] = strconv.Quote(v)
		}
	}
	return nil
}

// keys returns the names of the keys that the line sets, in the order of the file format.
func (l Field) keys() []string {
	var keys []string
	add := func(set bool, name string) {
		if set {
			keys = append(keys, name)
		}
	}
	add(l.Skip, "skip")
	add(len(l.ValueNames) != 0, "valueNames")
	add(l.AsBool, "asBool")
	add(l.AsString, "asString")
	add(l.TerraformType != "", "terraformType")
	add(l.StoredAs != "", "storedAs")
	add(l.DefaultArm, "defaultArm")
	add(l.Description != nil, "description")
	add(l.MarkdownDescription != nil, "markdownDescription")
	add(l.ReadOnly, "readOnly")
	add(l.Required, "required")
	add(l.Optional, "optional")
	add(l.SchemaOptional, "schemaOptional")
	add(len(l.CreateAs) != 0, "createAs")
	add(l.Collection != "", "collection")
	add(l.Name != "", "name")
	add(l.Deprecation != "", "deprecation")
	add(l.Sensitive, "sensitive")
	add(l.Computed != nil, "computed")
	add(l.UseStateForUnknown, "useStateForUnknown")
	add(l.UseNonNullStateForUnknown, "useNonNullStateForUnknown")
	add(l.Default != nil, "default")
	add(l.ReadEmptyAs != "", "readEmptyAs")
	add(l.ReadNullAs != "", "readNullAs")
	add(l.SendNullAsEmpty, "sendNullAsEmpty")
	add(l.SendUnknownAsZero, "sendUnknownAsZero")
	add(l.SplitArms, "splitArms")
	add(l.ShowInverted, "showInverted")
	add(l.RequestPosition, "requestPosition")
	add(l.CarrySiblings != nil, "carrySiblings")
	add(l.RequireOne, "requireOne")
	add(l.RequestValue != "", "requestValue")
	add(len(l.RequestValueBy) != 0, "requestValueBy")
	add(l.KeyedBy != nil, "keyedBy")
	add(l.KeepPriorOrder, "keepPriorOrder")
	add(len(l.Validators) != 0, "validators")
	add(l.Equality != "", "equality")
	add(l.Unwrap != nil, "unwrap")
	add(l.RequiresReplace, "requiresReplace")
	add(l.UniqueBy != "", "uniqueBy")
	add(l.RequestAs != "", "requestAs")
	add(l.UpdateAs != "", "updateAs")
	add(l.UpdateDefault != "", "updateDefault")
	add(l.ItemName != "", "itemName")
	add(l.Negate, "negate")
	add(l.Split != "", "split")
	add(l.Inline, "inline")
	return keys
}

// statesPresence reports whether the line says how an omitted value behaves. A line that only
// changes a text, a validator, or the order does not: the contract must still state presence.
func (l Field) statesPresence() bool {
	return l.Skip || l.ReadOnly || l.Required || l.Computed != nil || l.Default != nil || l.ReadEmptyAs != "" || l.ReadNullAs != "" || l.Fixed()
}

// Fixed reports whether Create and Update send a value that the line states, so the field is
// not a Terraform attribute.
func (l Field) Fixed() bool { return l.RequestValue != "" || len(l.RequestValueBy) != 0 }

// replacesServerDefault reports whether the line states a mode that a server default of the
// contract cannot have: its own default, no computed value, or what an omitted value plans and
// sends (sendUnknownAsZero, useStateForUnknown, useNonNullStateForUnknown), or a Terraform value
// that is the opposite of the API value (showInverted). The server-default plan modifier would
// make a removal unknown on top of them, or compare the inverted state with the API default.
// required: true needs no check: a server default needs a field that Get requires, and then the
// line is unused.
func (l Field) replacesServerDefault() bool {
	return l.Default != nil || l.Computed != nil && !*l.Computed || l.SendUnknownAsZero || l.UseStateForUnknown || l.UseNonNullStateForUnknown || l.ShowInverted
}

// KeepsReleasedOmission reports whether the line decides how an omitted value behaves, so a
// server default of the contract does not apply: it states presence, its own default, no computed
// value, or that the plan keeps the state. A split line shows the string as a set, which reads an
// omitted or empty string as an empty set, so the string default of the contract does not apply
// either. A line that only changes a text, a validator, the order, or the Terraform type keeps the
// server default.
func (l Field) KeepsReleasedOmission() bool {
	return l.statesPresence() || l.replacesServerDefault() || l.UseStateForUnknown || l.UseNonNullStateForUnknown || l.Split != ""
}

func (e ExtraAttribute) check(name string) error {
	return e.checkAt(name, false)
}

// checkAt checks the attribute; nested says that it is inside an object extra attribute.
func (e ExtraAttribute) checkAt(name string, nested bool) error {
	if err := checkExtraName(name); err != nil {
		return err
	}
	if err := e.checkShape(); err != nil {
		return err
	}
	if err := e.checkMode(nested); err != nil {
		return err
	}
	if e.MarkdownDescription == "" {
		return errors.New("markdownDescription is required")
	}
	for child, attribute := range e.Attributes {
		if err := attribute.checkAt(child, true); err != nil {
			return fmt.Errorf("attributes.%s: %w", child, err)
		}
	}
	return nil
}

// checkMode checks the keys that set how the attribute is planned: optional, default, equality,
// and what an object may hold.
func (e ExtraAttribute) checkMode(nested bool) error {
	if e.Optional && (!e.Computed || e.Required) {
		return errors.New("optional: true needs computed: true and no required: an extra attribute that is not computed is optional already")
	}
	if e.Optional && e.WriteOnly {
		return errors.New("optional: true cannot be combined with writeOnly")
	}
	if e.Default != nil && !e.Optional {
		return errors.New("default needs optional: true and computed: true: the Framework only allows a default on a computed attribute")
	}
	if e.UseStateForUnknown && !e.Computed {
		return errors.New("useStateForUnknown needs a computed attribute")
	}
	if nested && (e.Dynamic || e.WriteOnly || e.InUpdate || e.Equality != "") {
		return errors.New("an attribute inside an object cannot be dynamic, writeOnly, inUpdate, or have an equality line")
	}
	return e.checkEquality()
}

func (e ExtraAttribute) checkEquality() error {
	if e.Equality == "" {
		return nil
	}
	if e.Equality != "json" && e.Equality != "yaml" {
		return fmt.Errorf("equality is %q, want json or yaml", e.Equality)
	}
	if e.Type != "string" || e.Computed && !e.Optional || e.WriteOnly {
		return errors.New("equality needs a string that the user sets, and that is not writeOnly")
	}
	return nil
}

func checkExtraName(name string) error {
	if name == "" {
		return errors.New("the attribute name is empty")
	}
	for _, r := range name {
		if r != '_' && (r < 'a' || r > 'z') && (r < '0' || r > '9') {
			return fmt.Errorf("the attribute name %q must be a Terraform name (lowercase letters, digits, underscores)", name)
		}
	}
	if name[0] < 'a' || name[0] > 'z' {
		return fmt.Errorf("the attribute name %q must start with a letter", name)
	}
	return nil
}

func (e ExtraAttribute) checkShape() error {
	if e.Dynamic {
		return e.checkDynamic()
	}
	if err := e.checkType(); err != nil {
		return err
	}
	if e.Computed && !e.Optional && (e.Required || e.WriteOnly || e.InUpdate) {
		return errors.New("computed cannot be combined with required, writeOnly, or inUpdate")
	}
	if e.WriteOnly && e.InUpdate {
		return errors.New("inUpdate cannot be combined with writeOnly: a write-only value is not in the plan or the state, so a change of only that value cannot make an Update")
	}
	return nil
}

func (e ExtraAttribute) checkDynamic() error {
	if e.ElementType != "" || e.WriteOnly {
		return errors.New("a dynamic attribute has no elementType and cannot be writeOnly")
	}
	if e.Type != "" || e.Computed {
		return errors.New("a dynamic attribute has no type and cannot be computed")
	}
	return nil
}

func (e ExtraAttribute) checkType() error {
	if e.Type != "object" && len(e.Attributes) != 0 {
		return errors.New("attributes belong to type: object")
	}
	switch e.Type {
	case "":
		if e.ElementType != "string" && e.ElementType != "int64" {
			return fmt.Errorf("elementType is %q, want string or int64", e.ElementType)
		}
	case "string", "int64", "bool":
		if e.ElementType != "" {
			return fmt.Errorf("a %s attribute has no elementType", e.Type)
		}
	case "set", "list":
		if e.ElementType != "string" {
			return fmt.Errorf("a %s attribute needs elementType string, got %q", e.Type, e.ElementType)
		}
	case "object":
		return e.checkObject()
	default:
		return fmt.Errorf("type is %q, want string, int64, bool, set, list, or object", e.Type)
	}
	return nil
}

// checkObject checks an object extra attribute: the user sets it, and it has attributes.
func (e ExtraAttribute) checkObject() error {
	if e.ElementType != "" || len(e.Attributes) == 0 {
		return errors.New("an object attribute has attributes and no elementType")
	}
	if e.Computed && !e.Optional || e.WriteOnly {
		return errors.New("an object attribute is set by the user: it cannot be computed-only or writeOnly")
	}
	return nil
}

// hasTypeKey reports whether the line changes the Terraform type of the field: asBool, asString,
// terraformType, or storedAs.
func (l Field) hasTypeKey() bool {
	return l.AsBool || l.AsString || l.TerraformType != "" || l.StoredAs != ""
}

func (l Field) empty() bool {
	return l.emptyMode() && l.emptyValue()
}

func (l Field) emptyMode() bool {
	return !l.Skip && !l.ReadOnly && l.Description == nil && l.MarkdownDescription == nil && !l.Required && !l.Optional && !l.SchemaOptional && l.Name == "" && len(l.ValueNames) == 0 && !l.hasTypeKey() && !l.DefaultArm && l.Deprecation == "" && !l.Sensitive &&
		l.Computed == nil && !l.UseStateForUnknown && !l.UseNonNullStateForUnknown && !l.RequiresReplace && !l.Inline && !l.RequestPosition
}

func (l Field) emptyValue() bool {
	return l.Default == nil && len(l.CreateAs) == 0 && l.Collection == "" && l.KeyedBy == nil && l.ReadEmptyAs == "" && l.ReadNullAs == "" && !l.RequireOne && !l.Fixed() &&
		l.emptyShape()
}

// emptyShape reports whether the line sets none of the keys that change how a value is shown or
// compared.
func (l Field) emptyShape() bool {
	return !l.SendUnknownAsZero && !l.SplitArms && !l.ShowInverted && l.CarrySiblings == nil &&
		!l.KeepPriorOrder && len(l.Validators) == 0 && l.Equality == "" && l.Unwrap == nil && l.UniqueBy == "" && l.RequestAs == "" && l.UpdateAs == "" && l.UpdateDefault == "" && l.ItemName == "" && !l.Negate && l.Split == ""
}

func (l Field) check() error {
	if l.empty() {
		return errors.New("the line sets nothing")
	}
	if err := l.checkConflicts(); err != nil {
		return err
	}
	if err := l.checkDefault(); err != nil {
		return err
	}
	if err := l.checkSplitArms(); err != nil {
		return err
	}
	if l.Required && l.Computed != nil && *l.Computed {
		return errors.New("a required field cannot be computed")
	}
	if l.Required && l.Optional {
		return errors.New("set required or optional, not both")
	}
	// With computed: true, a schemaOptional attribute is Optional+Computed and can retain values
	// returned by the API. The request field remains required.
	if l.SchemaOptional && (l.Required || l.Optional || l.Computed != nil && !*l.Computed) {
		return errors.New("schemaOptional cannot be combined with required, optional, or computed: false")
	}
	if l.RequiresReplace && l.Computed != nil && *l.Computed {
		// Optional+Computed with RequiresReplace: when the field is omitted, any other change
		// plans its value as unknown, and the unknown value replaces the resource.
		return errors.New("requiresReplace cannot be combined with computed: true")
	}
	if err := l.Field.Check(); err != nil {
		return err
	}
	if l.Name != "" && !validAttrName.MatchString(l.Name) {
		return fmt.Errorf("name %q is not a Terraform attribute name: lower case letters, digits, and _", l.Name)
	}
	if l.Description != nil && l.MarkdownDescription != nil {
		return errors.New("set description or markdownDescription, not both")
	}
	if l.Skip && (l.hasBehavior() || l.Name != "") {
		return errors.New("a skipped field has no other behavior")
	}
	if l.TerraformType != "" && l.TerraformType != "Int64" && l.TerraformType != "String" {
		return fmt.Errorf("terraformType is %q, want \"Int64\" or \"String\"", l.TerraformType)
	}
	if l.StoredAs != "" && l.StoredAs != "float32" {
		return fmt.Errorf("storedAs is %q, want \"float32\"", l.StoredAs)
	}
	if l.RequestValue != "" && l.Name != "" {
		return errors.New("a requestValue field is no Terraform attribute, so name changes nothing")
	}
	if err := checkValueNames(l.ValueNames); err != nil {
		return err
	}
	if l.Inline && !slices.Equal(l.keys(), []string{"required", "inline"}) && len(l.keys()) != 1 {
		return errors.New("an inlined field has no attribute of its own, so the line sets only inline, and required when the API always has the object: put the other keys on the lines of its fields")
	}
	return checkValidators(l.Validators)
}

func (l Field) checkConflicts() error {
	if l.UseStateForUnknown && l.UseNonNullStateForUnknown {
		return errors.New("useStateForUnknown and useNonNullStateForUnknown cannot both be set")
	}
	if l.ReadEmptyAs != "" && l.ReadEmptyAs != "null" {
		return fmt.Errorf("readEmptyAs is %q, want \"null\"", l.ReadEmptyAs)
	}
	if l.ReadNullAs != "" && l.ReadNullAs != "empty" {
		return fmt.Errorf("readNullAs is %q, want \"empty\"", l.ReadNullAs)
	}
	if l.ReadEmptyAs != "" && l.ReadNullAs != "" {
		return errors.New("readEmptyAs and readNullAs cannot both be set")
	}
	if l.SendNullAsEmpty && l.ReadEmptyAs != "null" {
		return errors.New("sendNullAsEmpty needs readEmptyAs: \"null\", or the stored \"\" reads back as \"\"")
	}
	if l.Skip && l.Fixed() {
		return errors.New("requestValue sends the field, so skip cannot be set")
	}
	if err := l.checkReleasedKeys(); err != nil {
		return err
	}
	if l.RequiresReplace && l.Fixed() {
		return errors.New("a requestValue field is no Terraform attribute and Update always sends it, so requiresReplace cannot be set")
	}
	if l.RequestValue != "" && len(l.RequestValueBy) != 0 {
		return errors.New("requestValue and requestValueBy cannot both be set")
	}

	for _, field := range sortedKeys(l.RequestValueBy) {
		if field == "" || l.RequestValueBy[field] == "" {
			return errors.New("requestValueBy maps a field name to an API value")
		}
	}
	if l.Equality != "" && l.Equality != EqualityYAML && l.Equality != EqualityJSON {
		return fmt.Errorf("equality is %q, want %q or %q", l.Equality, EqualityYAML, EqualityJSON)
	}
	return nil
}

// checkAsString rejects a key that cannot work on a oneOf that Terraform shows as one string.
func (l Field) checkAsString() error {
	if !l.AsString {
		return nil
	}
	if l.AsBool {
		return errors.New("asString and asBool cannot both be set")
	}
	if l.CarrySiblings != nil || l.SplitArms || l.Inline || l.KeyedBy != nil || l.Collection != "" || l.Unwrap != nil {
		return errors.New("asString shows the oneOf as one string, so carrySiblings, splitArms, inline, keyedBy, collection, and unwrap cannot be set")
	}
	if l.RequireOne || l.Fixed() || l.TerraformType != "" || len(l.ValueNames) != 0 || l.Equality != "" || l.KeepPriorOrder {
		return errors.New("asString cannot be combined with requireOne, requestValue, requestValueBy, terraformType, valueNames, equality, or keepPriorOrder")
	}
	if slices.ContainsFunc(l.Validators, func(v Validator) bool { return v.EnumExceptZero }) {
		return errors.New("asString shows the zero value of the enum as a value of the string, so enumExceptZero cannot be set")
	}
	return nil
}

// checkDefault checks the type of a default: a string, a bool, a number, an empty list, null, or
// an object whose values are of those types.
func (l Field) checkDefault() error {
	return checkDefaultValue("default", l.Default)
}

func checkDefaultValue(where string, value any) error {
	switch d := value.(type) {
	case nil, nullDefaultValue, string, bool, int, float64:
	case []any:
		if len(d) != 0 {
			return fmt.Errorf("%s: a list default must be empty: []", where)
		}
	case map[string]any:
		for _, name := range sortedKeys(d) {
			if err := checkDefaultValue(where+"."+name, d[name]); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("%s is %T, want a string, a bool, a number, [], null, or an object", where, value)
	}
	return nil
}

// nullDefaultValue is the type of NullDefault.
type nullDefaultValue struct{}

// NullDefault is the Default of a line that says default: null. A YAML null decodes as nil, the
// same as a missing key, so Parse finds the key in the YAML tree and sets this value in its place.
// It plans a null list, set, map, or object for an omitted Optional and Computed attribute, where
// it would plan an unknown value without a default.
var NullDefault any = nullDefaultValue{}

// IsNullDefault reports whether a default is NullDefault.
func IsNullDefault(value any) bool {
	_, ok := value.(nullDefaultValue)
	return ok
}

// markNullDefaults finds each `default: null` of a field line, which the struct decoder cannot tell
// from a missing key, and sets NullDefault on the line. A `default: null` anywhere else is an
// error: it would change nothing.
func markNullDefaults(data []byte, f *File) error {
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return err
	}
	if len(root.Content) == 0 {
		return nil
	}
	return walkNullDefaults(root.Content[0], nil, f)
}

func walkNullDefaults(n *yaml.Node, path []string, f *File) error {
	switch n.Kind {
	case yaml.SequenceNode:
		for i, child := range n.Content {
			if err := walkNullDefaults(child, append(slices.Clone(path), strconv.Itoa(i)), f); err != nil {
				return err
			}
		}
	case yaml.MappingNode:
		for i := 0; i+1 < len(n.Content); i += 2 {
			if err := walkNullDefaultsOf(n.Content[i].Value, n.Content[i+1], path, f); err != nil {
				return err
			}
		}
	}
	return nil
}

// walkNullDefaultsOf handles one key of a mapping: a null default, or a node to walk into.
func walkNullDefaultsOf(key string, value *yaml.Node, path []string, f *File) error {
	here := append(slices.Clone(path), key)
	atFields := len(path) == 3 && path[0] == "types" && path[2] == "fields" // key names a field
	atLine := len(path) == 4 && path[0] == "types" && path[2] == "fields"   // key is a key of a line
	if key != "default" || value.Kind != yaml.ScalarNode || value.ShortTag() != "!!null" || atFields {
		return walkNullDefaults(value, here, f)
	}
	if !atLine {
		return fmt.Errorf("%s: default: null changes nothing here: only a field line (types.<Type>.fields.<field>) takes it", strings.Join(here, "."))
	}
	t := f.Types[path[1]]
	line := t.Fields[path[3]]
	line.Default = NullDefault
	t.Fields[path[3]] = line
	return nil
}

// checkSplitArms rejects a key of a splitArms line that only the lines of the arms can use.
func (l Field) checkSplitArms() error {
	if !l.SplitArms {
		return nil
	}
	for _, key := range l.keys() {
		if key != "splitArms" && key != "collection" && key != "computed" && key != "default" {
			return fmt.Errorf("a splitArms field has no attribute of its own, so %s has no effect: put it on the lines of the arms", key)
		}
	}
	return nil
}

// checkReleasedKeys checks the keys that keep a released behavior: sendUnknownAsZero,
// requestPosition, carrySiblings, and showInverted.
func (l Field) checkReleasedKeys() error {
	if l.SendUnknownAsZero && (l.Computed == nil || !*l.Computed) {
		return errors.New("sendUnknownAsZero needs computed: true: only a computed attribute is unknown in a plan")
	}
	if l.SendUnknownAsZero && l.Default != nil {
		return errors.New("a default is never unknown, so sendUnknownAsZero changes nothing")
	}
	if l.RequestPosition && (l.Required || l.Optional || l.Computed != nil || l.Default != nil || l.Fixed() || len(l.Validators) != 0) {
		return errors.New("requestPosition makes the attribute computed-only and sends the position of the item, so it takes no mode, default, requestValue, or validator")
	}
	if l.ShowInverted && l.Fixed() {
		return errors.New("a requestValue field is no Terraform attribute, so showInverted changes nothing: send the API value")
	}
	if err := l.checkAsString(); err != nil {
		return err
	}
	return l.checkCarry()
}

// checkCarry checks a carrySiblings line. The siblings live in the arm block, and a response
// without an arm has no block to hold them, so the line needs requireOne: the read then fails
// instead of dropping their values.
func (l Field) checkCarry() error {
	if l.CarrySiblings == nil {
		return nil
	}
	if err := l.CarrySiblings.check(); err != nil {
		return err
	}
	if !l.RequireOne {
		return errors.New("carrySiblings needs requireOne: true: a response without an arm has no block to hold the siblings")
	}
	return nil
}

// checkValueNames checks that every number has a name, and that no two numbers have one name.
func checkValueNames(names map[int64]string) error {
	seen := map[string]int64{}
	for _, n := range sortedIntKeys(names) {
		name := names[n]
		if name == "" {
			return fmt.Errorf("valueNames: %d has an empty name", n)
		}
		if other, ok := seen[name]; ok {
			return fmt.Errorf("valueNames: %d and %d both have the name %q", other, n, name)
		}
		seen[name] = n
	}
	return nil
}

// SortedValueNames returns the numbers of a valueNames line in ascending order with their names.
func (l Field) SortedValueNames() []ValueName {
	out := make([]ValueName, 0, len(l.ValueNames))
	for _, n := range sortedIntKeys(l.ValueNames) {
		out = append(out, ValueName{Number: n, Name: l.ValueNames[n]})
	}
	return out
}

// ValueName is one number of a valueNames line and its Terraform name.
type ValueName struct {
	Number int64
	Name   string
}

func sortedIntKeys[V any](m map[int64]V) []int64 {
	keys := make([]int64, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

// hasBehavior reports whether the line sets a behavior that a skipped field cannot have.
func (l Field) hasBehavior() bool {
	return l.ReadOnly || l.Computed != nil || l.Required || l.Optional || l.SchemaOptional || len(l.CreateAs) != 0 || l.Collection != "" || l.Name != "" || l.KeyedBy != nil || l.Default != nil || l.KeepPriorOrder ||
		l.ReadEmptyAs != "" || l.ReadNullAs != "" || l.RequireOne || l.Fixed() ||
		len(l.Validators) != 0 || l.Equality != "" || l.Unwrap != nil || len(l.ValueNames) != 0 || l.hasTypeKey() || l.DefaultArm ||
		l.RequiresReplace || l.UniqueBy != "" || l.Negate || l.Split != ""
}

func checkValidators(validators []Validator) error {
	for _, v := range validators {
		set := 0
		if len(v.OneOf) != 0 {
			set++
		}
		if v.SizeAtLeast != nil {
			set++
		}
		if v.LengthAtLeast != nil {
			set++
		}
		if v.LengthAtMost != nil {
			set++
		}
		if v.Enum {
			set++
		}
		if v.EnumExceptZero {
			set++
		}
		if v.Between != nil {
			if len(v.Between) != 2 || v.Between[0] > v.Between[1] {
				return errors.New("between needs a minimum and a maximum, in that order")
			}
			set++
		}
		if len(v.ExactlyOneOf) != 0 {
			set++
		}
		if len(v.AtLeastOneOf) != 0 {
			set++
		}
		if len(v.AlsoRequires) != 0 {
			set++
		}
		if len(v.ConflictsWith) != 0 {
			set++
		}
		if v.AtLeast != nil {
			set++
		}
		if v.RegexMatches != nil {
			set++
			if _, err := regexp.Compile(v.RegexMatches.Pattern); err != nil || v.RegexMatches.Pattern == "" {
				return fmt.Errorf("regexMatches needs a pattern that Go compiles: %q", v.RegexMatches.Pattern)
			}
		}
		if v.KeysAre != nil {
			set++
			if _, err := regexp.Compile(v.KeysAre.Pattern); err != nil || v.KeysAre.Pattern == "" {
				return fmt.Errorf("keysAre needs a pattern that Go compiles: %q", v.KeysAre.Pattern)
			}
		}
		if set != 1 {
			return errors.New("each validator sets exactly one of oneOf, sizeAtLeast, lengthAtLeast, lengthAtMost, enum, enumExceptZero, between, exactlyOneOf, atLeastOneOf, alsoRequires, conflictsWith, atLeast, regexMatches, and keysAre")
		}
	}
	return nil
}

// Policy returns the rule set for the model: the rules that the API keys state, and the
// exceptions that the Terraform keys cover.
func (f *File) Policy() model.Policy {
	p := f.Exceptions().Policy()
	if f.Upserts() {
		// The generator names the operations of the reshaped contract (reshape.go).
		p.Operations = model.OperationIDs{}
	}
	p.EnumAnyPrefix = append(p.EnumAnyPrefix, sortedKeys(f.Enums)...)
	for _, name := range sortedKeys(f.Types) {
		t := f.Types[name]
		for _, field := range sortedKeys(t.Fields) {
			line := t.Fields[field]
			if line.Required {
				p.ClientRequired = append(p.ClientRequired, name+"."+field)
			}
			if line.Fixed() {
				p.FixedRequest = append(p.FixedRequest, name+"."+field)
			}
			if c := line.Computed; c != nil && *c {
				p.Computed = append(p.Computed, name+"."+field)
			}
			if line.UseStateForUnknown || line.UseNonNullStateForUnknown {
				p.KeepState = append(p.KeepState, name+"."+field)
			}
			if n := line.Name; n != "" {
				if p.Names == nil {
					p.Names = map[string]string{}
				}
				p.Names[name+"."+field] = n
			}
			if line.TerraformType == "Int64" {
				p.Int64 = append(p.Int64, name+"."+field)
			}
			if line.AsBool {
				p.Bools = append(p.Bools, name+"."+field)
			}
			if line.DefaultArm {
				p.DefaultArms = append(p.DefaultArms, name+"."+field)
			}
			// Exceptions.Policy already lists a skipped or readOnly field.
			if line.statesPresence() && !line.Skip && !line.ReadOnly {
				p.Released = append(p.Released, name+"."+field)
			}
			if line.replacesServerDefault() {
				p.Defaults = append(p.Defaults, name+"."+field)
			}
		}
	}
	for _, name := range sortedKeys(f.Enums) {
		e := f.Enums[name]
		for _, v := range e.Values {
			if p.EnumValues == nil {
				p.EnumValues = map[string][]model.EnumValue{}
			}
			p.EnumValues[name] = append(p.EnumValues[name], model.EnumValue{API: v, Name: e.TFValue(v)})
		}
	}
	return p
}

// Exceptions returns the keys of the file that describe the API.
func (f *File) Exceptions() iacoverrides.Exceptions {
	e := iacoverrides.Exceptions{API: f.API.API, Unwrap: f.Unwrap}
	for name, t := range f.Types {
		if t.RequestWrapper != "" {
			if e.RequestWrappers == nil {
				e.RequestWrappers = map[string]string{}
			}
			e.RequestWrappers[name] = t.RequestWrapper
		}
		for field, line := range t.Fields {
			if e.Fields == nil {
				e.Fields = map[string]map[string]iacoverrides.Field{}
			}
			if e.Fields[name] == nil {
				e.Fields[name] = map[string]iacoverrides.Field{}
			}
			api := line.Field
			if line.Default != nil && !IsNullDefault(line.Default) {
				// optional with a static default is a Terraform exception: Terraform always sends
				// the default, so the request still has the value that the contract requires,
				// which the SDK holds as a value. A null default sends nothing: the request
				// keeps its optional line, which sends an empty list for a required one.
				api.Optional = false
			}
			e.Fields[name][field] = api
		}
	}
	return e
}

// Lines returns one description per line of the file that the contract must match,
// in a stable order. The generator uses it to report a line that matches nothing.
func (f *File) Lines() []Line {
	file := f
	var out []Line
	for _, name := range sortedKeys(f.Enums) {
		out = append(out, Line{Kind: KindEnum, Component: name, EnumZero: f.Enums[name].Zero, EnumZeroNull: f.Enums[name].ZeroNull, EnumValues: f.Enums[name].Values, EnumRejected: f.Enums[name].Rejected})
	}
	for _, name := range sortedKeys(f.Types) {
		t := f.Types[name]
		if t.Required != nil {
			out = append(out, Line{Kind: KindEmptyRequired, Component: name})
		}
		for _, field := range sortedKeys(t.Fields) {
			f := t.Fields[field]
			out = append(out, Line{Kind: KindField, Component: name, Field: field, ReadOnly: f.ReadOnly, Required: f.Required, Optional: f.Optional, Collection: f.Collection, Name: f.Name, NoAttribute: t.noAttribute(), RequestValueBy: f.RequestValueBy, KeyedBy: f.KeyedBy, Accepted: file.accepted(f.KeyedBy), Equality: f.Equality, Inline: f.Inline, Keys: f.keys()})
		}
		for _, group := range sortedKeys(t.Groups) {
			out = append(out, Line{Kind: KindGroup, Component: name, Field: group, GroupFields: t.Groups[group].Fields})
		}
	}
	return out
}

// noAttribute returns the fields of the type that have no Terraform attribute, in order: skipped
// fields and requestValue fields.
func (t Type) noAttribute() []string {
	var out []string
	for _, field := range sortedKeys(t.Fields) {
		if t.Fields[field].Skip || t.Fields[field].RequestValue != "" {
			out = append(out, field)
		}
	}
	return out
}

// Kind is the kind of a Line.
type Kind string

const (
	KindEnum          Kind = "enum"
	KindEmptyRequired Kind = "empty-required"
	KindField         Kind = "field"
	KindGroup         Kind = "group"
)

// Line is one place in the contract that the file refers to.
type Line struct {
	Kind       Kind
	Component  string
	Field      string
	EnumValues []string // for an enum line: the values that the resource accepts
	EnumZero   string   // for an enum line: the Terraform value of the zero value
	// EnumZeroNull: for an enum line, the zero value reads as null and has no Terraform value.
	EnumZeroNull bool
	// EnumRejected are the values of the contract that the resource does not accept.
	EnumRejected []string
	ReadOnly     bool   // for a field line: the file says that the server sets the field
	Equality     string // for a field line: "yaml" or "json" when the line compares the value as a document
	Required     bool   // for a field line: the file says that the field is required
	Optional     bool   // for a field line: the file says that the field is optional
	Collection   string // for a field line: "set" when the file shows a list as a set
	Name         string // for a field line: the Terraform attribute name
	// NoAttribute are, for a field line, the fields of the component without a Terraform
	// attribute: skipped fields and requestValue fields. A name line can take their name.
	NoAttribute []string
	Inline      bool // for a field line: the fields of the object are top-level attributes
	// RequestValueBy is, for a field line, the API value sent for each object field that is set.
	RequestValueBy map[string]string
	// KeyedBy is, for a field line, the key and value properties of a list shown as a map.
	KeyedBy *iacoverrides.KeyedBy
	// Accepted are, for a keyedBy object line, the accepted values of each enum that has an enums
	// line, with their Terraform values: the attributes that the object can have.
	Accepted map[string]map[string]string
	Keys     []string // for a field line: every key that the line sets
	// GroupFields are, for a group line (Field is the group name), the fields that the group holds.
	GroupFields []string
}

// String names the line as it is written in the file.
func (l Line) String() string {
	switch l.Kind {
	case KindEnum:
		return "enums." + l.Component
	case KindEmptyRequired:
		return "types." + l.Component + ".required"
	case KindGroup:
		return "types." + l.Component + ".groups." + l.Field
	default:
		return "types." + l.Component + ".fields." + l.Field
	}
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return slices.Clip(keys)
}

var validAttrName = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// accepted returns the accepted values of each enum that has an enums line, for a keyedBy object
// line, and nil for any other line.
func (f *File) accepted(k *iacoverrides.KeyedBy) map[string]map[string]string {
	if k == nil || !k.Object {
		return nil
	}
	out := map[string]map[string]string{}
	for name, e := range f.Enums {
		out[name] = map[string]string{}
		for _, v := range e.Values {
			out[name][v] = e.TFValue(v)
		}
	}
	return out
}

// TypedArmsOf returns the typedArms line of the component, or nil.
func (f *File) TypedArmsOf(component string) *TypedArms {
	if f == nil {
		return nil
	}
	return f.Types[component].TypedArms
}
