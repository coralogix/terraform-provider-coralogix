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

package dashboards

import (
	"context"
	"net/http"

	cxsdk "github.com/coralogix/coralogix-management-sdk/go/openapi/cxsdk"
	dbfs "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/dashboard_folders_service"
	"github.com/coralogix/terraform-provider-coralogix/internal/provider/generated/dashboardsfolder"

	"github.com/hashicorp/terraform-plugin-framework/resource"
)

// NewDashboardsFolderResource returns coralogix_dashboards_folder, which is generated from the
// API contract (internal/provider/generated/dashboardsfolder).
func NewDashboardsFolderResource() resource.Resource {
	return dashboardsfolder.NewResource(dashboardsfolder.Hooks{UpdateGone: folderGone, DeleteGone: folderGone})
}

// folderGone tells an update or a delete of a folder that does not exist from a real failure. The
// API answers a replace of a missing folder with 400 ("folder with id ... does not exist") and a
// delete with 403 ("dont have permissions to delete folder"), not 404. The same statuses mean an
// invalid request or a missing permission for a folder that exists, so the folder is read: only a
// 404 there means that it is gone.
func folderGone(ctx context.Context, client *dbfs.DashboardFoldersServiceAPIService, id string, err error) bool {
	if code := cxsdk.Code(err); code != http.StatusBadRequest && code != http.StatusForbidden {
		return false
	}
	_, httpResp, getErr := client.DashboardFoldersServiceGetDashboardFolder(ctx, id).Execute()
	return getErr != nil && cxsdk.IsNotFound(cxsdk.NewAPIError(httpResp, getErr))
}
