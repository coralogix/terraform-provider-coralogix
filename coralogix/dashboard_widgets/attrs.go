// Copyright 2025 Coralogix Ltd.
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

package dashboardwidgets

import (
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func SpansFieldModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"type":  types.StringType,
		"value": types.StringType,
	}
}

func GroupingAggregationModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"id":         types.StringType,
		"name":       types.StringType,
		"is_visible": types.BoolType,
		"aggregation": types.ObjectType{
			AttrTypes: AggregationModelAttr(),
		},
	}
}

func AggregationModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"type":              types.StringType,
		"field":             types.StringType,
		"percent":           types.Float64Type,
		"observation_field": ObservationFieldsObject(),
	}
}

func FilterSourceModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"logs":    types.ObjectType{AttrTypes: LogsFilterModelAttr()},
		"metrics": types.ObjectType{AttrTypes: filterSourceMetricsModelAttr()},
		"spans":   types.ObjectType{AttrTypes: filterSourceSpansModelAttr()},
	}
}

func LogsFilterModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"field": types.StringType,
		"operator": types.ObjectType{
			AttrTypes: FilterOperatorModelAttr(),
		},
		"observation_field": types.ObjectType{
			AttrTypes: ObservationFieldAttr(),
		},
	}
}

func filterSourceSpansModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"field": types.ObjectType{
			AttrTypes: SpansFieldModelAttr(),
		},
		"operator": types.ObjectType{
			AttrTypes: FilterOperatorModelAttr(),
		},
	}
}

func filterSourceMetricsModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"metric_name": types.StringType,
		"label":       types.StringType,
		"operator": types.ObjectType{
			AttrTypes: FilterOperatorModelAttr(),
		},
	}
}

func TimeFrameModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"absolute": types.ObjectType{AttrTypes: AbsoluteTimeFrameAttr()},
		"relative": types.ObjectType{AttrTypes: RelativeTimeFrameAttr()},
	}
}

func AbsoluteTimeFrameAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"start": types.StringType,
		"end":   types.StringType,
	}
}

func RelativeTimeFrameAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"duration": types.StringType,
	}
}

func SpansFilterModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"field": types.ObjectType{
			AttrTypes: SpansFieldModelAttr(),
		},
		"operator": types.ObjectType{
			AttrTypes: FilterOperatorModelAttr(),
		},
	}
}

func FilterOperatorModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"type": types.StringType,
		"selected_values": types.ListType{
			ElemType: types.StringType,
		},
	}
}

func SpansAggregationModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"type":             types.StringType,
		"aggregation_type": types.StringType,
		"field":            types.StringType,
	}
}

func MetricsFilterModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"metric": types.StringType,
		"label":  types.StringType,
		"operator": types.ObjectType{
			AttrTypes: FilterOperatorModelAttr(),
		},
	}
}

func ObservationFieldAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"keypath": types.ListType{
			ElemType: types.StringType,
		},
		"scope": types.StringType,
	}
}

func ThresholdAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"from":  types.NumberType,
		"color": types.StringType,
		"label": types.StringType,
	}
}

func LegendAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"is_visible": types.BoolType,
		"columns": types.ListType{
			ElemType: types.StringType,
		},
		"group_by_query": types.BoolType,
		"placement":      types.StringType,
	}
}
