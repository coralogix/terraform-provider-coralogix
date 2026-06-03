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

import (
	"context"
	"crypto/tls"
	"fmt"
	"runtime"
	"time"

	cxsdk "github.com/coralogix/coralogix-management-sdk/go"
	"github.com/google/uuid"
	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
)

// gRPC metadata header names (must match coralogix-management-sdk/go/constants.go).
const (
	sdkVersionHeaderName       = "x-cx-sdk-version"
	sdkLanguageHeaderName      = "x-cx-sdk-language"
	sdkGoVersionHeaderName     = "x-cx-go-version"
	sdkCorrelationIDHeaderName = "x-cx-correlation-id"
)

// terraformSDKCallPropertiesCreator dials an explicit gRPC target from the provider instead of
// relying on SDK region→endpoint mapping (which prefixes ng-api-grpc. for custom domains).
type terraformSDKCallPropertiesCreator struct {
	grpcTarget       string
	teamsLevelAPIKey string
	userLevelAPIKey  string
	correlationID    string
	sdkVersion       string
}

func newTerraformSDKCallPropertiesCreator(apiKey, terraformProviderVersion, grpcTarget string) cxsdk.CallPropertiesCreator {
	return &terraformSDKCallPropertiesCreator{
		grpcTarget:       grpcTarget,
		teamsLevelAPIKey: apiKey,
		userLevelAPIKey:  apiKey,
		correlationID:    uuid.NewString(),
		sdkVersion:       fmt.Sprint("terraform-", terraformProviderVersion),
	}
}

func (c *terraformSDKCallPropertiesCreator) GetTeamsLevelCallProperties(ctx context.Context) (*cxsdk.CallProperties, error) {
	return c.callProperties(ctx, c.teamsLevelAPIKey)
}

func (c *terraformSDKCallPropertiesCreator) GetUserLevelCallProperties(ctx context.Context) (*cxsdk.CallProperties, error) {
	return c.callProperties(ctx, c.userLevelAPIKey)
}

func (c *terraformSDKCallPropertiesCreator) callProperties(ctx context.Context, apiKey string) (*cxsdk.CallProperties, error) {
	ctx = grpcOutgoingContext(ctx, apiKey, c.correlationID, c.sdkVersion)

	conn, err := grpcSecureConnection(c.grpcTarget)
	if err != nil {
		return nil, err
	}

	return &cxsdk.CallProperties{
		Ctx:         ctx,
		Connection:  conn,
		CallOptions: grpcCallOptions(),
	}, nil
}

func grpcOutgoingContext(ctx context.Context, apiKey, correlationID, sdkVersion string) context.Context {
	md := metadata.New(map[string]string{
		"Authorization":            fmt.Sprintf("Bearer %s", apiKey),
		sdkVersionHeaderName:       sdkVersion,
		sdkLanguageHeaderName:      "go",
		sdkGoVersionHeaderName:     runtime.Version(),
		sdkCorrelationIDHeaderName: correlationID,
	})
	return metadata.NewOutgoingContext(ctx, md)
}

func grpcCallOptions() []grpc.CallOption {
	var opts []grpc.CallOption
	opts = append(opts, grpc_retry.WithMax(5))
	opts = append(opts, grpc_retry.WithBackoff(grpc_retry.BackoffLinear(time.Second)))
	opts = append(opts, grpc.MaxCallRecvMsgSize(50*1024*1024))
	opts = append(opts, grpc.MaxCallSendMsgSize(50*1024*1024))
	return opts
}

func grpcSecureConnection(targetURL string) (*grpc.ClientConn, error) {
	// Match coralogix-management-sdk/go/callPropertiesCreator.go (grpc.Dial for proxy compatibility).
	return grpc.Dial(targetURL,
		grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{})))
}
