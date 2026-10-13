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
	// Unwrap lists the components with one property that the released resource shows as the
	// value of that property, for example LuceneQuery in {"luceneQuery": {"value": "error"}}. It
	// applies in every place that has no unwrap key on its field line.
	Unwrap []string `yaml:"unwrap"`
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

// API is the HTTP contract of a released resource.
type API struct {
	// RequestWrapper is the property of the Create and Update bodies that holds the resource.
	RequestWrapper string `yaml:"requestWrapper"`
	// UpdateIDInBody: Update has no id in its path. The id is a field of the resource.
	UpdateIDInBody bool `yaml:"updateIDInBody"`
	// ClientSetID: the client can send the id on Create.
	ClientSetID bool `yaml:"clientSetID"`
	// ServerSetID: the server sets the id and returns it in every response, but the Get
	// response cannot require it, because the request shares the resource schema.
	ServerSetID bool `yaml:"serverSetID"`
	// CustomMethods: Get, Update, and Delete are not the Create path with an id.
	// Get and Delete each have the same {id} path parameter on their own paths.
	// Update has no path parameter, and the id stays in the body. New resources
	// keep the stricter path rules.
	CustomMethods bool `yaml:"customMethods"`
	// Operations names the lifecycle operationIds when they do not end in
	// _Create, _Get, _Update, or _Delete plus the resource name. Generate then
	// needs no operation flags.
	Operations *Operations `yaml:"operations"`
	// Delete names the operation that removes the resource when the API has no DELETE.
	Delete *Delete `yaml:"delete"`
	// UpdateMaskInBody: PATCH updateMask is an optional JSON body property,
	// and Update leaves it out, as the released resource did. The server then
	// updates every field present in the body. Every field Update sends must be
	// required or have a static default, because without a mask removing it
	// from the configuration would not clear it.
	UpdateMaskInBody bool `yaml:"updateMaskInBody"`
	// TopLevelMaskReplacesOneOf: the server replaces the whole value of a
	// top-level mask path, including the arm of a oneOf inside it. Update then
	// sends a changed oneOf under its top-level field, for an API whose
	// query-parameter mask pattern rejects dotted paths. UpdateMaskInBody sends
	// no mask, so the key is an error with it.
	TopLevelMaskReplacesOneOf bool `yaml:"topLevelMaskReplacesOneOf"`
}

// Operations names the four lifecycle operationIds. All four are required.
type Operations struct {
	Create string `yaml:"create"`
	Get    string `yaml:"get"`
	Update string `yaml:"update"`
	Delete string `yaml:"delete"`
}

// Delete is how a released resource is removed when the API has no DELETE, for example a
// POST /things/{id}/archive. It is an object, so that a later key can add a step before the call.
type Delete struct {
	// Operation is the operationId of a POST whose path is the Get path plus one segment. It
	// takes the id path parameter of Get and no request body. Its response is ignored.
	Operation string `yaml:"operation"`
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
	// UnsupportedArms are oneOf arms the released resource does not configure.
	// Flatten reports a response that selects one, with UnsupportedSummary and
	// UnsupportedDetail. Skipping an arm is an error; this key keeps the
	// remaining arms as the group.
	UnsupportedArms    []string `yaml:"unsupportedArms"`
	UnsupportedSummary string   `yaml:"unsupportedSummary"`
	UnsupportedDetail  string   `yaml:"unsupportedDetail"`
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
	// Collection: "set" makes a list a Terraform set, although the contract does
	// not mark it x-coralogix-collection: set. The SDK type of the field does not
	// change. Delete this key when the contract marks the field.
	Collection string `yaml:"collection"`
	// Deprecation is the released deprecation message.
	Deprecation string `yaml:"deprecation"`
	// Computed: true makes an optional field Optional+Computed. false makes it Optional.
	Computed *bool `yaml:"computed"`
	// UseStateForUnknown keeps the value in the state when the config has none.
	UseStateForUnknown bool `yaml:"useStateForUnknown"`
	// Default is a static default in the schema: a string, a bool, or a number.
	Default any `yaml:"default"`
	// ReadEmptyAs: "null" reads an empty list or object from the API as null.
	ReadEmptyAs string `yaml:"readEmptyAs"`
	// ReadNullAs: "empty" reads a missing list or set as an empty collection.
	ReadNullAs string `yaml:"readNullAs"`
	// RequireOne makes a oneOf that allows no arm use ExactlyOneOf. The released
	// resource required one arm even though the contract allows none.
	RequireOne bool `yaml:"requireOne"`
	// RequestValue is an API value that Create and Update always send. The field
	// is not a Terraform attribute.
	RequestValue string `yaml:"requestValue"`
	// KeepPriorOrder returns the list items in the order of the plan or state, because
	// the API does not keep the order.
	KeepPriorOrder bool `yaml:"keepPriorOrder"`
	// Validators are the released validators of the field.
	Validators []Validator `yaml:"validators"`
	// Equality: "yaml" or "json" compares a string field as a YAML or JSON document. The API
	// returns the document normalized, so a change of format alone must not plan a change.
	Equality string `yaml:"equality"`
	// Unwrap: true shows the value inside the wrapper of the field, or of its items, in this
	// place. false keeps the object of a component that the unwrap list names.
	Unwrap *bool `yaml:"unwrap"`
	// RequiresReplace: a change replaces the resource, although Update accepts
	// the field. Only a top-level field of the resource can set it. It cannot
	// be combined with computed: true: an omitted Optional+Computed value is
	// planned unknown, and RequiresReplace then replaces the resource.
	RequiresReplace bool `yaml:"requiresReplace"`
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
	// Rejected are the values of the contract that the resource does not accept. The generator
	// reports a value of the contract that is in neither list, so a new API value needs a decision.
	Rejected []string `yaml:"rejected"`
	// ReadRejected: a rejected value still reads back, spelled as the accepted values are (lower
	// case unless Verbatim). The write map and the schema validator do not accept it. Without
	// this key, a rejected value fails the read.
	ReadRejected bool `yaml:"readRejected"`
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
	if err := f.checkUnwrap(); err != nil {
		return err
	}
	if err := f.API.check(); err != nil {
		return err
	}
	return f.checkTypes()
}

// checkTypes checks the type lines: the required list, the field lines, and the extra attributes.
func (f *File) checkTypes() error {
	for name, t := range f.Types {
		if t.Required != nil && len(*t.Required) != 0 {
			return fmt.Errorf("types.%s.required: only the empty list [] is allowed", name)
		}
		if err := t.check(name); err != nil {
			return err
		}
		for field, line := range t.Fields {
			if err := line.check(); err != nil {
				return fmt.Errorf("types.%s.fields.%s: %w", name, field, err)
			}
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

// check checks that a Delete override names its operation, and that only one line names it.
func (a API) check() error {
	if a.ClientSetID && a.ServerSetID {
		return errors.New("api.clientSetID and api.serverSetID both describe the id: keep the one that states who sets it")
	}
	if a.Delete != nil && a.Delete.Operation == "" {
		return errors.New("api.delete.operation is required")
	}
	if a.Operations == nil {
		return nil
	}
	if a.Operations.Create == "" || a.Operations.Get == "" || a.Operations.Update == "" || a.Operations.Delete == "" {
		return errors.New("api.operations needs create, get, update, and delete")
	}
	if a.Delete != nil {
		return errors.New("api.delete and api.operations both name the Delete operation: keep only api.operations.delete")
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

// checkUnwrap checks that the unwrap list names each component once.
func (f *File) checkUnwrap() error {
	for i, name := range f.Unwrap {
		if name == "" {
			return fmt.Errorf("unwrap[%d] is empty", i)
		}
		if slices.Contains(f.Unwrap[:i], name) {
			return fmt.Errorf("unwrap names %s twice", name)
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
	add(l.ReadNullAs != "", "readNullAs")
	add(l.RequireOne, "requireOne")
	add(l.RequestValue != "", "requestValue")
	add(l.KeepPriorOrder, "keepPriorOrder")
	add(len(l.Validators) != 0, "validators")
	add(l.Equality != "", "equality")
	add(l.Unwrap != nil, "unwrap")
	add(l.RequiresReplace, "requiresReplace")
	add(l.Collection != "", "collection")
	return keys
}

// statesPresence reports whether the line says how an omitted value behaves. A line that only
// changes a text, a validator, or the order does not: the contract must still state presence.
func (l Field) statesPresence() bool {
	return l.Skip || l.ReadOnly || l.Required || l.Computed != nil || l.Default != nil || l.ReadEmptyAs != "" || l.ReadNullAs != "" || l.RequestValue != ""
}

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

func (t Type) check(name string) error {
	hasUnsupported := len(t.UnsupportedArms) != 0 || t.UnsupportedSummary != "" || t.UnsupportedDetail != ""
	if t.Required == nil && len(t.Fields) == 0 && len(t.ExtraAttributes) == 0 && !hasUnsupported {
		return fmt.Errorf("types.%s has no override", name)
	}
	if !hasUnsupported {
		return nil
	}
	if len(t.UnsupportedArms) == 0 || t.UnsupportedSummary == "" || t.UnsupportedDetail == "" {
		return fmt.Errorf("types.%s: unsupportedArms, unsupportedSummary, and unsupportedDetail are set together", name)
	}
	seen := map[string]bool{}
	for _, arm := range t.UnsupportedArms {
		if arm == "" || seen[arm] {
			return fmt.Errorf("types.%s.unsupportedArms: %q is empty or repeated", name, arm)
		}
		seen[arm] = true
		if _, ok := t.Fields[arm]; ok {
			// The arm is not in the schema, so a line on it would change nothing.
			return fmt.Errorf("types.%s.unsupportedArms: %q also has a field line", name, arm)
		}
	}
	return nil
}

func (l Field) empty() bool {
	return l.emptyMode() && l.emptyValue()
}

func (l Field) emptyMode() bool {
	return !l.Skip && !l.ReadOnly && l.Description == nil && l.MarkdownDescription == nil && !l.Required && l.Deprecation == "" &&
		l.Computed == nil && !l.UseStateForUnknown
}

func (l Field) emptyValue() bool {
	return l.Default == nil && l.ReadEmptyAs == "" && l.ReadNullAs == "" && !l.RequireOne && l.RequestValue == "" &&
		!l.KeepPriorOrder && len(l.Validators) == 0 && l.Equality == "" && l.Unwrap == nil && !l.RequiresReplace &&
		l.Collection == ""
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
	if l.RequiresReplace && l.Computed != nil && *l.Computed {
		return errors.New("requiresReplace cannot be combined with computed: true")
	}
	if l.Description != nil && l.MarkdownDescription != nil {
		return errors.New("set description or markdownDescription, not both")
	}
	if l.Skip && l.hasBehavior() {
		return errors.New("a skipped field has no other behavior")
	}
	return checkValidators(l.Validators)
}

func (l Field) checkConflicts() error {
	if l.Collection != "" && l.Collection != "set" {
		return fmt.Errorf("collection is %q, want \"set\"", l.Collection)
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
	if l.Skip && l.RequestValue != "" {
		return errors.New("requestValue sends the field, so skip cannot be set")
	}
	if l.Equality != "" && l.Equality != EqualityYAML && l.Equality != EqualityJSON {
		return fmt.Errorf("equality is %q, want %q or %q", l.Equality, EqualityYAML, EqualityJSON)
	}
	return nil
}

// hasBehavior reports whether the line sets a behavior that a skipped field cannot have.
func (l Field) hasBehavior() bool {
	return l.ReadOnly || l.Computed != nil || l.Required || l.Default != nil || l.KeepPriorOrder ||
		l.ReadEmptyAs != "" || l.ReadNullAs != "" || l.RequireOne || l.RequestValue != "" ||
		len(l.Validators) != 0 || l.Equality != "" || l.Unwrap != nil || l.RequiresReplace || l.Collection != ""
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
		if set != 1 {
			return errors.New("each validator sets exactly one of oneOf, sizeAtLeast, lengthAtLeast, enum, and enumExceptZero")
		}
	}
	return nil
}

// Policy returns the rule set for the model.
func (f *File) Policy() model.Policy {
	p := model.Policy{
		Existing:                  true,
		RequestWrapper:            f.API.RequestWrapper,
		UpdateIDInBody:            f.API.UpdateIDInBody,
		ClientSetID:               f.API.ClientSetID,
		ServerSetID:               f.API.ServerSetID,
		CustomMethods:             f.API.CustomMethods,
		UpdateMaskInBody:          f.API.UpdateMaskInBody,
		TopLevelMaskReplacesOneOf: f.API.TopLevelMaskReplacesOneOf,
		// Parse checked that the file states validators.inferred.
		NoInferredValidators: !*f.Validators.Inferred,
		RejectUnknown:        f.Validators.RejectUnknown,
	}
	if f.API.Operations != nil {
		p.Operations = model.OperationIDs{
			Create: f.API.Operations.Create,
			Get:    f.API.Operations.Get,
			Update: f.API.Operations.Update,
			Delete: f.API.Operations.Delete,
		}
	}
	if f.API.Delete != nil {
		p.DeleteOperation = f.API.Delete.Operation
	}
	p.EnumAnyPrefix = append(p.EnumAnyPrefix, sortedKeys(f.Enums)...)
	p.Unwrap = slices.Clone(f.Unwrap)
	for _, name := range sortedKeys(f.Types) {
		t := f.Types[name]
		for _, field := range sortedKeys(t.Fields) {
			if on := t.Fields[field].Unwrap; on != nil {
				if p.UnwrapFields == nil {
					p.UnwrapFields = map[string]bool{}
				}
				p.UnwrapFields[name+"."+field] = *on
			}
			if t.Fields[field].Skip {
				p.Skip = append(p.Skip, name+"."+field)
			}
			if t.Fields[field].RequestValue != "" {
				p.FixedRequest = append(p.FixedRequest, name+"."+field)
			}
			if t.Fields[field].ReadOnly {
				p.ReadOnly = append(p.ReadOnly, name+"."+field)
			}
			if t.Fields[field].statesPresence() {
				p.Released = append(p.Released, name+"."+field)
			}
			if t.Fields[field].replacesServerDefault() {
				p.Defaults = append(p.Defaults, name+"."+field)
			}
			if t.Fields[field].RequiresReplace {
				p.RequiresReplace = append(p.RequiresReplace, name+"."+field)
			}
			if t.Fields[field].Collection == "set" {
				p.Sets = append(p.Sets, name+"."+field)
			}
		}
		for _, arm := range t.UnsupportedArms {
			p.Unsupported = append(p.Unsupported, name+"."+arm)
		}
		if t.UnsupportedSummary != "" {
			if p.UnsupportedSummary == nil {
				p.UnsupportedSummary = map[string]string{}
			}
			p.UnsupportedSummary[name] = t.UnsupportedSummary
		}
		if t.UnsupportedDetail != "" {
			if p.UnsupportedDetail == nil {
				p.UnsupportedDetail = map[string]string{}
			}
			p.UnsupportedDetail[name] = t.UnsupportedDetail
		}
	}
	return p
}

// Lines returns one description per line of the file that the contract must match,
// in a stable order. The generator uses it to report a line that matches nothing.
func (f *File) Lines() []Line {
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
			out = append(out, Line{Kind: KindField, Component: name, Field: field, ReadOnly: f.ReadOnly, Required: f.Required, Collection: f.Collection, Equality: f.Equality, Keys: f.keys()})
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
	EnumZero   string   // for an enum line: the Terraform value of the zero value
	// EnumRejected are the values of the contract that the resource does not accept.
	EnumRejected []string
	ReadOnly     bool     // for a field line: the file says that the server sets the field
	Equality     string   // for a field line: "yaml" or "json" when the line compares the value as a document
	Required     bool     // for a field line: the file says that the field is required
	Collection   string   // for a field line: "set" says that the list is unordered
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
