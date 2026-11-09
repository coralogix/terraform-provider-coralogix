// Copyright 2024 Coralogix Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an AS IS BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package provider

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// The REST gateway of the alerts API writes every field that has no presence: a missing list is
// [], a missing map is {}, a missing enum is its zero value, a missing scalar is its zero value.
// The fake reads the shapes from the OpenAPI document of the pinned SDK, so it follows the
// contract that the provider is built against.
var alertSpec struct {
	once    sync.Once
	schemas map[string]any
	err     error
}

func alertSchemas() (map[string]any, error) {
	alertSpec.once.Do(func() {
		out, err := exec.Command("go", "list", "-m", "-f", "{{.Dir}}", "github.com/coralogix/coralogix-management-sdk").Output()
		if err != nil {
			alertSpec.err = err
			return
		}
		raw, err := os.ReadFile(filepath.Join(strings.TrimSpace(string(out)), "openapi.yaml"))
		if err != nil {
			alertSpec.err = err
			return
		}
		var doc map[string]any
		if alertSpec.err = yaml.Unmarshal(raw, &doc); alertSpec.err != nil {
			return
		}
		components, _ := doc["components"].(map[string]any)
		alertSpec.schemas, _ = components["schemas"].(map[string]any)
	})
	return alertSpec.schemas, alertSpec.err
}

// alertResolve follows $ref and a single allOf to the schema that holds the properties. The
// description and the presence marker of the outer schema win, as the generator reads them.
func alertResolve(schemas map[string]any, schema map[string]any) map[string]any {
	presence := schema["x-coralogix-presence"]
	for i := 0; i < 8; i++ {
		if ref, ok := schema["$ref"].(string); ok {
			next, _ := schemas[strings.TrimPrefix(ref, "#/components/schemas/")].(map[string]any)
			schema = next
			continue
		}
		if all, ok := schema["allOf"].([]any); ok && len(all) == 1 {
			next, _ := all[0].(map[string]any)
			schema = next
			continue
		}
		break
	}
	if presence == true {
		merged := map[string]any{}
		for k, v := range schema {
			merged[k] = v
		}
		merged["x-coralogix-presence"] = true
		return merged
	}
	return schema
}

// alertNoZeroFields are the fields that the gateway does not write when they are missing, in spite
// of having no presence marker in the contract.
var alertNoZeroFields = map[string]bool{"luceneQuery": true}

// alertZeroFill returns value with every field that has no presence written, recursively. A
// field that holds a message and is missing stays missing: the server decides which messages it
// fills in (see alertDefaults).
func alertZeroFill(schemas map[string]any, schema map[string]any, value any) any {
	schema = alertResolve(schemas, schema)
	switch x := value.(type) {
	case map[string]any:
		if props, ok := schema["properties"].(map[string]any); ok {
			for name, raw := range props {
				sub, _ := raw.(map[string]any)
				if sub == nil {
					continue
				}
				field := alertResolve(schemas, sub)
				if existing, present := x[name]; present {
					x[name] = alertZeroFill(schemas, sub, existing)
					continue
				}
				// An object with a oneOf has at most one arm: the gateway writes no zero into
				// the arms that are not set (the SDK refuses a response with two).
				if alertUnionArms(schema)[name] {
					continue
				}
				// Recorded: the API drops an empty lucene query and never writes one.
				if alertNoZeroFields[name] {
					continue
				}
				if zero, ok := alertZero(field); ok {
					x[name] = zero
				}
			}
		} else if extra, ok := schema["additionalProperties"].(map[string]any); ok {
			for k, item := range x {
				x[k] = alertZeroFill(schemas, extra, item)
			}
		}
		return x
	case []any:
		items, _ := schema["items"].(map[string]any)
		if items == nil {
			return x
		}
		for i, item := range x {
			x[i] = alertZeroFill(schemas, items, item)
		}
		return x
	}
	return value
}

// alertUnionArms returns the names of the properties that are the arms of the oneOf of schema.
// The other properties of the same object are ordinary fields.
func alertUnionArms(schema map[string]any) map[string]bool {
	arms := map[string]bool{}
	union, _ := schema["oneOf"].([]any)
	for _, raw := range union {
		arm, _ := raw.(map[string]any)
		required, _ := arm["required"].([]any)
		for _, name := range required {
			if n, ok := name.(string); ok {
				arms[n] = true
			}
		}
	}
	return arms
}

// alertZero is the value that the gateway writes for a missing field, and false when it writes
// none: a field with presence and a message.
func alertZero(field map[string]any) (any, bool) {
	if field["x-coralogix-presence"] == true {
		return nil, false
	}
	switch field["type"] {
	case "array":
		return []any{}, true
	case "object":
		if _, isMap := field["additionalProperties"]; isMap {
			return map[string]any{}, true
		}
		return nil, false
	case "string":
		if values, ok := field["enum"].([]any); ok && len(values) > 0 {
			return values[0], true
		}
		return "", true
	case "boolean":
		return false, true
	case "integer", "number":
		return float64(0), true
	}
	return nil, false
}
