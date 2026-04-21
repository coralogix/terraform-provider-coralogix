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

// NormalizeBaseHost returns the bare tenant host from a Coralogix `domain`
// input, stripping any scheme, path, port and a leading `api.` prefix.
func NormalizeBaseHost(raw string) (string, error) {
	host := strings.TrimSpace(raw)
	if host == "" {
		return "", fmt.Errorf("domain is empty")
	}

	host = strings.ToLower(host)

	if i := strings.Index(host, "://"); i >= 0 {
		host = host[i+3:]
	}

	if i := strings.IndexAny(host, "/?#"); i >= 0 {
		host = host[:i]
	}

	if i := strings.LastIndex(host, "@"); i >= 0 {
		host = host[i+1:]
	}

	if i := strings.LastIndex(host, ":"); i >= 0 {
		host = host[:i]
	}

	host = strings.Trim(host, ".")
	if host == "" {
		return "", fmt.Errorf("domain %q has no host component", raw)
	}

	if !strings.Contains(host, ".") {
		return "", fmt.Errorf("domain %q is not a fully qualified host", raw)
	}

	host = strings.TrimPrefix(host, "api.")
	if host == "" || !strings.Contains(host, ".") {
		return "", fmt.Errorf("domain %q resolves to an invalid base host after stripping the api. prefix", raw)
	}

	return host, nil
}
