// Copyright 2026 Coralogix Ltd.
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
	"errors"
	"sync"

	cxsdkOpenapi "github.com/coralogix/coralogix-management-sdk/go/openapi/cxsdk"
)

// teamIDCache holds the team id WhoAmI resolved from the configured API key.
type teamIDCache struct {
	mu     sync.Mutex
	teamID int64
}

// TeamID returns the team the configured API key belongs to. Every Users API path
// contains a team id, and WhoAmI is the only way to derive it from an API key.
//
// The result is cached on the client set, which the provider builds inside Configure
// from the key it just read. The cache therefore lives exactly as long as the key it
// was resolved with: a different key means a new Configure, a new client set and an
// empty cache. Provider aliases with different keys each get their own. A package-level
// variable would be wrong for both reasons.
//
// Resolution is lazy so that a run touching no users pays nothing, and so that a
// region where WhoAmI is not yet reachable fails only the user resources rather than
// the whole provider. A failure is not cached, so a transient error does not poison
// the rest of the run. The mutex is only here because Terraform calls resources
// concurrently.
func (c *ClientSet) TeamID(ctx context.Context) (int64, error) {
	c.teamID.mu.Lock()
	defer c.teamID.mu.Unlock()

	if c.teamID.teamID != 0 {
		return c.teamID.teamID, nil
	}

	resp, httpResp, err := c.identity.IdentityServiceWhoAmI(ctx).Execute()
	if err != nil {
		return 0, cxsdkOpenapi.NewAPIError(httpResp, err)
	}
	if resp == nil || resp.TeamId == 0 {
		return 0, errors.New("whoami returned no team id")
	}

	c.teamID.teamID = resp.TeamId
	return c.teamID.teamID, nil
}
