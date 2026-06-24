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
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	aiapplications "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/ai_applications_service"
	aievaluations "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/ai_evaluations_service"
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
