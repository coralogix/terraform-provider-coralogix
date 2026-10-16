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
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
)

const quotaTypeName = "coralogix_quota_allocation_rule_set"

const quotaBasePath = "/dataplan/quota-rules/v1"

func newQuotaHarness(t *testing.T) *grHarness {
	return newProtocolHarness(t, quotaTypeName, newQuotaFake(t))
}

func (h *grHarness) quota() *quotaFake {
	h.t.Helper()
	f, ok := h.api.(*quotaFake)
	if !ok {
		h.t.Fatal("expected *quotaFake")
	}
	return f
}

// quotaFake follows the quota allocation service as recorded in fidelity/recorded.json, not the
// Go client. The account always has one rule set: a read answers it, empty, before anything is
// written, and a delete clears the rules of the customer and keeps the set. It holds the rules
// that Coralogix manages (cxManaged: true) next to the rules of the customer, and every rule has
// cxManaged, false for the customer. A create or a replace replaces the customer rules and keeps
// the managed ones. The server sets the id, stores every allocation as a float32, stores a rule with
// no allocationType, or an unspecified one, as percentage, ignores the id query, and answers a
// write in the order of the request and a read by entity type.
//
// Two answers are not recorded, because the tenant has no Coralogix-managed rules and the API
// never answered 404: a create next to managed rules answers 409, as the provider expects, and
// notFound makes a read, replace, or delete answer 404, as the contract documents.
type quotaFake struct {
	mu       sync.Mutex
	set      map[string]any
	seq      int
	requests []grRequest
	// deleteStatus is the status of a delete that fails. 0 means that a delete works.
	// A failed delete still clears the customer rules when deleteClears is true.
	deleteStatus int
	deleteClears bool
	// notFound makes a read, replace, or delete answer 404. The recording never shows it.
	notFound bool
	// writeStatus is the status of a create or replace that fails. 0 means that a write works.
	writeStatus int
}

func newQuotaFake(t *testing.T) *quotaFake {
	t.Helper()
	// The real tenant always has a rule set: a GET answers it, empty, before anything is written.
	f := &quotaFake{set: map[string]any{"id": "00000000-0000-4000-8000-000000000001", "rules": []any{}}, seq: 1}
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)

	target, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	original := http.DefaultTransport
	http.DefaultTransport = redirectTransport{target: target, next: original}
	t.Cleanup(func() { http.DefaultTransport = original })
	return f
}

// seedManaged makes a rule set that holds rules that Coralogix manages, and nothing else.
func (f *quotaFake) seedManaged(rules ...map[string]any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.seq++
	stored := []any{}
	for _, r := range rules {
		r["cxManaged"] = true
		stored = append(stored, r)
	}
	f.set = map[string]any{"id": fmt.Sprintf("00000000-0000-4000-8000-%012d", f.seq), "rules": stored}
}

// seedCustomer adds customer rules to the rule set, or makes the rule set.
func (f *quotaFake) seedCustomer(rules ...map[string]any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.set == nil {
		f.seq++
		f.set = map[string]any{"id": fmt.Sprintf("00000000-0000-4000-8000-%012d", f.seq), "rules": []any{}}
	}
	for _, r := range rules {
		f.set["rules"] = append(f.set["rules"].([]any), quotaStoreRule(r))
	}
}

// clearOutside clears the rules of the customer, as if somebody deleted the rule set outside
// Terraform: the rule set stays and reads back empty.
func (f *quotaFake) clearOutside() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.set["rules"] = quotaManagedOnly(f.set["rules"].([]any))
}

func (f *quotaFake) takeRequests() []grRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := f.requests
	f.requests = nil
	return out
}

func (f *quotaFake) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()

	raw, _ := io.ReadAll(r.Body)
	rec := grRequest{Method: r.Method, Path: r.URL.Path}
	if q := r.URL.RawQuery; q != "" {
		rec.Path += "?" + q
	}
	rec.Body = rawRequestBody(raw)
	status, body := f.handle(r.Method, r.URL.Path, r.URL.Query().Get("id"), rec.Body)
	rec.Status = status
	f.requests = append(f.requests, rec)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func (f *quotaFake) handle(method, path, id string, body any) (int, any) {
	if _, rest, ok := strings.Cut(path, quotaBasePath); !ok || rest != "" {
		return quotaFail(http.StatusNotFound, "unknown path "+path)
	}
	in, _ := body.(map[string]any)
	switch method {
	case http.MethodGet:
		// The id query parameter is ignored.
		if f.notFound {
			return quotaFail(http.StatusNotFound, "quota allocation rule set not found")
		}
		return http.StatusOK, map[string]any{"ruleSet": f.set}
	case http.MethodPost:
		if f.writeStatus != 0 {
			return quotaFail(f.writeStatus, "quota allocation rule set could not be written")
		}
		// The recording shows a create over a rule set with customer rules, or with none, as a
		// replace. A 409 is not recorded: the tenant has no Coralogix-managed rules. The fake
		// answers it when managed rules exist, as the provider expects.
		if f.set != nil && len(quotaManagedOnly(f.set["rules"].([]any))) > 0 {
			return quotaFail(http.StatusConflict, "Quota allocation rule set already exists")
		}
		return f.write(in)
	case http.MethodPut:
		if f.writeStatus != 0 {
			return quotaFail(f.writeStatus, "quota allocation rule set could not be written")
		}
		if f.notFound {
			return quotaFail(http.StatusNotFound, "quota allocation rule set not found")
		}
		return f.write(in)
	case http.MethodDelete:
		if f.notFound {
			return quotaFail(http.StatusNotFound, "quota allocation rule set not found")
		}
		// A delete clears the customer rules and keeps the rule set, also when it is empty.
		if f.deleteStatus != 0 {
			if f.deleteClears {
				f.set["rules"] = quotaManagedOnly(f.set["rules"].([]any))
			}
			return quotaFail(f.deleteStatus, "quota allocation rule set could not be deleted")
		}
		f.set["rules"] = quotaManagedOnly(f.set["rules"].([]any))
		return http.StatusOK, map[string]any{"id": f.set["id"]}
	}
	return quotaFail(http.StatusMethodNotAllowed, "unsupported "+method+" "+path)
}

func quotaFail(status int, message string) (int, any) {
	// The message is the status text, a colon, and the reason, as other services of the facade write it.
	if !strings.HasPrefix(message, "Bad Request: ") {
		message = http.StatusText(status) + ": " + message
	}
	return status, map[string]any{"code": status, "message": message}
}

func quotaManagedOnly(rules []any) []any {
	out := []any{}
	for _, r := range rules {
		if managed, _ := r.(map[string]any)["cxManaged"].(bool); managed {
			out = append(out, r)
		}
	}
	return out
}

func quotaStoreRule(r map[string]any) map[string]any {
	out := map[string]any{
		"entityType":     r["entityType"],
		"allocation":     r["allocation"],
		"enabled":        r["enabled"],
		"canOverflow":    r["canOverflow"],
		"allocationType": "QUOTA_ALLOCATION_TYPE_PERCENTAGE",
		"cxManaged":      false,
	}
	// An omitted or unspecified type is stored as percentage.
	if v, ok := r["allocationType"]; ok && v != "QUOTA_ALLOCATION_TYPE_UNSPECIFIED" {
		out["allocationType"] = v
	}
	if a, ok := out["allocation"].(float64); ok {
		out["allocation"] = quotaFloat(a)
	}
	return out
}

func (f *quotaFake) write(in map[string]any) (int, any) {
	ruleSet, _ := in["ruleSet"].(map[string]any)
	if ruleSet == nil {
		return quotaFail(http.StatusBadRequest, "Bad Request: rule_set is required")
	}
	customer := []any{}
	seen := map[string]bool{}
	sum := 0.0
	for _, raw := range asList(ruleSet["rules"]) {
		r, _ := raw.(map[string]any)
		entityType, _ := r["entityType"].(string)
		if entityType == "" {
			return quotaFail(http.StatusBadRequest, "Bad Request: Invalid entity type.")
		}
		if seen[entityType] {
			return quotaFail(http.StatusBadRequest, "Bad Request: Duplicate rule found for "+strings.ToUpper(entityType[:1])+entityType[1:])
		}
		seen[entityType] = true
		switch r["allocationType"] {
		case nil, "QUOTA_ALLOCATION_TYPE_PERCENTAGE", "QUOTA_ALLOCATION_TYPE_UNSPECIFIED":
			a, _ := r["allocation"].(float64)
			if a < 0 || a > 100 {
				return quotaFail(http.StatusBadRequest, "Bad Request: Invalid allocation rules: Quota allocation must be in range [0, 100] while it is "+strconv.FormatFloat(a, 'f', -1, 64))
			}
			if enabled, _ := r["enabled"].(bool); enabled {
				sum += a
			}
		case "QUOTA_ALLOCATION_TYPE_LOCKED_UNITS":
		default:
			return quotaFail(http.StatusBadRequest, fmt.Sprintf("Bad Request: proto: (line 1:1): invalid value for enum field allocationType: %q", r["allocationType"]))
		}
		customer = append(customer, quotaStoreRule(r))
	}
	if sum > 100 {
		return quotaFail(http.StatusBadRequest, "Bad Request: Quota allocation must be in range [0, 100] while it is "+strconv.FormatFloat(quotaFloat(sum), 'f', -1, 64))
	}
	if f.set == nil {
		f.seq++
		f.set = map[string]any{"id": fmt.Sprintf("00000000-0000-4000-8000-%012d", f.seq)}
	}
	managed := quotaManagedOnly(f.set["rules"].([]any))
	// The answer to a write lists the rules in the order of the request. A read lists them by entity type.
	answer := map[string]any{"id": f.set["id"], "rules": append(append([]any{}, managed...), customer...)}
	stored := append(append([]any{}, managed...), customer...)
	sort.Slice(stored, func(i, j int) bool {
		return stored[i].(map[string]any)["entityType"].(string) < stored[j].(map[string]any)["entityType"].(string)
	})
	f.set["rules"] = stored
	return http.StatusOK, map[string]any{"ruleSet": answer}
}

// quotaFloat is a float32 allocation as the API returns it.
func quotaFloat(v float64) float64 {
	parsed, err := strconv.ParseFloat(strconv.FormatFloat(float64(float32(v)), 'f', -1, 32), 64)
	if err != nil || math.IsNaN(parsed) {
		return v
	}
	return parsed
}
