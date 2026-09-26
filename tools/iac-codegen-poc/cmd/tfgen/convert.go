package main

import (
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
	convTime    = "time"    // types.String ← *time.Time (flatten only)
	convEnum    = "enum"    // types.String ↔ *<enum type> (D12)
	convObj     = "object"  // *<Model> ↔ *<SDK type>
	convEmpty   = "empty"   // *<Model> ↔ map[string]interface{} (F16)
	convStrings = "strings" // types.Set/List ↔ []string or []<enum type>
	convObjects = "objects" // types.List ↔ []<SDK type>

	convInt32   = "int32"   // types.Int32 ↔ *int32
	convInt64   = "int64"   // types.Int64 ↔ *int64 (signed, a JSON number)
	convFloat32 = "float32" // types.Float32 ↔ *float32
	convScalars = "scalars" // types.List/Set ↔ []bool, []int32, []int64, []float32, []float64

	convStringMap = "stringmap" // types.Map ↔ map[string]string or map[string]<enum type>
	convScalarMap = "scalarmap" // types.Map ↔ map[string]bool, int32, int64, float32, float64
	convUint64Map = "uint64map" // types.Map of Int64 ↔ map[string]string (D7)
	convObjectMap = "objectmap" // types.Map ↔ map[string]<SDK type>
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
		return "*" + enum.Name, nil
	case model.Object, model.OneOf:
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
	case t.WireString && t.Format == "uint64":
		cf.Conv = convUint64
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
		return "[]" + enum.Name, nil
	case model.Bool, model.Number, model.Integer:
		goType, elem, ok := scalarElem(t.Elem)
		if !ok {
			break
		}
		cf.Conv, cf.SDKType, cf.ElemType = convScalars, goType, elem
		return "[]" + goType, nil
	case model.Object:
		// A set of objects needs path.AtSetValue for diagnostics. No
		// resource uses it yet.
		if t.Kind == model.Set || len(t.Elem.Fields) == 0 {
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
	for _, f := range t.Fields {
		cf, err := b.field(ref.Path, f.Name, f.Type)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", f.Name, err)
		}
		obj.Fields = append(obj.Fields, cf)
	}
	return obj, nil
}

// mark sets the direction that uses obj and its nested objects: expand
// (a request) or flatten (a response). A request cannot hold a date-time.
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
		switch {
		case f.Conv == convTime && expand:
			return fmt.Errorf("%s: date-time in a request is not supported", f.TFName)
		case f.Conv == convObj || f.Conv == convObjects || f.Conv == convObjectMap:
			if err := b.mark(f.Object, expand); err != nil {
				return fmt.Errorf("%s: %w", f.TFName, err)
			}
		}
	}
	return nil
}

// attrTypes fills the Terraform attribute types of obj, and of the objects
// inside it.
func (b *convBuilder) attrTypes(obj *convObject) error {
	if obj.AttrTypes != nil {
		return nil
	}
	obj.AttrTypes = []convAttrType{}
	for _, f := range obj.Fields {
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
	convEmpty: "types.ObjectType{AttrTypes: map[string]attr.Type{}}",
}

// attrType returns the Terraform attribute type expression of field f.
func (b *convBuilder) attrType(obj *convObject, f *convField) (string, error) {
	if expr, ok := scalarAttrTypes[f.Conv]; ok {
		return expr, nil
	}
	switch f.Conv {
	case convStrings:
		return "types." + f.Collection + "Type{ElemType: types.StringType}", nil
	case convScalars:
		return "types." + f.Collection + "Type{ElemType: " + f.ElemType + "}", nil
	case convScalarMap:
		return "types.MapType{ElemType: " + f.ElemType + "}", nil
	case convStringMap:
		return "types.MapType{ElemType: types.StringType}", nil
	case convUint64Map:
		return "types.MapType{ElemType: types.Int64Type}", nil
	case convObj, convObjects, convObjectMap:
		if err := b.attrTypes(f.Object); err != nil {
			return "", err
		}
		expr := "types.ObjectType{AttrTypes: " + f.Object.AttrTypesFunc + "()}"
		switch f.Conv {
		case convObjects:
			expr = "types.ListType{ElemType: " + expr + "}"
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
