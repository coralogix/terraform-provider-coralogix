package model

import (
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"slices"
	"strings"

	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi/datamodel"
	"github.com/pb33f/libopenapi/datamodel/high/base"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"go.yaml.in/yaml/v4"
)

const (
	componentPrefix = "#/components/schemas/"
	jsonMedia       = "application/json"
	updateMaskField = "updateMask" // contract: PATCH has an updateMask query parameter
	// updateMaskProtoField is the proto name of the update mask. The OpenAPI
	// fork names query parameters with the proto name, and the gateway accepts
	// both names.
	updateMaskProtoField = "update_mask"
	extPresence          = "x-coralogix-presence"
	extCollection        = "x-coralogix-collection"
	// A decimal uint64 with at most 18 digits always fits in Terraform Int64.
	terraformInt64SafeDecimalDigits int64 = 18
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
	return BuildWithOperationIDs(doc, name, OperationIDs{})
}

// BuildWithOperationIDs builds a resource with optional explicit lifecycle operation IDs.
func BuildWithOperationIDs(doc *v3.Document, name string, ids OperationIDs) (*Resource, error) {
	return BuildWithPolicy(doc, name, ids, Policy{})
}

// BuildWithPolicy builds a resource under the given rule set.
func BuildWithPolicy(doc *v3.Document, name string, ids OperationIDs, policy Policy) (*Resource, error) {
	ops, err := findOperations(doc, name, ids, policy)
	if err != nil {
		return nil, err
	}
	r := &Resource{Name: name, Replace: ops[opUpdate].method == "PUT", Policy: policy}
	if err := r.readOperations(ops); err != nil {
		return nil, err
	}
	r.readBodyTitles(doc, ops)
	if err := r.readFields(doc, ops); err != nil {
		return nil, err
	}
	if !r.Singleton {
		index := slices.IndexFunc(r.Fields, func(field *ResourceField) bool { return field.Name == r.IDParam })
		if index < 0 {
			return nil, fmt.Errorf("%s: no field %q for the id path parameter", r.Name, r.IDParam)
		}
		r.IDType = r.Fields[index].Type
	}
	r.pruneSkipped()
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
	return SurveyWithPolicy(doc, name, Policy{})
}

// SurveyWithPolicy is Survey under the given rule set.
func SurveyWithPolicy(doc *v3.Document, name string, policy Policy) (*Type, []error) {
	if doc.Components == nil || doc.Components.Schemas == nil {
		return nil, []error{errors.New("spec has no component schemas")}
	}
	proxy := doc.Components.Schemas.GetOrZero(name)
	if proxy == nil {
		return nil, []error{fmt.Errorf("component %s not found", name)}
	}
	var issues []error
	w := walk{stack: []string{"#/components/schemas/" + name}, issues: &issues, policy: &policy}
	t, err := typeOf(proxy, name, w)
	if err != nil {
		return nil, append(issues, err)
	}
	t.Schema = name // the component itself is not a $ref
	return t, issues
}

func findOperations(doc *v3.Document, name string, ids OperationIDs, p Policy) (map[verb]foundOp, error) {
	found := map[verb]foundOp{}
	if doc.Paths == nil {
		return nil, errors.New("spec has no paths")
	}
	ids, err := p.operationIDs(ids)
	if err != nil {
		return nil, err
	}
	explicit := map[verb]string{opCreate: ids.Create, opGet: ids.Get, opUpdate: ids.Update, opDelete: ids.Delete}
	for _, v := range verbs {
		matches := matchingOperations(doc, name, v, explicit[v])
		if len(matches) > 1 {
			var names []string
			for _, match := range matches {
				names = append(names, match.op.OperationId)
			}
			slices.Sort(names)
			return nil, fmt.Errorf("%s: multiple operations: %s", v, strings.Join(names, ", "))
		}
		if len(matches) == 1 {
			found[v] = matches[0]
		}
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
		if !slices.Contains(p.methods(v), f.method) {
			return nil, fmt.Errorf("%s: %s is %s, want %s", v, f.op.OperationId, f.method, strings.Join(p.methods(v), " or "))
		}
		if params := unsupportedRequiredParameters(v, f); len(params) != 0 {
			return nil, fmt.Errorf("%s: required %s parameter %q is not supported", v, params[0].In, params[0].Name)
		}
	}
	return found, nil
}

func unsupportedRequiredParameters(role verb, op foundOp) []*v3.Parameter {
	var params []*v3.Parameter
	for _, p := range slices.Concat(op.item.Parameters, op.op.Parameters) {
		if role == opUpdate && p.In == "query" && isUpdateMaskName(p.Name) {
			continue
		}
		if p.In != "path" && p.Required != nil && *p.Required {
			params = append(params, p)
		}
	}
	return params
}

func matchingOperations(doc *v3.Document, name string, role verb, explicit string) []foundOp {
	var matches []foundOp
	for path, item := range doc.Paths.PathItems.FromOldest() {
		for method, operation := range item.GetOperations().FromOldest() {
			match := operation.OperationId == explicit && explicit != ""
			if explicit == "" {
				match = strings.HasSuffix(operation.OperationId, "_"+string(role)+name)
				if role == opUpdate {
					match = match || strings.HasSuffix(operation.OperationId, "_"+string(opReplace)+name)
				}
			}
			if match {
				matches = append(matches, foundOp{path: path, method: strings.ToUpper(method), item: item, op: operation})
			}
		}
	}
	return matches
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
		if v == opDelete && r.Policy.DeleteOperation != "" {
			if !deleteActionPath(itemPath, ops[v].path) {
				return fmt.Errorf("%s: path %s, want %s/<action>", v, ops[v].path, itemPath)
			}
			continue
		}
		if ops[v].path != itemPath {
			return fmt.Errorf("%s: path %s, want %s (as in get)", v, ops[v].path, itemPath)
		}
	}

	return r.readTargets(ops)
}

// deleteActionPath reports whether path is the item path plus one literal
// segment, for example /things/{id}/archive. A Delete override uses this path.
func deleteActionPath(itemPath, path string) bool {
	segment, ok := strings.CutPrefix(path, itemPath+"/")
	return ok && segment != "" && !strings.ContainsAny(segment, "/{}")
}

// itemOperations returns the operations besides Get that must have the id in
// the path. Body-only Update ids exist in older APIs, but are outside the first
// generator PR. Migration support can add that contract separately.
func (r *Resource) itemOperations(ops map[verb]foundOp) ([]verb, error) {
	upd := ops[opUpdate]
	if r.Policy.UpdateIDInBody {
		if n := len(pathParams(upd)); n != 0 {
			return nil, fmt.Errorf("%s: the id is in the request body, but the path has %d parameters", opUpdate, n)
		}
		if upd.path != ops[opCreate].path {
			return nil, fmt.Errorf("%s: path %s, want %s (as in create)", opUpdate, upd.path, ops[opCreate].path)
		}
		return []verb{opDelete}, nil
	}
	if len(pathParams(upd)) == 0 {
		return nil, fmt.Errorf("%s: the id must be a required path parameter; an id in the request body is not supported", opUpdate)
	}
	return []verb{opUpdate, opDelete}, nil
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
	if r.Policy.DeleteOperation != "" && ops[opDelete].op.RequestBody != nil {
		return fmt.Errorf("%s: %s has a request body, want none", opDelete, r.Delete.OperationID)
	}
	return nil
}

// readBodyTitles reads the title of each inline request body, and whether a
// component schema has the same name.
func (r *Resource) readBodyTitles(doc *v3.Document, ops map[verb]foundOp) {
	for v, o := range map[verb]*Operation{opCreate: &r.Create, opUpdate: &r.Update} {
		proxy := bodyProxy(ops[v].op)
		if o.Body != "inline" || proxy == nil || proxy.Schema() == nil {
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
	if op == nil || op.RequestBody == nil || op.RequestBody.Content == nil {
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
	case proxy == nil && op != nil && op.RequestBody != nil:
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
	if op == nil || op.Responses == nil || op.Responses.Codes == nil {
		return Response{}, errors.New("no responses")
	}
	resp := op.Responses.Codes.GetOrZero("200")
	if resp == nil || resp.Content == nil || resp.Content.GetOrZero(jsonMedia) == nil || resp.Content.GetOrZero(jsonMedia).Schema == nil {
		return Response{}, fmt.Errorf("no 200 %s response schema", jsonMedia)
	}
	proxy := responseRef(resp.Content.GetOrZero(jsonMedia).Schema)
	if proxy == nil {
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
	inner, err := componentName(unwrapRef(propertyOf(s, keys[0])))
	if err != nil {
		return Response{}, fmt.Errorf("response %s.%s: %w", name, keys[0], err)
	}
	if inner != r.Name {
		return Response{}, fmt.Errorf("response %s.%s is %s, want %s", name, keys[0], inner, r.Name)
	}
	out.Field = keys[0]
	return out, nil
}

// responseRef returns a 200 response schema that is "$ref: X", or the $ref
// inside "allOf: [$ref: X]". The OpenAPI fork writes the allOf form for a
// response_body field with a description, because OpenAPI 3.0 ignores the
// siblings of a $ref. It returns nil for any other schema.
func responseRef(proxy *base.SchemaProxy) *base.SchemaProxy {
	if proxy == nil || proxy.IsReference() {
		return proxy
	}
	s, err := schemaOf(proxy)
	if err != nil {
		return nil
	}
	if inner, err := singleAllOf(s, "200 response schema"); err == nil && inner.IsReference() {
		return inner
	}
	return nil
}

// unwrapRef returns the $ref of a property that is "$ref: X" or "allOf: [$ref: X]".
func unwrapRef(proxy *base.SchemaProxy) string {
	if proxy == nil {
		return ""
	}
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
	if proxy == nil {
		return nil, errors.New("missing schema")
	}
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
	if s == nil || s.Properties == nil {
		return nil
	}
	return slices.Collect(s.Properties.KeysFromOldest())
}

func propertyOf(s *base.Schema, name string) *base.SchemaProxy {
	if s == nil || s.Properties == nil {
		return nil
	}
	return s.Properties.GetOrZero(name)
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
	if err := r.checkBodies(createBody, updateBody, getSchema, ops[opUpdate]); err != nil {
		return err
	}
	names := propertyNames(getSchema)
	for _, body := range []*base.Schema{createBody, updateBody} {
		for _, n := range propertyNames(body) {
			p, err := r.requestProperty(body, n)
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
	groups, err := r.rootGroups(createBody)
	if err != nil {
		return fmt.Errorf("%s: %w", r.Name, err)
	}
	r.Groups = groups
	return nil
}

// rootGroups returns the oneOf groups among the configurable top-level fields:
// the oneOf of the Create request and of its allOf entries. An arm is an optional
// top-level field, and a field is in at most one group.
func (r *Resource) rootGroups(requestSchema *base.Schema) ([]OneOfGroup, error) {
	if len(requestSchema.OneOf) == 0 && !groupsOnlyAllOf(requestSchema) {
		return nil, nil
	}
	schemas := []*base.Schema{requestSchema}
	for _, proxy := range requestSchema.AllOf {
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
	if createBody, err = schemaOf(r.Policy.requestBody(ops[opCreate].op)); err != nil {
		return nil, nil, nil, fmt.Errorf("create body: %w", err)
	}
	if updateBody, err = schemaOf(r.Policy.requestBody(ops[opUpdate].op)); err != nil {
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
func (r *Resource) checkBodies(createBody, updateBody, getSchema *base.Schema, update foundOp) error {
	if err := r.checkUpdateContract(updateBody, update); err != nil {
		return err
	}
	for _, loc := range []location{{"create body", createBody}, {r.Name, getSchema}} {
		if name := maskProperty(loc.schema); name != "" {
			return fmt.Errorf("%s: unexpected %s property", loc.name, name)
		}
	}
	if !r.Singleton && propertyOf(getSchema, r.IDParam) == nil {
		return fmt.Errorf("%s: no field %q for the id path parameter", r.Name, r.IDParam)
	}
	for _, loc := range []location{{"create body", createBody}, {"update body", updateBody}, {r.Name, getSchema}} {
		if err := checkRequired(loc.schema); err != nil {
			return fmt.Errorf("%s: %w", loc.name, err)
		}
	}
	return nil
}

// checkUpdateContract checks the PATCH updateMask query parameter. A full
// replace has no update mask.
func (r *Resource) checkUpdateContract(updateBody *base.Schema, update foundOp) error {
	return r.checkUpdateMask(updateBody, update)
}

func (r *Resource) checkUpdateMask(updateBody *base.Schema, update foundOp) error {
	bodyMask := maskProperty(updateBody)
	params := maskParameters(update)
	if r.Replace {
		return checkNoUpdateMask(bodyMask, params)
	}
	if bodyMask != "" {
		return fmt.Errorf("update body: %s must be a query parameter, not a body property", bodyMask)
	}
	if len(params) == 0 {
		return fmt.Errorf("update parameters: no %s or %s query parameter", updateMaskField, updateMaskProtoField)
	}
	if len(params) > 1 {
		return fmt.Errorf("update parameters: %d %s or %s parameters, want 1", len(params), updateMaskField, updateMaskProtoField)
	}
	return r.readUpdateMask(params[0])
}

func checkNoUpdateMask(bodyMask string, params []*v3.Parameter) error {
	if bodyMask != "" {
		return fmt.Errorf("update body: a full replace (PUT) has no %s property", bodyMask)
	}
	if len(params) != 0 {
		return fmt.Errorf("update parameters: a full replace (PUT) has no %s parameter", params[0].Name)
	}
	return nil
}

func (r *Resource) readUpdateMask(mask *v3.Parameter) error {
	if mask.In != "query" {
		return fmt.Errorf("update parameters: %s is in %q, want query", mask.Name, mask.In)
	}
	if mask.Required == nil || !*mask.Required {
		return fmt.Errorf("update parameters: %s must be required", mask.Name)
	}
	if mask.Schema == nil {
		return fmt.Errorf("update parameters: %s has no schema", mask.Name)
	}
	ms, err := schemaOf(mask.Schema)
	if err != nil || !slices.Equal(ms.Type, []string{"string"}) {
		return fmt.Errorf("update parameters: %s must have a string schema", mask.Name)
	}
	r.UpdateMask = mask.Name
	r.UpdateMaskPattern = ms.Pattern
	return nil
}

func isUpdateMaskName(name string) bool {
	return name == updateMaskField || name == updateMaskProtoField
}

// maskProperty returns the name of the update mask property of s, or "".
func maskProperty(s *base.Schema) string {
	for _, name := range []string{updateMaskField, updateMaskProtoField} {
		if propertyOf(s, name) != nil {
			return name
		}
	}
	return ""
}

// maskParameters returns the update mask parameters of op, under either name.
func maskParameters(op foundOp) []*v3.Parameter {
	var params []*v3.Parameter
	for _, p := range slices.Concat(op.item.Parameters, op.op.Parameters) {
		if isUpdateMaskName(p.Name) {
			params = append(params, p)
		}
	}
	return params
}

// requestProperty returns the property name of a request body when it is a
// resource field, else nil. A PATCH update mask is a query parameter, not a
// body property. A readOnly property is not a request field.
// A readOnly property is set by the server (proto OUTPUT_ONLY). A full
// replace often sends the whole resource, so its body also has them, for
// example createTime.
func (r *Resource) requestProperty(body *base.Schema, name string) (*base.SchemaProxy, error) {
	if r.Policy.readOnly(r.Name, name) {
		return nil, nil // the server sets the field, so it is not in a request
	}
	return requestProperty(body, name)
}

func requestProperty(body *base.Schema, name string) (*base.SchemaProxy, error) {
	p := propertyOf(body, name)
	if p == nil {
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
	cp, err := r.requestProperty(createBody, name)
	if err != nil {
		return nil, fmt.Errorf("create body: %w", err)
	}
	up, err := r.requestProperty(updateBody, name)
	if err != nil {
		return nil, fmt.Errorf("update body: %w", err)
	}
	gp := propertyOf(getSchema, name)
	behavior, err := Classify(cp != nil, up != nil, gp != nil)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	f := &ResourceField{Name: name, Behavior: behavior, InGet: gp != nil}
	if f.Description, err = description(gp); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	// The Get type comes first: Classify ensures that the field is in Get.
	if f.Type, err = typeOf(gp, name, walk{policy: &r.Policy}); err != nil {
		return nil, err
	}
	getAttrs, err := attrsOf(getSchema, name, gp)
	if err != nil {
		return nil, fmt.Errorf("%s: get: %w", name, err)
	}
	f.Get = &getAttrs
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
		t, err := typeOf(loc.proxy, name, walk{policy: &r.Policy})
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
	if s == nil {
		return errors.New("missing schema")
	}
	for _, n := range s.Required {
		if propertyOf(s, n) == nil {
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
	// policy is the rule set. nil is the strict rule set for new resources.
	policy *Policy
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
	if proxy == nil {
		return w, nil
	}
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
		err = stringType(t, s, w)
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
	}
	if err := checkUnsupportedSchemaKeywords(s); err != nil {
		return err
	}
	if s.Discriminator != nil && !discriminatorField(s) {
		return errors.New("discriminator is supported only as a string field beside a oneOf, with no mapping")
	}
	return nil
}

func checkUnsupportedSchemaKeywords(s *base.Schema) error {
	switch {
	case s.PatternProperties != nil && s.PatternProperties.Len() != 0:
		return errors.New("patternProperties is not supported")
	case (s.MinProperties != nil || s.MaxProperties != nil) && !isMapSchema(s):
		return errors.New("minProperties and maxProperties are supported only on a map")
	case s.ExclusiveMinimum != nil || s.ExclusiveMaximum != nil:
		return errors.New("exclusiveMinimum and exclusiveMaximum are not supported")
	case s.Format == "uint64" && (!slices.Equal(s.Type, []string{"string"}) || s.MaxLength == nil || *s.MaxLength > terraformInt64SafeDecimalDigits):
		return fmt.Errorf("uint64 needs string type and maxLength at most %d to fit Terraform Int64", terraformInt64SafeDecimalDigits)
	case len(s.PrefixItems) != 0:
		return errors.New("prefixItems is not supported")
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
	p := propertyOf(s, d.PropertyName)
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
		proxy := propertyOf(s, name)
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
	t.MinItems, t.MaxItems = s.MinProperties, s.MaxProperties
	return nil
}

// isMapSchema reports whether s is a map: string keys with one value schema,
// and no fixed properties.
func isMapSchema(s *base.Schema) bool {
	return s.AdditionalProperties != nil && s.AdditionalProperties.IsA() &&
		(s.Properties == nil || s.Properties.Len() == 0) && len(s.OneOf) == 0
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
	if unique && !set {
		return fmt.Errorf("%s: uniqueItems alone does not define unordered collection semantics; %s must be set", path, extCollection)
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

func stringType(t *Type, s *base.Schema, w walk) error {
	t.MinLength, t.MaxLength = s.MinLength, s.MaxLength
	switch {
	case len(s.Enum) != 0:
		t.Kind = Enum
		t.MinLength, t.MaxLength = nil, nil
		zero, err := enumZero(s.Enum, w.policy != nil && w.policy.enumAnyPrefix(t.Schema))
		if err != nil {
			return err
		}
		t.EnumZero = zero
		for _, n := range s.Enum[1:] {
			t.Values = append(t.Values, n.Value)
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

// enumZero returns the exact protobuf zero value. The zero value is first,
// ends in _UNSPECIFIED, and supplies the prefix of every business value.
// A later business value may also end in _UNSPECIFIED. With anyPrefix (existing
// resources), a business value need not use the prefix.
func enumZero(values []*yaml.Node, anyPrefix bool) (string, error) {
	if len(values) < 2 {
		return "", errors.New("enum needs one zero value and at least one business value")
	}
	zero := values[0].Value
	prefix, ok := strings.CutSuffix(zero, "_UNSPECIFIED")
	if !ok || prefix == "" {
		return "", fmt.Errorf("first enum value %q must be <PREFIX>_UNSPECIFIED", zero)
	}
	for _, value := range values[1:] {
		if value.Value == zero {
			return "", fmt.Errorf("enum zero value %q is repeated", zero)
		}
		if !anyPrefix && !strings.HasPrefix(value.Value, prefix+"_") {
			return "", fmt.Errorf("enum value %q does not use zero-value prefix %q", value.Value, prefix)
		}
	}
	return zero, nil
}

var topLevelMaskEntry = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

// UpdateMaskRule returns the check for one mask path and whether the API
// accepts dotted paths. Validation and rendering use this one decision. The
// pattern describes the whole query value, and the generated code joins the
// changed paths with commas, so the pattern must accept a list such as a,b.
func UpdateMaskRule(pattern string) (valid func(string) bool, leaf bool, err error) {
	if pattern == "" {
		return topLevelMaskEntry.MatchString, false, nil
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, false, fmt.Errorf("update mask pattern %q: %w", pattern, err)
	}
	if !re.MatchString("a") || !re.MatchString("a,b") || re.MatchString("*") {
		return nil, false, fmt.Errorf("update mask pattern %q must accept a field name and a comma-separated list of names, and reject *", pattern)
	}
	return re.MatchString, re.MatchString("a.b"), nil
}
