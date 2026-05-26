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

func normalizeProviderDomain(domain string) string {
	domain = strings.TrimSpace(domain)
	domain = strings.TrimPrefix(domain, "https://")
	domain = strings.TrimPrefix(domain, "http://")
	return strings.TrimSuffix(domain, "/")
}
