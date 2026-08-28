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

import (
	"fmt"
	"strings"
	"testing"
)

// mustGrpcTargetFromDomain is the test-side helper for the domains that must stay valid.
func mustGrpcTargetFromDomain(t *testing.T, domain string) string {
	t.Helper()
	target, err := GrpcTargetFromDomain(domain)
	if err != nil {
		t.Fatalf("GrpcTargetFromDomain(%q) returned an unexpected error: %v", domain, err)
	}
	return target
}

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
		// Customer-specific PrivateLink hosts are accepted without any suffix assumption.
		{
			domain: "cx.private.internal.customer-corp.net",
			want:   "ng-api-grpc.cx.private.internal.customer-corp.net:443",
		},
		// Scheme and trailing slash are tolerated, including on a single-label host.
		{
			domain: "https://host/",
			want:   "ng-api-grpc.host:443",
		},
		{
			domain: "http://coralogix.com",
			want:   "ng-api-grpc.coralogix.com:443",
		},
		{
			domain: "  api.eu2.coralogix.com  ",
			want:   "ng-api-grpc.api.eu2.coralogix.com:443",
		},
		// Case is preserved, not rejected.
		{
			domain: "EU2.Coralogix.COM",
			want:   "ng-api-grpc.EU2.Coralogix.COM:443",
		},
		// A fully qualified name with a root dot is not a false rejection.
		{
			domain: "coralogix.com.",
			want:   "ng-api-grpc.coralogix.com.:443",
		},
		// Names that resolve in internal DNS zones without being conformant hostnames.
		{
			domain: "cx_api.internal.customer-corp.net",
			want:   "ng-api-grpc.cx_api.internal.customer-corp.net:443",
		},
		{
			domain: "münchen.customer-corp.de",
			want:   "ng-api-grpc.münchen.customer-corp.de:443",
		},
		{
			domain: "192.168.10.20",
			want:   "ng-api-grpc.192.168.10.20:443",
		},
	}

	for _, tt := range tests {
		t.Run(tt.domain, func(t *testing.T) {
			t.Parallel()
			got, err := GrpcTargetFromDomain(tt.domain)
			if err != nil {
				t.Fatalf("GrpcTargetFromDomain(%q) returned an unexpected error: %v", tt.domain, err)
			}
			if got != tt.want {
				t.Fatalf("GrpcTargetFromDomain(%q) = %q, want %q", tt.domain, got, tt.want)
			}
		})
	}
}

func TestGrpcTargetFromDomainRejectsMalformedDomains(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		domain string
	}{
		{name: "empty", domain: ""},
		{name: "single space", domain: " "},
		{name: "whitespace only", domain: "\t\n"},
		{name: "scheme only", domain: "https://"},
		{name: "trailing slash only", domain: "/"},
		{name: "leading dot", domain: ".coralogix.com"},
		{name: "doubled dot", domain: "api..coralogix.com"},
		{name: "explicit port", domain: "api.eu2.coralogix.com:443"},
		{name: "path component", domain: "https://api.eu2.coralogix.com/path"},
		{name: "userinfo", domain: "user@coralogix.com"},
		{name: "leading hyphen label", domain: "-coralogix.com"},
		{name: "trailing hyphen label", domain: "coralogix-.com"},
		{name: "space inside", domain: "api eu2.coralogix.com"},
		{name: "non-breaking space", domain: "\u00a0"},
		{name: "label too long", domain: strings.Repeat("a", 64) + ".coralogix.com"},
		{name: "domain too long", domain: strings.Repeat("a.", 128) + "coralogix.com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := GrpcTargetFromDomain(tt.domain)
			if err == nil {
				t.Fatalf("GrpcTargetFromDomain(%q) = %q, want an error", tt.domain, got)
			}
			if got != "" {
				t.Fatalf("GrpcTargetFromDomain(%q) returned target %q alongside an error", tt.domain, got)
			}
			// The message has to be actionable from either configuration path,
			// and has to echo the value that was rejected.
			for _, want := range []string{"domain", "CORALOGIX_DOMAIN", fmt.Sprintf("%q", tt.domain)} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q does not mention %s", err, want)
				}
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
