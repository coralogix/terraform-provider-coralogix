package main

import (
	"context"
	"log"

	"terraform-provider-coralogix/coralogix"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6/tf6server"
	"github.com/hashicorp/terraform-plugin-mux/tf5to6server"
	"github.com/hashicorp/terraform-plugin-mux/tf6muxserver"
)

// Generate the Terraform provider documentation using `tfplugindocs`:
//
//go:generate go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs
func main() {
	ctx := context.Background()

	oldProvider, _ := tf5to6server.UpgradeServer(ctx, coralogix.OldProvider().GRPCProvider)

	providers := []func() tfprotov6.ProviderServer{
		func() tfprotov6.ProviderServer { return oldProvider },
		providerserver.NewProtocol6(coralogix.NewCoralogixProvider()),
	}

	muxServer, err := tf6muxserver.NewMuxServer(ctx, providers...)

	if err != nil {
		log.Fatal(err)
	}

	var serveOpts []tf6server.ServeOpt

	err = tf6server.Serve(
		"registry.terraform.io/coralogix/coralogix",
		muxServer.ProviderServer,
		serveOpts...,
	)

	if err != nil {
		log.Fatal(err)
	}
}
