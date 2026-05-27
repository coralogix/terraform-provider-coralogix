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

func TestOpenAPIURLFromDomain(t *testing.T) {
	tests := map[string]string{
		"root domain":     "https://api.coralogix.com/mgmt/openapi/5",
		"api domain":      "https://api.coralogix.com/mgmt/openapi/5",
		"https domain":    "https://api.coralogix.com/mgmt/openapi/5",
		"trailing slash":  "https://api.eu2.coralogix.com/mgmt/openapi/5",
		"regional domain": "https://api.eu2.coralogix.com/mgmt/openapi/5",
	}

	inputs := map[string]string{
		"root domain":     "coralogix.com",
		"api domain":      "api.coralogix.com",
		"https domain":    "https://coralogix.com",
		"trailing slash":  "eu2.coralogix.com/",
		"regional domain": "eu2.coralogix.com",
	}

	for name, want := range tests {
		t.Run(name, func(t *testing.T) {
			if got := openAPIURLFromDomain(inputs[name]); got != want {
				t.Fatalf("openAPIURLFromDomain(%q) = %q, want %q", inputs[name], got, want)
			}
		})
	}
}
