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
	"regexp"
	"strings"
	"sync"
	"testing"
)

const alPath = "/logs/data-setup/v2"

// alServerRegion is the region of the Coralogix environment. The API stores it when the request
// has no region.
const alServerRegion = "eu-north-1"

var alBucketName = regexp.MustCompile(`^[a-z0-9-.]{3,63}$`)

// alFake is a small in-memory backend of the log archive target (POST and GET /logs/data-setup/v2),
// as ws-archive serves it. It is checked against recorded answers of the real API
// (archive_logs_fidelity_test.go). A company has one target. Get without one returns an empty IBM
// COS target. Set validates the bucket (400), rejects a region other than the server region (408),
// stores the server region when the request has none, deletes the role when the request has none,
// keeps enableTags of the previous target, and derives the external id. The response, like the
// REST gateway, has every scalar field of the archive spec, also false.
type alFake struct {
	mu       sync.Mutex
	target   map[string]any // the stored S3 target, nil when the company has none
	requests []grRequest
	// getFail, when set, answers every GET instead of the stored target.
	getFail func() (int, any)
	// setFail, when set, answers every POST instead of storing the target.
	setFail func() (int, any)
}

func newALFake(t *testing.T) *alFake {
	t.Helper()
	f := &alFake{}
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

// seed stores a target as if another client had set it.
func (f *alFake) seed(s3 map[string]any, active bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.target = map[string]any{"s3": s3, "isActive": active, "enableTags": false}
}

// failGets makes every GET answer with the given error.
func (f *alFake) failGets(status int, prefix, message string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.getFail = func() (int, any) { return alFail(status, prefix, message) }
}

// failSets makes every POST answer with the given error.
func (f *alFake) failSets(status int, prefix, message string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.setFail = func() (int, any) { return alFail(status, prefix, message) }
}

// forget removes the target, as a company that never had one.
func (f *alFake) forget() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.target = nil
}

func (f *alFake) takeRequests() []grRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := f.requests
	f.requests = nil
	return out
}

func (f *alFake) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

func (f *alFake) handle(method, path string, body any) (int, any) {
	if !strings.HasSuffix(path, alPath) {
		return grFail(http.StatusNotFound, "unknown path "+path)
	}
	switch method {
	case http.MethodGet:
		if f.getFail != nil {
			return f.getFail()
		}
		return http.StatusOK, map[string]any{"target": f.present()}
	case http.MethodPost:
		if f.setFail != nil {
			return f.setFail()
		}
		return f.set(body)
	}
	return grFail(http.StatusMethodNotAllowed, "unsupported "+method+" "+path)
}

func (f *alFake) set(body any) (int, any) {
	in, _ := body.(map[string]any)
	s3, _ := in["s3"].(map[string]any)
	bucket, _ := s3["bucket"].(string)
	if !alBucketName.MatchString(bucket) {
		return alFail(http.StatusBadRequest, "Bad Request", `"targetSpec" does not match any of the allowed types`)
	}
	region, _ := s3["region"].(string)
	if region != "" && region != alServerRegion {
		return alFail(http.StatusRequestTimeout, "Deadline Exceeded", "Region "+region+" is not supported, must use "+alServerRegion+".")
	}
	active, _ := in["isActive"].(bool)
	stored := map[string]any{"bucket": bucket, "region": alServerRegion}
	if role, _ := s3["roleArn"].(string); role != "" {
		stored["roleArn"] = role
	}
	enableTags := false
	if f.target != nil {
		enableTags, _ = f.target["enableTags"].(bool)
	}
	f.target = map[string]any{"s3": stored, "isActive": active, "enableTags": enableTags}
	return http.StatusOK, map[string]any{"target": f.present()}
}

// alFail is an error answer of ws-archive through the REST gateway: the gRPC status text and the message.
func alFail(status int, prefix, message string) (int, any) {
	return status, map[string]any{"code": status, "message": prefix + ": " + message}
}

func (f *alFake) present() map[string]any {
	spec := map[string]any{"archivingFormatId": "cx_data_v1", "format": "TARGET_FORMAT_CX_DATA_V1", "enableTags": false}
	if f.target == nil {
		spec["isActive"] = false
		return map[string]any{"ibmCos": map[string]any{"bucketCrn": "", "endpoint": ""}, "archiveSpec": spec}
	}
	spec["isActive"], spec["enableTags"] = f.target["isActive"], f.target["enableTags"]
	s3 := mapsClone(f.target["s3"].(map[string]any))
	s3["externalId"] = "external-id-of-the-company"
	return map[string]any{"s3": s3, "archiveSpec": spec}
}
