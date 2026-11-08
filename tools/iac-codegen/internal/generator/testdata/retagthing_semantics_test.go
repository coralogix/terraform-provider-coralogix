package retagthing

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"example.com/iac-test-sdk/go/openapi/gen/retag_things_service"
)

var unknown = struct{}{}

// fillIDs is the BeforeWrite hook of a resource like the archive retention tags: a write without ids
// (Create) reads the tags and sends their ids by position. A failed read stops the write.
func fillIDs(ctx context.Context, _ tfsdk.Config, body any) diag.Diagnostics {
	var diags diag.Diagnostics
	req := body.(*retag_things_service.RenameTagsRequest)
	if len(req.TagUpdates) == 0 || req.TagUpdates[0].Id != "" {
		return diags
	}
	list, _, err := (&retag_things_service.RetagThingsServiceAPIService{}).RetagThingsServiceListTags(ctx).Execute()
	if err != nil {
		diags.AddError("Unable to read the tags", err.Error())
		return diags
	}
	if len(list.Tags) < len(req.TagUpdates) {
		diags.AddError("Unable to read the tags", fmt.Sprintf("the API has %d tags, the configuration %d", len(list.Tags), len(req.TagUpdates)))
		return diags
	}
	for i := range req.TagUpdates {
		req.TagUpdates[i].Id = list.Tags[i].Id
	}
	return diags
}

func setup(t *testing.T) (*Resource, schema.Schema, tftypes.Type) {
	t.Helper()
	retag_things_service.Stored = []retag_things_service.Tag{{Id: "id-1", Order: 1, Name: "one"}, {Id: "id-2", Order: 2, Name: "two"}}
	retag_things_service.Writes, retag_things_service.Gets = nil, 0
	retag_things_service.WriteErr, retag_things_service.ReadErr = nil, nil
	r := &Resource{client: &retag_things_service.RetagThingsServiceAPIService{}, hooks: Hooks{BeforeWrite: fillIDs}}
	var s frameworkresource.SchemaResponse
	r.Schema(context.Background(), frameworkresource.SchemaRequest{}, &s)
	return r, s.Schema, s.Schema.Type().TerraformType(context.Background())
}

// value builds the resource value: tags of {id, order, name}, where unknown is a planned value.
func value(typ tftypes.Type, id any, tags ...[3]any) tftypes.Value {
	obj := typ.(tftypes.Object)
	tagType := obj.AttributeTypes["tags"].(tftypes.List).ElementType.(tftypes.Object)
	scalar := func(t tftypes.Type, v any) tftypes.Value {
		switch v {
		case nil:
			return tftypes.NewValue(t, nil)
		case unknown:
			return tftypes.NewValue(t, tftypes.UnknownValue)
		}
		if n, ok := v.(int); ok {
			return tftypes.NewValue(t, int64(n))
		}
		return tftypes.NewValue(t, v)
	}
	var items []tftypes.Value
	for _, tag := range tags {
		items = append(items, tftypes.NewValue(tagType, map[string]tftypes.Value{
			"id": scalar(tftypes.String, tag[0]), "order": scalar(tftypes.Number, tag[1]), "name": scalar(tftypes.String, tag[2]),
		}))
	}
	return tftypes.NewValue(obj, map[string]tftypes.Value{"id": scalar(tftypes.String, id), "tags": tftypes.NewValue(obj.AttributeTypes["tags"], items)})
}

func noErrors(t *testing.T, diags diag.Diagnostics) {
	t.Helper()
	if diags.HasError() {
		t.Fatal(diags)
	}
}

// The user cannot set the id of a tag: only the server sets it. The name is optional and computed.
func TestTagIDIsComputedOnly(t *testing.T) {
	_, s, _ := setup(t)
	attrs := s.Attributes["tags"].(schema.ListNestedAttribute).NestedObject.Attributes
	if id := attrs["id"]; !id.IsComputed() || id.IsOptional() || id.IsRequired() {
		t.Fatalf("tags.id: computed %t, optional %t, required %t; want computed only", id.IsComputed(), id.IsOptional(), id.IsRequired())
	}
	if name := attrs["name"]; !name.IsComputed() || !name.IsOptional() || name.IsRequired() {
		t.Fatalf("tags.name: computed %t, optional %t, required %t; want optional and computed", name.IsComputed(), name.IsOptional(), name.IsRequired())
	}
}

// Create sends the ids that the hook read, and takes the state from the write: no Get of its own.
func TestCreateSendsHookIDsAndReadsTheWrite(t *testing.T) {
	r, s, typ := setup(t)
	config := value(typ, nil, [3]any{nil, nil, "a"}, [3]any{nil, nil, "b"})
	plan := value(typ, "retag-things", [3]any{unknown, unknown, "a"}, [3]any{unknown, unknown, "b"})
	resp := &frameworkresource.CreateResponse{State: tfsdk.State{Schema: s}}
	r.Create(context.Background(), frameworkresource.CreateRequest{Config: tfsdk.Config{Schema: s, Raw: config}, Plan: tfsdk.Plan{Schema: s, Raw: plan}}, resp)
	noErrors(t, resp.Diagnostics)
	if len(retag_things_service.Writes) != 1 {
		t.Fatalf("writes = %d, want 1", len(retag_things_service.Writes))
	}
	sent := retag_things_service.Writes[0].TagUpdates
	if len(sent) != 2 || sent[0].Id != "id-1" || sent[1].Id != "id-2" || sent[1].Name != "b" {
		t.Fatalf("sent = %+v, want the ids of the API by position", sent)
	}
	if retag_things_service.Gets != 1 {
		t.Fatalf("Gets = %d, want only the read of the hook", retag_things_service.Gets)
	}
	var m TagListModel
	noErrors(t, resp.State.Get(context.Background(), &m))
	var tags []TagModel
	noErrors(t, m.Tags.ElementsAs(context.Background(), &tags, false))
	if len(tags) != 2 || tags[1].Id.ValueString() != "id-2" || tags[1].Order.ValueInt32() != 2 || tags[1].Name.ValueString() != "b" {
		t.Fatalf("state = %+v, want the tags that the write returned", tags)
	}
}

// A failed read of the hook stops Create before the write.
func TestCreateStopsWhenTheHookReadFails(t *testing.T) {
	r, s, typ := setup(t)
	retag_things_service.ReadErr = errors.New("500 Internal")
	config := value(typ, nil, [3]any{nil, nil, "a"})
	plan := value(typ, "retag-things", [3]any{unknown, unknown, "a"})
	resp := &frameworkresource.CreateResponse{State: tfsdk.State{Schema: s}}
	r.Create(context.Background(), frameworkresource.CreateRequest{Config: tfsdk.Config{Schema: s, Raw: config}, Plan: tfsdk.Plan{Schema: s, Raw: plan}}, resp)
	if !resp.Diagnostics.HasError() {
		t.Fatal("Create succeeded, want the error of the read")
	}
	if len(retag_things_service.Writes) != 0 {
		t.Fatalf("writes = %d, want none after a failed read", len(retag_things_service.Writes))
	}
}

// Update sends the ids that the state keeps, with no Get before or after the write.
func TestUpdateSendsStateIDsWithoutAGet(t *testing.T) {
	r, s, typ := setup(t)
	config := value(typ, nil, [3]any{nil, nil, "a"}, [3]any{nil, nil, "c"})
	state := value(typ, "retag-things", [3]any{"id-1", 1, "a"}, [3]any{"id-2", 2, "b"})
	// useStateForUnknown keeps the ids in the plan.
	plan := value(typ, "retag-things", [3]any{"id-1", 1, "a"}, [3]any{"id-2", 2, "c"})
	resp := &frameworkresource.UpdateResponse{State: tfsdk.State{Schema: s, Raw: state}}
	r.Update(context.Background(), frameworkresource.UpdateRequest{Config: tfsdk.Config{Schema: s, Raw: config}, Plan: tfsdk.Plan{Schema: s, Raw: plan}, State: tfsdk.State{Schema: s, Raw: state}}, resp)
	noErrors(t, resp.Diagnostics)
	if len(retag_things_service.Writes) != 1 {
		t.Fatalf("writes = %d, want 1", len(retag_things_service.Writes))
	}
	sent := retag_things_service.Writes[0].TagUpdates
	if len(sent) != 2 || sent[0].Id != "id-1" || sent[1].Id != "id-2" || sent[1].Name != "c" {
		t.Fatalf("sent = %+v, want the ids of the state", sent)
	}
	if retag_things_service.Gets != 0 {
		t.Fatalf("Gets = %d, want none", retag_things_service.Gets)
	}
	if retag_things_service.Stored[1].Name != "c" {
		t.Fatalf("stored = %+v, want the second tag renamed", retag_things_service.Stored)
	}
}

// testProvider serves the generated resource with the hook, so that a test can plan and apply it
// through the protocol as Terraform does.
type testProvider struct{}

func (testProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "test"
}
func (testProvider) Schema(context.Context, provider.SchemaRequest, *provider.SchemaResponse) {}
func (testProvider) Configure(context.Context, provider.ConfigureRequest, *provider.ConfigureResponse) {
}
func (testProvider) Resources(context.Context) []func() frameworkresource.Resource {
	return []func() frameworkresource.Resource{func() frameworkresource.Resource {
		return NewResourceWithClient(&retag_things_service.RetagThingsServiceAPIService{}, Hooks{BeforeWrite: fillIDs})
	}}
}
func (testProvider) DataSources(context.Context) []func() datasource.DataSource { return nil }

// proposed is the proposed new state of Terraform core: the config, with the prior value of each
// computed attribute that the config leaves null, item by item.
func proposed(t *testing.T, typ tftypes.Type, config, prior tftypes.Value) tftypes.Value {
	t.Helper()
	obj := typ.(tftypes.Object)
	listType := obj.AttributeTypes["tags"].(tftypes.List)
	tagType := listType.ElementType.(tftypes.Object)
	var c, p map[string]tftypes.Value
	if err := config.As(&c); err != nil {
		t.Fatal(err)
	}
	if err := prior.As(&p); err != nil {
		t.Fatal(err)
	}
	var cTags, pTags []tftypes.Value
	if err := c["tags"].As(&cTags); err != nil {
		t.Fatal(err)
	}
	if err := p["tags"].As(&pTags); err != nil {
		t.Fatal(err)
	}
	var items []tftypes.Value
	for i, ct := range cTags {
		var ci map[string]tftypes.Value
		if err := ct.As(&ci); err != nil {
			t.Fatal(err)
		}
		out := map[string]tftypes.Value{"id": ci["id"], "order": ci["order"], "name": ci["name"]}
		if i < len(pTags) {
			var pi map[string]tftypes.Value
			if err := pTags[i].As(&pi); err != nil {
				t.Fatal(err)
			}
			out["id"], out["order"] = pi["id"], pi["order"]
		}
		items = append(items, tftypes.NewValue(tagType, out))
	}
	return tftypes.NewValue(obj, map[string]tftypes.Value{"id": p["id"], "tags": tftypes.NewValue(listType, items)})
}

// A name change plans the ids of the state, which the Framework would otherwise plan as unknown.
// The apply sends those ids, the state takes the write, and the next plan is empty.
func TestProtocolUpdateKeepsTheIDsOfTheState(t *testing.T) {
	_, _, typ := setup(t)
	ctx := context.Background()
	server, err := providerserver.NewProtocol6WithError(testProvider{})()
	if err != nil {
		t.Fatal(err)
	}
	encode := func(v tftypes.Value) *tfprotov6.DynamicValue {
		dv, err := tfprotov6.NewDynamicValue(typ, v)
		if err != nil {
			t.Fatal(err)
		}
		return &dv
	}
	noProtocolErrors := func(step string, diags []*tfprotov6.Diagnostic) {
		t.Helper()
		for _, d := range diags {
			if d.Severity == tfprotov6.DiagnosticSeverityError {
				t.Fatalf("%s: %s: %s", step, d.Summary, d.Detail)
			}
		}
	}
	plan := func(prior, config tftypes.Value) tftypes.Value {
		t.Helper()
		resp, err := server.PlanResourceChange(ctx, &tfprotov6.PlanResourceChangeRequest{
			TypeName: "test_retag_thing", PriorState: encode(prior), Config: encode(config), ProposedNewState: encode(proposed(t, typ, config, prior)),
		})
		if err != nil {
			t.Fatal(err)
		}
		noProtocolErrors("plan", resp.Diagnostics)
		planned, err := resp.PlannedState.Unmarshal(typ)
		if err != nil {
			t.Fatal(err)
		}
		return planned
	}
	at := func(v tftypes.Value, i int, attr string) tftypes.Value {
		t.Helper()
		got, _, err := tftypes.WalkAttributePath(v, tftypes.NewAttributePath().WithAttributeName("tags").WithElementKeyInt(i).WithAttributeName(attr))
		if err != nil {
			t.Fatal(err)
		}
		return got.(tftypes.Value)
	}

	state := value(typ, "retag-things", [3]any{"id-1", 1, "one"}, [3]any{"id-2", 2, "two"})
	config := value(typ, nil, [3]any{nil, nil, "one"}, [3]any{nil, nil, "changed"})
	planned := plan(state, config)
	for i, want := range []string{"id-1", "id-2"} {
		if got := at(planned, i, "id"); !got.Equal(tftypes.NewValue(tftypes.String, want)) {
			t.Fatalf("planned tags[%d].id = %s, want %s from the state", i, got, want)
		}
	}

	resp, err := server.ApplyResourceChange(ctx, &tfprotov6.ApplyResourceChangeRequest{
		TypeName: "test_retag_thing", PriorState: encode(state), PlannedState: encode(planned), Config: encode(config),
	})
	if err != nil {
		t.Fatal(err)
	}
	noProtocolErrors("apply", resp.Diagnostics)
	sent := retag_things_service.Writes
	if len(sent) != 1 || sent[0].TagUpdates[0].Id != "id-1" || sent[0].TagUpdates[1].Id != "id-2" || sent[0].TagUpdates[1].Name != "changed" {
		t.Fatalf("writes = %+v, want one write with the ids of the state", sent)
	}
	if retag_things_service.Gets != 0 {
		t.Fatalf("Gets = %d, want none", retag_things_service.Gets)
	}
	next, err := resp.NewState.Unmarshal(typ)
	if err != nil {
		t.Fatal(err)
	}
	if got := at(next, 1, "name"); !got.Equal(tftypes.NewValue(tftypes.String, "changed")) {
		t.Fatalf("state tags[1].name = %s, want changed", got)
	}
	if got := at(next, 1, "id"); !got.Equal(tftypes.NewValue(tftypes.String, "id-2")) {
		t.Fatalf("state tags[1].id = %s, want id-2", got)
	}

	if again := plan(next, config); !again.Equal(next) {
		t.Fatalf("second plan = %s, want the state %s", again, next)
	}
}
