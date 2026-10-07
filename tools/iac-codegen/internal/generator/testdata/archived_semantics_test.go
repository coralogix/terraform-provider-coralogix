package archivedthing

import (
	"context"
	"net/http"
	"slices"
	"testing"

	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"example.com/iac-test-sdk/go/openapi/gen/archived_things_service"
)

type statusError int

func (e statusError) Error() string   { return http.StatusText(int(e)) }
func (e statusError) StatusCode() int { return int(e) }

// deleteThing runs the generated Delete on a state with the id, with an archive call that
// returns err.
func deleteThing(t *testing.T, id string, err error) (*frameworkresource.DeleteResponse, []string) {
	t.Helper()
	ctx := context.Background()
	archived_things_service.ArchiveError, archived_things_service.Archived = err, nil
	r := &Resource{client: &archived_things_service.ArchivedThingsServiceAPIService{}}
	var schemaResp frameworkresource.SchemaResponse
	r.Schema(ctx, frameworkresource.SchemaRequest{}, &schemaResp)
	root := schemaResp.Schema.Type().TerraformType(ctx)
	raw := tftypes.NewValue(root, map[string]tftypes.Value{
		"id":   tftypes.NewValue(tftypes.String, id),
		"name": tftypes.NewValue(tftypes.String, "name"),
	})
	resp := &frameworkresource.DeleteResponse{State: tfsdk.State{Schema: schemaResp.Schema, Raw: raw}}
	r.Delete(ctx, frameworkresource.DeleteRequest{State: tfsdk.State{Schema: schemaResp.Schema, Raw: raw}}, resp)
	return resp, archived_things_service.Archived
}

func TestDeleteCallsTheArchiveOperation(t *testing.T) {
	resp, archived := deleteThing(t, "thing-1", nil)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	if !slices.Equal(archived, []string{"thing-1"}) {
		t.Fatalf("archived ids = %v, want [thing-1]", archived)
	}
}

func TestDeleteTreatsNotFoundAsDeleted(t *testing.T) {
	resp, archived := deleteThing(t, "gone", statusError(http.StatusNotFound))
	if resp.Diagnostics.HasError() {
		t.Fatalf("a resource that the API does not find is deleted: %v", resp.Diagnostics)
	}
	if !slices.Equal(archived, []string{"gone"}) {
		t.Fatalf("archived ids = %v, want [gone]", archived)
	}
}

func TestDeleteReportsOtherErrors(t *testing.T) {
	for _, status := range []int{http.StatusPreconditionFailed, http.StatusInternalServerError} {
		resp, _ := deleteThing(t, "thing-1", statusError(status))
		if !resp.Diagnostics.HasError() {
			t.Fatalf("status %d: want an error diagnostic", status)
		}
	}
}
