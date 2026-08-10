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

func TestGrpcTargetFromDomain(t *testing.T) {
	t.Parallel()

	tests := []struct {
		domain string
		want   string
	}{
		{
			domain: "api.private.eu2.coralogix.com",
			want:   "api.private.eu2.coralogix.com:443",
		},
		{
			domain: "https://api.private.us1.coralogix.com/",
			want:   "api.private.us1.coralogix.com:443",
		},
		{
			domain: "custom.example.com",
			want:   "ng-api-grpc.custom.example.com:443",
		},
		{
			domain: "api.eu2.coralogix.com",
			want:   "ng-api-grpc.api.eu2.coralogix.com:443",
		},
	}

	for _, tt := range tests {
		t.Run(tt.domain, func(t *testing.T) {
			t.Parallel()
			if got := GrpcTargetFromDomain(tt.domain); got != tt.want {
				t.Fatalf("GrpcTargetFromDomain(%q) = %q, want %q", tt.domain, got, tt.want)
			}
		})
	}
}

func TestScimRestBaseURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		regionOrDomain string
		want           string
	}{
		// All 8 regions, short and long aliases.
		{regionOrDomain: "us1", want: "https://api.coralogix.us"},
		{regionOrDomain: "usa1", want: "https://api.coralogix.us"},
		{regionOrDomain: "us2", want: "https://api.cx498.coralogix.com"},
		{regionOrDomain: "usa2", want: "https://api.cx498.coralogix.com"},
		{regionOrDomain: "us3", want: "https://api.us3.coralogix.com"},
		{regionOrDomain: "usa3", want: "https://api.us3.coralogix.com"},
		{regionOrDomain: "eu1", want: "https://api.coralogix.com"},
		{regionOrDomain: "europe1", want: "https://api.coralogix.com"},
		{regionOrDomain: "eu2", want: "https://api.eu2.coralogix.com"},
		{regionOrDomain: "europe2", want: "https://api.eu2.coralogix.com"},
		{regionOrDomain: "ap1", want: "https://api.app.coralogix.in"},
		{regionOrDomain: "apac1", want: "https://api.app.coralogix.in"},
		{regionOrDomain: "ap2", want: "https://api.coralogixsg.com"},
		{regionOrDomain: "apac2", want: "https://api.coralogixsg.com"},
		{regionOrDomain: "ap3", want: "https://api.ap3.coralogix.com"},
		{regionOrDomain: "apac3", want: "https://api.ap3.coralogix.com"},
		// Region codes are case-insensitive.
		{regionOrDomain: "EU2", want: "https://api.eu2.coralogix.com"},
		{regionOrDomain: "EUROPE2", want: "https://api.eu2.coralogix.com"},
		// Custom domains get the api. prefix.
		{regionOrDomain: "custom.example.com", want: "https://api.custom.example.com"},
		// A domain that already carries the prefix must not become api.api.<domain>.
		{regionOrDomain: "api.eu2.coralogix.com", want: "https://api.eu2.coralogix.com"},
		// PrivateLink hosts are used as-is.
		{regionOrDomain: "api.private.eu2.coralogix.com", want: "https://api.private.eu2.coralogix.com"},
		{regionOrDomain: "api.private.eu1.coralogix.com", want: "https://api.private.eu1.coralogix.com"},
		// Scheme and trailing slash are trimmed.
		{regionOrDomain: "https://custom.example.com/", want: "https://api.custom.example.com"},
		{regionOrDomain: "https://api.private.us1.coralogix.com/", want: "https://api.private.us1.coralogix.com"},
	}

	for _, tt := range tests {
		t.Run(tt.regionOrDomain, func(t *testing.T) {
			t.Parallel()
			if got := ScimRestBaseURL(tt.regionOrDomain); got != tt.want {
				t.Fatalf("ScimRestBaseURL(%q) = %q, want %q", tt.regionOrDomain, got, tt.want)
			}
		})
	}
}
