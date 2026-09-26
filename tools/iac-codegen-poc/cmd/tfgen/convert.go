package main

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen-poc/internal/model"
)

// convData is the template data for expand (Terraform model → SDK request)
// and flatten (SDK response → Terraform model). Every SDK name comes from
// the checked sdkRefs.
type convData struct {
	SDKPkg  string // import path of the SDK package
	SDKName string // package name
	// The root objects: the Create body and the Update body (expand), and
	// the resource (flatten).
	Create, Update, Resource *convObject
	Objects                  []*convObject // all objects, roots first
	// UpdateMask is the SDK field of the update mask in the Update body.
	UpdateMask string
	// MaskFields are the Update fields, in model order. The update mask
	// lists the API names of the fields that changed (D8).
	MaskFields []*maskField
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
	UpdateFields []string
	// Exported is true in the type mode (D20): other packages use the
	// attribute types functions, so they have doc comments.
	Exported bool
}

// maskField is one top-level Update field. With leaf masks, it is also a
// node of the mask tree: Children are the fields of an object or the arms of
// a oneOf. A node without children is compared as a whole: a scalar, a list,
// a set, a map, or an object with no fields.
type maskField struct {
	TFName   string // Terraform attribute name, to read the plan and the state
	API      string // API property name, the update mask entry
	OneOf    bool
	Children []*maskField
	// Groups are the oneOf groups among Children, as Terraform names.
	Groups [][]string
}

// maskEntry is one entry of the update mask when the spec has no pattern
// (F23): a top-level name. The contract allows no "*".
var maskEntry = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

// convObject is one pair of Terraform model struct and SDK struct.
type convObject struct {
	Func    string // function name suffix: expand<Func>, flatten<Func>
	Model   string // Terraform model struct
	SDK     string // SDK struct
	Fields  []*convField
	Expand  bool // generate expand<Func>
	Flatten bool // generate flatten<Func>
	// AttrTypes are the Terraform attribute types, when a list holds the
	// object. The function AttrTypesFunc returns them.
	AttrTypes     []convAttrType
	AttrTypesFunc string
	// Wrapper is true for the object of a wrapper (D21): SDK is the SDK
	// struct of the parent, and expand and flatten set and read its fields.
	// NilCheck is the Go condition that is true when the API response has
	// none of the wrapped fields; then flatten returns nil. "" when a
	// wrapped field is a value, which is never missing.
	Wrapper  bool
	NilCheck string
	// Unwrap is true for an unwrapped object (D21, unwrap.go): Terraform
	// shows it as its only field. Its expand takes, and its flatten
	// returns, a model value of the Go type ValueType.
	Unwrap    bool
	ValueType string
	// Inline is true for the object of an inlined field (D21, inline.go):
	// Model is the parent model, which holds the fields, and SDK is the
	// nested SDK struct. UnsetCheck is the Go condition that is true when
	// none of its fields is set; then expand sends no object.
	Inline     bool
	UnsetCheck string
}

type convAttrType struct {
	TFName string
	Expr   string
}

// Conversion kinds of a convField.
const (
	convString  = "string"  // types.String ↔ *string
	convBool    = "bool"    // types.Bool ↔ *bool
	convFloat64 = "float64" // types.Float64 ↔ *float64
	convUint64  = "uint64"  // types.Int64 ↔ *string (D7)
	convTime    = "time"    // types.String ↔ *time.Time, RFC 3339 in UTC (F51)
	convEnum    = "enum"    // types.String ↔ *<enum type> (D12)
	convObj     = "object"  // *<Model> ↔ *<SDK type>
	convEmpty   = "empty"   // *<Model> ↔ map[string]interface{} (F16)
	convStrings = "strings" // types.Set/List ↔ []string or []<enum type>
	convObjects = "objects" // types.List ↔ []<SDK type>

	convInt32 = "int32" // types.Int32 ↔ *int32
	convInt64 = "int64" // types.Int64 ↔ *int64 (signed, a JSON number)
	// convInt64String is a signed 64-bit number that JSON sends as a string:
	// types.Int64 ↔ *string, like uint64 (F68).
	convInt64String = "int64string"
	convFloat32     = "float32" // types.Float32 ↔ *float32
	convScalars     = "scalars" // types.List/Set ↔ []bool, []int32, []int64, []float32, []float64

	convStringMap = "stringmap" // types.Map ↔ map[string]string or map[string]<enum type>
	convScalarMap = "scalarmap" // types.Map ↔ map[string]bool, int32, int64, float32, float64
	convUint64Map = "uint64map" // types.Map of Int64 ↔ map[string]string (D7)
	convObjectMap = "objectmap" // types.Map ↔ map[string]<SDK type>

	// Overrides of the type mode (D21).
	convInt32Wide   = "int32wide"   // types.Int64 ↔ *int32, with a range check
	convFloat32Wide = "float32wide" // types.Float64 ↔ *float32
	convEnumName    = "enumname"    // types.String (a Terraform name) ↔ *<enum type>
	convEnumNames   = "enumnames"   // types.Set/List of Terraform names ↔ []<enum type>
	convWrap        = "wrap"        // *<wrapper Model> ↔ fields of the same SDK struct (see wrapper)
	// convObjValue is a computed object: types.Object ↔ *<SDK type>.
	// Terraform plans it as unknown, which a struct pointer cannot hold.
	convObjValue = "objectvalue"
	convUnwrap   = "unwrap"  // the value of the only field ↔ *<SDK type> (unwrap.go)
	convUnwraps  = "unwraps" // types.Set/List of those values ↔ []<SDK type>
	convInline   = "inline"  // fields of the parent model ↔ *<SDK type> (inline.go)
	// convInt64Text is the string override (F68): types.String with the
	// decimal number ↔ *int64.
	convInt64Text = "int64text"
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
	// Value is true when the SDK field is a value, not a pointer. The SDK
	// does that for a required field (F18). Expand sends the zero value for
	// null; the schema requires the attribute, so it is not null.
	Value bool
	// Enum is the Go prefix of the enum name maps of enumname and
	// enumnames: <Enum>ByName and <Enum>Name. Zero is the SDK value that
	// only means "not set", as a Go expression; flatten makes it null.
	Enum, Zero string
	// Read is how flatten reads a missing or empty value, from the
	// overrides (D21): "" as it is, readZero or readNull.
	Read string
	// ProtoZero is the SDK constant of the proto zero value of an enum (its
	// first value), for readZeroEnum: a response leaves it out.
	ProtoZero string
	// ReadOnly is true for a field that the server sets (the readOnly
	// override): expand does not send it.
	ReadOnly bool
	// Unwrapped is true for the only field of an unwrapped object: its
	// diagnostics have the path of the attribute, p.
	Unwrapped bool
}

// Values of convField.Read.
const (
	readZero      = "zero"       // a missing scalar → its zero value
	readEmptyList = "emptylist"  // a missing list → an empty list
	readEmptyMap  = "emptymap"   // a missing map → an empty map
	readNullList  = "nulllist"   // an empty list or map → null
	readNullObj   = "nullobject" // an object with no fields set → null
	readZeroEnum  = "zeroenum"   // a missing enum → its proto zero value (ProtoZero)
)

// Normalizes reports whether flatten of obj changes a missing or empty
// value first (D21).
func (obj *convObject) Normalizes() bool {
	for _, f := range obj.Fields {
		if f.Read != "" {
			return true
		}
	}
	return false
}

// Reads reports whether a field is read with the rule read.
func (d *convData) Reads(read string) bool {
	for _, obj := range d.Objects {
		for _, f := range obj.Fields {
			if obj.Flatten && f.Read == read {
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

// ExpandsTime reports whether a request has a timestamp. The template then
// emits the expandTime helper.
func (d *convData) ExpandsTime() bool {
	for _, obj := range d.Objects {
		for _, f := range obj.Fields {
			if obj.Expand && f.Conv == convTime {
				return true
			}
		}
	}
	return false
}

// HasValue reports whether an expanded SDK field is a value, not a pointer.
// The template then emits the valueOf helper.
func (d *convData) HasValue() bool {
	for _, obj := range d.Objects {
		for _, f := range obj.Fields {
			if obj.Expand && f.Value {
				return true
			}
		}
	}
	return false
}

// buildConv maps the model and its SDK names to the conversion data. Rules:
//   - A null Terraform value → nil. nil → a null Terraform value. So
//     false, 0, "", [] and {} are sent and read as values.
//   - An empty object (F16) is a non-nil empty map when it is set.
//   - uint64 → the decimal string (D7). An enum → its API value (D12).
//   - The Update body has only the Update fields, so application,
//     subsystem, and target are never set (D9).
func buildConv(r *model.Resource, refs []sdkRef) (*convData, error) {
	ix, err := indexRefs(refs)
	if err != nil {
		return nil, err
	}
	b := &convBuilder{ix: ix, bySchema: map[string]*convObject{}}
	out := &convData{SDKPkg: ix.pkg.Pkg, SDKName: ix.pkg.Name}

	roots := []struct {
		obj    **convObject
		path   string // SDK name path of the struct
		expand bool
		has    func(*model.ResourceField) bool
	}{
		{&out.Create, "create.body", true, func(f *model.ResourceField) bool { return f.Create != nil }},
		{&out.Update, "update.body", true, func(f *model.ResourceField) bool { return f.Update != nil }},
		{&out.Resource, "fields", false, func(f *model.ResourceField) bool { return f.InGet }},
	}
	for _, root := range roots {
		ref, err := ix.typeRef(root.path)
		if err != nil {
			return nil, err
		}
		obj := &convObject{Func: camelize(ref.Name), Model: modelTypeName(r.Name), SDK: ref.Name}
		if root.path == "fields" {
			obj = b.object(r.Name, ref)
		}
		b.objects = append(b.objects, obj)
		for _, f := range r.Fields {
			if !root.has(f) {
				continue
			}
			cf, err := b.field(root.path, f.Name, f.Type)
			if err != nil {
				return nil, fmt.Errorf("%s.%s: %w", root.path, f.Name, err)
			}
			if f.Behavior == model.Computed {
				if err := b.objectValue(cf); err != nil {
					return nil, fmt.Errorf("%s.%s: %w", root.path, f.Name, err)
				}
			}
			obj.Fields = append(obj.Fields, cf)
		}
		if err := b.mark(obj, root.expand); err != nil {
			return nil, fmt.Errorf("%s: %w", root.path, err)
		}
		*root.obj = obj
	}
	for _, obj := range b.listed {
		if err := b.attrTypes(obj); err != nil {
			return nil, err
		}
	}
	out.Objects = b.objects
	if r.Replace {
		return out, replaceFields(r, out)
	}
	if err := buildMask(r, ix, out); err != nil {
		return nil, err
	}
	return out, nil
}

// replaceFields sets the Update fields of a full replace. The body has all of
// them, and the server clears a field that the body does not have.
func replaceFields(r *model.Resource, out *convData) error {
	out.Replace = true
	for _, f := range r.Fields {
		if f.Update != nil {
			out.UpdateFields = append(out.UpdateFields, tfName(f.Name))
		}
	}
	if len(out.UpdateFields) == 0 {
		return fmt.Errorf("update.body: no Update fields")
	}
	return nil
}

// buildMask sets the update mask data. Only the Update fields can be in the
// mask, so application, subsystem, and target never are (D9). The SDK name
// of the mask comes from the checked refs.
func buildMask(r *model.Resource, ix *refIndex, out *convData) error {
	ref, err := ix.fieldRef("update.body." + r.UpdateMask)
	if err != nil {
		return err
	}
	if ref.Want != "*string" {
		return fmt.Errorf("SDK field %s has type %s, the update mask needs *string", ref.sdkName(), ref.Want)
	}
	out.UpdateMask = ref.Name
	valid, leaf, err := maskRule(r.UpdateMaskPattern)
	if err != nil {
		return err
	}
	out.LeafMask = leaf
	for _, f := range r.Fields {
		if f.Update == nil {
			continue
		}
		mf := &maskField{TFName: tfName(f.Name), API: f.Name}
		if leaf {
			mf = maskTree(f.Name, f.Type)
		}
		if err := checkMaskPaths(mf, "", valid); err != nil {
			return err
		}
		out.MaskFields = append(out.MaskFields, mf)
	}
	if len(out.MaskFields) == 0 {
		return fmt.Errorf("update.body: no Update fields for the update mask")
	}
	out.MaskGroups = groupNames(r.Groups)
	return nil
}

// groupNames returns the arms of each group as Terraform names.
func groupNames(groups []model.OneOfGroup) [][]string {
	var out [][]string
	for _, g := range groups {
		var arms []string
		for _, a := range g.Arms {
			arms = append(arms, tfName(a))
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
	if pattern == "" {
		return maskEntry.MatchString, false, nil
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, false, fmt.Errorf("update mask pattern %q: %w", pattern, err)
	}
	if !re.MatchString("a") || re.MatchString("*") {
		return nil, false, fmt.Errorf("update mask pattern %q must accept a field name and reject *", pattern)
	}
	return re.MatchString, re.MatchString("a.b"), nil
}

// maskTree returns the mask node of the Update field name of type t.
func maskTree(name string, t *model.Type) *maskField {
	n := &maskField{TFName: tfName(name), API: name, OneOf: t.Kind == model.OneOf}
	if t.Kind != model.Object && t.Kind != model.OneOf {
		return n
	}
	for _, f := range t.Fields {
		n.Children = append(n.Children, maskTree(f.Name, f.Type))
	}
	n.Groups = groupNames(t.Groups)
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
	ix       *refIndex
	objects  []*convObject
	bySchema map[string]*convObject
	listed   []*convObject // objects that a list holds
	ov       *overrides    // the overrides of the type mode (D21); nil in the resource mode
}

// object adds the convObject of a component schema, without fields.
func (b *convBuilder) object(schema string, ref sdkRef) *convObject {
	obj := &convObject{Func: camelize(schema), Model: modelTypeName(schema), SDK: ref.Name}
	obj.AttrTypesFunc = lowerFirst(obj.Func) + "AttrTypes"
	b.bySchema[schema] = obj
	return obj
}

// field returns the conversion of the property name of the SDK struct at
// owner (an SDK name path).
func (b *convBuilder) field(owner, name string, t *model.Type) (*convField, error) {
	ref, err := b.ix.fieldRef(owner + "." + name)
	if err != nil {
		return nil, err
	}
	cf := &convField{TFName: tfName(name), Model: camelize(name), SDK: ref.Name}
	want, err := b.fieldConv(cf, t)
	if err == nil && b.ov.wide() && isNarrow(t) {
		want, err = wideConv(cf, t)
	}
	if err != nil {
		return nil, err
	}
	// The SDK type must be the one the conversion writes, or its value type.
	switch {
	case ref.Want == want:
	case strings.HasPrefix(want, "*") && ref.Want == want[1:]:
		cf.Value = true
	default:
		return nil, fmt.Errorf("SDK field %s has type %s, the %s conversion needs %s", ref.sdkName(), ref.Want, cf.Conv, want)
	}
	return cf, nil
}

// fieldConv sets the conversion of cf for t. It returns the SDK Go type that
// the conversion needs.
func (b *convBuilder) fieldConv(cf *convField, t *model.Type) (string, error) {
	switch t.Kind {
	case model.String, model.Bool, model.Number, model.Integer:
		return scalarConv(cf, t)
	case model.Enum:
		enum, err := b.ix.schemaRef(t.Schema)
		if err != nil {
			return "", err
		}
		cf.Conv, cf.SDKType = convEnum, b.qualify(enum.Name)
		if b.ov.named(t.Schema) {
			cf.Conv = convEnumName
			b.enumNames(cf, t, enum)
		}
		return "*" + enum.Name, nil
	case model.Object, model.OneOf:
		if b.ov.unwrapped(t.Schema) {
			obj, err := b.unwrapObject(t)
			if err != nil {
				return "", err
			}
			cf.Conv, cf.Object = convUnwrap, obj
			return "*" + obj.SDK, nil
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
		return b.mapConv(cf, t)
	}
	return "", fmt.Errorf("kind %s is not supported", t.Kind)
}

// scalarConv sets the conversion of cf for a string, bool, number, or
// integer t. It returns the SDK Go type that the conversion needs.
func scalarConv(cf *convField, t *model.Type) (string, error) {
	switch {
	case t.Kind == model.String && t.Format == "date-time":
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
	case t.Kind == model.Integer:
		return integerConv(cf, t)
	}
	return "", fmt.Errorf("%s format %q is not supported", t.Kind, t.Format)
}

// integerConv sets the conversion of cf for an integer t. A 64-bit number
// that JSON sends as a string is a string in the SDK (D7, F68).
func integerConv(cf *convField, t *model.Type) (string, error) {
	switch {
	case t.WireString && t.Format == "uint64":
		cf.Conv = convUint64
		return "*string", nil
	case t.WireString && t.Format == "int64":
		cf.Conv = convInt64String
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
func (b *convBuilder) collectionConv(cf *convField, t *model.Type) (string, error) {
	cf.Collection = "Set"
	if t.Kind == model.List {
		cf.Collection = "List"
	}
	switch t.Elem.Kind {
	case model.String:
		cf.Conv, cf.SDKType = convStrings, "string"
		return "[]string", nil
	case model.Enum:
		enum, err := b.ix.schemaRef(t.Elem.Schema)
		if err != nil {
			return "", err
		}
		cf.Conv, cf.SDKType = convStrings, b.qualify(enum.Name)
		if b.ov.named(t.Elem.Schema) {
			cf.Conv = convEnumNames
			b.enumNames(cf, t.Elem, enum)
		}
		return "[]" + enum.Name, nil
	case model.Bool, model.Number, model.Integer:
		if b.ov.wide() && isNarrow(t.Elem) {
			break
		}
		goType, elem, ok := scalarElem(t.Elem)
		if !ok {
			break
		}
		cf.Conv, cf.SDKType, cf.ElemType = convScalars, goType, elem
		return "[]" + goType, nil
	case model.Object, model.OneOf:
		if len(t.Elem.Fields) != 0 {
			return b.objectsConv(cf, t)
		}
	}
	return "", fmt.Errorf("%s of %s is not supported", t.Kind, t.Elem.Kind)
}

// objectsConv sets the conversion of cf for a Set or List of objects t. A
// diagnostic in a set item has the path of the set: an item path
// (path.AtSetValue) needs the Terraform value of the item.
func (b *convBuilder) objectsConv(cf *convField, t *model.Type) (string, error) {
	if b.ov.unwrapped(t.Elem.Schema) {
		obj, err := b.unwrapObject(t.Elem)
		if err != nil {
			return "", err
		}
		cf.Conv, cf.Object, cf.SDKType = convUnwraps, obj, b.qualify(obj.SDK)
		return "[]" + obj.SDK, nil
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

// nested returns the convObject of an object or a oneOf, and fills its
// fields on the first call.
func (b *convBuilder) nested(t *model.Type) (*convObject, error) {
	ref, err := b.ix.schemaRef(t.Schema)
	if err != nil {
		return nil, err
	}
	if obj, ok := b.bySchema[t.Schema]; ok {
		return obj, nil
	}
	obj := b.object(t.Schema, ref)
	b.objects = append(b.objects, obj)
	for _, f := range b.ov.fields(t) {
		var cf *convField
		if b.ov.inlined(t.Schema, f) {
			cf, err = b.inline(obj, ref.Path, f)
		} else {
			cf, err = b.objectField(ref.Path, t.Schema, f)
		}
		if err != nil {
			return nil, fmt.Errorf("%s: %w", f.Name, err)
		}
		obj.Fields = append(obj.Fields, cf)
	}
	b.wrap(t, obj)
	return obj, nil
}

// objectField returns the conversion of the field f of the component schema,
// whose SDK struct is at owner (an SDK name path), with its overrides.
func (b *convBuilder) objectField(owner, schema string, f *model.Field) (*convField, error) {
	cf, err := b.field(owner, f.Name, b.ov.fieldType(schema, f))
	if err != nil {
		return nil, err
	}
	cf.TFName = b.ov.tfName(schema, f.Name)
	cf.ReadOnly = b.ov.effective(schema, f).ReadOnly
	if b.ov.field(schema, f.Name).String {
		// The SDK keeps its int64; Terraform has the decimal string (F68).
		// checkNumberText allows the override only on an int64.
		cf.Conv = convInt64Text
	}
	if _, _, comp := flags(b.ov.effective(schema, f), f.Attrs); comp && b.ov != nil {
		if err := b.objectValue(cf); err != nil {
			return nil, err
		}
	}
	if cf.Read, err = b.readRule(b.ov.field(schema, f.Name), cf, f.Type); err != nil {
		return nil, err
	}
	return cf, nil
}

// objectValue makes a computed object field a types.Object (convObjValue).
func (b *convBuilder) objectValue(cf *convField) error {
	if cf.Conv == convUnwrap && strings.HasPrefix(cf.Object.ValueType, "*") {
		return errors.New("unwrap: a computed field of an unwrapped object whose value is an object is not supported")
	}
	if cf.Conv != convObj {
		return nil
	}
	cf.Conv = convObjValue
	if !contains(b.listed, cf.Object) {
		b.listed = append(b.listed, cf.Object)
	}
	return nil
}

// unwrapObject returns the convObject of the unwrapped object t (D21), and
// fills its only field on the first call.
func (b *convBuilder) unwrapObject(t *model.Type) (*convObject, error) {
	ref, err := b.ix.schemaRef(t.Schema)
	if err != nil {
		return nil, err
	}
	if obj, ok := b.bySchema[t.Schema]; ok {
		return obj, nil
	}
	obj := b.object(t.Schema, ref)
	obj.Unwrap = true
	b.objects = append(b.objects, obj)
	f := t.Fields[0]
	cf, err := b.field(ref.Path, f.Name, b.ov.fieldType(t.Schema, f))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", f.Name, err)
	}
	if cf.Value {
		// Then a missing object and a zero value read the same.
		return nil, fmt.Errorf("unwrap: the SDK field %s.%s is a value, not a pointer", obj.SDK, cf.SDK)
	}
	cf.Unwrapped = true
	if cf.Read, err = b.readRule(b.ov.field(t.Schema, f.Name), cf, f.Type); err != nil {
		return nil, fmt.Errorf("%s: %w", f.Name, err)
	}
	obj.Fields = []*convField{cf}
	if obj.ValueType, err = modelGoType(cf); err != nil {
		return nil, fmt.Errorf("%s: %w", f.Name, err)
	}
	return obj, nil
}

// modelGoType returns the Go type of the model field of cf. It is the type
// that tfBuilder.modelField writes.
func modelGoType(cf *convField) (string, error) {
	switch cf.Conv {
	case convString, convTime, convEnum, convEnumName, convInt64Text:
		return "types.String", nil
	case convBool:
		return "types.Bool", nil
	case convFloat64, convFloat32Wide:
		return "types.Float64", nil
	case convFloat32:
		return "types.Float32", nil
	case convInt32:
		return "types.Int32", nil
	case convInt64, convInt32Wide, convUint64, convInt64String:
		return "types.Int64", nil
	case convStrings, convScalars, convObjects, convEnumNames, convUnwraps:
		return "types." + cf.Collection, nil
	case convStringMap, convScalarMap, convUint64Map, convObjectMap:
		return "types.Map", nil
	case convObj, convEmpty:
		return "*" + cf.Object.Model, nil
	case convUnwrap:
		return cf.Object.ValueType, nil
	}
	return "", fmt.Errorf("unwrap: a value of kind %s is not supported", cf.Conv)
}

// wrap moves the fields of each wrapper of t (D21) from obj into a wrapper
// object, and puts a wrap field at the place of the first one.
func (b *convBuilder) wrap(t *model.Type, obj *convObject) {
	byName := map[string]*convField{}
	for _, cf := range obj.Fields {
		byName[cf.TFName] = cf
	}
	first := map[*convField]*convField{} // first wrapped field → the wrap field
	wrapped := map[*convField]bool{}
	for _, w := range b.ov.wrappers(t) {
		wobj := &convObject{Func: obj.Func + camelize(w.Name), Model: wrapperModelName(t.Schema, w.Name), SDK: obj.SDK, Wrapper: true}
		wobj.AttrTypesFunc = lowerFirst(wobj.Func) + "AttrTypes"
		var checks []string
		for _, name := range w.Fields {
			cf := byName[b.ov.tfName(t.Schema, name)]
			wobj.Fields = append(wobj.Fields, cf)
			wrapped[cf] = true
			if !cf.Value {
				checks = append(checks, "v."+cf.SDK+" == nil")
			}
		}
		if len(checks) == len(wobj.Fields) {
			wobj.NilCheck = strings.Join(checks, " && ")
		}
		b.objects = append(b.objects, wobj)
		first[byName[b.ov.tfName(t.Schema, w.Fields[0])]] = &convField{TFName: w.Name, Model: camelize(w.Name), Conv: convWrap, Object: wobj}
	}
	if len(first) == 0 {
		return
	}
	var fields []*convField
	for _, cf := range obj.Fields {
		if wf, ok := first[cf]; ok {
			fields = append(fields, wf)
		}
		if !wrapped[cf] {
			fields = append(fields, cf)
		}
	}
	obj.Fields = fields
}

// zeroable are the conversions whose zero value is a valid Terraform value.
var zeroable = map[string]bool{convString: true, convBool: true, convFloat64: true, convFloat32: true,
	convInt32: true, convInt64: true, convInt32Wide: true, convFloat32Wide: true}

// readRule returns how flatten reads a missing or empty value of cf, whose
// API type is t.
func (b *convBuilder) readRule(ov fieldOverride, cf *convField, t *model.Type) (string, error) {
	slice := map[string]bool{convStrings: true, convScalars: true, convObjects: true, convEnumNames: true}[cf.Conv]
	mapped := map[string]bool{convStringMap: true, convScalarMap: true, convUint64Map: true, convObjectMap: true}[cf.Conv]
	switch {
	case ov.MissingAsZero && (cf.Conv == convEnum || cf.Conv == convEnumName) && !cf.Value:
		// Proto3 JSON leaves out an enum at its zero value, the first one.
		enum, err := b.ix.schemaRef(t.Schema)
		if err != nil {
			return "", err
		}
		zero := t.Zero
		if zero == "" {
			zero = t.Values[0]
		}
		cf.ProtoZero = b.qualify(enumConstName(enum.Name, zero))
		return readZeroEnum, nil
	case ov.MissingAsZero:
		return zeroRule(cf, slice, mapped)
	case ov.EmptyAsNull && (slice || mapped):
		return readNullList, nil
	case ov.EmptyAsNull && cf.Conv == convObj && !cf.Value:
		return readNullObj, nil
	case ov.EmptyAsNull:
		return "", fmt.Errorf("emptyAsNull is not supported for %s", cf.Conv)
	}
	return "", nil
}

// zeroRule is the read rule of missingAsZero for cf.
func zeroRule(cf *convField, slice, mapped bool) (string, error) {
	switch {
	case slice:
		return readEmptyList, nil
	case mapped:
		return readEmptyMap, nil
	case cf.Value:
		return "", nil // a value is never missing
	case zeroable[cf.Conv]:
		return readZero, nil
	}
	// "", a zero time, or an empty uint64 string is not a valid value.
	return "", fmt.Errorf("missingAsZero is not supported for %s", cf.Conv)
}

// enumNames sets the name maps and the zero value of an enum with
// Terraform names (D21). The type mode writes the maps in enums.go.
func (b *convBuilder) enumNames(cf *convField, t *model.Type, enum sdkRef) {
	cf.Enum, cf.Zero = camelize(t.Schema), `""`
	if t.Zero != "" {
		cf.Zero = b.qualify(enumConstName(enum.Name, t.Zero))
	}
}

// wideConv sets the wide conversion of a narrow number (D21): Terraform
// Int64 or Float64, SDK int32 or float32. It returns the SDK Go type.
func wideConv(cf *convField, t *model.Type) (string, error) {
	switch cf.Conv {
	case convInt32:
		cf.Conv = convInt32Wide
		return "*int32", nil
	case convFloat32:
		cf.Conv = convFloat32Wide
		return "*float32", nil
	}
	return "", fmt.Errorf("wideNumbers: %s is not supported", typeName(t))
}

// mark sets the direction that uses obj and its nested objects: expand
// (a request) or flatten (a response).
func (b *convBuilder) mark(obj *convObject, expand bool) error {
	if (expand && obj.Expand) || (!expand && obj.Flatten) {
		return nil
	}
	if expand {
		obj.Expand = true
	} else {
		obj.Flatten = true
	}
	for _, f := range obj.Fields {
		switch f.Conv {
		case convObj, convObjects, convObjectMap, convWrap, convObjValue, convUnwrap, convUnwraps, convInline:
			if err := b.mark(f.Object, expand); err != nil {
				return fmt.Errorf("%s: %w", f.TFName, err)
			}
		}
	}
	return nil
}

// attrTypes fills the Terraform attribute types of obj, and of the objects
// inside it. The fields of an inlined object are attributes of obj.
func (b *convBuilder) attrTypes(obj *convObject) error {
	if obj.AttrTypes != nil {
		return nil
	}
	obj.AttrTypes = []convAttrType{}
	for _, f := range withInlined(obj) {
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
	convString: "types.StringType", convTime: "types.StringType", convEnum: "types.StringType",
	convBool: "types.BoolType", convFloat64: "types.Float64Type", convFloat32: "types.Float32Type",
	convUint64: "types.Int64Type", convInt64: "types.Int64Type", convInt32: "types.Int32Type",
	convInt32Wide: "types.Int64Type", convFloat32Wide: "types.Float64Type", convEnumName: "types.StringType",
	convEmpty:       "types.ObjectType{AttrTypes: map[string]attr.Type{}}",
	convInt64String: "types.Int64Type", convInt64Text: "types.StringType",
}

// attrType returns the Terraform attribute type expression of field f.
func (b *convBuilder) attrType(obj *convObject, f *convField) (string, error) {
	if expr, ok := scalarAttrTypes[f.Conv]; ok {
		return expr, nil
	}
	switch f.Conv {
	case convStrings, convEnumNames:
		return "types." + f.Collection + "Type{ElemType: types.StringType}", nil
	case convScalars:
		return "types." + f.Collection + "Type{ElemType: " + f.ElemType + "}", nil
	case convScalarMap:
		return "types.MapType{ElemType: " + f.ElemType + "}", nil
	case convStringMap:
		return "types.MapType{ElemType: types.StringType}", nil
	case convUint64Map:
		return "types.MapType{ElemType: types.Int64Type}", nil
	case convUnwrap:
		return b.attrType(f.Object, f.Object.Fields[0])
	case convUnwraps:
		// flatten also needs the element type.
		elem, err := b.attrType(f.Object, f.Object.Fields[0])
		if err != nil {
			return "", err
		}
		f.ElemType = elem
		return "types." + f.Collection + "Type{ElemType: " + elem + "}", nil
	case convObj, convObjects, convObjectMap, convWrap, convObjValue:
		if err := b.attrTypes(f.Object); err != nil {
			return "", err
		}
		expr := "types.ObjectType{AttrTypes: " + f.Object.AttrTypesFunc + "()}"
		switch f.Conv {
		case convObjects:
			expr = "types." + f.Collection + "Type{ElemType: " + expr + "}"
		case convObjectMap:
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
	if err := b.checkMapOverrides(t.Elem); err != nil {
		return "", err
	}
	if goType, elem, ok := scalarElem(t.Elem); ok {
		cf.Conv, cf.SDKType, cf.ElemType = convScalarMap, goType, elem
		return "map[string]" + goType, nil
	}
	switch e := t.Elem; {
	case e.Kind == model.String && e.Format == "":
		cf.Conv, cf.SDKType = convStringMap, "string"
		return "map[string]string", nil
	case e.Kind == model.Enum:
		enum, err := b.ix.schemaRef(e.Schema)
		if err != nil {
			return "", err
		}
		cf.Conv, cf.SDKType = convStringMap, b.qualify(enum.Name)
		return "map[string]" + enum.Name, nil
	case e.Kind == model.Integer && e.WireString && e.Format == "uint64":
		cf.Conv = convUint64Map
		return "map[string]string", nil

	case (e.Kind == model.Object || e.Kind == model.OneOf) && len(e.Fields) != 0:
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

// checkMapOverrides fails for a map value that an override would convert:
// the map conversions do not support that yet.
func (b *convBuilder) checkMapOverrides(e *model.Type) error {
	if b.ov.wide() && isNarrow(e) || e.Kind == model.Enum && b.ov.named(e.Schema) {
		return fmt.Errorf("overrides: a map of %s with wideNumbers or terraformNames is not supported", typeName(e))
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
	pkg      sdkRef              // the SDK package of the resource
	pkgs     map[string]sdkRef   // package refs by path
	types    map[string]sdkRef   // type refs by path
	fields   map[string]sdkRef   // field refs by path
	methods  map[string][]sdkRef // method refs by path
	funcs    map[string]sdkRef   // func refs by path and name
	bySchema map[string]sdkRef   // component type refs by schema name
}

func indexRefs(refs []sdkRef) (*refIndex, error) {
	ix := &refIndex{pkgs: map[string]sdkRef{}, types: map[string]sdkRef{}, fields: map[string]sdkRef{},
		methods: map[string][]sdkRef{}, funcs: map[string]sdkRef{}, bySchema: map[string]sdkRef{}}
	for _, ref := range refs {
		switch ref.Kind {
		case kindPackage:
			if _, ok := ix.pkgs[ref.Path]; ok {
				return nil, fmt.Errorf("more than one SDK package for %s", ref.Path)
			}
			ix.pkgs[ref.Path] = ref
		case kindType:
			ix.types[ref.Path] = ref
			if ref.Schema != "" {
				ix.bySchema[ref.Schema] = ref
			}
		case kindField:
			ix.fields[ref.Path] = ref
		case kindMethod:
			ix.methods[ref.Path] = append(ix.methods[ref.Path], ref)
		case kindFunc:
			ix.funcs[ref.Path+"."+ref.Name] = ref
		}
	}
	pkg, ok := ix.pkgs["resource"]
	if !ok {
		return nil, fmt.Errorf("no SDK package")
	}
	ix.pkg = pkg
	return ix, nil
}

func (ix *refIndex) pkgRef(path string) (sdkRef, error) {
	ref, ok := ix.pkgs[path]
	if !ok {
		return sdkRef{}, fmt.Errorf("no SDK package for %s", path)
	}
	return ref, nil
}

// methodRef returns the method of owner at path.
func (ix *refIndex) methodRef(path, owner string) (sdkRef, error) {
	for _, ref := range ix.methods[path] {
		if ref.Owner == owner {
			return ref, nil
		}
	}
	return sdkRef{}, fmt.Errorf("no SDK method of %s for %s", owner, path)
}

// funcRef returns the func name at path.
func (ix *refIndex) funcRef(path, name string) (sdkRef, error) {
	ref, ok := ix.funcs[path+"."+name]
	if !ok {
		return sdkRef{}, fmt.Errorf("no SDK func %s for %s", name, path)
	}
	return ref, nil
}

func (ix *refIndex) typeRef(path string) (sdkRef, error) {
	ref, ok := ix.types[path]
	if !ok {
		return sdkRef{}, fmt.Errorf("no SDK type for %s", path)
	}
	return ref, nil
}

func (ix *refIndex) fieldRef(path string) (sdkRef, error) {
	ref, ok := ix.fields[path]
	if !ok {
		return sdkRef{}, fmt.Errorf("no SDK field for %s", path)
	}
	return ref, nil
}

func (ix *refIndex) schemaRef(schema string) (sdkRef, error) {
	ref, ok := ix.bySchema[schema]
	if !ok {
		return sdkRef{}, fmt.Errorf("no SDK type for component %s", schema)
	}
	return ref, nil
}
