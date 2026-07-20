// Copyright 2026 Coralogix Ltd.
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

package dashboardjson

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strings"
	"unicode"

	dashboardservice "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/dashboard_service"
)

var dashboardServicePackage = reflect.TypeOf(dashboardservice.Dashboard{}).PkgPath()

// Unmarshal accepts both the lowerCamelCase OpenAPI field names and the
// snake_case protobuf field names historically accepted by protojson.
func Unmarshal(data []byte, target any) error {
	targetType := reflect.TypeOf(target)
	if targetType == nil || targetType.Kind() != reflect.Pointer || reflect.ValueOf(target).IsNil() {
		return fmt.Errorf("dashboard JSON target must be a non-nil pointer")
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var raw any
	if err := decoder.Decode(&raw); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("invalid JSON after top-level value")
		}
		return err
	}

	normalized := normalizeProtoFieldNames(raw, targetType)
	encoded, err := json.Marshal(normalized)
	if err != nil {
		return fmt.Errorf("marshal normalized dashboard JSON: %w", err)
	}
	if err := json.Unmarshal(encoded, target); err != nil {
		return fmt.Errorf("unmarshal normalized dashboard JSON: %w", err)
	}
	return nil
}

func normalizeProtoFieldNames(raw any, targetType reflect.Type) any {
	for targetType.Kind() == reflect.Pointer {
		targetType = targetType.Elem()
	}

	switch targetType.Kind() {
	case reflect.Struct:
		if targetType.PkgPath() != dashboardServicePackage {
			return raw
		}
		object, ok := raw.(map[string]any)
		if !ok {
			return raw
		}
		return normalizeProtoObjectFieldNames(object, targetType)
	case reflect.Slice, reflect.Array:
		items, ok := raw.([]any)
		if !ok {
			return raw
		}
		normalized := make([]any, len(items))
		for i, item := range items {
			normalized[i] = normalizeProtoFieldNames(item, targetType.Elem())
		}
		return normalized
	case reflect.Map:
		object, ok := raw.(map[string]any)
		if !ok || targetType.Key().Kind() != reflect.String {
			return raw
		}
		normalized := make(map[string]any, len(object))
		for key, value := range object {
			normalized[key] = normalizeProtoFieldNames(value, targetType.Elem())
		}
		return normalized
	case reflect.String:
		// protobuf int64/uint64 fields (e.g. seriesCountLimit) are modeled as
		// Go strings but commonly authored as bare JSON numbers.
		if number, ok := raw.(json.Number); ok {
			return number.String()
		}
		return raw
	default:
		return raw
	}
}

func normalizeProtoObjectFieldNames(object map[string]any, targetType reflect.Type) map[string]any {
	normalized := make(map[string]any, len(object))
	consumed := make(map[string]struct{})

	for i := 0; i < targetType.NumField(); i++ {
		field := targetType.Field(i)
		jsonName := strings.Split(field.Tag.Get("json"), ",")[0]
		if jsonName == "" || jsonName == "-" {
			continue
		}

		value, found := object[jsonName]
		if found {
			consumed[jsonName] = struct{}{}
		}

		alias := protobufJSONFieldName(jsonName)
		if alias != jsonName {
			if aliasValue, aliasFound := object[alias]; aliasFound {
				// Preserve the compatibility helper's historical behavior when
				// both spellings are present: the protobuf spelling wins.
				value = aliasValue
				found = true
				consumed[alias] = struct{}{}
			}
		}

		if found {
			normalized[jsonName] = normalizeProtoFieldNames(value, field.Type)
		}
	}

	for key, value := range object {
		if _, ok := consumed[key]; !ok {
			normalized[key] = value
		}
	}
	return normalized
}

func protobufJSONFieldName(jsonName string) string {
	var result strings.Builder
	for i, character := range jsonName {
		if unicode.IsUpper(character) {
			if i > 0 {
				result.WriteByte('_')
			}
			character = unicode.ToLower(character)
		}
		result.WriteRune(character)
	}
	return result.String()
}
