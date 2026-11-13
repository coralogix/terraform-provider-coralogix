package generator

import (
	"fmt"
	"maps"
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
	nested := file.NestedInlined()
	if file == nil || !file.Upserts() && len(inlined) == 0 && len(nested) == 0 && len(file.PlaceClones) == 0 {
		return data, ids, nil
	}
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil || len(root.Content) == 0 {
		return data, ids, issue.Report{{Code: "OPENAPI_INVALID", Location: "openapi", Message: fmt.Sprintf("read the contract: %v", err), Remediation: "Correct the source OpenAPI document."}}
	}
	doc := root.Content[0]
	schemas := mapValue(mapValue(doc, "components"), "schemas")
	h := inliner{schemas: schemas, fields: inlined, resource: component, file: file, done: map[*yaml.Node]map[string]overrides.Parent{}, relied: map[*yaml.Node][]string{}}
	shape := &overrides.Inlined{}
	if report := h.placeClones(); len(report) != 0 {
		return data, ids, report
	}
	if len(inlined) == 0 && len(nested) == 0 && !file.Upserts() {
		// Only a place copy changed the contract: no field is inlined, so the bodies keep their shape.
		written, err := yaml.Marshal(&root)
		if err != nil {
			return data, ids, issue.Report{{Code: "OPENAPI_INVALID", Location: "openapi", Message: fmt.Sprintf("write the contract: %v", err), Remediation: "Correct the source OpenAPI document."}}
		}
		file.Inlined = shape
		return written, ids, nil
	}
	var needed []*yaml.Node // the schemas of the resource that an inline line with required: true can be needed in
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
		needed = append(needed, response)
	}
	if len(nested) != 0 {
		var report issue.Report
		if shape.Nested, report = h.inlineNested(doc, nested); len(report) != 0 {
			return data, ids, report
		}
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
			needed = append(needed, bodySchema(h.schemas, set))
		}
		out = model.OperationIDs{Create: ops.Create, Get: ops.Get, Update: ops.Create + upsertPutSuffix, Delete: ops.Get + upsertDeleteSuffix}
	}
	written, err := yaml.Marshal(&root)
	if err != nil {
		return data, ids, issue.Report{{Code: "OPENAPI_INVALID", Location: "openapi", Message: fmt.Sprintf("write the contract: %v", err), Remediation: "Correct the source OpenAPI document."}}
	}
	if !file.Upserts() {
		bodies, report := h.inlineCreateAndUpdate(&root, component, ids, policy, shape)
		if len(report) != 0 {
			return data, ids, report
		}
		needed = append(needed, bodies...)
		if written, err = yaml.Marshal(&root); err != nil {
			return data, ids, issue.Report{{Code: "OPENAPI_INVALID", Location: "openapi", Message: fmt.Sprintf("write the contract: %v", err), Remediation: "Correct the source OpenAPI document."}}
		}
	}
	if report := h.unusedRequired(inlined, needed); len(report) != 0 {
		return data, ids, report
	}
	file.Inlined = shape
	return written, out, nil
}

// inlineCreateAndUpdate inlines the bodies of a resource without an upsert. The iac module finds
// the operations, but only in a contract whose bodies are inlined already, because a request field
// can sit at another depth than the response field before. So every request body with an inlined
// field is inlined; the Create and Update bodies that the model then selects give the parents, and
// an error counts only for them. The Update must send the whole resource.
func (h inliner) inlineCreateAndUpdate(root *yaml.Node, component string, ids model.OperationIDs, policy model.Policy, shape *overrides.Inlined) ([]*yaml.Node, issue.Report) {
	parents, errs, nodes := map[string]map[string]overrides.Parent{}, map[string]error{}, map[string]*yaml.Node{}
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
			nodes[id.Value] = bodySchema(h.schemas, op)
		}
	}
	data, err := yaml.Marshal(root)
	if err != nil {
		return nil, issue.Report{inlineIssue(component, err.Error())}
	}
	doc, err := model.Load(data)
	if err != nil {
		return nil, issue.Report{inlineIssue(component, err.Error())}
	}
	r, err := model.BuildWithPolicy(doc, component, ids, policy)
	if err != nil {
		return nil, issue.Report{inlineIssue(component, "find the Create and Update operations of "+component+": "+err.Error())}
	}
	for _, op := range []struct{ name, id string }{{"create body", r.Create.OperationID}, {"update body", r.Update.OperationID}} {
		if errs[op.id] != nil {
			return nil, issue.Report{inlineIssue(component, op.name+": "+errs[op.id].Error())}
		}
	}
	if !r.Replace && len(parents[r.Update.OperationID]) != 0 {
		return nil, issue.Report{inlineIssue(component, fmt.Sprintf("inline needs an Update that sends the whole resource, a PUT or the Set call of an upsert, but %s is %s with an update mask: the mask names API fields, and an inlined field is inside its object in the API", r.Update.OperationID, r.Update.Method))}
	}
	shape.Create, shape.Update = parents[r.Create.OperationID], parents[r.Update.OperationID]
	return []*yaml.Node{nodes[r.Create.OperationID], nodes[r.Update.OperationID]}, nil
}

// body inlines the request body of op, once per schema: a body that is a component that another
// place inlined already gets the same parents.
func (h inliner) body(op *yaml.Node) (map[string]overrides.Parent, error) {
	schema := bodySchema(h.schemas, op)
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

// bodySchema returns the schema of the request body of op: the component that it refers to, or the
// inline schema.
func bodySchema(schemas, op *yaml.Node) *yaml.Node {
	schema := mapValue(mapValue(mapValue(mapValue(op, "requestBody"), "content"), "application/json"), "schema")
	if ref := refName(schema); ref != "" {
		return mapValue(schemas, ref)
	}
	return schema
}

// unusedRequired reports an inline line of the resource with required: true that none of the
// schemas needs: each of them requires the object, so the contract states what the line says.
func (h inliner) unusedRequired(inlined []string, schemas []*yaml.Node) issue.Report {
	var report issue.Report
	for _, field := range inlined {
		if !h.file.Types[h.resource].Fields[field].Required || h.reliesOn(schemas, field) {
			continue
		}
		report = append(report, issue.Issue{Code: "OVERRIDE_UNUSED", Location: overrides.FileName + ":types." + h.resource + ".fields." + field, Message: fmt.Sprintf("the contract already requires %q in the response and in the Create and Update bodies, so required: true on its inline line says nothing", field), Remediation: "Delete only the key required from the inline line."})
	}
	return report
}

// unusedNestedRequired is unusedRequired for the inline lines of nested components. relied has
// component.field of each line that some schema needs required: true for.
func (h inliner) unusedNestedRequired(nested map[string][]string, relied map[string]bool) issue.Report {
	var report issue.Report
	for _, component := range slices.Sorted(maps.Keys(nested)) {
		for _, field := range nested[component] {
			if h.file.Types[component].Fields[field].Required && !relied[component+"."+field] {
				report = append(report, issue.Issue{Code: "OVERRIDE_UNUSED", Location: overrides.FileName + ":types." + component + ".fields." + field, Message: fmt.Sprintf("no schema needs required: true on the inline line of %q, so it says nothing: every schema that holds the object requires it, the object is an arm that stays beside other arms, or only a response holds the component", field), Remediation: "Delete only the key required from the inline line."})
			}
		}
	}
	return report
}

// reliesOn reports whether one of schemas does not require the inlined field, so that the line
// of the field needs required: true there.
func (h inliner) reliesOn(schemas []*yaml.Node, field string) bool {
	return slices.ContainsFunc(schemas, func(s *yaml.Node) bool { return slices.Contains(h.relied[s], field) })
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
	if mapValue(item, "delete") != nil {
		return nil, issue.Report{upsertShapeIssue("", fmt.Sprintf("%s also has a DELETE operation, so the API can remove the singleton.", getPath))}
	}
	// The POST creates and replaces the singleton, so another write on its path is not used.
	deleteMap(item, "put")
	deleteMap(item, "patch")
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
	done    map[*yaml.Node]map[string]overrides.Parent
	schemas *yaml.Node
	fields  []string // the inlined fields of the resource
	// resource is the component whose field lines state the inline lines: the resource component,
	// or a nested one.
	resource string
	file     *overrides.File
	// relied has, for each schema that inline changed, the inlined fields that the schema does not
	// require and that the inline line states as required: the line is needed there.
	relied map[*yaml.Node][]string
	// nested: the line is on a nested component. Its object can be optional, and an arm can be
	// inlined while the other arms stay.
	nested bool
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
	arm, kept, err := h.inlinedArm(s, response)
	if err != nil {
		return nil, err
	}
	parents := map[string]overrides.Parent{}
	flat := &yaml.Node{Kind: yaml.MappingNode}
	var flatRequired []string
	var carried *yaml.Node        // the oneOf of an inlined object
	var typedObjects []*yaml.Node // the properties of each inlined object that has a typedArms line
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
		if objectProps == nil || len(objectProps.Content) == 0 || mapValue(object, "allOf") != nil || mapValue(object, "anyOf") != nil {
			return nil, fmt.Errorf("inline: %s must be an object with fields and no allOf or anyOf", name)
		}
		if carried, err = carryOneOf(s, object, name, carried); err != nil {
			return nil, err
		}
		if typedObjects, err = h.typedObject(typedObjects, name, component, objectProps); err != nil {
			return nil, err
		}
		isArm := name == arm
		armOnly := isArm && len(kept) == 0
		optional := h.optionalObject(s, name, isArm, kept, required)
		if !isArm && !slices.Contains(required, name) && !optional {
			if err := h.requireObject(s, name); err != nil {
				return nil, err
			}
		}
		objectRequired := stringList(mapValue(object, "required"))
		if optional && len(objectRequired) != 0 {
			return nil, fmt.Errorf("inline: %s is optional, so its fields must be optional too, but it requires %s", name, strings.Join(objectRequired, ", "))
		}
		// A field of the object has the name of the object: every field of it moves out of an object
		// whose reference needs a path of its own.
		same := mapValue(objectProps, name) != nil
		for j := 0; j+1 < len(objectProps.Content); j += 2 {
			child := objectProps.Content[j].Value
			// The inlined field leaves its place, so on a nested component a field of its object can
			// take its name.
			if slices.Contains(h.fields, child) && !(h.nested && child == name) {
				return nil, fmt.Errorf("inline: %s.%s is inlined too, but only a field of the resource can be", name, child)
			}
			if err := add(objectProps.Content[j], objectProps.Content[j+1], slices.Contains(objectRequired, child)); err != nil {
				return nil, err
			}
			parents[child] = overrides.Parent{Field: name, Schema: component, Arm: armOnly, Kept: strings.Join(kept, ","), Same: same, Always: h.alwaysSent(name, armOnly, response)}
		}
	}
	if err := h.checkTypedFields(typedObjects, flat); err != nil {
		return nil, err
	}
	setMap(s, "properties", flat)
	setMap(s, "required", stringNode(flatRequired))
	if arm != "" {
		deleteMap(s, "oneOf") // the group is the inlined arm; its other arms are skipped fields
	}
	if carried != nil {
		setMap(s, "oneOf", carried)
	}
	return parents, nil
}

// typedObject adds the properties of an inlined object that has a typedArms line to typed, and
// checks the object. Its fields keep the line: the component that receives them shows them as that
// object does (File.TypedArmsOf). A list has no fields to move, and the resource has no line of its
// own that could take the arms.
func (h inliner) typedObject(typed []*yaml.Node, name, object string, props *yaml.Node) ([]*yaml.Node, error) {
	line := h.file.Types[object].TypedArms
	switch {
	case line == nil:
		return typed, nil
	case line.AsList:
		return nil, fmt.Errorf("inline: %s has typedArms.asList, which shows a list: no fields of an object are left to inline", name)
	case !h.nested:
		return nil, fmt.Errorf("inline: %s has a typedArms line, which only a nested component takes with its fields; put the line on the resource component to show the arms as one object", name)
	}
	return append(typed, props), nil
}

// checkTypedFields checks the component that receives the fields of inlined objects with a
// typedArms line. It takes that line when it has none of its own. The line shows every field of the
// component as an arm, so the fields must be those of one object: another field, or a second
// object, would be an arm too.
func (h inliner) checkTypedFields(typed []*yaml.Node, flat *yaml.Node) error {
	if len(typed) == 0 || h.file.Types[h.resource].TypedArms != nil {
		return nil
	}
	if len(typed) > 1 {
		return fmt.Errorf("inline: more than one inlined object has a typedArms line, so the line of the component is unclear; put a typedArms line on %s", h.resource)
	}
	if h.keptFields(flat) != len(typed[0].Content) {
		return fmt.Errorf("inline: the fields of an object with a typedArms line are arms in %s, but %s has other fields; put a typedArms line on %s to say what it shows (a field that its line skips does not count)", h.resource, h.resource, h.resource)
	}
	return nil
}

// keptFields counts the entries (names and schemas) of the properties of the component that its
// lines do not skip.
func (h inliner) keptFields(props *yaml.Node) int {
	lines := h.file.Types[h.resource].Fields
	kept := 0
	for i := 0; i+1 < len(props.Content); i += 2 {
		if line, ok := lines[props.Content[i].Value]; !ok || !line.Skip {
			kept += 2
		}
	}
	return kept
}

// carryOneOf returns the oneOf group that the fields of the inlined object bring to s: the group of
// the object, whose arms are its fields. s must not have a group of its own, and no other inlined
// object may have brought one (carried).
func carryOneOf(s, object *yaml.Node, name string, carried *yaml.Node) (*yaml.Node, error) {
	oneOf := mapValue(object, "oneOf")
	if oneOf == nil {
		return carried, nil
	}
	if carried != nil || mapValue(s, "oneOf") != nil {
		return nil, fmt.Errorf("inline: %s has a oneOf, and its arms become the arms of the oneOf group of the schema that receives its fields, but that schema has a group already: its own, or one that another inlined object brought", name)
	}
	return oneOf, nil
}

// optionalObject reports whether the inlined object name of s is optional. On a nested component an
// object that the schema does not require and that the line does not state as required is optional:
// a request sends it only when a field of it is set, and a response without it reads as nulls. An
// arm that stays beside other arms is optional too.
func (h inliner) optionalObject(s *yaml.Node, name string, isArm bool, kept, required []string) bool {
	if !h.nested || slices.Contains(required, name) {
		return false
	}
	if isArm {
		return len(kept) != 0
	}
	return !h.file.Types[h.resource].Fields[name].Required
}

// alwaysSent reports whether a request always sends the inlined object name: it is an arm that is
// the object (the other arms are skipped) of a nested component that a request holds, and the line
// says that the API always has it.
func (h inliner) alwaysSent(name string, armOnly, response bool) bool {
	return h.nested && armOnly && !response && h.file.Types[h.resource].Fields[name].Required
}

// requireObject checks an inlined object that s does not require. The line states that the API
// always has it, when the contract cannot say so, and the line is then needed for s.
func (h inliner) requireObject(s *yaml.Node, name string) error {
	if !h.file.Types[h.resource].Fields[name].Required {
		return fmt.Errorf("inline: %s must be required, so that its fields are always in the API value; if the API always has it but the contract has no required list that can say so, add required: true to the inline line of %s", name, name)
	}
	h.relied[s] = append(h.relied[s], name)
	return nil
}

// inlinedArm returns the inlined field that is an arm of the oneOf group of s, or "". The other
// arms must be skipped, so that the group means that the inlined arm is set, or that the resource
// does not exist. A request cannot have an inlined arm of the resource: the request must always send
// the object. On a nested component, a line with required: true says that the API always has the
// arm, so a request always sends it.
func (h inliner) inlinedArm(s *yaml.Node, response bool) (string, []string, error) {
	group := mapValue(s, "oneOf")
	if group == nil {
		return "", nil, nil
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
		return "", nil, nil
	}
	var kept []string
	for _, other := range arms {
		if other != arm && !h.file.Types[h.resource].Fields[other].Skip {
			kept = append(kept, other)
		}
	}
	if h.nested && len(kept) != 0 {
		// The other arms stay. The fields of the arm join the object, and conflict with them. The
		// group goes away, and the conflicts do not say that an arm is set, so the group must allow
		// that none is.
		if !slices.ContainsFunc(group.Content, func(entry *yaml.Node) bool { return mapValue(entry, "not") != nil }) {
			return "", nil, fmt.Errorf("inline: %s is an arm of a oneOf group that requires one arm, and the group goes away when %s is inlined beside %s, so a request could send no arm; the generator cannot keep that rule", arm, arm, strings.Join(kept, ", "))
		}
		return arm, kept, nil
	}
	if !response {
		if !h.nested {
			return "", nil, fmt.Errorf("inline: %s is an arm of a oneOf group of a request, so the request does not always send it", arm)
		}
		if !h.file.Types[h.resource].Fields[arm].Required {
			return "", nil, fmt.Errorf("inline: %s is an arm of a oneOf group of a request, so the request does not always send it; if the API always has it, add required: true to the inline line of %s", arm, arm)
		}
		// The line says that the API always has the arm: a request always sends it.
		h.relied[s] = append(h.relied[s], arm)
		return arm, nil, nil
	}
	if len(kept) != 0 {
		return "", nil, fmt.Errorf("inline: %s is an arm of a oneOf group; skip the other arm %s, or the resource could be that arm", arm, kept[0])
	}
	return arm, nil, nil
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
				sdkcheck.Ref{Path: path, Kind: sdkcheck.KindType, Pkg: pkg, Name: typ, Rule: sdkcheck.RuleComponent})
		}
	}
	return inlineNestedSDKNames(out, file)
}

// inlineNestedSDKNames moves the SDK names of the fields that an inline line of a nested component
// moved to the type of the object that holds them, and adds that object: its field of the
// component, next to the moved field, and its type. When the object has an inline line of its own
// for the field, the field moves on, to the type of the inner object, which the outer one holds.
func inlineNestedSDKNames(refs []sdkcheck.Ref, file *overrides.File) []sdkcheck.Ref {
	if len(file.Inlined.Nested) == 0 {
		return refs
	}
	out := slices.Clone(refs)
	added := map[string]bool{}
	for i, ref := range refs {
		if ref.Kind != sdkcheck.KindField {
			continue
		}
		at := strings.LastIndex(ref.Path, ".")
		if at < 0 {
			continue
		}
		field := ref.Path[at+1:]
		for schema := range file.Inlined.Nested {
			if ref.Owner != model.GoName(schema) {
				continue
			}
			prefix, owner := ref.Path[:at], ref.Owner
			for _, parent := range file.NestedParents(schema, field) {
				typ := model.GoName(parent.Schema)
				out[i].Owner = typ
				plain := prefix + "." + parent.Field
				path := parent.ObjectPath(plain)
				prefix, schema = plain, parent.Schema
				if added[path] {
					owner = typ
					continue
				}
				added[path] = true
				// A required object can be a value in the SDK. An arm of a oneOf is a pointer.
				out = append(out,
					sdkcheck.Ref{Path: path, Kind: sdkcheck.KindField, Pkg: ref.Pkg, Owner: owner, Name: goFieldName(parent.Field), Want: "*" + typ, WantValue: typ, Rule: sdkcheck.RuleProperty},
					sdkcheck.Ref{Path: path, Kind: sdkcheck.KindType, Pkg: ref.Pkg, Name: typ, Rule: sdkcheck.RuleComponent})
				owner = typ
			}
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

// inlineNested inlines the object fields of nested components that inline lines name. A line on
// field F of component C inlines F in C, and in each component that holds the same object under F
// and that the resource component or the request bodies of the operations in api.operations reach:
// the request item of C. It returns the object that holds each moved field, by component.
func (h inliner) inlineNested(doc *yaml.Node, nested map[string][]string) (map[string]map[string]overrides.Parent, issue.Report) {
	ops := h.file.API.Operations
	if ops == nil || ops.Create == "" || ops.Update == "" {
		return nil, issue.Report{inlineIssue(h.resource, "an inline line on a nested component needs api.operations, which name the requests that hold the object too")}
	}
	// The object component that each inlined field holds: component, then field.
	objects := map[string]map[string]string{}
	for _, component := range slices.Sorted(maps.Keys(nested)) {
		schema := mapValue(h.schemas, component)
		if schema == nil {
			return nil, issue.Report{inlineIssue(component, "the contract has no component "+component)}
		}
		objects[component] = map[string]string{}
		for _, field := range nested[component] {
			value := mapValue(mapValue(schema, "properties"), field)
			object := refName(value)
			if object == "" {
				object = allOfRef(value)
			}
			if object == "" {
				return nil, issue.Report{inlineIssue(component, fmt.Sprintf("inline: %s.%s must be a $ref to an object component", component, field))}
			}
			objects[component][field] = object
		}
	}
	responses := h.reachable(mapValue(h.schemas, h.resource), map[string]bool{})
	requests := map[string]bool{}
	for _, id := range []string{ops.Create, ops.Update} {
		_, method, op := findOperation(doc, id)
		if op == nil {
			continue // the model reports the operation
		}
		if method == "patch" {
			return nil, issue.Report{inlineIssue(h.resource, fmt.Sprintf("an inlined nested field is inside its object in the API, so the Update must send the whole resource, but %s is a PATCH", id))}
		}
		h.reachable(mapValue(mapValue(mapValue(mapValue(op, "requestBody"), "content"), "application/json"), "schema"), requests)
	}
	out := map[string]map[string]overrides.Parent{}
	relied := map[string]bool{}         // component.field of each inline line that a schema needs required: true for
	counterparts := map[string]string{} // the request counterpart of each component
	for _, name := range h.holdersLast(slices.Sorted(maps.Keys(mergeSets(responses, requests)))) {
		schema := mapValue(h.schemas, name)
		props := mapValue(schema, "properties")
		for _, component := range slices.Sorted(maps.Keys(objects)) {
			// A line on C.f inlines f in C, and in a component that only a request holds, the request
			// counterpart of C. Another component of the response that has the same field keeps it.
			if name != component && !h.isCounterpart(component, responses[name], requests[name]) {
				continue
			}
			var fields []string
			for field, object := range objects[component] {
				if refName(mapValue(props, field)) == object || allOfRef(mapValue(props, field)) == object {
					fields = append(fields, field)
				}
			}
			if len(fields) == 0 {
				continue
			}
			if name != component {
				// Only the line names C, so two request components that hold the object leave its
				// counterpart unclear: one of them may not be C at all.
				if other, ok := counterparts[component]; ok {
					return nil, issue.Report{inlineIssue(component, fmt.Sprintf("inline: the requests hold the inlined object of %s in both %s and %s, so its request counterpart is unclear", component, other, name))}
				}
				counterparts[component] = name
			}
			slices.Sort(fields)
			nestedInliner := inliner{schemas: h.schemas, fields: fields, resource: component, file: h.file, done: h.done, relied: h.relied, nested: true}
			parents, err := nestedInliner.inline(schema, responses[name] && !requests[name])
			if err != nil {
				return nil, issue.Report{inlineIssue(name, name+": "+err.Error())}
			}
			for _, field := range fields {
				if slices.Contains(h.relied[schema], field) {
					relied[component+"."+field] = true
				}
			}
			if out[name] == nil {
				out[name] = map[string]overrides.Parent{}
			}
			maps.Copy(out[name], parents)
		}
	}
	for _, component := range slices.Sorted(maps.Keys(nested)) {
		if len(out[component]) == 0 {
			return nil, issue.Report{h.unheldIssue(component)}
		}
	}
	if report := h.unusedNestedRequired(nested, relied); len(report) != 0 {
		return nil, report
	}
	return out, nil
}

// holdersLast orders names so that a schema comes after the schemas of names that its fields refer
// to, otherwise in the order of names. An inline line of a component takes the fields of its object
// as the object has them: when the object has inline lines too, they must be done first, so that
// the fields move up through both (a chain of lines).
func (h inliner) holdersLast(names []string) []string {
	known := map[string]bool{}
	for _, name := range names {
		known[name] = true
	}
	var out []string
	seen := map[string]bool{}
	var visit func(name string)
	visit = func(name string) {
		if seen[name] {
			return
		}
		seen[name] = true
		props := mapValue(mapValue(h.schemas, name), "properties")
		for i := 1; props != nil && i < len(props.Content); i += 2 {
			ref := refName(props.Content[i])
			if ref == "" {
				ref = allOfRef(props.Content[i])
			}
			if known[ref] {
				visit(ref)
			}
		}
		out = append(out, name)
	}
	for _, name := range names {
		visit(name)
	}
	return out
}

// isCounterpart reports whether a schema that is not component can be its request counterpart: only
// a request holds it. The copy of a component at a place has none.
func (h inliner) isCounterpart(component string, inResponse, inRequest bool) bool {
	return !inResponse && inRequest && !h.file.IsPlaceClone(component)
}

// unheldIssue reports an inline line, or an at line that inlines, of a component that the resource
// does not hold.
func (h inliner) unheldIssue(component string) issue.Issue {
	if c := h.file.PlaceCloneOf(component); c != nil {
		return inlineIssue(c.Component, "the resource does not hold "+c.Holder+", so the at line of "+c.Component+" changes nothing")
	}
	return inlineIssue(component, "the resource does not hold "+component+", so its inline lines change nothing")
}

// reachable adds the components that s refers to, directly or through other components, to seen.
func (h inliner) reachable(s *yaml.Node, seen map[string]bool) map[string]bool {
	if s == nil {
		return seen
	}
	if name := refName(s); name != "" {
		if seen[name] {
			return seen
		}
		seen[name] = true
		return h.reachable(mapValue(h.schemas, name), seen)
	}
	for _, child := range s.Content {
		h.reachable(child, seen)
	}
	return seen
}

// allOfRef is the component of an allOf with one $ref, or "".
func allOfRef(value *yaml.Node) string {
	if all := mapValue(value, "allOf"); all != nil && len(all.Content) == 1 {
		return refName(all.Content[0])
	}
	return ""
}

func mergeSets(a, b map[string]bool) map[string]bool {
	out := maps.Clone(a)
	maps.Copy(out, b)
	return out
}
