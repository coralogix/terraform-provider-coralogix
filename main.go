package main

import (
	"github.com/coralogix/terraform-provider-coralogix/coralogix"
	"github.com/hashicorp/terraform-plugin-sdk/v2/plugin"
)

func main() {
	plugin.Serve(&plugin.ServeOpts{ProviderFunc: coralogix.Provider})
}
