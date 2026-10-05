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
	"slices"
	"sort"
	"strings"

	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/model"
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
	// limits, and a new validator would reject configs that work today. Only false is
	// supported. The validators of the released resource are written as field lines.
	Inferred *bool `yaml:"inferred"`
}

// API is the HTTP contract of a released resource.
type API struct {
	// RequestWrapper is the property of the Create and Update bodies that holds the resource.
	RequestWrapper string `yaml:"requestWrapper"`
	// UpdateIDInBody: Update has no id in its path. The id is a field of the resource.
	UpdateIDInBody bool `yaml:"updateIDInBody"`
	// ClientSetID: the client can send the id on Create.
	ClientSetID bool `yaml:"clientSetID"`
}

// Type overrides one OpenAPI object component.
type Type struct {
	// Required: [] says that the object declares no required list and means
	// "no field is required". Only the empty list is allowed.
	Required *[]string `yaml:"required"`
	// Fields overrides single fields of the object.
	Fields map[string]Field `yaml:"fields"`
}

// Field overrides one field of an object. A field with a line keeps the released
// behavior that the line states. A field without a line follows the contract.
type Field struct {
	// Skip: the resource does not manage the field. It is not sent and not stored.
	Skip bool `yaml:"skip"`
	// Description and MarkdownDescription are the released texts of the docs. An empty
	// string means that the released field had no description.
	Description         *string `yaml:"description"`
	MarkdownDescription *string `yaml:"markdownDescription"`
	// ReadOnly: true says that the server sets the field, although the contract does not mark
	// it readOnly yet. The field is read and stored, and never sent. Delete this key when the
	// contract marks the field readOnly.
	ReadOnly bool `yaml:"readOnly"`
	// Required: true makes the Terraform attribute Required, although the contract does not
	// require the field. The SDK type of the field does not change. Delete this key when the
	// contract requires the field.
	Required bool `yaml:"required"`
	// Deprecation is the released deprecation message.
	Deprecation string `yaml:"deprecation"`
	// Computed: true makes an optional field Optional+Computed. false makes it Optional.
	Computed *bool `yaml:"computed"`
	// UseStateForUnknown keeps the value in the state when the config has none.
	UseStateForUnknown bool `yaml:"useStateForUnknown"`
	// Default is a static default in the schema (a string or a bool).
	Default any `yaml:"default"`
	// ReadEmptyAs: "null" reads an empty list or object from the API as null.
	ReadEmptyAs string `yaml:"readEmptyAs"`
	// KeepPriorOrder returns the list items in the order of the plan or state, because
	// the API does not keep the order.
	KeepPriorOrder bool `yaml:"keepPriorOrder"`
	// Validators are the released validators of the field.
	Validators []Validator `yaml:"validators"`
}

// Validator is one released validator. Exactly one key is set.
type Validator struct {
	OneOf       []string `yaml:"oneOf"`
	SizeAtLeast *int     `yaml:"sizeAtLeast"`
	// Enum: true accepts the Terraform values of the enum of the field, as the enums line of
	// the enum states them. The list of values has one source, so it cannot drift.
	Enum bool `yaml:"enum"`
}

// Enum overrides one enum component.
type Enum struct {
	// Zero is the Terraform value of the protobuf zero value (ENTITY_TYPE_UNSPECIFIED).
	// A new resource uses null for it. A released resource kept a value in its state.
	Zero string `yaml:"zero"`
	// Values are the enum values that the resource accepts. The Terraform value is the
	// lower case of the API value.
	Values []string `yaml:"values"`
	// Rejected are the values of the contract that the resource does not accept. The generator
	// reports a value of the contract that is in neither list, so a new API value needs a decision.
	Rejected []string `yaml:"rejected"`
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
	if f.Validators.Inferred == nil || *f.Validators.Inferred {
		return errors.New("validators.inferred: false is required: a released resource does not turn contract limits into validators")
	}
	if err := f.checkEnums(); err != nil {
		return err
	}
	for name, t := range f.Types {
		if t.Required != nil && len(*t.Required) != 0 {
			return fmt.Errorf("types.%s.required: only the empty list [] is allowed", name)
		}
		if t.Required == nil && len(t.Fields) == 0 {
			return fmt.Errorf("types.%s has no override", name)
		}
		for field, line := range t.Fields {
			if err := line.check(); err != nil {
				return fmt.Errorf("types.%s.fields.%s: %w", name, field, err)
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

// checkEnums checks that no value is accepted and rejected at the same time.
func (f *File) checkEnums() error {
	for _, name := range sortedKeys(f.Enums) {
		for _, v := range f.Enums[name].Rejected {
			if slices.Contains(f.Enums[name].Values, v) {
				return fmt.Errorf("enums.%s: %q is in values and in rejected", name, v)
			}
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
	add(l.Description != nil, "description")
	add(l.MarkdownDescription != nil, "markdownDescription")
	add(l.ReadOnly, "readOnly")
	add(l.Required, "required")
	add(l.Deprecation != "", "deprecation")
	add(l.Computed != nil, "computed")
	add(l.UseStateForUnknown, "useStateForUnknown")
	add(l.Default != nil, "default")
	add(l.ReadEmptyAs != "", "readEmptyAs")
	add(l.KeepPriorOrder, "keepPriorOrder")
	add(len(l.Validators) != 0, "validators")
	return keys
}

// statesPresence reports whether the line says how an omitted value behaves. A line that only
// changes a text, a validator, or the order does not: the contract must still state presence.
func (l Field) statesPresence() bool {
	return l.Skip || l.ReadOnly || l.Required || l.Computed != nil || l.Default != nil || l.ReadEmptyAs != ""
}

func (l Field) empty() bool {
	return !l.Skip && !l.ReadOnly && l.Description == nil && l.MarkdownDescription == nil && !l.Required && l.Deprecation == "" &&
		l.Computed == nil && !l.UseStateForUnknown && l.Default == nil && l.ReadEmptyAs == "" &&
		!l.KeepPriorOrder && len(l.Validators) == 0
}

func (l Field) check() error {
	if l.empty() {
		return errors.New("the line sets nothing")
	}
	if l.ReadEmptyAs != "" && l.ReadEmptyAs != "null" {
		return fmt.Errorf("readEmptyAs is %q, want \"null\"", l.ReadEmptyAs)
	}
	switch l.Default.(type) {
	case nil, string, bool:
	default:
		return fmt.Errorf("default is %T, want a string or a bool", l.Default)
	}
	if l.Required && l.Computed != nil && *l.Computed {
		return errors.New("a required field cannot be computed")
	}
	if l.Description != nil && l.MarkdownDescription != nil {
		return errors.New("set description or markdownDescription, not both")
	}
	if l.Skip && l.hasBehavior() {
		return errors.New("a skipped field has no other behavior")
	}
	return checkValidators(l.Validators)
}

// hasBehavior reports whether the line sets a behavior that a skipped field cannot have.
func (l Field) hasBehavior() bool {
	return l.ReadOnly || l.Computed != nil || l.Required || l.Default != nil || l.KeepPriorOrder ||
		l.ReadEmptyAs != "" || len(l.Validators) != 0
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
		if v.Enum {
			set++
		}
		if set != 1 {
			return errors.New("each validator sets exactly one of oneOf, sizeAtLeast, and enum")
		}
	}
	return nil
}

// Policy returns the rule set for the model.
func (f *File) Policy() model.Policy {
	p := model.Policy{
		Existing:       true,
		RequestWrapper: f.API.RequestWrapper,
		UpdateIDInBody: f.API.UpdateIDInBody,
		ClientSetID:    f.API.ClientSetID,
		// The file states validators.inferred: false; Parse checked it.
		NoInferredValidators: true,
	}
	p.EnumAnyPrefix = append(p.EnumAnyPrefix, sortedKeys(f.Enums)...)
	for _, name := range sortedKeys(f.Types) {
		t := f.Types[name]
		if t.Required != nil {
			p.EmptyRequired = append(p.EmptyRequired, name)
		}
		for _, field := range sortedKeys(t.Fields) {
			if t.Fields[field].Skip {
				p.Skip = append(p.Skip, name+"."+field)
			}
			if t.Fields[field].ReadOnly {
				p.ReadOnly = append(p.ReadOnly, name+"."+field)
			}
			if t.Fields[field].statesPresence() {
				p.Released = append(p.Released, name+"."+field)
			}
		}
	}
	return p
}

// Lines returns one description per line of the file that the contract must match,
// in a stable order. The generator uses it to report a line that matches nothing.
func (f *File) Lines() []Line {
	var out []Line
	for _, name := range sortedKeys(f.Enums) {
		out = append(out, Line{Kind: KindEnum, Component: name, EnumValues: f.Enums[name].Values, EnumRejected: f.Enums[name].Rejected})
	}
	for _, name := range sortedKeys(f.Types) {
		t := f.Types[name]
		if t.Required != nil {
			out = append(out, Line{Kind: KindEmptyRequired, Component: name})
		}
		for _, field := range sortedKeys(t.Fields) {
			f := t.Fields[field]
			out = append(out, Line{Kind: KindField, Component: name, Field: field, ReadOnly: f.ReadOnly, Required: f.Required, Keys: f.keys()})
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
)

// Line is one place in the contract that the file refers to.
type Line struct {
	Kind       Kind
	Component  string
	Field      string
	EnumValues []string // for an enum line: the values that the resource accepts
	// EnumRejected are the values of the contract that the resource does not accept.
	EnumRejected []string
	ReadOnly     bool     // for a field line: the file says that the server sets the field
	Required     bool     // for a field line: the file says that the field is required
	Keys         []string // for a field line: every key that the line sets
}

// String names the line as it is written in the file.
func (l Line) String() string {
	switch l.Kind {
	case KindEnum:
		return "enums." + l.Component
	case KindEmptyRequired:
		return "types." + l.Component + ".required"
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
