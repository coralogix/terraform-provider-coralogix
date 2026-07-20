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

func TestNewClientSet_UsersClientNotNil(t *testing.T) {
	t.Parallel()

	cs := NewClientSet("eu2", "dummy-key", GrpcTargetFromDomain("eu2.coralogix.com"))
	if cs.Users() == nil {
		t.Fatal("Users() must not be nil")
	}
	if cs.Dashboards() == nil {
		t.Fatal("Dashboards() must not be nil")
	}
	if cs.Users().BaseURL() != "https://ng-api-http.eu2.coralogix.com/scim/Users" {
		t.Fatalf("Users().BaseURL() = %q", cs.Users().BaseURL())
	}

	pl := NewClientSet("api.private.eu2.coralogix.com", "dummy-key", GrpcTargetFromDomain("api.private.eu2.coralogix.com"))
	if pl.Users().BaseURL() != "https://api.private.eu2.coralogix.com/scim/Users" {
		t.Fatalf("PrivateLink Users().BaseURL() = %q", pl.Users().BaseURL())
	}
	if pl.Groups().TargetUrl != "https://api.private.eu2.coralogix.com/scim/Groups" {
		t.Fatalf("PrivateLink Groups TargetUrl = %q", pl.Groups().TargetUrl)
	}
}
