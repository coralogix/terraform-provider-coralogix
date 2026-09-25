// Package model is the target-neutral model of one API resource. It has no
// Terraform and no SDK types. Build reads it from an OpenAPI document.
package model

// Resource is one API resource with Create, Get, Update, and Delete operations.
type Resource struct {
	Name   string // component name of the resource schema, for example "AiEvaluation"
	Create Operation
	Get    Operation
	Update Operation
	Delete Operation
	// IDParam is the path parameter of Get, Update, and Delete. The Get
	// response has a field with the same name.
	IDParam string
	// UpdateMask is the Update body property that holds the update mask.
	// It is not a resource field.
	UpdateMask string
	// UpdateMaskPattern is the "pattern" of the update mask string, "" when
	// the spec has none. It shows which mask paths the API accepts.
	UpdateMaskPattern string
	// Groups are the oneOf groups among the top-level fields.
	Groups []OneOfGroup
	// Fields are the top-level resource fields: the Get fields in spec order,
	// then Create-only and Update-only fields (Build rejects those).
	Fields []*ResourceField
}

// Operation is one HTTP operation of the resource.
type Operation struct {
	Method      string // upper case, for example "POST"
	Path        string
	OperationID string
	Body        string // "" (no body), "inline", or a component name
	Response    Response
}

// Response is the 200 response body.
type Response struct {
	Schema string // component name
	// Field is the property that wraps the resource, for example "aiEvaluation".
	// It is empty when the response does not return the resource.
	Field string
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
	InGet       bool
}

// Field is a field of a nested object, or an arm of a oneOf.
type Field struct {
	Name        string
	Description string
	Attrs       Attrs
	Type        *Type
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
	Arms      []string // field names
	AllowNone bool     // the value can have no arm set
}

// Type is the type of a field.
type Type struct {
	Kind   Kind
	Schema string // component name, "" for an inline schema
	Format string // for example "double", "uint64", "date-time"
	// WireString is true for an integer that JSON sends as a string
	// (protobuf 64-bit numbers).
	WireString bool
	Values     []string // Enum: the values, without *_UNSPECIFIED
	Elem       *Type    // List, Set, Map
	Fields     []*Field // Object: the properties; OneOf: the arms
	AllowNone  bool     // OneOf: the value can have no arm set
	// Groups are the oneOf groups of an Object that also has normal fields,
	// or has more than one group. Each arm is one of Fields.
	Groups []OneOfGroup
	// Discriminator is the string field that names the set arm (an OpenAPI
	// discriminator with no mapping). It is also a normal field (F36).
	Discriminator string

	MinLength, MaxLength *int64
	Minimum, Maximum     *float64
	MinItems, MaxItems   *int64
}
