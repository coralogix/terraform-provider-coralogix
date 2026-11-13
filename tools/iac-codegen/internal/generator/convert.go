package generator

import (
	"errors"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/coralogix/coralogix-management-sdk/iac/model"
	"github.com/coralogix/coralogix-management-sdk/iac/sdkcheck"
	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/overrides"
)

// convData is the template data for expand (Terraform model → SDK request)
// and flatten (SDK response → Terraform model). Every SDK name comes from
// the checked sdkRefs.
type convData struct {
	// ExtraEquality are the kinds ("yaml", "json") of the equality lines of extra attributes. They
	// are no API field, so no field of an object carries them.
	ExtraEquality []string
	// IDGuard is the model field of the resource id when flatten must check that the response
	// has it, or "".
	IDGuard  string
	IDString bool   // a string id must also be non-empty
	SDKPkg   string // import path of the SDK package
	SDKName  string // package name
	// The root objects: the Create body and the Update body (expand), and
	// the resource (flatten).
	Create, Update, Resource *convObject
	Objects                  []*convObject // all objects, roots first
	// MaskFields are the Update fields, in model order. The update mask
	// lists the API names of the fields that changed (D8).
	MaskFields []*maskField
	// CreateFields are the Create fields checked for unknown values before
	// the generated resource calls the API.
	CreateFields []*maskField
	// LeafMask is true when the spec pattern of the update mask accepts
	// dotted paths. Then the mask names the changed leaves (contract 2.1).
	LeafMask bool
	// MaskGroups are the oneOf groups among the Update fields, as Terraform
	// names.
	MaskGroups [][]string
	// Replace is true when Update is a full replace (PUT, E11). It has no
	// update mask. UpdateFields are the Terraform names of the Update fields:
	// when none of them changed, Update sends no request (D14).
	Replace      bool
	UpdateFields []*maskField
	// OneOfArms are the Terraform paths of the oneOf arms, with list and map
	// steps left out. An empty arm selects that arm, so state keeps it apart
	// from a missing arm (contract 2.8).
	OneOfArms []string
	// CreateBody and UpdateBody are the SDK request body types. They are the SDK
	// types of the roots, unless the body wraps the resource (CreateWrap, UpdateWrap).
	CreateBody, UpdateBody string
	// CreateWrap and UpdateWrap are set when the request body has one field that holds
	// the resource. The roots are then the resource type, and expand wraps them.
	CreateWrap, UpdateWrap *bodyWrap
	// EnumMaps map the Terraform values of an enum to its API values, and back.
	EnumMaps []*enumMap
	// WrapHelpers are the functions that put a value in its API wrapper objects and take it out.
	WrapHelpers []*wrapHelper
	// ItemProjections are the functions that take the strings out of the items of a response list.
	ItemProjections []*itemProjection
	// KeyedObjects are the local structs of keyedBy objects, with the functions that turn them
	// into the API list and back.
	KeyedObjects []*keyedObject
	// AcceptUnknown: the resource has users. An unknown planned value of a computed attribute
	// is left out of the request, and the server supplies it, as the released resource did.
	AcceptUnknown bool
	// UpdateMaskField is the SDK field of the JSON-body update mask.
	// UpdateMaskPointer is true when that field is *string.
	UpdateMaskField   string
	UpdateMaskPointer bool
	// Arms are the inlined oneOf arms of the resource response. A response without one has no
	// resource: flatten returns an error, and Read removes the resource from the state.
	Arms []convArm
	// RootUnsupported are the oneOf arms of the resource itself that it does not configure
	// (unsupportedArms). flatten reports a response that selects one before it checks Arms.
	RootUnsupported []unsupportedArm
	// Distributions are the lists that distributeTo lines moved into the items of a component.
	Distributions []*convDistribution
}

// convArm is an inlined object of the resource response that the SDK holds in a pointer.
type convArm struct {
	SDK string // Go field of the resource type
	API string // API property
}

// convParent is an inlined object of a request body that the SDK holds in a pointer field. Expand
// sets it before the fields that it holds, so they are always sent.
type convParent struct {
	SDK  string // Go field of the body
	Type string // qualified SDK type of the object
	// DropEmpty: the object is optional in the request, so expand leaves it out when no field in
	// it is set, instead of sending an empty object that an API can read as a value.
	DropEmpty bool
}

// itemProjection is the generated function that takes the strings out of the objects that the
// response holds for a list that the resource shows as the strings of one property.
type itemProjection struct {
	Func    string // the function name
	Item    string // qualified SDK type of one item
	Field   string // the Go field of the item that holds the string
	Pointer bool   // the field is a pointer
}

// wrapHelper is one pair of generated functions, wrap<Func> and unwrap<Func>. They convert a value
// to the API wrapper objects that hold it, and back: one value, or each item of a list.
type wrapHelper struct {
	Func  string
	Outer string // qualified Go type of the wrapped value
	Inner string // qualified Go type of the value
	// One value: Build wraps the non-nil value v. Unwrap returns nil when NilCheck holds, else Read.
	Build, NilCheck, Read string
	// Items: Item is the Func of the helper of one item, and ItemValue the Go type of one value.
	Item, ItemValue string
}

// fieldWrap is the conversion of a field whose value the API holds in wrapper objects. Expand
// converts the Terraform value to Inner as for a field without wrappers, then calls the wrap
// helpers. Flatten calls the unwrap helpers, then converts the value.
type fieldWrap struct {
	Inner string   // qualified Go type of the value that the field conversion writes and reads
	Outer string   // qualified Go type of the outer wrapper, the SDK type of the field
	Funcs []string // the helpers, outer first
}

// ExpandOf returns the Go expression that wraps the value expr.
func (w *fieldWrap) ExpandOf(expr string) string {
	for i := len(w.Funcs) - 1; i >= 0; i-- {
		expr = "wrap" + w.Funcs[i] + "(" + expr + ")"
	}
	return expr
}

// UnwrapOf returns the Go expression that takes the value out of the wrapped value expr.
func (w *fieldWrap) UnwrapOf(expr string) string {
	for _, f := range w.Funcs {
		expr = "unwrap" + f + "(" + expr + ")"
	}
	return expr
}

// bodyWrap is the request body that holds the resource in one field.
type bodyWrap struct {
	Field string // Go field of the body
	Value bool   // the field is a value, not a pointer
}

// enumMap is the mapping of an enum that a released resource exposes under other names.
type enumMap struct {
	Name    string // Go name prefix of the two map variables
	SDKType string // qualified SDK enum type
	Values  []enumMapValue
	// Read are rejected API values. Flatten returns the API spelling. Expand and the
	// schema validator do not accept them.
	Read []enumMapValue
	// ZeroNull is the API zero value that flatten reads as null (zeroNull), or "".
	ZeroNull string
	// NullValues are rejected API values that flatten reads as null (readRejectedAs: "null").
	NullValues []string
}

// NullAPIValues are the API values that flatten reads as null: the zero value with zeroNull, and the
// rejected values with readRejectedAs: "null".
func (m *enumMap) NullAPIValues() []string {
	var out []string
	if m.ZeroNull != "" {
		out = append(out, m.ZeroNull)
	}
	return append(out, m.NullValues...)
}

type enumMapValue struct{ TF, API string }

// maskField is one top-level Update field. With leaf masks, it is also a
// node of the mask tree: Children are the fields of an object or the arms of
// a oneOf. A node without children is compared as a whole: a scalar, a list,
// a set, a map, or an object with no fields.
type maskField struct {
	TFName string // Terraform attribute name, to read the plan and the state
	API    string // API property name, the update mask entry
	OneOf  bool
	// ServerDefault permits one planned unknown value: removing this field
	// resets it to its declared server default.
	ServerDefault bool
	Children      []*maskField
	// Groups are the oneOf groups among Children, as Terraform names.
	Groups [][]string
}

// convObject is one pair of Terraform model struct and SDK struct.
type convObject struct {
	Func    string // function name suffix: expand<Func>, flatten<Func>
	Model   string // Terraform model struct
	SDK     string // SDK struct
	Fields  []*convField
	Expand  bool // generate expand<Func>
	Flatten bool // generate flatten<Func>
	// Group: the object is a group of the behavior-overrides file. Its fields are fields of the
	// SDK struct of the parent, so expand writes them to that struct, and flatten reads them from it.
	Group bool
	// RequireOne: the group holds the arms of a oneOf that the user must set. Flatten fails when
	// the response has none of them, because the object would have no arm.
	RequireOne bool
	// OptionalGroup: the group is optional (the line has optional). Flatten returns nil, a null
	// group, when the response has none of its fields.
	OptionalGroup bool
	// KeepPriorArm: flatten keeps the prior value when the response has other fields than it does
	// (the keepPriorArm key of a group).
	KeepPriorArm bool
	// AttrTypes are the Terraform attribute types, when a list holds the
	// object. The function AttrTypesFunc returns them.
	AttrTypes     []convAttrType
	AttrTypesFunc string
	// NeedsPrior: flatten takes the prior model, because a field of this object, or of a
	// nested object, keeps the prior order.
	NeedsPrior bool
	// Same: the package has same<Func>, which compares two SDK values of this object. It
	// ignores the fields that the resource does not manage. A list that keeps the prior
	// order uses it. SameChecks are the Go conditions that must hold for equal values.
	Same       bool
	SameChecks []string
	// ExtraFlatten writes typed nulls for extraAttributes of the behavior-overrides file.
	ExtraFlatten []extraFlatten
	// Local: SDK is a struct of the generated package, not of the SDK (keyedBy object).
	Local bool
	// DefaultArms are the arms that expand sends empty when the configuration sets no arm of
	// their oneOf group (defaultArm).
	DefaultArms []defaultArm
	// Unsupported are the oneOf arms that the resource does not configure (unsupportedArms).
	// Flatten reports a response that selects one.
	Unsupported []unsupportedArm
	// Parents are the inlined objects of a request body that are SDK pointers.
	Parents []convParent
	// FlattenParents are the SDK pointers of a response object that hold the fields of an inlined
	// nested object. Flatten reads a missing one as an empty object, so its fields are null.
	FlattenParents []convParent
	// Carry is set when Terraform shows the object as the arms of its oneOf field (carrySiblings).
	Carry *convCarry
	// Typed is set when Terraform shows the oneOf as a type attribute plus the merged fields of
	// its arms (typedArms). Fields then holds the type and the merged fields, all Carried.
	Typed *convTyped
	// Lists are the top-level lists of a root that distributeTo lines moved into other objects.
	// Expand sends the items that those objects collect, and flatten gives the response list to them.
	Lists []convRootList
}

// convCarry is an object that Terraform shows as the arms of its oneOf field: each arm attribute
// holds the other fields of the object (the siblings) and the fields of the arm.
type convCarry struct {
	Pkg   string     // the name of the SDK package
	Field *convField // the field of the object that holds the oneOf
	Arms  []*carryArm
	// RequireOne: the line has requireOne. Flatten fails when the response has no arm or more
	// than one, as for a oneOf group that requires an arm.
	RequireOne bool
}

// carryArm is one arm attribute of a carrySiblings object.
type carryArm struct {
	TFName string      // Terraform attribute of the arm
	Model  string      // Go field of the object model that holds the arm model
	SDK    string      // Go field of the oneOf struct
	Object *convObject // the arm; its model also holds the siblings
	// Siblings are the conversions of the other fields of the object, read from and written to
	// the arm model. A requestValues line makes one a fixed value in this arm.
	Siblings []*convField
	Folds    []*carryFold
}

// carryFold is an arm that Terraform shows as another arm with a false flag.
type carryFold struct {
	SDK     string   // Go field of the folded arm in the oneOf struct
	SDKType string   // qualified SDK type of the folded arm
	Flag    string   // Go field of the flag in the arm model
	Fields  []string // Go fields of the two arm structs, which are the same
}

// defaultArm is an arm that expand sends empty when no arm of its group is set.
type defaultArm struct {
	Field string   // SDK field of the arm
	Value string   // Go expression of the empty arm
	Arms  []string // SDK fields of every arm of the group
	// Unknown are the model fields of the arms that can be unknown. An unknown arm is no missing
	// arm, so expand does not send the default arm then.
	Unknown []string
}

type convAttrType struct {
	TFName string
	Expr   string
}

// Conversion kinds of a convField.
const (
	convString  = "string"  // types.String ↔ *string
	convSplit   = "split"   // types.Set of strings ↔ *string of values joined by a separator (split)
	convBool    = "bool"    // types.Bool ↔ *bool
	convFloat64 = "float64" // types.Float64 ↔ *float64
	convUint64  = "uint64"  // types.Int64 ↔ *string (D7)
	// convInt64Wire is a signed 64-bit number that the API sends as a string, shown as an Int64
	// (terraformType: Int64 on a string whose pattern is ^-?[0-9]+$).
	convInt64Wire = "int64wire" // types.Int64 ↔ *string
	// convFoldString shows a oneOf of an enum and a string as one string (asString).
	convFoldString = "foldstring" // types.String ↔ *<oneOf SDK type>
	convTime       = "time"       // types.String ← *time.Time: a time that only the response has
	convRFC3339    = "rfc3339"    // timetypes.RFC3339 ↔ *time.Time: a time that a request sends
	convTimes      = "times"      // types.Set/List of timetypes.RFC3339 ↔ []time.Time
	convEnum       = "enum"       // types.String ↔ *<enum type> (D12)
	convObj        = "object"     // *<Model> ↔ *<SDK type>
	convEmpty      = "empty"      // *<Model> ↔ map[string]interface{} (F16)
	// convFoldList shows a oneOf of an arm without fields and an arm with one list as that list
	// (typedArms.asList): no values are the first arm, some values the second.
	convFoldList = "foldlist" // types.List of strings ↔ *<oneOf SDK type>
	// convEmptyBool shows an object without fields as a bool (asBool): true ↔ a non-nil empty map.
	convEmptyBool = "emptybool"
	convStrings   = "strings" // types.Set/List ↔ []string or []<enum type>
	// convJoin shows a list of strings as one string with the elements joined (join).
	convJoin    = "join"    // types.String ↔ []string
	convObjects = "objects" // types.List ↔ []<SDK type>

	convInt32   = "int32"   // types.Int32 ↔ *int32
	convInt64   = "int64"   // types.Int64 ↔ *int64 (signed, a JSON number)
	convFloat32 = "float32" // types.Float32 ↔ *float32, read as its shortest decimal
	convScalars = "scalars" // types.List/Set ↔ []bool, []int32, []int64, []float32, []float64
	// convNamedInts is a list or set of integers that Terraform shows as names (valueNames).
	convNamedInts = "namedints" // types.List/Set of types.String ↔ []int32, []int64
	// convInt32AsInt64 and convInts32AsInt64 keep the released Int64 type of an int32 field
	// (terraformType: Int64).
	convIntegerAsString = "integerasstring" // computed integer → released string
	convInt32AsInt64    = "int32asint64"    // types.Int64 ↔ *int32
	// convInt64AsString keeps the released String type of an int64 resource id (terraformType:
	// String): the decimal text of the id.
	convInt64AsString = "int64asstring" // types.String ↔ *int64
	// convFloat64AsFloat32 reads a double that the API stores as a float32 (storedAs: float32) as
	// the shortest decimal of the float32.
	convFloat64AsFloat32 = "float64asfloat32" // types.Float64 ↔ *float64
	convInts32AsInt64    = "ints32asint64"    // types.List/Set of types.Int64Type ↔ []int32
	// convFloat32AsFloat64 keeps the released Float64 type of a float32 field (terraformType:
	// Float64). convInt32AsNumber and convFloat64AsNumber keep the released Number type of an
	// int32 or a double field (terraformType: Number).
	convFloat32AsFloat64 = "float32asfloat64" // types.Float64 ↔ *float32
	convInt32AsNumber    = "int32asnumber"    // types.Number ↔ *int32
	convFloat64AsNumber  = "float64asnumber"  // types.Number ↔ *float64

	convStringMap = "stringmap" // types.Map ↔ map[string]string or map[string]<enum type>
	convScalarMap = "scalarmap" // types.Map ↔ map[string]bool, int32, int64, float32, float64
	convUint64Map = "uint64map" // types.Map of Int64 ↔ map[string]string (D7)
	convObjectMap = "objectmap" // types.Map ↔ map[string]<SDK type>
	convFold      = "fold"      // *<Model> → top-level Create fields (createAs)

	// convGroup is a group of the behavior-overrides file: *<Model> ↔ fields of the SDK struct
	// of the parent. The API has no object for it.
	convGroup = "group"

	// convNotBool is a bool that Terraform shows as its opposite (showInverted).
	convNotBool = "notbool" // types.Bool ↔ *bool, negated

	// convSplitArms is a list of oneOf items that Terraform shows as one collection per arm
	// (splitArms). Object holds the conversion of each arm, whose Model is the attribute.
	convSplitArms = "splitarms" // types.Set/List per arm ↔ []<SDK type>

	// convDistributed is a list that Terraform shows in the items of another object (distributeTo).
	convDistributed = "distributed" // types.List of objects ↔ the items of a list of the resource

	convKeyedObjects = "keyedobjects" // types.Map of objects ↔ []<SDK type> keyed by a property (keyedBy)
	convKeyedValues  = "keyedvalues"  // types.Map of String ↔ []<SDK type> of a key and a value (keyedBy)
)

// convField is one field of a convObject.
type convField struct {
	TFName     string // Terraform attribute name, for diagnostic paths
	Model      string // Go field of the Terraform model
	SDK        string // Go field of the SDK struct
	Conv       string // conversion kind
	Collection string // strings, objects: "Set" or "List"
	// SDKType is the qualified SDK value type: enum: the enum type;
	// strings, objects: the element type.
	SDKType string
	Object  *convObject // object, empty, objects: the nested object
	// ElemType is the Terraform element type of scalars and scalarmap, for
	// example "types.Int32Type".
	ElemType string
	// ValueNames are the numbers of a namedints field and their Terraform names, in ascending order.
	ValueNames []overrides.ValueName
	// Enum is true for a collection or map whose string values are enum
	// values. Response conversion then rejects protobuf zero sentinels.
	Enum     bool
	EnumZero string   // exact protobuf zero sentinel for enum conversion
	EnumMap  *enumMap // set when the overrides state the Terraform values of the enum
	// ReadEmptyAsNull: flatten reads an empty string, an empty list, or an object
	// with no value, as null.
	ReadEmptyAsNull bool
	// ReadZeroAsNull: flatten reads the zero value of a string or a number as null. The
	// contract sends the scalar without presence, and a configuration cannot hold zero.
	ReadZeroAsNull bool
	Zero           string // the zero value that ReadZeroAsNull compares, as model.Type.Zero
	// Projection: the response holds each item as an object, and flatten reads the strings from one
	// property of each item (an itemName line). Expand sends the strings.
	Projection *itemProjection
	// refPath is the SDK ref path of the field, for the refs of the items of a projection.
	refPath string
	// ReadNullAsEmpty: flatten reads a missing string as "", and a missing list or set of objects
	// as an empty collection.
	ReadNullAsEmpty bool
	// EnumVariant is the enumAs of the line: the spelling of the enum at this field.
	EnumVariant string
	// ReadNullAsValue is the Terraform value that flatten gives an enum that the response does not
	// have (readNullAs with a value).
	ReadNullAsValue string
	// SendNullAsEmpty: expand sends "" for a null string, so the API clears the stored value.
	SendNullAsEmpty bool
	// EmptyError is the error that flatten reports when the response has no value of this object
	// field, or an object with all fields unset (unsupportedIfEmpty).
	EmptyError *unsupportedArm
	// CaseInsensitive: the enums line of a scalar enum field has caseInsensitive. Expand matches
	// the value in any case, and flatten keeps the prior spelling of an equal value.
	CaseInsensitive bool
	// EmptyDefault is the Go expression of the empty list or set that the line gives as its default
	// (default: []), for a field of a typedArms arm. A typedArms object reads a null of it as that
	// value, because an arm that lacks the field gives null, and an import has no prior to restore.
	EmptyDefault string
	// Carried: a sibling or a fold flag in the model of a carrySiblings arm. The arm functions
	// leave it to the object that holds the oneOf; it counts for the attribute types.
	Carried bool
	// SplitArms: the line of the field has splitArms. ShowInverted: the line has showInverted.
	// RequestPosition: the line has requestPosition; the list that holds the object sends the
	// position of each item.
	SplitArms, ShowInverted, RequestPosition bool
	// SendUnknownAsZero: expand sends the zero value of the scalar when the plan is unknown
	// (sendUnknownAsZero). ZeroType is its Go type.
	SendUnknownAsZero bool
	ZeroType          string
	// UnknownValue: expand sends this Go literal, as a ZeroType, when the plan is unknown
	// (updateDefault). Update omits nothing: the API keeps the stored value of an omitted field.
	UnknownValue string
	// RequestValue is an API value expand always sends. The field is not in the Terraform model.
	// FixedValue is its Go expression: an enum cast, a quoted string, or a bool.
	RequestValue string
	FixedValue   string
	// StringFold holds the two arms of a oneOf that Terraform shows as one string (asString).
	StringFold *foldArms
	// ListFold holds the arms of a oneOf that Terraform shows as one list (typedArms.asList).
	ListFold *foldList
	// ClientID: expand makes a UUID for an unknown value (clientID: uuid).
	ClientID bool
	// Dist and ItemID are set on a distributed field (distributeTo): the attribute of the object that
	// holds the items of a list of the resource. ItemID is the field of the object that names them.
	Dist   *convDistribution
	ItemID *convField
	// Join is the separator of a list of strings that Terraform shows as one string (join).
	Join string
	// RequestValueBy are the API values expand sends when an object field of the model is set,
	// in the order of the field names.
	RequestValueBy []fixedBy
	// FoldModel is the model of a fold field that the model holds as a types.Object.
	FoldModel string
	// SendEmpty: the contract requires the list or the map, and an optional line lets Terraform
	// leave it out.
	// Expand sends an empty list or map for a null one, as the request needs the key.
	SendEmpty    bool
	SDKSliceType string // the qualified Go type of the SDK list or map, for SendEmpty
	// Keyed is the key and the value of the API items of a keyedobjects or keyedvalues field.
	Keyed *keyedFields
	// KeepPriorOrder: flatten returns the items in the order of the prior model, when the
	// API returns the same items in another order.
	KeepPriorOrder bool
	// PriorVia is the request object that pairs the items of a KeepPriorOrder list whose
	// response object the request does not send. flatten converts each API item to the model
	// and both it and each prior item to this request object, so only the fields that the
	// request sends decide whether two items are the same. It is nil when the request sends
	// the response object.
	PriorVia *convObject
	// Equality is "yaml" or "json" when flatten keeps the prior text of a string that the API
	// returns as the same document in another format, and "" otherwise.
	Equality    string
	ObjectValue bool // a computed object stored as unknown-capable types.Object
	// Missing is the string default from the behavior overrides. Flatten writes it
	// when the API omits an enum that has no presence. A field with presence stays null.
	Missing string
	// Value is true when the SDK field is a value, not a pointer. The SDK
	// does that for a required field (F18). Expand sends the zero value for
	// null; the schema requires the attribute, so it is not null.
	Value bool
	// SetPrior: a set of objects whose items need the prior model. Flatten pairs each API item with the
	// prior item that has the same request value.
	SetPrior bool
	// Wrap is set when the API holds the value in wrapper objects that Terraform does not show.
	Wrap *fieldWrap
	// FlattenOnly is a field that no request sends, of an object that a request holds, such as
	// a computed field of an object that the request and the response share. Flatten reads it.
	// Expand does not send it.
	FlattenOnly bool
	// Fold: for the fold conversion, the properties of the object that Create sends as
	// top-level fields. Each one reads the object model and writes its own SDK field.
	Fold []*convField
	// Negated: the request sends the opposite of the bool (a negate line), for example
	// disabled for enabled.
	Negated bool
	// Split is the separator of a split field: the API holds the values of the set joined by it.
	Split string
	// Duration is the released text of a duration (protoText or goDuration): the API holds a
	// protobuf JSON duration such as 900s.
	Duration string
}

// dropParent is an optional request object that expand leaves out when the configuration sets
// none of the attributes that write to it.
type dropParent struct {
	SDK   string   // the Go path of the object in the SDK struct
	Nulls []string // a Go condition for each attribute that writes to it: true when it is not set
}

// DropEmptyParents are the optional request objects that expand leaves out when the configuration
// sets none of their attributes, the deepest first. A configured "", false, 0, or {} still sends the
// object, so the check reads the model, not the zero value of the SDK struct. Only the attributes
// that write to the object count, also inside a group. An object with a field that the request
// always sends is always sent.
func (o *convObject) DropEmptyParents() []dropParent {
	var out []dropParent
	for i := len(o.Parents) - 1; i >= 0; i-- {
		p := o.Parents[i]
		if !p.DropEmpty {
			continue
		}
		d := dropParent{SDK: p.SDK}
		always := false
		for _, f := range o.Fields {
			nulls, alwaysSent := nullsUnder(f, p.SDK+".", "m", nil)
			if alwaysSent {
				always = true
				break
			}
			d.Nulls = append(d.Nulls, nulls...)
		}
		if !always {
			out = append(out, d)
		}
	}
	return out
}

// nullsUnder returns a Go condition for each attribute of field f, a field of the model at model,
// that expand writes to an SDK path that starts with prefix: true when the configuration does not
// set it. It also reports whether expand always writes there: a fixed value or a fold. A group has
// no SDK path: its fields write to the struct of the object that holds it, so each one is checked
// through the group, which is nil when the configuration leaves it out. groups are the group
// expressions on the way.
func nullsUnder(f *convField, prefix, model string, groups []string) (nulls []string, always bool) {
	if f.FlattenOnly {
		return nil, false
	}
	at := model + "." + f.Model
	if f.Conv == convGroup && f.Object != nil {
		for _, child := range f.Object.Fields {
			n, a := nullsUnder(child, prefix, at, append(slices.Clone(groups), at))
			if a {
				return nil, true
			}
			nulls = append(nulls, n...)
		}
		return nulls, false
	}
	if !strings.HasPrefix(f.SDK, prefix) {
		return nil, false
	}
	if f.Conv == "fixed" || len(f.Fold) != 0 {
		return nil, true
	}
	cond := "modelNull(" + at + ")"
	if len(groups) != 0 {
		var parts []string
		for _, g := range groups {
			parts = append(parts, g+" == nil")
		}
		cond = "(" + strings.Join(append(parts, cond), " || ") + ")"
	}
	return []string{cond}, false
}

// NilParents are the objects that a field of the object goes through and that can be nil in an
// SDK value: an optional request object that expand leaves out, and an optional response object
// that holds inlined fields. The outer one comes first. same<Func> compares copies with an empty
// object for a missing one, so that a missing object and an empty one are equal.
func (o *convObject) NilParents() []convParent {
	var out []convParent
	for _, p := range append(slices.Clone(o.Parents), o.FlattenParents...) {
		if (p.DropEmpty || slices.Contains(o.FlattenParents, p)) && !slices.ContainsFunc(out, func(q convParent) bool { return q.SDK == p.SDK }) {
			out = append(out, p)
		}
	}
	slices.SortStableFunc(out, func(a, b convParent) int { return strings.Count(a.SDK, ".") - strings.Count(b.SDK, ".") })
	return out
}

// DropsEmpty reports whether expand leaves out an optional request object. The template then emits
// modelNull.
func (d *convData) DropsEmpty() bool {
	for _, obj := range d.Objects {
		if obj.Expand && len(obj.DropEmptyParents()) != 0 {
			return true
		}
	}
	return false
}

// Dst is the Go expression that expand writes the value to: the SDK field, or the value inside
// the wrappers.
func (f *convField) Dst() string {
	if f.Wrap != nil {
		return "inner"
	}
	return "out." + f.SDK
}

// Src is the Go expression that flatten reads the value from.
func (f *convField) Src() string {
	if f.Wrap != nil {
		return "inner"
	}
	return "v." + f.SDK
}

// DirectValue reports whether Dst and Src are an SDK value field, not a pointer.
func (f *convField) DirectValue() bool { return f.Value && f.Wrap == nil }

// WrappedSrc is the Go expression of the wrapped SDK field that the unwrap helpers take.
func (f *convField) WrappedSrc() string {
	if f.Value {
		return "&v." + f.SDK
	}
	return "v." + f.SDK
}

// PositionField returns the field of obj that a list of obj sets to the position of each item,
// or nil.
func (obj *convObject) PositionField() *convField {
	for _, f := range obj.Fields {
		// A carried copy in the model of a carrySiblings arm is no request field.
		if f.RequestPosition && !f.Carried {
			return f
		}
	}
	return nil
}

// UsesFoldedSame reports whether the comparison of two items of a set compares a set of strings in
// any case (equality: caseInsensitive on a set inside an object that the package compares).
func (d *convData) UsesFoldedSame() bool {
	for _, obj := range d.Objects {
		if !obj.Same {
			continue
		}
		for _, f := range obj.Fields {
			if f.Conv == convStrings && f.Collection == "Set" && f.Equality == overrides.EqualityCaseInsensitive {
				return true
			}
		}
	}
	return false
}

// UsesElementCase reports whether a field keeps the spelling of the prior elements of a list or
// set of strings (equality: caseInsensitive).
func (d *convData) UsesElementCase() bool {
	objects := d.Objects
	if d.Resource != nil {
		objects = append(slices.Clone(objects), d.Resource)
	}
	for _, obj := range objects {
		for _, f := range obj.Fields {
			if f.Equality == overrides.EqualityCaseInsensitive {
				return true
			}
		}
	}
	return false
}

// UsesCaseInsensitive reports whether a field is an enum that reads in any case.
func (d *convData) UsesCaseInsensitive() bool {
	for _, obj := range d.Objects {
		for _, f := range obj.Fields {
			if f.CaseInsensitive {
				return true
			}
		}
	}
	return false
}

// Uses reports whether a field uses the conversion kind conv. The template
// emits the helpers of a kind only when it is used.
func (d *convData) Uses(conv string) bool {
	for _, obj := range d.Objects {
		for _, f := range obj.Fields {
			if f.Conv == conv {
				return true
			}
		}
	}
	return false
}

// UsesNegated reports whether a request sends a bool negated. The template then emits negateBool.
func (d *convData) UsesNegated() bool {
	for _, obj := range d.Objects {
		for _, f := range obj.Fields {
			if f.Negated {
				return true
			}
		}
	}
	return false
}

// UsesEnumsMapped reports whether a list or set of an enum has an enums line. The template then
// emits the helpers that map its elements.
func (d *convData) UsesEnumsMapped() bool {
	for _, obj := range d.Objects {
		for _, f := range obj.Fields {
			if f.Conv == convStrings && f.EnumMap != nil {
				return true
			}
		}
	}
	return false
}

// applyJoin turns a list of strings with a join line into the string of its elements.
func (b *convBuilder) applyJoin(cf *convField, sep string) error {
	if cf.Conv != convStrings || cf.Collection != "List" && cf.Collection != "" || cf.SDKType != "string" || cf.EnumMap != nil || cf.Enum || cf.Projection != nil {
		return fmt.Errorf("%s: join needs a list of plain strings", cf.TFName)
	}
	cf.Conv, cf.Collection, cf.Join = convJoin, "", sep
	return nil
}

// UpgradePrior are the fields of the resource that a state upgrade copies from the prior state into
// the prior model of flatten: the groups that keep the arm the user wrote. The prior schema holds
// them with the same type as the current one, and the API cannot tell which arm the user wrote.
func (d *convData) UpgradePrior() []*convField {
	var out []*convField
	if d.Resource == nil {
		return nil
	}
	for _, f := range d.Resource.Fields {
		if f.Conv == convGroup && !f.ObjectValue && f.Object != nil && f.Object.KeepPriorArm {
			out = append(out, f)
		}
	}
	return out
}

// UsesKeepPriorArm reports whether a group keeps the prior arm. The template then emits its helper.
func (d *convData) UsesKeepPriorArm() bool {
	return slices.ContainsFunc(d.Objects, func(o *convObject) bool { return o.KeepPriorArm })
}

// SetIn is the Go condition that the value of the field in the model v (a name) is set: not null, not
// unknown, and not a nil object.
func (f *convField) SetIn(v string) string {
	if (f.Conv == convObj || f.Conv == convGroup) && !f.ObjectValue {
		return v + "." + f.Model + " != nil"
	}
	return "!" + v + "." + f.Model + ".IsNull() && !" + v + "." + f.Model + ".IsUnknown()"
}

// UsesUnsupported reports whether the resource has unsupported fields or arms (unsupportedArms).
// The template then emits the context switch that the AllowUnsupported hook turns on.
func (d *convData) UsesUnsupported() bool {
	return len(d.RootUnsupported) != 0 || slices.ContainsFunc(d.Objects, func(o *convObject) bool {
		return len(o.Unsupported) != 0 || slices.ContainsFunc(o.Fields, func(f *convField) bool { return f.EmptyError != nil })
	})
}

// UsesDuration reports whether a field keeps a released duration text. The template then emits its helpers.
func (d *convData) UsesDuration() bool {
	return slices.ContainsFunc(d.Objects, func(o *convObject) bool {
		return slices.ContainsFunc(o.Fields, func(f *convField) bool { return f.Duration != "" })
	})
}

// UsesJoin reports whether a field shows a list as one string. The template then emits its helpers.
func (d *convData) UsesJoin() bool { return d.Uses(convJoin) }

// UsesClientID reports whether a field gets a UUID that the client makes (clientID: uuid). The template
// then emits the function that makes it.
func (d *convData) UsesClientID() bool {
	for _, obj := range d.Objects {
		for _, f := range obj.Fields {
			if f.ClientID {
				return true
			}
		}
	}
	return false
}

// UsesFloat32 reports whether a field holds float32 values, alone or in a list, set, or map.
// The template then emits the helpers that widen them.
func (d *convData) UsesFloat32() bool {
	for _, obj := range d.Objects {
		for _, f := range obj.Fields {
			if f.Conv == convFloat32 || (f.Conv == convScalars || f.Conv == convScalarMap) && f.SDKType == "float32" {
				return true
			}
		}
	}
	return false
}

// HasValue reports whether an expanded SDK field is a value, not a pointer.
// The template then emits the valueOf helper.
func (d *convData) HasValue() bool {
	if d.CreateWrap != nil && d.CreateWrap.Value || d.UpdateWrap != nil && d.UpdateWrap.Value {
		return true
	}
	for _, obj := range d.Objects {
		for _, f := range obj.Fields {
			if obj.Expand && f.Value {
				return true
			}
		}
	}
	return slices.ContainsFunc(d.Distributions, func(dist *convDistribution) bool { return dist.List.Value })
}

// setBodyMask records the SDK field that carries a JSON-body update mask.
func (d *convData) setBodyMask(ix *refIndex, r *model.Resource) error {
	if !r.MaskInBody {
		return nil
	}
	ref, err := ix.fieldRef("update.body." + r.UpdateMask)
	if err != nil {
		return err
	}
	d.UpdateMaskField = ref.Name
	d.UpdateMaskPointer = strings.HasPrefix(ref.Want, "*")
	return nil
}

// acceptUnknown reports whether Create and Update leave an unknown value out of the request, as a
// released resource did, instead of stopping before the API call.
func acceptUnknown(r *model.Resource, file *overrides.File) bool {
	return r.Policy.Existing && (file == nil || !file.Validators.RejectUnknown)
}

// buildConv maps the model and its SDK names to the conversion data. Rules:
//   - A null Terraform value → nil. nil → a null Terraform value. So
//     false, 0, "", [] and {} are sent and read as values.
//   - An empty object (F16) is a non-nil empty map when it is set.
//   - uint64 → the decimal string (D7). An enum → its API value (D12).
//   - The Update body has only the Update fields, so application,
//     subsystem, and target are never set (D9).
//
// file is the behavior-overrides file of a resource that users already have (nil for a new resource).
func buildConvWith(r *model.Resource, refs []sdkcheck.Ref, file *overrides.File) (*convData, error) {
	ix, err := indexRefs(refs)
	if err != nil {
		return nil, err
	}
	b := &convBuilder{ix: ix, bySchema: map[string]*convObject{}, dists: map[string]*convDistribution{}, file: file, resource: r, sent: sentFields(r, file)}
	out := &convData{SDKPkg: ix.pkg.Pkg, SDKName: ix.pkg.Name, AcceptUnknown: acceptUnknown(r, file)}
	if err := out.setBodyMask(ix, r); err != nil {
		return nil, err
	}
	out.IDGuard = idGuard(r)
	if out.IDGuard != "" {
		out.IDString = r.IDType.Kind == model.String
	}
	for _, f := range r.Fields {
		if r.Policy.DefaultArm(r.Name, f.Name) {
			return nil, fmt.Errorf("%s: defaultArm is supported on an arm of a nested object only", f.Name)
		}
	}

	roots := []convRoot{
		{&out.Create, "create.body", true, func(f *model.ResourceField) bool { return f.Create != nil }, func(f *model.ResourceField) *overrides.Parent { return file.ParentOf("create", f.Name) }},
		{&out.Update, "update.body", true, func(f *model.ResourceField) bool { return f.Update != nil }, func(f *model.ResourceField) *overrides.Parent { return file.ParentOf("update", f.Name) }},
		{&out.Resource, "fields", false, func(f *model.ResourceField) bool { return f.InGet }, func(f *model.ResourceField) *overrides.Parent { return file.ParentOf("get", f.Name) }},
	}
	for _, root := range roots {
		if root.path == "update.body" && file.Upserts() {
			// The Set call creates and updates, so Update expands the Create body.
			out.Update, out.UpdateBody = out.Create, out.CreateBody
			continue
		}
		if err := b.buildRoot(out, root); err != nil {
			return nil, err
		}
	}
	if err := b.distribute(out); err != nil {
		return nil, err
	}
	if err := b.arms(out); err != nil {
		return nil, err
	}
	unmarkCarriedOneOfs(b.objects)
	if err := checkObjects(b.objects); err != nil {
		return nil, err
	}
	rootUnsupported, err := b.unsupportedArms(&model.Type{Schema: b.resource.Name}, "fields")
	if err != nil {
		return nil, err
	}
	out.RootUnsupported = rootUnsupported
	for _, obj := range b.listed {
		// Only flatten builds Terraform values. An object that only a request
		// holds needs no attribute types.
		if !obj.Flatten {
			continue
		}
		if err := b.attrTypes(obj); err != nil {
			return nil, err
		}
	}
	if err := checkExpandNames(b.objects); err != nil {
		return nil, err
	}
	out.Objects = b.objects
	out.OneOfArms = oneOfArmPaths(r, b.file)
	out.EnumMaps = b.enumMaps
	out.WrapHelpers = b.wraps
	out.ItemProjections = b.projections
	out.KeyedObjects = b.keyedObjects
	if err := markPrior(out); err != nil {
		return nil, err
	}
	if err := checkDistributions(out); err != nil {
		return nil, err
	}
	for _, f := range r.Fields {
		appendCreateField(out, r, f, b.fixedRequest(r.Name, f.Name), b.file)
	}
	fixed := func(name string) bool { return b.fixedRequest(r.Name, name) }
	if r.Replace {
		return out, replaceFields(r, out, fixed, b.file)
	}
	// Without a mask the Update body holds a derived value, and the server updates it.
	omit := b.file != nil && b.file.UpdateMask == overrides.UpdateMaskOmit
	if r.UpdateMethod == "POST" && !omit {
		// The model records that the Update has no mask; which fields to send is the decision of
		// this tool, and the file states it.
		return nil, fmt.Errorf("%s: an Update by POST has no update mask: state updateMask: omit, so that Update sends every field and the server updates each field present in the body (a removed optional attribute cannot be cleared)", r.Update.OperationID)
	}
	if !omit {
		if err := replaceOnlyLines(b.file); err != nil {
			return nil, err
		}
	}
	if err := buildMask(r, out, fixed, omit, b.file); err != nil {
		return nil, err
	}
	return out, nil
}

// oneOfArmPaths returns the Terraform paths of every oneOf arm of r, in model
// order. A list, set, or map step adds nothing to the path.
func oneOfArmPaths(r *model.Resource, file *overrides.File) []string {
	var out []string
	for _, g := range r.Groups {
		for _, arm := range g.Arms {
			if file != nil && file.Groups[file.GroupOf(arm)].TypedArms != nil {
				continue // the type attribute names the arm: no attribute is the arm
			}
			out = append(out, rootPath(file.GroupPath(arm), terraformName(r.Policy, r.Name, arm)).dotted())
		}
	}
	for _, f := range r.Fields {
		if splitArms(file, r.Name, f.Name) || asString(file, r.Name, f.Name) {
			// The arms are collections of values, or the arms of a string, not objects of the state.
			continue
		}
		out = typeArmPaths(r.Policy, file, out, rootPath(file.GroupPath(f.Name), terraformName(r.Policy, r.Name, f.Name)).dotted(), f.Type, map[*model.Type]bool{})
	}
	return out
}

func typeArmPaths(p model.Policy, file *overrides.File, out []string, at string, t *model.Type, seen map[*model.Type]bool) []string {
	if t == nil || seen[t] {
		return out
	}
	seen[t] = true
	defer delete(seen, t)
	if t.Elem != nil {
		return typeArmPaths(p, file, out, at, t.Elem, seen)
	}
	if carry := carryField(file, t); carry != nil {
		return carryArmPaths(p, file, out, at, t, carry, seen)
	}
	arms := map[string]bool{}
	for _, g := range t.Groups {
		for _, arm := range g.Arms {
			arms[arm] = true
		}
	}
	for _, f := range t.Fields {
		child := at + "." + terraformName(p, model.ComponentOf(t), f.Name)
		if group := nestedGroupOf(file, model.ComponentOf(t), f.Name); group != "" {
			child = at + "." + group + "." + terraformName(p, model.ComponentOf(t), f.Name)
		}
		if t.Kind == model.OneOf && f.Name != t.Discriminator || arms[f.Name] {
			out = append(out, child)
		}
		if asString(file, model.ComponentOf(t), f.Name) {
			// The arms of the oneOf are one string of the state.
			continue
		}
		out = typeArmPaths(p, file, out, child, f.Type, seen)
	}
	return out
}

// carryArmPaths adds the arm paths of a carrySiblings object: the arms of the oneOf are attributes
// of the object, and each holds the other fields.
func carryArmPaths(p model.Policy, file *overrides.File, out []string, at string, t *model.Type, carry *model.Field, seen map[*model.Type]bool) []string {
	line := file.Types[t.Schema].Fields[carry.Name].CarrySiblings
	for _, arm := range carry.Type.Fields {
		if _, folded := line.Fold[arm.Name]; folded {
			continue
		}
		child := at + "." + terraformName(p, model.ComponentOf(carry.Type), arm.Name)
		out = append(out, child)
		out = typeArmPaths(p, file, out, child, arm.Type, seen)
		for _, sibling := range t.Fields {
			if sibling != carry {
				out = typeArmPaths(p, file, out, child+"."+terraformName(p, t.Schema, sibling.Name), sibling.Type, seen)
			}
		}
	}
	return out
}

// convRoot is a root object of the conversion: the Create body, the Update body, or the resource.
type convRoot struct {
	obj    **convObject
	path   string // SDK name path of the struct
	expand bool
	has    func(*model.ResourceField) bool
	parent func(*model.ResourceField) *overrides.Parent // the inlined object that holds a field here
}

// arms sets the inlined objects of the resource response that the SDK holds in a pointer. An arm
// is always one: a response without it has no resource. flatten reads the fields through the
// pointer, so it returns an error for a response without the object.
func (b *convBuilder) arms(out *convData) error {
	for _, parent := range inlinedParents(b.resource, b.file) {
		ref, err := b.ix.fieldRef("fields." + parent.Field)
		if err != nil {
			return err
		}
		pointer := strings.HasPrefix(ref.Want, "*")
		if parent.Arm && !pointer {
			return fmt.Errorf("SDK field %s of the oneOf arm %s has type %s, want a pointer", ref.SDKName(), parent.Field, ref.Want)
		}
		if pointer {
			out.Arms = append(out.Arms, convArm{SDK: ref.Name, API: parent.Field})
		}
	}
	return nil
}

// parentField returns the Go field of the root that holds an inlined object. A request body that
// holds it in a pointer gets it set by expand. A response that holds it in a pointer can leave it
// out, so flatten reads a missing one as an empty object, and its fields are null.
func (b *convBuilder) parentField(obj *convObject, root convRoot, parent *overrides.Parent) (string, error) {
	ref, err := b.ix.fieldRef(root.path + "." + parent.Field)
	if err != nil {
		return "", err
	}
	typ, err := b.ix.typeRef(root.path + "." + parent.Field)
	if err != nil {
		return "", err
	}
	switch ref.Want {
	case typ.Name:
	case "*" + typ.Name:
		list := &obj.FlattenParents
		if root.expand {
			list = &obj.Parents
		}
		if !slices.ContainsFunc(*list, func(p convParent) bool { return p.SDK == ref.Name }) {
			*list = append(*list, convParent{SDK: ref.Name, Type: b.qualify(typ.Name)})
		}
	default:
		return "", fmt.Errorf("SDK field %s has type %s, want %s or *%s", ref.SDKName(), ref.Want, typ.Name, typ.Name)
	}
	return ref.Name, nil
}

// requestComponent returns the component that the request wrapper of the body at path holds when
// it is not the resource, and "" otherwise.
func (b *convBuilder) requestComponent(path string) string {
	op := b.resource.Create
	if path == "update.body" {
		op = b.resource.Update
	}
	if op.RequestComponent == b.resource.Name {
		return ""
	}
	return op.RequestComponent
}

// requestObjects returns the Go selector prefix of the objects that the request path of f goes
// through, for example "IntegrationParameters.". An object that the SDK holds in a pointer is set
// by expand before the fields that it holds, so they are always sent.
func (b *convBuilder) requestObjects(obj *convObject, root convRoot, f *model.ResourceField) (string, error) {
	update := root.path == "update.body"
	requestPath := f.RequestPathOf(update)
	segments := strings.Split(requestPath, ".")
	prefix := ""
	for _, segment := range segments[:len(segments)-1] {
		via := root.path + "." + f.Name + ".via." + segment
		ref, err := b.ix.fieldRef(via)
		if err != nil {
			return "", err
		}
		typ, err := b.ix.typeRef(via)
		if err != nil {
			return "", err
		}
		prefix += ref.Name + "."
		switch ref.Want {
		case typ.Name:
		case "*" + typ.Name:
			if attrs := requestAttrs(root, f); attrs != nil && !attrs.Required {
				// expand allocates the object before it reads the field, so a null value would
				// send an empty object, which the API can read as a value.
				return "", fmt.Errorf("the request path %q goes through the optional object %s, which expand would send empty for a null %s: make the object required in the contract, or set the field in a hook", requestPath, ref.SDKName(), f.Name)
			}
			if at := strings.TrimSuffix(prefix, "."); !slices.ContainsFunc(obj.Parents, func(p convParent) bool { return p.SDK == at }) {
				obj.Parents = append(obj.Parents, convParent{SDK: at, Type: b.qualify(typ.Name)})
			}
		default:
			return "", fmt.Errorf("SDK field %s has type %s, want %s or *%s", ref.SDKName(), ref.Want, typ.Name, typ.Name)
		}
	}
	return prefix, nil
}

// requestAttrs returns the lifecycle attributes of f in the request of the root, or nil.
func requestAttrs(root convRoot, f *model.ResourceField) *model.Attrs {
	if root.path == "update.body" {
		return f.Update
	}
	return f.Create
}

// buildRoot builds one root object and its fields.
func (b *convBuilder) buildRoot(out *convData, root convRoot) error {
	ref, err := b.ix.typeRef(root.path)
	if err != nil {
		return err
	}
	obj, fieldsPath, err := b.rootObject(out, root, ref)
	if err != nil {
		return err
	}
	b.objects = append(b.objects, obj)
	groups := map[string]*convObject{}
	for _, f := range b.resource.Fields {
		if b.file != nil && b.file.Types[b.resource.Name].Fields[f.Name].Negate {
			// Only the request object of a nested component sends a field negated.
			return fmt.Errorf("%s.%s: negate works on a field of a nested request object, not on a field of the resource component", b.resource.Name, f.Name)
		}
		fixed := b.fixedRequest(b.resource.Name, f.Name)
		if !root.has(f) && !(root.expand && fixed) {
			continue
		}
		if folds := b.resource.CreateFolds(f); folds != nil && root.path == "create.body" {
			cf, err := b.foldField(fieldsPath, f, folds)
			if err != nil {
				return fmt.Errorf("%s.%s: %w", root.path, f.Name, err)
			}
			obj.Fields = append(obj.Fields, cf)
			continue
		}
		typ := rootType(root.path, f)
		if fixed && !root.has(f) {
			// The contract marks the field read-only, so the request type omits it.
			// The SDK struct still has it, and the released client sends the fixed value.
			typ = f.Type
		}
		cf, err := b.rootField(obj, root, fieldsPath, f, typ)
		if err != nil {
			return err
		}
		if path := b.file.GroupPath(f.Name); path != nil {
			b.addToGroup(obj, groups, path, cf)
			continue
		}
		obj.Fields = append(obj.Fields, cf)
	}
	if err := b.rootListsAndGroups(obj, root, fieldsPath, groups); err != nil {
		return err
	}
	if err := b.mark(obj, root.expand); err != nil {
		return fmt.Errorf("%s: %w", root.path, err)
	}
	*root.obj = obj
	return nil
}

// rootListsAndGroups finishes the fields of the root that need all of them: the typedArms groups, which
// replace the arms that they hold, and the lists that other components distribute into.
func (b *convBuilder) rootListsAndGroups(obj *convObject, root convRoot, fieldsPath string, groups map[string]*convObject) error {
	if err := b.typedGroups(groups); err != nil {
		return err
	}
	return b.rootLists(obj, root, fieldsPath)
}

// rootField returns the conversion of the top-level field f of type typ in the root: the Go
// expression of its SDK field goes through the request objects and the inlined object that hold it.
func (b *convBuilder) rootField(obj *convObject, root convRoot, fieldsPath string, f *model.ResourceField, typ *model.Type) (*convField, error) {
	cf, err := b.field(fieldsPath, b.resource.Name, f.Name, typ, responseOnly(f.Behavior), hasPresence(f.Get))
	if err != nil {
		return nil, fmt.Errorf("%s.%s: %w", root.path, f.Name, err)
	}
	if root.expand && fieldsPath == root.path && f.RequestPathOf(root.path == "update.body") != "" {
		// The request holds the field deeper than the wrapper, so the Go expression goes through the objects.
		prefix, err := b.requestObjects(obj, root, f)
		if err != nil {
			return nil, fmt.Errorf("%s.%s: %w", root.path, f.Name, err)
		}
		cf.SDK = prefix + cf.SDK
	}
	if parent := root.parent(f); parent != nil {
		// The SDK holds the field in the inlined object, so the Go expression goes through it.
		field, err := b.parentField(obj, root, parent)
		if err != nil {
			return nil, fmt.Errorf("%s.%s: %w", root.path, f.Name, err)
		}
		cf.SDK = field + "." + cf.SDK
	}
	b.markComputedObjectValue(cf, f, root.path == "fields")
	if err := checkReadEmptyAs(cf); err != nil {
		return nil, fmt.Errorf("%s.%s: %w", root.path, f.Name, err)
	}
	return cf, nil
}

// rootType is the type of field f in the root at path: the request type in
// the Create or the Update body, else the type in the resource response.
func rootType(path string, f *model.ResourceField) *model.Type {
	switch path {
	case "create.body":
		return f.Type.CreateType()
	case "update.body":
		return f.Type.UpdateType()
	}
	return f.Type
}

// rootObject returns the object of a root and the SDK path of its fields. A request body that
// wraps the resource is expanded as the resource type, and expandCreate and expandUpdate wrap it.
func (b *convBuilder) rootObject(out *convData, root convRoot, ref sdkcheck.Ref) (*convObject, string, error) {
	r := b.resource
	switch {
	case root.path == "fields":
		return b.object(r.Name, ref), root.path, nil
	case r.Policy.WrapperOf(root.path == "create.body") != "" && root.expand:
		// The wrapper holds the resource, or another component of the spec when the request has a
		// shape of its own (the request component). Expand then writes that component, and each
		// field has the SDK name that its request path gives.
		wrapped, fieldsPath := "", "fields"
		resourceRef, err := b.ix.typeRef("fields")
		if err != nil {
			return nil, "", err
		}
		wrapped = resourceRef.Name
		if b.requestComponent(root.path) != "" {
			requestRef, err := b.ix.typeRef(root.path + "." + r.Policy.RequestWrapper)
			if err != nil {
				return nil, "", err
			}
			wrapped, fieldsPath = requestRef.Name, root.path
		}
		body, wrap, err := b.bodyWrapper(root.path, r.Policy.RequestWrapper, wrapped)
		if err != nil {
			return nil, "", err
		}
		verb := strings.ToUpper(root.path[:1]) + root.path[1:strings.Index(root.path, ".")]
		obj := &convObject{Func: camelize(r.Name) + verb, Model: modelTypeName(r.Name), SDK: wrapped}
		if root.path == "create.body" {
			out.CreateBody, out.CreateWrap = body, wrap
		} else {
			out.UpdateBody, out.UpdateWrap = body, wrap
		}
		return obj, fieldsPath, nil
	}
	function := camelize(ref.Name)
	if root.path == "create.body" {
		out.CreateBody = ref.Name
	} else {
		out.UpdateBody = ref.Name
		if ref.Name == out.CreateBody {
			// A released API can take one body type for Create and Update. Each expands its own fields.
			function += "Update"
		}
	}
	return &convObject{Func: function, Model: modelTypeName(r.Name), SDK: ref.Name}, root.path, nil
}

// checkEmptyError rejects unsupportedIfEmpty on a field that is no pointer to an object.
func checkEmptyError(cf *convField) error {
	if cf.EmptyError != nil && (cf.Conv != convObj || cf.ObjectValue) {
		return fmt.Errorf("unsupportedIfEmpty needs an object field that the SDK holds in a pointer: %s", cf.TFName)
	}
	return nil
}

// checkReadEmptyAs rejects readEmptyAs where the renderer has no such normalization. The flag
// works for an object that is a pointer, and for a list or set of objects. Elsewhere it would
// do nothing, and the released normalization that the line states would be missing.
func checkReadEmptyAs(cf *convField) error {
	if err := checkEmptyError(cf); err != nil {
		return err
	}
	if cf.SendNullAsEmpty && cf.Conv != convString {
		return fmt.Errorf("sendNullAsEmpty is supported for a string, not for the %s field %s", cf.Conv, cf.TFName)
	}
	if !cf.ReadEmptyAsNull {
		return nil
	}
	if cf.Conv == convString || (cf.Conv == convObj && !cf.ObjectValue) || cf.Conv == convObjects || cf.Conv == convStrings ||
		cf.Conv == convKeyedObjects || cf.Conv == convKeyedValues {
		return nil
	}
	return fmt.Errorf("readEmptyAs: \"null\" is supported for a string, an object, a list or set of strings or objects, and a keyedBy map, not for the %s field %s", cf.Conv, cf.TFName)
}

// ValueKind is the Terraform value kind of a string or number field, for example "Int32".
func (f *convField) ValueKind() string {
	switch f.Conv {
	case convString:
		return "String"
	case convInt32:
		return "Int32"
	case convFloat32:
		return "Float32"
	case convFloat64, convFloat64AsFloat32, convFloat32AsFloat64:
		return "Float64"
	case convInt32AsNumber, convFloat64AsNumber:
		return "Number"
	case convInt64AsString:
		return "String"
	}
	return "Int64" // int64, uint64, int64wire
}

// markComputedObjectValue makes a directly nested computed field capable of
// holding the unknown value that Terraform plans before Create.
func (b *convBuilder) markComputedObjectValue(cf *convField, f *model.ResourceField, response bool) {
	// An object of a computed line is optional and computed: every root holds it as a value.
	line := b.resource.Policy.ComputedLine(b.resource.Name, f.Name)
	if !line && (!response || f.Behavior != model.Computed) {
		return
	}
	if f.Type.Kind != model.Object && f.Type.Kind != model.OneOf {
		return
	}
	cf.ObjectValue = true
	if cf.Conv == convObj && !contains(b.listed, cf.Object) {
		b.listed = append(b.listed, cf.Object)
	}
}

// appendCreateField adds a Create field to the unknown-value check. requestValue
// is not a Terraform attribute. The check reads attributes, so a missing one fails
// every Create.
func appendCreateField(out *convData, r *model.Resource, f *model.ResourceField, fixed bool, file *overrides.File) {
	if fixed || f.Create == nil {
		return
	}
	out.CreateFields = appendCheckField(out.CreateFields, r, f, file)
}

// appendCheckField adds the top-level attribute of field f to a request check: the attribute
// itself, or the group that holds it, once. A group holds no field with a server default.
func appendCheckField(fields []*maskField, r *model.Resource, f *model.ResourceField, file *overrides.File) []*maskField {
	if group := file.GroupOf(f.Name); group != "" {
		if slices.ContainsFunc(fields, func(m *maskField) bool { return m.TFName == group }) {
			return fields
		}
		return append(fields, &maskField{TFName: group})
	}
	if splitArms(file, r.Name, f.Name) && f.Type.Elem != nil {
		// The arms are the attributes.
		for _, arm := range f.Type.Elem.Fields {
			fields = append(fields, &maskField{TFName: terraformName(r.Policy, model.ComponentOf(f.Type.Elem), arm.Name)})
		}
		return fields
	}
	return append(fields, &maskField{TFName: terraformName(r.Policy, r.Name, f.Name), ServerDefault: serverDefault(r, f)})
}

// replaceFields sets the Update fields of a full replace. The body has all of
// them, and the server clears a field that the body does not have.
func replaceFields(r *model.Resource, out *convData, fixed func(string) bool, file *overrides.File) error {
	out.Replace = true
	for _, f := range r.Fields {
		// requestValue is not a Terraform attribute. The check reads attributes,
		// so a missing one fails every Update.
		if f.Update != nil && !fixed(f.Name) {
			out.UpdateFields = appendCheckField(out.UpdateFields, r, f, file)
		}
	}
	if len(out.UpdateFields) == 0 {
		return fmt.Errorf("update.body: no Update fields")
	}
	return nil
}

// replaceOnlyLines refuses the lines that need an Update that replaces the resource, on a
// resource that Update patches with a mask. The mask holds attributes. A requestValueBy value is
// not one: when the configuration switches the object that sets it, expand sends the new value
// but the mask leaves it out, so the API can keep the old one. A carrySiblings object shows its
// fields inside the arms, so its Terraform paths are not the API paths that a mask names.
func replaceOnlyLines(file *overrides.File) error {
	if file == nil {
		return nil
	}
	for _, component := range slices.Sorted(maps.Keys(file.Types)) {
		for _, field := range slices.Sorted(maps.Keys(file.Types[component].Fields)) {
			line := file.Types[component].Fields[field]
			if len(line.RequestValueBy) != 0 {
				return fmt.Errorf("%s.%s: requestValueBy needs an Update that replaces the resource; an update mask would leave the derived value out", component, field)
			}
			if line.AsString {
				return fmt.Errorf("%s.%s: asString needs an Update that replaces the resource; the update mask names the arms of the oneOf, not the string that Terraform shows", component, field)
			}
			if line.CarrySiblings != nil {
				return fmt.Errorf("%s.%s: carrySiblings needs an Update that replaces the resource; the update mask names API fields, not the arms that Terraform shows", component, field)
			}
		}
	}
	return nil
}

// buildMask sets the update mask data. Only the Update fields can be in the
// mask, so application, subsystem, and target never are (D9).
func buildMask(r *model.Resource, out *convData, fixed func(string) bool, omit bool, file *overrides.File) error {
	// With updateMask: omit, Update sends no mask, so the mask pattern does not matter: the
	// top-level mask fields only decide whether a value changed.
	valid, leaf := func(string) bool { return true }, false
	if !omit {
		var err error
		if valid, leaf, err = maskRule(r.UpdateMaskPattern); err != nil {
			return err
		}
	}
	out.LeafMask = leaf
	for _, f := range r.Fields {
		if f.Update == nil || fixed(f.Name) {
			continue
		}
		if splitArms(file, r.Name, f.Name) {
			return fmt.Errorf("%s: splitArms needs an Update that replaces the resource; the update mask names API fields", f.Name)
		}
		mf := &maskField{TFName: terraformName(r.Policy, r.Name, f.Name), API: f.Name}
		if leaf {
			mf = maskTree(r.Policy, r.Name, f.Name, f.Type.UpdateType())
		}
		mf.ServerDefault = serverDefault(r, f)
		if err := checkMaskPaths(mf, "", valid); err != nil {
			return err
		}
		out.MaskFields = append(out.MaskFields, mf)
	}
	if len(out.MaskFields) == 0 {
		return fmt.Errorf("update.body: no Update fields for the update mask")
	}
	out.MaskGroups = groupNames(r.Policy, r.Name, r.Groups)
	return nil
}

// groupNames returns the arms of each group as Terraform names.
func groupNames(p model.Policy, component string, groups []model.OneOfGroup) [][]string {
	var out [][]string
	for _, g := range groups {
		var arms []string
		for _, a := range g.Arms {
			arms = append(arms, terraformName(p, component, a))
		}
		out = append(out, arms)
	}
	return out
}

// HasGroups reports whether the update mask has oneOf groups to handle.
func (d *convData) HasGroups() bool {
	var has func(fs []*maskField) bool
	has = func(fs []*maskField) bool {
		for _, f := range fs {
			if len(f.Groups) != 0 || has(f.Children) {
				return true
			}
		}
		return false
	}
	return len(d.MaskGroups) != 0 || has(d.MaskFields)
}

// maskRule returns the check for one mask path, and whether the API accepts
// dotted paths. With a pattern, a path is valid when the pattern accepts it
// as a whole mask. Without one, only a top-level name is valid (F23).
func maskRule(pattern string) (valid func(string) bool, leaf bool, err error) {
	return model.UpdateMaskRule(pattern)
}

// maskTree returns the mask node of the Update field name of type t.
func maskTree(p model.Policy, component, name string, t *model.Type) *maskField {
	n := &maskField{TFName: terraformName(p, component, name), API: name, OneOf: t.Kind == model.OneOf}
	if t.Kind != model.Object && t.Kind != model.OneOf {
		return n
	}
	for _, f := range t.Fields {
		child := maskTree(p, model.ComponentOf(t), f.Name, f.Type)
		if len(t.Wrappers) != 0 {
			// Terraform does not show the wrappers, but the mask path names them.
			child.API = t.WrapperPath() + "." + child.API
		}
		n.Children = append(n.Children, child)
	}
	n.Groups = groupNames(p, model.ComponentOf(t), t.Groups)
	return n
}

// checkMaskPaths checks that the spec pattern accepts the path of n and of
// every node below it.
func checkMaskPaths(n *maskField, prefix string, valid func(string) bool) error {
	p := n.API
	if prefix != "" {
		p = prefix + "." + n.API
	}
	if !valid(p) {
		return fmt.Errorf("update.body.%s: the path is not a valid update mask entry", p)
	}
	for _, c := range n.Children {
		if err := checkMaskPaths(c, p, valid); err != nil {
			return err
		}
	}
	return nil
}

type convBuilder struct {
	ix           *refIndex
	objects      []*convObject
	bySchema     map[string]*convObject
	listed       []*convObject // objects that a list holds
	file         *overrides.File
	resource     *model.Resource
	sent         sentSet // component fields that a request sends in some place
	enumMaps     []*enumMap
	wraps        []*wrapHelper
	projections  []*itemProjection
	keyedObjects []*keyedObject
	dists        map[string]*convDistribution // by the name of the top-level list
}

// serverDefault reports whether removing the field resets it to a declared server default.
// A default or computed: false line in the behavior-overrides file replaces the declared default.
func serverDefault(r *model.Resource, f *model.ResourceField) bool {
	return !r.Policy.OverridesDefault(r.Name, f.Name) && hasServerDefault(f)
}

// bodyWrapper returns the SDK request body type and the field that holds the resource.
func (b *convBuilder) bodyWrapper(bodyPath, wrapper, wrapped string) (string, *bodyWrap, error) {
	body, err := b.ix.typeRef(bodyPath)
	if err != nil {
		return "", nil, err
	}
	field, err := b.ix.fieldRef(bodyPath + "." + wrapper)
	if err != nil {
		return "", nil, err
	}
	switch field.Want {
	case "*" + wrapped:
		return body.Name, &bodyWrap{Field: field.Name}, nil
	case wrapped:
		return body.Name, &bodyWrap{Field: field.Name, Value: true}, nil
	}
	return "", nil, fmt.Errorf("SDK field %s has type %s, want *%s", field.SDKName(), field.Want, wrapped)
}

// enumMapFor returns the mapping of the enum, or nil when the overrides do not state it.
func (b *convBuilder) enumMapFor(schema, sdkType string, t *model.Type) (*enumMap, error) {
	if b.file == nil {
		return nil, nil
	}
	over, ok := b.file.Enums[schema]
	if !ok {
		return nil, nil
	}
	for _, m := range b.enumMaps {
		if m.Name == lowerFirst(camelize(schema)) {
			return m, nil
		}
	}
	m := &enumMap{Name: lowerFirst(camelize(schema)), SDKType: sdkType}
	if over.ZeroNull {
		if t.EnumZero == "" {
			return nil, fmt.Errorf("enum %s has no *_UNSPECIFIED zero value", schema)
		}
		m.ZeroNull = t.EnumZero
	}
	if over.Zero != "" {
		if t.EnumZero == "" {
			return nil, fmt.Errorf("enum %s has no *_UNSPECIFIED zero value", schema)
		}
		m.Values = append(m.Values, enumMapValue{TF: over.Zero, API: t.EnumZero})
	}
	for _, v := range over.Values {
		m.Values = append(m.Values, enumMapValue{TF: over.TFValue(v), API: v})
	}
	switch {
	case over.ReadRejectedAs == "null":
		m.NullValues = slices.Clone(over.Rejected)
	case over.ReadsRejected():
		for _, v := range over.Rejected {
			m.Read = append(m.Read, enumMapValue{TF: over.RejectedTFValue(v), API: v})
		}
	}
	if over.CaseInsensitive {
		// Expand folds the case of the configured value, so two values may not differ only in case.
		for i, a := range m.Values {
			for _, c := range m.Values[i+1:] {
				if strings.EqualFold(a.TF, c.TF) {
					return nil, fmt.Errorf("enum %s: caseInsensitive needs values that differ in more than case: %s and %s", schema, a.TF, c.TF)
				}
			}
		}
	}
	b.enumMaps = append(b.enumMaps, m)
	return m, nil
}

// object adds the convObject of a component schema, without fields.
func (b *convBuilder) object(schema string, ref sdkcheck.Ref) *convObject {
	obj := &convObject{Func: camelize(schema), Model: modelTypeName(schema), SDK: ref.Name}
	obj.AttrTypesFunc = lowerFirst(obj.Func) + "AttrTypes"
	b.bySchema[schema] = obj
	return obj
}

// checkExpandNames rejects two objects with one expand function name: a
// request component that holds two response models.
func checkExpandNames(objects []*convObject) error {
	models := map[string]string{}
	for _, obj := range objects {
		if !obj.Expand {
			continue
		}
		if previous, ok := models[obj.Func]; ok {
			return fmt.Errorf("expand%s converts both %s and %s", obj.Func, previous, obj.Model)
		}
		models[obj.Func] = obj.Model
	}
	return nil
}

func hasPresence(a *model.Attrs) bool {
	return a != nil && a.Presence
}

// keyedFields are the SDK fields of the key and the value of an API item. KeyValue and
// ValueValue are true when the SDK field is a value, not a pointer.
type keyedFields struct {
	Key, Value           string
	KeyValue, ValueValue bool
}

// foldArms are the arms of a oneOf that asString shows as one string. Enum converts the spelling of
// an enum value, and Str holds any other text.
type foldArms struct {
	Enum, Str *convField
	// Empty are the arms of a oneOf of objects without fields: the string is the Terraform name of
	// the arm that is set.
	Empty []emptyFoldArm
}

// foldList holds the arms of a oneOf that typedArms.asList shows as one list: the Go fields of the
// arm without fields and of the arm with the list, the type of that arm, and the Go field of its list.
type foldList struct {
	All, List, ListType, Values string
}

// emptyFoldArm is one arm of a oneOf of objects without fields that asString shows as a string.
type emptyFoldArm struct {
	Value string // the Terraform name of the arm
	SDK   string // the Go field of the oneOf
}

// fixedBy is the API value that expand sends when the object field Model is set.
type fixedBy struct {
	Model, Value string
	// ObjectValue is true when the model holds the object as a types.Object.
	ObjectValue bool
	// Group is the Go field of the top-level group that holds the object, "" for an object outside
	// a group. NestedGroup: a group of types.<Type>.groups holds it, which the model reaches
	// through the object of the component, not through m.
	Group       string
	NestedGroup bool
}

func (b *convBuilder) applyFieldLine(cf *convField, component, name string) {
	if b.file == nil {
		return
	}
	line := b.file.Types[component].Fields[name]
	cf.ReadEmptyAsNull, cf.KeepPriorOrder = line.ReadEmptyAs == "null", line.KeepPriorOrder
	cf.EnumVariant = line.EnumAs
	cf.ReadNullAsEmpty = line.ReadNullAs == "empty"
	if line.ReadNullAs != "" && line.ReadNullAs != "empty" {
		cf.ReadNullAsValue = line.ReadNullAs
	}
	cf.SendNullAsEmpty = line.SendNullAsEmpty
	if t := line.UnsupportedIfEmpty; t != nil {
		cf.EmptyError = &unsupportedArm{SDK: cf.SDK, Summary: t.Summary, Detail: t.Detail}
	}
	cf.ClientID = line.ClientID != ""
	cf.SendUnknownAsZero = line.SendUnknownAsZero
	cf.SplitArms = line.SplitArms
	cf.ShowInverted = line.ShowInverted
	cf.RequestPosition = line.RequestPosition
	cf.Equality = line.Equality
	cf.RequestValue = line.RequestValue
	cf.RequestValueBy = nil
	for _, field := range slices.Sorted(maps.Keys(line.RequestValueBy)) {
		by := fixedBy{Model: camelize(field), Value: line.RequestValueBy[field], ObjectValue: b.resource.Policy.ComputedLine(component, field)}
		if component == b.resource.Name {
			if group := b.file.GroupOf(field); group != "" {
				by.Group = camelize(group)
			}
		} else {
			by.NestedGroup = nestedGroupOf(b.file, component, field) != ""
		}
		cf.RequestValueBy = append(cf.RequestValueBy, by)
	}
}

// applyTerraformType keeps released integer or computed string representations.
func (b *convBuilder) applyTerraformType(cf *convField, component, name string, computed bool) error {
	var line overrides.Field
	if b.file != nil {
		line = b.file.Types[component].Fields[name]
	}
	if line.StoredAs != "" {
		if cf.Conv != convFloat64 {
			return fmt.Errorf("%s: storedAs float32 needs a double field", cf.TFName)
		}
		cf.Conv = convFloat64AsFloat32
	}
	if line.TerraformType == "String" {
		if cf.Conv != convInt32 && cf.Conv != convInt64 {
			return fmt.Errorf("%s: terraformType String needs an integer field", cf.TFName)
		}
		switch {
		case component == b.resource.Name && name == b.resource.ID():
			cf.Conv = convInt64AsString
		case cf.Conv == convInt64 && !computed:
			// A value that a request sends, such as the team id of an owner: the state holds the
			// decimal text and the request holds the number.
			cf.Conv = convInt64AsString
		default:
			cf.Conv = convIntegerAsString
		}
		return nil
	}
	if conv, ok := floatOrNumberType(line.TerraformType, cf.Conv); ok {
		cf.Conv = conv
		return nil
	}
	if line.TerraformType == "Float64" || line.TerraformType == "Number" {
		return fmt.Errorf("%s: terraformType %s needs %s", cf.TFName, line.TerraformType, floatOrNumberNeeds[line.TerraformType])
	}
	if terraformType(b.resource.Policy, component, name) == "" {
		return nil
	}
	switch {
	case cf.Conv == convUint64, cf.Conv == convInt64Wire:
	case cf.Conv == convInt32:
		cf.Conv = convInt32AsInt64
	case cf.Conv == convScalars && cf.SDKType == "int32":
		cf.Conv, cf.ElemType = convInts32AsInt64, "types.Int64Type"
	default:
		return fmt.Errorf("%s: terraformType Int64 needs an int32 field, a 64-bit number that the API sends as a string, or a list or set of int32", cf.TFName)
	}
	return nil
}

// floatOrNumberNeeds is what terraformType Float64 and Number each need, for an error text.
var floatOrNumberNeeds = map[string]string{
	"Float64": "a float32 field",
	"Number":  "an int32 or a double field",
}

// floatOrNumberType returns the conversion that terraformType Float64 (a float32 field) or Number
// (an int32 or a double field) gives to a field with the conversion conv.
func floatOrNumberType(terraformType, conv string) (string, bool) {
	switch {
	case terraformType == "Float64" && conv == convFloat32:
		return convFloat32AsFloat64, true
	case terraformType == "Number" && conv == convInt32:
		return convInt32AsNumber, true
	case terraformType == "Number" && conv == convFloat64:
		return convFloat64AsNumber, true
	}
	return "", false
}

// applyValueNames converts a list or set of integers to the names of its values and back.
func (b *convBuilder) applyValueNames(cf *convField, component, name string) error {
	if b.file == nil {
		return nil
	}
	line := b.file.Types[component].Fields[name]
	if len(line.ValueNames) == 0 {
		return nil
	}
	if cf.Conv != convScalars || cf.SDKType != "int32" && cf.SDKType != "int64" {
		return fmt.Errorf("%s: valueNames needs a list or set of integers", cf.TFName)
	}
	cf.Conv, cf.ElemType, cf.ValueNames = convNamedInts, "types.StringType", line.SortedValueNames()
	return nil
}

func (b *convBuilder) applyFixedRequest(cf *convField) error {
	if cf.RequestValue == "" && len(cf.RequestValueBy) == 0 {
		return nil
	}
	if i := slices.IndexFunc(cf.RequestValueBy, func(by fixedBy) bool { return by.NestedGroup }); i >= 0 {
		return fmt.Errorf("%s: requestValueBy names %s, which is in a group of types.<Type>.groups: only an object outside a group, or in a group of the resource, is supported", cf.TFName, cf.RequestValueBy[i].Model)
	}
	switch {
	case cf.RequestValue == "":
	case cf.Conv == convEnum:
		if cf.EnumMap != nil && !slices.ContainsFunc(cf.EnumMap.Values, func(v enumMapValue) bool { return v.API == cf.RequestValue }) {
			return fmt.Errorf("%s: requestValue %q is no accepted value of the enum %s", cf.TFName, cf.RequestValue, cf.SDKType)
		}
		cf.FixedValue = fmt.Sprintf("%s(%q)", cf.SDKType, cf.RequestValue)
	case cf.Conv == convString:
		cf.FixedValue, cf.SDKType = fmt.Sprintf("%q", cf.RequestValue), "string"
	case cf.Conv == convBool && (cf.RequestValue == "true" || cf.RequestValue == "false"):
		cf.FixedValue, cf.SDKType = cf.RequestValue, "bool"
	default:
		return fmt.Errorf("%s: requestValue %q needs an enum, a string, or a bool field (true or false), not a %s field", cf.TFName, cf.RequestValue, cf.Conv)
	}
	if cf.EnumMap != nil {
		b.releaseEnumMap(cf.EnumMap)
		cf.EnumMap = nil
	}
	cf.Conv = "fixed"
	return nil
}

// applyMissingEnum reads a missing business-first enum as the string default.
// Presence means a missing value was not set, so it stays null. A schema default
// on an enum that has a zero value stays a schema default.
func (b *convBuilder) applyMissingEnum(cf *convField, component, name string, presence bool) {
	if cf.Conv != convEnum || cf.EnumZero != "" || cf.EnumMap == nil || presence || b.file == nil {
		return
	}
	if s, ok := b.file.Types[component].Fields[name].Default.(string); ok {
		cf.Missing = s
	}
}

// fixedRequest reports whether the field is always sent as a fixed API value.
func (b *convBuilder) fixedRequest(component, name string) bool {
	return b.file != nil && b.file.Types[component].Fields[name].Fixed()
}

// field returns the conversion of the property name of the SDK struct at
// owner (an SDK name path).
func (b *convBuilder) field(owner, component, name string, t *model.Type, computed, presence bool) (*convField, error) {
	return b.fieldAt(owner+"."+name, component, name, t, computed, presence)
}

// foldField is the Create field of an object whose properties Create sends as top-level
// fields (createAs). Each property keeps the field lines of the object component and writes
// the SDK field of its top-level Create field.
func (b *convBuilder) foldField(owner string, f *model.ResourceField, folds []model.CreateFold) (*convField, error) {
	cf := &convField{TFName: terraformName(b.resource.Policy, b.resource.Name, f.Name), Model: camelize(f.Name), Conv: convFold}
	if b.resource.Policy.ComputedLine(b.resource.Name, f.Name) {
		// The model holds the object as a types.Object, which is unknown when Create
		// leaves the value to the server.
		cf.ObjectValue, cf.FoldModel = true, modelTypeName(f.Type.Schema)
	}
	for _, fd := range folds {
		inner, err := b.fieldAt(owner+"."+fd.Field, f.Type.Schema, fd.Property, fd.Type, false, true)
		if err != nil {
			return nil, err
		}
		if inner.Conv == convObj || inner.Conv == convObjects || inner.Wrap != nil {
			return nil, fmt.Errorf("createAs: %s.%s is a %s; a folded property must be a scalar, a list of scalars, or an enum", f.Name, fd.Property, inner.Conv)
		}
		cf.Fold = append(cf.Fold, inner)
	}
	return cf, nil
}

// fieldAt is field for the SDK field at refPath, which can differ from the place of the
// Terraform attribute.
func (b *convBuilder) fieldAt(refPath, component, name string, t *model.Type, computed, presence bool) (*convField, error) {
	ref, err := b.ix.fieldRef(refPath)
	if err != nil {
		return nil, err
	}
	cf := &convField{TFName: terraformName(b.resource.Policy, component, name), Model: camelize(name), SDK: ref.Name, refPath: refPath}
	b.applyFieldLine(cf, component, name)
	cf.ReadZeroAsNull, cf.Zero = t.ReadZeroAsNull, t.Zero
	if b.resource.Policy.OptionalLine(component, name) && (strings.HasPrefix(ref.Want, "[]") || strings.HasPrefix(ref.Want, "map[")) {
		cf.SendEmpty, cf.SDKSliceType = true, b.qualifyType(ref.Want)
	}
	want, err := b.fieldConvOf(cf, t, computed, component, name)
	if err != nil {
		return nil, err
	}
	if sep := joinOf(b.file, component, name, t); sep != "" {
		if err := b.applyJoin(cf, sep); err != nil {
			return nil, err
		}
	}
	if sep := splitOf(b.file, component, name); sep != "" {
		if cf.Conv != convString || cf.Wrap != nil {
			return nil, fmt.Errorf("%s: split needs a plain string field", cf.TFName)
		}
		cf.Conv, cf.Split, cf.Collection = convSplit, sep, "Set"
	}
	if b.file != nil && b.file.ReadEmptyAs == "null" && (cf.Conv == convObjects || cf.Conv == convStrings) && !cf.ReadEmptyAsNull && !cf.ReadNullAsEmpty && !cf.SendEmpty {
		cf.ReadEmptyAsNull = true
	}
	if format := durationOf(b.file, component, name); format != "" {
		if cf.Conv != convString || cf.Wrap != nil {
			return nil, fmt.Errorf("%s: durationFormat needs a plain string field", cf.TFName)
		}
		cf.Duration = format
	}
	// A caseInsensitive line acts on the read, and the field of an Update body is only expanded, so
	// a wrapper that only Update holds does not meet it.
	updateOnly := cf.Equality == overrides.EqualityCaseInsensitive && strings.HasPrefix(refPath, "update.body.")
	if cf.Wrap != nil && (cf.KeepPriorOrder || cf.Equality != "" && !updateOnly) {
		return nil, fmt.Errorf("%s: %w", cf.TFName, errUnwrapCombination)
	}
	if err := b.applyValueNames(cf, component, name); err != nil {
		return nil, err
	}
	if err := b.applyTerraformType(cf, component, name, computed); err != nil {
		return nil, err
	}
	if b.resource.Policy.AsBool(component, name) {
		if cf.Conv != convEmpty {
			return nil, fmt.Errorf("%s: asBool needs an object without fields", cf.TFName)
		}
		cf.Conv, cf.Object = convEmptyBool, nil
	}
	if err := b.applyReleasedKeys(cf); err != nil {
		return nil, err
	}
	if err := b.applyUpdateDefault(cf, component, name); err != nil {
		return nil, err
	}
	b.applyMissingEnum(cf, component, name, presence)
	if cf.Conv == convEnum && cf.EnumMap != nil && cf.EnumMap.ZeroNull != "" && b.file != nil && b.file.Types[component].Fields[name].Default != nil {
		// A missing value reads as null, so the planned default would come back as null.
		return nil, fmt.Errorf("%s: zeroNull reads a missing value as null, so the field cannot have a default", cf.TFName)
	}
	if err := checkReadAs(cf); err != nil {
		return nil, err
	}
	return checkFieldType(cf, ref, want)
}

// fieldConvOf is fieldConv for the field name of component: a oneOf that asString shows as one
// string converts as the string.
func (b *convBuilder) fieldConvOf(cf *convField, t *model.Type, computed bool, component, name string) (string, error) {
	if asString(b.file, component, name) {
		return b.foldStringConv(cf, t, component, name)
	}
	return b.fieldConv(cf, t, computed)
}

// foldStringConv sets the conversion of a oneOf of an enum and a string that Terraform shows as one
// string. A spelling of the enum selects the enum arm, and any other text is the string arm, so a
// text that is both would be ambiguous: the generator stops when a pattern of the line matches a
// spelling.
func (b *convBuilder) foldStringConv(cf *convField, t *model.Type, component, name string) (string, error) {
	if arms := emptyArms(t); arms != nil {
		return b.foldEmptyArmsConv(cf, t, arms)
	}
	enum, str, err := stringArms(t)
	if err != nil {
		return "", fmt.Errorf("%s: %w", cf.TFName, err)
	}
	obj, err := b.nested(t)
	if err != nil {
		return "", err
	}
	arms := &foldArms{Enum: fieldByModel(obj.Fields, camelize(enum.Name)), Str: fieldByModel(obj.Fields, camelize(str.Name))}
	switch {
	case arms.Enum == nil || arms.Str == nil:
		return "", fmt.Errorf("%s: asString needs the arms %s and %s to be fields of the oneOf", cf.TFName, enum.Name, str.Name)
	case arms.Enum.EnumMap == nil:
		return "", fmt.Errorf("%s: asString needs an enums line for %s, which gives the Terraform spelling of each value", cf.TFName, enum.Type.Schema)
	case arms.Enum.CaseInsensitive || arms.Enum.Wrap != nil || arms.Str.Wrap != nil || arms.Enum.Value || arms.Str.Value:
		return "", fmt.Errorf("%s: asString needs arms that are pointers, without wrappers, and an enum that is not caseInsensitive", cf.TFName)
	}
	for _, v := range b.file.Types[component].Fields[name].Validators {
		if v.RegexMatches == nil {
			continue
		}
		re, err := regexp.Compile(v.RegexMatches.Pattern)
		if err != nil {
			return "", fmt.Errorf("%s: regexMatches: %w", cf.TFName, err)
		}
		for _, value := range arms.Enum.EnumMap.Values {
			if re.MatchString(value.TF) {
				return "", fmt.Errorf("%s: asString is ambiguous: the text %q is a spelling of the enum and matches the pattern %q of the string arm", cf.TFName, value.TF, v.RegexMatches.Pattern)
			}
		}
	}
	cf.Conv, cf.StringFold, cf.SDKType = convFoldString, arms, b.qualify(obj.SDK)
	return "*" + obj.SDK, nil
}

// foldListConv sets the conversion of a oneOf that typedArms.asList shows as one list of strings.
func (b *convBuilder) foldListConv(cf *convField, t *model.Type) (string, error) {
	all, list, err := listArms(t)
	if err != nil {
		return "", fmt.Errorf("%s: %w", cf.TFName, err)
	}
	obj, err := b.nested(t)
	if err != nil {
		return "", err
	}
	allField, listField := fieldByModel(obj.Fields, camelize(all.Name)), fieldByModel(obj.Fields, camelize(list.Name))
	if allField == nil || listField == nil || listField.Object == nil || len(listField.Object.Fields) != 1 {
		return "", fmt.Errorf("%s: typedArms.asList needs the arms %s and %s", cf.TFName, all.Name, list.Name)
	}
	values := listField.Object.Fields[0]
	if values.Conv != convStrings || values.EnumMap != nil || values.Enum || values.Wrap != nil || values.Projection != nil {
		return "", fmt.Errorf("%s: typedArms.asList needs a list arm with one list of plain strings", cf.TFName)
	}
	cf.Conv, cf.SDKType = convFoldList, b.qualify(obj.SDK)
	cf.ListFold = &foldList{All: allField.SDK, List: listField.SDK, ListType: b.qualify(listField.Object.SDK), Values: values.SDK}
	return "*" + obj.SDK, nil
}

// foldEmptyArmsConv sets the conversion of a oneOf of objects without fields that Terraform shows
// as one string: the name of the arm that is set. The SDK holds each arm as a map.
func (b *convBuilder) foldEmptyArmsConv(cf *convField, t *model.Type, arms []*model.Field) (string, error) {
	obj, err := b.nested(t)
	if err != nil {
		return "", err
	}
	fold := &foldArms{}
	for _, arm := range arms {
		field := fieldByModel(obj.Fields, camelize(arm.Name))
		if field == nil || field.Conv != convEmpty {
			return "", fmt.Errorf("%s: asString needs arms that are objects without fields, and %s is not", cf.TFName, arm.Name)
		}
		fold.Empty = append(fold.Empty, emptyFoldArm{Value: terraformName(b.resource.Policy, t.Schema, arm.Name), SDK: field.SDK})
	}
	cf.Conv, cf.StringFold, cf.SDKType = convFoldString, fold, b.qualify(obj.SDK)
	return "*" + obj.SDK, nil
}

// checkObjects checks the objects that the fields of the resource built.
func checkObjects(objects []*convObject) error {
	if err := checkOptionalGroups(objects); err != nil {
		return err
	}
	return checkPositions(objects)
}

// checkOptionalGroups reports an optional group that has a field whose absence flatten cannot tell: a
// field that the SDK holds as a value, or as a list or a map. A field in wrappers is held by its
// outer wrapper, so that wrapper must be a pointer: a nil wrapper tells that the field is absent. The
// group is null when every field is absent, so each one must be a pointer.
func checkOptionalGroups(objects []*convObject) error {
	for _, obj := range objects {
		if !obj.OptionalGroup {
			continue
		}
		for _, f := range obj.Fields {
			if _, scalar := scalarAttrTypes[f.Conv]; (!scalar && f.Conv != convObj && f.Conv != convGroup) || f.Value || f.Wrap != nil && !strings.HasPrefix(f.Wrap.Outer, "*") {
				return fmt.Errorf("%s: an optional group needs fields that the SDK holds by pointer (a scalar or an object), so that a missing value tells the group is absent; %s is not one", obj.Model, f.TFName)
			}
		}
	}
	return nil
}

// checkPositions reports a requestPosition field outside the items of a list: only a list gives
// an item a place. The component must be a list item in every place, hold one requestPosition
// field, and the requests must send it.
func checkPositions(objects []*convObject) error {
	listed, elsewhere := map[*convObject]bool{}, map[*convObject]bool{}
	for _, obj := range objects {
		for _, f := range obj.Fields {
			switch {
			case f.Object == nil:
			case f.Conv == convObjects && f.Collection == "List":
				listed[f.Object] = true
			default:
				elsewhere[f.Object] = true
			}
		}
	}
	for _, obj := range objects {
		var positions []*convField
		for _, f := range obj.Fields {
			if f.RequestPosition && !f.Carried {
				positions = append(positions, f)
			}
		}
		if len(positions) == 0 {
			continue
		}
		p := positions[0]
		switch {
		case len(positions) > 1:
			return fmt.Errorf("%s: requestPosition on %s and %s: a list item has one requestPosition field", obj.Model, p.TFName, positions[1].TFName)
		case !listed[obj]:
			return fmt.Errorf("%s: requestPosition needs a field of the items of a list, and %s is no list item", p.TFName, obj.Model)
		case elsewhere[obj]:
			return fmt.Errorf("%s: requestPosition needs a component that is a list item in every place, and %s is also held outside a list", p.TFName, obj.Model)
		case p.FlattenOnly:
			return fmt.Errorf("%s: requestPosition needs a field that Create and Update send, and %s only reads it", p.TFName, obj.Model)
		}
	}
	return nil
}

// splitArmsConv sets the conversion of a list of oneOf items that Terraform shows as one collection
// per arm. The object converts one value of an arm; it has no expand or flatten function of its own.
func (b *convBuilder) splitArmsConv(cf *convField, t *model.Type) (string, error) {
	obj, err := b.nested(t.Elem)
	if err != nil {
		return "", err
	}
	for _, arm := range obj.Fields {
		if arm.Conv != convString && arm.Conv != convEnum {
			return "", fmt.Errorf("splitArms: the arm %s is a %s, want a string or an enum", arm.TFName, arm.Conv)
		}
		if arm.CaseInsensitive {
			return "", fmt.Errorf("%s: enums caseInsensitive is for a scalar field of the enum: a splitArms set has no prior value per element to keep", arm.TFName)
		}
	}
	cf.Conv, cf.Object, cf.SDKType = convSplitArms, obj, b.qualify(obj.SDK)
	return "[]" + obj.SDK, nil
}

// applyReleasedKeys applies the keys of a field line that change how a scalar is sent:
// requestPosition, showInverted, requestValue, and sendUnknownAsZero.
func (b *convBuilder) applyReleasedKeys(cf *convField) error {
	if cf.RequestPosition && cf.Conv != convInt64 && cf.Conv != convInt32 {
		return fmt.Errorf("%s: requestPosition needs an int32 or int64 field, not a %s field", cf.TFName, cf.Conv)
	}
	if cf.ShowInverted {
		if cf.Conv != convBool || cf.Wrap != nil {
			return fmt.Errorf("%s: showInverted needs a bool field without wrappers, not a %s field", cf.TFName, cf.Conv)
		}
		cf.Conv = convNotBool
	}
	if err := b.applyFixedRequest(cf); err != nil {
		return err
	}
	return applySendUnknownAsZero(cf)
}

// zeroTypes are the Go types of the scalar conversions that sendUnknownAsZero supports.
var zeroTypes = map[string]string{
	convString: "string", convBool: "bool", convInt32: "int32", convInt64: "int64",
	convFloat32: "float32", convFloat64: "float64", convInt32AsInt64: "int32", convFloat64AsFloat32: "float64",
}

// applySendUnknownAsZero sets the Go type of the zero value that expand sends for an unknown plan.
func applySendUnknownAsZero(cf *convField) error {
	if !cf.SendUnknownAsZero {
		return nil
	}
	zero, ok := zeroTypes[cf.Conv]
	if !ok || cf.Wrap != nil {
		return fmt.Errorf("%s: sendUnknownAsZero needs a string, bool, or number field without wrappers, not a %s field", cf.TFName, cf.Conv)
	}
	cf.ZeroType = zero
	return nil
}

// applyUpdateDefault makes expand send the value of an updateDefault line when the plan of the field
// is unknown. The field has a server default, so the plan is unknown after a removal of a value that
// differs from it, and the API keeps the stored value of a field that the request omits: sending the
// value that a new resource has is what resets it.
func (b *convBuilder) applyUpdateDefault(cf *convField, component, name string) error {
	value, ok := b.resource.Policy.UpdateDefaults[component+"."+name]
	if !ok {
		return nil
	}
	goType, supported := zeroTypes[cf.Conv]
	if !supported || cf.Wrap != nil {
		return fmt.Errorf("%s: updateDefault needs a string, bool, or number field without wrappers, not a %s field", cf.TFName, cf.Conv)
	}
	switch cf.Conv {
	case convString:
		cf.UnknownValue = strconv.Quote(value)
	case convBool:
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("%s: updateDefault %q is not a boolean", cf.TFName, value)
		}
		cf.UnknownValue = strconv.FormatBool(parsed)
	default:
		if _, err := strconv.ParseFloat(value, 64); err != nil {
			return fmt.Errorf("%s: updateDefault %q is not a number", cf.TFName, value)
		}
		cf.UnknownValue = value
	}
	cf.ZeroType = goType
	return nil
}

// enumKey is the key of the enums entry of the field for the contract enum schema.
func (f *convField) enumKey(schema string) string {
	if f.EnumVariant != "" && schema != "" {
		return schema + "@" + f.EnumVariant
	}
	return schema
}

// checkReadAs checks the readEmptyAs and readNullAs keys of the line of cf.
func checkReadAs(cf *convField) error {
	if err := checkReadEmptyAs(cf); err != nil {
		return err
	}
	if cf.ReadNullAsValue != "" {
		if cf.Conv != convEnum || cf.EnumMap == nil {
			return fmt.Errorf("%s: readNullAs: %s needs an enum", cf.TFName, cf.ReadNullAsValue)
		}
		cf.Missing = cf.ReadNullAsValue
	}
	if cf.ReadNullAsEmpty && cf.Conv != convObjects && cf.Conv != convString {
		return fmt.Errorf("%s: readNullAs: empty needs a string, or a list or set of objects", cf.TFName)
	}
	return nil
}

// releaseEnumMap drops a map that no field uses. A fixed request value casts the
// API spelling, so the Terraform map would be an unused variable.
func (b *convBuilder) releaseEnumMap(m *enumMap) {
	for _, obj := range b.objects {
		for _, cf := range obj.Fields {
			if cf.EnumMap == m {
				return
			}
		}
	}
	b.enumMaps = slices.DeleteFunc(b.enumMaps, func(e *enumMap) bool { return e == m })
}

func checkFieldType(cf *convField, ref sdkcheck.Ref, want string) (*convField, error) {
	// The SDK type must be the one the conversion writes, or its value type.
	switch {
	case ref.Want == want:
	case strings.HasPrefix(want, "*") && ref.Want == want[1:]:
		cf.Value = true
	default:
		return nil, fmt.Errorf("SDK field %s has type %s, the %s conversion needs %s", ref.SDKName(), ref.Want, cf.Conv, want)
	}
	return cf, nil
}

// fieldConv sets the conversion of cf for t. It returns the SDK Go type that
// the conversion needs. computed is true for a field that only the response has.
func (b *convBuilder) fieldConv(cf *convField, t *model.Type, computed bool) (string, error) {
	if len(t.Wrappers) != 0 {
		return b.wrappedConv(cf, t, computed)
	}
	if (t.Kind == model.List || t.Kind == model.Set) && len(t.Elem.Wrappers) != 0 {
		return b.wrappedItemsConv(cf, t)
	}
	if t.Keyed != nil && t.Keyed.Object {
		return b.keyedObjectConv(cf, t)
	}
	switch t.Kind {
	case model.String, model.Bool, model.Number, model.Integer:
		return scalarConv(cf, t, computed)
	case model.Enum:
		enum, err := b.ix.schemaRef(t.Schema)
		if err != nil {
			return "", err
		}
		cf.Conv, cf.SDKType, cf.EnumZero = convEnum, b.qualify(enum.Name), t.EnumZero
		m, err := b.enumMapFor(cf.enumKey(t.Schema), b.qualify(enum.Name), t)
		if err != nil {
			return "", err
		}
		cf.EnumMap = m
		cf.CaseInsensitive = m != nil && b.file != nil && b.file.Enums[cf.enumKey(t.Schema)].CaseInsensitive
		return "*" + enum.Name, nil
	case model.Object, model.OneOf:
		if line := b.file.TypedArmsOf(t.Schema); line != nil && line.AsList {
			return b.foldListConv(cf, t)
		}
		if len(t.Fields) == 0 {
			cf.Conv, cf.Object = convEmpty, &convObject{Model: modelTypeName(t.Schema)}
			return "map[string]interface{}", nil
		}
		obj, err := b.nested(t)
		if err != nil {
			return "", err
		}
		cf.Conv, cf.Object = convObj, obj
		return "*" + obj.SDK, nil
	case model.Set, model.List:
		return b.collectionConv(cf, t)
	case model.Map:
		if t.Keyed != nil {
			return b.keyedConv(cf, t)
		}
		return b.mapConv(cf, t)
	}
	return "", fmt.Errorf("kind %s is not supported", t.Kind)
}

// keyedConv sets the conversion of a map that the API holds as a list of items keyed by a
// property (keyedBy). The value is the rest of the item, or one string property of it.
func (b *convBuilder) keyedConv(cf *convField, t *model.Type) (string, error) {
	cf.Collection = "Map"
	item, err := b.ix.schemaRef(t.Keyed.Item.Schema)
	if err != nil {
		return "", err
	}
	key, err := b.ix.fieldRef(item.Path + "." + t.Keyed.Key)
	if err != nil {
		return "", err
	}
	if key.Want != "string" && key.Want != "*string" {
		return "", fmt.Errorf("keyedBy: the key %s has type %s, want a string", key.SDKName(), key.Want)
	}
	cf.Keyed = &keyedFields{Key: key.Name, KeyValue: key.Want == "string"}
	cf.SDKType = b.qualify(item.Name)
	if t.Keyed.Value != "" {
		value, err := b.ix.fieldRef(item.Path + "." + t.Keyed.Value)
		if err != nil {
			return "", err
		}
		if t.Elem.Kind != model.String || value.Want != "string" && value.Want != "*string" {
			return "", fmt.Errorf("keyedBy: the value %s has type %s, want a string", value.SDKName(), value.Want)
		}
		cf.Keyed.Value, cf.Keyed.ValueValue = value.Name, value.Want == "string"
		cf.Conv = convKeyedValues
		return "[]" + item.Name, nil
	}
	obj, err := b.nested(t.Elem)
	if err != nil {
		return "", err
	}
	if !contains(b.listed, obj) {
		b.listed = append(b.listed, obj)
	}
	cf.Conv, cf.Object = convKeyedObjects, obj
	return "[]" + item.Name, nil
}

// scalarConv sets the conversion of cf for a string, bool, number, or
// integer t. It returns the SDK Go type that the conversion needs. A time
// keeps the type of tfschema: see sentTime.
func scalarConv(cf *convField, t *model.Type, computed bool) (string, error) {
	switch {
	case sentTime(t, computed):
		cf.Conv = convRFC3339
		return "*time.Time", nil
	case isTime(t):
		cf.Conv = convTime
		return "*time.Time", nil
	case t.Kind == model.String:
		cf.Conv = convString
		return "*string", nil
	case t.Kind == model.Bool:
		cf.Conv = convBool
		return "*bool", nil
	case t.Kind == model.Number && t.Format == "double":
		cf.Conv = convFloat64
		return "*float64", nil
	case t.Kind == model.Number && t.Format == "float":
		cf.Conv = convFloat32
		return "*float32", nil
	case t.WireString && t.Format == "uint64":
		cf.Conv = convUint64
		return "*string", nil
	case t.WireString && t.Format == "int64":
		cf.Conv = convInt64Wire
		return "*string", nil
	case !t.WireString && t.Format == "int64":
		cf.Conv = convInt64
		return "*int64", nil
	case !t.WireString && t.Format == "int32":
		cf.Conv = convInt32
		return "*int32", nil
	}
	return "", fmt.Errorf("%s format %q is not supported", t.Kind, t.Format)
}

// collectionConv sets the conversion of cf for a Set or List t. It returns
// the SDK Go type that the conversion needs.
// enumsConv is the conversion of a list or set of enum values.
func (b *convBuilder) enumsConv(cf *convField, t *model.Type) (string, error) {
	enum, err := b.ix.schemaRef(t.Elem.Schema)
	if err != nil {
		return "", err
	}
	if b.file != nil && b.file.Enums[cf.enumKey(t.Elem.Schema)].CaseInsensitive {
		return "", fmt.Errorf("%s: enums caseInsensitive (%s) is for a scalar field of the enum: a list or set has no prior value per element to keep", cf.TFName, t.Elem.Schema)
	}
	cf.Conv, cf.SDKType, cf.Enum, cf.EnumZero = convStrings, b.qualify(enum.Name), true, t.Elem.EnumZero
	// An enums line maps each element, the zero value too when the line names it.
	m, err := b.enumMapFor(cf.enumKey(t.Elem.Schema), cf.SDKType, t.Elem)
	if err != nil {
		return "", err
	}
	if m != nil && len(m.NullAPIValues()) != 0 {
		// A list or set has no null element to read the zero value as.
		return "", fmt.Errorf("zeroNull on %s reads its zero value as null, but a list or set of it has no null element", t.Elem.Schema)
	}
	cf.EnumMap = m
	return "[]" + enum.Name, nil
}

func (b *convBuilder) collectionConv(cf *convField, t *model.Type) (string, error) {
	cf.Collection = "Set"
	if t.Kind == model.List {
		cf.Collection = "List"
	}
	if t.Projected != nil {
		return b.projectedConv(cf, t)
	}
	switch t.Elem.Kind {
	case model.String:
		if isTime(t.Elem) {
			cf.Conv, cf.SDKType = convTimes, "time.Time"
			return "[]time.Time", nil
		}
		cf.Conv, cf.SDKType = convStrings, "string"
		return "[]string", nil
	case model.Enum:
		return b.enumsConv(cf, t)
	case model.Bool, model.Number, model.Integer:
		goType, elem, ok := scalarElem(t.Elem)
		if !ok {
			break
		}
		cf.Conv, cf.SDKType, cf.ElemType = convScalars, goType, elem
		return "[]" + goType, nil
	case model.Object, model.OneOf:
		if t.Elem.Kind == model.OneOf && cf.SplitArms {
			return b.splitArmsConv(cf, t)
		}
		if len(t.Elem.Fields) == 0 {
			break
		}
		obj, err := b.nested(t.Elem)
		if err != nil {
			return "", err
		}
		if !contains(b.listed, obj) {
			b.listed = append(b.listed, obj)
		}
		cf.Conv, cf.Object, cf.SDKType = convObjects, obj, b.qualify(obj.SDK)
		return "[]" + obj.SDK, nil
	}
	return "", fmt.Errorf("%s of %s is not supported", t.Kind, t.Elem.Kind)
}

// projectedConv sets the conversion of a list that the resource shows as the strings of one
// property of its items: the response holds the items as objects, so flatten takes the strings
// out of them. It returns the SDK Go type of the response field.
func (b *convBuilder) projectedConv(cf *convField, t *model.Type) (string, error) {
	item, err := b.ix.schemaRef(t.Projected.Schema)
	if err != nil {
		return "", err
	}
	property, err := b.ix.fieldRef(cf.refPath + "." + t.Projected.Property)
	if err != nil {
		return "", err
	}
	p := &itemProjection{
		Func: lowerFirst(item.Name) + property.Name + "s", Item: b.qualify(item.Name), Field: property.Name,
		Pointer: strings.HasPrefix(property.Want, "*"),
	}
	if !slices.ContainsFunc(b.projections, func(o *itemProjection) bool { return o.Func == p.Func }) {
		b.projections = append(b.projections, p)
	}
	cf.Conv, cf.SDKType, cf.Projection = convStrings, "string", p
	return "[]" + item.Name, nil
}

// nested returns the convObject of an object or a oneOf, and fills its
// fields on the first call. A request type whose component is also the
// response component shares the object with the response: the component has
// the same fields in both. A request type with its own component, for example
// ThingSpecCreate for ThingSpec, gets its own object. It expands the response
// model into the request component, and has only the request fields.
func (b *convBuilder) nested(t *model.Type) (*convObject, error) {
	ref, err := b.ix.schemaRef(t.Schema)
	if err != nil {
		return nil, err
	}
	key, component := t.Schema, t.Schema
	if t.Model != "" && t.Model != t.Schema {
		key, component = "request:"+t.Schema, t.Model
	}
	key += t.Variant
	modelName := model.GoName(component) + t.Variant + "Model"
	if obj, ok := b.bySchema[key]; ok {
		if obj.Model != modelName {
			return nil, fmt.Errorf("request component %s holds both %s and %s", t.Schema, obj.Model, modelName)
		}
		// In existing mode, a request that shares the component leaves out its read-only
		// fields. The object serves both, so it gets the fields of either walk. A typedArms
		// object holds the merged fields, not the arms, so it has nothing to add.
		if obj.Typed != nil {
			return obj, b.typedParentsOfBoth(obj, ref.Path, t)
		}
		if err := b.addMissingFields(obj, ref.Path, component, t); err != nil {
			return nil, err
		}
		return obj, nil
	}
	obj := b.object(key, ref)
	obj.Func, obj.Model = camelize(t.Schema)+t.Variant, modelName
	obj.AttrTypesFunc = lowerFirst(obj.Func) + "AttrTypes"
	b.objects = append(b.objects, obj)
	if err := b.addMissingFields(obj, ref.Path, component, t); err != nil {
		return nil, err
	}
	// A read-only field is absent from the request fields. A requestValue is
	// still sent, as the root loop sends it, or Create and Update drop the value
	// the override promised. In a component that the request shares with the
	// response, the object also flattens the response, so it reads the other
	// read-only fields and does not send them.
	if err := b.injectReadOnly(obj, ref.Path, t); err != nil {
		return nil, err
	}
	if err := b.carry(obj, component, t); err != nil {
		return nil, err
	}
	arms, err := b.defaultArms(component, t, obj.Fields)
	if err != nil {
		return nil, err
	}
	obj.DefaultArms = arms
	if obj.Unsupported, err = b.unsupportedArms(t, ref.Path); err != nil {
		return nil, err
	}
	// Last: the steps above read the arms as the fields of the oneOf.
	if err := b.typed(obj, t); err != nil {
		return nil, err
	}
	return obj, nil
}

// typedParentsOfBoth makes the SDK pointer that holds an inlined object a parent of both the request
// and the response conversion of the typedArms object obj. The second walk of a shared component
// adds no field to it, but the pointer that holds the arms is a parent for both.
func (b *convBuilder) typedParentsOfBoth(obj *convObject, sdkPath string, t *model.Type) error {
	for _, f := range t.Fields {
		if err := b.nestedParentsOfBoth(obj, sdkPath, b.file.NestedParents(t.Schema, f.Name)); err != nil {
			return fmt.Errorf("%s: %w", f.Name, err)
		}
	}
	return nil
}

// carry sets obj.Carry when the line of a field of component has carrySiblings, and adds the
// siblings and the fold flags to the arm objects, so their attribute types match the schema.
func (b *convBuilder) carry(obj *convObject, component string, t *model.Type) error {
	if b.file == nil {
		return nil
	}
	var carryName string
	// The API names of the arms: the line names an arm as the API does, and lowerFirst of the Go
	// name is another spelling for an arm such as HTTPServer.
	armNames := map[string]string{}
	for _, f := range t.Fields {
		if b.file.Types[component].Fields[f.Name].CarrySiblings != nil {
			carryName = f.Name
			for _, arm := range f.Type.Fields {
				armNames[camelize(arm.Name)] = arm.Name
			}
		}
	}
	if carryName == "" {
		return nil
	}
	line := b.file.Types[component].Fields[carryName].CarrySiblings
	field := fieldByModel(obj.Fields, camelize(carryName))
	if field == nil || field.Conv != convObj || field.Object == nil {
		return fmt.Errorf("%s.%s: carrySiblings needs a oneOf object field", component, carryName)
	}
	names := apiNames(obj, t)
	oneOf := field.Object
	byArm := map[string]*convField{}
	for _, arm := range oneOf.Fields {
		byArm[arm.Model] = arm
	}
	c := &convCarry{Pkg: b.ix.pkg.Name, Field: field, RequireOne: b.file.Types[component].Fields[carryName].RequireOne}
	for _, armField := range oneOf.Fields {
		armName := armNames[armField.Model]
		if _, folded := line.Fold[armName]; folded {
			continue
		}
		if armField.Conv != convObj || armField.Object == nil {
			return fmt.Errorf("%s.%s: the arm %s is no object", component, carryName, armName)
		}
		arm := &carryArm{TFName: armField.TFName, Model: armField.Model, SDK: armField.SDK, Object: armField.Object}
		if err := b.carrySiblings(arm, obj, field, line.RequestValues[armName], names); err != nil {
			return err
		}
		if err := b.carryFolds(arm, line, armName, byArm); err != nil {
			return fmt.Errorf("%s.%s: %w", component, carryName, err)
		}
		c.Arms = append(c.Arms, arm)
	}
	obj.Carry = c
	return nil
}

// carrySiblings adds the other fields of obj to an arm: a fixed value where the line gives one,
// and else the field, which the arm object also holds for its attribute types.
func (b *convBuilder) carrySiblings(arm *carryArm, obj *convObject, carry *convField, values map[string]string, names map[*convField]string) error {
	for _, sibling := range obj.Fields {
		if sibling == carry {
			continue
		}
		s := sibling
		if value := values[names[sibling]]; value != "" {
			fixed := *sibling
			fixed.RequestValue = value
			if err := b.applyFixedRequest(&fixed); err != nil {
				return err
			}
			s = &fixed
		} else {
			addCarried(arm.Object, sibling)
		}
		arm.Siblings = append(arm.Siblings, s)
	}
	return nil
}

// carryFolds adds the folds into an arm, and their flags to the arm object.
func (b *convBuilder) carryFolds(arm *carryArm, line *overrides.Carry, armName string, byArm map[string]*convField) error {
	for _, folded := range slices.Sorted(maps.Keys(line.Fold)) {
		fold := line.Fold[folded]
		if fold.Into != armName {
			continue
		}
		from := byArm[camelize(folded)]
		if from == nil || from.Object == nil {
			return fmt.Errorf("the folded arm %s is no object", folded)
		}
		f := &carryFold{SDK: from.SDK, SDKType: b.qualify(from.Object.SDK), Flag: camelize(fold.Flag)}
		for _, af := range arm.Object.Fields {
			if !af.Carried {
				f.Fields = append(f.Fields, af.SDK)
			}
		}
		addCarried(arm.Object, &convField{TFName: fold.Flag, Model: f.Flag, Conv: convBool})
		arm.Folds = append(arm.Folds, f)
	}
	return nil
}

// apiNames maps the fields of obj to the API names of the fields of t.
func apiNames(obj *convObject, t *model.Type) map[*convField]string {
	names := map[*convField]string{}
	for _, f := range t.Fields {
		if cf := fieldByModel(obj.Fields, camelize(f.Name)); cf != nil {
			names[cf] = f.Name
		}
	}
	return names
}

// fieldByModel returns the field of fields with the Go model name, or nil.
func fieldByModel(fields []*convField, model string) *convField {
	for _, cf := range fields {
		if cf.Model == model {
			return cf
		}
	}
	return nil
}

// addCarried adds a sibling or a flag to the fields of an arm object, once.
func addCarried(arm *convObject, f *convField) {
	if slices.ContainsFunc(arm.Fields, func(o *convField) bool { return o.Model == f.Model }) {
		return
	}
	carried := *f
	carried.Carried = true
	arm.Fields = append(arm.Fields, &carried)
}

// defaultArms returns the arms of t that expand sends empty when the configuration sets no arm of
// their group. fields are the conversions of the object, which hold the fields of t.
func (b *convBuilder) defaultArms(component string, t *model.Type, fields []*convField) ([]defaultArm, error) {
	byName := map[string]*convField{}
	var lines []string
	for _, f := range t.Fields {
		name := terraformName(b.resource.Policy, component, f.Name)
		if placed, ok := placeFieldName(b.file, t, f.Name); ok {
			name = placed
		}
		cf := findField(fields, name)
		if cf == nil {
			return nil, fmt.Errorf("%s: no conversion of the field", f.Name)
		}
		byName[f.Name] = cf
		if b.resource.Policy.DefaultArm(component, f.Name) {
			lines = append(lines, f.Name)
		}
	}
	if len(lines) == 0 {
		return nil, nil
	}
	if !b.resource.Replace {
		// A PATCH mask names the arm that the plan selects, which is not the arm that expand adds.
		return nil, fmt.Errorf("defaultArm %s needs a full-replace Update (PUT)", strings.Join(lines, ", "))
	}
	groups := t.Groups
	if t.Kind == model.OneOf {
		group := model.OneOfGroup{AllowNone: t.AllowNone}
		for _, f := range t.Fields {
			if f.Name != t.Discriminator {
				group.Arms = append(group.Arms, f.Name)
			}
		}
		groups = append(groups, group)
	}
	var out []defaultArm
	for _, g := range groups {
		var arm string
		for _, name := range g.Arms {
			if !slices.Contains(lines, name) {
				continue
			}
			if arm != "" {
				return nil, fmt.Errorf("defaultArm: %s and %s are arms of one group", arm, name)
			}
			arm = name
		}
		if arm == "" {
			continue
		}
		lines = slices.DeleteFunc(lines, func(name string) bool { return name == arm })
		if nestedGroupOf(b.file, component, arm) != "" {
			return nil, fmt.Errorf("defaultArm %s: an arm in a group of the behavior-overrides file is not supported", arm)
		}
		cf := byName[arm]
		value, err := emptyArm(cf)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", cf.TFName, err)
		}
		d := defaultArm{Field: cf.SDK, Value: value}
		for _, name := range g.Arms {
			arm := byName[name]
			d.Arms = append(d.Arms, arm.SDK)
			if arm.Conv != convObj && arm.Conv != convEmpty || arm.ObjectValue {
				d.Unknown = append(d.Unknown, arm.Model)
			}
		}
		out = append(out, d)
	}
	if len(lines) != 0 {
		return nil, fmt.Errorf("defaultArm: %s is no oneOf arm", strings.Join(lines, ", "))
	}
	return out, nil
}

// emptyArm returns the Go expression of the empty arm f: the empty outer wrapper ({}), not a
// wrapper of an empty value ({"value": []}), as the released resources send it. f must be a list
// of strings in wrapper objects that reads empty as null, so the empty arm reads back as the null
// that the plan has. An empty object arm reads back as an object of nulls, which no plan has.
func emptyArm(f *convField) (string, error) {
	var rest string
	ok := false
	if f.Wrap != nil {
		rest, ok = strings.CutPrefix(f.Wrap.Outer, "*")
	}
	if !ok || f.Conv != convStrings || !f.ReadEmptyAsNull {
		return "", errors.New(`defaultArm needs a list of strings in wrapper objects with readEmptyAs: "null", which reads the empty arm back as null`)
	}
	return "&" + rest + "{}", nil
}

// injectReadOnly adds the response fields that the request type omitted
// because the contract marks them read-only. The SDK struct still has them.
// A requestValue field is sent. In a component that the request shares with
// the response, any other omitted field is flattened and not sent.
func (b *convBuilder) injectReadOnly(obj *convObject, owner string, t *model.Type) error {
	if t.Model == "" {
		return nil
	}
	present := map[string]bool{}
	for _, f := range t.Fields {
		present[f.Name] = true
	}
	added := map[string]bool{}
	for _, cf := range obj.Fields {
		// An earlier use of the shared object can have added the field already.
		added[cf.TFName] = true
	}
	shared := t.Model == t.Schema
	for _, f := range schemaFields(b.resource, t.Model) {
		fixed := b.fixedRequest(t.Model, f.Name)
		if present[f.Name] || added[terraformName(b.resource.Policy, t.Model, f.Name)] || !fixed && !shared {
			continue
		}
		cf, err := b.field(owner, t.Model, f.Name, f.Type, b.sent.responseOnly(t.Model, f), f.Attrs.Presence)
		if err != nil {
			return fmt.Errorf("%s: %w", f.Name, err)
		}
		cf.FlattenOnly = !fixed
		obj.Fields = append(obj.Fields, cf)
	}
	return nil
}

// schemaFields returns the response fields of the nested component schema.
func schemaFields(r *model.Resource, schema string) []*model.Field {
	if r == nil || schema == "" {
		return nil
	}
	seen := map[*model.Type]bool{}
	for _, f := range r.Fields {
		if fields, ok := typeSchemaFields(f.Type, schema, seen); ok {
			return fields
		}
	}
	return nil
}

func typeSchemaFields(t *model.Type, schema string, seen map[*model.Type]bool) ([]*model.Field, bool) {
	if t == nil || seen[t] {
		return nil, false
	}
	seen[t] = true
	if t.Model == "" && t.Schema == schema && (t.Kind == model.Object || t.Kind == model.OneOf) {
		return t.Fields, true
	}
	if fields, ok := typeSchemaFields(t.Elem, schema, seen); ok {
		return fields, true
	}
	for _, f := range t.Fields {
		if fields, ok := typeSchemaFields(f.Type, schema, seen); ok {
			return fields, true
		}
	}
	return nil, false
}

// mark sets the direction that uses obj and its nested objects: expand
// (a request) or flatten (a response). A request holds a time as convRFC3339, never as convTime.
func (b *convBuilder) mark(obj *convObject, expand bool) error {
	if (expand && obj.Expand) || (!expand && obj.Flatten) {
		return nil
	}
	if expand {
		obj.Expand = true
	} else {
		obj.Flatten = true
	}
	if obj.Typed != nil {
		// The merged fields are copies. The arm objects convert each arm.
		for _, arm := range obj.Typed.Arms {
			if arm.Object != nil {
				if err := b.mark(arm.Object, expand); err != nil {
					return fmt.Errorf("%s: %w", arm.Value, err)
				}
			}
		}
	}
	for _, f := range obj.Fields {
		switch {
		case f.Conv == convTime && expand && !f.FlattenOnly:
			return fmt.Errorf("%s: a time that only the response has is in a request", f.TFName)
		case f.Conv == convObj || f.Conv == convObjects || f.Conv == convObjectMap || f.Conv == convGroup || f.Conv == convKeyedObjects || f.Conv == convDistributed:
			if err := b.mark(f.Object, expand); err != nil {
				return fmt.Errorf("%s: %w", f.TFName, err)
			}
		}
	}
	return nil
}

// unmarkCarriedOneOfs turns off the functions of the oneOf objects of carrySiblings objects, and of
// their folded arms. The arms are attributes of the object, so the oneOf has no model struct, and a
// folded arm converts through the arm that Terraform shows. The shown arms stay marked.
func unmarkCarriedOneOfs(objects []*convObject) {
	for _, obj := range objects {
		if obj.Carry == nil {
			continue
		}
		oneOf := obj.Carry.Field.Object
		oneOf.Expand, oneOf.Flatten = false, false
		for _, f := range oneOf.Fields {
			if f.Object != nil && !slices.ContainsFunc(obj.Carry.Arms, func(a *carryArm) bool { return a.Object == f.Object }) {
				f.Object.Expand, f.Object.Flatten = false, false
			}
		}
	}
}

// attrTypes fills the Terraform attribute types of obj, and of the objects
// inside it.
func (b *convBuilder) attrTypes(obj *convObject) error {
	if obj.AttrTypes != nil {
		return nil
	}
	obj.AttrTypes = []convAttrType{}
	if obj.Carry != nil {
		for _, arm := range obj.Carry.Arms {
			if err := b.attrTypes(arm.Object); err != nil {
				return err
			}
			obj.AttrTypes = append(obj.AttrTypes, convAttrType{TFName: arm.TFName, Expr: "types.ObjectType{AttrTypes: " + arm.Object.AttrTypesFunc + "()}"})
		}
		return nil
	}
	for _, f := range obj.Fields {
		if f.Conv == "fixed" {
			continue
		}
		expr, err := b.attrType(obj, f)
		if err != nil {
			return err
		}
		obj.AttrTypes = append(obj.AttrTypes, convAttrType{TFName: f.TFName, Expr: expr})
	}
	return nil
}

// scalarAttrTypes are the Terraform attribute types of the scalar kinds.
var scalarAttrTypes = map[string]string{
	convString: "types.StringType", convSplit: "types.SetType{ElemType: types.StringType}", convTime: "types.StringType", convEnum: "types.StringType", convRFC3339: rfc3339Type,
	convBool: "types.BoolType", convNotBool: "types.BoolType", convFloat64: "types.Float64Type", convFloat32: "types.Float32Type",
	convFoldString: "types.StringType", convJoin: "types.StringType", convFoldList: "types.ListType{ElemType: types.StringType}", convUint64: "types.Int64Type", convInt64Wire: "types.Int64Type", convInt64: "types.Int64Type", convInt32: "types.Int32Type", convInt32AsInt64: "types.Int64Type",
	convIntegerAsString: "types.StringType",
	convInt64AsString:   "types.StringType", convFloat64AsFloat32: "types.Float64Type", convFloat32AsFloat64: "types.Float64Type",
	convInt32AsNumber: "types.NumberType", convFloat64AsNumber: "types.NumberType",
	convEmpty: "types.ObjectType{AttrTypes: map[string]attr.Type{}}", convEmptyBool: "types.BoolType",
}

// attrType returns the Terraform attribute type expression of field f.
func (b *convBuilder) attrType(obj *convObject, f *convField) (string, error) {
	if expr, ok := scalarAttrTypes[f.Conv]; ok {
		return expr, nil
	}
	switch f.Conv {
	case convStrings:
		return "types." + f.Collection + "Type{ElemType: types.StringType}", nil
	case convTimes:
		return "types." + f.Collection + "Type{ElemType: " + rfc3339Type + "}", nil
	case convScalars, convNamedInts, convInts32AsInt64:
		return "types." + f.Collection + "Type{ElemType: " + f.ElemType + "}", nil
	case convScalarMap:
		return "types.MapType{ElemType: " + f.ElemType + "}", nil
	case convStringMap:
		return "types.MapType{ElemType: types.StringType}", nil
	case convUint64Map:
		return "types.MapType{ElemType: types.Int64Type}", nil
	case convKeyedValues:
		return "types.MapType{ElemType: types.StringType}", nil
	case convObj, convObjects, convObjectMap, convGroup, convKeyedObjects, convDistributed:
		if err := b.attrTypes(f.Object); err != nil {
			return "", err
		}
		expr := "types.ObjectType{AttrTypes: " + f.Object.AttrTypesFunc + "()}"
		switch f.Conv {
		case convObjects, convDistributed:
			expr = "types." + f.Collection + "Type{ElemType: " + expr + "}"
		case convObjectMap, convKeyedObjects:
			expr = "types.MapType{ElemType: " + expr + "}"
		}
		return expr, nil
	}
	return "", fmt.Errorf("%s: no attribute type for %s", obj.Model, f.Conv)
}

// mapConv sets the conversion of cf for a map t. It returns the SDK Go type
// that the conversion needs.
func (b *convBuilder) mapConv(cf *convField, t *model.Type) (string, error) {
	cf.Collection = "Map"
	if goType, elem, ok := scalarElem(t.Elem); ok {
		cf.Conv, cf.SDKType, cf.ElemType = convScalarMap, goType, elem
		return "map[string]" + goType, nil
	}
	switch e := t.Elem; {
	case e.Kind == model.String && e.Format == "":
		cf.Conv, cf.SDKType = convStringMap, "string"
		return "map[string]string", nil
	case e.Kind == model.Enum:
		if err := b.rejectEnumLine(e.Schema); err != nil {
			return "", err
		}
		enum, err := b.ix.schemaRef(e.Schema)
		if err != nil {
			return "", err
		}
		cf.Conv, cf.SDKType, cf.Enum, cf.EnumZero = convStringMap, b.qualify(enum.Name), true, e.EnumZero
		return "map[string]" + enum.Name, nil
	case e.Kind == model.Integer && e.WireString && e.Format == "uint64":
		cf.Conv = convUint64Map
		return "map[string]string", nil

	case e.Kind == model.Object && len(e.Fields) != 0:
		obj, err := b.nested(e)
		if err != nil {
			return "", err
		}
		if !contains(b.listed, obj) {
			b.listed = append(b.listed, obj)
		}
		cf.Conv, cf.Object, cf.SDKType = convObjectMap, obj, b.qualify(obj.SDK)
		return "map[string]" + obj.SDK, nil
	}
	return "", fmt.Errorf("map of %s is not supported", typeName(t.Elem))
}

// rejectEnumLine stops an enums line from applying to the items of a list or set, or the values of
// a map. Those conversions cast the Terraform strings to the SDK enum as they are, so the
// Terraform values that the line states would reach the API unchanged and drift on refresh.
func (b *convBuilder) rejectEnumLine(schema string) error {
	if b.resource.Policy.EnumOverride(schema) {
		return fmt.Errorf("the enums line of %s applies to a single enum field or a list or set of it, not to a map of it", schema)
	}
	return nil
}

// scalarElem returns the Go type and the Terraform element type of a bool or
// number element of a list, set, or map. A uint64 (a JSON string) is not one.
func scalarElem(t *model.Type) (goType, elem string, ok bool) {
	switch {
	case t.Kind == model.Bool:
		return "bool", "types.BoolType", true
	case t.Kind == model.Number && t.Format == "double":
		return "float64", "types.Float64Type", true
	case t.Kind == model.Number && t.Format == "float":
		return "float32", "types.Float32Type", true
	case t.Kind == model.Integer && !t.WireString && t.Format == "int64":
		return "int64", "types.Int64Type", true
	case t.Kind == model.Integer && !t.WireString && t.Format == "int32":
		return "int32", "types.Int32Type", true
	}
	return "", "", false
}

// typeName is a short name of a model type for errors.
func typeName(t *model.Type) string {
	if t.Format != "" {
		return string(t.Kind) + " " + t.Format
	}
	return string(t.Kind)
}

// qualify writes an SDK type name with its package name.
func (b *convBuilder) qualify(name string) string { return b.ix.pkg.Name + "." + name }

func contains(objs []*convObject, obj *convObject) bool {
	for _, o := range objs {
		if o == obj {
			return true
		}
	}
	return false
}

// refIndex finds sdkRefs by SDK name path and by component schema.
type refIndex struct {
	pkg      sdkcheck.Ref              // the SDK package of the resource
	pkgs     map[string]sdkcheck.Ref   // package refs by path
	types    map[string]sdkcheck.Ref   // type refs by path
	fields   map[string]sdkcheck.Ref   // field refs by path
	methods  map[string][]sdkcheck.Ref // method refs by path
	funcs    map[string]sdkcheck.Ref   // func refs by path and name
	consts   map[string]sdkcheck.Ref   // const refs by path
	bySchema map[string]sdkcheck.Ref   // component type refs by schema name
}

func indexRefs(refs []sdkcheck.Ref) (*refIndex, error) {
	ix := &refIndex{pkgs: map[string]sdkcheck.Ref{}, types: map[string]sdkcheck.Ref{}, fields: map[string]sdkcheck.Ref{},
		methods: map[string][]sdkcheck.Ref{}, funcs: map[string]sdkcheck.Ref{}, consts: map[string]sdkcheck.Ref{}, bySchema: map[string]sdkcheck.Ref{}}
	for _, ref := range refs {
		switch ref.Kind {
		case sdkcheck.KindPackage:
			if _, ok := ix.pkgs[ref.Path]; ok {
				return nil, fmt.Errorf("more than one SDK package for %s", ref.Path)
			}
			ix.pkgs[ref.Path] = ref
		case sdkcheck.KindType:
			ix.types[ref.Path] = ref
			if ref.Schema != "" {
				ix.bySchema[ref.Schema] = ref
			}
		case sdkcheck.KindField:
			ix.fields[ref.Path] = ref
		case sdkcheck.KindMethod:
			ix.methods[ref.Path] = append(ix.methods[ref.Path], ref)
		case sdkcheck.KindFunc:
			ix.funcs[ref.Path+"."+ref.Name] = ref
		case sdkcheck.KindConst:
			ix.consts[ref.Path] = ref
		}
	}
	pkg, ok := ix.pkgs["resource"]
	if !ok {
		return nil, fmt.Errorf("no SDK package")
	}
	ix.pkg = pkg
	return ix, nil
}

func (ix *refIndex) pkgRef(path string) (sdkcheck.Ref, error) {
	ref, ok := ix.pkgs[path]
	if !ok {
		return sdkcheck.Ref{}, fmt.Errorf("no SDK package for %s", path)
	}
	return ref, nil
}

// methodRef returns the method of owner at path.
func (ix *refIndex) methodRef(path, owner string) (sdkcheck.Ref, error) {
	for _, ref := range ix.methods[path] {
		if ref.Owner == owner {
			return ref, nil
		}
	}
	return sdkcheck.Ref{}, fmt.Errorf("no SDK method of %s for %s", owner, path)
}

// funcRef returns the func name at path.
func (ix *refIndex) funcRef(path, name string) (sdkcheck.Ref, error) {
	ref, ok := ix.funcs[path+"."+name]
	if !ok {
		return sdkcheck.Ref{}, fmt.Errorf("no SDK func %s for %s", name, path)
	}
	return ref, nil
}

func (ix *refIndex) typeRef(path string) (sdkcheck.Ref, error) {
	ref, ok := ix.types[path]
	if !ok {
		return sdkcheck.Ref{}, fmt.Errorf("no SDK type for %s", path)
	}
	return ref, nil
}

func (ix *refIndex) fieldRef(path string) (sdkcheck.Ref, error) {
	ref, ok := ix.fields[path]
	if !ok {
		return sdkcheck.Ref{}, fmt.Errorf("no SDK field for %s", path)
	}
	return ref, nil
}

func (ix *refIndex) schemaRef(schema string) (sdkcheck.Ref, error) {
	ref, ok := ix.bySchema[schema]
	if !ok {
		return sdkcheck.Ref{}, fmt.Errorf("no SDK type for component %s", schema)
	}
	return ref, nil
}

// markPrior sets NeedsPrior on every object with a KeepPriorOrder field, a string that reads
// an empty value as null, an Equality field, or a double stored as a float32, and on every
// object that holds such an object. Only these flatten functions take the prior model. It also
// marks the objects that a list keeping the prior order holds, and their nested objects: the
// package compares their SDK values.
func markPrior(d *convData) error {
	if err := markPriorFields(d); err != nil {
		return err
	}
	propagatePrior(d)
	markSetPriors(d)
	if err := checkPriorContainers(d); err != nil {
		return err
	}
	for _, obj := range d.Objects {
		if !obj.Same {
			continue
		}
		for _, f := range obj.Fields {
			check, err := sameCheck(f)
			if err != nil {
				return fmt.Errorf("%s.%s: %w", obj.Model, f.TFName, err)
			}
			obj.SameChecks = append(obj.SameChecks, check)
		}
	}
	return nil
}

// markPriorFields marks the objects with a KeepPriorOrder field, an Equality field, or a string
// that reads an empty value as null, and the objects that a list keeping the prior order holds.
func markPriorFields(d *convData) error {
	for _, obj := range d.Objects {
		for _, f := range obj.Fields {
			if f.Equality == overrides.EqualityCaseInsensitive {
				// The elements of a list or set of strings keep the spelling of the prior elements.
				if f.Conv != convStrings || f.Enum || f.Projection != nil || f.EnumMap != nil {
					return fmt.Errorf("%s.%s: equality caseInsensitive needs a list or set of plain strings", obj.Model, f.TFName)
				}
				obj.NeedsPrior = true
			} else if f.Equality != "" {
				if f.Conv != convString {
					return fmt.Errorf("%s.%s: equality needs a string field", obj.Model, f.TFName)
				}
				obj.NeedsPrior = true
			}
			// A double that the API stores as a float32 keeps the prior value of the same float32.
			if f.Conv == convFloat64AsFloat32 || f.Conv == convFloat32AsFloat64 || f.Conv == convFloat64AsNumber {
				obj.NeedsPrior = true
			}
			// An enum read in any case keeps the spelling of the prior value.
			if f.CaseInsensitive {
				obj.NeedsPrior = true
			}
			// An empty string from the API is null when the value was omitted, and ""
			// when the configuration set "". Flatten needs the prior value to tell them apart.
			if f.ReadEmptyAsNull && f.Conv == convString {
				obj.NeedsPrior = true
			}
			if !f.KeepPriorOrder {
				continue
			}
			if f.Conv != convObjects || f.Collection != "List" {
				return fmt.Errorf("%s.%s: keepPriorOrder needs a list of objects", obj.Model, f.TFName)
			}
			obj.NeedsPrior = true
			if f.Object.Expand {
				// keepOrder expands the prior items with the response object.
				f.Object.Same = true
				continue
			}
			via, err := requestObject(d, f.Object)
			if err != nil {
				return fmt.Errorf("%s.%s: %w", obj.Model, f.TFName, err)
			}
			f.PriorVia, via.Same = via, true
		}
	}
	return nil
}

// requestObject returns the request object that converts the model of the response object resp:
// the item of a list that the request sends with another component. Create and Update can each
// have one. They must send the same fields, so either pairs the items alike.
func requestObject(d *convData, resp *convObject) (*convObject, error) {
	var found *convObject
	for _, obj := range d.Objects {
		if !obj.Expand || obj == resp || obj.Model != resp.Model {
			continue
		}
		if found != nil && !slices.Equal(sortedFieldNames(found), sortedFieldNames(obj)) {
			return nil, fmt.Errorf("keepPriorOrder needs one set of request fields, but %s and %s send different fields", found.SDK, obj.SDK)
		}
		if found == nil {
			found = obj
		}
	}
	if found == nil {
		return nil, errors.New("keepPriorOrder needs a list that the request sends")
	}
	return found, nil
}

// sortedFieldNames returns the Terraform names of the fields of obj, sorted.
func sortedFieldNames(obj *convObject) []string {
	names := make([]string, 0, len(obj.Fields))
	for _, f := range obj.Fields {
		names = append(names, f.TFName)
	}
	slices.Sort(names)
	return names
}

// markSetPriors marks the objects that a set of objects holds, when they need the prior model: flatten
// pairs each API item with the prior item that is the same value (same<Func>), because a set has no
// order, so the prior item at the same index is not the same item. Their nested objects are compared too.
func markSetPriors(d *convData) {
	marked := false
	for _, obj := range d.Objects {
		for _, f := range obj.Fields {
			if f.Conv == convObjects && f.Collection == "Set" && f.Object != nil && f.Object.NeedsPrior && f.Object.Expand {
				marked = marked || !f.Object.Same
				f.Object.Same, f.SetPrior = true, true
			}
		}
	}
	if marked {
		propagatePrior(d)
	}
}

// errPriorContainer reports a field that needs the prior model in a container whose flatten has no
// prior value to pass: a map of objects, or a computed object stored as types.Object. A set of
// objects has no order, so the prior item at the same index is not the same item.
var errPriorContainer = errors.New("equality, keepPriorOrder, and readEmptyAs on a string are not supported in a map or set of objects or in a computed object")

func checkPriorContainers(d *convData) error {
	for _, obj := range d.Objects {
		for _, f := range obj.Fields {
			if f.Object != nil && f.Object.NeedsPrior && (f.Conv == convObjectMap || f.ObjectValue || f.Conv == convObjects && f.Collection == "Set" && !f.SetPrior) {
				return fmt.Errorf("%s.%s holds %s: %w", obj.Model, f.TFName, f.Object.Model, errPriorContainer)
			}
		}
	}
	return nil
}

// propagatePrior marks every object that holds an object that needs the prior model, and every object
// nested in an object that the package compares.
func propagatePrior(d *convData) {
	for changed := true; changed; {
		changed = false
		for _, obj := range d.Objects {
			if !obj.NeedsPrior && obj.Typed != nil && typedArmNeedsPrior(obj.Typed) {
				obj.NeedsPrior, changed = true, true
			}
			for _, f := range obj.Fields {
				if (f.Conv != convObj && f.Conv != convObjects && f.Conv != convGroup && f.Conv != convKeyedObjects) || f.Object == nil {
					continue
				}
				if f.Object.NeedsPrior && !obj.NeedsPrior {
					obj.NeedsPrior, changed = true, true
				}
				if obj.Same && !f.Object.Same {
					f.Object.Same, changed = true, true
				}
			}
		}
	}
}

// sameCheck is the Go condition that two SDK values, a and b, have an equal field f. A missing
// value and an empty one are equal, as in the released resources.
func sameCheck(f *convField) (string, error) {
	a, b, value := sameOperands(f)
	switch f.Conv {
	case convRFC3339:
		if !value {
			a, b = "pointerValue("+a+")", "pointerValue("+b+")"
		}
		// Two texts of one instant are the same time.
		return a + ".Equal(" + b + ")", nil
	case convTimes:
		if f.Collection == "Set" {
			break
		}
		return "slices.EqualFunc(" + a + ", " + b + ", time.Time.Equal)", nil
	case convString, convBool, convNotBool, convFloat64, convFloat32, convFloat64AsFloat32, convFloat32AsFloat64, convInt32, convInt32AsInt64, convInt32AsNumber, convFloat64AsNumber, convInt64, convInt64AsString, convUint64, convInt64Wire, convEnum, convTime:
		if !value {
			a, b = "pointerValue("+a+")", "pointerValue("+b+")"
		}
		if f.Equality != "" {
			// The API can return the document in another format, so the items compare as documents.
			return f.Equality + "Equal(" + a + ", " + b + ")", nil
		}
		return a + " == " + b, nil
	case convStringMap, convScalarMap, convUint64Map:
		return "maps.Equal(" + a + ", " + b + ")", nil
	case convStrings, convScalars, convNamedInts, convInts32AsInt64:
		if f.Conv == convStrings && f.Equality == overrides.EqualityCaseInsensitive {
			// The API can return an element in another case, so two items with the same elements in
			// any case are the same item. A set has no order.
			if f.Collection == "Set" {
				return "sameUnordered(" + a + ", " + b + ", sameFoldedString)", nil
			}
			return "slices.EqualFunc(" + a + ", " + b + ", strings.EqualFold)", nil
		}
		if f.Collection == "Set" {
			break
		}
		return "slices.Equal(" + a + ", " + b + ")", nil
	case convObj:
		if value {
			return "same" + f.Object.Func + "(&" + a + ", &" + b + ")", nil
		}
		return "sameObject(" + a + ", " + b + ", same" + f.Object.Func + ")", nil
	case convObjects:
		if f.Collection == "Set" || f.KeepPriorOrder {
			return "sameUnordered(" + a + ", " + b + ", same" + f.Object.Func + ")", nil
		}
		return "sameList(" + a + ", " + b + ", same" + f.Object.Func + ")", nil
	}
	return "", fmt.Errorf("a field of kind %s cannot be compared for keepPriorOrder yet", f.Conv)
}

// sameOperands returns the Go expressions of field f in the SDK values a and b, and whether they are
// values, not pointers. A wrapped field compares the values inside its wrappers, because two wrapper
// structs hold pointers.
func sameOperands(f *convField) (a, b string, value bool) {
	a, b = "a."+f.SDK, "b."+f.SDK
	if f.Wrap == nil {
		return a, b, f.Value
	}
	if f.Value {
		a, b = "&"+a, "&"+b
	}
	return f.Wrap.UnwrapOf(a), f.Wrap.UnwrapOf(b), false
}

// UsesPrior reports whether a field keeps the prior order.
func (d *convData) UsesPrior() bool {
	for _, obj := range d.Objects {
		for _, f := range obj.Fields {
			if f.KeepPriorOrder {
				return true
			}
		}
	}
	return false
}

// UsesSetPrior reports whether a set of objects pairs its items with prior items.
func (d *convData) UsesSetPrior() bool {
	for _, obj := range d.Objects {
		for _, f := range obj.Fields {
			if f.SetPrior {
				return true
			}
		}
	}
	return false
}

// UsesPriorVia reports whether a list that keeps the prior order pairs its items with a request object.
func (d *convData) UsesPriorVia() bool {
	for _, obj := range d.Objects {
		for _, f := range obj.Fields {
			if f.PriorVia != nil {
				return true
			}
		}
	}
	return false
}

// PriorLists reports whether flatten reads the items of a prior list of objects.
func (d *convData) PriorLists() bool {
	for _, obj := range d.Objects {
		for _, f := range obj.Fields {
			if f.Conv == convObjects && f.Object != nil && f.Object.NeedsPrior {
				return true
			}
		}
	}
	return false
}

// UsesEquality reports whether a field compares its value as a document of the kind ("yaml" or
// "json"). With no kind, it reports whether any field does.
func (d *convData) UsesEquality(kind ...string) bool {
	for _, k := range d.ExtraEquality {
		if len(kind) == 0 || slices.Contains(kind, k) {
			return true
		}
	}
	return d.UsesFieldEquality(kind...)
}

// UsesFieldEquality is UsesEquality for the equality lines of API fields only. An equality line of an
// extra attribute is a plan modifier of that attribute, and needs no ModifyPlan.
func (d *convData) UsesFieldEquality(kind ...string) bool {
	for _, obj := range d.Objects {
		for _, f := range obj.Fields {
			// A caseInsensitive line is a flatten step, with no document comparison or ModifyPlan.
			if f.Equality != "" && f.Equality != overrides.EqualityCaseInsensitive && (len(kind) == 0 || slices.Contains(kind, f.Equality)) {
				return true
			}
		}
	}
	return false
}

// HasPrior reports whether flatten takes the prior model of the resource.
func (d *convData) HasPrior() bool { return d.Resource != nil && d.Resource.NeedsPrior }

// idGuard returns the model field of the id when flatten must check it, else "". A required
// string id can still decode as empty. An optional id can be absent. Either leaves later
// lifecycle calls without an identity.
func idGuard(r *model.Resource) string {
	if r.Singleton || r.IDType.Kind != model.String && !r.Policy.ClientSetID && !r.Policy.ServerSetID {
		return ""
	}
	return camelize(r.ID())
}

// addToGroup puts the field cf of the root object in its group, or in the group inside it (path is
// the group names, outer first). The group is an object of its own in Terraform, and its fields are
// fields of the SDK struct of the root.
func (b *convBuilder) addToGroup(root *convObject, groups map[string]*convObject, path []string, cf *convField) {
	outer := path[0]
	line := b.file.Groups[outer]
	g, found := groups[outer]
	if !found {
		g = &convObject{Func: root.Func + camelize(outer), Model: groupModelName(b.resource.Name, outer), SDK: root.SDK, Group: true,
			RequireOne: line.RequireOne, OptionalGroup: line.Optional, KeepPriorArm: line.KeepPriorArm, NeedsPrior: line.KeepPriorArm}
		g.AttrTypesFunc = lowerFirst(g.Func) + "AttrTypes"
		groups[outer] = g
		b.objects = append(b.objects, g)
		// The model holds a computed group as a types.Object, which can be unknown in a plan.
		root.Fields = append(root.Fields, &convField{TFName: outer, Model: camelize(outer), Conv: convGroup, Object: g, ObjectValue: line.Computed})
		if line.Computed {
			b.listed = append(b.listed, g)
		}
	}
	if len(path) == 1 {
		g.Fields = append(g.Fields, cf)
		return
	}
	key := innerKey(outer, path[1])
	inner, found := groups[key]
	if !found {
		inner = &convObject{Func: g.Func + camelize(path[1]), Model: groupModelName(b.resource.Name, outer+"_"+path[1]), SDK: root.SDK, Group: true, OptionalGroup: true}
		inner.AttrTypesFunc = lowerFirst(inner.Func) + "AttrTypes"
		groups[key] = inner
		b.objects = append(b.objects, inner)
		g.Fields = append(g.Fields, &convField{TFName: path[1], Model: camelize(path[1]), Conv: convGroup, Object: inner})
	}
	inner.Fields = append(inner.Fields, cf)
}

// typedGroups turns each group of the resource with a typedArms line into a typedArms object: the
// type attribute replaces the arms, which stay the SDK fields that expand sets and flatten reads. The
// arms are objects without fields.
func (b *convBuilder) typedGroups(groups map[string]*convObject) error {
	for name, g := range groups {
		line := b.file.Groups[name].TypedArms
		if line == nil || strings.Contains(name, ".") {
			continue
		}
		values, err := typedValues(&model.Type{Schema: b.resource.Name}, line, groupArmFields(b.resource, b.file.Groups[name]), func(arm string) string { return terraformName(b.resource.Policy, b.resource.Name, arm) })
		if err != nil {
			return fmt.Errorf("groups.%s: %w", name, err)
		}
		typeField := &convField{TFName: line.TypeField(), Model: camelize(line.TypeField()), Conv: convString, Carried: true}
		typed := &convTyped{Pkg: b.ix.pkg.Name, TypeField: typeField}
		var allowed []string
		for _, f := range b.resource.Fields {
			if !slices.Contains(b.file.Groups[name].Fields, f.Name) {
				continue
			}
			cf := fieldByModel(g.Fields, camelize(f.Name))
			if cf == nil || cf.Conv != convEmpty {
				return fmt.Errorf("groups.%s: typedArms needs arms that are objects without fields, and %s is not one", name, f.Name)
			}
			typed.Arms = append(typed.Arms, &typedArm{Value: values[f.Name], SDK: cf.SDK, Empty: true})
			allowed = append(allowed, values[f.Name])
		}
		typed.Allowed = strings.Join(allowed, ", ")
		g.Fields, g.Typed = []*convField{typeField}, typed
	}
	return nil
}

// groupArmFields returns the fields that the group holds, as the arms of a oneOf, in the order of
// the resource.
func groupArmFields(r *model.Resource, line overrides.Group) []*model.Field {
	var arms []*model.Field
	for _, f := range r.Fields {
		if slices.Contains(line.Fields, f.Name) {
			arms = append(arms, &model.Field{Name: f.Name, Type: f.Type})
		}
	}
	return arms
}

// LeafSDK returns the SDK fields that the fields of the object hold, also inside its groups. A
// typedArms object holds the fields of its arms.
func (o *convObject) LeafSDK() []string {
	var out []string
	if o.Typed != nil {
		for _, arm := range o.Typed.Arms {
			out = append(out, arm.SDK)
		}
		return out
	}
	for _, f := range o.Fields {
		if f.Conv == convGroup {
			out = append(out, f.Object.LeafSDK()...)
			continue
		}
		out = append(out, f.SDK)
	}
	return out
}

// addMissingFields adds the fields of t that obj does not have yet. A field that no request
// sends is FlattenOnly: expand leaves it out, and flatten reads it.
func (b *convBuilder) addMissingFields(obj *convObject, sdkPath, component string, t *model.Type) error {
	for _, f := range t.Fields {
		name := terraformName(b.resource.Policy, component, f.Name)
		if placed, ok := placeFieldName(b.file, t, f.Name); ok {
			name = placed // the component has another name for the field at this place
		}
		ftype := placedType(b.file, f.Type, component+"."+f.Name)
		if findField(obj.Fields, name) != nil {
			// The field is there, but an object inside it can still miss fields.
			if err := b.mergeNested(ftype); err != nil {
				return fmt.Errorf("%s: %w", f.Name, err)
			}
			// The component is a request and a response. The first pass made one list of parents.
			if err := b.nestedParentsOfBoth(obj, sdkPath, b.file.NestedParents(t.Schema, f.Name)); err != nil {
				return fmt.Errorf("%s: %w", f.Name, err)
			}
			continue
		}
		responseOnly := b.sent.responseOnly(component, f)
		cf, err := b.field(sdkPath, component, f.Name, ftype, responseOnly, f.Attrs.Presence)
		if err != nil {
			return fmt.Errorf("%s: %w", f.Name, err)
		}
		cf.TFName = name
		cf.FlattenOnly = cf.Conv != "fixed" && responseOnly
		if parents := b.file.NestedParents(t.Schema, f.Name); len(parents) != 0 {
			// An inline line moved the field out of its object: the Go expression goes through it.
			if err := b.nestedInlineParent(obj, sdkPath, t.Model != "", parents, cf, f.Attrs.Required && scalarKind(f.Type)); err != nil {
				return fmt.Errorf("%s: %w", f.Name, err)
			}
		}
		if t.Model != "" && len(f.RequestPath) != 0 {
			// A request object that holds the field in a wrapper or under another name: the Go
			// expression goes through the objects of the request path.
			if err := b.nestedRequestPath(obj, sdkPath, f, cf); err != nil {
				return fmt.Errorf("%s: %w", f.Name, err)
			}
		}
		if (f.Behavior == model.Computed || f.Attrs.Computed) && (f.Type.Kind == model.Object || f.Type.Kind == model.OneOf) {
			// The model holds a computed object as a types.Object, which can
			// be unknown while its parent is known.
			cf.ObjectValue = true
			if cf.Conv == convObj && !contains(b.listed, cf.Object) {
				b.listed = append(b.listed, cf.Object)
			}
		}
		if group := nestedGroupOf(b.file, component, f.Name); group != "" {
			g := b.nestedGroup(obj, component, group)
			g.Fields = append(g.Fields, cf)
			continue
		}
		obj.Fields = append(obj.Fields, cf)
	}
	return nil
}

// nestedGroupOf returns the group of the behavior-overrides file that holds field of component, or "".
func nestedGroupOf(file *overrides.File, component, field string) string {
	if file == nil {
		return ""
	}
	return file.Types[component].GroupOf(field)
}

// nestedGroup returns the group object of obj that holds fields of component in group, and makes it
// on first use. As a top-level group, it is an object of its own in Terraform, and its fields are
// fields of the SDK struct of obj.
func (b *convBuilder) nestedGroup(obj *convObject, component, group string) *convObject {
	for _, cf := range obj.Fields {
		if cf.Conv == convGroup && cf.TFName == group {
			return cf.Object
		}
	}
	line := b.file.Types[component].Groups[group]
	g := &convObject{Func: obj.Func + camelize(group), Model: groupModelName(component, group), SDK: obj.SDK, Group: true,
		RequireOne: line.RequireOne, OptionalGroup: line.Optional}
	g.AttrTypesFunc = lowerFirst(g.Func) + "AttrTypes"
	b.objects = append(b.objects, g)
	// The model holds a computed group as a types.Object, which can be unknown in a plan.
	obj.Fields = append(obj.Fields, &convField{TFName: group, Model: camelize(group), Conv: convGroup, Object: g, ObjectValue: line.Computed})
	if line.Computed {
		b.listed = append(b.listed, g)
	}
	return g
}

// findField returns the field of fields with the Terraform name, also inside a group, or nil.
func findField(fields []*convField, name string) *convField {
	for _, cf := range fields {
		if cf.TFName == name {
			return cf
		}
		if cf.Conv == convGroup {
			if found := findField(cf.Object.Fields, name); found != nil {
				return found
			}
		}
	}
	return nil
}

// mergeNested adds the missing fields of the objects inside t to their shared objects.
func (b *convBuilder) mergeNested(t *model.Type) error {
	if t == nil || len(t.Wrappers) != 0 {
		return nil
	}
	switch t.Kind {
	case model.Object, model.OneOf:
		if len(t.Fields) == 0 {
			return nil
		}
		if t.Keyed != nil && t.Keyed.Object {
			// The object of a keyedBy line has no SDK type: its attributes hold the items.
			for _, f := range t.Fields {
				if err := b.mergeNested(f.Type); err != nil {
					return err
				}
			}
			return nil
		}
		_, err := b.nested(t)
		return err
	case model.List, model.Set, model.Map:
		return b.mergeNested(t.Elem)
	}
	return nil
}

// keyedObject is the local struct of a keyedBy object: a pointer to the API item of each value
// of the key. wrap<Func> turns it into the API list, and unwrap<Func> back.
type keyedObject struct {
	Func, Local string
	Item        string // qualified SDK type of an item
	// Key is the SDK field of the key, KeyType its qualified enum type, and KeyValue is true when
	// the field is a value. KeyAs is the string field that wrap sets to the attribute name.
	Key, KeyType string
	KeyValue     bool
	KeyAs        string
	KeyAsValue   bool
	Values       []keyedObjectValue
}

// keyedObjectValue is one value of the key: its API value, the field of the local struct, and the
// Terraform attribute name.
type keyedObjectValue struct {
	API, Field, TFName string
}

// keyedObjectConv sets the conversion of a list that the resource shows as an object with an
// attribute per value of an enum key (keyedBy object). The object converts to a local struct with
// a field per value, and wrap and unwrap helpers turn that struct into the API list and back.
func (b *convBuilder) keyedObjectConv(cf *convField, t *model.Type) (string, error) {
	item, err := b.ix.schemaRef(t.Keyed.Item.Schema)
	if err != nil {
		return "", err
	}
	key, err := b.ix.fieldRef(item.Path + "." + t.Keyed.Key)
	if err != nil {
		return "", err
	}
	keyType := key.Want
	ko := &keyedObject{Func: camelize(t.Schema) + t.Variant, Item: b.qualify(item.Name), Key: key.Name, KeyValue: !strings.HasPrefix(keyType, "*")}
	ko.Local = lowerFirst(ko.Func)
	ko.KeyType = b.qualify(strings.TrimPrefix(keyType, "*"))
	if t.Keyed.KeyAs != "" {
		as, err := b.ix.fieldRef(item.Path + "." + t.Keyed.KeyAs)
		if err != nil {
			return "", err
		}
		if as.Want != "string" && as.Want != "*string" {
			return "", fmt.Errorf("keyedBy: keyAs %s has type %s, want a string", as.SDKName(), as.Want)
		}
		ko.KeyAs, ko.KeyAsValue = as.Name, as.Want == "string"
	}
	cf.Wrap = &fieldWrap{Inner: "*" + ko.Local, Funcs: []string{ko.Func}}
	cf.Conv = convObj
	// Create, Update, and the response share the object.
	if obj, ok := b.bySchema["keyed:"+ko.Func]; ok {
		cf.Object = obj
		return "[]" + item.Name, nil
	}
	obj := &convObject{Func: ko.Func, Model: typeModelName(t), SDK: ko.Local, Local: true}
	obj.AttrTypesFunc = lowerFirst(obj.Func) + "AttrTypes"
	b.objects = append(b.objects, obj)
	b.bySchema["keyed:"+ko.Func] = obj
	for i, f := range t.Fields {
		v := t.Keyed.Values[i]
		vobj, err := b.nested(f.Type)
		if err != nil {
			return "", fmt.Errorf("%s: %w", f.Name, err)
		}
		field := camelize(f.Name)
		fc := &convField{TFName: terraformName(b.resource.Policy, t.Schema, f.Name), Model: field, SDK: field, Conv: convObj, Object: vobj}
		if f.Attrs.Computed {
			fc.ObjectValue = true
			if !contains(b.listed, vobj) {
				b.listed = append(b.listed, vobj)
			}
		}
		obj.Fields = append(obj.Fields, fc)
		ko.Values = append(ko.Values, keyedObjectValue{API: v.API, Field: field, TFName: fc.TFName})
	}
	b.keyedObjects = append(b.keyedObjects, ko)
	cf.Object = obj
	return "[]" + item.Name, nil
}

// inlinedParents returns the inlined objects that hold top-level fields in the resource response,
// in field order, each once.
func inlinedParents(r *model.Resource, file *overrides.File) []overrides.Parent {
	var out []overrides.Parent
	for _, f := range r.Fields {
		if parent := file.ParentOf("get", f.Name); parent != nil && !slices.Contains(out, *parent) {
			out = append(out, *parent)
		}
	}
	return out
}

// nestedRequestPath points cf, a field of the request object at sdkPath, at the property that its
// request path names, through the objects on the way. An object that the SDK holds in a pointer is
// set by expand before the fields that it holds; one that it holds as a value needs nothing.
func (b *convBuilder) nestedRequestPath(obj *convObject, sdkPath string, f *model.Field, cf *convField) error {
	prefix := ""
	for _, segment := range f.RequestPath[:len(f.RequestPath)-1] {
		via := sdkPath + "." + f.Name + ".via." + segment
		ref, err := b.ix.fieldRef(via)
		if err != nil {
			return err
		}
		typ, err := b.ix.typeRef(via)
		if err != nil {
			return err
		}
		prefix += ref.Name + "."
		switch ref.Want {
		case typ.Name:
		case "*" + typ.Name:
			if at := strings.TrimSuffix(prefix, "."); !slices.ContainsFunc(obj.Parents, func(p convParent) bool { return p.SDK == at }) {
				obj.Parents = append(obj.Parents, convParent{SDK: at, Type: b.qualify(typ.Name), DropEmpty: true})
			}
		default:
			return fmt.Errorf("SDK field %s has type %s, want %s or *%s", ref.SDKName(), ref.Want, typ.Name, typ.Name)
		}
	}
	cf.SDK = prefix + cf.SDK
	if f.Negated {
		if cf.Conv != "bool" {
			return fmt.Errorf("negate needs a bool, the conversion is %s", cf.Conv)
		}
		cf.Negated = true
	}
	return nil
}

// nestedInlineParent points cf, a field that inline lines moved out of the objects in parents (the
// outer one first) of the object at sdkPath, at the field of the innermost one. A request allocates
// each SDK pointer on the way before it writes the field; a response reads a missing one as empty.
func (b *convBuilder) nestedInlineParent(obj *convObject, sdkPath string, request bool, parents []overrides.Parent, cf *convField, requiredScalar bool) error {
	selector, prefix := "", sdkPath
	var pointers []convParent
	for _, parent := range parents {
		plain := prefix + "." + parent.Field
		objectPath := parent.ObjectPath(plain)
		ref, err := b.ix.fieldRef(objectPath)
		if err != nil {
			return err
		}
		typ, err := b.ix.typeRef(objectPath)
		if err != nil {
			return err
		}
		selector += ref.Name
		switch ref.Want {
		case typ.Name:
		case "*" + typ.Name:
			if !request && (cf.Value || requiredScalar) {
				// A missing object flattens as a zero value of its type, so a field that the SDK holds as a
				// value would read as "", false, or 0 instead of null. The SDK holds a required scalar as a
				// value; the contract says so before the SDK exists.
				return fmt.Errorf("%s is an optional object, and its field %s is a value in the SDK, so it would read back as a zero value instead of null when the API returns no object", ref.SDKName(), selector+"."+cf.SDK)
			}
			pointers = append(pointers, convParent{SDK: selector, Type: b.qualify(typ.Name), DropEmpty: request && !parent.Always})
		default:
			return fmt.Errorf("SDK field %s has type %s, want %s or *%s", ref.SDKName(), ref.Want, typ.Name, typ.Name)
		}
		selector += "."
		prefix = plain
	}
	cf.SDK = selector + cf.SDK
	list := &obj.FlattenParents
	if request {
		list = &obj.Parents
	}
	for _, p := range pointers {
		if !slices.Contains(*list, p) {
			*list = append(*list, p)
		}
	}
	if request {
		// The same objects are flattened: a response can leave an object out, as expand does when no
		// attribute of it is set, and flatten reads a missing one as an empty object, with null fields.
		for _, p := range pointers {
			read := convParent{SDK: p.SDK, Type: p.Type}
			if !slices.Contains(obj.FlattenParents, read) {
				obj.FlattenParents = append(obj.FlattenParents, read)
			}
		}
	}
	return nil
}

// nestedParentsOfBoth makes the SDK pointers that hold an inlined object, the outer one first, a
// parent of both the request and the response conversion of obj: a component that a request and a
// response share has one object. An object that the SDK holds as a value needs nothing.
func (b *convBuilder) nestedParentsOfBoth(obj *convObject, sdkPath string, parents []overrides.Parent) error {
	selector, prefix := "", sdkPath
	for _, parent := range parents {
		plain := prefix + "." + parent.Field
		objectPath := parent.ObjectPath(plain)
		ref, err := b.ix.fieldRef(objectPath)
		if err != nil {
			return err
		}
		typ, err := b.ix.typeRef(objectPath)
		if err != nil {
			return err
		}
		selector += ref.Name
		if ref.Want == "*"+typ.Name {
			p := convParent{SDK: selector, Type: b.qualify(typ.Name)}
			if q := (convParent{SDK: selector, Type: p.Type, DropEmpty: !parent.Always}); !slices.Contains(obj.Parents, q) {
				obj.Parents = append(obj.Parents, q)
			}
			if !slices.Contains(obj.FlattenParents, p) {
				obj.FlattenParents = append(obj.FlattenParents, p)
			}
		}
		selector += "."
		prefix = plain
	}
	return nil
}

// scalarKind reports whether t is a plain value: a string, a bool, a number, or an enum.
func scalarKind(t *model.Type) bool {
	if t == nil {
		return false
	}
	switch t.Kind {
	case model.String, model.Bool, model.Number, model.Integer, model.Enum:
		return true
	}
	return false
}
