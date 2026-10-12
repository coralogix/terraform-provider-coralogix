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
	// Hoisted is what the generator derives from the upsert and hoist lines. It is not a key of
	// the file.
	Hoisted *Hoisted `yaml:"-"`
	// FixedID is the released value of the id attribute of a singleton, for example
	// "archive-logs-settings". "" uses the resource type name.
	FixedID string `yaml:"fixedID"`
	// TypeName is the released resource type name without the provider prefix, for example
	// "slo_v2" for coralogix_slo_v2. "" names the resource after its OpenAPI component.
	TypeName string `yaml:"typeName"`
	// SingletonID: the resource is a singleton (Get has no path parameter) whose component has an id
	// field that only the server sets. The id attribute then holds that value, and Create, Get,
	// Update, and Delete still take no id. Without the key, a singleton has the fixed id. Mark the
	// field readOnly. The id must be a string.
	SingletonID bool `yaml:"singletonID"`
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
}

// UpdateMaskOmit is the updateMask value that sends no update mask.
const UpdateMaskOmit = "omit"

// API is the api section: the API keys of the iac module, and the Terraform keys.
type API struct {
	iacoverrides.API `yaml:",inline"`
	// Upsert names the two calls of a singleton that always exists: a Get, and a Set that
	// creates or replaces it. The API has no Create, Update, or Delete, so the generated Delete
	// makes no call.
	Upsert *Upsert `yaml:"upsert"`
}

// Upsert is a singleton that one call sets and another reads, for example GET and POST on
// /logs/data-setup/v2. Create and Update send the whole resource with Set.
type Upsert struct {
	// Get is the operationId of the GET without path parameters.
	Get string `yaml:"get"`
	// Set is the operationId of the POST on the same path. It creates or replaces the resource.
	Set string `yaml:"set"`
}

// Check checks the API keys, and that an upsert names both calls and no other key that names an
// operation or the id: the singleton has one Set call and no id.
func (a API) Check() error {
	if err := a.API.Check(); err != nil {
		return err
	}
	if a.Upsert == nil {
		return nil
	}
	if a.Upsert.Get == "" || a.Upsert.Set == "" {
		return errors.New("api.upsert.get and api.upsert.set are required")
	}
	if a.Operations != nil || a.Delete != nil || a.RequestWrapper != "" || a.UpdateIDInBody || a.ClientSetID || a.ServerSetID || a.IDProperty != "" || a.CustomMethods || a.UpdateMaskInBody {
		return errors.New("api.upsert is a singleton that one call sets: it cannot have operations, delete, requestWrapper, updateIDInBody, clientSetID, serverSetID, idProperty, customMethods, or updateMaskInBody")
	}
	return nil
}

// Hoisted is, for each hoisted top-level field, the API object that holds it in the Create body,
// the Update body, and the resource response. The generator reads the contract with each hoisted
// object replaced by its fields, so the model sees top-level fields; the renderer reads and writes
// them through the object.
type Hoisted struct {
	Create, Update, Get map[string]Parent
}

// Parent is the object field of the API that holds a hoisted top-level field.
type Parent struct {
	Field  string // API property of the object, for example "s3"
	Schema string // component of the object
	// Arm: the object is an arm of a oneOf group of the resource response, and the file skips the
	// other arms. When Get returns no such object, the resource does not exist.
	Arm bool
}

// Upserts reports whether the resource is a singleton that one call sets (api.upsert).
func (f *File) Upserts() bool { return f != nil && f.API.Upsert != nil }

// HoistedFields returns the fields of the resource that hoist lines name, in order.
func (f *File) HoistedFields() []string {
	if f == nil {
		return nil
	}
	var out []string
	for _, field := range sortedKeys(f.Types[f.Resource].Fields) {
		if f.Types[f.Resource].Fields[field].Hoist {
			out = append(out, field)
		}
	}
	return out
}

// ParentOf returns the object that holds field in root ("create", "update", or "get"), or nil.
func (f *File) ParentOf(root, field string) *Parent {
	if f == nil || f.Hoisted == nil {
		return nil
	}
	m := map[string]map[string]Parent{"create": f.Hoisted.Create, "update": f.Hoisted.Update, "get": f.Hoisted.Get}[root]
	parent, ok := m[field]
	if !ok {
		return nil
	}
	return &parent
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
	// ElementType is the map value type: "string" or "int64".
	ElementType string `yaml:"elementType"`
	// WriteOnly: Terraform sends the value and does not store it.
	WriteOnly bool `yaml:"writeOnly"`
	// MarkdownDescription is the docs text of the attribute.
	MarkdownDescription string `yaml:"markdownDescription"`
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
	// Hoist: true shows the fields of this object field of the resource as top-level
	// attributes, as the released resource did. The API sends and returns them inside the object.
	// The lines of those fields are lines of the resource component.
	Hoist bool `yaml:"hoist"`
	// AsBool shows an object without fields as a bool, for example a oneOf arm alwaysActive: {}.
	// true sends the empty object, false is an error, and a response that has the object reads
	// as true; one without it reads as null.
	AsBool bool `yaml:"asBool"`
	// TerraformType keeps the released Terraform type of an int32 field, or of a list or set of
	// int32: Int64. Expand converts the value and reports one outside the int32 range.
	TerraformType string `yaml:"terraformType"`
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
	// Name is the Terraform attribute name of the field, when the released resource does not
	// use the name derived from the JSON name (applications for applicationnameFilters).
	Name string `yaml:"name"`
	// Deprecation is the released deprecation message.
	Deprecation string `yaml:"deprecation"`
	// Computed: true makes an optional field Optional+Computed. false makes it Optional.
	Computed *bool `yaml:"computed"`
	// UseStateForUnknown keeps the value in the state when the config has none.
	UseStateForUnknown bool `yaml:"useStateForUnknown"`
	// UseNonNullStateForUnknown keeps a non-null value of the state when the config has none.
	// A null state value stays unknown, so the server can fill it in.
	UseNonNullStateForUnknown bool `yaml:"useNonNullStateForUnknown"`
	// Default is a static default in the schema: a string, a bool, or a number.
	Default any `yaml:"default"`
	// ReadEmptyAs: "null" reads an empty list or object from the API as null.
	ReadEmptyAs string `yaml:"readEmptyAs"`
	// ReadNullAs: "empty" reads a missing list or set as an empty collection.
	ReadNullAs string `yaml:"readNullAs"`
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
	// LengthAtLeast is a released stringvalidator.LengthAtLeast.
	LengthAtLeast *int `yaml:"lengthAtLeast"`
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
	ExactlyOneOf []string `yaml:"exactlyOneOf"`
	AtLeastOneOf []string `yaml:"atLeastOneOf"`
	// AtLeast is a released AtLeast validator of a number.
	AtLeast *float64 `yaml:"atLeast"`
	// RegexMatches is a released stringvalidator.RegexMatches with its message.
	RegexMatches *RegexMatches `yaml:"regexMatches"`
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
	if f.SingletonID && (f.API.ClientSetID || f.API.ServerSetID || f.API.UpdateIDInBody) {
		return errors.New("singletonID is for a singleton, which has no id in a path: it cannot be combined with api.clientSetID, api.serverSetID, or api.updateIDInBody")
	}
	if f.SingletonID && f.FixedID != "" {
		return errors.New("fixedID and singletonID both set the id of a singleton: keep fixedID for a fixed id, or singletonID for the id that the server sets")
	}
	if err := f.checkTypes(); err != nil {
		return err
	}
	return f.checkHoist()
}

// checkHoist checks that hoist lines are on fields of the resource component: a hoisted object
// shows its fields as top-level attributes, and a nested object has none.
func (f *File) checkHoist() error {
	for _, name := range sortedKeys(f.Types) {
		if name == f.Resource {
			continue
		}
		for _, field := range sortedKeys(f.Types[name].Fields) {
			line := f.Types[name].Fields[field]
			if line.Hoist {
				return fmt.Errorf("types.%s.fields.%s: hoist applies to a field of the resource %s", name, field, f.Resource)
			}
		}
	}
	return nil
}

// checkTypes checks the type lines: the required list, the field lines, and the extra attributes.
func (f *File) checkTypes() error {
	for name, t := range f.Types {
		if t.Required != nil && len(*t.Required) != 0 {
			return fmt.Errorf("types.%s.required: only the empty list [] is allowed", name)
		}
		if t.Required == nil && len(t.Fields) == 0 && len(t.ExtraAttributes) == 0 && len(t.Groups) == 0 && len(t.UnsupportedArms) == 0 &&
			t.UnsupportedSummary == "" && t.UnsupportedDetail == "" {
			return fmt.Errorf("types.%s has no override", name)
		}
		if err := t.checkUnsupported(); err != nil {
			return fmt.Errorf("types.%s: %w", name, err)
		}
		names := map[string]string{}
		for _, field := range sortedKeys(t.Fields) {
			line := t.Fields[field]
			if err := line.check(); err != nil {
				return fmt.Errorf("types.%s.fields.%s: %w", name, field, err)
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
			if _, ok := t.Fields[field]; ok {
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
	add(l.TerraformType != "", "terraformType")
	add(l.DefaultArm, "defaultArm")
	add(l.Description != nil, "description")
	add(l.MarkdownDescription != nil, "markdownDescription")
	add(l.ReadOnly, "readOnly")
	add(l.Required, "required")
	add(l.Optional, "optional")
	add(len(l.CreateAs) != 0, "createAs")
	add(l.Collection != "", "collection")
	add(l.Name != "", "name")
	add(l.Deprecation != "", "deprecation")
	add(l.Computed != nil, "computed")
	add(l.UseStateForUnknown, "useStateForUnknown")
	add(l.UseNonNullStateForUnknown, "useNonNullStateForUnknown")
	add(l.Default != nil, "default")
	add(l.ReadEmptyAs != "", "readEmptyAs")
	add(l.ReadNullAs != "", "readNullAs")
	add(l.SendNullAsEmpty, "sendNullAsEmpty")
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
	add(l.Hoist, "hoist")
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
// contract cannot have: its own default, or no computed value. required: true needs no check:
// a server default needs a field that Get requires, and then the line is unused.
func (l Field) replacesServerDefault() bool {
	return l.Default != nil || l.Computed != nil && !*l.Computed
}

func (e ExtraAttribute) check(name string) error {
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
	if e.ElementType != "string" && e.ElementType != "int64" {
		return fmt.Errorf("elementType is %q, want string or int64", e.ElementType)
	}
	if e.MarkdownDescription == "" {
		return errors.New("markdownDescription is required")
	}
	return nil
}

func (l Field) empty() bool {
	return l.emptyMode() && l.emptyValue()
}

func (l Field) emptyMode() bool {
	return !l.Skip && !l.ReadOnly && l.Description == nil && l.MarkdownDescription == nil && !l.Required && !l.Optional && l.Name == "" && len(l.ValueNames) == 0 && !l.AsBool && l.TerraformType == "" && !l.DefaultArm && l.Deprecation == "" &&
		l.Computed == nil && !l.UseStateForUnknown && !l.UseNonNullStateForUnknown && !l.RequiresReplace && !l.Hoist
}

func (l Field) emptyValue() bool {
	return l.Default == nil && len(l.CreateAs) == 0 && l.Collection == "" && l.KeyedBy == nil && l.ReadEmptyAs == "" && l.ReadNullAs == "" && !l.RequireOne && !l.Fixed() &&
		!l.KeepPriorOrder && len(l.Validators) == 0 && l.Equality == "" && l.Unwrap == nil && l.UniqueBy == ""
}

func (l Field) check() error {
	if l.empty() {
		return errors.New("the line sets nothing")
	}
	if err := l.checkConflicts(); err != nil {
		return err
	}
	switch l.Default.(type) {
	case nil, string, bool, int, float64:
	default:
		return fmt.Errorf("default is %T, want a string, a bool, or a number", l.Default)
	}
	if l.Required && l.Computed != nil && *l.Computed {
		return errors.New("a required field cannot be computed")
	}
	if l.Required && l.Optional {
		return errors.New("set required or optional, not both")
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
	if l.TerraformType != "" && l.TerraformType != "Int64" {
		return fmt.Errorf("terraformType is %q, want \"Int64\"", l.TerraformType)
	}
	if l.RequestValue != "" && l.Name != "" {
		return errors.New("a requestValue field is no Terraform attribute, so name changes nothing")
	}
	if err := checkValueNames(l.ValueNames); err != nil {
		return err
	}
	if l.Hoist && len(l.keys()) != 1 {
		return errors.New("a hoisted field has no attribute of its own, so the line sets only hoist: put the other keys on the lines of its fields")
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
	return l.ReadOnly || l.Computed != nil || l.Required || l.Optional || len(l.CreateAs) != 0 || l.Collection != "" || l.Name != "" || l.KeyedBy != nil || l.Default != nil || l.KeepPriorOrder ||
		l.ReadEmptyAs != "" || l.ReadNullAs != "" || l.RequireOne || l.Fixed() ||
		len(l.Validators) != 0 || l.Equality != "" || l.Unwrap != nil || len(l.ValueNames) != 0 || l.AsBool || l.TerraformType != "" || l.DefaultArm ||
		l.RequiresReplace || l.UniqueBy != ""
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
		if v.AtLeast != nil {
			set++
		}
		if v.RegexMatches != nil {
			set++
			if _, err := regexp.Compile(v.RegexMatches.Pattern); err != nil || v.RegexMatches.Pattern == "" {
				return fmt.Errorf("regexMatches needs a pattern that Go compiles: %q", v.RegexMatches.Pattern)
			}
		}
		if set != 1 {
			return errors.New("each validator sets exactly one of oneOf, sizeAtLeast, lengthAtLeast, enum, enumExceptZero, between, exactlyOneOf, atLeastOneOf, atLeast, and regexMatches")
		}
	}
	return nil
}

// Policy returns the rule set for the model: the rules that the API keys state, and the
// exceptions that the Terraform keys cover.
func (f *File) Policy() model.Policy {
	p := f.Exceptions().Policy()
	p.EnumAnyPrefix = append(p.EnumAnyPrefix, sortedKeys(f.Enums)...)
	for _, name := range sortedKeys(f.Types) {
		t := f.Types[name]
		for _, field := range sortedKeys(t.Fields) {
			line := t.Fields[field]
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
		for field, line := range t.Fields {
			if e.Fields == nil {
				e.Fields = map[string]map[string]iacoverrides.Field{}
			}
			if e.Fields[name] == nil {
				e.Fields[name] = map[string]iacoverrides.Field{}
			}
			e.Fields[name][field] = line.Field
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
		out = append(out, Line{Kind: KindEnum, Component: name, EnumZero: f.Enums[name].Zero, EnumValues: f.Enums[name].Values, EnumRejected: f.Enums[name].Rejected})
	}
	for _, name := range sortedKeys(f.Types) {
		t := f.Types[name]
		if t.Required != nil {
			out = append(out, Line{Kind: KindEmptyRequired, Component: name})
		}
		for _, field := range sortedKeys(t.Fields) {
			f := t.Fields[field]
			out = append(out, Line{Kind: KindField, Component: name, Field: field, ReadOnly: f.ReadOnly, Required: f.Required, Optional: f.Optional, Collection: f.Collection, Name: f.Name, NoAttribute: t.noAttribute(), RequestValueBy: f.RequestValueBy, KeyedBy: f.KeyedBy, Accepted: file.accepted(f.KeyedBy), Equality: f.Equality, Hoist: f.Hoist, Keys: f.keys()})
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
	Hoist       bool // for a field line: the fields of the object are top-level attributes
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
