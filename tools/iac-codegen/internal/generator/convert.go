package generator

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/model"
	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/overrides"
)

// convData is the template data for expand (Terraform model → SDK request)
// and flatten (SDK response → Terraform model). Every SDK name comes from
// the checked sdkRefs.
type convData struct {
	// IDGuard is the model field of the resource id when flatten must check that the response
	// has it, or "".
	IDGuard string
	SDKPkg  string // import path of the SDK package
	SDKName string // package name
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
	// Existing: the resource has users. An unknown planned value of a computed attribute is
	// left out of the request, and the server supplies it, as the released resource did.
	Existing bool
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
	// Enum is true for a collection or map whose string values are enum
	// values. Response conversion then rejects protobuf zero sentinels.
	Enum     bool
	EnumZero string   // exact protobuf zero sentinel for enum conversion
	EnumMap  *enumMap // set when the overrides state the Terraform values of the enum
	// ReadEmptyAsNull: flatten reads an empty list, or an object with no value, as null.
	ReadEmptyAsNull bool
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
	return false
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
func buildConvWith(r *model.Resource, refs []sdkRef, file *overrides.File) (*convData, error) {
	ix, err := indexRefs(refs)
	if err != nil {
		return nil, err
	}
	b := &convBuilder{ix: ix, bySchema: map[string]*convObject{}, file: file, resource: r}
	out := &convData{SDKPkg: ix.pkg.Pkg, SDKName: ix.pkg.Name, Existing: r.Policy.Existing}
	if r.Policy.ClientSetID && !r.Singleton {
		// The contract may not require the id in the response (a proto3 optional field), so
		// flatten checks it. Without the id, Read, Update, and Delete would have no identity.
		out.IDGuard = camelize(r.IDParam)
	}

	roots := []convRoot{
		{&out.Create, "create.body", true, func(f *model.ResourceField) bool { return f.Create != nil }},
		{&out.Update, "update.body", true, func(f *model.ResourceField) bool { return f.Update != nil }},
		{&out.Resource, "fields", false, func(f *model.ResourceField) bool { return f.InGet }},
	}
	for _, root := range roots {
		if err := b.buildRoot(out, root); err != nil {
			return nil, err
		}
	}
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
	out.OneOfArms = oneOfArmPaths(r)
	out.EnumMaps = b.enumMaps
	if err := markPrior(out); err != nil {
		return nil, err
	}
	for _, f := range r.Fields {
		if f.Create != nil {
			out.CreateFields = append(out.CreateFields, &maskField{TFName: tfName(f.Name), ServerDefault: serverDefault(r, f)})
		}
	}
	if r.Replace {
		return out, replaceFields(r, out)
	}
	if err := buildMask(r, out); err != nil {
		return nil, err
	}
	return out, nil
}

// oneOfArmPaths returns the Terraform paths of every oneOf arm of r, in model
// order. A list, set, or map step adds nothing to the path.
func oneOfArmPaths(r *model.Resource) []string {
	var out []string
	for _, g := range r.Groups {
		for _, arm := range g.Arms {
			out = append(out, tfName(arm))
		}
	}
	for _, f := range r.Fields {
		out = typeArmPaths(out, tfName(f.Name), f.Type, map[*model.Type]bool{})
	}
	return out
}

func typeArmPaths(out []string, at string, t *model.Type, seen map[*model.Type]bool) []string {
	if t == nil || seen[t] {
		return out
	}
	seen[t] = true
	defer delete(seen, t)
	if t.Elem != nil {
		return typeArmPaths(out, at, t.Elem, seen)
	}
	arms := map[string]bool{}
	for _, g := range t.Groups {
		for _, arm := range g.Arms {
			arms[arm] = true
		}
	}
	for _, f := range t.Fields {
		child := at + "." + tfName(f.Name)
		if t.Kind == model.OneOf && f.Name != t.Discriminator || arms[f.Name] {
			out = append(out, child)
		}
		out = typeArmPaths(out, child, f.Type, seen)
	}
	return out
}

// convRoot is a root object of the conversion: the Create body, the Update body, or the resource.
type convRoot struct {
	obj    **convObject
	path   string // SDK name path of the struct
	expand bool
	has    func(*model.ResourceField) bool
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
	for _, f := range b.resource.Fields {
		if !root.has(f) {
			continue
		}
		cf, err := b.field(fieldsPath, b.resource.Name, f.Name, rootType(root.path, f))
		if err != nil {
			return fmt.Errorf("%s.%s: %w", root.path, f.Name, err)
		}
		b.markComputedObjectValue(cf, f, root.path == "fields")
		if err := checkReadEmptyAs(cf); err != nil {
			return fmt.Errorf("%s.%s: %w", root.path, f.Name, err)
		}
		obj.Fields = append(obj.Fields, cf)
	}
	if err := b.mark(obj, root.expand); err != nil {
		return fmt.Errorf("%s: %w", root.path, err)
	}
	*root.obj = obj
	return nil
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
func (b *convBuilder) rootObject(out *convData, root convRoot, ref sdkRef) (*convObject, string, error) {
	r := b.resource
	switch {
	case root.path == "fields":
		return b.object(r.Name, ref), root.path, nil
	case r.Policy.RequestWrapper != "" && root.expand:
		body, wrap, err := b.bodyWrapper(root.path, r.Policy.RequestWrapper)
		if err != nil {
			return nil, "", err
		}
		resourceRef, err := b.ix.typeRef("fields")
		if err != nil {
			return nil, "", err
		}
		verb := strings.ToUpper(root.path[:1]) + root.path[1:strings.Index(root.path, ".")]
		obj := &convObject{Func: camelize(r.Name) + verb, Model: modelTypeName(r.Name), SDK: resourceRef.Name}
		if root.path == "create.body" {
			out.CreateBody, out.CreateWrap = body, wrap
		} else {
			out.UpdateBody, out.UpdateWrap = body, wrap
		}
		return obj, "fields", nil
	}
	if root.path == "create.body" {
		out.CreateBody = ref.Name
	} else {
		out.UpdateBody = ref.Name
	}
	return &convObject{Func: camelize(ref.Name), Model: modelTypeName(r.Name), SDK: ref.Name}, root.path, nil
}

// checkReadEmptyAs rejects readEmptyAs where the renderer has no such normalization. The flag
// works for an object that is a pointer, and for a list or set of objects. Elsewhere it would
// do nothing, and the released normalization that the line states would be missing.
func checkReadEmptyAs(cf *convField) error {
	if !cf.ReadEmptyAsNull {
		return nil
	}
	if (cf.Conv == convObj && !cf.ObjectValue) || cf.Conv == convObjects {
		return nil
	}
	return fmt.Errorf("readEmptyAs: \"null\" is supported for an object and for a list or set of objects, not for the %s field %s", cf.Conv, cf.TFName)
}

// markComputedObjectValue makes a directly nested computed field capable of
// holding the unknown value that Terraform plans before Create.
func (b *convBuilder) markComputedObjectValue(cf *convField, f *model.ResourceField, response bool) {
	if !response || f.Behavior != model.Computed {
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

// replaceFields sets the Update fields of a full replace. The body has all of
// them, and the server clears a field that the body does not have.
func replaceFields(r *model.Resource, out *convData) error {
	out.Replace = true
	for _, f := range r.Fields {
		if f.Update != nil {
			out.UpdateFields = append(out.UpdateFields, &maskField{TFName: tfName(f.Name), ServerDefault: serverDefault(r, f)})
		}
	}
	if len(out.UpdateFields) == 0 {
		return fmt.Errorf("update.body: no Update fields")
	}
	return nil
}

// buildMask sets the update mask data. Only the Update fields can be in the
// mask, so application, subsystem, and target never are (D9).
func buildMask(r *model.Resource, out *convData) error {
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
			mf = maskTree(f.Name, f.Type.UpdateType())
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
	return model.UpdateMaskRule(pattern)
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
	file     *overrides.File
	resource *model.Resource
	enumMaps []*enumMap
}

// serverDefault reports whether removing the field resets it to a declared server default.
// A default line in the behavior-overrides file replaces the declared default.
func serverDefault(r *model.Resource, f *model.ResourceField) bool {
	return !r.Policy.OverridesDefault(r.Name, f.Name) && hasServerDefault(f)
}

// bodyWrapper returns the SDK request body type and the field that holds the resource.
func (b *convBuilder) bodyWrapper(bodyPath, wrapper string) (string, *bodyWrap, error) {
	body, err := b.ix.typeRef(bodyPath)
	if err != nil {
		return "", nil, err
	}
	field, err := b.ix.fieldRef(bodyPath + "." + wrapper)
	if err != nil {
		return "", nil, err
	}
	resource, err := b.ix.typeRef("fields")
	if err != nil {
		return "", nil, err
	}
	switch field.Want {
	case "*" + resource.Name:
		return body.Name, &bodyWrap{Field: field.Name}, nil
	case resource.Name:
		return body.Name, &bodyWrap{Field: field.Name, Value: true}, nil
	}
	return "", nil, fmt.Errorf("SDK field %s has type %s, want *%s", field.sdkName(), field.Want, resource.Name)
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
	if over.Zero != "" {
		m.Values = append(m.Values, enumMapValue{TF: over.Zero, API: t.EnumZero})
	}
	for _, v := range over.Values {
		m.Values = append(m.Values, enumMapValue{TF: strings.ToLower(v), API: v})
	}
	b.enumMaps = append(b.enumMaps, m)
	return m, nil
}

// object adds the convObject of a component schema, without fields.
func (b *convBuilder) object(schema string, ref sdkRef) *convObject {
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

// field returns the conversion of the property name of the SDK struct at
// owner (an SDK name path).
func (b *convBuilder) field(owner, component, name string, t *model.Type) (*convField, error) {
	ref, err := b.ix.fieldRef(owner + "." + name)
	if err != nil {
		return nil, err
	}
	cf := &convField{TFName: tfName(name), Model: camelize(name), SDK: ref.Name}
	if b.file != nil {
		line := b.file.Types[component].Fields[name]
		cf.ReadEmptyAsNull, cf.KeepPriorOrder = line.ReadEmptyAs == "null", line.KeepPriorOrder
		cf.Equality = line.Equality
	}
	want, err := b.fieldConv(cf, t)
	if err != nil {
		return nil, err
	}
	if err := checkReadEmptyAs(cf); err != nil {
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
		cf.Conv, cf.SDKType, cf.EnumZero = convEnum, b.qualify(enum.Name), t.EnumZero
		m, err := b.enumMapFor(t.Schema, b.qualify(enum.Name), t)
		if err != nil {
			return "", err
		}
		cf.EnumMap = m
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
		if err := b.rejectEnumLine(t.Elem.Schema); err != nil {
			return "", err
		}
		enum, err := b.ix.schemaRef(t.Elem.Schema)
		if err != nil {
			return "", err
		}
		cf.Conv, cf.SDKType, cf.Enum, cf.EnumZero = convStrings, b.qualify(enum.Name), true, t.Elem.EnumZero
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
	if obj, ok := b.bySchema[key]; ok {
		if obj.Model != modelTypeName(component) {
			return nil, fmt.Errorf("request component %s holds both %s and %s", t.Schema, obj.Model, modelTypeName(component))
		}
		return obj, nil
	}
	obj := b.object(key, ref)
	obj.Func, obj.Model = camelize(t.Schema), modelTypeName(component)
	obj.AttrTypesFunc = lowerFirst(obj.Func) + "AttrTypes"
	b.objects = append(b.objects, obj)
	for _, f := range t.Fields {
		cf, err := b.field(ref.Path, component, f.Name, f.Type)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", f.Name, err)
		}
		if f.Behavior == model.Computed && (f.Type.Kind == model.Object || f.Type.Kind == model.OneOf) {
			// The model holds a computed object as a types.Object, which can
			// be unknown while its parent is known.
			cf.ObjectValue = true
			if cf.Conv == convObj && !contains(b.listed, cf.Object) {
				b.listed = append(b.listed, cf.Object)
			}
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
		return fmt.Errorf("the enums line of %s applies to a single enum field, not to a list, set, or map of it", schema)
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

// markPrior sets NeedsPrior on every object with a KeepPriorOrder field, and on every object
// that holds such an object. Only these flatten functions take the prior model. It also
// marks the objects that a list keeping the prior order holds, and their nested objects: the
// package compares their SDK values.
func markPrior(d *convData) error {
	if err := markPriorFields(d); err != nil {
		return err
	}
	propagatePrior(d)
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

// markPriorFields marks the objects with a KeepPriorOrder or an Equality field, and the objects that
// a list keeping the prior order holds.
func markPriorFields(d *convData) error {
	for _, obj := range d.Objects {
		for _, f := range obj.Fields {
			if f.Equality != "" {
				if f.Conv != convString {
					return fmt.Errorf("%s.%s: equality needs a string field", obj.Model, f.TFName)
				}
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

// errPriorContainer reports a field that needs the prior model in a container whose flatten has no
// prior value to pass: a map of objects, or a computed object stored as types.Object. A set of
// objects has no order, so the prior item at the same index is not the same item.
var errPriorContainer = errors.New("equality and keepPriorOrder are not supported in a map or set of objects or in a computed object")

func checkPriorContainers(d *convData) error {
	for _, obj := range d.Objects {
		for _, f := range obj.Fields {
			if f.Object != nil && f.Object.NeedsPrior && (f.Conv == convObjectMap || f.ObjectValue || f.Conv == convObjects && f.Collection == "Set") {
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
			for _, f := range obj.Fields {
				if (f.Conv != convObj && f.Conv != convObjects) || f.Object == nil {
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
	a, b := "a."+f.SDK, "b."+f.SDK
	switch f.Conv {
	case convString, convBool, convFloat64, convFloat32, convInt32, convInt64, convUint64, convEnum, convTime:
		if !f.Value {
			a, b = "pointerValue("+a+")", "pointerValue("+b+")"
		}
		if f.Equality != "" {
			// The API can return the document in another format, so the items compare as documents.
			return f.Equality + "Equal(" + a + ", " + b + ")", nil
		}
		return a + " == " + b, nil
	case convStringMap, convScalarMap, convUint64Map:
		return "maps.Equal(" + a + ", " + b + ")", nil
	case convStrings, convScalars:
		if f.Collection == "Set" {
			break
		}
		return "slices.Equal(" + a + ", " + b + ")", nil
	case convObj:
		if f.Value {
			return "same" + f.Object.Func + "(&" + a + ", &" + b + ")", nil
		}
		return "sameObject(" + a + ", " + b + ", same" + f.Object.Func + ")", nil
	case convObjects:
		return "sameList(" + a + ", " + b + ", same" + f.Object.Func + ")", nil
	}
	return "", fmt.Errorf("a field of kind %s cannot be compared for keepPriorOrder yet", f.Conv)
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
	for _, obj := range d.Objects {
		for _, f := range obj.Fields {
			if f.Equality != "" && (len(kind) == 0 || slices.Contains(kind, f.Equality)) {
				return true
			}
		}
	}
	return false
}

// HasPrior reports whether flatten takes the prior model of the resource.
func (d *convData) HasPrior() bool { return d.Resource != nil && d.Resource.NeedsPrior }
