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

// The upsert and hoist keys of behavior-overrides.yaml are Terraform keys. The iac module reads
// the contract as it is, so the generator gives it the contract that the keys describe:
//
//   - api.upsert: the singleton path gets a PUT that is a copy of the Set POST, and a DELETE with
//     the responses of the Get. The model then sees a singleton with every lifecycle step. Their
//     operationIds end in upsertPutSuffix and upsertDeleteSuffix, and upsertSDKNames maps their
//     SDK names to the Set and the Get. The renderer calls Set for Create and Update, and makes no
//     Delete call.
//   - hoist: each hoisted object of the resource component and of the Set body is replaced by
//     its fields. The model then sees them as top-level fields. File.Hoisted records the object
//     that holds each one, and hoistSDKNames moves their SDK names to the object's type.

// The operationIds of the PUT and the DELETE that reshape adds for an upsert.
const (
	upsertPutSuffix    = "_UpsertPut"
	upsertDeleteSuffix = "_UpsertDelete"
)

// upsertLocation is the line of the behavior-overrides file that names the upsert operations.
const upsertLocation = "behavior-overrides.yaml:api.upsert"

// reshape returns the contract that the model reads for a resource with upsert or hoist lines,
// and the operation ids that select its lifecycle. It sets file.Hoisted. A resource without those
// lines gets data and ids unchanged.
func reshape(data []byte, component string, ids model.OperationIDs, file *overrides.File) ([]byte, model.OperationIDs, issue.Report) {
	hoisted := file.HoistedFields()
	if file == nil || !file.Upserts() && len(hoisted) == 0 {
		return data, ids, nil
	}
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil || len(root.Content) == 0 {
		return data, ids, issue.Report{{Code: "OPENAPI_INVALID", Location: "openapi", Message: fmt.Sprintf("read the contract: %v", err), Remediation: "Correct the source OpenAPI document."}}
	}
	doc := root.Content[0]
	if !file.Upserts() {
		return data, ids, issue.Report{hoistIssue(component, "hoist needs api.upsert: its Set call sends the whole resource. An update mask names API fields, and a hoisted field is inside its object in the API")}
	}
	if ids != (model.OperationIDs{}) {
		return data, ids, issue.Report{{Code: "UPSERT_OPERATION_CONFLICT", Location: upsertLocation, Message: "the overrides name the operations of the upsert, so no operation flag can name one", Remediation: "Name the operations of the upsert only in api.upsert."}}
	}
	set, report := upsertOperations(doc, file.API.Upsert)
	if len(report) != 0 {
		return data, ids, report
	}
	shape := &overrides.Hoisted{}
	if len(hoisted) != 0 {
		schemas := mapValue(mapValue(doc, "components"), "schemas")
		response := mapValue(schemas, component)
		if response == nil {
			return data, ids, issue.Report{hoistIssue(component, "the contract has no component "+component)}
		}
		h := hoister{schemas: schemas, fields: hoisted, resource: component, file: file}
		var err error
		if shape.Get, err = h.hoist(response, true); err != nil {
			return data, ids, issue.Report{hoistIssue(component, component+": "+err.Error())}
		}
		body := mapValue(mapValue(mapValue(mapValue(set, "requestBody"), "content"), "application/json"), "schema")
		if ref := refName(body); ref != "" {
			body = mapValue(schemas, ref)
		}
		if body != nil && body != response {
			if shape.Create, err = h.hoist(body, false); err != nil {
				return data, ids, issue.Report{hoistIssue(component, "create body: "+err.Error())}
			}
		} else if body == response {
			shape.Create = shape.Get
		}
		shape.Update = shape.Create
	}
	out, err := yaml.Marshal(&root)
	if err != nil {
		return data, ids, issue.Report{{Code: "OPENAPI_INVALID", Location: "openapi", Message: fmt.Sprintf("write the contract: %v", err), Remediation: "Correct the source OpenAPI document."}}
	}
	file.Hoisted = shape
	u := file.API.Upsert
	return out, model.OperationIDs{Create: u.Set, Get: u.Get, Update: u.Set + upsertPutSuffix, Delete: u.Get + upsertDeleteSuffix}, nil
}

// upsertOperations checks the two calls of an upsert, a GET and a POST on one path without path
// parameters, and adds the PUT and the DELETE that the model needs. It returns the Set operation.
func upsertOperations(doc *yaml.Node, u *overrides.Upsert) (*yaml.Node, issue.Report) {
	getPath, getMethod, get := findOperation(doc, u.Get)
	setPath, setMethod, set := findOperation(doc, u.Set)
	var report issue.Report
	switch {
	case get == nil:
		report = append(report, upsertShapeIssue(".get", fmt.Sprintf("The contract has no operation %s.", u.Get)))
	case getMethod != "get" || strings.Contains(getPath, "{"):
		report = append(report, upsertShapeIssue(".get", fmt.Sprintf("%s is %s %s. An upsert is a singleton: its Get is a GET with no path parameters.", u.Get, strings.ToUpper(getMethod), getPath)))
	}
	switch {
	case set == nil:
		report = append(report, upsertShapeIssue(".set", fmt.Sprintf("The contract has no operation %s.", u.Set)))
	case get != nil && (setMethod != "post" || setPath != getPath):
		report = append(report, upsertShapeIssue(".set", fmt.Sprintf("%s is %s %s. The Set of an upsert is a POST on the Get path %s, with no path parameters.", u.Set, strings.ToUpper(setMethod), setPath, getPath)))
	}
	if len(report) != 0 {
		return nil, report
	}
	item := mapValue(mapValue(doc, "paths"), getPath)
	for _, method := range []string{"put", "patch", "delete"} {
		if mapValue(item, method) != nil {
			return nil, issue.Report{upsertShapeIssue("", fmt.Sprintf("%s also has a %s operation, so the API has more than the two calls of an upsert.", getPath, strings.ToUpper(method)))}
		}
	}
	// The PUT is the Set call; the DELETE makes no call in the renderer.
	put := cloneMapping(set)
	setMap(put, "operationId", &yaml.Node{Kind: yaml.ScalarNode, Value: u.Set + upsertPutSuffix})
	setMap(item, "put", put)
	del := &yaml.Node{Kind: yaml.MappingNode}
	setMap(del, "operationId", &yaml.Node{Kind: yaml.ScalarNode, Value: u.Get + upsertDeleteSuffix})
	for _, key := range []string{"responses", "tags"} {
		if v := mapValue(get, key); v != nil {
			setMap(del, key, v)
		}
	}
	setMap(item, "delete", del)
	return set, nil
}

func upsertShapeIssue(suffix, message string) issue.Issue {
	return issue.Issue{Code: "UPSERT_SHAPE_UNSUPPORTED", Location: upsertLocation + suffix, Message: message, Remediation: "Name the GET of a singleton and the POST on its path, or use the Create, Update, and Delete of the API."}
}

func hoistIssue(component, message string) issue.Issue {
	return issue.Issue{Code: "HOIST_UNSUPPORTED", Location: "components.schemas." + component, Message: message, Remediation: "Hoist a required object, or an arm of a oneOf group whose other arms the file skips, that has no field name of its parent."}
}

// hoister replaces the hoisted objects of a schema by their fields.
type hoister struct {
	schemas  *yaml.Node
	fields   []string // the hoisted fields of the resource
	resource string
	file     *overrides.File
}

// hoist replaces each hoisted object of s by its fields, in place and in order. A hoisted object
// must be sent in every request that has it, so that its required fields are sent: it must be
// required there. In the response it is required, or an arm of a oneOf group whose other arms the
// file skips: an absent arm then means that the resource does not exist. The skipped arms stay as
// plain fields, which their skip lines name. It returns the object that holds each moved field.
func (h hoister) hoist(s *yaml.Node, response bool) (map[string]overrides.Parent, error) {
	props := mapValue(s, "properties")
	if props == nil || !slices.ContainsFunc(h.fields, func(f string) bool { return mapValue(props, f) != nil }) {
		return nil, nil
	}
	if mapValue(s, "allOf") != nil || mapValue(s, "anyOf") != nil {
		return nil, fmt.Errorf("hoist: an object with allOf or anyOf is not supported")
	}
	required := stringList(mapValue(s, "required"))
	arm, err := h.hoistedArm(s, response)
	if err != nil {
		return nil, err
	}
	parents := map[string]overrides.Parent{}
	flat := &yaml.Node{Kind: yaml.MappingNode}
	var flatRequired []string
	add := func(key, value *yaml.Node, isRequired bool) error {
		if mapValue(flat, key.Value) != nil {
			return fmt.Errorf("hoist: two fields have the name %q", key.Value)
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
			return nil, fmt.Errorf("hoist: %s must be a $ref to an object component", name)
		}
		objectProps := mapValue(object, "properties")
		if objectProps == nil || len(objectProps.Content) == 0 || mapValue(object, "oneOf") != nil || mapValue(object, "allOf") != nil || mapValue(object, "anyOf") != nil {
			return nil, fmt.Errorf("hoist: %s must be an object with fields and no oneOf, allOf, or anyOf", name)
		}
		isArm := name == arm
		if !isArm && !slices.Contains(required, name) {
			return nil, fmt.Errorf("hoist: %s must be required, so that its fields are always in the API value", name)
		}
		objectRequired := stringList(mapValue(object, "required"))
		for j := 0; j+1 < len(objectProps.Content); j += 2 {
			child := objectProps.Content[j].Value
			if slices.Contains(h.fields, child) {
				return nil, fmt.Errorf("hoist: %s.%s is hoisted too, but only a field of the resource can be", name, child)
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
		deleteMap(s, "oneOf") // the group is the hoisted arm; its other arms are skipped fields
	}
	return parents, nil
}

// hoistedArm returns the hoisted field that is an arm of the oneOf group of s, or "". The other
// arms must be skipped, so that the group means that the hoisted arm is set, or that the resource
// does not exist. A request cannot have a hoisted arm: the request must always send the object.
func (h hoister) hoistedArm(s *yaml.Node, response bool) (string, error) {
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
		return "", fmt.Errorf("hoist: %s is an arm of a oneOf group of a request, so the request does not always send it", arm)
	}
	for _, other := range arms {
		if other != arm && !h.file.Types[h.resource].Fields[other].Skip {
			return "", fmt.Errorf("hoist: %s is an arm of a oneOf group; skip the other arm %s, or the resource could be that arm", arm, other)
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
	u := file.API.Upsert
	names := strings.NewReplacer(
		camelize(u.Set+upsertPutSuffix), camelize(u.Set),
		camelize(u.Get+upsertDeleteSuffix), camelize(u.Get))
	out := slices.Clone(refs)
	for i := range out {
		out[i].Name, out[i].Owner = names.Replace(out[i].Name), names.Replace(out[i].Owner)
		out[i].Want, out[i].WantValue = names.Replace(out[i].Want), names.Replace(out[i].WantValue)
	}
	return out
}

// hoistSDKNames moves the SDK names of the hoisted fields to the type of the object that holds
// them, and adds the object: its field of the root type at <root>.<object>, and its type. The
// model gave them to the root type, because it reads the reshaped contract.
func hoistSDKNames(refs []sdkcheck.Ref, file *overrides.File) []sdkcheck.Ref {
	if file == nil || file.Hoisted == nil {
		return refs
	}
	roots := []struct {
		path    string
		parents map[string]overrides.Parent
	}{{"fields", file.Hoisted.Get}, {"create.body", file.Hoisted.Create}, {"update.body", file.Hoisted.Update}}
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
			parent, hoisted := root.parents[name]
			if !ok || !hoisted || ref.Kind != sdkcheck.KindField || ref.Owner != rootType {
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
