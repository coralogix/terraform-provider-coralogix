package generator

import (
	"errors"
	"fmt"
	"strings"

	"github.com/coralogix/coralogix-management-sdk/iac/issue"
	"github.com/coralogix/coralogix-management-sdk/iac/model"
	"github.com/coralogix/coralogix-management-sdk/iac/sdkcheck"
	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/overrides"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"go.yaml.in/yaml/v4"
)

// A released API can answer Create with only the id of the new resource, and Update with an object
// that has no fields. The generated code then reads the resource with Get. Only Terraform reads
// the responses like that, so the SDK iac model does not know the shapes: before the model reads
// the contract, the response component gets one extra property that holds the resource, and after
// the build the model is set back to the shape that the API has.

// readBackProperty is the property that the generator adds to a response component. It is no API
// field, and it never reaches the generated code.
const readBackProperty = "tfgenReadBack"

// readBackShapes are the response components of the lifecycle that the generated code reads back.
type readBackShapes struct {
	createSchema string // the Create response, when it holds only the id
	createID     string // its property that holds the id
	updateSchema string // the Update response, when it has no fields
}

func (s readBackShapes) any() bool { return s.createSchema != "" || s.updateSchema != "" }

// findReadBackShapes finds the responses to read back in a document that is not changed yet. The
// Create response needs the api.createResponseID line. An Update response of an existing resource
// with no fields needs no line.
func findReadBackShapes(doc *v3.Document, component string, ids model.OperationIDs, file *overrides.File) (readBackShapes, issue.Report) {
	var s readBackShapes
	if file == nil || doc.Components == nil || doc.Components.Schemas == nil {
		return s, nil
	}
	if key := file.API.CreateResponseID; key != "" {
		name := lifecycleResponse(doc, component, firstNonEmpty(ids.Create, operationID(file, "create")), "_Create")
		if name == "" {
			return s, readBackIssue("The Create operation has no 200 response that is a component, so api.createResponseID has nothing to read.")
		}
		schema := doc.Components.Schemas.GetOrZero(name).Schema()
		property := schema.Properties.GetOrZero(key)
		if property == nil {
			return s, readBackIssue(fmt.Sprintf("The Create response %s has no property %q (api.createResponseID).", name, key))
		}
		if types := property.Schema().Type; len(types) != 1 || types[0] != "string" {
			return s, readBackIssue(fmt.Sprintf("The Create response property %s.%s has type %v, want a string id.", name, key, types))
		}
		s.createSchema, s.createID = name, key
	}
	if name := lifecycleResponse(doc, component, firstNonEmpty(ids.Update, operationID(file, "update")), "_Update", "_Replace"); name != "" && name != component {
		if schema := doc.Components.Schemas.GetOrZero(name).Schema(); schema != nil && (schema.Properties == nil || schema.Properties.Len() == 0) {
			s.updateSchema = name
		}
	}
	return s, nil
}

func readBackIssue(message string) issue.Report {
	return issue.Report{{Code: "READ_BACK_UNSUPPORTED", Location: "paths", Message: message, Remediation: "Correct api.createResponseID, or the Create response of the source API contract."}}
}

// operationID is the operation id that the overrides file names for a lifecycle step, or "".
func operationID(file *overrides.File, step string) string {
	ops := file.API.Operations
	if ops == nil {
		return ""
	}
	if step == "create" {
		return ops.Create
	}
	return ops.Update
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// lifecycleResponse returns the component of the 200 response of the operation with the given
// operation id, or with an id that ends in one of the suffixes plus the resource component. It
// returns "" when there is none, or the response is not a component.
func lifecycleResponse(doc *v3.Document, component, operationID string, suffixes ...string) string {
	if doc.Paths == nil || doc.Paths.PathItems == nil {
		return ""
	}
	for _, item := range doc.Paths.PathItems.FromOldest() {
		for _, op := range item.GetOperations().FromOldest() {
			if op == nil || !matchesOperation(op.OperationId, operationID, component, suffixes) {
				continue
			}
			if op.Responses == nil || op.Responses.Codes == nil {
				return ""
			}
			ok := op.Responses.Codes.GetOrZero("200")
			if ok == nil || ok.Content == nil || ok.Content.GetOrZero("application/json") == nil {
				return ""
			}
			proxy := ok.Content.GetOrZero("application/json").Schema
			if proxy == nil || !proxy.IsReference() {
				return ""
			}
			name, found := strings.CutPrefix(proxy.GetReference(), "#/components/schemas/")
			if !found {
				return ""
			}
			return name
		}
	}
	return ""
}

func matchesOperation(id, explicit, component string, suffixes []string) bool {
	if explicit != "" {
		return id == explicit
	}
	for _, suffix := range suffixes {
		if strings.HasSuffix(id, suffix+component) {
			return true
		}
	}
	return false
}

// withReadBackProperty returns the document with the extra property in each response component to
// read back. The property refers to the resource, so the model sees a response that wraps it.
func withReadBackProperty(data []byte, component string, s readBackShapes) ([]byte, error) {
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, err
	}
	if len(root.Content) == 0 {
		return nil, errors.New("empty OpenAPI document")
	}
	for _, name := range []string{s.createSchema, s.updateSchema} {
		if name == "" {
			continue
		}
		schema := yamlPath(root.Content[0], "components", "schemas", name)
		if schema == nil || schema.Kind != yaml.MappingNode {
			return nil, fmt.Errorf("response component %s is not a mapping", name)
		}
		properties := yamlGet(schema, "properties")
		if properties == nil {
			schema.Content = append(schema.Content, yamlScalar("properties"), &yaml.Node{Kind: yaml.MappingNode})
			properties = schema.Content[len(schema.Content)-1]
		}
		reference := &yaml.Node{Kind: yaml.MappingNode, Content: []*yaml.Node{yamlScalar("$ref"), yamlScalar("#/components/schemas/" + component)}}
		properties.Content = append(properties.Content, yamlScalar(readBackProperty), reference)
	}
	return yaml.Marshal(&root)
}

func yamlScalar(v string) *yaml.Node { return &yaml.Node{Kind: yaml.ScalarNode, Value: v} }

func yamlGet(n *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			return n.Content[i+1]
		}
	}
	return nil
}

func yamlPath(n *yaml.Node, keys ...string) *yaml.Node {
	for _, key := range keys {
		if n = yamlGet(n, key); n == nil {
			return nil
		}
	}
	return n
}

// restore sets the model back to the shapes that the API has: a Create response that is a
// component with only the id, and an Update response with no fields.
func (s readBackShapes) restore(r *model.Resource) error {
	if s.createSchema != "" {
		if r.Create.Response.Field != readBackProperty {
			return fmt.Errorf("the Create response %s was not read as a response that wraps the resource", s.createSchema)
		}
		if r.IDType == nil || r.IDType.Kind != model.String {
			return errors.New("api.createResponseID needs a string id")
		}
		r.Create.Response = model.Response{Schema: s.createSchema}
	}
	if s.updateSchema != "" {
		if r.Update.Response.Field != readBackProperty {
			return fmt.Errorf("the Update response %s was not read as a response that wraps the resource", s.updateSchema)
		}
		r.Update.Response = model.Response{Schema: s.updateSchema, Empty: true}
	}
	return nil
}

// refs adds the SDK field that holds the id of the Create response.
func (s readBackShapes) refs(refs []sdkcheck.Ref) []sdkcheck.Ref {
	if s.createSchema == "" {
		return refs
	}
	for _, ref := range refs {
		if ref.Path == "create.response" && ref.Kind == sdkcheck.KindType {
			return append(refs, sdkcheck.Ref{Path: "create.response." + s.createID, Kind: sdkcheck.KindField, Pkg: ref.Pkg, Owner: ref.Name,
				Name: goFieldName(s.createID), Want: "*string", WantValue: "string", Rule: sdkcheck.RuleProperty})
		}
	}
	return refs
}
