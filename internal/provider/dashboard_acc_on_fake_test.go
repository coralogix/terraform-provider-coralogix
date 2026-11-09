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

package provider

import "os"

// The dashboard acceptance tests pass against the real API. With DASHBOARD_FAKE_ACC=1 they run
// against the fake instead, with a placeholder key and no network. A test that fails there names a
// place where the fake, or the behavior recorded in the baseline, differs from what the real API
// tests expect. See testdata/dashboard_baseline/README.md.
//
//	DASHBOARD_FAKE_ACC=1 TF_ACC=1 CORALOGIX_API_KEY=placeholder CORALOGIX_DOMAIN=eu2.coralogix.com \
//	  go test ./internal/provider -run 'TestAccCoralogix.*Dashboard' -count=1
func init() {
	if os.Getenv("DASHBOARD_FAKE_ACC") == "1" {
		startDashboardFake()
	}
}
