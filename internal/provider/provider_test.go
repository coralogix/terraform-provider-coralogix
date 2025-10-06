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

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

var testAccProvider *schema.Provider
var testAccProviderFactories map[string]func() (*schema.Provider, error)
var testAccProtoV6ProviderFactories map[string]func() (tfprotov6.ProviderServer, error)

func Init() {
	testAccProvider = OldProvider()
	testAccProviderFactories = map[string]func() (*schema.Provider, error){
		"coralogix": func() (*schema.Provider, error) {
			return testAccProvider, nil
		},
	}
	testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
		"coralogix": providerserver.NewProtocol6WithError(NewCoralogixProvider()),
	}
}

func TestProvider(t *testing.T) {
	provider := OldProvider()
	if err := provider.InternalValidate(); err != nil {
		t.Fatalf("err: %s", err)
	}
}

func TestProvider_impl(t *testing.T) {
	var _ = OldProvider()
}

func testAccPreCheck(t *testing.T) {
	//ctx := context.TODO()

	if os.Getenv("CORALOGIX_API_KEY") == "" {
		t.Fatalf("CORALOGIX_API_KEY must be set for acceptance tests")
	}

	if os.Getenv("CORALOGIX_ENV") == "" && os.Getenv("CORALOGIX_DOMAIN") == "" {
		t.Fatalf("CORALOGIX_ENV or CORALOGIX_DOMAIN must be set for acceptance tests")
	}
}
