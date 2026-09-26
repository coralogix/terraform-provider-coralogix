package fakeboard

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"

	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen-poc/fakesdk/go/openapi/gen/fake_boards_service"

	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen-poc/schemadump"
)

// TestPlanWithUnknowns checks that an update plan with every computed
// attribute unknown reads into the model and expands. routing is a computed
// object, so its model field must be a types.Object.
func TestPlanWithUnknowns(t *testing.T) {
	ctx := context.Background()
	m := full(t)
	var diags diag.Diagnostics
	name := "on-call"
	m.Routing = objectValue(ctx, routingAttrTypes(), flattenRouting(ctx, path.Root("routing"),
		&fake_boards_service.Routing{RoutingName: &name}, &diags), &diags)
	if diags.HasError() {
		t.Fatal(diags)
	}
	raw, err := schemadump.PlanWithUnknowns(ctx, Schema(), toState(t, m).Raw)
	if err != nil {
		t.Fatal(err)
	}
	var plan FakeBoardModel
	if diags := (tfsdk.Plan{Schema: Schema(), Raw: raw}).Get(ctx, &plan); diags.HasError() {
		t.Fatalf("get plan: %v", diags)
	}
	if !plan.Routing.IsUnknown() || !plan.Id.IsUnknown() {
		t.Errorf("routing %v, id %v: want unknown", plan.Routing, plan.Id)
	}
	if _, diags := expandUpdate(ctx, &plan); diags.HasError() {
		t.Fatalf("expand: %v", diags)
	}
}
