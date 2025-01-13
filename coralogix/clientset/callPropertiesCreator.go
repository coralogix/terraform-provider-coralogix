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

	"github.com/google/uuid"
	"google.golang.org/grpc"
)

type CallPropertiesCreator struct {
	targetUrl     string
	apiKey        string
	correlationID string
}

type CallProperties struct {
	Ctx         context.Context
	Connection  *grpc.ClientConn
	CallOptions []grpc.CallOption
}

func NewCallPropertiesCreator(targetUrl, apiKey string) *CallPropertiesCreator {
	return &CallPropertiesCreator{
		targetUrl:     targetUrl,
		apiKey:        apiKey,
		correlationID: uuid.NewString(),
	}
}
