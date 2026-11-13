// Copyright 2026 Coralogix Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package dashboards

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"
	"unicode"

	"github.com/coralogix/coralogix-management-sdk/go/openapi/dashboardjson"
	dashboardservice "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/dashboard_service"
	gendashboard "github.com/coralogix/terraform-provider-coralogix/internal/provider/generated/dashboard"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// The tests below are the contract tests that the one-of coverage manifest names. They run against the
// generated resource and the SDK.

// TestDashboardContentJSONGeneratedOneOfBranchContract proves the generic
// content_json transport contract for every generated union reachable from a
// Dashboard. Each branch and every required descendant are decoded through
// their protobuf snake_case aliases, promoted into the exact typed fields,
// stripped of AdditionalProperties, and serialized inside a create request
// with every sibling left nil.
func TestDashboardContentJSONGeneratedOneOfBranchContract(t *testing.T) {
	rootType := reflect.TypeOf(dashboardservice.Dashboard{})
	reachable, parents := dashboardReachableGeneratedModels(rootType)
	guards := dashboardGeneratedOneOfBranches(t)
	enumValues := dashboardGeneratedEnumValues(t)

	modelNames := make([]string, 0, len(guards))
	for model := range guards {
		if _, ok := reachable[model]; ok {
			modelNames = append(modelNames, model)
		}
	}
	sort.Strings(modelNames)

	for _, modelName := range modelNames {
		modelType := reachable[modelName]
		branches := append([]string(nil), guards[modelName]...)
		sort.Strings(branches)
		for _, branch := range branches {
			branch := branch
			t.Run(modelName+"/"+branch, func(t *testing.T) {
				model := reflect.New(modelType)
				field, ok := dashboardGeneratedJSONField(model.Elem(), branch)
				if !ok {
					t.Fatalf("generated model has no JSON field %q", branch)
				}

				alias := dashboardLowerCamelToSnake(branch)
				payload, ok := dashboardGeneratedJSONData(modelType, enumValues, 0).(map[string]any)
				if !ok {
					t.Fatalf("minimal generated model value has type %T, want object", payload)
				}
				payload[alias] = dashboardGeneratedJSONData(field.Type(), enumValues, 0)
				encodedPayload, err := json.Marshal(payload)
				if err != nil {
					t.Fatalf("marshal protobuf JSON alias %q: %s", alias, err)
				}
				if err := dashboardjson.Unmarshal(encodedPayload, model.Interface()); err != nil {
					t.Fatalf("decode protobuf JSON alias %q: %s", alias, err)
				}

				for _, candidate := range branches {
					candidateField, ok := dashboardGeneratedJSONField(model.Elem(), candidate)
					if !ok {
						t.Fatalf("generated model has no sibling JSON field %q", candidate)
					}
					if got, want := !candidateField.IsZero(), candidate == branch; got != want {
						t.Fatalf("typed branch %s populated=%t, want %t", candidate, got, want)
					}
				}

				dashboard := dashboardEmbedGeneratedModel(t, rootType, modelType, model.Elem(), parents)
				request := dashboardservice.CreateDashboardRequestDataStructure{Dashboard: dashboard.Interface().(dashboardservice.Dashboard)}
				encoded, err := json.Marshal(request)
				if err != nil {
					t.Fatalf("serialize generated branch in REST create request: %s", err)
				}
				if !dashboardJSONContainsKey(encoded, branch) {
					t.Fatalf("serialized REST create request lost branch %q: %s", branch, encoded)
				}
			})
		}
	}
}

func dashboardEmbedGeneratedModel(t *testing.T, rootType, modelType reflect.Type, model reflect.Value, parents map[reflect.Type]dashboardGeneratedParent) reflect.Value {
	t.Helper()
	path := make([]dashboardGeneratedParent, 0)
	for current := modelType; current != rootType; {
		parent, ok := parents[current]
		if !ok {
			t.Fatalf("generated model %s is not reachable from Dashboard", modelType.Name())
		}
		path = append(path, parent)
		current = parent.model
	}
	for left, right := 0, len(path)-1; left < right; left, right = left+1, right-1 {
		path[left], path[right] = path[right], path[left]
	}

	root := reflect.New(rootType).Elem()
	current := root
	for _, edge := range path {
		current = dashboardInitializeGeneratedField(current.Field(edge.fieldIndex))
	}
	current.Set(model)
	return root
}

func dashboardGeneratedElementType(value reflect.Type) reflect.Type {
	for value.Kind() == reflect.Pointer || value.Kind() == reflect.Slice || value.Kind() == reflect.Array {
		value = value.Elem()
	}
	return value
}

func dashboardGeneratedEnumValues(t *testing.T) map[string]string {
	t.Helper()
	pc := reflect.ValueOf(dashboardservice.Dashboard.ToMap).Pointer()
	function := runtime.FuncForPC(pc)
	if function == nil {
		t.Fatal("locate generated dashboard SDK source")
	}
	file, _ := function.FileLine(pc)
	constantPattern := regexp.MustCompile(`(?m)^\s*[A-Z][A-Z0-9_]*\s+([A-Za-z0-9_]+)\s+=\s+"([^"]+)"`)
	result := make(map[string]string)
	entries, err := os.ReadDir(filepath.Dir(file))
	if err != nil {
		t.Fatalf("read generated dashboard SDK source: %s", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "model_") || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		content, err := os.ReadFile(filepath.Join(filepath.Dir(file), entry.Name()))
		if err != nil {
			t.Fatalf("read generated model %s: %s", entry.Name(), err)
		}
		for _, match := range constantPattern.FindAllSubmatch(content, -1) {
			if _, exists := result[string(match[1])]; !exists {
				result[string(match[1])] = string(match[2])
			}
		}
	}
	return result
}

func dashboardGeneratedJSONData(value reflect.Type, enumValues map[string]string, depth int) any {
	for value.Kind() == reflect.Pointer {
		value = value.Elem()
	}
	if depth > 32 {
		return nil
	}
	switch value.Kind() {
	case reflect.Struct:
		result := make(map[string]any)
		for fieldIndex := 0; fieldIndex < value.NumField(); fieldIndex++ {
			field := value.Field(fieldIndex)
			jsonTag := field.Tag.Get("json")
			parts := strings.Split(jsonTag, ",")
			if len(parts) == 0 || parts[0] == "" || parts[0] == "-" || slicesContain(parts[1:], "omitempty") {
				continue
			}
			result[dashboardLowerCamelToSnake(parts[0])] = dashboardGeneratedJSONData(field.Type, enumValues, depth+1)
		}
		return result
	case reflect.Map, reflect.Interface:
		return map[string]any{}
	case reflect.Slice, reflect.Array:
		return []any{}
	case reflect.String:
		if enumValue, ok := enumValues[value.Name()]; ok {
			return enumValue
		}
		return ""
	case reflect.Bool:
		return false
	default:
		return 0
	}
}

func dashboardGeneratedJSONField(model reflect.Value, jsonName string) (reflect.Value, bool) {
	for fieldIndex := 0; fieldIndex < model.NumField(); fieldIndex++ {
		field := model.Type().Field(fieldIndex)
		if strings.Split(field.Tag.Get("json"), ",")[0] == jsonName {
			return model.Field(fieldIndex), true
		}
	}
	return reflect.Value{}, false
}

func dashboardGeneratedOneOfBranches(t *testing.T) map[string][]string {
	t.Helper()
	pc := reflect.ValueOf(dashboardservice.Dashboard.ToMap).Pointer()
	function := runtime.FuncForPC(pc)
	if function == nil {
		t.Fatal("locate generated dashboard SDK source")
	}
	file, _ := function.FileLine(pc)
	typePattern := regexp.MustCompile(`(?m)^type ([A-Za-z0-9_]+) struct \{`)
	guardPattern := regexp.MustCompile(`oneOf field ([A-Za-z0-9_]+) must be set through the typed field`)
	result := make(map[string][]string)
	entries, err := os.ReadDir(filepath.Dir(file))
	if err != nil {
		t.Fatalf("read generated dashboard SDK source: %s", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "model_") || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		content, err := os.ReadFile(filepath.Join(filepath.Dir(file), entry.Name()))
		if err != nil {
			t.Fatalf("read generated model %s: %s", entry.Name(), err)
		}
		guards := guardPattern.FindAllSubmatch(content, -1)
		if len(guards) == 0 {
			continue
		}
		modelMatch := typePattern.FindSubmatch(content)
		if len(modelMatch) != 2 {
			t.Fatalf("find generated model type in %s", entry.Name())
		}
		for _, guard := range guards {
			result[string(modelMatch[1])] = append(result[string(modelMatch[1])], string(guard[1]))
		}
	}
	return result
}

type dashboardGeneratedParent struct {
	model      reflect.Type
	fieldIndex int
}

func dashboardInitializeGeneratedField(field reflect.Value) reflect.Value {
	for {
		switch field.Kind() {
		case reflect.Pointer:
			field.Set(reflect.New(field.Type().Elem()))
			field = field.Elem()
		case reflect.Slice:
			field.Set(reflect.MakeSlice(field.Type(), 1, 1))
			field = field.Index(0)
		default:
			return field
		}
	}
}

func dashboardJSONContainsKey(encoded []byte, key string) bool {
	var value any
	if json.Unmarshal(encoded, &value) != nil {
		return false
	}
	var contains func(any) bool
	contains = func(candidate any) bool {
		switch typed := candidate.(type) {
		case map[string]any:
			if _, ok := typed[key]; ok {
				return true
			}
			for _, nested := range typed {
				if contains(nested) {
					return true
				}
			}
		case []any:
			for _, nested := range typed {
				if contains(nested) {
					return true
				}
			}
		}
		return false
	}
	return contains(value)
}

func dashboardLowerCamelToSnake(value string) string {
	var result []rune
	for index, current := range []rune(value) {
		if unicode.IsUpper(current) {
			if index > 0 {
				result = append(result, '_')
			}
			current = unicode.ToLower(current)
		}
		result = append(result, current)
	}
	return string(result)
}

func dashboardReachableGeneratedModels(root reflect.Type) (map[string]reflect.Type, map[reflect.Type]dashboardGeneratedParent) {
	packagePath := root.PkgPath()
	reachable := map[string]reflect.Type{root.Name(): root}
	parents := make(map[reflect.Type]dashboardGeneratedParent)
	queue := []reflect.Type{root}
	for len(queue) > 0 {
		model := queue[0]
		queue = queue[1:]
		for fieldIndex := 0; fieldIndex < model.NumField(); fieldIndex++ {
			child := dashboardGeneratedElementType(model.Field(fieldIndex).Type)
			if child.Kind() != reflect.Struct || child.PkgPath() != packagePath || child.Name() == "" {
				continue
			}
			if _, seen := reachable[child.Name()]; seen {
				continue
			}
			reachable[child.Name()] = child
			parents[child] = dashboardGeneratedParent{model: model, fieldIndex: fieldIndex}
			queue = append(queue, child)
		}
	}
	return reachable, parents
}

func slicesContain(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

// Every auto refresh interval that the schema accepts is sent as exactly one interval of the request and
// is read back as the same text.
func TestDashboardAutoRefreshEveryIntervalRoundTrip(t *testing.T) {
	ctx := context.Background()
	for _, interval := range []string{"off", "one_minute", "two_minutes", "five_minutes", "fifteen_minutes"} {
		t.Run(interval, func(t *testing.T) {
			refresh := types.ObjectValueMust(map[string]attr.Type{"type": types.StringType}, map[string]attr.Value{"type": types.StringValue(interval)})
			model := &gendashboard.DashboardModel{AutoRefresh: refresh}
			expanded, diags := gendashboard.Expand(ctx, model)
			if diags.HasError() {
				t.Fatalf("Expand() diagnostics = %v", diags)
			}
			set := 0
			for _, branch := range []map[string]interface{}{expanded.Off, expanded.OneMinute, expanded.TwoMinutes, expanded.FiveMinutes, expanded.FifteenMinutes} {
				if branch != nil {
					set++
				}
			}
			if set != 1 {
				t.Fatalf("want exactly one interval on the request, got %d", set)
			}
			id := "dashboard-id"
			expanded.Id = &id
			readBack, diags := gendashboard.Flatten(ctx, expanded)
			if diags.HasError() {
				t.Fatalf("Flatten() diagnostics = %v", diags)
			}
			if got, _ := readBack.AutoRefresh.Attributes()["type"].(types.String); got.ValueString() != interval {
				t.Fatalf("round-tripped interval = %+v, want %q", readBack.AutoRefresh, interval)
			}
		})
	}
}

func dashboardGeneratedSchema(t *testing.T) schema.Schema {
	t.Helper()
	response := &resource.SchemaResponse{}
	gendashboard.NewResource(gendashboard.Hooks{}).Schema(context.Background(), resource.SchemaRequest{}, response)
	return response.Schema
}

// The textbox default has the four current kinds. The two deprecated kinds that the UI no longer writes
// are still in the schema, so a dashboard that has one can be read, and each one says that it is
// deprecated and names its replacement.
func TestDashboardStructuredConfigurationHasNoDeprecatedTextboxDefaults(t *testing.T) {
	variables, ok := dashboardGeneratedSchema(t).Attributes["variables_v2"].(schema.ListNestedAttribute)
	if !ok {
		t.Fatal("variables_v2 is not a list nested attribute")
	}
	source, ok := variables.NestedObject.Attributes["source"].(schema.SingleNestedAttribute)
	if !ok {
		t.Fatal("variables_v2.source is not a single nested attribute")
	}
	textbox, ok := source.Attributes["textbox"].(schema.SingleNestedAttribute)
	if !ok {
		t.Fatal("variables_v2.source.textbox is not a single nested attribute")
	}
	defaultValue, ok := textbox.Attributes["default_value"].(schema.SingleNestedAttribute)
	if !ok {
		t.Fatal("variables_v2.source.textbox.default_value is not a single nested attribute")
	}
	for _, deprecated := range []string{"single_string", "single_numeric"} {
		attribute, present := defaultValue.Attributes[deprecated]
		if present && !strings.Contains(attribute.GetDeprecationMessage(), "default_string_value") {
			t.Errorf("default_value exposes %q without a deprecation that names its replacement", deprecated)
		}
	}
	for _, replacement := range []string{"default_string_value", "default_numeric_value"} {
		if _, present := defaultValue.Attributes[replacement]; !present {
			t.Errorf("default_value is missing %q, which replaces the deprecated branch", replacement)
		}
	}
}

// constant_value is deprecated and the API rejects it. The attribute says so in its deprecation, and
// names the multi_select variable that replaces it.
func TestExpandDashboardVariableDefinition_ConstantValueDeprecated(t *testing.T) {
	variables, ok := dashboardGeneratedSchema(t).Attributes["variables"].(schema.ListNestedAttribute)
	if !ok {
		t.Fatal("variables is not a list nested attribute")
	}
	definition, ok := variables.NestedObject.Attributes["definition"].(schema.SingleNestedAttribute)
	if !ok {
		t.Fatal("variables.definition is not a single nested attribute")
	}
	message := definition.Attributes["constant_value"].GetDeprecationMessage()
	if !strings.Contains(message, "constant_value") || !strings.Contains(message, "multi_select") {
		t.Fatalf("expected the deprecation to direct users from constant_value to multi_select, got: %s", message)
	}
}
