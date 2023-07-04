package coralogix

import (
	"context"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-mux/tf5to6server"
	"github.com/hashicorp/terraform-plugin-mux/tf6muxserver"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

var testAccProvider *schema.Provider
var testAccProviderFactories map[string]func() (*schema.Provider, error)

//func init() {
//	testAccProvider = Provider()
//	testAccProviderFactories = map[string]func() (*schema.Provider, error){
//		"coralogix": func() (*schema.Provider, error) {
//			return testAccProvider, nil
//		},
//	}
//}
//
//func TestProvider(t *testing.T) {
//	provider := Provider()
//	if err := provider.InternalValidate(); err != nil {
//		t.Fatalf("err: %s", err)
//	}
//}
//
//func TestProvider_impl(t *testing.T) {
//	var _ = Provider()
//}

func testAccPreCheck(t *testing.T) {
	//ctx := context.TODO()

	if os.Getenv("CORALOGIX_API_KEY") == "" {
		t.Fatalf("CORALOGIX_API_KEY must be set for acceptance tests")
	}

	if os.Getenv("CORALOGIX_ENV") == "" {
		t.Fatalf("CORALOGIX_ENV must be set for acceptance tests")
	}

	//diags := testAccProvider.Configure(ctx, terraform.NewResourceConfigRaw(nil))
	//if diags.HasError() {
	//	t.Fatal(diags[0].Summary)
	//}
}

func testProvider() map[string]func() (tfprotov6.ProviderServer, error) {
	return map[string]func() (tfprotov6.ProviderServer, error){
		"coralogix": func() (tfprotov6.ProviderServer, error) {
			ctx := context.Background()

			oldProvider, _ := tf5to6server.UpgradeServer(ctx, Provider().GRPCProvider)

			providers := []func() tfprotov6.ProviderServer{
				providerserver.NewProtocol6(NewCoralogixProvider()),
				func() tfprotov6.ProviderServer { return oldProvider },
			}

			muxServer, err := tf6muxserver.NewMuxServer(ctx, providers...)

			if err != nil {
				return nil, err
			}

			return muxServer.ProviderServer(), nil
		},
	}
}
