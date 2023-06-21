package main

import (
	"context"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6/tf6server"
	"github.com/hashicorp/terraform-plugin-mux/tf5to6server"
	"github.com/hashicorp/terraform-plugin-mux/tf6muxserver"
	"terraform-provider-coralogix/coralogix"
)

// Generate the Terraform provider documentation using `tfplugindocs`:
//
//go:generate go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs
func main() {
	ctx := context.Background()

	oldProvider, _ := tf5to6server.UpgradeServer(ctx, coralogix.Provider().GRPCProvider)

	providers := []func() tfprotov6.ProviderServer{
		providerserver.NewProtocol6(coralogix.NewCoralogixProvider()),
		func() tfprotov6.ProviderServer { return oldProvider },
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

	//providerserver.Serve(context.Background(), coralogix.NewCoralogixProvider, providerserver.ServeOpts{
	//	// NOTE: This is not a typical Terraform Registry provider address,
	//	// such as registry.terraform.io/hashicorp/hashicups. This specific
	//	// provider address is used in these tutorials in conjunction with a
	//	// specific Terraform CLI configuration for manual development testing
	//	// of this provider.
	//	Address: "coralogix.com/coralogix/coralogix",
	//})

}
