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

package dashboardschema

import (
	"context"
	"fmt"
	dashboardwidgets "terraform-provider-coralogix/coralogix/dashboard_widgets"
	"time"

	cxsdk "github.com/coralogix/coralogix-management-sdk/go"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

var (
	JSONUnmarshal = protojson.UnmarshalOptions{
		DiscardUnknown: true,
		AllowPartial:   true,
	}
)

type intervalValidator struct{}

func (i intervalValidator) Description(_ context.Context) string {
	return "A duration string, such as 1s or 1m."
}

func (i intervalValidator) MarkdownDescription(_ context.Context) string {
	return "A duration string, such as 1s or 1m."
}

func (i intervalValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() {
		return
	}
	_, err := time.ParseDuration(req.ConfigValue.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("invalid duration", err.Error())
	}
}

type ContentJsonValidator struct{}

func (c ContentJsonValidator) Description(_ context.Context) string {
	return ""
}

func (c ContentJsonValidator) MarkdownDescription(_ context.Context) string {
	return ""
}

func (c ContentJsonValidator) ValidateString(_ context.Context, request validator.StringRequest, response *validator.StringResponse) {
	if request.ConfigValue.IsNull() || request.ConfigValue.IsUnknown() {
		return
	}

	err := JSONUnmarshal.Unmarshal([]byte(request.ConfigValue.ValueString()), &cxsdk.Dashboard{})
	if err != nil {
		response.Diagnostics.Append(diag.NewErrorDiagnostic("content_json validation failed", fmt.Sprintf("json content is not matching layout schema. got an err while unmarshalling - %s", err)))
	}
}

func stringOrVariableSchema() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Attributes: stringOrVariableAttr(),
		Optional:   true,
	}
}

func stringOrVariableAttr() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"string_value": schema.StringAttribute{
			Optional: true,
			Validators: []validator.String{
				stringvalidator.ExactlyOneOf(
					path.MatchRelative().AtParent().AtName("variable_name"),
				),
			},
		},
		"variable_name": schema.StringAttribute{
			Optional: true,
		},
	}
}

func logsAndSpansAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"lucene_query": schema.StringAttribute{
			Optional: true,
		},
		"strategy": logsAndSpansStrategy(),
		"message_template": schema.StringAttribute{
			Optional: true,
		},
		"label_fields": schema.ListNestedAttribute{
			NestedObject: schema.NestedAttributeObject{
				Attributes: dashboardwidgets.ObservationFieldSchema(),
			},
			Optional: true,
		},
	}
}

func logsAndSpansStrategy() schema.Attribute {
	return schema.SingleNestedAttribute{
		Attributes: map[string]schema.Attribute{
			"instant": schema.SingleNestedAttribute{
				Attributes: map[string]schema.Attribute{
					"timestamp_field": observationFieldSingleNestedAttribute(),
				},
				Optional: true,
			},
			"range": schema.SingleNestedAttribute{
				Attributes: map[string]schema.Attribute{
					"start_timestamp_field": observationFieldSingleNestedAttribute(),
					"end_timestamp_field":   observationFieldSingleNestedAttribute(),
				},
				Optional: true,
			},
			"duration": schema.SingleNestedAttribute{
				Attributes: map[string]schema.Attribute{
					"start_timestamp_field": observationFieldSingleNestedAttribute(),
					"duration_field":        observationFieldSingleNestedAttribute(),
				},
				Optional: true,
			},
		},
		Required: true,
	}
}

func observationFieldSingleNestedAttribute() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Attributes: dashboardwidgets.ObservationFieldSchema(),
		Required:   true,
	}
}
