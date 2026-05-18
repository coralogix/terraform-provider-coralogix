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

package clientset

import "testing"

func TestNormalizeBaseHost(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{name: "bare", in: "factset.coralogix.com", want: "factset.coralogix.com"},
		{name: "api prefix", in: "api.factset.coralogix.com", want: "factset.coralogix.com"},
		{name: "uppercase api prefix", in: "API.FACTSET.coralogix.com", want: "factset.coralogix.com"},
		{name: "with https scheme", in: "https://api.factset.coralogix.com", want: "factset.coralogix.com"},
		{name: "with scheme and trailing slash", in: "https://api.factset.coralogix.com/", want: "factset.coralogix.com"},
		{name: "with path", in: "https://api.factset.coralogix.com/mgmt/openapi/4", want: "factset.coralogix.com"},
		{name: "with port", in: "api.factset.coralogix.com:443", want: "factset.coralogix.com"},
		{name: "with scheme host port path", in: "https://api.factset.coralogix.com:443/mgmt/openapi/4", want: "factset.coralogix.com"},
		{name: "surrounding whitespace", in: "  api.factset.coralogix.com  ", want: "factset.coralogix.com"},
		{name: "known region bare", in: "eu1.coralogix.com", want: "eu1.coralogix.com"},
		{name: "known region api", in: "api.eu1.coralogix.com", want: "eu1.coralogix.com"},

		{name: "empty", in: "", wantErr: true},
		{name: "whitespace only", in: "   ", wantErr: true},
		{name: "only api prefix", in: "api.", wantErr: true},
		{name: "only dots", in: "...", wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NormalizeBaseHost(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("NormalizeBaseHost(%q) = %q, want error", tc.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("NormalizeBaseHost(%q) unexpected error: %v", tc.in, err)
			}
			if got != tc.want {
				t.Fatalf("NormalizeBaseHost(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestResolveCustomDomainHosts(t *testing.T) {
	cases := []struct {
		name        string
		in          string
		wantBase    string
		wantGrpc    string
		wantOpenAPI string
		wantErr     bool
	}{
		{
			name:        "bare base host",
			in:          "factset.coralogix.com",
			wantBase:    "factset.coralogix.com",
			wantGrpc:    "ng-api-grpc.factset.coralogix.com:443",
			wantOpenAPI: "api.factset.coralogix.com",
		},
		{
			name:        "api prefixed host",
			in:          "api.factset.coralogix.com",
			wantBase:    "factset.coralogix.com",
			wantGrpc:    "ng-api-grpc.factset.coralogix.com:443",
			wantOpenAPI: "api.factset.coralogix.com",
		},
		{
			name:        "https URL form",
			in:          "https://api.factset.coralogix.com",
			wantBase:    "factset.coralogix.com",
			wantGrpc:    "ng-api-grpc.factset.coralogix.com:443",
			wantOpenAPI: "api.factset.coralogix.com",
		},
		{
			name:        "URL with port and path",
			in:          "https://api.factset.coralogix.com:443/mgmt/openapi/5",
			wantBase:    "factset.coralogix.com",
			wantGrpc:    "ng-api-grpc.factset.coralogix.com:443",
			wantOpenAPI: "api.factset.coralogix.com",
		},
		{
			name:        "known region treated as domain",
			in:          "eu1.coralogix.com",
			wantBase:    "eu1.coralogix.com",
			wantGrpc:    "ng-api-grpc.eu1.coralogix.com:443",
			wantOpenAPI: "api.eu1.coralogix.com",
		},
		{name: "empty domain errors", in: "", wantErr: true},
		{name: "only api prefix errors", in: "api.", wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			base, grpc, openapi, err := resolveCustomDomainHosts(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("resolveCustomDomainHosts(%q) = (%q,%q,%q), want error", tc.in, base, grpc, openapi)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveCustomDomainHosts(%q) unexpected error: %v", tc.in, err)
			}
			if base != tc.wantBase {
				t.Errorf("base = %q, want %q", base, tc.wantBase)
			}
			if grpc != tc.wantGrpc {
				t.Errorf("grpc = %q, want %q", grpc, tc.wantGrpc)
			}
			if openapi != tc.wantOpenAPI {
				t.Errorf("openapi = %q, want %q", openapi, tc.wantOpenAPI)
			}
		})
	}
}
