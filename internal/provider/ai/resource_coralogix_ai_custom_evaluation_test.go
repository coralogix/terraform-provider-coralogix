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

package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	aiapplications "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/ai_applications_service"
	aievaluations "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/ai_evaluations_service"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestListAIApplicationsRequestsConsecutivePageOffsets(t *testing.T) {
	t.Parallel()

	var requestedOffsets []string
	cfg := aiapplications.NewConfiguration()
	cfg.Servers = aiapplications.ServerConfigurations{
		{
			URL: "https://example.test",
		},
	}
	cfg.HTTPClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if r.URL.Path != "/ai/applications/v3" {
				t.Fatalf("unexpected request path: %s", r.URL.Path)
			}
			if got := r.URL.Query().Get("page_size"); got != "200" {
				t.Fatalf("expected page_size=200, got %q", got)
			}

			offset := r.URL.Query().Get("page_offset")
			requestedOffsets = append(requestedOffsets, offset)

			body := ""
			switch offset {
			case "0":
				body = aiApplicationsResponse(0, 200)
			case "1":
				body = aiApplicationsResponse(200, 1)
			default:
				t.Fatalf("unexpected page_offset: %s", offset)
			}

			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Header: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body:    io.NopCloser(strings.NewReader(body)),
				Request: r,
			}, nil
		}),
	}

	resource := &aiCustomEvaluationResource{
		aiApplicationsClient: aiapplications.NewAPIClient(cfg).AIApplicationsServiceAPI,
	}

	applications, err := resource.listAIApplications(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(applications) != 201 {
		t.Fatalf("expected 201 applications, got %d", len(applications))
	}
	if strings.Join(requestedOffsets, ",") != "0,1" {
		t.Fatalf("expected page offsets 0,1, got %v", requestedOffsets)
	}
	if applications[200].ID != "app-200" {
		t.Fatalf("expected last application from page 1, got %q", applications[200].ID)
	}
}

// The criteria of a configuration become one list of examples, with the score 1 for an acceptable
// example and 0 for a prohibited one, and the flags become the fields safe and violates.
func TestExtractAICustomEvaluationCriteriaMapsExampleScores(t *testing.T) {
	t.Parallel()

	criterion := func(flags string, examples ...string) types.Object {
		values := make([]attr.Value, 0, len(examples))
		for _, example := range examples {
			values = append(values, types.StringValue(example))
		}
		return types.ObjectValueMust(aiCustomEvaluationCriterionAttributeTypes(), map[string]attr.Value{
			"flags":    types.StringValue(flags),
			"examples": types.ListValueMust(types.StringType, values),
		})
	}
	criteria := types.ObjectValueMust(aiCustomEvaluationCriteriaAttributeTypes(), map[string]attr.Value{
		"acceptable": criterion("polite", "acceptable example"),
		"prohibited": criterion("rude", "prohibited example"),
	})

	examples, safe, violates, diags := extractAICustomEvaluationCriteria(context.Background(), criteria)
	if diags.HasError() {
		t.Fatalf("expected no diagnostics, got %v", diags)
	}
	if safe != "polite" || violates != "rude" {
		t.Fatalf("expected the flags polite and rude, got %q and %q", safe, violates)
	}
	if len(examples) != 2 {
		t.Fatalf("expected 2 examples, got %d", len(examples))
	}
	if got := examples[0].GetConversation(); got != "acceptable example" {
		t.Fatalf("expected first example to be acceptable, got %q", got)
	}
	if got := examples[0].GetScore(); got != aiCustomEvaluationAcceptableScore {
		t.Fatalf("expected acceptable score %q, got %q", aiCustomEvaluationAcceptableScore, got)
	}
	if got := examples[1].GetConversation(); got != "prohibited example" {
		t.Fatalf("expected second example to be prohibited, got %q", got)
	}
	if got := examples[1].GetScore(); got != aiCustomEvaluationProhibitedScore {
		t.Fatalf("expected prohibited score %q, got %q", aiCustomEvaluationProhibitedScore, got)
	}
}

// A criteria object that is null or unknown, as in a configuration that omits it, sends nothing.
func TestExtractAICustomEvaluationCriteriaTreatsMissingCriteriaAsEmpty(t *testing.T) {
	t.Parallel()

	for _, criteria := range []types.Object{
		types.ObjectNull(aiCustomEvaluationCriteriaAttributeTypes()),
		types.ObjectUnknown(aiCustomEvaluationCriteriaAttributeTypes()),
	} {
		examples, safe, violates, diags := extractAICustomEvaluationCriteria(context.Background(), criteria)
		if diags.HasError() || len(examples) != 0 || safe != "" || violates != "" {
			t.Fatalf("expected empty criteria, got %v, %q, %q, %v", examples, safe, violates, diags)
		}
	}
}

// A field of an example that the resource does not show makes the read fail. The next update would
// replace the whole list of examples and lose it.
func TestFlattenAICustomEvaluationCriteriaRejectsUnknownExampleFields(t *testing.T) {
	t.Parallel()

	var config aievaluations.CustomEvaluationConfig
	if err := json.Unmarshal([]byte(`{"examples":[{"conversation":"c","score":"1","reason":"because"}]}`), &config); err != nil {
		t.Fatal(err)
	}

	_, diags := flattenAICustomEvaluationCriteria(context.Background(), config)
	if !diags.HasError() || !strings.Contains(diags[0].Summary(), "Unsupported AI custom evaluation example field") {
		t.Fatalf("expected an error for the unknown field, got %v", diags)
	}
}

func aiApplicationsResponse(start int, count int) string {
	var builder strings.Builder
	builder.WriteString(`{"aiApplications":[`)
	for i := range count {
		if i > 0 {
			builder.WriteByte(',')
		}
		index := start + i
		fmt.Fprintf(
			&builder,
			`{"id":"app-%03d","application":"application-%03d","subsystem":"subsystem-%03d"}`,
			index,
			index,
			index,
		)
	}
	builder.WriteString(`]}`)
	return builder.String()
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}
