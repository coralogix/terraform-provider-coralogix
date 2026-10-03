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
	"context"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-mux/tf5to6server"
	"github.com/hashicorp/terraform-plugin-mux/tf6muxserver"
)

// MuxServer serves the SDKv2 provider and the framework provider behind one mux, as the provider
// binary does. Tests that use it see every resource of the provider.
func MuxServer(ctx context.Context) (func() tfprotov6.ProviderServer, error) {
	oldProvider, err := tf5to6server.UpgradeServer(ctx, OldProvider().GRPCProvider)
	if err != nil {
		return nil, err
	}
	mux, err := tf6muxserver.NewMuxServer(ctx,
		func() tfprotov6.ProviderServer { return oldProvider },
		providerserver.NewProtocol6(NewCoralogixProvider()),
	)
	if err != nil {
		return nil, err
	}
	return mux.ProviderServer, nil
}
