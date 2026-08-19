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
)

// GrpcTargetFromDomain returns the host:port used for gRPC management API calls when the
// provider is configured with domain (CORALOGIX_DOMAIN).
//
// AWS PrivateLink exposes management REST and gRPC on api.private.<region>.coralogix.com
// (see Coralogix endpoints docs). The public SaaS pattern ng-api-grpc.<domain> does not
// apply to api.private.* hostnames.
func GrpcTargetFromDomain(domain string) string {
	domain = normalizeProviderDomain(domain)
	if strings.HasPrefix(domain, "api.private.") {
		return domain + ":443"
	}
	return fmt.Sprintf("ng-api-grpc.%s:443", domain)
}

// ScimRestBaseURL returns the HTTPS base URL for SCIM REST APIs (users) for the
// given provider env or domain. PrivateLink management hosts use api.private.* directly;
// public regions use api.* (same host family as the OpenAPI management clients).
// Note: CoralogixRestEndpointFromRegion still returns ng-api-http.*; SCIM is on api.*.
func ScimRestBaseURL(regionOrDomain string) string {
	regionOrDomain = normalizeProviderDomain(regionOrDomain)
	if strings.HasPrefix(regionOrDomain, "api.private.") {
		return "https://" + regionOrDomain
	}

	switch strings.ToLower(regionOrDomain) {
	case "us1", "usa1":
		return "https://api.coralogix.us"
	case "us2", "usa2":
		return "https://api.cx498.coralogix.com"
	case "us3", "usa3":
		return "https://api.us3.coralogix.com"
	case "eu1", "europe1":
		return "https://api.coralogix.com"
	case "eu2", "europe2":
		return "https://api.eu2.coralogix.com"
	case "ap1", "apac1":
		return "https://api.app.coralogix.in"
	case "ap2", "apac2":
		return "https://api.coralogixsg.com"
	case "ap3", "apac3":
		return "https://api.ap3.coralogix.com"
	default:
		// Mirrors the SDK's normalizeCoralogixDomain: a domain that already carries the
		// api. prefix must not become api.api.<domain>.
		return fmt.Sprintf("https://api.%s", strings.TrimPrefix(regionOrDomain, "api."))
	}
}

func normalizeProviderDomain(domain string) string {
	domain = strings.TrimSpace(domain)
	domain = strings.TrimPrefix(domain, "https://")
	domain = strings.TrimPrefix(domain, "http://")
	return strings.TrimSuffix(domain, "/")
}
