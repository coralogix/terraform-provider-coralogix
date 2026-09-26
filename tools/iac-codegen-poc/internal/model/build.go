package model

import (
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"unicode"

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
// "_Update<name>" or "_Replace<name>", and "_Delete<name>". An Update with
// PATCH has an update mask. An Update with PUT, or a Replace, is a full
// replace (E11).
func Build(doc *v3.Document, name string) (*Resource, error) {
	ops, err := findOperations(doc, name)
	if err != nil {
		return nil, err
	}
	r := &Resource{Name: name, Replace: ops[opUpdate].method == "PUT"}
	if !r.Replace {
		r.UpdateMask = updateMaskField
	}
	if err := r.readOperations(ops); err != nil {
		return nil, err
	}
	r.readBodyTitles(doc, ops)
	if err := r.readFields(doc, ops); err != nil {
		return nil, err
	}
	return r, nil
}

type verb string

const (
	opCreate  verb = "Create"
	opGet     verb = "Get"
	opUpdate  verb = "Update"
	opReplace verb = "Replace" // a full-replace Update; findOperations stores it as opUpdate
	opDelete  verb = "Delete"
)

var verbs = []verb{opCreate, opGet, opUpdate, opDelete}

var verbMethods = map[verb][]string{
	opCreate: {"POST"}, opGet: {"GET"}, opUpdate: {"PATCH", "PUT"}, opReplace: {"PUT"}, opDelete: {"DELETE"},
}

type foundOp struct {
	path   string
	method string
	item   *v3.PathItem
	op     *v3.Operation
}

// Survey builds the type of the component schema name, as Build does for
// the resource fields, but it collects every error and goes on. Each error
// starts with its path. It is for measuring which shapes a spec uses that
// the model does not support.
func Survey(doc *v3.Document, name string) (*Type, []error) {
	proxy, err := component(doc, name)
	if err != nil {
		return nil, []error{err}
	}
	var issues []error
	w := walk{stack: []string{componentPrefix + name}, issues: &issues}
	t, err := typeOf(proxy, name, w)
	if err != nil {
		return nil, append(issues, err)
	}
	t.Schema = name // the component itself is not a $ref
	return t, issues
}

// BuildType builds the type of the component schema name and of every type
// inside it, for the type mode of the generator (D20). It stops at the first
// unsupported shape, as Build does. The type must be an object or a oneOf
// with fields.
func BuildType(doc *v3.Document, name string) (*Type, error) {
	proxy, err := component(doc, name)
	if err != nil {
		return nil, err
	}
	t, err := typeOf(proxy, name, walk{stack: []string{componentPrefix + name}})
	if err != nil {
		return nil, err
	}
	if t.Kind != Object && t.Kind != OneOf || len(t.Fields) == 0 {
		return nil, fmt.Errorf("%s: %s with %d fields, want an object or a oneOf with fields", name, t.Kind, len(t.Fields))
	}
	t.Schema = name // the component itself is not a $ref
	return t, nil
}

// BuildEnum builds the enum component schema name, for the enum names of
// the type mode (F54).
func BuildEnum(doc *v3.Document, name string) (*Type, error) {
	proxy, err := component(doc, name)
	if err != nil {
		return nil, err
	}
	w := walk{stack: []string{componentPrefix + name}}
	s, err := schemaOf(proxy)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	t := &Type{Schema: name, Format: s.Format}
	if len(s.Type) != 1 {
		return nil, fmt.Errorf("%s: type %v, want an enum", name, s.Type)
	}
	if err := fillType(t, s, name, w); err != nil {
		return nil, err
	}
	if t.Kind != Enum {
		return nil, fmt.Errorf("%s: %s, want an enum", name, t.Kind)
	}
	return t, nil
}

func component(doc *v3.Document, name string) (*base.SchemaProxy, error) {
	if doc.Components == nil || doc.Components.Schemas == nil {
		return nil, errors.New("spec has no component schemas")
	}
	proxy := doc.Components.Schemas.GetOrZero(name)
	if proxy == nil {
		return nil, fmt.Errorf("component %s not found", name)
	}
	return proxy, nil
}

func findOperations(doc *v3.Document, name string) (map[verb]foundOp, error) {
	found := map[verb]foundOp{}
	if doc.Paths == nil {
		return nil, errors.New("spec has no paths")
	}
	for path, item := range doc.Paths.PathItems.FromOldest() {
		for method, op := range item.GetOperations().FromOldest() {
			for _, v := range append(slices.Clone(verbs), opReplace) {
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
	if err := mergeReplace(found); err != nil {
		return nil, err
	}
	get, singleton := found[opGet]
	singleton = singleton && len(pathParams(get)) == 0
	for _, v := range verbs {
		f, ok := found[v]
		suffix := "_" + string(v) + name
		if v == opUpdate {
			suffix += " or _" + string(opReplace) + name
		}
		if !ok && singleton {
			return nil, fmt.Errorf("%s: a singleton (Get has no path parameter) needs Create, Get, Update, and Delete on one path; no operation with operationId suffix %s", v, suffix)
		}
		if !ok {
			return nil, fmt.Errorf("%s: no operation with operationId suffix %s", v, suffix)
		}
		if !slices.Contains(verbMethods[v], f.method) {
			return nil, fmt.Errorf("%s: %s is %s, want %s", v, f.op.OperationId, f.method, strings.Join(verbMethods[v], " or "))
		}
	}
	return found, nil
}

// mergeReplace stores a Replace operation as the Update. A resource has one
// of them, and a Replace is always a PUT.
func mergeReplace(found map[verb]foundOp) error {
	rep, ok := found[opReplace]
	if !ok {
		return nil
	}
	if up, ok := found[opUpdate]; ok {
		return fmt.Errorf("%s: two operations: %s and %s", opUpdate, up.op.OperationId, rep.op.OperationId)
	}
	if !slices.Contains(verbMethods[opReplace], rep.method) {
		return fmt.Errorf("%s: %s is %s, want PUT", opReplace, rep.op.OperationId, rep.method)
	}
	found[opUpdate] = rep
	delete(found, opReplace)
	return nil
}

func (r *Resource) readOperations(ops map[verb]foundOp) error {
	if len(pathParams(ops[opGet])) == 0 {
		if err := checkSingleton(ops); err != nil {
			return err
		}
		r.Singleton = true
		return r.readTargets(ops)
	}
	id, err := idParam(ops[opGet])
	if err != nil {
		return fmt.Errorf("get: %w", err)
	}
	r.IDParam = id
	item, err := r.itemOperations(ops)
	if err != nil {
		return err
	}
	for _, v := range item {
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
	for _, v := range item {
		if ops[v].path != itemPath {
			return fmt.Errorf("%s: path %s, want %s (as in get)", v, ops[v].path, itemPath)
		}
	}

	return r.readTargets(ops)
}

// itemOperations returns the operations besides Get that have the id in the
// path: Delete, and Update unless it is on the Create path with no path
// parameters. Then the Update body has the id (IDInBody), as in a PUT on the
// collection (for example E2M and Slo).
func (r *Resource) itemOperations(ops map[verb]foundOp) ([]verb, error) {
	upd := ops[opUpdate]
	if len(pathParams(upd)) != 0 {
		return []verb{opUpdate, opDelete}, nil
	}
	if upd.path != ops[opCreate].path {
		return nil, fmt.Errorf("%s: path %s has no id; want the Get path %s, or the Create path %s with the id in the body",
			opUpdate, upd.path, ops[opGet].path, ops[opCreate].path)
	}
	r.IDInBody = true
	return []verb{opDelete}, nil
}

// checkSingleton checks a singleton (D18): Create, Get, Update, and Delete on
// one path, with no path parameters.
func checkSingleton(ops map[verb]foundOp) error {
	path := ops[opGet].path
	for _, v := range verbs {
		if ops[v].path != path {
			return fmt.Errorf("%s: a singleton has one path; %s is not %s (as in get)", v, ops[v].path, path)
		}
		if n := len(pathParams(ops[v])); n != 0 {
			return fmt.Errorf("%s: a singleton has no path parameters, this one has %d", v, n)
		}
	}
	return nil
}

// readTargets reads the method, path, body, and response of each operation.
func (r *Resource) readTargets(ops map[verb]foundOp) error {
	var err error
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

// readBodyTitles reads the title of each inline request body, and whether a
// component schema has the same name.
func (r *Resource) readBodyTitles(doc *v3.Document, ops map[verb]foundOp) {
	for v, o := range map[verb]*Operation{opCreate: &r.Create, opUpdate: &r.Update} {
		proxy := bodyProxy(ops[v].op)
		if o.Body != "inline" || proxy.Schema() == nil {
			continue
		}
		o.BodyTitle = proxy.Schema().Title
		o.BodyTitleIsComponent = o.BodyTitle != "" && doc.Components != nil && doc.Components.Schemas != nil &&
			doc.Components.Schemas.GetOrZero(o.BodyTitle) != nil
	}
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

// emptyResponse sets Empty when the response schema has no fields.
func emptyResponse(out Response, proxy *base.SchemaProxy) (Response, error) {
	rs, err := schemaOf(proxy)
	if err != nil {
		return Response{}, err
	}
	out.Empty = rs.Properties == nil || rs.Properties.Len() == 0
	return out, nil
}

// response reads the 200 response. When wrapped is true, the response must
// have exactly one property, and that property must be the resource.
// response reads the 200 response. With wrapped, it must return the resource:
// the resource itself (Direct), or one field that wraps it.
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
		return emptyResponse(out, proxy)
	}
	if name == r.Name {
		out.Direct = true
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
	names := propertyNames(getSchema)
	for _, body := range []struct {
		schema *base.Schema
		update bool
	}{{createBody, false}, {updateBody, true}} {
		for _, n := range propertyNames(body.schema) {
			p, err := r.requestProperty(body.schema, n, body.update)
			if err != nil {
				return fmt.Errorf("%s: %w", r.Name, err)
			}
			if p != nil && !slices.Contains(names, n) {
				names = append(names, n)
			}
		}
	}
	for _, n := range names {
		f, err := r.resourceField(n, createBody, updateBody, getSchema)
		if err != nil {
			return fmt.Errorf("%s: %w", r.Name, err)
		}
		r.Fields = append(r.Fields, f)
	}
	groups, err := r.rootGroups(getSchema)
	if err != nil {
		return fmt.Errorf("%s: %w", r.Name, err)
	}
	r.Groups = groups
	return nil
}

// rootGroups returns the oneOf groups among the top-level fields: the oneOf of
// the resource schema and of its allOf entries. An arm is an optional
// top-level field, and a field is in at most one group.
func (r *Resource) rootGroups(getSchema *base.Schema) ([]OneOfGroup, error) {
	if len(getSchema.OneOf) == 0 && !groupsOnlyAllOf(getSchema) {
		return nil, nil
	}
	schemas := []*base.Schema{getSchema}
	for _, proxy := range getSchema.AllOf {
		e, err := schemaOf(proxy)
		if err != nil {
			return nil, err
		}
		schemas = append(schemas, e)
	}
	used := map[string]bool{}
	var groups []OneOfGroup
	for _, e := range schemas {
		if len(e.OneOf) == 0 {
			continue
		}
		g, err := oneOfArms(e)
		if err != nil {
			return nil, err
		}
		for _, arm := range g.Arms {
			i := slices.IndexFunc(r.Fields, func(f *ResourceField) bool { return f.Name == arm })
			switch {
			case i < 0:
				return nil, fmt.Errorf("oneOf arm %s is not a field", arm)
			case used[arm]:
				return nil, fmt.Errorf("oneOf arm %s is in two groups", arm)
			case r.Fields[i].Create != nil && r.Fields[i].Create.Required:
				return nil, fmt.Errorf("oneOf arm %s is required in Create", arm)
			}
			used[arm] = true
		}
		groups = append(groups, g)
	}
	return groups, nil
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

// checkBodies checks the update mask, the id fields, and the required lists.
func (r *Resource) checkBodies(createBody, updateBody, getSchema *base.Schema) error {
	if err := r.checkUpdateBody(updateBody); err != nil {
		return err
	}
	for _, loc := range []location{{"create body", createBody}, {r.Name, getSchema}} {
		if loc.schema.Properties.GetOrZero(updateMaskField) != nil {
			return fmt.Errorf("%s: unexpected %s property", loc.name, updateMaskField)
		}
	}
	if !r.Singleton && getSchema.Properties.GetOrZero(r.IDParam) == nil {
		return fmt.Errorf("%s: no field %q for the id path parameter", r.Name, r.IDParam)
	}
	for _, loc := range []location{{"create body", createBody}, {"update body", updateBody}, {r.Name, getSchema}} {
		if err := checkRequired(loc.schema); err != nil {
			return fmt.Errorf("%s: %w", loc.name, err)
		}
	}
	return nil
}

// checkUpdateBody checks the properties of the Update body that are not
// resource fields: the update mask of a PATCH, which a full replace (PUT)
// does not have, and the id when the Update path has none.
func (r *Resource) checkUpdateBody(updateBody *base.Schema) error {
	mask := updateBody.Properties.GetOrZero(updateMaskField)
	switch {
	case r.Replace && mask != nil:
		return fmt.Errorf("update body: a full replace (PUT) has no %s property", updateMaskField)
	case !r.Replace && mask == nil:
		return fmt.Errorf("update body: no %s property", updateMaskField)
	case !r.Replace:
		ms, err := schemaOf(mask)
		if err != nil || !slices.Equal(ms.Type, []string{"string"}) {
			return fmt.Errorf("update body: %s must be a string", updateMaskField)
		}
		r.UpdateMaskPattern = ms.Pattern
	}
	if !r.IDInBody {
		return nil
	}
	id := updateBody.Properties.GetOrZero(r.IDParam)
	if id == nil {
		return fmt.Errorf("update body: the Update path has no id, and the body has no %q property", r.IDParam)
	}
	s, err := schemaOf(id)
	if err != nil || !slices.Equal(s.Type, []string{"string"}) || s.ReadOnly != nil && *s.ReadOnly {
		return fmt.Errorf("update body: the id %q must be a string that is not readOnly", r.IDParam)
	}
	return nil
}

// requestProperty returns the property name of a request body when it is a
// resource field, else nil. These properties are not resource fields: the
// update mask, the id in the Update body (IDInBody), and a readOnly property.
// A readOnly property is set by the server (proto OUTPUT_ONLY). A full
// replace often sends the whole resource, so its body also has them, for
// example createTime.
func (r *Resource) requestProperty(body *base.Schema, name string, update bool) (*base.SchemaProxy, error) {
	p := body.Properties.GetOrZero(name)
	if p == nil || name == updateMaskField || update && r.IDInBody && name == r.IDParam {
		return nil, nil
	}
	s, err := schemaOf(p)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	if s.ReadOnly != nil && *s.ReadOnly {
		return nil, nil
	}
	return p, nil
}

func (r *Resource) resourceField(name string, createBody, updateBody, getSchema *base.Schema) (*ResourceField, error) {
	cp, err := r.requestProperty(createBody, name, false)
	if err != nil {
		return nil, fmt.Errorf("create body: %w", err)
	}
	up, err := r.requestProperty(updateBody, name, true)
	if err != nil {
		return nil, fmt.Errorf("update body: %w", err)
	}
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
	if f.Type, err = typeOf(gp, name, walk{}); err != nil {
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
		t, err := typeOf(loc.proxy, name, walk{})
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
	a.ReadOnly = s.ReadOnly != nil && *s.ReadOnly
	return a, nil
}

// typeOf builds the type of a schema. path is the field path for errors.
func typeOf(proxy *base.SchemaProxy, path string, w walk) (*Type, error) {
	w, err := pushRef(proxy, path, w)
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
	if len(s.AllOf) > 0 && !groupsOnlyAllOf(s) {
		inner, err := singleAllOf(s, path)
		switch {
		case err == nil:
			return typeOf(inner, path, w)
		case s.Properties == nil || !w.keep(err):
			return nil, err
		}
		// Survey: an object with fields and allOf (for example several oneOf
		// groups). Go on with the fields only.
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
	if err := fillType(t, s, path, w); err != nil {
		return nil, err
	}
	return t, nil
}

// walk is the state of one typeOf walk.
type walk struct {
	stack []string // the $refs being built, to find loops
	// issues collects the errors of Survey. With issues, the walk records an
	// error and goes on. Without, it stops at the first error.
	issues *[]error
}

// keep records err when the walk collects errors, and reports whether the
// caller can go on.
func (w walk) keep(err error) bool {
	if w.issues == nil {
		return false
	}
	*w.issues = append(*w.issues, err)
	return true
}

// pushRef adds the $ref of proxy to the stack. A $ref that is already in the
// stack is a loop.
func pushRef(proxy *base.SchemaProxy, path string, w walk) (walk, error) {
	ref := proxy.GetReference()
	if ref == "" {
		return w, nil
	}
	if slices.Contains(w.stack, ref) {
		return w, fmt.Errorf("%s: recursive schema %s is not supported", path, ref)
	}
	w.stack = append(slices.Clone(w.stack), ref)
	return w, nil
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
func fillType(t *Type, s *base.Schema, path string, w walk) error {
	var err error
	switch s.Type[0] {
	case "object":
		// Errors from objectType and arrayType already name the path.
		return objectType(t, s, path, w)
	case "array":
		return arrayType(t, s, path, w)
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
	case s.Discriminator != nil && !discriminatorField(s):
		return errors.New("discriminator is supported only as a string field beside a oneOf, with no mapping")
	}
	return nil
}

// discriminatorField reports whether the discriminator of s names a string
// field of s beside a oneOf, with no mapping. Then it is only a label: the
// model keeps the field as a normal field (F36).
func discriminatorField(s *base.Schema) bool {
	d := s.Discriminator
	if d.Mapping != nil && d.Mapping.Len() != 0 || s.Properties == nil || len(s.OneOf) == 0 {
		return false
	}
	p := s.Properties.GetOrZero(d.PropertyName)
	if p == nil {
		return false
	}
	ps, err := schemaOf(p)
	return err == nil && slices.Equal(ps.Type, []string{"string"})
}

func objectType(t *Type, s *base.Schema, path string, w walk) error {
	if ap := s.AdditionalProperties; ap != nil && (ap.IsA() || ap.B) {
		return mapType(t, s, path, w)
	}
	if err := checkRequired(s); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	fields, err := objectFields(s, path, w)
	if err != nil {
		return err
	}
	t.Fields = fields
	if len(s.OneOf) == 0 && len(s.AllOf) == 0 {
		t.Kind = Object
		return nil
	}
	groups, err := oneOfGroups(s, path, fields)
	if err != nil {
		if !w.keep(err) {
			return err
		}
		t.Kind = Object // Survey: go on as an object, to see the fields
		return nil
	}
	// One group with every field as an arm is a oneOf. Else the object has
	// normal fields and groups.
	if s.Discriminator != nil {
		t.Discriminator = s.Discriminator.PropertyName
	}
	if len(groups) == 1 && sameSet(groups[0].Arms, propertyNames(s)) {
		t.Kind, t.AllowNone = OneOf, groups[0].AllowNone
		return nil
	}
	t.Kind, t.Groups = Object, groups
	return nil
}

// groupsOnlyAllOf reports whether s is an object whose allOf entries are
// only oneOf groups, like a proto message with more than one oneof.
func groupsOnlyAllOf(s *base.Schema) bool {
	if s.Properties == nil || len(s.AllOf) == 0 {
		return false
	}
	for _, proxy := range s.AllOf {
		e, err := schemaOf(proxy)
		if err != nil || proxy.IsReference() || len(e.OneOf) == 0 || e.Properties != nil || len(e.AllOf) != 0 {
			return false
		}
	}
	return true
}

// oneOfGroups returns the oneOf groups of s: its own oneOf, and the oneOf of
// each allOf entry. The arms must be fields with no attributes, and a field
// is in at most one group.
func oneOfGroups(s *base.Schema, path string, fields []*Field) ([]OneOfGroup, error) {
	schemas := []*base.Schema{s}
	for _, proxy := range s.AllOf {
		e, err := schemaOf(proxy)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		schemas = append(schemas, e)
	}
	byName := map[string]*Field{}
	for _, f := range fields {
		byName[f.Name] = f
	}
	used := map[string]bool{}
	var groups []OneOfGroup
	for _, e := range schemas {
		if len(e.OneOf) == 0 {
			continue
		}
		g, err := oneOfArms(e)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		for _, arm := range g.Arms {
			f := byName[arm]
			switch {
			case f == nil:
				return nil, fmt.Errorf("%s: oneOf arm %s is not a field", path, arm)
			case used[arm]:
				return nil, fmt.Errorf("%s: oneOf arm %s is in two groups", path, arm)
			case f.Attrs != (Attrs{}):
				return nil, fmt.Errorf("%s: oneOf arm %s has attributes %+v, want none", path, arm, f.Attrs)
			}
			used[arm] = true
		}
		groups = append(groups, g)
	}
	return groups, nil
}

func objectFields(s *base.Schema, path string, w walk) ([]*Field, error) {
	var fields []*Field
	for _, name := range propertyNames(s) {
		proxy := s.Properties.GetOrZero(name)
		ft, err := typeOf(proxy, path+"."+name, w)
		if err != nil {
			if !w.keep(err) {
				return nil, err
			}
			ft = &Type{Kind: String} // Survey: a placeholder, to go on
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

// oneOfArms reads the oneOf of s: one {required: [arm]} entry for each arm,
// and at most one "no arm" entry {not: {anyOf: [...]}} that lists every arm.
func oneOfArms(s *base.Schema) (OneOfGroup, error) {
	var arms []string
	allowNone := false
	var none []string
	for i, proxy := range s.OneOf {
		entry, err := schemaOf(proxy)
		if err != nil {
			return OneOfGroup{}, fmt.Errorf("oneOf[%d]: %w", i, err)
		}
		if entry.Properties != nil || len(entry.AllOf) != 0 || len(entry.OneOf) != 0 || len(entry.AnyOf) != 0 {
			return OneOfGroup{}, fmt.Errorf("oneOf[%d]: want only required or not", i)
		}
		if entry.Not == nil {
			if len(entry.Required) != 1 {
				return OneOfGroup{}, fmt.Errorf("oneOf[%d]: required %v, want one arm", i, entry.Required)
			}
			arms = append(arms, entry.Required[0])
			continue
		}
		if allowNone {
			return OneOfGroup{}, fmt.Errorf("oneOf[%d]: second \"no arm\" entry", i)
		}
		if none, err = noArmList(entry.Not); err != nil {
			return OneOfGroup{}, fmt.Errorf("oneOf[%d]: %w", i, err)
		}
		allowNone = true
	}
	if allowNone && !sameSet(none, arms) {
		return OneOfGroup{}, fmt.Errorf("oneOf: not.anyOf lists %v, want the arms %v", none, arms)
	}
	return OneOfGroup{Arms: arms, AllowNone: allowNone}, nil
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
func mapType(t *Type, s *base.Schema, path string, w walk) error {
	ap := s.AdditionalProperties
	if !ap.IsA() || ap.A == nil {
		return fmt.Errorf("%s: a map without a value schema (additionalProperties: true) is not supported", path)
	}
	if s.Properties != nil && s.Properties.Len() != 0 || len(s.OneOf) != 0 {
		return fmt.Errorf("%s: an object with both properties and additionalProperties is not supported", path)
	}
	elem, err := typeOf(ap.A, path+"{}", w)
	if err != nil {
		if !w.keep(err) {
			return err
		}
		elem = &Type{Kind: String}
	}
	t.Kind, t.Elem = Map, elem
	return nil
}

func arrayType(t *Type, s *base.Schema, path string, w walk) error {
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
	elem, err := typeOf(s.Items.A, path+"[]", w)
	if err != nil {
		if !w.keep(err) {
			return err
		}
		elem = &Type{Kind: String}
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
		var all []string
		for _, n := range s.Enum {
			all = append(all, n.Value)
		}
		t.EnumPrefix = enumPrefix(t.Schema, all)
		for _, v := range all {
			// Contract: the enum zero value is *_UNSPECIFIED, and it is not a
			// valid value. Older enums also give it a real meaning, for
			// example MORE_THAN_OR_UNSPECIFIED ("more than", F50) and
			// VERTICAL_UNSPECIFIED ("vertical", F54). Those are kept.
			if enumZero(t.Schema, t.EnumPrefix, v) {
				t.Zero = v
				continue
			}
			t.Values = append(t.Values, v)
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

// enumPrefix returns the prefix of the enum values, with its "_": the
// longest prefix of whole words that every value has and that leaves a word
// in each (TEXT_ALIGNMENT_LEFT, TEXT_ALIGNMENT_RIGHT → TEXT_ALIGNMENT_;
// PRIORITY_TYPE_LOW, PRIORITY_TYPE_HIGH → PRIORITY_TYPE_). With one value,
// the last part of the enum name in upper snake case, when the value has
// it. Else "".
func enumPrefix(schema string, values []string) string {
	if len(values) == 1 {
		if p := enumTypePrefix(schema); allHavePrefix(values, p) {
			return p
		}
		return ""
	}
	if len(values) == 0 {
		return ""
	}
	words := strings.Split(values[0], "_")
	for n := len(words) - 1; n > 0; n-- {
		p := strings.Join(words[:n], "_") + "_"
		if allHavePrefix(values, p) {
			return p
		}
	}
	return ""
}

func allHavePrefix(values []string, p string) bool {
	for _, v := range values {
		if !strings.HasPrefix(v, p) || v == p {
			return false
		}
	}
	return true
}

// enumTypePrefix is the upper snake case of the last part of an enum name,
// with "_": "widgets.common.DataModeType" → "DATA_MODE_TYPE_".
func enumTypePrefix(schema string) string {
	name := schema[strings.LastIndex(schema, ".")+1:]
	var b strings.Builder
	for i, r := range name {
		if i > 0 && unicode.IsUpper(r) && !unicode.IsUpper(rune(name[i-1])) {
			b.WriteByte('_')
		}
		b.WriteRune(unicode.ToUpper(r))
	}
	if b.Len() == 0 {
		return ""
	}
	return b.String() + "_"
}

// enumZero reports whether the enum value v only means "not set": nothing but
// UNSPECIFIED after the prefix, or the enum name with _UNSPECIFIED (an enum
// whose other values have no prefix).
func enumZero(schema, prefix, v string) bool {
	return strings.TrimPrefix(v, prefix) == "UNSPECIFIED" || v == enumTypePrefix(schema)+"UNSPECIFIED"
}
