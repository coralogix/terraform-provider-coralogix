// Copyright 2025 Coralogix Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use it except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package dashboards

import (
	"encoding/json"
	"reflect"

	cxsdk "github.com/coralogix/coralogix-management-sdk/go"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// injectDynamicWidgetQueryDefinitions populates queryDefinitions on dynamic widgets when they are
// missing after unmarshalling content_json with DiscardUnknown: true. The API and SDK both support
// queryDefinitions (it is a known field), but some JSON exports or unmarshal paths can leave it
// empty; this ensures the create request includes at least one query definition so the server
// validation "either query or queryDefinitions (with at least 1 element) is required" is satisfied.
// DiscardUnknown remains true globally so the provider continues to act as a gatekeeper.
func injectDynamicWidgetQueryDefinitions(contentJSON string, dashboard *cxsdk.Dashboard) error {
	raw := make(map[string]interface{})
	if err := json.Unmarshal([]byte(contentJSON), &raw); err != nil {
		return err
	}
	rawByPath, err := collectRawDynamicQueryDefinitions(raw)
	if err != nil || len(rawByPath) == 0 {
		return err
	}
	layout := dashboard.GetLayout()
	if layout == nil {
		return nil
	}
	sections := layout.GetSections()
	if len(sections) == 0 {
		return nil
	}
	unmarshalOpts := protojson.UnmarshalOptions{AllowPartial: true}
	for si, section := range sections {
		rows := section.GetRows()
		if rows == nil {
			continue
		}
		for ri, row := range rows {
			widgets := row.GetWidgets()
			if widgets == nil {
				continue
			}
			for wi, widget := range widgets {
				def := widget.GetDefinition()
				if def == nil {
					continue
				}
				dyn := def.GetDynamic()
				if dyn == nil {
					continue
				}
				if len(dyn.GetQueryDefinitions()) > 0 {
					continue
				}
				key := pathKey{si, ri, wi}
				rawList, ok := rawByPath[key]
				if !ok || len(rawList) == 0 {
					continue
				}
				if err := setQueryDefinitionsViaReflection(dyn, rawList, unmarshalOpts); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

type pathKey struct {
	section, row, widget int
}

func collectRawDynamicQueryDefinitions(raw map[string]interface{}) (map[pathKey][]interface{}, error) {
	out := make(map[pathKey][]interface{})
	layout, _ := raw["layout"].(map[string]interface{})
	if layout == nil {
		return out, nil
	}
	sections, _ := layout["sections"].([]interface{})
	if sections == nil {
		return out, nil
	}
	for si, s := range sections {
		section, _ := s.(map[string]interface{})
		if section == nil {
			continue
		}
		rows, _ := section["rows"].([]interface{})
		if rows == nil {
			continue
		}
		for ri, r := range rows {
			row, _ := r.(map[string]interface{})
			if row == nil {
				continue
			}
			widgets, _ := row["widgets"].([]interface{})
			if widgets == nil {
				continue
			}
			for wi, w := range widgets {
				widget, _ := w.(map[string]interface{})
				if widget == nil {
					continue
				}
				definition, _ := widget["definition"].(map[string]interface{})
				if definition == nil {
					continue
				}
				dynamic, _ := definition["dynamic"].(map[string]interface{})
				if dynamic == nil {
					continue
				}
				var list []interface{}
				if qd, ok := dynamic["queryDefinitions"].([]interface{}); ok && len(qd) > 0 {
					list = qd
				} else if qd, ok := dynamic["query_definitions"].([]interface{}); ok && len(qd) > 0 {
					list = qd
				}
				if len(list) > 0 {
					out[pathKey{si, ri, wi}] = list
				}
			}
		}
	}
	return out, nil
}

func setQueryDefinitionsViaReflection(dyn interface{}, rawList []interface{}, opts protojson.UnmarshalOptions) error {
	if dyn == nil {
		return nil
	}
	v := reflect.ValueOf(dyn)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	f := v.FieldByName("QueryDefinitions")
	if !f.IsValid() || !f.CanSet() {
		return nil
	}
	sliceType := f.Type()
	elemPtrType := sliceType.Elem() // *Dynamic_QueryDefinition
	elemType := elemPtrType.Elem()  // Dynamic_QueryDefinition
	slice := reflect.MakeSlice(sliceType, 0, len(rawList))
	for _, rawItem := range rawList {
		rawBytes, err := json.Marshal(rawItem)
		if err != nil {
			continue
		}
		newMsg := reflect.New(elemType).Interface()
		pm, ok := newMsg.(proto.Message)
		if !ok {
			continue
		}
		if err := opts.Unmarshal(rawBytes, pm); err != nil {
			continue
		}
		slice = reflect.Append(slice, reflect.ValueOf(pm))
	}
	if slice.Len() > 0 {
		f.Set(slice)
	}
	return nil
}
