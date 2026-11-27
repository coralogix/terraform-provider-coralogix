package generator

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"

	"github.com/coralogix/coralogix-management-sdk/iac/model"
	"go.yaml.in/yaml/v4"
)

// The future-change probe answers one question for a resource: when the API grows in a typical way,
// does the resource still generate with no edit to its overrides, its hooks, or its acceptance
// file? It changes the contract the way an API team does, runs the same check as `tfgen check`, and
// reports each change that the resource cannot take.
//
// A change adds a new optional field to every message that the resource uses, or a new value to
// every enum that it uses. The new field is added to the response message and to the request
// messages together, as a real API change does. Each new name is unique.

// Probe changes, in report order.
const (
	ProbeStringField  = "string-field"
	ProbeBoolField    = "bool-field"
	ProbeIntegerField = "integer-field"
	ProbeEnumValue    = "enum-value"
)

// ProbeChanges are the changes that Probe tries.
var ProbeChanges = []string{ProbeStringField, ProbeBoolField, ProbeIntegerField, ProbeEnumValue}

// ProbeOptions are the inputs of Probe. They are the inputs of `tfgen check`.
type ProbeOptions struct {
	Resource       string
	OpenAPIPath    string
	OperationIDs   model.OperationIDs
	OverridesPath  string
	AcceptancePath string
}

// ProbeResult is the outcome of one change.
type ProbeResult struct {
	Change string
	// Passed: the resource passes the check after the change, or the resource has nothing that the
	// change can touch.
	Passed bool
	// Component is the message or enum whose change breaks the resource, and Problem the first
	// issue of the check. Both are empty when Passed.
	Component, Problem string
}

// Probe tries every change in ProbeChanges on the contract of the resource. It returns an error when
// the resource does not pass the check before any change: there is nothing to compare with.
func Probe(options ProbeOptions) ([]ProbeResult, error) {
	data, err := os.ReadFile(options.OpenAPIPath)
	if err != nil {
		return nil, fmt.Errorf("read OpenAPI %s: %w", options.OpenAPIPath, err)
	}
	if err := Check(checkOptionsOf(options, options.OpenAPIPath)); err != nil {
		return nil, fmt.Errorf("the resource fails the check before any change: %w", err)
	}
	file, err := readOverrides(options.OverridesPath)
	if err != nil {
		return nil, err
	}
	validated, err := validateOpenAPIWith(data, options.Resource, options.OperationIDs, "candidate.invalid/coralogix-management-sdk", providerModulePath, file)
	if err != nil {
		return nil, err
	}
	fieldGroups, enums := probeTargets(validated.resource)

	dir, err := os.MkdirTemp("", "tfgen-probe-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	// A change has its own candidate file, so the changes run at once. The dashboard needs most of
	// the time, and its changes are what the others wait for.
	tryChange := func(change string, groups [][]string) error {
		mutated, err := probeMutate(data, change, groups)
		if err != nil {
			return err
		}
		path := filepath.Join(dir, change+".yaml")
		if err := os.WriteFile(path, mutated, 0o600); err != nil {
			return err
		}
		return Check(checkOptionsOf(options, path))
	}

	results := make([]ProbeResult, len(ProbeChanges))
	var wg sync.WaitGroup
	for i, change := range ProbeChanges {
		groups := fieldGroups
		if change == ProbeEnumValue {
			groups = enums
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i] = probeChange(change, groups, tryChange)
		}()
	}
	wg.Wait()
	return results, nil
}

// probeChange tries one change on every group, and narrows a failure to one group.
func probeChange(change string, groups [][]string, try func(string, [][]string) error) ProbeResult {
	if len(groups) == 0 {
		return ProbeResult{Change: change, Passed: true}
	}
	err := try(change, groups)
	if err == nil {
		return ProbeResult{Change: change, Passed: true}
	}
	culprit, problem := probeCulprit(groups, err, func(sub [][]string) error { return try(change, sub) })
	return ProbeResult{Change: change, Component: culprit, Problem: problem}
}

func checkOptionsOf(options ProbeOptions, openAPIPath string) CheckOptions {
	return CheckOptions{
		Resource:       options.Resource,
		OpenAPIPath:    openAPIPath,
		OperationIDs:   options.OperationIDs,
		OverridesPath:  options.OverridesPath,
		AcceptancePath: options.AcceptancePath,
	}
}

// probeCulprit narrows a failing change to one group by halving, and returns the first component
// of that group and the first issue of the failure. When both halves pass alone, the groups fail
// only together, and the first group stands for them.
func probeCulprit(groups [][]string, failure error, try func([][]string) error) (string, string) {
	for len(groups) > 1 {
		half := len(groups) / 2
		if err := try(groups[:half]); err != nil {
			groups, failure = groups[:half], err
		} else if err := try(groups[half:]); err != nil {
			groups, failure = groups[half:], err
		} else {
			break
		}
	}
	return strings.Join(groups[0], "+"), firstProblem(failure)
}

// firstProblem is the first issue of an eligibility error, as "CODE location: message".
func firstProblem(err error) string {
	var eligibility *EligibilityError
	if errors.As(err, &eligibility) && len(eligibility.Report) != 0 {
		first := eligibility.Report[0]
		return strings.TrimSpace(fmt.Sprintf("%s %s: %s", first.Code, first.Location, first.Message))
	}
	return err.Error()
}

// probeTargets lists what the changes touch: the groups of messages that hold one field of the
// resource (the response message and the request messages of one object, changed together), and
// the enums.
func probeTopLevel(resource *model.Resource) ([]string, bool) {
	names := []string{resource.Name}
	for _, operation := range []model.Operation{resource.Create, resource.Update} {
		if operation.Method == "" {
			continue // the resource has no such write
		}
		component := operation.RequestComponent
		if component == "" {
			component = operation.Body
		}
		if component == "" || component == "inline" {
			return nil, false
		}
		names = append(names, component)
	}
	return names, true
}

func probeTargets(resource *model.Resource) (fieldGroups [][]string, enums [][]string) {
	c := &probeCollector{seenGroup: map[string]bool{}, seenEnum: map[string]bool{}, seen: map[*model.Type]bool{}}
	// The resource itself: the response message, and the request message of each write that it has.
	// A request body that is no component (an inline schema) cannot take the field with the response,
	// so the top level is left out: the field would be in one place only.
	if names, ok := probeTopLevel(resource); ok {
		c.addGroup(names...)
	}
	for _, field := range resource.Fields {
		c.walk(field.Type)
	}
	sort.Slice(c.groups, func(i, j int) bool { return strings.Join(c.groups[i], "+") < strings.Join(c.groups[j], "+") })
	sort.Slice(c.enums, func(i, j int) bool { return c.enums[i][0] < c.enums[j][0] })
	return c.groups, c.enums
}

// probeCollector gathers the groups and the enums of a resource type.
type probeCollector struct {
	groups, enums       [][]string
	seenGroup, seenEnum map[string]bool
	seen                map[*model.Type]bool
}

func (c *probeCollector) addGroup(names ...string) {
	var group []string
	for _, name := range names {
		if name != "" && name != "inline" && !slices.Contains(group, name) {
			group = append(group, name)
		}
	}
	key := strings.Join(group, "+")
	if len(group) != 0 && !c.seenGroup[key] {
		c.seenGroup[key] = true
		c.groups = append(c.groups, group)
	}
}

func (c *probeCollector) walk(t *model.Type) {
	if t == nil || c.seen[t] {
		return
	}
	c.seen[t] = true
	switch t.Kind {
	case model.Object:
		c.addGroup(t.Schema, t.CreateSchema, t.UpdateSchema)
		for _, field := range t.Fields {
			c.walk(field.Type)
		}
	case model.OneOf:
		for _, arm := range t.Fields {
			c.walk(arm.Type)
		}
	case model.List, model.Set, model.Map:
		c.walk(t.Elem)
	case model.Enum:
		if t.Schema != "" && !c.seenEnum[t.Schema] {
			c.seenEnum[t.Schema] = true
			c.enums = append(c.enums, []string{t.Schema})
		}
	}
}

// probeMutate returns the contract with the change applied to each group. A component that has no
// properties mapping (a message that is only an allOf) or no enum list is left as it is.
func probeMutate(data []byte, change string, groups [][]string) ([]byte, error) {
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("parse the contract: %w", err)
	}
	if root.Kind != yaml.DocumentNode || len(root.Content) == 0 {
		return nil, errors.New("the contract is not a document")
	}
	schemas := mapValue(mapValue(root.Content[0], "components"), "schemas")
	if schemas == nil {
		return nil, errors.New("the contract has no components.schemas")
	}
	for index, group := range groups {
		for _, name := range group {
			component := mapValue(schemas, name)
			switch change {
			case ProbeEnumValue:
				probeAddEnumValue(component, index)
			default:
				probeAddField(component, change, fmt.Sprintf("zzProbe%d", index))
			}
		}
	}
	clearFlowStyle(&root)
	return yaml.Marshal(&root)
}

func probeAddField(component *yaml.Node, change, name string) {
	properties := mapValue(component, "properties")
	if properties == nil || properties.Kind != yaml.MappingNode {
		return
	}
	schema := map[string]string{
		ProbeStringField:  "type: string",
		ProbeBoolField:    "type: boolean",
		ProbeIntegerField: "{type: integer, format: int32}",
	}[change]
	var value yaml.Node
	if err := yaml.Unmarshal([]byte(schema), &value); err != nil || len(value.Content) == 0 {
		return
	}
	properties.Content = append(properties.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: name}, value.Content[0])
}

func probeAddEnumValue(component *yaml.Node, index int) {
	values := mapValue(component, "enum")
	if values == nil || values.Kind != yaml.SequenceNode || len(values.Content) == 0 {
		return
	}
	var texts []string
	for _, item := range values.Content {
		if item.Kind != yaml.ScalarNode || item.Tag != "!!str" {
			return // an enum of numbers is not a protobuf enum
		}
		texts = append(texts, item.Value)
	}
	// The new value has the prefix of the others, as a new protobuf value has.
	prefix := texts[0]
	for _, text := range texts[1:] {
		for !strings.HasPrefix(text, prefix) {
			prefix = prefix[:len(prefix)-1]
		}
	}
	if cut := strings.LastIndex(prefix, "_"); cut >= 0 {
		prefix = prefix[:cut+1]
	} else {
		prefix = ""
	}
	values.Content = append(values.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: fmt.Sprintf("%sZZ_PROBE_%d", prefix, index)})
}
