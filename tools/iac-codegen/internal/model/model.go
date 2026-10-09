// Package model is the target-neutral model of one API resource. It has no
// Terraform and no SDK types. Build reads it from an OpenAPI document.
package model

import "strings"

// OperationIDs selects lifecycle operations when suffix discovery is ambiguous.
// Empty fields use deterministic suffix discovery.
type OperationIDs struct {
	Create string
	Get    string
	Update string
	Delete string
}

// Resource is one API resource with Create, Get, Update, and Delete operations.
type Resource struct {
	Name   string // component name of the resource schema
	Create Operation
	Get    Operation
	Update Operation
	Delete Operation
	// IDParam is the path parameter of Get, Update, and Delete. The Get
	// response has a field with the same name. It is "" for a singleton.
	IDParam string
	// IDType is the type shared by the path parameter and response ID field.
	// It is nil for a singleton.
	IDType *Type
	// Singleton is true when Get has no path parameter: there is one
	// resource per company (D18). Create, Get, Update, and Delete use one
	// path.
	Singleton bool
	// Replace is true when Update is a full replace (PUT, E11): the body has
	// every Update field, and the server clears a field that the body does
	// not have. It has no update mask. False: PATCH with an update mask.
	Replace bool
	// UpdateMask is the update-mask name: the query parameter, or the JSON body
	// property when MaskInBody is set. It is "" for a full replace. It is not
	// a resource field.
	UpdateMask string
	// UpdateMaskPattern is the "pattern" of the update mask string, "" when
	// the spec has none. It shows which mask paths the API accepts.
	UpdateMaskPattern string
	// MaskInBody: PATCH updateMask is an optional JSON body property. Update
	// sets it to the same changed-field mask a query parameter would send.
	MaskInBody bool
	// Policy is the rule set that Build used. It is the zero value for a new resource.
	Policy Policy
	// Groups are the oneOf groups among the top-level fields.
	Groups []OneOfGroup
	// Fields are the top-level resource fields: the Get fields in spec order,
	// then Create-only and Update-only fields (Build rejects those).
	Fields []*ResourceField
	// UnwrapUsed are the unwrap lines of the policy that decided a place: "unwrap.<Component>"
	// for a component of the list, and "<Component>.<field>" for a field line.
	UnwrapUsed map[string]bool
	// HiddenWrappers are the wrapper components that collapse in every place, so Terraform has
	// no attribute for their property.
	HiddenWrappers map[string]bool
}

// Operation is one HTTP operation of the resource.
type Operation struct {
	Method      string // upper case, for example "POST"
	Path        string
	OperationID string
	Body        string // "" (no body), "inline", or a component name
	// BodyTitle is the "title" of an inline body, "" when it has none.
	// BodyTitleIsComponent: a component schema has the same name.
	BodyTitle            string
	BodyTitleIsComponent bool
	Response             Response
}

// Response is the 200 response body.
type Response struct {
	Schema string // component name, "" for an inline empty object
	// Field is the property that wraps the resource, for example "aiEvaluation".
	// It is empty when the response does not return the resource, or is the
	// resource itself (Direct).
	Field string
	// Direct is true when the response is the resource itself, with no
	// wrapper (proto google.api.http response_body).
	Direct bool
	// Empty is true when the response schema has no fields, for example many
	// Delete responses. The SDK then returns map[string]interface{}.
	Empty bool
}

// Behavior is how a top-level field is managed. See Classify.
type Behavior string

const (
	Normal    Behavior = "normal"    // Create, Update, Get
	Immutable Behavior = "immutable" // Create, Get: a change needs a new resource
	Computed  Behavior = "computed"  // Get only: read and store, never send
)

// ResourceField is a top-level field of the resource.
type ResourceField struct {
	Name        string
	Description string // from the Get schema
	Type        *Type
	Behavior    Behavior
	Create      *Attrs // nil: not in the Create body
	Update      *Attrs // nil: not in the Update body
	Get         *Attrs // attributes in the resource response
	InGet       bool
}

// Field is a field of a nested object, or an arm of a oneOf.
type Field struct {
	Name        string
	Description string
	Attrs       Attrs // attributes in the resource response
	Type        *Type
	// Behavior is how the field is managed inside its parent object, as for a
	// top-level field. It is "" inside a computed value: the whole value is
	// computed.
	Behavior Behavior
	// Create and Update are the attributes of the field in the request type of
	// its parent object. nil: that request does not have the field.
	Create, Update *Attrs
}

// Attrs are the facts about a field in one place (a request body or an object).
type Attrs struct {
	Required bool    // in the parent "required" list
	Presence bool    // x-coralogix-presence: true
	Default  *string // YAML text of "default", nil when there is none
}

// Kind is the kind of value a Type holds.
type Kind string

const (
	String  Kind = "string"
	Bool    Kind = "bool"
	Number  Kind = "number"
	Integer Kind = "integer"
	Enum    Kind = "enum"
	Object  Kind = "object"
	OneOf   Kind = "oneOf" // an object with exactly one arm set
	List    Kind = "list"  // ordered
	Set     Kind = "set"   // unordered, unique items
	Map     Kind = "map"   // string keys; Elem is the value type
)

// OneOfGroup is a set of fields of an object of which at most one is set.
type OneOfGroup struct {
	Arms []string // field names
	// AllowNone: the value can have no arm set. When Create sends the value,
	// it is the Create rule, which the Terraform configuration follows.
	AllowNone bool
}

// Type is the type of a field.
type Type struct {
	Kind   Kind
	Schema string // component name, "" for an inline schema
	// CreateSchema and UpdateSchema are the components of an Object or a
	// OneOf in the Create and Update requests. They can differ from Schema,
	// the component in the resource response. "" when the request does not
	// send the value.
	CreateSchema, UpdateSchema string
	// Model is the component of the resource response that holds the
	// Terraform model of a request type. It is set only on the types that
	// CreateType and UpdateType return.
	Model  string
	Format string // for example "double", "uint64", "date-time"
	// WireString is true for an integer that JSON sends as a string
	// (protobuf 64-bit numbers).
	WireString bool
	Values     []string // Enum: the business values, without EnumZero
	EnumZero   string   // Enum: the exact protobuf zero value
	Elem       *Type    // List, Set, Map
	Fields     []*Field // Object: the properties; OneOf: the arms
	AllowNone  bool     // OneOf: the value can have no arm set, as in OneOfGroup
	// Groups are the oneOf groups of an Object that also has normal fields,
	// or has more than one group. Each arm is one of Fields.
	Groups []OneOfGroup
	// Unsupported are oneOf arms the resource does not configure. Flatten
	// reports a response that selects one.
	Unsupported                           []*Field
	UnsupportedSummary, UnsupportedDetail string
	// Discriminator is the string field that names the set arm (an OpenAPI
	// discriminator with no mapping). It is also a normal field (F36).
	Discriminator string

	MinLength, MaxLength *int64
	// Pattern is the regular expression that a String must match, "" for
	// none. The free-text pattern is none. Existing resources have none.
	Pattern            string
	Minimum, Maximum   *float64
	MinItems, MaxItems *int64 // List, Set: items. Map: entries (minProperties, maxProperties).
	// Wrappers are the one-property objects that hold the value in the API, outer first. The
	// rest of the type is the value inside them, which Terraform shows. They are set only by
	// the unwrap lines of a behavior-overrides file.
	Wrappers []Wrapper
}

// Wrapper is one object with one property that holds a value in the API, for example
// LuceneQuery in {"luceneQuery": {"value": "error"}}. Terraform shows only the value.
type Wrapper struct {
	// Schema is the component in the resource response. CreateSchema and UpdateSchema are the
	// components in the requests, "" when that request does not send the value.
	Schema, CreateSchema, UpdateSchema string
	// Field is the one property that holds the value.
	Field string
	// Attrs are the attributes of Field in the resource response. Create and Update are its
	// attributes in the requests, nil when that request does not send the value.
	Attrs          Attrs
	Create, Update *Attrs
}

// WrapperPath returns the API path from the outer wrapper to the value, for example
// "filter.value", or "" when the value has no wrapper.
func (t *Type) WrapperPath() string {
	names := make([]string, 0, len(t.Wrappers))
	for _, w := range t.Wrappers {
		names = append(names, w.Field)
	}
	return strings.Join(names, ".")
}

// Unwrapped returns the type of the value inside the wrappers. It is t when t has none.
func (t *Type) Unwrapped() *Type {
	if len(t.Wrappers) == 0 {
		return t
	}
	out := *t
	out.Wrappers = nil
	return &out
}
