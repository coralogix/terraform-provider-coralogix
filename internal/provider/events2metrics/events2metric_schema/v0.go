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

package events2metricschema

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// V0 is the schema of version 0 of coralogix_events2metric, which used blocks. State of this
// version is upgraded by reading the events2metric from the API.
func V0() schema.Schema {
	return schema.Schema{
		Version: 0,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
			},
			"name": schema.StringAttribute{
				Required: true,
			},
			"description": schema.StringAttribute{
				Optional: true,
			},
		},
		Blocks: map[string]schema.Block{
			"metric_labels": schema.SetNestedBlock{
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"target_label": schema.StringAttribute{
							Required: true,
						},
						"source_field": schema.StringAttribute{
							Required: true,
						},
					},
				},
			},
			"metric_fields": schema.SetNestedBlock{
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"source_field": schema.StringAttribute{
							Required: true,
						},
					},
					Blocks: map[string]schema.Block{
						"aggregations": schema.SetNestedBlock{
							NestedObject: schema.NestedBlockObject{
								Blocks: map[string]schema.Block{
									"min":   commonAggregationV0(),
									"max":   commonAggregationV0(),
									"count": commonAggregationV0(),
									"avg":   commonAggregationV0(),
									"sum":   commonAggregationV0(),
									"samples": schema.ListNestedBlock{
										NestedObject: schema.NestedBlockObject{
											Attributes: map[string]schema.Attribute{
												"enable": schema.BoolAttribute{
													Optional: true,
													Computed: true,
												},
												"target_metric_name": schema.StringAttribute{
													Computed: true,
												},
												"type": schema.StringAttribute{
													Required: true,
												},
											},
										},
									},
									"histogram": schema.ListNestedBlock{
										NestedObject: schema.NestedBlockObject{
											Attributes: map[string]schema.Attribute{
												"enable": schema.BoolAttribute{
													Optional: true,
													Computed: true,
												},
												"target_metric_name": schema.StringAttribute{
													Computed: true,
												},
												"buckets": schema.ListAttribute{
													ElementType: types.Float64Type,
													Required:    true,
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			"spans_query": schema.ListNestedBlock{
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"lucene": schema.StringAttribute{
							Optional: true,
						},
						"applications": schema.SetAttribute{
							ElementType: types.StringType,
							Optional:    true,
						},
						"subsystems": schema.SetAttribute{
							ElementType: types.StringType,
							Optional:    true,
						},
						"actions": schema.SetAttribute{
							ElementType: types.StringType,
							Optional:    true,
						},
						"services": schema.SetAttribute{
							ElementType: types.StringType,
							Optional:    true,
						},
					},
				},
			},
			"logs_query": schema.ListNestedBlock{
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"lucene": schema.StringAttribute{
							Optional: true,
						},
						"applications": schema.SetAttribute{
							ElementType: types.StringType,
							Optional:    true,
						},
						"subsystems": schema.SetAttribute{
							ElementType: types.StringType,
							Optional:    true,
						},
						"severities": schema.SetAttribute{
							Optional:    true,
							Computed:    true,
							ElementType: types.StringType,
						},
					},
				},
			},
			"permutations": schema.ListNestedBlock{
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"limit":            schema.StringAttribute{},
						"has_exceed_limit": schema.BoolAttribute{},
					},
				},
			},
		},
	}
}

func commonAggregationV0() schema.ListNestedBlock {
	return schema.ListNestedBlock{
		NestedObject: schema.NestedBlockObject{
			Attributes: map[string]schema.Attribute{
				"enable": schema.BoolAttribute{
					Optional: true,
					Computed: true,
				},
				"target_metric_name": schema.StringAttribute{
					Computed: true,
				},
			},
		},
	}
}
