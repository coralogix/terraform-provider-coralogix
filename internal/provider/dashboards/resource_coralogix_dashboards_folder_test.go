package dashboards

import (
	"context"
	"testing"

	dbfs "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/dashboard_folders_service"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// The SDK rejects a malformed folder ID before sending the request and returns
// a nil *http.Response. Read must report an error instead of panicking.
func TestDashboardsFolderReadNilHTTPResponse(t *testing.T) {
	ctx := context.Background()
	r := &DashboardsFolderResource{client: dbfs.NewAPIClient(dbfs.NewConfiguration()).DashboardFoldersServiceAPI}

	var schemaResp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)
	state := tfsdk.State{Schema: schemaResp.Schema}
	if diags := state.Set(ctx, DashboardsFolderResourceModel{
		ID:       types.StringValue("not-a-uuid"),
		Name:     types.StringValue("folder"),
		ParentId: types.StringNull(),
	}); diags.HasError() {
		t.Fatalf("set state: %v", diags)
	}

	resp := &resource.ReadResponse{State: state}
	r.Read(ctx, resource.ReadRequest{State: state}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatalf("expected an error diagnostic, got %v", resp.Diagnostics)
	}
}
