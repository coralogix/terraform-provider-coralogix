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
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// teamFirstID is the first id that the fake gives a team. Ids are synthetic.
const teamFirstID = 1001

// teamFake models the teams REST API as recorded on a real tenant (fidelity/recorded.json):
//   - Create returns only {"teamId":{"id":N}}. Update and Delete return {}.
//   - The service stores the daily quota as a 32-bit float, so a read returns the float64 of that
//     float32: 0.3 reads back as 0.30000001192092896.
//   - Create moves the quota above 0.01 from the default team. An omitted quota, or 0.01 or less,
//     gives the minimum quota, which reads back as float32(0.02).
//   - Update moves the difference to the target quota between the team and the default team. An
//     increase reaches the target. A decrease leaves the target plus 0.01, and a decrease of 0.01
//     or less is rejected. Update rejects a quota of 0 or less.
//   - Delete first sets the quota to 0.01 by the same rule, so a team with the minimum quota
//     (a decrease of 0.01) cannot be deleted.
//   - A deleted team is not a 404: Get answers 400, Update 500, and Delete 400. An id that never
//     existed answers 404.
type teamFake struct {
	mu       sync.Mutex
	teams    map[int64]*teamRecord
	nextID   int64
	requests []grRequest
	// getStatus, updateStatus, and deleteStatus force an answer of the next calls.
	getStatus, updateStatus, deleteStatus int
}

type teamRecord struct {
	name    string
	quota   float32
	deleted bool
}

func newTeamFidelityFake() *teamFake {
	return &teamFake{teams: map[int64]*teamRecord{}, nextID: teamFirstID}
}

func newTeamFake(t *testing.T) *teamFake {
	t.Helper()
	f := newTeamFidelityFake()
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

func (h *grHarness) team() *teamFake {
	h.t.Helper()
	f, ok := h.api.(*teamFake)
	if !ok {
		h.t.Fatal("expected *teamFake")
	}
	return f
}

func (f *teamFake) takeRequests() []grRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := f.requests
	f.requests = nil
	return out
}

// seed adds a team as if it was created outside Terraform and returns its id.
func (f *teamFake) seed(name string, quota float64) int64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	id := f.nextID
	f.nextID++
	f.teams[id] = &teamRecord{name: name, quota: float32(quota)}
	return id
}

// deleteOutside deletes a team as the UI would: the API then answers 400 for it.
func (f *teamFake) deleteOutside(id int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.teams[id].deleted = true
}

// forgetOutside removes every trace of a team, so the API answers 404 for it.
func (f *teamFake) forgetOutside(id int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.teams, id)
}

func (f *teamFake) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	raw, _ := io.ReadAll(r.Body)
	rec := grRequest{Method: r.Method, Path: r.URL.Path}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &rec.Body)
	}
	status, body := f.handle(r.Method, r.URL.Path, rec.Body)
	rec.Status = status
	f.requests = append(f.requests, rec)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func (f *teamFake) handle(method, path string, body any) (int, any) {
	i := strings.Index(path, teamBasePath)
	if i < 0 {
		return teamError(http.StatusNotFound, "Not Found")
	}
	rest := strings.TrimPrefix(path[i+len(teamBasePath):], "/")
	if rest == "" {
		if method != http.MethodPost {
			return teamError(http.StatusNotFound, "Not Found")
		}
		return f.create(asObject(body))
	}
	id, err := strconv.ParseUint(rest, 10, 32)
	if err != nil {
		return teamError(http.StatusBadRequest, fmt.Sprintf("Bad Request: type mismatch, parameter: team_id.id, error: parsing field \"id\": strconv.ParseUint: parsing %q: invalid syntax", rest))
	}
	switch method {
	case http.MethodGet:
		return f.get(int64(id))
	case http.MethodPut:
		return f.update(int64(id), asObject(body))
	case http.MethodDelete:
		return f.delete(int64(id))
	default:
		return teamError(http.StatusNotFound, "Not Found")
	}
}

func (f *teamFake) create(request map[string]any) (int, any) {
	name, _ := request["teamName"].(string)
	if name == "" {
		return teamError(http.StatusBadRequest, "Bad Request: must be not empty.")
	}
	if f.nameTaken(name, 0) {
		return teamError(http.StatusBadRequest, fmt.Sprintf("Bad Request: Team url %s is already taken", name))
	}
	quota := float32(0.02)
	if q, ok := request["dailyQuota"].(float64); ok && q > 0.01 {
		quota = float32(q)
	}
	id := f.nextID
	f.nextID++
	f.teams[id] = &teamRecord{name: name, quota: quota}
	return http.StatusOK, map[string]any{"teamId": map[string]any{"id": id}}
}

func (f *teamFake) get(id int64) (int, any) {
	if f.getStatus != 0 {
		return teamError(f.getStatus, http.StatusText(f.getStatus))
	}
	team, status, answer := f.lookup(id, http.StatusBadRequest, "Bad Request: Invalid argument: Team url not found")
	if team == nil {
		return status, answer
	}
	return http.StatusOK, map[string]any{
		"teamId":     map[string]any{"id": id},
		"teamName":   team.name,
		"dailyQuota": float64(team.quota),
		"retention":  1,
	}
}

func (f *teamFake) update(id int64, request map[string]any) (int, any) {
	if f.updateStatus != 0 {
		return teamError(f.updateStatus, http.StatusText(f.updateStatus))
	}
	team, status, answer := f.lookup(id, http.StatusInternalServerError, "Internal Server Error: Target team is not active")
	if team == nil {
		return status, answer
	}
	quota, setQuota := request["dailyQuota"].(float64)
	if setQuota && quota <= 0 {
		return teamError(http.StatusBadRequest, fmt.Sprintf("Bad Request: units: value %s is not more than 0", strconv.FormatFloat(quota, 'f', -1, 32)))
	}
	name, setName := request["teamName"].(string)
	if setName && name == "" {
		return teamError(http.StatusBadRequest, "Bad Request: Team invalid name: invalid or restricted team name")
	}
	if setName && f.nameTaken(name, id) {
		return teamError(http.StatusInternalServerError, "Internal Server Error: Already exist: Team name has already been taken")
	}
	if setQuota {
		moved, ok := teamMoveQuota(team.quota, quota)
		if !ok {
			return teamError(http.StatusBadRequest, "Bad Request: DailyQuota must be greater than 0.01")
		}
		team.quota = moved
	}
	if setName {
		team.name = name
	}
	return http.StatusOK, map[string]any{}
}

func (f *teamFake) delete(id int64) (int, any) {
	if f.deleteStatus != 0 {
		return teamError(f.deleteStatus, http.StatusText(f.deleteStatus))
	}
	team, ok := f.teams[id]
	if !ok {
		return teamError(http.StatusNotFound, fmt.Sprintf("Not Found: Could not find team in database: %d", id))
	}
	if _, ok := teamMoveQuota(team.quota, 0.01); team.deleted || !ok {
		return teamError(http.StatusBadRequest, "Bad Request: DailyQuota must be greater than 0.01")
	}
	team.deleted = true
	return http.StatusOK, map[string]any{}
}

// lookup returns the active team with id, or the answer for an id that never existed (404) or
// a deleted team (deletedStatus and deletedMessage).
func (f *teamFake) lookup(id int64, deletedStatus int, deletedMessage string) (*teamRecord, int, any) {
	team, ok := f.teams[id]
	if !ok {
		status, answer := teamError(http.StatusNotFound, fmt.Sprintf("Not Found: Could not find team in database: %d", id))
		return nil, status, answer
	}
	if team.deleted {
		status, answer := teamError(deletedStatus, deletedMessage)
		return nil, status, answer
	}
	return team, 0, nil
}

func (f *teamFake) nameTaken(name string, except int64) bool {
	for id, team := range f.teams {
		if id != except && !team.deleted && team.name == name {
			return true
		}
	}
	return false
}

// teamMoveQuota returns the quota that a team with quota current has after the API sets it to
// target, and false when the API rejects the move.
func teamMoveQuota(current float32, target float64) (float32, bool) {
	decrease := float64(current) - target
	switch {
	case decrease <= 0:
		return float32(target), true
	case decrease <= 0.01:
		return current, false
	default:
		return float32(target + 0.01), true
	}
}

func teamError(status int, message string) (int, any) {
	return status, map[string]any{"code": status, "message": message}
}
