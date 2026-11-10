package getthing

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"example.com/iac-test-sdk/go/openapi/gen/folder_things_service"
)

type service = folder_things_service.FolderThingsServiceAPIService
type getRequest = folder_things_service.ApiFolderThingsServiceGetFolderThingRequest

// goneError is the 404 of a resource that does not exist. routeError is the 404 of a route that
// does not exist, which is no resource that is gone.
type goneError struct{}

func (goneError) Error() string   { return "not found" }
func (goneError) StatusCode() int { return http.StatusNotFound }

type routeError struct{}

func (routeError) Error() string   { return "Not Found: Not Found" }
func (routeError) StatusCode() int { return http.StatusNotFound }
func (routeError) Body() []byte    { return []byte("Not Found: Not Found") }

// reset sets the stub SDK: the stored thing, and the revision that Get returns for a request that
// asks for it.
func reset() {
	id, name := folder_things_service.CreatedID, "stored"
	folder_things_service.Stored = folder_things_service.FolderThing{Id: &id, Name: &name}
	folder_things_service.StoredRevision = "r1"
	folder_things_service.Created, folder_things_service.Replaced, folder_things_service.GetIDs, folder_things_service.GetIncludes = nil, nil, nil, nil
	folder_things_service.GetErr, folder_things_service.RejectInclude = nil, false
}

// retryingGet asks for the revision, and asks again without it when the server answers 403.
func retryingGet(calls *int) func(context.Context, func() getRequest) (*folder_things_service.GetFolderThingResponse, *http.Response, error) {
	return func(ctx context.Context, request func() getRequest) (*folder_things_service.GetFolderThingResponse, *http.Response, error) {
		*calls++
		out, httpResp, err := request().IncludeRevision(true).Execute()
		if err != nil && httpResp != nil && httpResp.StatusCode == http.StatusForbidden {
			*calls++
			out, httpResp, err = request().Execute()
		}
		return out, httpResp, err
	}
}

// setRevision sets the revision attribute from the revision next to the thing.
func setRevision(ctx context.Context, state *tfsdk.State, response *folder_things_service.GetFolderThingResponse) diag.Diagnostics {
	if response.Revision == nil {
		return nil
	}
	return state.SetAttribute(ctx, path.Root("revision"), types.StringValue(*response.Revision))
}

func thingSchema(t *testing.T, hooks Hooks) (*Resource, frameworkresource.SchemaResponse, tftypes.Type) {
	t.Helper()
	ctx := context.Background()
	r := &Resource{client: &service{}, hooks: hooks}
	var schemaResp frameworkresource.SchemaResponse
	r.Schema(ctx, frameworkresource.SchemaRequest{}, &schemaResp)
	return r, schemaResp, schemaResp.Schema.Type().TerraformType(ctx)
}

func thingValue(root tftypes.Type, id, name tftypes.Value) tftypes.Value {
	values := map[string]tftypes.Value{}
	for attr, attrType := range root.(tftypes.Object).AttributeTypes {
		values[attr] = tftypes.NewValue(attrType, nil)
	}
	values["id"], values["name"] = id, name
	return tftypes.NewValue(root, values)
}

func readThing(t *testing.T, hooks Hooks) (*frameworkresource.ReadResponse, frameworkresource.SchemaResponse) {
	t.Helper()
	r, schemaResp, root := thingSchema(t, hooks)
	state := thingValue(root, tftypes.NewValue(tftypes.String, folder_things_service.CreatedID), tftypes.NewValue(tftypes.String, "old"))
	resp := &frameworkresource.ReadResponse{State: tfsdk.State{Schema: schemaResp.Schema, Raw: state}}
	r.Read(context.Background(), frameworkresource.ReadRequest{State: tfsdk.State{Schema: schemaResp.Schema, Raw: state}}, resp)
	return resp, schemaResp
}

func revisionOf(t *testing.T, state tfsdk.State) types.String {
	t.Helper()
	var revision types.String
	if diags := state.GetAttribute(context.Background(), path.Root("revision"), &revision); diags.HasError() {
		t.Fatal(diags)
	}
	return revision
}

// Without hooks the generated Get call runs, and the revision stays null: the call asks for none.
func TestDefaultGetRunsWithoutHooks(t *testing.T) {
	reset()
	resp, _ := readThing(t, Hooks{})
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	if len(folder_things_service.GetIncludes) != 1 || folder_things_service.GetIncludes[0] {
		t.Fatalf("get requests that included the revision = %v, want one that did not", folder_things_service.GetIncludes)
	}
	if got := revisionOf(t, resp.State); !got.IsNull() {
		t.Fatalf("revision = %v, want null", got)
	}
}

// The Get hook adds a query parameter. A 403 is retried once without it, so a key without the
// permission keeps working, and AfterGet sees a response without the revision then.
func TestGetHookRetriesWithoutTheParameterOn403(t *testing.T) {
	reset()
	calls := 0
	after := 0
	resp, _ := readThing(t, Hooks{Get: retryingGet(&calls), AfterGet: func(ctx context.Context, state *tfsdk.State, response *folder_things_service.GetFolderThingResponse) diag.Diagnostics {
		after++
		return setRevision(ctx, state, response)
	}})
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	if calls != 1 || len(folder_things_service.GetIncludes) != 1 || !folder_things_service.GetIncludes[0] {
		t.Fatalf("hook calls = %d, requests = %v, want one request with the parameter", calls, folder_things_service.GetIncludes)
	}
	if got := revisionOf(t, resp.State); got.ValueString() != "r1" {
		t.Fatalf("revision = %v, want r1", got)
	}

	reset()
	calls, after = 0, 0
	folder_things_service.RejectInclude = true
	resp, _ = readThing(t, Hooks{Get: retryingGet(&calls), AfterGet: func(ctx context.Context, state *tfsdk.State, response *folder_things_service.GetFolderThingResponse) diag.Diagnostics {
		after++
		return setRevision(ctx, state, response)
	}})
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	if calls != 2 || len(folder_things_service.GetIncludes) != 2 || !folder_things_service.GetIncludes[0] || folder_things_service.GetIncludes[1] {
		t.Fatalf("hook calls = %d, requests = %v, want the parameter, then none", calls, folder_things_service.GetIncludes)
	}
	if after != 1 {
		t.Fatalf("AfterGet ran %d times, want once", after)
	}
	if got := revisionOf(t, resp.State); !got.IsNull() {
		t.Fatalf("revision = %v, want null: the retried response has none", got)
	}
	var name types.String
	resp.State.GetAttribute(context.Background(), path.Root("name"), &name)
	if name.ValueString() != "stored" {
		t.Fatalf("name = %v, want the stored name: the resource comes from the retried response", name)
	}
}

// A 404 of the resource from the hook removes the resource from state, as for the generated call.
// A 404 of a route that does not exist is an error, and Read keeps the state.
func TestGetHookErrorsAreClassifiedLikeTheGeneratedCall(t *testing.T) {
	reset()
	afterRan := false
	hooks := Hooks{
		Get: func(context.Context, func() getRequest) (*folder_things_service.GetFolderThingResponse, *http.Response, error) {
			return nil, &http.Response{StatusCode: http.StatusNotFound}, goneError{}
		},
		AfterGet: func(context.Context, *tfsdk.State, *folder_things_service.GetFolderThingResponse) diag.Diagnostics {
			afterRan = true
			return nil
		},
	}
	resp, _ := readThing(t, hooks)
	if resp.Diagnostics.HasError() || !resp.State.Raw.IsNull() || len(resp.Diagnostics.Warnings()) != 1 {
		t.Fatalf("read of a resource that is gone: state null = %v, diagnostics = %v, want a warning and no state", resp.State.Raw.IsNull(), resp.Diagnostics)
	}
	hooks.Get = func(context.Context, func() getRequest) (*folder_things_service.GetFolderThingResponse, *http.Response, error) {
		return nil, &http.Response{StatusCode: http.StatusNotFound}, routeError{}
	}
	resp, _ = readThing(t, hooks)
	if !resp.Diagnostics.HasError() || resp.State.Raw.IsNull() {
		t.Fatalf("read with a route that does not exist: state null = %v, diagnostics = %v, want an error and the state kept", resp.State.Raw.IsNull(), resp.Diagnostics)
	}
	hooks.Get = func(context.Context, func() getRequest) (*folder_things_service.GetFolderThingResponse, *http.Response, error) {
		return nil, nil, errors.New("read failed")
	}
	if resp, _ = readThing(t, hooks); !resp.Diagnostics.HasError() {
		t.Fatal("read with a failing hook: want an error")
	}
	if afterRan {
		t.Fatal("AfterGet ran although the Get call failed")
	}
}

// AfterGet runs after the state is written and before AfterRead, which sees what it set. An error
// of AfterGet is an error of the read.
func TestAfterGetRunsBeforeAfterRead(t *testing.T) {
	reset()
	var seen types.String
	hooks := Hooks{
		AfterGet: func(ctx context.Context, state *tfsdk.State, _ *folder_things_service.GetFolderThingResponse) diag.Diagnostics {
			return state.SetAttribute(ctx, path.Root("revision"), types.StringValue("from-after-get"))
		},
		AfterRead: func(ctx context.Context, state *tfsdk.State, _ any) diag.Diagnostics {
			return state.GetAttribute(ctx, path.Root("revision"), &seen)
		},
	}
	resp, _ := readThing(t, hooks)
	if resp.Diagnostics.HasError() || seen.ValueString() != "from-after-get" {
		t.Fatalf("AfterRead saw revision %v (%v), want the value of AfterGet", seen, resp.Diagnostics)
	}
	hooks.AfterGet = func(context.Context, *tfsdk.State, *folder_things_service.GetFolderThingResponse) diag.Diagnostics {
		var diags diag.Diagnostics
		diags.AddError("after get failed", "boom")
		return diags
	}
	if resp, _ = readThing(t, hooks); !resp.Diagnostics.HasError() {
		t.Fatal("read with a failing AfterGet: want an error")
	}
}

// Create reads the thing with Get, because its response holds only the id, so the hooks run: the
// Get hook sends the parameter and AfterGet sets the revision from the response.
func TestCreateRunsTheGetHooks(t *testing.T) {
	reset()
	calls := 0
	r, schemaResp, root := thingSchema(t, Hooks{Get: retryingGet(&calls), AfterGet: setRevision})
	plan := thingValue(root, tftypes.NewValue(tftypes.String, tftypes.UnknownValue), tftypes.NewValue(tftypes.String, "a"))
	plan = withUnknownRevision(root, plan)
	config := thingValue(root, tftypes.NewValue(tftypes.String, nil), tftypes.NewValue(tftypes.String, "a"))
	resp := &frameworkresource.CreateResponse{State: tfsdk.State{Schema: schemaResp.Schema, Raw: tftypes.NewValue(root, nil)}}
	r.Create(context.Background(), frameworkresource.CreateRequest{
		Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: config},
		Plan:   tfsdk.Plan{Schema: schemaResp.Schema, Raw: plan},
	}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	if calls != 1 || len(folder_things_service.GetIncludes) != 1 || !folder_things_service.GetIncludes[0] {
		t.Fatalf("hook calls = %d, requests = %v, want one request with the parameter", calls, folder_things_service.GetIncludes)
	}
	if got := revisionOf(t, resp.State); got.ValueString() != "r1" {
		t.Fatalf("revision after create = %v, want r1", got)
	}
}

// withUnknownRevision plans the computed revision as unknown, as Terraform does for a new resource.
func withUnknownRevision(root tftypes.Type, value tftypes.Value) tftypes.Value {
	var values map[string]tftypes.Value
	if err := value.As(&values); err != nil {
		panic(err)
	}
	values["revision"] = tftypes.NewValue(tftypes.String, tftypes.UnknownValue)
	return tftypes.NewValue(root, values)
}

// Replace returns no fields, so Update reads the thing with Get, and the hooks run. A 404 of the
// hook is the error of an update, because the framework cannot remove the resource there.
func TestUpdateRunsTheGetHooks(t *testing.T) {
	reset()
	calls := 0
	r, schemaResp, root := thingSchema(t, Hooks{Get: retryingGet(&calls), AfterGet: setRevision})
	id := tftypes.NewValue(tftypes.String, folder_things_service.CreatedID)
	state := thingValue(root, id, tftypes.NewValue(tftypes.String, "old"))
	plan := thingValue(root, id, tftypes.NewValue(tftypes.String, "new"))
	plan = withUnknownRevision(root, plan)
	update := func() *frameworkresource.UpdateResponse {
		resp := &frameworkresource.UpdateResponse{State: tfsdk.State{Schema: schemaResp.Schema, Raw: plan}}
		r.Update(context.Background(), frameworkresource.UpdateRequest{
			Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: thingValue(root, tftypes.NewValue(tftypes.String, nil), tftypes.NewValue(tftypes.String, "new"))},
			Plan:   tfsdk.Plan{Schema: schemaResp.Schema, Raw: plan},
			State:  tfsdk.State{Schema: schemaResp.Schema, Raw: state},
		}, resp)
		return resp
	}
	resp := update()
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	if calls != 1 || len(folder_things_service.Replaced) != 1 {
		t.Fatalf("hook calls = %d, replaces = %d, want one each", calls, len(folder_things_service.Replaced))
	}
	if got := revisionOf(t, resp.State); got.ValueString() != "r1" {
		t.Fatalf("revision after update = %v, want r1", got)
	}
	r.hooks.Get = func(context.Context, func() getRequest) (*folder_things_service.GetFolderThingResponse, *http.Response, error) {
		return nil, &http.Response{StatusCode: http.StatusNotFound}, goneError{}
	}
	if resp = update(); !resp.Diagnostics.HasError() {
		t.Fatal("update of a resource that is gone: want an error")
	}
}

func readDataSource(t *testing.T, hooks Hooks) *datasource.ReadResponse {
	t.Helper()
	ctx := context.Background()
	d := &DataSource{resource: Resource{client: &service{}, hooks: hooks}}
	var schemaResp datasource.SchemaResponse
	d.Schema(ctx, datasource.SchemaRequest{}, &schemaResp)
	root := schemaResp.Schema.Type().TerraformType(ctx).(tftypes.Object)
	values := map[string]tftypes.Value{}
	for attr, attrType := range root.AttributeTypes {
		values[attr] = tftypes.NewValue(attrType, nil)
	}
	values["id"] = tftypes.NewValue(tftypes.String, folder_things_service.CreatedID)
	config := tftypes.NewValue(root, values)
	resp := &datasource.ReadResponse{State: tfsdk.State{Schema: schemaResp.Schema, Raw: config}}
	d.Read(ctx, datasource.ReadRequest{Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: config}}, resp)
	return resp
}

// The data source runs the same hooks: the Get hook replaces its Get call, AfterGet sets the
// revision on the state that the data source wrote, and a 404 of the hook is an error.
func TestDataSourceRunsTheGetHooks(t *testing.T) {
	reset()
	calls := 0
	resp := readDataSource(t, Hooks{Get: retryingGet(&calls), AfterGet: setRevision})
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	if calls != 1 || len(folder_things_service.GetIncludes) != 1 || !folder_things_service.GetIncludes[0] {
		t.Fatalf("hook calls = %d, requests = %v, want one request with the parameter", calls, folder_things_service.GetIncludes)
	}
	if got := revisionOf(t, resp.State); got.ValueString() != "r1" {
		t.Fatalf("data source revision = %v, want r1", got)
	}
	resp = readDataSource(t, Hooks{Get: func(context.Context, func() getRequest) (*folder_things_service.GetFolderThingResponse, *http.Response, error) {
		return nil, &http.Response{StatusCode: http.StatusNotFound}, goneError{}
	}, AfterGet: setRevision})
	if !resp.Diagnostics.HasError() {
		t.Fatal("data source with a resource that is gone: want an error")
	}
	reset()
	if resp = readDataSource(t, Hooks{}); resp.Diagnostics.HasError() || !revisionOf(t, resp.State).IsNull() {
		t.Fatalf("data source without hooks: %v, revision %v, want no error and a null revision", resp.Diagnostics, revisionOf(t, resp.State))
	}
}
