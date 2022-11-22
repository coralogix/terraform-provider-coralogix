package main

import (
	"terraform-provider-coralogix-v2/coralogix"

	"github.com/hashicorp/terraform-plugin-sdk/v2/plugin"
)

func main() {
	plugin.Serve(&plugin.ServeOpts{
		ProviderFunc: coralogix.Provider,
	})
}
