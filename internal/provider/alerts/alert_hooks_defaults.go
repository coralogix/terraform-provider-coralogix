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

package alerts

import (
	"reflect"

	alertapi "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/alert_definitions_service"
)

var (
	undetectedValuesType = reflect.TypeOf((*alertapi.V3UndetectedValuesManagement)(nil))
	simpleFilterType     = reflect.TypeOf(alertapi.LogsSimpleFilter{})
	noDataPolicyType     = reflect.TypeOf((*alertapi.NoDataPolicy)(nil))
	// noDataPolicyReadAsEmpty are the alert types whose released read shows a missing no-data policy
	// as an object without values.
	noDataPolicyReadAsEmpty = []reflect.Type{reflect.TypeOf(alertapi.LogsThresholdType{}), reflect.TypeOf(alertapi.MetricThresholdType{})}
)

// walkStructs calls visit with every struct below v, children first, so a visit can set a pointer field.
func walkStructs(v reflect.Value, visit func(reflect.Value)) {
	switch v.Kind() {
	case reflect.Pointer, reflect.Interface:
		if !v.IsNil() {
			walkStructs(v.Elem(), visit)
		}
	case reflect.Slice:
		for i := 0; i < v.Len(); i++ {
			walkStructs(v.Index(i), visit)
		}
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if v.Field(i).CanSet() {
				walkStructs(v.Field(i), visit)
			}
		}
		visit(v)
	}
}

// fillReleasedReads gives the values that the API leaves out the form that the released resource read them
// in: a missing Lucene query is "", missing undetected values settings are the defaults, and a missing
// no-data policy of a logs or metric threshold alert is an object without values. It walks the whole API
// value, so a type that has such a field is covered.
func fillReleasedReads(v reflect.Value) {
	walkStructs(v, fillStruct)
}

func fillStruct(v reflect.Value) {
	if v.Type() == simpleFilterType && v.CanAddr() {
		if filter, ok := v.Addr().Interface().(*alertapi.LogsSimpleFilter); ok && filter.LuceneQuery == nil {
			empty := ""
			filter.LuceneQuery = &empty
		}
	}
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		if !field.CanSet() || field.Kind() != reflect.Pointer || !field.IsNil() {
			continue
		}
		switch {
		case field.Type() == undetectedValuesType:
			field.Set(reflect.ValueOf(defaultUndetectedValues()))
		case field.Type() == noDataPolicyType && isNoDataPolicyReadAsEmpty(v.Type()):
			field.Set(reflect.ValueOf(&alertapi.NoDataPolicy{}))
		}
	}
}

// dropEmptyNoDataPolicies removes the no-data policy that sets nothing from a request: the released
// resource sent none for it, and the API answers a policy without values with the UNSPECIFIED state.
func dropEmptyNoDataPolicies(v reflect.Value) {
	walkStructs(v, func(s reflect.Value) {
		for i := 0; i < s.NumField(); i++ {
			field := s.Field(i)
			if !field.CanSet() || field.Type() != noDataPolicyType || field.IsNil() {
				continue
			}
			if policy := field.Interface().(*alertapi.NoDataPolicy); policy.State == nil && policy.AutoRetireSeconds == nil {
				field.Set(reflect.Zero(noDataPolicyType))
			}
		}
	})
}

func isNoDataPolicyReadAsEmpty(t reflect.Type) bool {
	for _, candidate := range noDataPolicyReadAsEmpty {
		if t == candidate {
			return true
		}
	}
	return false
}

// defaultUndetectedValues is the undetected values management of an alert that sets none.
func defaultUndetectedValues() *alertapi.V3UndetectedValuesManagement {
	never := alertapi.V3AUTORETIRETIMEFRAME_AUTO_RETIRE_TIMEFRAME_NEVER_OR_UNSPECIFIED
	trigger := false
	return &alertapi.V3UndetectedValuesManagement{TriggerUndetectedValues: &trigger, AutoRetireTimeframe: &never}
}
