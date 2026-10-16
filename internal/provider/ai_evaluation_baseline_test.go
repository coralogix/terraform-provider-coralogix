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

package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"

	"github.com/coralogix/terraform-provider-coralogix/internal/utils/schemadump"
)

const (
	aiEvaluationGoldenDir = "testdata/ai_evaluation_baseline"
	aiEvalTypeName        = "coralogix_ai_evaluation"
)

// These tests record the behavior of the released coralogix_ai_evaluation.
// After the generator switch, the golden diff must show only message wording.
//
//	UPDATE_GOLDEN=1 go test ./internal/provider -run TestAIEvaluationBaseline

func TestAIEvaluationBaselineSchema(t *testing.T) {
	var current resource.SchemaResponse
	aiEvaluationResource().Schema(context.Background(), resource.SchemaRequest{}, &current)
	checkGoldenAt(t, aiEvaluationGoldenDir, "schema_current.txt", []byte(schemadump.Text(current.Schema)))
}

func TestAIEvaluationBaselineScenarios(t *testing.T) {
	for name, run := range aiEvaluationScenarios {
		t.Run(name, func(t *testing.T) {
			h := newAIEvalHarness(t)
			run(h)
			checkGoldenAt(t, aiEvaluationGoldenDir, "scenario_"+name+".json", marshalGolden(t, h.steps))
		})
	}
}

func aiEvalConfig(config j, extra ...j) j {
	out := j{
		"application": aiEvalFakeApplication,
		"subsystem":   aiEvalFakeSubsystem,
		"target":      "response",
		"threshold":   0.5,
		"config":      config,
	}
	for _, e := range extra {
		for k, v := range e {
			out[k] = v
		}
	}
	return out
}

var aiEvaluationScenarios = map[string]func(h *grHarness){
	"minimal": func(h *grHarness) {
		s := h.Apply("create", h.null(), aiEvalConfig(j{"toxicity": j{}}))
		s = h.Refresh("refresh", s)
		h.Destroy("destroy", s)
	},

	"full-lifecycle": func(h *grHarness) {
		s := h.Apply("create", h.null(), aiEvalConfig(j{"allowed_topics": j{"topics": l{"billing", "returns"}}}, j{"is_enabled": false}))
		s = h.Refresh("refresh", s)
		s = h.Apply("update: topics, threshold, and is_enabled", s, aiEvalConfig(j{"allowed_topics": j{"topics": l{"returns", "shipping"}}}, j{"threshold": 0.9, "is_enabled": true}))
		s = h.Apply("update: switch to pii", s, aiEvalConfig(j{"pii": j{"categories": l{"EMAIL_ADDRESS", "US_SSN"}}}, j{"threshold": 0.9}))
		s = h.Apply("update: switch to an empty variant", s, aiEvalConfig(j{"sexism": j{}}, j{"threshold": 0.0}))
		h.Destroy("destroy", s)
	},

	"unset-is-enabled": func(h *grHarness) {
		s := h.Apply("create disabled", h.null(), aiEvalConfig(j{"toxicity": j{}}, j{"is_enabled": false}))
		s = h.Apply("update: remove is_enabled", s, aiEvalConfig(j{"toxicity": j{}}))
		h.Refresh("refresh", s)
	},

	"prompt-injection-context": func(h *grHarness) {
		s := h.Apply("create without additional_context", h.null(), aiEvalConfig(j{"prompt_injection": j{}}))
		s = h.Apply("update: set additional_context", s, aiEvalConfig(j{"prompt_injection": j{"additional_context": "an assistant for refunds"}}))
		s = h.Apply("update: remove additional_context", s, aiEvalConfig(j{"prompt_injection": j{}}))
		h.Refresh("refresh", s)
	},

	"every-config-variant": func(h *grHarness) {
		variants := []j{
			{"allowed_topics": j{"topics": l{"a"}}},
			{"competition": j{"competitors": l{"acme"}}},
			{"hallucination_completeness": j{}},
			{"hallucination_context_adherence": j{}},
			{"hallucination_context_relevance": j{}},
			{"hallucination_correctness": j{}},
			{"hallucination_task_adherence": j{}},
			{"language_mismatch": j{}},
			{"pii": j{"categories": l{"PHONE_NUMBER", "EMAIL_ADDRESS", "CREDIT_CARD", "IBAN_CODE", "US_SSN"}}},
			{"prompt_injection": j{"additional_context": "x"}},
			{"restricted_topics": j{"topics": l{"b"}}},
			{"sexism": j{}},
			{"sql_allowed_tables": j{"tables": l{"t1"}}},
			{"sql_hallucination": j{}},
			{"sql_read_only": j{}},
			{"sql_restricted_tables": j{"tables": l{"t2"}}},
			{"toxicity": j{}},
		}
		s := h.null()
		for _, v := range variants {
			for name := range v {
				s = h.Apply("apply: "+name, s, aiEvalConfig(v))
			}
		}
		h.Destroy("destroy", s)
	},

	"requires-replace": func(h *grHarness) {
		h.aiEvals().applications[aiEvalFakeApplication+"/other-subsystem"] = true
		s := h.Apply("create", h.null(), aiEvalConfig(j{"toxicity": j{}}))
		s = h.Apply("update: change target", s, aiEvalConfig(j{"toxicity": j{}}, j{"target": "prompt"}))
		h.Apply("update: change subsystem", s, aiEvalConfig(j{"toxicity": j{}}, j{"target": "prompt", "subsystem": "other-subsystem"}))
	},

	"config-validation": func(h *grHarness) {
		h.Validate("no config variant", aiEvalConfig(j{}))
		h.Validate("two config variants", aiEvalConfig(j{"toxicity": j{}, "sexism": j{}}))
		h.Validate("target conversation", aiEvalConfig(j{"toxicity": j{}}, j{"target": "conversation"}))
		h.Validate("target in upper case", aiEvalConfig(j{"toxicity": j{}}, j{"target": "RESPONSE"}))
		h.Validate("threshold above 1", aiEvalConfig(j{"toxicity": j{}}, j{"threshold": 1.5}))
		h.Validate("empty topics", aiEvalConfig(j{"allowed_topics": j{"topics": l{}}}))
		h.Validate("pii category in lower case", aiEvalConfig(j{"pii": j{"categories": l{"email_address"}}}))
		h.Validate("empty application", aiEvalConfig(j{"toxicity": j{}}, j{"application": ""}))
		h.Validate("missing threshold", j{"application": aiEvalFakeApplication, "subsystem": aiEvalFakeSubsystem, "target": "response", "config": j{"toxicity": j{}}})
	},

	"create-conflict": func(h *grHarness) {
		h.aiEvals().seed(j{"application": aiEvalFakeApplication, "subsystem": aiEvalFakeSubsystem, "target": "RESPONSE", "isEnabled": true, "config": j{"toxicity": j{}}})
		h.Apply("create: the same type and target exist", h.null(), aiEvalConfig(j{"toxicity": j{}}))
	},

	"unknown-application": func(h *grHarness) {
		h.Apply("create", h.null(), aiEvalConfig(j{"toxicity": j{}}, j{"application": "no-such-app"}))
	},

	"import": func(h *grHarness) {
		id := h.aiEvals().seed(j{
			"application": aiEvalFakeApplication, "subsystem": aiEvalFakeSubsystem, "target": "PROMPT",
			"isEnabled": false, "threshold": 0.3,
			"config": j{"restrictedTopics": j{"topics": l{"politics", "religion"}}},
		})
		s := h.Import("import", id)
		h.Apply("plan the matching config", s, aiEvalConfig(j{"restricted_topics": j{"topics": l{"religion", "politics"}}}, j{"target": "prompt", "threshold": 0.3, "is_enabled": false}))
	},

	"import-without-threshold": func(h *grHarness) {
		id := h.aiEvals().seed(j{
			"application": aiEvalFakeApplication, "subsystem": aiEvalFakeSubsystem, "target": "RESPONSE",
			"isEnabled": true, "config": j{"toxicity": j{}},
		})
		s := h.Import("import", id)
		h.Apply("plan with a threshold", s, aiEvalConfig(j{"toxicity": j{}}))
	},

	// A CONVERSATION target reads into state as "conversation", although a
	// configuration cannot set it. The fidelity case "create and read a
	// conversation evaluation" records that the API stores and returns it.
	"import-conversation-target": func(h *grHarness) {
		id := h.aiEvals().seed(j{
			"application": aiEvalFakeApplication, "subsystem": aiEvalFakeSubsystem, "target": "CONVERSATION",
			"isEnabled": true, "threshold": 0.5, "config": j{"toxicity": j{}},
		})
		s := h.Import("import", id)
		h.Refresh("refresh", s)
	},

	"import-not-a-uuid": func(h *grHarness) {
		h.Import("import an id that is not a uuid", "does-not-exist")
	},

	"import-unknown-id": func(h *grHarness) {
		h.Import("import an id that does not exist", "00000000-0000-4000-8000-999999999999")
	},

	"read-not-found": func(h *grHarness) {
		s := h.Apply("create", h.null(), aiEvalConfig(j{"toxicity": j{}}))
		h.aiEvals().forget(idOf(s))
		h.Refresh("refresh: evaluation was deleted outside Terraform", s)
	},

	"delete-not-found": func(h *grHarness) {
		s := h.Apply("create", h.null(), aiEvalConfig(j{"toxicity": j{}}))
		h.aiEvals().forget(idOf(s))
		h.Destroy("destroy: the API answers 404", s)
	},

	"update-not-found": func(h *grHarness) {
		s := h.Apply("create", h.null(), aiEvalConfig(j{"toxicity": j{}}))
		h.aiEvals().forget(idOf(s))
		h.Apply("update: evaluation was deleted outside Terraform", s, aiEvalConfig(j{"toxicity": j{}}, j{"threshold": 0.7}))
	},
}

// aiEvaluationResource returns the resource that the provider registers under
// coralogix_ai_evaluation, whichever package implements it.
func aiEvaluationResource() resource.Resource {
	p := NewCoralogixProvider()
	for _, newResource := range p.Resources(context.Background()) {
		r := newResource()
		var meta resource.MetadataResponse
		r.Metadata(context.Background(), resource.MetadataRequest{ProviderTypeName: "coralogix"}, &meta)
		if meta.TypeName == aiEvalTypeName {
			return r
		}
	}
	panic("the provider has no " + aiEvalTypeName)
}
