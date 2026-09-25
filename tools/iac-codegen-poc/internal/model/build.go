package model

import (
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"

	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi/datamodel"
	"github.com/pb33f/libopenapi/datamodel/high/base"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
)

const (
	componentPrefix = "#/components/schemas/"
	jsonMedia       = "application/json"
	updateMaskField = "updateMask" // contract: Update is PATCH with an updateMask
	extPresence     = "x-coralogix-presence"
	extCollection   = "x-coralogix-collection"
)

// Load parses an OpenAPI 3 document.
func Load(data []byte) (*v3.Document, error) {
	// Some specs have a $ref loop through an array (README.md, finding F11). An empty
	// array ends the loop, so it is valid. By default libopenapi rejects it.
	config := datamodel.NewDocumentConfiguration()
	config.IgnoreArrayCircularReferences = true
	doc, err := libopenapi.NewDocumentWithConfiguration(data, config)
	if err != nil {
		return nil, fmt.Errorf("new document: %w", err)
	}
	built, err := doc.BuildV3Model()
	if err != nil {
		return nil, fmt.Errorf("build v3 model: %w", err)
	}
	return &built.Model, nil
}

// Build reads the resource whose schema is the component name. It finds the
// operations by operationId suffix: "_Create<name>", "_Get<name>",
// "_Update<name>", and "_Delete<name>".
func Build(doc *v3.Document, name string) (*Resource, error) {
	ops, err := findOperations(doc, name)
	if err != nil {
		return nil, err
	}
	r := &Resource{Name: name, UpdateMask: updateMaskField}
	if err := r.readOperations(ops); err != nil {
		return nil, err
	}
	if err := r.readFields(doc, ops); err != nil {
		return nil, err
	}
	return r, nil
}

type verb string

const (
	opCreate verb = "Create"
	opGet    verb = "Get"
	opUpdate verb = "Update"
	opDelete verb = "Delete"
)

var verbs = []verb{opCreate, opGet, opUpdate, opDelete}

var verbMethod = map[verb]string{opCreate: "POST", opGet: "GET", opUpdate: "PATCH", opDelete: "DELETE"}

type foundOp struct {
	path   string
	method string
	item   *v3.PathItem
	op     *v3.Operation
}

func findOperations(doc *v3.Document, name string) (map[verb]foundOp, error) {
	found := map[verb]foundOp{}
	if doc.Paths == nil {
		return nil, errors.New("spec has no paths")
	}
	for path, item := range doc.Paths.PathItems.FromOldest() {
		for method, op := range item.GetOperations().FromOldest() {
			for _, v := range verbs {
				if !strings.HasSuffix(op.OperationId, "_"+string(v)+name) {
					continue
				}
				if prev, ok := found[v]; ok {
					return nil, fmt.Errorf("%s: two operations: %s and %s", v, prev.op.OperationId, op.OperationId)
				}
				found[v] = foundOp{path: path, method: strings.ToUpper(method), item: item, op: op}
			}
		}
	}
	for _, v := range verbs {
		f, ok := found[v]
		if !ok {
			return nil, fmt.Errorf("%s: no operation with operationId suffix _%s%s", v, v, name)
		}
		if f.method != verbMethod[v] {
			return nil, fmt.Errorf("%s: %s is %s, want %s", v, f.op.OperationId, f.method, verbMethod[v])
		}
	}
	return found, nil
}

func (r *Resource) readOperations(ops map[verb]foundOp) error {
	id, err := idParam(ops[opGet])
	if err != nil {
		return fmt.Errorf("get: %w", err)
	}
	r.IDParam = id
	for _, v := range []verb{opUpdate, opDelete} {
		other, err := idParam(ops[v])
		if err != nil {
			return fmt.Errorf("%s: %w", v, err)
		}
		if other != id {
			return fmt.Errorf("%s: id path parameter %q, want %q (as in get)", v, other, id)
		}
	}
	if params := pathParams(ops[opCreate]); len(params) != 0 {
		return fmt.Errorf("create: %d path parameters, want none", len(params))
	}
	itemPath := ops[opGet].path
	if want := ops[opCreate].path + "/{" + id + "}"; itemPath != want {
		return fmt.Errorf("get: path %s, want %s", itemPath, want)
	}
	for _, v := range []verb{opUpdate, opDelete} {
		if ops[v].path != itemPath {
			return fmt.Errorf("%s: path %s, want %s (as in get)", v, ops[v].path, itemPath)
		}
	}

	targets := map[verb]*Operation{opCreate: &r.Create, opGet: &r.Get, opUpdate: &r.Update, opDelete: &r.Delete}
	for _, v := range verbs {
		f := ops[v]
		o := targets[v]
		o.Method, o.Path, o.OperationID = f.method, f.path, f.op.OperationId
		if o.Body, err = bodyName(f.op); err != nil {
			return fmt.Errorf("%s: %w", v, err)
		}
		if o.Response, err = r.response(f.op, v != opDelete); err != nil {
			return fmt.Errorf("%s: %w", v, err)
		}
	}
	for _, v := range []verb{opCreate, opUpdate} {
		if targets[v].Body == "" {
			return fmt.Errorf("%s: no request body", v)
		}
	}
	return nil
}

func pathParams(f foundOp) []*v3.Parameter {
	var params []*v3.Parameter
	for _, p := range slices.Concat(f.item.Parameters, f.op.Parameters) {
		if p.In == "path" {
			params = append(params, p)
		}
	}
	return params
}

func idParam(f foundOp) (string, error) {
	params := pathParams(f)
	if len(params) != 1 {
		return "", fmt.Errorf("%d path parameters, want 1", len(params))
	}
	p := params[0]
	if p.Required == nil || !*p.Required {
		return "", fmt.Errorf("path parameter %q is not required", p.Name)
	}
	return p.Name, nil
}

func bodyProxy(op *v3.Operation) *base.SchemaProxy {
	if op.RequestBody == nil {
		return nil
	}
	media := op.RequestBody.Content.GetOrZero(jsonMedia)
	if media == nil {
		return nil
	}
	return media.Schema
}

func bodyName(op *v3.Operation) (string, error) {
	proxy := bodyProxy(op)
	switch {
	case proxy == nil && op.RequestBody != nil:
		return "", fmt.Errorf("request body has no %s schema", jsonMedia)
	case proxy == nil:
		return "", nil
	case proxy.IsReference():
		return componentName(proxy.GetReference())
	}
	return "inline", nil
}

// response reads the 200 response. When wrapped is true, the response must
// have exactly one property, and that property must be the resource.
func (r *Resource) response(op *v3.Operation, wrapped bool) (Response, error) {
	if op.Responses == nil || op.Responses.Codes == nil {
		return Response{}, errors.New("no responses")
	}
	resp := op.Responses.Codes.GetOrZero("200")
	if resp == nil || resp.Content.GetOrZero(jsonMedia) == nil || resp.Content.GetOrZero(jsonMedia).Schema == nil {
		return Response{}, fmt.Errorf("no 200 %s response schema", jsonMedia)
	}
	proxy := resp.Content.GetOrZero(jsonMedia).Schema
	if !proxy.IsReference() {
		return Response{}, errors.New("200 response schema is inline, want a $ref")
	}
	name, err := componentName(proxy.GetReference())
	if err != nil {
		return Response{}, err
	}
	out := Response{Schema: name}
	if !wrapped {
		return out, nil
	}
	s, err := schemaOf(proxy)
	if err != nil {
		return Response{}, err
	}
	keys := propertyNames(s)
	if len(keys) != 1 {
		return Response{}, fmt.Errorf("response %s has properties %v, want one that wraps %s", name, keys, r.Name)
	}
	inner, err := componentName(unwrapRef(s.Properties.GetOrZero(keys[0])))
	if err != nil {
		return Response{}, fmt.Errorf("response %s.%s: %w", name, keys[0], err)
	}
	if inner != r.Name {
		return Response{}, fmt.Errorf("response %s.%s is %s, want %s", name, keys[0], inner, r.Name)
	}
	out.Field = keys[0]
	return out, nil
}

// unwrapRef returns the $ref of a property that is "$ref: X" or "allOf: [$ref: X]".
func unwrapRef(proxy *base.SchemaProxy) string {
	if proxy.IsReference() {
		return proxy.GetReference()
	}
	if s := proxy.Schema(); s != nil && len(s.AllOf) == 1 {
		return s.AllOf[0].GetReference()
	}
	return ""
}

func componentName(ref string) (string, error) {
	name, ok := strings.CutPrefix(ref, componentPrefix)
	if !ok || name == "" {
		return "", fmt.Errorf("$ref %q is not a local component schema", ref)
	}
	return name, nil
}

func schemaOf(proxy *base.SchemaProxy) (*base.Schema, error) {
	s := proxy.Schema()
	if s == nil {
		if err := proxy.GetBuildError(); err != nil {
			return nil, err
		}
		return nil, errors.New("empty schema")
	}
	return s, nil
}

func propertyNames(s *base.Schema) []string {
	if s.Properties == nil {
		return nil
	}
	return slices.Collect(s.Properties.KeysFromOldest())
}

// location is one of the three schemas where a resource field can appear.
type location struct {
	name   string
	schema *base.Schema
}

func (r *Resource) readFields(doc *v3.Document, ops map[verb]foundOp) error {
	createBody, updateBody, getSchema, err := r.fieldSchemas(doc, ops)
	if err != nil {
		return err
	}
	if err := r.checkBodies(createBody, updateBody, getSchema); err != nil {
		return err
	}
	var names []string
	for _, s := range []*base.Schema{getSchema, createBody, updateBody} {
		for _, n := range propertyNames(s) {
			if n != updateMaskField && !slices.Contains(names, n) {
				names = append(names, n)
			}
		}
	}
	for _, n := range names {
		f, err := resourceField(n, createBody, updateBody, getSchema)
		if err != nil {
			return fmt.Errorf("%s: %w", r.Name, err)
		}
		r.Fields = append(r.Fields, f)
	}
	return nil
}

// fieldSchemas returns the three schemas where a resource field can appear:
// the Create body, the Update body, and the resource component (Get).
func (r *Resource) fieldSchemas(doc *v3.Document, ops map[verb]foundOp) (createBody, updateBody, getSchema *base.Schema, err error) {
	if createBody, err = schemaOf(bodyProxy(ops[opCreate].op)); err != nil {
		return nil, nil, nil, fmt.Errorf("create body: %w", err)
	}
	if updateBody, err = schemaOf(bodyProxy(ops[opUpdate].op)); err != nil {
		return nil, nil, nil, fmt.Errorf("update body: %w", err)
	}
	if doc.Components == nil || doc.Components.Schemas == nil {
		return nil, nil, nil, errors.New("spec has no component schemas")
	}
	component := doc.Components.Schemas.GetOrZero(r.Name)
	if component == nil {
		return nil, nil, nil, fmt.Errorf("component %s not found", r.Name)
	}
	if getSchema, err = schemaOf(component); err != nil {
		return nil, nil, nil, fmt.Errorf("%s: %w", r.Name, err)
	}
	return createBody, updateBody, getSchema, nil
}

// checkBodies checks the update mask, the id field, and the required lists.
func (r *Resource) checkBodies(createBody, updateBody, getSchema *base.Schema) error {
	mask := updateBody.Properties.GetOrZero(updateMaskField)
	if mask == nil {
		return fmt.Errorf("update body: no %s property", updateMaskField)
	}
	ms, err := schemaOf(mask)
	if err != nil || !slices.Equal(ms.Type, []string{"string"}) {
		return fmt.Errorf("update body: %s must be a string", updateMaskField)
	}
	r.UpdateMaskPattern = ms.Pattern
	for _, loc := range []location{{"create body", createBody}, {r.Name, getSchema}} {
		if loc.schema.Properties.GetOrZero(updateMaskField) != nil {
			return fmt.Errorf("%s: unexpected %s property", loc.name, updateMaskField)
		}
	}
	if getSchema.Properties.GetOrZero(r.IDParam) == nil {
		return fmt.Errorf("%s: no field %q for the id path parameter", r.Name, r.IDParam)
	}
	for _, loc := range []location{{"create body", createBody}, {"update body", updateBody}, {r.Name, getSchema}} {
		if err := checkRequired(loc.schema); err != nil {
			return fmt.Errorf("%s: %w", loc.name, err)
		}
	}
	return nil
}

func resourceField(name string, createBody, updateBody, getSchema *base.Schema) (*ResourceField, error) {
	cp := createBody.Properties.GetOrZero(name)
	up := updateBody.Properties.GetOrZero(name)
	gp := getSchema.Properties.GetOrZero(name)
	behavior, err := Classify(cp != nil, up != nil, gp != nil)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	f := &ResourceField{Name: name, Behavior: behavior, InGet: gp != nil}
	if f.Description, err = description(gp); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	// The Get type comes first: Classify ensures that the field is in Get.
	if f.Type, err = typeOf(gp, name, nil); err != nil {
		return nil, err
	}
	for _, loc := range []struct {
		name  string
		body  *base.Schema
		proxy *base.SchemaProxy
		attrs **Attrs
	}{
		{"create", createBody, cp, &f.Create},
		{"update", updateBody, up, &f.Update},
	} {
		if loc.proxy == nil {
			continue
		}
		t, err := typeOf(loc.proxy, name, nil)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", loc.name, err)
		}
		if !reflect.DeepEqual(t, f.Type) {
			return nil, fmt.Errorf("%s: %s type differs from the get type", name, loc.name)
		}
		a, err := attrsOf(loc.body, name, loc.proxy)
		if err != nil {
			return nil, fmt.Errorf("%s: %s: %w", name, loc.name, err)
		}
		*loc.attrs = &a
	}
	return f, nil
}

// description is the description of a property schema, also when it wraps a
// $ref in allOf.
func description(proxy *base.SchemaProxy) (string, error) {
	s, err := schemaOf(proxy)
	if err != nil {
		return "", err
	}
	return s.Description, nil
}

// checkRequired checks that each name in "required" is a property.
func checkRequired(s *base.Schema) error {
	for _, n := range s.Required {
		if s.Properties.GetOrZero(n) == nil {
			return fmt.Errorf("required %q is not a property", n)
		}
	}
	return nil
}

// attrsOf reads the attributes of property name of parent. The attributes are
// on the property schema, also when it wraps a $ref in allOf.
func attrsOf(parent *base.Schema, name string, proxy *base.SchemaProxy) (Attrs, error) {
	s, err := schemaOf(proxy)
	if err != nil {
		return Attrs{}, err
	}
	a := Attrs{Required: slices.Contains(parent.Required, name)}
	if n := s.Extensions.GetOrZero(extPresence); n != nil {
		switch n.Value {
		case "true":
			a.Presence = true
		case "false":
		default:
			return Attrs{}, fmt.Errorf("%s: %q, want true or false", extPresence, n.Value)
		}
	}
	if s.Default != nil {
		v := s.Default.Value
		a.Default = &v
	}
	return a, nil
}

// typeOf builds the type of a schema. path is the field path for errors.
// stack holds the $refs being built, to find loops.
func typeOf(proxy *base.SchemaProxy, path string, stack []string) (*Type, error) {
	stack, err := pushRef(proxy, path, stack)
	if err != nil {
		return nil, err
	}
	s, err := schemaOf(proxy)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if err := checkSupported(s); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if len(s.AllOf) > 0 {
		inner, err := singleAllOf(s, path)
		if err != nil {
			return nil, err
		}
		return typeOf(inner, path, stack)
	}
	var name string
	if ref := proxy.GetReference(); ref != "" {
		if name, err = componentName(ref); err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
	}
	if len(s.Type) != 1 {
		return nil, fmt.Errorf("%s: type %v, want exactly one type", path, s.Type)
	}
	t := &Type{Schema: name, Format: s.Format}
	if err := fillType(t, s, path, stack); err != nil {
		return nil, err
	}
	return t, nil
}

// pushRef adds the $ref of proxy to stack. A $ref that is already in stack is
// a loop.
func pushRef(proxy *base.SchemaProxy, path string, stack []string) ([]string, error) {
	ref := proxy.GetReference()
	if ref == "" {
		return stack, nil
	}
	if slices.Contains(stack, ref) {
		return nil, fmt.Errorf("%s: recursive schema %s is not supported", path, ref)
	}
	return append(stack, ref), nil
}

// singleAllOf returns the only allOf entry of s. The generator uses allOf
// only to wrap one $ref.
func singleAllOf(s *base.Schema, path string) (*base.SchemaProxy, error) {
	if len(s.AllOf) != 1 {
		return nil, fmt.Errorf("%s: allOf with %d entries, want 1", path, len(s.AllOf))
	}
	if len(s.Type) != 0 || s.Properties != nil || len(s.OneOf) != 0 {
		return nil, fmt.Errorf("%s: allOf with sibling type keywords", path)
	}
	return s.AllOf[0], nil
}

// fillType sets the kind of t, and the parts of t that depend on the kind.
// Every error names the path.
func fillType(t *Type, s *base.Schema, path string, stack []string) error {
	var err error
	switch s.Type[0] {
	case "object":
		// Errors from objectType and arrayType already name the path.
		return objectType(t, s, path, stack)
	case "array":
		return arrayType(t, s, path, stack)
	case "string":
		err = stringType(t, s)
	case "boolean":
		t.Kind = Bool
	case "number":
		t.Kind = Number
		t.Minimum, t.Maximum = s.Minimum, s.Maximum
	case "integer":
		t.Kind = Integer
		t.Minimum, t.Maximum = s.Minimum, s.Maximum
	default:
		err = fmt.Errorf("type %q is not supported", s.Type[0])
	}
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

// checkSupported rejects keywords that the model cannot express.
func checkSupported(s *base.Schema) error {
	switch {
	case len(s.AnyOf) != 0:
		return errors.New("anyOf is not supported")
	case s.Not != nil:
		return errors.New("not is not supported")
	case s.Const != nil:
		return errors.New("const is not supported")
	case s.Nullable != nil && *s.Nullable:
		return errors.New("nullable is not supported")
	case s.PatternProperties != nil && s.PatternProperties.Len() != 0:
		return errors.New("patternProperties is not supported")
	case len(s.PrefixItems) != 0:
		return errors.New("prefixItems is not supported")
	case s.Discriminator != nil:
		return errors.New("discriminator is not supported")
	}
	return nil
}

func objectType(t *Type, s *base.Schema, path string, stack []string) error {
	if ap := s.AdditionalProperties; ap != nil && (ap.IsA() || ap.B) {
		return mapType(t, s, path, stack)
	}
	if err := checkRequired(s); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	fields, err := objectFields(s, path, stack)
	if err != nil {
		return err
	}
	t.Fields = fields
	if len(s.OneOf) == 0 {
		t.Kind = Object
		return nil
	}
	t.Kind = OneOf
	for _, f := range fields {
		if f.Attrs != (Attrs{}) {
			return fmt.Errorf("%s: oneOf arm %s has attributes %+v, want none", path, f.Name, f.Attrs)
		}
	}
	allowNone, err := oneOfArms(s, propertyNames(s))
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	t.AllowNone = allowNone
	return nil
}

func objectFields(s *base.Schema, path string, stack []string) ([]*Field, error) {
	var fields []*Field
	for _, name := range propertyNames(s) {
		proxy := s.Properties.GetOrZero(name)
		ft, err := typeOf(proxy, path+"."+name, stack)
		if err != nil {
			return nil, err
		}
		a, err := attrsOf(s, name, proxy)
		if err != nil {
			return nil, fmt.Errorf("%s.%s: %w", path, name, err)
		}
		d, err := description(proxy)
		if err != nil {
			return nil, fmt.Errorf("%s.%s: %w", path, name, err)
		}
		fields = append(fields, &Field{Name: name, Description: d, Attrs: a, Type: ft})
	}
	return fields, nil
}

// oneOfArms checks that the oneOf has one {required: [arm]} entry for each
// property, and at most one "no arm" entry {not: {anyOf: [...]}} that lists
// every arm. It reports whether the "no arm" entry exists.
func oneOfArms(s *base.Schema, props []string) (bool, error) {
	var arms []string
	allowNone := false
	for i, proxy := range s.OneOf {
		entry, err := schemaOf(proxy)
		if err != nil {
			return false, fmt.Errorf("oneOf[%d]: %w", i, err)
		}
		if entry.Properties != nil || len(entry.AllOf) != 0 || len(entry.OneOf) != 0 || len(entry.AnyOf) != 0 {
			return false, fmt.Errorf("oneOf[%d]: want only required or not", i)
		}
		if entry.Not == nil {
			if len(entry.Required) != 1 {
				return false, fmt.Errorf("oneOf[%d]: required %v, want one arm", i, entry.Required)
			}
			arms = append(arms, entry.Required[0])
			continue
		}
		if allowNone {
			return false, fmt.Errorf("oneOf[%d]: second \"no arm\" entry", i)
		}
		none, err := noArmList(entry.Not)
		if err != nil {
			return false, fmt.Errorf("oneOf[%d]: %w", i, err)
		}
		if !sameSet(none, props) {
			return false, fmt.Errorf("oneOf[%d]: not.anyOf lists %v, want %v", i, none, props)
		}
		allowNone = true
	}
	if !sameSet(arms, props) {
		return false, fmt.Errorf("oneOf arms %v do not match properties %v", arms, props)
	}
	return allowNone, nil
}

func noArmList(not *base.SchemaProxy) ([]string, error) {
	s, err := schemaOf(not)
	if err != nil {
		return nil, err
	}
	var arms []string
	for _, proxy := range s.AnyOf {
		entry, err := schemaOf(proxy)
		if err != nil {
			return nil, err
		}
		if len(entry.Required) != 1 {
			return nil, fmt.Errorf("not.anyOf entry required %v, want one arm", entry.Required)
		}
		arms = append(arms, entry.Required[0])
	}
	return arms, nil
}

// sameSet reports whether a and b have the same names, each name once.
func sameSet(a, b []string) bool {
	a, b = slices.Sorted(slices.Values(a)), slices.Sorted(slices.Values(b))
	return len(slices.Compact(slices.Clone(a))) == len(a) && slices.Equal(a, b)
}

// mapType builds a map: an object with only additionalProperties. The keys
// are strings. A free-form map (additionalProperties: true) has no value type,
// and an object with both properties and additionalProperties has two shapes,
// so both are rejected.
func mapType(t *Type, s *base.Schema, path string, stack []string) error {
	ap := s.AdditionalProperties
	if !ap.IsA() || ap.A == nil {
		return fmt.Errorf("%s: a map without a value schema (additionalProperties: true) is not supported", path)
	}
	if s.Properties != nil && s.Properties.Len() != 0 || len(s.OneOf) != 0 {
		return fmt.Errorf("%s: an object with both properties and additionalProperties is not supported", path)
	}
	elem, err := typeOf(ap.A, path+"{}", stack)
	if err != nil {
		return err
	}
	t.Kind, t.Elem = Map, elem
	return nil
}

func arrayType(t *Type, s *base.Schema, path string, stack []string) error {
	if s.Items == nil || !s.Items.IsA() || s.Items.A == nil {
		return fmt.Errorf("%s: array without an items schema", path)
	}
	unique := s.UniqueItems != nil && *s.UniqueItems
	var set bool
	if n := s.Extensions.GetOrZero(extCollection); n != nil {
		if n.Value != "set" {
			return fmt.Errorf("%s: %s: %q, want set", path, extCollection, n.Value)
		}
		set = true
	}
	if unique != set {
		return fmt.Errorf("%s: uniqueItems %t and %s set %t must agree", path, unique, extCollection, set)
	}
	elem, err := typeOf(s.Items.A, path+"[]", stack)
	if err != nil {
		return err
	}
	t.Kind = List
	if set {
		t.Kind = Set
	}
	t.Elem = elem
	t.MinItems, t.MaxItems = s.MinItems, s.MaxItems
	return nil
}

func stringType(t *Type, s *base.Schema) error {
	t.MinLength, t.MaxLength = s.MinLength, s.MaxLength
	switch {
	case len(s.Enum) != 0:
		t.Kind = Enum
		t.MinLength, t.MaxLength = nil, nil
		for _, n := range s.Enum {
			// Contract: the enum zero value is *_UNSPECIFIED. It is not a valid value.
			if !strings.HasSuffix(n.Value, "_UNSPECIFIED") {
				t.Values = append(t.Values, n.Value)
			}
		}
		if len(t.Values) == 0 {
			return errors.New("enum has no values")
		}
	case s.Format == "int64" || s.Format == "uint64":
		t.Kind = Integer
		t.WireString = true
	default:
		t.Kind = String
	}
	return nil
}
