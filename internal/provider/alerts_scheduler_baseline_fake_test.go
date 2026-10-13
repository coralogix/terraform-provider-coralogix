// Copyright 2024 Coralogix Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an AS IS BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package provider

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

const asBasePath = "/alerts/suppression-rules/v1"

// asFake is a small in-memory backend for alert scheduler rules. It follows the server
// (alert-scheduler-api):
//   - Create and Update take {"alertSchedulerRule": ...}. Create ignores a rule that sets
//     uniqueIdentifier or id and answers 200 with an empty rule. Update needs uniqueIdentifier;
//     an unknown one answers 404. Every Update stores a new version id.
//   - The filter needs exactly one of alertUniqueIds (an empty list means all alerts) and
//     alertMetaLabels. whatExpression must start with "source logs | filter ".
//   - Omitted enabled is false. Omitted description reads back as "". Meta labels come back
//     with ids: the rule labels in request order, the filter labels in reverse order. An alert
//     id that the tenant does not have is rejected; the fake knows alert-1 and alert-2.
//   - A time is read in the timezone of the time frame (an explicit offset or Z wins), kept to
//     the second, and returned in that timezone with milliseconds and no offset. Create and
//     Update echo the raw start time in submittedStartTime; Get does not. The server code moves
//     a recurring start time to the first scheduled day, but the acceptance test reads a weekly
//     Sunday rule back with its Monday start, so the fake keeps the start time.
//   - always is the empty object. A dynamic rule with repeatEvery <= 0 reads back as always.
//   - Get, Update, and Delete of a missing rule answer 404. Errors are text, as recorded:
//     "Bad Request: [ <path> ] <message>" and "Not Found: <message>".
type asFake struct {
	mu       sync.Mutex
	rules    map[string]map[string]any
	alerts   []string // the alert ids of the tenant
	seq      int
	requests []grRequest
}

var asKnownAlerts = []string{"alert-1", "alert-2"}

// asFail is an error answer of the API: the status text, then the message.
func asFail(status int, message string) (int, any) {
	return status, map[string]any{"code": status, "message": http.StatusText(status) + ": " + message}
}

// asInvalid is a validation error at the path of the field.
func asInvalid(path, message string) (int, any) {
	return asFail(http.StatusBadRequest, "[ "+path+" ] "+message)
}

func asNotFound(id string) (int, any) {
	return asFail(http.StatusNotFound, "No suppression rule found with unique identifier "+id)
}

func newASFake(t *testing.T) *asFake {
	t.Helper()
	f := &asFake{rules: map[string]map[string]any{}, alerts: asKnownAlerts}
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

// seed stores a rule as Create would, without a request record. It returns the unique identifier.
func (f *asFake) seed(rule map[string]any) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	status, body := f.write(map[string]any{"alertSchedulerRule": rule}, false)
	if status != http.StatusOK {
		panic(fmt.Sprintf("seed failed: %d %v", status, body))
	}
	return body.(map[string]any)["alertSchedulerRule"].(map[string]any)["uniqueIdentifier"].(string)
}

func (f *asFake) forget(id string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.rules, id)
}

func (f *asFake) takeRequests() []grRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := f.requests
	f.requests = nil
	return out
}

func (f *asFake) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()

	raw, _ := io.ReadAll(r.Body)
	rec := grRequest{Method: r.Method, Path: r.URL.Path}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &rec.Body)
	}
	status, body := f.handle(r.Method, r.URL.Path, rec.Body)
	rec.Body = dropEmptyCollections(rec.Body)
	rec.Status = status
	f.requests = append(f.requests, rec)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func (f *asFake) handle(method, path string, body any) (int, any) {
	_, rest, ok := strings.Cut(path, asBasePath)
	if !ok {
		return grFail(http.StatusNotFound, "unknown path "+path)
	}
	id := strings.TrimPrefix(rest, "/")
	in, _ := body.(map[string]any)

	switch {
	case method == http.MethodPost && id == "":
		return f.write(in, false)
	case method == http.MethodPut && id == "":
		return f.write(in, true)
	case method == http.MethodGet && id != "":
		stored, found := f.rules[id]
		if !found {
			return asNotFound(id)
		}
		return http.StatusOK, map[string]any{"alertSchedulerRule": stored}
	case method == http.MethodDelete && id != "":
		if _, found := f.rules[id]; !found {
			return asNotFound(id)
		}
		delete(f.rules, id)
		return http.StatusOK, map[string]any{}
	}
	return grFail(http.StatusMethodNotAllowed, "unsupported "+method+" "+path)
}

func (f *asFake) uuidLike() string {
	f.seq++
	return fmt.Sprintf("00000000-0000-4000-8000-%012d", f.seq)
}

func (f *asFake) write(in map[string]any, update bool) (int, any) {
	rule, _ := in["alertSchedulerRule"].(map[string]any)
	if rule == nil {
		return asInvalid("alertSchedulerRule", "Required")
	}
	uid, _ := rule["uniqueIdentifier"].(string)
	if update {
		if uid == "" {
			return asInvalid("alertSchedulerRule", "uniqueIdentifier is required; the value provided in the id field is a version id and cannot be used to update a rule")
		}
		if _, found := f.rules[uid]; !found {
			return asFail(http.StatusNotFound, "No result found")
		}
	} else if uid != "" || rule["id"] != nil {
		// The server ignores the error of this check and answers OK with an empty rule.
		return http.StatusOK, map[string]any{"alertSchedulerRule": map[string]any{}}
	}
	if path, msg := f.check(rule); msg != "" {
		return asInvalid(path, msg)
	}
	if !update {
		uid = f.uuidLike()
	}
	stored, submitted := f.normalize(rule)
	stored["uniqueIdentifier"] = uid
	stored["id"] = f.uuidLike()
	stamp := fmt.Sprintf("2026-01-01T00:00:%02d.000Z", f.seq%60)
	stored["createdAt"], stored["updatedAt"] = stamp, stamp
	f.rules[uid] = stored

	out := mapsClone(stored)
	if submitted != "" {
		out["schedule"] = withSubmittedStartTime(stored["schedule"].(map[string]any), submitted)
	}
	return http.StatusOK, map[string]any{"alertSchedulerRule": out}
}

// check returns the path and the message of the first rule that the request breaks, as the
// server words them.
func (f *asFake) check(rule map[string]any) (string, string) {
	const r = "alertSchedulerRule"
	if name, _ := rule["name"].(string); name == "" {
		return r + ".name", "String must contain at least 1 character(s)"
	}
	filter, _ := rule["filter"].(map[string]any)
	if what, _ := filter["whatExpression"].(string); !strings.HasPrefix(what, "source logs | filter") {
		return r + ".filter.whatExpression", `Invalid dataPrime expression : {"syntax_error":"expression does not start with 'source logs | filter'"}`
	}
	ids, hasIDs := filter["alertUniqueIds"].(map[string]any)
	_, hasLabels := filter["alertMetaLabels"]
	if hasIDs == hasLabels {
		return r + ".filter.whichAlerts", "Invalid input"
	}
	for _, id := range asList(ids["value"]) {
		if !slices.Contains(f.alerts, id.(string)) {
			return r + ".filter.whichAlerts.alertUniqueIds.value", "invalid alert uniqe ids"
		}
	}
	schedule, _ := rule["schedule"].(map[string]any)
	if op, _ := schedule["scheduleOperation"].(string); op == "" || op == "SCHEDULE_OPERATION_UNSPECIFIED" {
		return r + ".schedule.scheduleOperation", "ScheduleOperation value UNSPECIFIED is not allowed"
	}
	if one, ok := schedule["oneTime"].(map[string]any); ok {
		return asCheckTimeframe(r+".schedule.scheduler.oneTime.timeframe", one["timeframe"])
	}
	recurring, _ := schedule["recurring"].(map[string]any)
	if _, ok := recurring["alwaysActive"]; ok {
		return "", ""
	}
	dynamic, ok := recurring["schedule"].(map[string]any)
	if !ok {
		return r + ".schedule.scheduler", "Invalid input"
	}
	at := r + ".schedule.scheduler.recurring.condition.schedule"
	if w, ok := dynamic["weekly"].(map[string]any); ok {
		days := asList(w["daysOfWeek"])
		if len(days) == 0 {
			return at + ".frequency.weekly.daysOfWeek", "Need to insert daysOfWeek"
		}
		for _, d := range days {
			if n, _ := d.(float64); n < 1 || n > 7 {
				return at + ".frequency.weekly.daysOfWeek", "Invalid days in week (accept 1-7)"
			}
		}
	}
	if m, ok := dynamic["monthly"].(map[string]any); ok && len(asList(m["daysOfMonth"])) == 0 {
		return at + ".frequency.monthly.daysOfMonth", "Need to insert daysOfWeek"
	}
	return asCheckTimeframe(at+".timeframe", dynamic["timeframe"])
}

func asCheckTimeframe(at string, v any) (string, string) {
	tf, _ := v.(map[string]any)
	tz, _ := tf["timezone"].(string)
	if _, ok := asZone(tz); !ok {
		return at + ".timezone", "invalid timezone"
	}
	if _, ok := tf["endTime"]; ok {
		return "", ""
	}
	if _, ok := tf["duration"]; ok {
		return "", ""
	}
	return at + ".until", "Invalid input"
}

// normalize returns what the server stores, and the raw start time of the request.
func (f *asFake) normalize(rule map[string]any) (map[string]any, string) {
	out := map[string]any{
		"name":        rule["name"],
		"description": orDefault(rule["description"], ""),
		"enabled":     orDefault(rule["enabled"], false),
		"metaLabels":  f.labels(asList(rule["metaLabels"]), false),
	}
	filter := rule["filter"].(map[string]any)
	outFilter := map[string]any{"whatExpression": filter["whatExpression"]}
	if ids, ok := filter["alertUniqueIds"].(map[string]any); ok {
		outFilter["alertUniqueIds"] = map[string]any{"value": orDefault(ids["value"], []any{})}
	} else {
		labels := filter["alertMetaLabels"].(map[string]any)
		outFilter["alertMetaLabels"] = map[string]any{"value": f.labels(asList(labels["value"]), true)}
	}
	out["filter"] = outFilter

	schedule := rule["schedule"].(map[string]any)
	outSchedule := map[string]any{"scheduleOperation": schedule["scheduleOperation"]}
	var submitted string
	if one, ok := schedule["oneTime"].(map[string]any); ok {
		tf := one["timeframe"].(map[string]any)
		submitted, _ = tf["startTime"].(string)
		outSchedule["oneTime"] = map[string]any{"timeframe": asTimeframe(tf)}
	} else {
		recurring := schedule["recurring"].(map[string]any)
		dynamic, _ := recurring["schedule"].(map[string]any)
		if n, _ := dynamic["repeatEvery"].(float64); dynamic == nil || n <= 0 {
			outSchedule["recurring"] = map[string]any{"alwaysActive": map[string]any{}}
		} else {
			tf := dynamic["timeframe"].(map[string]any)
			submitted, _ = tf["startTime"].(string)
			outDynamic := map[string]any{"repeatEvery": dynamic["repeatEvery"], "timeframe": asTimeframe(tf)}
			for _, k := range []string{"daily", "weekly", "monthly"} {
				if v, ok := dynamic[k]; ok {
					outDynamic[k] = v
				}
			}
			term, _ := dynamic["terminationDate"].(string)
			outDynamic["terminationDate"] = asLocal(term, tf["timezone"].(string))
			outSchedule["recurring"] = map[string]any{"schedule": outDynamic}
		}
	}
	out["schedule"] = outSchedule
	return out, submitted
}

// labels gives each label an id. The server keeps the order of the rule labels and returns the
// filter labels in reverse order.
func (f *asFake) labels(in []any, reverse bool) []any {
	out := make([]any, 0, len(in))
	for _, l := range in {
		label := mapsClone(l.(map[string]any))
		if label["id"] == nil {
			label["id"] = f.uuidLike()
		}
		out = append(out, label)
	}
	if reverse {
		slices.Reverse(out)
	}
	return out
}

func asTimeframe(tf map[string]any) map[string]any {
	tz := tf["timezone"].(string)
	start, _ := tf["startTime"].(string)
	out := map[string]any{"timezone": tz, "startTime": asLocal(start, tz)}
	if end, ok := tf["endTime"].(string); ok {
		out["endTime"] = asLocal(end, tz)
	}
	if d, ok := tf["duration"]; ok {
		out["duration"] = d
	}
	return out
}

func withSubmittedStartTime(schedule map[string]any, submitted string) map[string]any {
	out := mapsClone(schedule)
	if one, ok := out["oneTime"].(map[string]any); ok {
		tf := mapsClone(one["timeframe"].(map[string]any))
		tf["submittedStartTime"] = submitted
		out["oneTime"] = map[string]any{"timeframe": tf}
		return out
	}
	recurring := out["recurring"].(map[string]any)
	dynamic, ok := recurring["schedule"].(map[string]any)
	if !ok {
		return out
	}
	dynamic = mapsClone(dynamic)
	tf := mapsClone(dynamic["timeframe"].(map[string]any))
	tf["submittedStartTime"] = submitted
	dynamic["timeframe"] = tf
	out["recurring"] = map[string]any{"schedule": dynamic}
	return out
}

const asLocalLayout = "2006-01-02T15:04:05.000"

// asZone reads a timezone such as UTC, UTC+0, or UTC-5.
func asZone(tz string) (*time.Location, bool) {
	if tz == "UTC" {
		return time.UTC, true
	}
	offset, ok := strings.CutPrefix(tz, "UTC")
	if !ok || offset == "" {
		return nil, false
	}
	hours, err := strconv.Atoi(offset)
	if err != nil {
		return nil, false
	}
	return time.FixedZone(tz, hours*3600), true
}

// asLocal reads s in the timezone tz, unless s has its own offset, and returns it in tz with
// milliseconds and no offset, to the second. "" stays "".
func asLocal(s, tz string) string {
	if s == "" {
		return ""
	}
	loc, _ := asZone(tz)
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.In(loc).Truncate(time.Second).Format(asLocalLayout)
		}
	}
	for _, layout := range []string{asLocalLayout, "2006-01-02T15:04:05"} {
		if t, err := time.ParseInLocation(layout, s, loc); err == nil {
			return t.Truncate(time.Second).Format(asLocalLayout)
		}
	}
	return s
}
