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

package rest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestGetSuccess(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"1"}`))
	}))
	defer server.Close()

	body, err := NewRestClient(server.URL, "api-key").Get(context.Background(), "/1")
	if err != nil {
		t.Fatalf("Get() returned error %v, want nil", err)
	}
	if body != `{"id":"1"}` {
		t.Fatalf("Get() = %q, want %q", body, `{"id":"1"}`)
	}
}

func TestGetNotFound(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	_, err := NewRestClient(server.URL, "api-key").Get(context.Background(), "/missing")
	if err == nil {
		t.Fatal("Get() returned nil error, want not-found error")
	}
	// REST call sites use status.Code, not cxsdk.Code.
	if got := status.Code(err); got != codes.NotFound {
		t.Fatalf("status.Code(%v) = %v, want %v", err, got, codes.NotFound)
	}
}
