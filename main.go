package main

import (
	"terraform-provider-coralogix/coralogix"

	"github.com/hashicorp/terraform-plugin-sdk/v2/plugin"
)

// Generate the Terraform provider documentation using `tfplugindocs`:
//
//go:generate go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs
func main() {
	plugin.Serve(&plugin.ServeOpts{
		ProviderFunc: coralogix.Provider,
	})
}
