package generator

import (
	"fmt"
	"slices"
	"strings"

	"github.com/coralogix/coralogix-management-sdk/iac/issue"
	"github.com/coralogix/coralogix-management-sdk/iac/model"
	"github.com/coralogix/coralogix-management-sdk/iac/sdkcheck"
	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/overrides"
	"go.yaml.in/yaml/v4"
)

// Two cases of behavior-overrides.yaml change the contract that the iac module reads, so the
// generator gives it the contract that they describe:
//
//   - api.operations with delete: none, and the same operation as create and update: a singleton
//     that a GET reads and a POST on the same path sets. The path gets a PUT that is a copy of the
//     Set POST, and a DELETE with the responses of the Get. The model then sees a singleton with
//     every lifecycle step. Their operationIds end in upsertPutSuffix and upsertDeleteSuffix, and
//     upsertSDKNames maps their SDK names to the Set and the Get. The renderer calls Set for Create
//     and Update, and makes no Delete call.
//   - inline: each inlined object of the resource component and of the Create and Update bodies is
//     replaced by its fields. The model then sees them as top-level fields. File.Inlined records
//     the object that holds each one, and inlineSDKNames moves their SDK names to the object's
//     type. An update mask names API fields, and an inlined field is inside its object in the API,
//     so the Update must send the whole resource: a PUT, or the Set call of an upsert.

// The operationIds of the PUT and the DELETE that reshape adds for an upsert.
const (
	upsertPutSuffix    = "_UpsertPut"
	upsertDeleteSuffix = "_UpsertDelete"
)

// upsertLocation is the line of the behavior-overrides file that names the upsert operations.
const upsertLocation = "behavior-overrides.yaml:api.operations"

// reshape returns the contract that the model reads for a resource with an upsert or inline lines,
// and the operation ids that select its lifecycle. It sets file.Inlined. A resource without them
// gets data and ids unchanged. policy is the policy of file.
func reshape(data []byte, component string, ids model.OperationIDs, file *overrides.File, policy model.Policy) ([]byte, model.OperationIDs, issue.Report) {
	inlined := file.InlinedFields()
	if file == nil || !file.Upserts() && len(inlined) == 0 {
		return data, ids, nil
	}
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil || len(root.Content) == 0 {
		return data, ids, issue.Report{{Code: "OPENAPI_INVALID", Location: "openapi", Message: fmt.Sprintf("read the contract: %v", err), Remediation: "Correct the source OpenAPI document."}}
	}
	doc := root.Content[0]
	schemas := mapValue(mapValue(doc, "components"), "schemas")
	h := inliner{schemas: schemas, fields: inlined, resource: component, file: file, done: map[*yaml.Node]map[string]overrides.Parent{}}
	shape := &overrides.Inlined{}
	if len(inlined) != 0 {
		response := mapValue(schemas, component)
		if response == nil {
			return data, ids, issue.Report{inlineIssue(component, "the contract has no component "+component)}
		}
		var err error
		if shape.Get, err = h.inline(response, true); err != nil {
			return data, ids, issue.Report{inlineIssue(component, component+": "+err.Error())}
		}
		h.done[response] = shape.Get
	}
	out := ids
	if file.Upserts() {
		if ids != (model.OperationIDs{}) {
			return data, ids, issue.Report{{Code: "UPSERT_OPERATION_CONFLICT", Location: upsertLocation, Message: "the overrides name the operations of the upsert, so no operation flag can name one", Remediation: "Name the operations of the upsert only in api.operations."}}
		}
		ops := file.API.Operations
		set, report := upsertOperations(doc, ops.Get, ops.Create)
		if len(report) != 0 {
			return data, ids, report
		}
		if len(inlined) != 0 {
			var err error
			if shape.Create, err = h.body(set); err != nil {
				return data, ids, issue.Report{inlineIssue(component, "create body: "+err.Error())}
			}
			shape.Update = shape.Create
		}
		out = model.OperationIDs{Create: ops.Create, Get: ops.Get, Update: ops.Create + upsertPutSuffix, Delete: ops.Get + upsertDeleteSuffix}
	}
	written, err := yaml.Marshal(&root)
	if err != nil {
		return data, ids, issue.Report{{Code: "OPENAPI_INVALID", Location: "openapi", Message: fmt.Sprintf("write the contract: %v", err), Remediation: "Correct the source OpenAPI document."}}
	}
	if !file.Upserts() {
		if report := h.inlineCreateAndUpdate(&root, component, ids, policy, shape); len(report) != 0 {
			return data, ids, report
		}
		if written, err = yaml.Marshal(&root); err != nil {
			return data, ids, issue.Report{{Code: "OPENAPI_INVALID", Location: "openapi", Message: fmt.Sprintf("write the contract: %v", err), Remediation: "Correct the source OpenAPI document."}}
		}
	}
	file.Inlined = shape
	return written, out, nil
}

// inlineCreateAndUpdate inlines the bodies of a resource without an upsert. The iac module finds
// the operations, but only in a contract whose bodies are inlined already, because a request field
// can sit at another depth than the response field before. So every request body with an inlined
// field is inlined; the Create and Update bodies that the model then selects give the parents, and
// an error counts only for them. The Update must send the whole resource.
func (h inliner) inlineCreateAndUpdate(root *yaml.Node, component string, ids model.OperationIDs, policy model.Policy, shape *overrides.Inlined) issue.Report {
	parents, errs := map[string]map[string]overrides.Parent{}, map[string]error{}
	paths := mapValue(root.Content[0], "paths")
	for i := 0; paths != nil && i+1 < len(paths.Content); i += 2 {
		item := paths.Content[i+1]
		for j := 0; j+1 < len(item.Content); j += 2 {
			op := item.Content[j+1]
			id := mapValue(op, "operationId")
			if id == nil || mapValue(op, "requestBody") == nil {
				continue
			}
			parents[id.Value], errs[id.Value] = h.body(op)
		}
	}
	data, err := yaml.Marshal(root)
	if err != nil {
		return issue.Report{inlineIssue(component, err.Error())}
	}
	doc, err := model.Load(data)
	if err != nil {
		return issue.Report{inlineIssue(component, err.Error())}
	}
	r, err := model.BuildWithPolicy(doc, component, ids, policy)
	if err != nil {
		return issue.Report{inlineIssue(component, "find the Create and Update operations of "+component+": "+err.Error())}
	}
	for _, op := range []struct{ name, id string }{{"create body", r.Create.OperationID}, {"update body", r.Update.OperationID}} {
		if errs[op.id] != nil {
			return issue.Report{inlineIssue(component, op.name+": "+errs[op.id].Error())}
		}
	}
	if !r.Replace {
		return issue.Report{inlineIssue(component, fmt.Sprintf("inline needs an Update that sends the whole resource, a PUT or the Set call of an upsert, but %s is %s with an update mask: the mask names API fields, and an inlined field is inside its object in the API", r.Update.OperationID, r.Update.Method))}
	}
	shape.Create, shape.Update = parents[r.Create.OperationID], parents[r.Update.OperationID]
	return nil
}

// body inlines the request body of op, once per schema: a body that is a component that another
// place inlined already gets the same parents.
func (h inliner) body(op *yaml.Node) (map[string]overrides.Parent, error) {
	schema := mapValue(mapValue(mapValue(mapValue(op, "requestBody"), "content"), "application/json"), "schema")
	if ref := refName(schema); ref != "" {
		schema = mapValue(h.schemas, ref)
	}
	if schema == nil {
		return nil, nil
	}
	if parents, ok := h.done[schema]; ok {
		return parents, nil
	}
	parents, err := h.inline(schema, false)
	if err != nil {
		return nil, err
	}
	h.done[schema] = parents
	return parents, nil
}

// upsertOperations checks the two calls of an upsert, a GET and a POST on one path without path
// parameters, and adds the PUT and the DELETE that the model needs. It returns the Set operation.
func upsertOperations(doc *yaml.Node, getID, setID string) (*yaml.Node, issue.Report) {
	getPath, getMethod, get := findOperation(doc, getID)
	setPath, setMethod, set := findOperation(doc, setID)
	var report issue.Report
	switch {
	case get == nil:
		report = append(report, upsertShapeIssue(".get", fmt.Sprintf("The contract has no operation %s.", getID)))
	case getMethod != "get" || strings.Contains(getPath, "{"):
		report = append(report, upsertShapeIssue(".get", fmt.Sprintf("%s is %s %s. A singleton that one call sets has a GET with no path parameters.", getID, strings.ToUpper(getMethod), getPath)))
	}
	switch {
	case set == nil:
		report = append(report, upsertShapeIssue(".create", fmt.Sprintf("The contract has no operation %s.", setID)))
	case get != nil && (setMethod != "post" || setPath != getPath):
		report = append(report, upsertShapeIssue(".create", fmt.Sprintf("%s is %s %s. The call that sets a singleton is a POST on the Get path %s, with no path parameters.", setID, strings.ToUpper(setMethod), setPath, getPath)))
	}
	if len(report) != 0 {
		return nil, report
	}
	item := mapValue(mapValue(doc, "paths"), getPath)
	for _, method := range []string{"put", "patch", "delete"} {
		if mapValue(item, method) != nil {
			return nil, issue.Report{upsertShapeIssue("", fmt.Sprintf("%s also has a %s operation, so the API has more than the two calls of a singleton that one call sets.", getPath, strings.ToUpper(method)))}
		}
	}
	// The PUT is the Set call; the DELETE makes no call in the renderer.
	put := cloneMapping(set)
	setMap(put, "operationId", &yaml.Node{Kind: yaml.ScalarNode, Value: setID + upsertPutSuffix})
	setMap(item, "put", put)
	del := &yaml.Node{Kind: yaml.MappingNode}
	setMap(del, "operationId", &yaml.Node{Kind: yaml.ScalarNode, Value: getID + upsertDeleteSuffix})
	for _, key := range []string{"responses", "tags"} {
		if v := mapValue(get, key); v != nil {
			setMap(del, key, v)
		}
	}
	setMap(item, "delete", del)
	return set, nil
}

func upsertShapeIssue(suffix, message string) issue.Issue {
	return issue.Issue{Code: "UPSERT_SHAPE_UNSUPPORTED", Location: upsertLocation + suffix, Message: message, Remediation: "Name the GET of the singleton as api.operations.get, and the POST on its path as create and update."}
}

func inlineIssue(component, message string) issue.Issue {
	return issue.Issue{Code: "INLINE_UNSUPPORTED", Location: "components.schemas." + component, Message: message, Remediation: "Inline a required object, or an arm of a oneOf group whose other arms the file skips, that has no field name of its parent."}
}

// inliner replaces the inlined objects of a schema by their fields.
type inliner struct {
	// done has the parents of each schema that inline replaced already.
	done     map[*yaml.Node]map[string]overrides.Parent
	schemas  *yaml.Node
	fields   []string // the inlined fields of the resource
	resource string
	file     *overrides.File
}

// inline replaces each inlined object of s by its fields, in place and in order. An inlined object
// must be sent in every request that has it, so that its required fields are sent: it must be
// required there. In the response it is required, or an arm of a oneOf group whose other arms the
// file skips: an absent arm then means that the resource does not exist. The skipped arms stay as
// plain fields, which their skip lines name. It returns the object that holds each moved field.
func (h inliner) inline(s *yaml.Node, response bool) (map[string]overrides.Parent, error) {
	props := mapValue(s, "properties")
	if props == nil || !slices.ContainsFunc(h.fields, func(f string) bool { return mapValue(props, f) != nil }) {
		return nil, nil
	}
	if mapValue(s, "allOf") != nil || mapValue(s, "anyOf") != nil {
		return nil, fmt.Errorf("inline: an object with allOf or anyOf is not supported")
	}
	required := stringList(mapValue(s, "required"))
	arm, err := h.inlinedArm(s, response)
	if err != nil {
		return nil, err
	}
	parents := map[string]overrides.Parent{}
	flat := &yaml.Node{Kind: yaml.MappingNode}
	var flatRequired []string
	add := func(key, value *yaml.Node, isRequired bool) error {
		if mapValue(flat, key.Value) != nil {
			return fmt.Errorf("inline: two fields have the name %q", key.Value)
		}
		flat.Content = append(flat.Content, key, value)
		if isRequired {
			flatRequired = append(flatRequired, key.Value)
		}
		return nil
	}
	for i := 0; i+1 < len(props.Content); i += 2 {
		key, value := props.Content[i], props.Content[i+1]
		name := key.Value
		if !slices.Contains(h.fields, name) {
			if err := add(key, value, slices.Contains(required, name)); err != nil {
				return nil, err
			}
			continue
		}
		component := refName(value)
		if component == "" {
			if all := mapValue(value, "allOf"); all != nil && len(all.Content) == 1 {
				component = refName(all.Content[0])
			}
		}
		object := mapValue(h.schemas, component)
		if component == "" || object == nil {
			return nil, fmt.Errorf("inline: %s must be a $ref to an object component", name)
		}
		objectProps := mapValue(object, "properties")
		if objectProps == nil || len(objectProps.Content) == 0 || mapValue(object, "oneOf") != nil || mapValue(object, "allOf") != nil || mapValue(object, "anyOf") != nil {
			return nil, fmt.Errorf("inline: %s must be an object with fields and no oneOf, allOf, or anyOf", name)
		}
		isArm := name == arm
		if !isArm && !slices.Contains(required, name) {
			return nil, fmt.Errorf("inline: %s must be required, so that its fields are always in the API value", name)
		}
		objectRequired := stringList(mapValue(object, "required"))
		for j := 0; j+1 < len(objectProps.Content); j += 2 {
			child := objectProps.Content[j].Value
			if slices.Contains(h.fields, child) {
				return nil, fmt.Errorf("inline: %s.%s is inlined too, but only a field of the resource can be", name, child)
			}
			if err := add(objectProps.Content[j], objectProps.Content[j+1], slices.Contains(objectRequired, child)); err != nil {
				return nil, err
			}
			parents[child] = overrides.Parent{Field: name, Schema: component, Arm: isArm}
		}
	}
	setMap(s, "properties", flat)
	setMap(s, "required", stringNode(flatRequired))
	if arm != "" {
		deleteMap(s, "oneOf") // the group is the inlined arm; its other arms are skipped fields
	}
	return parents, nil
}

// inlinedArm returns the inlined field that is an arm of the oneOf group of s, or "". The other
// arms must be skipped, so that the group means that the inlined arm is set, or that the resource
// does not exist. A request cannot have an inlined arm: the request must always send the object.
func (h inliner) inlinedArm(s *yaml.Node, response bool) (string, error) {
	group := mapValue(s, "oneOf")
	if group == nil {
		return "", nil
	}
	var arms []string
	for _, entry := range group.Content {
		arms = append(arms, stringList(mapValue(entry, "required"))...)
	}
	arm := ""
	for _, a := range arms {
		if slices.Contains(h.fields, a) {
			arm = a
		}
	}
	if arm == "" {
		return "", nil
	}
	if !response {
		return "", fmt.Errorf("inline: %s is an arm of a oneOf group of a request, so the request does not always send it", arm)
	}
	for _, other := range arms {
		if other != arm && !h.file.Types[h.resource].Fields[other].Skip {
			return "", fmt.Errorf("inline: %s is an arm of a oneOf group; skip the other arm %s, or the resource could be that arm", arm, other)
		}
	}
	return arm, nil
}

// upsertSDKNames gives the SDK names of the PUT and the DELETE that reshape added the names of the
// Set and the Get: the SDK has only those two calls.
func upsertSDKNames(refs []sdkcheck.Ref, file *overrides.File) []sdkcheck.Ref {
	if !file.Upserts() {
		return refs
	}
	ops := file.API.Operations
	names := strings.NewReplacer(
		camelize(ops.Create+upsertPutSuffix), camelize(ops.Create),
		camelize(ops.Get+upsertDeleteSuffix), camelize(ops.Get))
	out := slices.Clone(refs)
	for i := range out {
		out[i].Name, out[i].Owner = names.Replace(out[i].Name), names.Replace(out[i].Owner)
		out[i].Want, out[i].WantValue = names.Replace(out[i].Want), names.Replace(out[i].WantValue)
	}
	return out
}

// inlineSDKNames moves the SDK names of the inlined fields to the type of the object that holds
// them, and adds the object: its field of the root type at <root>.<object>, and its type. The
// model gave them to the root type, because it reads the reshaped contract.
func inlineSDKNames(refs []sdkcheck.Ref, file *overrides.File) []sdkcheck.Ref {
	if file == nil || file.Inlined == nil {
		return refs
	}
	roots := []struct {
		path    string
		parents map[string]overrides.Parent
	}{{"fields", file.Inlined.Get}, {"create.body", file.Inlined.Create}, {"update.body", file.Inlined.Update}}
	out := slices.Clone(refs)
	for _, root := range roots {
		rootRef := slices.IndexFunc(out, func(r sdkcheck.Ref) bool { return r.Path == root.path && r.Kind == sdkcheck.KindType })
		if rootRef < 0 || len(root.parents) == 0 {
			continue
		}
		rootType, pkg := out[rootRef].Name, out[rootRef].Pkg
		added := map[string]bool{}
		for i, ref := range out {
			name, ok := strings.CutPrefix(ref.Path, root.path+".")
			parent, inlined := root.parents[name]
			if !ok || !inlined || ref.Kind != sdkcheck.KindField || ref.Owner != rootType {
				continue
			}
			typ := model.GoName(parent.Schema)
			out[i].Owner = typ
			if added[parent.Field] {
				continue
			}
			added[parent.Field] = true
			path := root.path + "." + parent.Field
			// A required object can be a value in the SDK. An arm of a oneOf is a pointer.
			out = append(out,
				sdkcheck.Ref{Path: path, Kind: sdkcheck.KindField, Pkg: pkg, Owner: rootType, Name: goFieldName(parent.Field), Want: "*" + typ, WantValue: typ, Rule: sdkcheck.RuleProperty},
				sdkcheck.Ref{Path: path, Kind: sdkcheck.KindType, Pkg: pkg, Name: typ, Rule: sdkcheck.RuleComponent, Schema: parent.Schema})
		}
	}
	return out
}

// findOperation returns the path, the lower-case method, and the node of the operation with the
// operationId id, or a nil node.
func findOperation(doc *yaml.Node, id string) (string, string, *yaml.Node) {
	paths := mapValue(doc, "paths")
	if paths == nil {
		return "", "", nil
	}
	for i := 0; i+1 < len(paths.Content); i += 2 {
		item := paths.Content[i+1]
		for j := 0; j+1 < len(item.Content); j += 2 {
			op := item.Content[j+1]
			if v := mapValue(op, "operationId"); v != nil && v.Value == id {
				return paths.Content[i].Value, item.Content[j].Value, op
			}
		}
	}
	return "", "", nil
}

func mapValue(m *yaml.Node, key string) *yaml.Node {
	if m == nil || m.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	return nil
}

func setMap(m *yaml.Node, key string, value *yaml.Node) {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			m.Content[i+1] = value
			return
		}
	}
	m.Content = append(m.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: key}, value)
}

func deleteMap(m *yaml.Node, key string) {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			m.Content = append(m.Content[:i], m.Content[i+2:]...)
			return
		}
	}
}

func cloneMapping(m *yaml.Node) *yaml.Node {
	out := *m
	out.Content = slices.Clone(m.Content)
	return &out
}

// refName returns the component that a {$ref: '#/components/schemas/X'} node names, or "".
func refName(n *yaml.Node) string {
	ref := mapValue(n, "$ref")
	if ref == nil {
		return ""
	}
	name, ok := strings.CutPrefix(ref.Value, "#/components/schemas/")
	if !ok {
		return ""
	}
	return name
}

func stringList(n *yaml.Node) []string {
	if n == nil {
		return nil
	}
	var out []string
	for _, item := range n.Content {
		out = append(out, item.Value)
	}
	return out
}

func stringNode(values []string) *yaml.Node {
	out := &yaml.Node{Kind: yaml.SequenceNode}
	for _, v := range values {
		out.Content = append(out.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: v})
	}
	return out
}
