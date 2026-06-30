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

func TestGetCustomEvaluationByIDTreatsListNotFoundAsEmpty(t *testing.T) {
	t.Parallel()

	cfg := aievaluations.NewConfiguration()
	cfg.Servers = aievaluations.ServerConfigurations{
		{
			URL: "https://example.test",
		},
	}
	cfg.HTTPClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if r.URL.Path != "/ai/custom-evaluations/v3" {
				t.Fatalf("unexpected request path: %s", r.URL.Path)
			}

			return &http.Response{
				StatusCode: http.StatusNotFound,
				Status:     "404 Not Found",
				Header: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body:    io.NopCloser(strings.NewReader(`{"message":"not found"}`)),
				Request: r,
			}, nil
		}),
	}

	resource := &AICustomEvaluationResource{
		aiEvaluationsClient: aievaluations.NewAPIClient(cfg).AIEvaluationsServiceAPI,
	}

	customEvaluation, found, err := resource.getCustomEvaluationByID(context.Background(), "missing-custom-evaluation-id")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if found {
		t.Fatalf("expected custom evaluation to be missing, got found=true with id %q", customEvaluation.GetId())
	}
}

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

	resource := &AICustomEvaluationResource{
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

func TestExtractUpdateAICustomEvaluationKeepsEmptyExamplesNonNil(t *testing.T) {
	t.Parallel()

	acceptable := aiCustomEvaluationEmptyCriterionModel()
	prohibited := aiCustomEvaluationEmptyCriterionModel()
	plan := AICustomEvaluationResourceModel{
		Name:                      types.StringValue("custom evaluation"),
		PolicyType:                types.StringValue(aiCustomEvaluationPolicyTypeQuality),
		Description:               types.StringValue("description"),
		Instructions:              types.StringValue("Score whether {response} matches the policy."),
		ShouldIncludeSystemPrompt: types.BoolValue(false),
		Criteria: &AICustomEvaluationCriteriaModel{
			Acceptable: &acceptable,
			Prohibited: &prohibited,
		},
	}

	rq, diags := extractUpdateAICustomEvaluation(context.Background(), plan)
	if diags.HasError() {
		t.Fatalf("expected no diagnostics, got %v", diags)
	}
	if rq.UpdateMask != nil {
		t.Fatalf("expected no update mask on the primary update request, got %q", *rq.UpdateMask)
	}
	if rq.Examples == nil {
		t.Fatal("expected examples to be a non-nil empty slice")
	}
	if len(rq.Examples) != 0 {
		t.Fatalf("expected no examples, got %d", len(rq.Examples))
	}
}

func TestExtractAICustomEvaluationCriteriaMapsExampleScores(t *testing.T) {
	t.Parallel()

	acceptable := aiCustomEvaluationEmptyCriterionModel()
	acceptable.Examples = types.ListValueMust(
		types.StringType,
		[]attr.Value{
			types.StringValue("acceptable example"),
		},
	)
	prohibited := aiCustomEvaluationEmptyCriterionModel()
	prohibited.Examples = types.ListValueMust(
		types.StringType,
		[]attr.Value{
			types.StringValue("prohibited example"),
		},
	)
	criteria := &AICustomEvaluationCriteriaModel{
		Acceptable: &acceptable,
		Prohibited: &prohibited,
	}

	examples, _, _, diags := extractAICustomEvaluationCriteria(context.Background(), criteria)
	if diags.HasError() {
		t.Fatalf("expected no diagnostics, got %v", diags)
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

func TestClearCustomEvaluationExamplesSendsExamplesUpdateMask(t *testing.T) {
	t.Parallel()

	customEvaluationID := "00000000-0000-0000-0000-000000000000"
	cfg := aievaluations.NewConfiguration()
	cfg.Servers = aievaluations.ServerConfigurations{
		{
			URL: "https://example.test",
		},
	}
	cfg.HTTPClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if r.Method != http.MethodPatch {
				t.Fatalf("expected PATCH, got %s", r.Method)
			}
			if r.URL.Path != "/ai/custom-evaluations/v3/"+customEvaluationID {
				t.Fatalf("unexpected request path: %s", r.URL.Path)
			}

			bodyBytes, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("failed to read request body: %v", err)
			}
			var body map[string]any
			if err := json.Unmarshal(bodyBytes, &body); err != nil {
				t.Fatalf("failed to decode request body %q: %v", string(bodyBytes), err)
			}
			if got := body["updateMask"]; got != aiCustomEvaluationExamplesUpdateMask {
				t.Fatalf("expected updateMask %q, got %v", aiCustomEvaluationExamplesUpdateMask, got)
			}
			examples, ok := body["examples"].([]any)
			if !ok {
				t.Fatalf("expected examples array, got %T", body["examples"])
			}
			if len(examples) != 0 {
				t.Fatalf("expected empty examples, got %v", examples)
			}
			if _, ok := body["policyType"]; ok {
				t.Fatalf("did not expect policyType in examples clear request: %v", body)
			}

			response := fmt.Sprintf(`{
				"item": {
					"id": %q,
					"name": "custom evaluation",
					"description": "",
					"config": {
						"instructions": "Score whether {response} matches the policy.",
						"policyType": "quality",
						"safe": "",
						"violates": "",
						"shouldIncludeSystemPrompt": false,
						"examples": []
					}
				}
			}`, customEvaluationID)
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Header: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body:    io.NopCloser(strings.NewReader(response)),
				Request: r,
			}, nil
		}),
	}
	resource := &AICustomEvaluationResource{
		aiEvaluationsClient: aievaluations.NewAPIClient(cfg).AIEvaluationsServiceAPI,
	}

	customEvaluation, _, err := resource.clearCustomEvaluationExamples(context.Background(), customEvaluationID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	config := customEvaluation.GetConfig()
	if examples := config.GetExamples(); len(examples) != 0 {
		t.Fatalf("expected cleared examples, got %v", examples)
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
