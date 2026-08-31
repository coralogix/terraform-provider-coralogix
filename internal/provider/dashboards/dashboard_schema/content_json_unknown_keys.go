// Copyright 2024 Coralogix Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package dashboard_schema

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"unicode"

	dashboardservice "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/dashboard_service"
)

// dashboardModelPackage identifies the generated models. The SDK's decoder only
// descends into those, so anything else is an untyped subtree whose keys are
// meaningful to the backend and must not be reported.
var dashboardModelPackage = reflect.TypeOf(dashboardservice.Dashboard{}).PkgPath()

// unknownContentJSONKeys lists the JSON paths the SDK decoder will discard.
//
// The traversal mirrors the decoder's own field matching: a key is known when it
// equals a field's JSON tag or that tag's snake_case spelling, both of which the
// decoder accepts. Mirroring it is the point, because a key this reports but the
// decoder keeps would be a false alarm on a working configuration.
func unknownContentJSONKeys(content string) []string {
	var raw any
	if err := json.Unmarshal([]byte(content), &raw); err != nil {
		// Malformed JSON is already reported as an error by the caller.
		return nil
	}

	var found []string
	collectUnknownKeys(raw, reflect.TypeOf(dashboardservice.Dashboard{}), "", &found)
	sort.Strings(found)
	return found
}

func collectUnknownKeys(raw any, target reflect.Type, path string, found *[]string) {
	if target == nil {
		return
	}
	for target.Kind() == reflect.Pointer {
		target = target.Elem()
	}

	switch target.Kind() {
	case reflect.Struct:
		object, ok := raw.(map[string]any)
		if !ok || target.PkgPath() != dashboardModelPackage {
			return
		}
		collectUnknownObjectKeys(object, target, path, found)
	case reflect.Slice, reflect.Array:
		items, ok := raw.([]any)
		if !ok {
			return
		}
		for i, item := range items {
			collectUnknownKeys(item, target.Elem(), fmt.Sprintf("%s[%d]", path, i), found)
		}
	case reflect.Map:
		object, ok := raw.(map[string]any)
		if !ok || target.Key().Kind() != reflect.String {
			return
		}
		// Map keys are user data, so only the values are checked.
		for key, value := range object {
			collectUnknownKeys(value, target.Elem(), joinPath(path, key), found)
		}
	}
}

func collectUnknownObjectKeys(object map[string]any, target reflect.Type, path string, found *[]string) {
	known := make(map[string]reflect.Type, target.NumField()*2)
	for i := 0; i < target.NumField(); i++ {
		field := target.Field(i)
		name := strings.Split(field.Tag.Get("json"), ",")[0]
		if name == "" || name == "-" {
			continue
		}
		known[name] = field.Type
		if alias := snakeCaseFieldName(name); alias != name {
			known[alias] = field.Type
		}
	}

	for key, value := range object {
		fieldType, ok := known[key]
		if !ok {
			*found = append(*found, joinPath(path, key))
			continue
		}
		collectUnknownKeys(value, fieldType, joinPath(path, key), found)
	}
}

func snakeCaseFieldName(jsonName string) string {
	var out strings.Builder
	for i, character := range jsonName {
		if unicode.IsUpper(character) {
			if i > 0 {
				out.WriteByte('_')
			}
			character = unicode.ToLower(character)
		}
		out.WriteRune(character)
	}
	return out.String()
}

func joinPath(parent, field string) string {
	if parent == "" {
		return field
	}
	return parent + "." + field
}
