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

package provider

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// A scenario golden of the alert baseline saves the first config, plan, and state of a scenario in
// full. Each later step saves, for each of the three, only what changed against the same field of the
// step before (as <field>Delta), or the whole value when that is shorter:
//
//	"stateDelta": {"set": {"/case_settings/auto_resolve_mode": "enabled"}, "remove": ["/access_policy"]}
//
// The requests of a step are compared one by one with the latest request before them that has the
// same method. A request that is shorter as a change is saved as the method it follows and the change:
//
//	{"deltaOf": "PUT", "set": {"/body/alertDefProperties/name": "new name"}}
//
// A path is a JSON Pointer (RFC 6901). A delta is lossless: the values on a changed path are saved
// in full, and TestAlertBaselineGoldenDeltaRoundTrip rebuilds every step from its deltas.
// The goldens are a diff, so an unchanged value shows once per scenario, and a change shows once.

// alertDeltaFields are the fields of a step that a golden saves as a delta.
var alertDeltaFields = []string{"config", "plan", "state"}

// alertStepKeyOrder is the order of the keys of a step, the same as grStep. Any other key follows
// them in sorted order.
var alertStepKeyOrder = []string{
	"step", "config", "validateDiagnostics", "plan", "noChange", "planDiagnostics", "requiresReplace",
	"requests", "applyDiagnostics", "inconsistentResult", "state", "drift", "replanEmpty", "replanDiff",
}

// alertDelta is the change of one value: the values to set by path, and the paths to remove.
type alertDelta struct {
	Set    map[string]any `json:"set,omitempty"`
	Remove []string       `json:"remove,omitempty"`
}

func (d *alertDelta) set(path string, v any) {
	if d.Set == nil {
		d.Set = map[string]any{}
	}
	d.Set[path] = v
}

func alertEscapePointer(key string) string {
	return strings.ReplaceAll(strings.ReplaceAll(key, "~", "~0"), "/", "~1")
}

func alertUnescapePointer(key string) string {
	return strings.ReplaceAll(strings.ReplaceAll(key, "~1", "/"), "~0", "~")
}

// alertDiff adds to d what turns old into new. Objects and arrays of the same length are compared
// member by member. Anything else is set whole.
func alertDiff(path string, old, next any, d *alertDelta) {
	switch o := old.(type) {
	case map[string]any:
		if n, ok := next.(map[string]any); ok {
			alertDiffObjects(path, o, n, d)
			return
		}
	case []any:
		if n, ok := next.([]any); ok && len(n) == len(o) {
			for i := range o {
				alertDiff(path+"/"+strconv.Itoa(i), o[i], n[i], d)
			}
			return
		}
	}
	if !reflect.DeepEqual(old, next) {
		d.set(path, next)
	}
}

func alertDiffObjects(path string, old, next map[string]any, d *alertDelta) {
	keys := make([]string, 0, len(next))
	for k := range next {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		child := path + "/" + alertEscapePointer(k)
		if before, ok := old[k]; ok {
			alertDiff(child, before, next[k], d)
		} else {
			d.set(child, next[k])
		}
	}
	removed := []string{}
	for k := range old {
		if _, ok := next[k]; !ok {
			removed = append(removed, path+"/"+alertEscapePointer(k))
		}
	}
	sort.Strings(removed)
	d.Remove = append(d.Remove, removed...)
}

// alertClone copies a decoded JSON value.
func alertClone(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, item := range x {
			out[k] = alertClone(item)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, item := range x {
			out[i] = alertClone(item)
		}
		return out
	}
	return v
}

func alertPointerParts(path string) []string {
	if path == "" {
		return nil
	}
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	for i, p := range parts {
		parts[i] = alertUnescapePointer(p)
	}
	return parts
}

func alertSetAt(node any, parts []string, v any) (any, error) {
	if len(parts) == 0 {
		return v, nil
	}
	switch n := node.(type) {
	case map[string]any:
		child, err := alertSetAt(n[parts[0]], parts[1:], v)
		n[parts[0]] = child
		return n, err
	case []any:
		i, err := strconv.Atoi(parts[0])
		if err != nil || i < 0 || i >= len(n) {
			return nil, fmt.Errorf("no array member %q", parts[0])
		}
		child, err := alertSetAt(n[i], parts[1:], v)
		n[i] = child
		return n, err
	}
	return nil, fmt.Errorf("cannot set %q inside a value that is no object or array", parts[0])
}

func alertRemoveAt(node any, parts []string) error {
	if len(parts) == 0 {
		return fmt.Errorf("cannot remove the whole value")
	}
	switch n := node.(type) {
	case map[string]any:
		if len(parts) == 1 {
			delete(n, parts[0])
			return nil
		}
		return alertRemoveAt(n[parts[0]], parts[1:])
	case []any:
		i, err := strconv.Atoi(parts[0])
		if err != nil || i < 0 || i >= len(n) || len(parts) == 1 {
			return fmt.Errorf("cannot remove %v from an array", parts)
		}
		return alertRemoveAt(n[i], parts[1:])
	}
	return fmt.Errorf("cannot remove %v from a value that is no object or array", parts)
}

// alertApply returns base with the delta applied. It does not change base.
func alertApply(base any, d alertDelta) (any, error) {
	out := alertClone(base)
	for _, path := range d.Remove {
		if err := alertRemoveAt(out, alertPointerParts(path)); err != nil {
			return nil, err
		}
	}
	paths := make([]string, 0, len(d.Set))
	for path := range d.Set {
		paths = append(paths, path)
	}
	// A shorter path first: a value that is set whole comes before a change inside it.
	sort.Slice(paths, func(i, j int) bool {
		return len(paths[i]) < len(paths[j]) || (len(paths[i]) == len(paths[j]) && paths[i] < paths[j])
	})
	for _, path := range paths {
		var err error
		if out, err = alertSetAt(out, alertPointerParts(path), alertClone(d.Set[path])); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// alertKV and alertOrdered write an object with its keys in the order that they were added.
type alertKV struct {
	key   string
	value any
}

type alertOrdered []alertKV

func alertMarshalPlain(v any) ([]byte, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(b.Bytes(), []byte("\n")), nil
}

func (o alertOrdered) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, kv := range o {
		if i > 0 {
			b.WriteByte(',')
		}
		key, err := alertMarshalPlain(kv.key)
		if err != nil {
			return nil, err
		}
		value, err := alertMarshalPlain(kv.value)
		if err != nil {
			return nil, err
		}
		b.Write(key)
		b.WriteByte(':')
		b.Write(value)
	}
	b.WriteByte('}')
	return b.Bytes(), nil
}

// alertDecode reads JSON and keeps the text of every number.
func alertDecode(raw []byte, out any) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	return dec.Decode(out)
}

// alertStepKeys returns the keys of a step in the order of a golden.
func alertStepKeys(step map[string]any) []string {
	var keys, rest []string
	known := map[string]bool{}
	for _, k := range alertStepKeyOrder {
		known[k] = true
		if _, ok := step[k]; ok {
			keys = append(keys, k)
		}
	}
	for k := range step {
		if !known[k] {
			rest = append(rest, k)
		}
	}
	sort.Strings(rest)
	return append(keys, rest...)
}

func alertIsDeltaField(key string) bool {
	for _, f := range alertDeltaFields {
		if f == key {
			return true
		}
	}
	return false
}

// alertEncodeField returns the pair that a golden saves for one delta field: the whole value, or
// the delta against the same field of the step before when that is shorter.
func alertEncodeField(key string, before any, hadBefore bool, value any) (alertKV, error) {
	full := alertKV{key, value}
	if !hadBefore {
		return full, nil
	}
	var d alertDelta
	alertDiff("", before, value, &d)
	deltaText, err := alertMarshalPlain(d)
	if err != nil {
		return full, err
	}
	fullText, err := alertMarshalPlain(value)
	if err != nil {
		return full, err
	}
	if len(deltaText) < len(fullText) {
		return alertKV{key + "Delta", d}, nil
	}
	return full, nil
}

// alertEncodeRequests saves each request in full, or as the change against the latest request with
// the same method when that is shorter. last holds the latest request of each method.
func alertEncodeRequests(value any, last map[string]any) (any, error) {
	list, ok := value.([]any)
	if !ok {
		return value, nil
	}
	out := make([]any, len(list))
	for i, item := range list {
		out[i] = item
		req, ok := item.(map[string]any)
		if !ok {
			continue
		}
		method, _ := req["method"].(string)
		if base, had := last[method]; had {
			var d alertDelta
			alertDiff("", base, req, &d)
			line := alertOrdered{{"deltaOf", method}}
			if d.Set != nil {
				line = append(line, alertKV{"set", d.Set})
			}
			if len(d.Remove) > 0 {
				line = append(line, alertKV{"remove", d.Remove})
			}
			short, err := alertShorter(line, req)
			if err != nil {
				return nil, err
			}
			if short {
				out[i] = line
			}
		}
		last[method] = req
	}
	return out, nil
}

// alertShorter reports whether the text of a is shorter than the text of b.
func alertShorter(a, b any) (bool, error) {
	at, err := alertMarshalPlain(a)
	if err != nil {
		return false, err
	}
	bt, err := alertMarshalPlain(b)
	if err != nil {
		return false, err
	}
	return len(at) < len(bt), nil
}

// alertEncodeStep returns the keys of one step of a golden, with deltas against the steps before.
func alertEncodeStep(step map[string]any, before, last map[string]any) (alertOrdered, error) {
	var line alertOrdered
	for _, key := range alertStepKeys(step) {
		switch {
		case key == "requests":
			value, err := alertEncodeRequests(step[key], last)
			if err != nil {
				return nil, err
			}
			line = append(line, alertKV{key, value})
		case alertIsDeltaField(key):
			prev, had := before[key]
			kv, err := alertEncodeField(key, prev, had, step[key])
			if err != nil {
				return nil, err
			}
			line = append(line, kv)
			before[key] = step[key]
		default:
			line = append(line, alertKV{key, step[key]})
		}
	}
	return line, nil
}

// encodeAlertSteps writes the golden of a scenario from its full steps (decoded JSON objects).
func encodeAlertSteps(steps []map[string]any) ([]byte, error) {
	before, last := map[string]any{}, map[string]any{}
	out := make([]alertOrdered, 0, len(steps))
	for _, step := range steps {
		line, err := alertEncodeStep(step, before, last)
		if err != nil {
			return nil, err
		}
		out = append(out, line)
	}
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// alertParseDelta reads a delta that was decoded as a generic value.
func alertParseDelta(v any) (alertDelta, error) {
	var d alertDelta
	text, err := json.Marshal(v)
	if err != nil {
		return d, err
	}
	return d, alertDecode(text, &d)
}

// alertDecodeRequests rebuilds the requests of a step. last holds the latest request of each method.
func alertDecodeRequests(value any, last map[string]any) (any, error) {
	list, ok := value.([]any)
	if !ok {
		return value, nil
	}
	for i, item := range list {
		req, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if method, isDelta := req["deltaOf"].(string); isDelta {
			base, had := last[method]
			if !had {
				return nil, fmt.Errorf("a request follows the %s request before it, and there is none", method)
			}
			delete(req, "deltaOf")
			d, err := alertParseDelta(req)
			if err != nil {
				return nil, err
			}
			if req, err = alertApplyObject(base, d); err != nil {
				return nil, err
			}
			list[i] = req
		}
		method, _ := req["method"].(string)
		last[method] = req
	}
	return list, nil
}

func alertApplyObject(base any, d alertDelta) (map[string]any, error) {
	out, err := alertApply(base, d)
	if err != nil {
		return nil, err
	}
	obj, ok := out.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("a request is no object")
	}
	return obj, nil
}

// decodeAlertSteps rebuilds the full steps of a scenario from its golden.
func decodeAlertSteps(raw []byte) ([]map[string]any, error) {
	var steps []map[string]any
	if err := alertDecode(raw, &steps); err != nil {
		return nil, err
	}
	before, last := map[string]any{}, map[string]any{}
	for i, step := range steps {
		if err := alertDecodeStep(step, before, last); err != nil {
			return nil, fmt.Errorf("step %d: %w", i+1, err)
		}
	}
	return steps, nil
}

func alertDecodeStep(step map[string]any, before, last map[string]any) error {
	if value, ok := step["requests"]; ok {
		requests, err := alertDecodeRequests(value, last)
		if err != nil {
			return fmt.Errorf("requests: %w", err)
		}
		step["requests"] = requests
	}
	for _, key := range alertDeltaFields {
		if v, ok := step[key]; ok {
			before[key] = v
			continue
		}
		rawDelta, ok := step[key+"Delta"]
		if !ok {
			continue
		}
		d, err := alertParseDelta(rawDelta)
		if err != nil {
			return fmt.Errorf("%sDelta: %w", key, err)
		}
		base, had := before[key]
		if !had {
			return fmt.Errorf("%sDelta has no value before it", key)
		}
		value, err := alertApply(base, d)
		if err != nil {
			return fmt.Errorf("%sDelta: %w", key, err)
		}
		delete(step, key+"Delta")
		step[key] = value
		before[key] = value
	}
	return nil
}

// marshalAlertGolden writes the golden of the steps that a scenario recorded.
func marshalAlertGolden(t *testing.T, steps []grStep) []byte {
	t.Helper()
	raw, err := json.Marshal(steps)
	if err != nil {
		t.Fatal(err)
	}
	var full []map[string]any
	if err := alertDecode(raw, &full); err != nil {
		t.Fatal(err)
	}
	out, err := encodeAlertSteps(full)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// The goldens on disk decode to full steps, and those steps encode to the same bytes.
func TestAlertBaselineGoldenDeltaRoundTrip(t *testing.T) {
	files, err := filepath.Glob(filepath.Join(alertGoldenDir, "scenario_*.json"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no scenario golden: %v", err)
	}
	for _, file := range files {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		steps, err := decodeAlertSteps(raw)
		if err != nil {
			t.Errorf("%s: %v", filepath.Base(file), err)
			continue
		}
		again, err := encodeAlertSteps(steps)
		if err != nil {
			t.Errorf("%s: %v", filepath.Base(file), err)
			continue
		}
		if !bytes.Equal(raw, again) {
			t.Errorf("%s does not encode back to itself:\n%s", filepath.Base(file), firstDifference(string(raw), string(again)))
		}
	}
}

func TestAlertBaselineDeltaApplyUndoesDiff(t *testing.T) {
	cases := []struct{ name, old, next string }{
		{"equal", `{"a":1}`, `{"a":1}`},
		{"changed scalar", `{"a":1,"b":"x"}`, `{"a":2,"b":"x"}`},
		{"added and removed keys", `{"a":1,"b":2}`, `{"a":1,"c":3}`},
		{"key with a slash and a tilde", `{"a/b":1,"c~d":2}`, `{"a/b":5,"c~d":6}`},
		{"same length array", `{"l":[1,2,{"x":1}]}`, `{"l":[1,3,{"x":2}]}`},
		{"longer array", `{"l":[1,2]}`, `{"l":[1,2,3]}`},
		{"shorter array", `{"l":[1,2,3]}`, `{"l":[]}`},
		{"null to object", `null`, `{"a":1}`},
		{"object to null", `{"a":1}`, `null`},
		{"type change", `{"a":[1]}`, `{"a":{"b":1}}`},
		{"null member", `{"a":null}`, `{"a":{"b":null}}`},
		{"number text kept", `{"a":1.0}`, `{"a":1.50}`},
		{"empty key", `{"":1}`, `{"":2}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var old, next any
			if err := alertDecode([]byte(c.old), &old); err != nil {
				t.Fatal(err)
			}
			if err := alertDecode([]byte(c.next), &next); err != nil {
				t.Fatal(err)
			}
			var d alertDelta
			alertDiff("", old, next, &d)
			got, err := alertApply(old, d)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, next) {
				t.Fatalf("apply(diff) = %v, want %v (delta %+v)", got, next, d)
			}
		})
	}
}

// randomAlertJSON makes a small random value with the shapes of a state: objects, arrays, null,
// strings, numbers, and booleans.
func randomAlertJSON(r *rand.Rand, depth int) any {
	kinds := 6
	if depth <= 0 {
		kinds = 4
	}
	switch r.Intn(kinds) {
	case 0:
		return nil
	case 1:
		return r.Intn(2) == 0
	case 2:
		return json.Number(strconv.Itoa(r.Intn(5)))
	case 3:
		return []string{"a", "b/c", "d~e", ""}[r.Intn(4)]
	case 4:
		out := make([]any, r.Intn(4))
		for i := range out {
			out[i] = randomAlertJSON(r, depth-1)
		}
		return out
	}
	out := map[string]any{}
	for i := r.Intn(4); i > 0; i-- {
		out[[]string{"a", "b/c", "d~e", ""}[r.Intn(4)]] = randomAlertJSON(r, depth-1)
	}
	return out
}

func TestAlertBaselineDeltaApplyUndoesDiffRandom(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	for i := 0; i < 2000; i++ {
		old, next := randomAlertJSON(r, 3), randomAlertJSON(r, 3)
		var d alertDelta
		alertDiff("", old, next, &d)
		got, err := alertApply(old, d)
		if err != nil {
			t.Fatalf("%v: apply(%v, %+v): %v", i, old, d, err)
		}
		if !reflect.DeepEqual(got, next) {
			t.Fatalf("%v: apply(diff(%v, %v)) = %v (delta %+v)", i, old, next, got, d)
		}
	}
}

func TestAlertBaselineRequestDeltaRoundTrip(t *testing.T) {
	text := `[
	  {"step": "create", "state": null, "requests": [{"method": "POST", "path": "/a", "status": 200, "body": {"name": "one", "tags": ["x"], "deep": {"k": 1, "j": 2, "l": [1, 2, 3, 4, 5, 6, 7, 8]}}}]},
	  {"step": "refresh", "state": null, "requests": [{"method": "GET", "path": "/a/1", "status": 200}]},
	  {"step": "update", "state": null, "requests": [{"method": "POST", "path": "/a", "status": 200, "body": {"name": "two", "tags": ["x"], "deep": {"k": 1, "j": 2, "l": [1, 2, 3, 4, 5, 6, 7, 8]}}}, {"method": "GET", "path": "/a/1", "status": 404}]},
	  {"step": "no request", "state": null},
	  {"step": "update again", "state": null, "requests": [{"method": "POST", "path": "/a", "status": 200, "body": {"name": "three", "tags": [], "deep": {"k": 1, "j": 2, "l": [1, 2, 3, 4, 5, 6, 7, 8]}}}]}
	]`
	var steps []map[string]any
	if err := alertDecode([]byte(text), &steps); err != nil {
		t.Fatal(err)
	}
	golden, err := encodeAlertSteps(steps)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(golden), `"deltaOf": "POST"`) {
		t.Errorf("a long request that differs in one place is not saved as a change:\n%s", golden)
	}
	back, err := decodeAlertSteps(golden)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(back, steps) {
		t.Fatalf("decode(encode(steps)) differs from the steps:\n%s", golden)
	}
}
