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

	slis "terraform-provider-coralogix/coralogix/clientset/grpc/sli"
)

type SLIClient struct {
	callPropertiesCreator *CallPropertiesCreator
}

func (c SLIClient) CreateSLI(ctx context.Context, req *slis.CreateSliRequest) (*slis.CreateSliResponse, error) {
	callProperties, err := c.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := slis.NewSliServiceClient(conn)

	return client.CreateSli(callProperties.Ctx, req, callProperties.CallOptions...)
}

func (c SLIClient) GetSLIs(ctx context.Context, req *slis.GetSlisRequest) (*slis.GetSlisResponse, error) {
	callProperties, err := c.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := slis.NewSliServiceClient(conn)

	return client.GetSlis(callProperties.Ctx, req, callProperties.CallOptions...)
}

func (c SLIClient) UpdateSLI(ctx context.Context, req *slis.UpdateSliRequest) (*slis.UpdateSliResponse, error) {
	callProperties, err := c.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := slis.NewSliServiceClient(conn)

	return client.UpdateSli(callProperties.Ctx, req, callProperties.CallOptions...)
}

func (c SLIClient) DeleteSLI(ctx context.Context, req *slis.DeleteSliRequest) (*slis.DeleteSliResponse, error) {
	callProperties, err := c.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := slis.NewSliServiceClient(conn)

	return client.DeleteSli(callProperties.Ctx, req, callProperties.CallOptions...)
}

func NewSLIsClient(c *CallPropertiesCreator) *SLIClient {
	return &SLIClient{callPropertiesCreator: c}
}
