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
	"errors"
	"fmt"
	"strings"
	"unicode"
)

const (
	maxDomainLength = 253
	maxLabelLength  = 63
)

// GrpcTargetFromDomain returns the host:port used for gRPC management API calls when the
// provider is configured with domain (CORALOGIX_DOMAIN).
//
// AWS PrivateLink exposes management REST and gRPC on api.private.<region>.coralogix.com
// (see Coralogix endpoints docs). The public SaaS pattern ng-api-grpc.<domain> does not
// apply to api.private.* hostnames.
//
// A domain that is not a structurally valid hostname returns an error instead of a
// malformed target such as "ng-api-grpc.:443", which surfaces as an opaque dial failure.
func GrpcTargetFromDomain(domain string) (string, error) {
	host, err := normalizeAndValidateProviderDomain(domain)
	if err != nil {
		return "", err
	}
	if strings.HasPrefix(host, "api.private.") {
		return host + ":443", nil
	}
	return fmt.Sprintf("ng-api-grpc.%s:443", host), nil
}

// normalizeAndValidateProviderDomain normalizes domain and checks that the result is a
// structurally valid hostname. The check is structural only: any host that DNS could
// resolve is accepted, including PrivateLink and customer-specific private hosts.
func normalizeAndValidateProviderDomain(domain string) (string, error) {
	host := normalizeProviderDomain(domain)
	if host == "" {
		return "", invalidDomainError(domain, "it is empty after trimming whitespace, a scheme prefix and a trailing \"/\"")
	}
	if len(host) > maxDomainLength {
		return "", invalidDomainError(domain, fmt.Sprintf("it is %d characters long, above the %d character hostname limit", len(host), maxDomainLength))
	}
	for _, label := range strings.Split(strings.TrimSuffix(host, "."), ".") {
		if err := validateHostnameLabel(label); err != nil {
			return "", invalidDomainError(domain, err.Error())
		}
	}
	return host, nil
}

func validateHostnameLabel(label string) error {
	if label == "" {
		return errors.New("it contains an empty label (a leading, trailing or doubled \".\")")
	}
	if len(label) > maxLabelLength {
		return fmt.Errorf("the label %q is %d characters long, above the %d character label limit", label, len(label), maxLabelLength)
	}
	if strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
		return fmt.Errorf("the label %q starts or ends with \"-\"", label)
	}
	// Underscores and non-ASCII names are not conformant hostnames but do resolve in
	// internal DNS zones, so the check rejects only what cannot be a host at all.
	for _, r := range label {
		if !isHostnameLabelChar(r) {
			return fmt.Errorf("the label %q contains the invalid character %q", label, r)
		}
	}
	return nil
}

func isHostnameLabelChar(r rune) bool {
	switch {
	case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		return true
	case r == '-', r == '_':
		return true
	case r > unicode.MaxASCII:
		return !unicode.IsSpace(r)
	default:
		return false
	}
}

func invalidDomainError(domain, reason string) error {
	return fmt.Errorf("invalid Coralogix domain %q: %s. The provider \"domain\" argument, or the CORALOGIX_DOMAIN environment variable, must be a hostname such as \"coralogix.com\", \"eu2.coralogix.com\" or \"api.private.eu2.coralogix.com\"; an \"https://\" prefix and a trailing \"/\" are accepted", domain, reason)
}

// ScimRestBaseURL returns the HTTPS base URL for SCIM REST APIs (users, groups) for the
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
