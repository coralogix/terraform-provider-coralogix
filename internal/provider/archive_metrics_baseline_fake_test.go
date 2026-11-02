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
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
)

const amPath = "/metrics/data-setup/v1"

// amServerRegion is the region of the S3 buckets that the tests use.
const amServerRegion = "eu-north-1"

// amCustomPrefix is the prefix of the custom storage format, the default of a new configuration.
const amCustomPrefix = "cx_metrics/"

// amTenantID is the company id that Get returns.
const amTenantID = 4711

// amFake is a small in-memory backend of the metrics archive configuration (GET and POST
// /metrics/data-setup/v1), as metrics-configurator serves it. A company has at most one
// configuration. Get without one is a 404. Set (ConfigureTenant) has no existence check: it
// replaces the configuration and enables it again. A new configuration gets the prefix of the
// custom format; an existing one keeps its format and prefix. Set rejects a request without a
// storage arm and an IBM CRN with fewer than 10 components. It returns {}. Get returns
// retentionPolicy only when the last Set sent one, and always disabled and the managed fields.
// Errors are JSON with the gRPC status text, as the REST gateway sends them to a client that
// accepts JSON. It follows the answers recorded on EU2 (archive_metrics_fidelity_test.go).
type amFake struct {
	mu       sync.Mutex
	config   map[string]any // the stored configuration, nil when the company has none
	requests []grRequest
	// getFail, when set, answers every GET instead of the stored configuration.
	getFail func() (int, any)
	// setFail, when set, answers every POST instead of storing the configuration.
	setFail func() (int, any)
	// getFailAfterSet makes every GET fail once a POST has stored the configuration.
	getFailAfterSet func() (int, any)
	// unwritable are the S3 buckets that the archive role cannot write. Set probes the bucket
	// before it saves, so it rejects them and keeps the stored configuration.
	unwritable map[string]bool
}

func newAMFake(t *testing.T) *amFake {
	t.Helper()
	f := &amFake{}
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

// seed stores a configuration as if another client had set it. It has the custom storage format
// unless config sets prefix ("" for a legacy configuration).
func (f *amFake) seed(config map[string]any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.config = map[string]any{"prefix": amCustomPrefix, "disabled": false}
	for k, v := range config {
		f.config[k] = v
	}
}

// disable sets the configuration disabled, as the DisableArchive call does.
func (f *amFake) disable() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.config["disabled"] = true
}

// forget removes the configuration, as a company that never had one.
func (f *amFake) forget() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.config = nil
}

// failGets makes every GET answer with the given error.
func (f *amFake) failGets(status int, prefix, message string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.getFail = func() (int, any) { return amFail(status, prefix, message) }
}

// failGetsAfterSet makes every GET answer with the given error once a POST has succeeded.
func (f *amFake) failGetsAfterSet(status int, prefix, message string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.getFailAfterSet = func() (int, any) { return amFail(status, prefix, message) }
}

// healGets makes GET answer from the stored configuration again.
func (f *amFake) healGets() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.getFail, f.getFailAfterSet = nil, nil
}

// failSets makes every POST answer with the given error.
func (f *amFake) failSets(status int, prefix, message string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.setFail = func() (int, any) { return amFail(status, prefix, message) }
}

func (f *amFake) takeRequests() []grRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := f.requests
	f.requests = nil
	return out
}

func (f *amFake) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

func (f *amFake) handle(method, path string, body any) (int, any) {
	// Disable and Enable set the flag, and succeed without a configuration too.
	for suffix, disabled := range map[string]bool{"/disable": true, "/enable": false} {
		if method == http.MethodPost && strings.HasSuffix(path, amPath+suffix) {
			if f.config != nil {
				f.config["disabled"] = disabled
			}
			return http.StatusOK, map[string]any{}
		}
	}
	if !strings.HasSuffix(path, amPath) {
		return grFail(http.StatusNotFound, "unknown path "+path)
	}
	switch method {
	case http.MethodGet:
		if f.getFail != nil {
			return f.getFail()
		}
		if f.config == nil {
			return amFail(http.StatusNotFound, "Not Found", "Tenant config not found")
		}
		return http.StatusOK, map[string]any{"tenantConfig": f.present()}
	case http.MethodPost:
		if f.setFail != nil {
			return f.setFail()
		}
		status, resp := f.set(body)
		if status == http.StatusOK && f.getFailAfterSet != nil {
			f.getFail = f.getFailAfterSet
		}
		return status, resp
	}
	return grFail(http.StatusMethodNotAllowed, "unsupported "+method+" "+path)
}

func (f *amFake) set(body any) (int, any) {
	in, _ := body.(map[string]any)
	stored := map[string]any{"disabled": false}
	switch {
	case in["s3"] != nil:
		s3, _ := in["s3"].(map[string]any)
		if bucket, _ := s3["bucket"].(string); f.unwritable[bucket] {
			return amFail(http.StatusBadRequest, "Bad Request", "Invalid bucket: Cannot write file")
		}
		stored["s3"] = s3
	case in["ibm"] != nil:
		ibm, _ := in["ibm"].(map[string]any)
		crn, _ := ibm["crn"].(string)
		// The bucket is the resource part of the CRN, its 10th colon-separated component.
		if len(strings.Split(crn, ":")) < 10 {
			return amFail(http.StatusBadRequest, "Bad Request", "invalid storage config")
		}
		stored["ibm"] = ibm
	default:
		return amFail(http.StatusBadRequest, "Bad Request", "missing storage config")
	}
	if p, ok := in["retentionPolicy"].(map[string]any); ok {
		// The API stores the three levels of a sent policy, a missing one as 0, and returns each,
		// 0 too (public_server.rs, From<RetentionPolicyRequest>).
		levels := map[string]any{}
		for _, level := range []string{"rawResolution", "fiveMinutesResolution", "oneHourResolution"} {
			levels[level] = 0.0
			if v, ok := p[level]; ok {
				levels[level] = v
			}
		}
		stored["retentionPolicy"] = levels
	}
	// The storage format of an existing configuration stays: a legacy one keeps its empty prefix.
	stored["prefix"] = amCustomPrefix
	if f.config != nil {
		stored["prefix"] = f.config["prefix"]
	}
	f.config = stored
	return http.StatusOK, map[string]any{}
}

// amFail is an error answer of metrics-configurator through the REST gateway: the gRPC status text
// and the message.
func amFail(status int, prefix, message string) (int, any) {
	return status, map[string]any{"code": status, "message": prefix + ": " + message}
}

func (f *amFake) present() map[string]any {
	out := mapsClone(f.config)
	out["tenantId"] = amTenantID
	out["managedBy"] = "MANAGED_BY_UNSPECIFIED"
	out["managedRetentionClass"] = "MANAGED_RETENTION_CLASS_UNSPECIFIED"
	return out
}
