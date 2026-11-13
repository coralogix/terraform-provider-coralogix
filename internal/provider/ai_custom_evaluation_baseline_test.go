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

package provider

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"

	"github.com/coralogix/terraform-provider-coralogix/internal/provider/ai"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils/schemadump"
)

// These tests record the handwritten coralogix_ai_custom_evaluation resource against a fake API
// that follows answers recorded from the real API. After the switch to generated code, the golden
// diff must show only message wording or proven bug fixes.
//
//	UPDATE_GOLDEN=1 go test ./internal/provider -run 'TestAICustomEvaluationBaseline'
func newAICEHarness(t *testing.T) *grHarness {
	return newProtocolHarness(t, aiceTypeName, newAICEFake(t))
}

func TestAICustomEvaluationBaselineSchema(t *testing.T) {
	var current resource.SchemaResponse
	ai.NewAICustomEvaluationResource().Schema(context.Background(), resource.SchemaRequest{}, &current)
	checkGoldenAt(t, aiceGoldenDir, "schema_current.txt", []byte(schemadump.Text(current.Schema)))
}

func TestAICustomEvaluationBaselineScenarios(t *testing.T) {
	for name, run := range aiceScenarios {
		t.Run(name, func(t *testing.T) {
			h := newAICEHarness(t)
			run(h)
			checkGoldenAt(t, aiceGoldenDir, "scenario_"+name+".json", marshalGolden(t, h.steps))
		})
	}
}

// aiceConfig is the configuration of an evaluation with only the required attributes.
func aiceConfig(name string) j {
	return j{"name": name, "policy_type": "quality", "instructions": "Check {prompt}"}
}

// aiceWith is a copy of config with the attributes of changes. A nil value in changes removes the
// attribute.
func aiceWith(config j, changes j) j {
	out := j{}
	for key, value := range config {
		out[key] = value
	}
	for key, value := range changes {
		if value == nil {
			delete(out, key)
			continue
		}
		out[key] = value
	}
	return out
}

func aiceSel(application, subsystem string) j {
	return j{"application": application, "subsystem": subsystem}
}

// aiceCriteria is the criteria attribute with both criteria.
func aiceCriteria(acceptableFlags string, acceptable l, prohibitedFlags string, prohibited l) j {
	return j{
		"acceptable": j{"flags": acceptableFlags, "examples": acceptable},
		"prohibited": j{"flags": prohibitedFlags, "examples": prohibited},
	}
}

// aiceFull is the configuration of an evaluation with every attribute set.
func aiceFull(name string) j {
	return aiceWith(aiceConfig(name), j{
		"description":                  "made with Terraform",
		"instructions":                 "Check {prompt} and {response}",
		"should_include_system_prompt": true,
		"applications":                 l{aiceSel("checkout", "api")},
		"criteria":                     aiceCriteria("polite", l{"ok 1", "ok 2"}, "rude", l{"bad 1"}),
	})
}

func aiceExampleList(n int, prefix string) l {
	out := make(l, 0, n)
	for i := 1; i <= n; i++ {
		out = append(out, fmt.Sprintf("%s %d", prefix, i))
	}
	return out
}

// aiceApps adds the applications that the scenarios link to, and returns their ids.
func aiceApps(h *grHarness) (checkoutAPI, checkoutWeb, searchAPI string) {
	f := h.aice()
	return f.seedApp("checkout", "api"), f.seedApp("checkout", "web"), f.seedApp("search", "api")
}

var aiceScenarios = map[string]func(h *grHarness){
	"minimal-lifecycle": func(h *grHarness) {
		s := h.Apply("create with the required attributes", h.null(), aiceConfig("quality-check"))
		s = h.RefreshPlan("refresh and replan", s)
		s = h.Apply("update: rename", s, aiceConfig("quality-check-renamed"))
		s = h.Apply("update: change the instructions", s, aiceWith(aiceConfig("quality-check-renamed"), j{"instructions": "Judge {response}"}))
		s = h.Apply("update: change the policy type", s, aiceWith(aiceConfig("quality-check-renamed"), j{"instructions": "Judge {response}", "policy_type": "security"}))
		s = h.RefreshPlan("refresh and replan after update", s)
		h.Destroy("destroy", s)
	},
	"full-lifecycle": func(h *grHarness) {
		aiceApps(h)
		s := h.Apply("create with every attribute", h.null(), aiceFull("full-check"))
		s = h.RefreshPlan("refresh and replan", s)
		s = h.Apply("update: rename", s, aiceFull("full-check-renamed"))
		s = h.Apply("update: change the description", s, aiceWith(aiceFull("full-check-renamed"), j{"description": "changed"}))
		s = h.Apply("update: change the instructions", s, aiceWith(aiceFull("full-check-renamed"), j{"description": "changed", "instructions": "Judge {chat_history}"}))
		s = h.Apply("update: turn the system prompt off", s, aiceWith(aiceFull("full-check-renamed"), j{"description": "changed", "instructions": "Judge {chat_history}", "should_include_system_prompt": false}))
		s = h.RefreshPlan("refresh and replan after update", s)
		h.Destroy("destroy", s)
	},
	"no-change-apply": func(h *grHarness) {
		aiceApps(h)
		s := h.Apply("create", h.null(), aiceFull("same"))
		h.Apply("apply the same configuration", s, aiceFull("same"))
	},

	// policy_type
	"policy-type-security": func(h *grHarness) {
		s := h.Apply("create a security evaluation", h.null(), aiceWith(aiceConfig("sec"), j{"policy_type": "security"}))
		h.RefreshPlan("refresh and replan", s)
	},
	"policy-type-invalid": func(h *grHarness) {
		h.Validate("validate an unknown policy type", aiceWith(aiceConfig("n"), j{"policy_type": "bogus"}))
	},
	"policy-type-upper-case": func(h *grHarness) {
		h.Validate("validate a policy type in upper case", aiceWith(aiceConfig("n"), j{"policy_type": "QUALITY"}))
	},
	"policy-type-empty": func(h *grHarness) {
		h.Validate("validate an empty policy type", aiceWith(aiceConfig("n"), j{"policy_type": ""}))
	},
	"policy-type-missing": func(h *grHarness) {
		h.Validate("validate without policy_type", aiceWith(aiceConfig("n"), j{"policy_type": nil}))
	},
	"policy-type-changed-outside": func(h *grHarness) {
		s := h.Apply("create", h.null(), aiceConfig("outside"))
		h.aice().evaluation(aiceFirstEvalID).policyType = "other"
		s = h.RefreshPlan("refresh: the API has another policy type", s)
		h.Apply("apply the configuration again", s, aiceConfig("outside"))
	},

	// name
	"name-empty": func(h *grHarness) {
		h.Validate("validate an empty name", aiceConfig(""))
	},
	"name-too-long": func(h *grHarness) {
		h.Validate("validate a name of 257 characters", aiceConfig(strings.Repeat("a", 257)))
	},
	"name-longest": func(h *grHarness) {
		s := h.Apply("create with a name of 256 characters", h.null(), aiceConfig(strings.Repeat("a", 256)))
		h.RefreshPlan("refresh and replan", s)
	},
	"name-missing": func(h *grHarness) {
		h.Validate("validate without name", aiceWith(aiceConfig("n"), j{"name": nil}))
	},
	"name-duplicate": func(h *grHarness) {
		h.aice().seedEvaluation("taken")
		h.Apply("create with a name that another evaluation has", h.null(), aiceConfig("taken"))
	},
	"rename-to-taken-name": func(h *grHarness) {
		h.aice().seedEvaluation("taken")
		s := h.Apply("create", h.null(), aiceConfig("mine"))
		h.Apply("update: rename to a name that another evaluation has", s, aiceConfig("taken"))
	},
	"name-changed-outside": func(h *grHarness) {
		s := h.Apply("create", h.null(), aiceConfig("outside"))
		h.aice().evaluation(aiceFirstEvalID).name = "changed in the UI"
		s = h.RefreshPlan("refresh: the API has another name", s)
		h.Apply("apply the configuration again", s, aiceConfig("outside"))
	},

	// instructions
	"instructions-missing": func(h *grHarness) {
		h.Validate("validate without instructions", aiceWith(aiceConfig("n"), j{"instructions": nil}))
	},
	"instructions-empty": func(h *grHarness) {
		h.Validate("validate empty instructions", aiceWith(aiceConfig("n"), j{"instructions": ""}))
	},
	"instructions-without-placeholder": func(h *grHarness) {
		h.Validate("validate instructions without a placeholder", aiceWith(aiceConfig("n"), j{"instructions": "Be nice"}))
		h.Validate("validate instructions with an unknown placeholder", aiceWith(aiceConfig("n"), j{"instructions": "Check {text}"}))
		h.Validate("validate instructions with a placeholder in upper case", aiceWith(aiceConfig("n"), j{"instructions": "Check {PROMPT}"}))
	},
	"instructions-placeholders": func(h *grHarness) {
		h.Validate("validate {prompt}", aiceWith(aiceConfig("n"), j{"instructions": "Check {prompt}"}))
		h.Validate("validate {response}", aiceWith(aiceConfig("n"), j{"instructions": "Check {response}"}))
		h.Validate("validate {chat_history}", aiceWith(aiceConfig("n"), j{"instructions": "Check {chat_history}"}))
		s := h.Apply("create with the placeholder inside a long text", h.null(), aiceWith(aiceConfig("n"), j{"instructions": "A\nB {response} C\n"}))
		h.RefreshPlan("refresh and replan", s)
	},
	"instructions-changed-outside": func(h *grHarness) {
		s := h.Apply("create", h.null(), aiceConfig("outside"))
		h.aice().evaluation(aiceFirstEvalID).instructions = "Changed {prompt}"
		s = h.RefreshPlan("refresh: the API has other instructions", s)
		h.Apply("apply the configuration again", s, aiceConfig("outside"))
	},

	// description
	"description-omitted": func(h *grHarness) {
		s := h.Apply("create without description", h.null(), aiceConfig("n"))
		h.RefreshPlan("refresh and replan", s)
	},
	"description-removed-on-update": func(h *grHarness) {
		s := h.Apply("create with a description", h.null(), aiceWith(aiceConfig("n"), j{"description": "something"}))
		s = h.Apply("update: remove the description", s, aiceConfig("n"))
		h.RefreshPlan("refresh and replan", s)
	},
	"description-empty-string": func(h *grHarness) {
		s := h.Apply("create with an empty description", h.null(), aiceWith(aiceConfig("n"), j{"description": ""}))
		h.RefreshPlan("refresh and replan", s)
	},
	"description-changed-outside": func(h *grHarness) {
		s := h.Apply("create", h.null(), aiceWith(aiceConfig("n"), j{"description": "mine"}))
		h.aice().evaluation(aiceFirstEvalID).description = "changed in the UI"
		s = h.RefreshPlan("refresh: the API has another description", s)
		h.Apply("apply the configuration again", s, aiceWith(aiceConfig("n"), j{"description": "mine"}))
	},

	// should_include_system_prompt
	"system-prompt-omitted": func(h *grHarness) {
		s := h.Apply("create without should_include_system_prompt", h.null(), aiceConfig("n"))
		h.RefreshPlan("refresh and replan", s)
	},
	"system-prompt-true-then-removed": func(h *grHarness) {
		s := h.Apply("create with should_include_system_prompt", h.null(), aiceWith(aiceConfig("n"), j{"should_include_system_prompt": true}))
		s = h.Apply("update: remove should_include_system_prompt", s, aiceConfig("n"))
		h.RefreshPlan("refresh and replan", s)
	},
	"system-prompt-changed-outside": func(h *grHarness) {
		s := h.Apply("create", h.null(), aiceWith(aiceConfig("n"), j{"should_include_system_prompt": true}))
		h.aice().evaluation(aiceFirstEvalID).shouldIncludeSystemPrompt = false
		s = h.RefreshPlan("refresh: the API has the flag off", s)
		h.Apply("apply the configuration again", s, aiceWith(aiceConfig("n"), j{"should_include_system_prompt": true}))
	},

	// applications
	// The evaluation has no applications, so Create does not list them.
	"applications-omitted": func(h *grHarness) {
		aiceApps(h)
		s := h.Apply("create without applications", h.null(), aiceConfig("n"))
		h.RefreshPlan("refresh and replan", s)
	},
	"applications-empty-set": func(h *grHarness) {
		aiceApps(h)
		s := h.Apply("create with an empty applications set", h.null(), aiceWith(aiceConfig("n"), j{"applications": l{}}))
		h.RefreshPlan("refresh and replan", s)
	},
	"applications-one": func(h *grHarness) {
		aiceApps(h)
		s := h.Apply("create with one application", h.null(), aiceWith(aiceConfig("n"), j{"applications": l{aiceSel("checkout", "api")}}))
		s = h.RefreshPlan("refresh and replan: the stored selectors are kept", s)
		h.Apply("apply the same configuration", s, aiceWith(aiceConfig("n"), j{"applications": l{aiceSel("checkout", "api")}}))
	},
	"applications-multiple": func(h *grHarness) {
		aiceApps(h)
		s := h.Apply("create with three applications", h.null(), aiceWith(aiceConfig("n"), j{"applications": l{aiceSel("search", "api"), aiceSel("checkout", "web"), aiceSel("checkout", "api")}}))
		h.RefreshPlan("refresh and replan", s)
	},
	"applications-added-on-update": func(h *grHarness) {
		aiceApps(h)
		s := h.Apply("create without applications", h.null(), aiceConfig("n"))
		s = h.Apply("update: add one application", s, aiceWith(aiceConfig("n"), j{"applications": l{aiceSel("checkout", "api")}}))
		s = h.Apply("update: add a second application", s, aiceWith(aiceConfig("n"), j{"applications": l{aiceSel("checkout", "api"), aiceSel("search", "api")}}))
		h.RefreshPlan("refresh and replan", s)
	},
	"applications-removed-on-update": func(h *grHarness) {
		aiceApps(h)
		s := h.Apply("create with two applications", h.null(), aiceWith(aiceConfig("n"), j{"applications": l{aiceSel("checkout", "api"), aiceSel("search", "api")}}))
		s = h.Apply("update: remove one application", s, aiceWith(aiceConfig("n"), j{"applications": l{aiceSel("search", "api")}}))
		s = h.Apply("update: remove the attribute", s, aiceConfig("n"))
		h.RefreshPlan("refresh and replan", s)
	},
	"applications-replaced": func(h *grHarness) {
		aiceApps(h)
		s := h.Apply("create", h.null(), aiceWith(aiceConfig("n"), j{"applications": l{aiceSel("checkout", "api")}}))
		s = h.Apply("update: another application", s, aiceWith(aiceConfig("n"), j{"applications": l{aiceSel("checkout", "web")}}))
		h.RefreshPlan("refresh and replan", s)
	},
	"applications-not-found": func(h *grHarness) {
		aiceApps(h)
		h.Apply("create with an application that does not exist", h.null(), aiceWith(aiceConfig("n"), j{"applications": l{aiceSel("nope", "api")}}))
	},
	"applications-subsystem-not-found": func(h *grHarness) {
		aiceApps(h)
		h.Apply("create with an application and another subsystem", h.null(), aiceWith(aiceConfig("n"), j{"applications": l{aiceSel("checkout", "nope")}}))
	},
	"applications-not-found-on-update": func(h *grHarness) {
		aiceApps(h)
		s := h.Apply("create", h.null(), aiceConfig("n"))
		h.Apply("update: add an application that does not exist", s, aiceWith(aiceConfig("n"), j{"applications": l{aiceSel("nope", "api")}}))
	},
	"applications-ambiguous": func(h *grHarness) {
		aiceApps(h)
		h.aice().seedApp("checkout", "api")
		h.Apply("create with a selector that two applications match", h.null(), aiceWith(aiceConfig("n"), j{"applications": l{aiceSel("checkout", "api")}}))
	},
	"applications-subsystem-empty": func(h *grHarness) {
		h.Validate("validate an empty subsystem", aiceWith(aiceConfig("n"), j{"applications": l{aiceSel("checkout", "")}}))
	},
	"applications-application-empty": func(h *grHarness) {
		h.Validate("validate an empty application", aiceWith(aiceConfig("n"), j{"applications": l{aiceSel("", "api")}}))
	},
	"applications-subsystem-missing": func(h *grHarness) {
		h.Validate("validate a selector without subsystem", aiceWith(aiceConfig("n"), j{"applications": l{j{"application": "checkout"}}}))
	},
	"applications-duplicate-selector": func(h *grHarness) {
		aiceApps(h)
		h.Apply("create with the same selector twice", h.null(), aiceWith(aiceConfig("n"), j{"applications": l{aiceSel("checkout", "api"), aiceSel("checkout", "api")}}))
	},
	// The applications are listed in pages of 200. The application is on the second page.
	"applications-second-page": func(h *grHarness) {
		for i := 0; i < 230; i++ {
			h.aice().seedApp(fmt.Sprintf("filler-%03d", i), "api")
		}
		h.aice().seedApp("late", "api")
		s := h.Apply("create with an application on the second page", h.null(), aiceWith(aiceConfig("n"), j{"applications": l{aiceSel("late", "api")}}))
		h.RefreshPlan("refresh and replan", s)
	},
	// The page is full and the next page is empty.
	"applications-exactly-one-page": func(h *grHarness) {
		for i := 0; i < 199; i++ {
			h.aice().seedApp(fmt.Sprintf("filler-%03d", i), "api")
		}
		h.aice().seedApp("last", "api")
		h.Apply("create with the last application of a full page", h.null(), aiceWith(aiceConfig("n"), j{"applications": l{aiceSel("last", "api")}}))
	},
	"applications-list-error": func(h *grHarness) {
		aiceApps(h)
		h.aice().appsStatus = 500
		s := h.Apply("create: the application list answers 500", h.null(), aiceWith(aiceConfig("n"), j{"applications": l{aiceSel("checkout", "api")}}))
		h.aice().appsStatus = 0
		s = h.Apply("create without the applications", s, aiceConfig("n"))
		h.aice().appsStatus = 500
		h.Apply("update: the application list answers 500", s, aiceWith(aiceConfig("n"), j{"applications": l{aiceSel("checkout", "api")}}))
	},
	// A link made outside Terraform: the ids differ, so the refresh resolves the selectors again.
	"applications-linked-outside": func(h *grHarness) {
		_, web, _ := aiceApps(h)
		s := h.Apply("create without applications", h.null(), aiceConfig("n"))
		h.aice().evaluation(aiceFirstEvalID).appIDs = []string{web}
		s = h.RefreshPlan("refresh: the API has a link", s)
		h.Apply("apply the configuration again", s, aiceConfig("n"))
	},
	"applications-unlinked-outside": func(h *grHarness) {
		aiceApps(h)
		cfg := aiceWith(aiceConfig("n"), j{"applications": l{aiceSel("checkout", "api"), aiceSel("search", "api")}})
		s := h.Apply("create with two applications", h.null(), cfg)
		evaluation := h.aice().evaluation(aiceFirstEvalID)
		evaluation.appIDs = evaluation.appIDs[:1]
		s = h.RefreshPlan("refresh: the API lost a link", s)
		h.Apply("apply the configuration again", s, cfg)
	},
	// The link points at an application that was deleted: the refresh cannot name it.
	"applications-deleted-outside": func(h *grHarness) {
		api, _, _ := aiceApps(h)
		cfg := aiceWith(aiceConfig("n"), j{"applications": l{aiceSel("checkout", "api"), aiceSel("search", "api")}})
		s := h.Apply("create with two applications", h.null(), cfg)
		h.aice().forgetApp(api)
		h.Refresh("refresh: an application is gone", s)
	},
	"applications-list-error-on-refresh": func(h *grHarness) {
		_, web, _ := aiceApps(h)
		s := h.Apply("create without applications", h.null(), aiceConfig("n"))
		h.aice().evaluation(aiceFirstEvalID).appIDs = []string{web}
		h.aice().appsStatus = 500
		h.Refresh("refresh: a link changed and the application list answers 500", s)
	},
	"update-link-fails": func(h *grHarness) {
		aiceApps(h)
		s := h.Apply("create", h.null(), aiceConfig("n"))
		h.aice().linkStatus = 500
		s = h.Apply("update: the name and an application, and the link answers 500", s, aiceWith(aiceConfig("n-renamed"), j{"applications": l{aiceSel("checkout", "api")}}))
		h.aice().linkStatus = 0
		h.RefreshPlan("refresh after the failed update", s)
	},
	"update-unlink-fails": func(h *grHarness) {
		aiceApps(h)
		s := h.Apply("create with an application", h.null(), aiceWith(aiceConfig("n"), j{"applications": l{aiceSel("checkout", "api")}}))
		h.aice().unlinkStatus = 500
		s = h.Apply("update: the name and no application, and the unlink answers 500", s, aiceConfig("n-renamed"))
		h.aice().unlinkStatus = 0
		h.RefreshPlan("refresh after the failed update", s)
	},
	// The state does not know a link that exists: the apply links again and the API refuses.
	"update-link-already-exists": func(h *grHarness) {
		_, web, _ := aiceApps(h)
		s := h.Apply("create", h.null(), aiceConfig("n"))
		h.aice().evaluation(aiceFirstEvalID).appIDs = []string{web}
		h.Apply("update: add the application that is linked already (no refresh)", s, aiceWith(aiceConfig("n"), j{"applications": l{aiceSel("checkout", "web")}}))
	},
	"application-ids-in-config": func(h *grHarness) {
		h.Validate("validate a configured application_ids", aiceWith(aiceConfig("n"), j{"application_ids": l{"00000000-0000-4000-8000-000000002001"}}))
	},
	"id-in-config": func(h *grHarness) {
		h.Validate("validate a configured id", aiceWith(aiceConfig("n"), j{"id": "00000000-0000-4000-8000-000000001001"}))
	},

	// criteria
	"criteria-omitted": func(h *grHarness) {
		s := h.Apply("create without criteria", h.null(), aiceConfig("n"))
		h.RefreshPlan("refresh and replan", s)
	},
	"criteria-empty-object": func(h *grHarness) {
		s := h.Apply("create with an empty criteria object", h.null(), aiceWith(aiceConfig("n"), j{"criteria": j{}}))
		h.RefreshPlan("refresh and replan", s)
	},
	"criteria-acceptable-only": func(h *grHarness) {
		s := h.Apply("create with acceptable criteria only", h.null(), aiceWith(aiceConfig("n"), j{"criteria": j{"acceptable": j{"flags": "polite", "examples": l{"ok"}}}}))
		h.RefreshPlan("refresh and replan", s)
	},
	"criteria-prohibited-only": func(h *grHarness) {
		s := h.Apply("create with prohibited criteria only", h.null(), aiceWith(aiceConfig("n"), j{"criteria": j{"prohibited": j{"flags": "rude", "examples": l{"bad"}}}}))
		h.RefreshPlan("refresh and replan", s)
	},
	"criteria-flags-only": func(h *grHarness) {
		s := h.Apply("create with flags and no examples", h.null(), aiceWith(aiceConfig("n"), j{"criteria": j{"acceptable": j{"flags": "polite"}, "prohibited": j{"flags": "rude"}}}))
		h.RefreshPlan("refresh and replan", s)
	},
	"criteria-examples-only": func(h *grHarness) {
		s := h.Apply("create with examples and no flags", h.null(), aiceWith(aiceConfig("n"), j{"criteria": j{"acceptable": j{"examples": l{"ok"}}, "prohibited": j{"examples": l{"bad"}}}}))
		h.RefreshPlan("refresh and replan", s)
	},
	"criteria-examples-reordered": func(h *grHarness) {
		s := h.Apply("create", h.null(), aiceWith(aiceConfig("n"), j{"criteria": aiceCriteria("", l{"a", "b", "c"}, "", l{"x", "y"})}))
		s = h.Apply("update: reverse the examples", s, aiceWith(aiceConfig("n"), j{"criteria": aiceCriteria("", l{"c", "b", "a"}, "", l{"y", "x"})}))
		h.RefreshPlan("refresh and replan", s)
	},
	"criteria-examples-duplicates": func(h *grHarness) {
		s := h.Apply("create with the same example twice", h.null(), aiceWith(aiceConfig("n"), j{"criteria": aiceCriteria("", l{"same", "same"}, "", l{"same"})}))
		h.RefreshPlan("refresh and replan", s)
	},
	"criteria-example-empty-string": func(h *grHarness) {
		h.Validate("validate an empty example", aiceWith(aiceConfig("n"), j{"criteria": aiceCriteria("", l{""}, "", l{})}))
	},
	"criteria-too-many-examples-in-list": func(h *grHarness) {
		h.Validate("validate 101 acceptable examples", aiceWith(aiceConfig("n"), j{"criteria": aiceCriteria("", aiceExampleList(101, "ok"), "", l{})}))
	},
	"criteria-too-many-examples-total": func(h *grHarness) {
		h.Apply("create with 60 acceptable and 60 prohibited examples", h.null(), aiceWith(aiceConfig("n"), j{"criteria": aiceCriteria("", aiceExampleList(60, "ok"), "", aiceExampleList(60, "bad"))}))
	},
	"criteria-examples-limit": func(h *grHarness) {
		s := h.Apply("create with 60 acceptable and 40 prohibited examples", h.null(), aiceWith(aiceConfig("n"), j{"criteria": aiceCriteria("", aiceExampleList(60, "ok"), "", aiceExampleList(40, "bad"))}))
		h.RefreshPlan("refresh and replan", s)
	},
	// Removing the criteria sends empty examples, which Update ignores, so a second call clears them.
	"criteria-removed-on-update": func(h *grHarness) {
		s := h.Apply("create with criteria", h.null(), aiceWith(aiceConfig("n"), j{"criteria": aiceCriteria("polite", l{"ok"}, "rude", l{"bad"})}))
		s = h.Apply("update: remove the criteria", s, aiceConfig("n"))
		h.RefreshPlan("refresh and replan", s)
	},
	"criteria-acceptable-removed-on-update": func(h *grHarness) {
		s := h.Apply("create with criteria", h.null(), aiceWith(aiceConfig("n"), j{"criteria": aiceCriteria("polite", l{"ok"}, "rude", l{"bad"})}))
		s = h.Apply("update: remove the acceptable criteria", s, aiceWith(aiceConfig("n"), j{"criteria": j{"prohibited": j{"flags": "rude", "examples": l{"bad"}}}}))
		h.RefreshPlan("refresh and replan", s)
	},
	"criteria-flags-removed-on-update": func(h *grHarness) {
		s := h.Apply("create with flags", h.null(), aiceWith(aiceConfig("n"), j{"criteria": aiceCriteria("polite", l{"ok"}, "rude", l{"bad"})}))
		s = h.Apply("update: remove the flags", s, aiceWith(aiceConfig("n"), j{"criteria": aiceCriteria("", l{"ok"}, "", l{"bad"})}))
		s = h.Apply("update: omit the flags", s, aiceWith(aiceConfig("n"), j{"criteria": j{"acceptable": j{"examples": l{"ok"}}, "prohibited": j{"examples": l{"bad"}}}}))
		h.RefreshPlan("refresh and replan", s)
	},
	"criteria-examples-cleared-on-update": func(h *grHarness) {
		s := h.Apply("create with examples", h.null(), aiceWith(aiceConfig("n"), j{"criteria": aiceCriteria("polite", l{"ok"}, "rude", l{"bad"})}))
		s = h.Apply("update: empty example lists", s, aiceWith(aiceConfig("n"), j{"criteria": aiceCriteria("polite", l{}, "rude", l{})}))
		h.RefreshPlan("refresh and replan", s)
	},
	"criteria-values-changed": func(h *grHarness) {
		s := h.Apply("create", h.null(), aiceWith(aiceConfig("n"), j{"criteria": aiceCriteria("polite", l{"ok 1", "ok 2"}, "rude", l{"bad 1"})}))
		s = h.Apply("update: change the flags", s, aiceWith(aiceConfig("n"), j{"criteria": aiceCriteria("friendly", l{"ok 1", "ok 2"}, "hostile", l{"bad 1"})}))
		s = h.Apply("update: change one example of each list", s, aiceWith(aiceConfig("n"), j{"criteria": aiceCriteria("friendly", l{"ok 1", "ok 2 changed"}, "hostile", l{"bad 1 changed"})}))
		s = h.Apply("update: add an example to each list", s, aiceWith(aiceConfig("n"), j{"criteria": aiceCriteria("friendly", l{"ok 1", "ok 2 changed", "ok 3"}, "hostile", l{"bad 1 changed", "bad 2"})}))
		h.RefreshPlan("refresh and replan", s)
	},
	"criteria-examples-moved-between-lists": func(h *grHarness) {
		s := h.Apply("create", h.null(), aiceWith(aiceConfig("n"), j{"criteria": aiceCriteria("", l{"a"}, "", l{"b"})}))
		s = h.Apply("update: swap the examples", s, aiceWith(aiceConfig("n"), j{"criteria": aiceCriteria("", l{"b"}, "", l{"a"})}))
		h.RefreshPlan("refresh and replan", s)
	},
	// The released resource cleared the examples with a second call. One call with a mask does it.
	"criteria-examples-cleared-in-one-call": func(h *grHarness) {
		s := h.Apply("create with examples", h.null(), aiceWith(aiceConfig("n"), j{"criteria": aiceCriteria("", l{"ok"}, "", l{})}))
		s = h.Apply("update: rename and remove the examples", s, aiceWith(aiceConfig("n-renamed"), j{}))
		h.RefreshPlan("refresh and replan", s)
	},
	"criteria-examples-changed-outside": func(h *grHarness) {
		cfg := aiceWith(aiceConfig("n"), j{"criteria": aiceCriteria("", l{"ok"}, "", l{"bad"})})
		s := h.Apply("create", h.null(), cfg)
		h.aice().evaluation(aiceFirstEvalID).examples = []map[string]any{{"conversation": "other", "score": "1"}}
		s = h.RefreshPlan("refresh: the API has other examples", s)
		h.Apply("apply the configuration again", s, cfg)
	},
	"criteria-flags-changed-outside": func(h *grHarness) {
		cfg := aiceWith(aiceConfig("n"), j{"criteria": aiceCriteria("polite", l{}, "rude", l{})})
		s := h.Apply("create", h.null(), cfg)
		evaluation := h.aice().evaluation(aiceFirstEvalID)
		evaluation.safe, evaluation.violates = "changed", "changed"
		s = h.RefreshPlan("refresh: the API has other flags", s)
		h.Apply("apply the configuration again", s, cfg)
	},
	// The API accepts any score. The resource reads only 1 and 0.
	"criteria-score-unexpected": func(h *grHarness) {
		s := h.Apply("create", h.null(), aiceConfig("n"))
		h.aice().evaluation(aiceFirstEvalID).examples = []map[string]any{{"conversation": "odd", "score": "7"}}
		h.Refresh("refresh: an example has the score 7", s)
	},

	// API errors
	"create-api-error": func(h *grHarness) {
		h.aice().createStatus = 500
		h.Apply("create: the API answers 500", h.null(), aiceConfig("n"))
	},
	"update-api-error": func(h *grHarness) {
		s := h.Apply("create", h.null(), aiceConfig("n"))
		h.aice().updateScript = []int{500}
		h.Apply("update: the API answers 500", s, aiceConfig("n-renamed"))
	},
	"update-not-found": func(h *grHarness) {
		s := h.Apply("create", h.null(), aiceConfig("gone"))
		h.aice().forgetOutside(aiceFirstEvalID)
		h.Apply("update: the API answers 404", s, aiceConfig("gone-renamed"))
	},
	"read-not-found": func(h *grHarness) {
		s := h.Apply("create", h.null(), aiceConfig("gone"))
		h.aice().forgetOutside(aiceFirstEvalID)
		h.Refresh("refresh: the API does not find the evaluation", s)
	},
	"read-api-error": func(h *grHarness) {
		s := h.Apply("create", h.null(), aiceConfig("broken"))
		h.aice().getStatus = 500
		h.Refresh("refresh: the read answers 500", s)
	},
	"read-route-not-found": func(h *grHarness) {
		s := h.Apply("create", h.null(), aiceConfig("n"))
		h.aice().getStatus = 404
		h.Refresh("refresh: the route answers 404", s)
	},
	"delete-not-found": func(h *grHarness) {
		s := h.Apply("create", h.null(), aiceConfig("gone"))
		h.aice().forgetOutside(aiceFirstEvalID)
		h.Destroy("destroy: the API answers 404", s)
	},
	// A 404 with no message of its own is a missing route, not a deleted evaluation.
	"delete-route-not-found": func(h *grHarness) {
		s := h.Apply("create", h.null(), aiceConfig("n"))
		h.aice().deleteStatus = 404
		h.Destroy("destroy: the route answers 404", s)
	},
	"delete-api-error": func(h *grHarness) {
		s := h.Apply("create", h.null(), aiceConfig("broken"))
		h.aice().deleteStatus = 500
		h.Destroy("destroy: the API answers 500", s)
	},
	"delete-with-applications": func(h *grHarness) {
		aiceApps(h)
		s := h.Apply("create with an application", h.null(), aiceWith(aiceConfig("n"), j{"applications": l{aiceSel("checkout", "api")}}))
		h.Destroy("destroy", s)
	},

	// import
	"import": func(h *grHarness) {
		id := h.aice().seedEvaluation("imported")
		s := h.Import("import", id)
		h.Apply("plan the matching config", s, aiceWith(aiceConfig("imported"), j{"description": "seeded"}))
	},
	"import-with-applications": func(h *grHarness) {
		api, web, _ := aiceApps(h)
		id := h.aice().seedEvaluation("imported", web, api)
		s := h.Import("import an evaluation that is linked to two applications", id)
		h.Apply("plan the matching config", s, aiceWith(aiceConfig("imported"), j{"description": "seeded", "applications": l{aiceSel("checkout", "api"), aiceSel("checkout", "web")}}))
	},
	"import-with-criteria": func(h *grHarness) {
		id := h.aice().seedEvaluation("imported")
		evaluation := h.aice().evaluation(id)
		evaluation.safe, evaluation.violates = "polite", "rude"
		evaluation.examples = []map[string]any{{"conversation": "ok 1", "score": "1"}, {"conversation": "bad 1", "score": "0"}, {"conversation": "ok 2", "score": "1"}}
		s := h.Import("import an evaluation with criteria", id)
		h.Apply("plan the matching config", s, aiceWith(aiceConfig("imported"), j{"description": "seeded", "criteria": aiceCriteria("polite", l{"ok 1", "ok 2"}, "rude", l{"bad 1"})}))
	},
	"import-then-update": func(h *grHarness) {
		_, web, _ := aiceApps(h)
		id := h.aice().seedEvaluation("imported", web)
		s := h.Import("import", id)
		h.Apply("update to another configuration", s, aiceFull("imported"))
	},
	"import-not-found": func(h *grHarness) {
		h.Import("import an id that does not exist", aiceID(9999))
	},
	"import-not-a-uuid": func(h *grHarness) {
		h.Import("import an id that is no UUID", "not-a-uuid")
	},
	"import-api-error": func(h *grHarness) {
		id := h.aice().seedEvaluation("imported")
		h.aice().getStatus = 500
		h.Import("import: the read answers 500", id)
	},
	"import-score-unexpected": func(h *grHarness) {
		id := h.aice().seedEvaluation("imported")
		h.aice().evaluation(id).examples = []map[string]any{{"conversation": "odd", "score": "7"}}
		h.Import("import an evaluation with the score 7", id)
	},

	// unknown values
	"unknown-values-plan": func(h *grHarness) {
		h.Plan("plan with unknown values", h.null(), j{
			"name": unknownJSON{}, "policy_type": "quality", "instructions": unknownJSON{}, "description": unknownJSON{},
			"should_include_system_prompt": unknownJSON{},
		})
	},
	"unknown-applications-plan": func(h *grHarness) {
		h.Plan("plan with an unknown selector", h.null(), aiceWith(aiceConfig("n"), j{"applications": l{j{"application": "checkout", "subsystem": unknownJSON{}}}}))
		h.Plan("plan with an unknown criteria", h.null(), aiceWith(aiceConfig("n"), j{"criteria": j{"acceptable": j{"flags": unknownJSON{}, "examples": unknownJSON{}}}}))
	},
}
