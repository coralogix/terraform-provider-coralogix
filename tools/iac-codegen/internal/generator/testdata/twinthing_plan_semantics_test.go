package parsething

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

type planProvider struct{}

func (planProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "test"
}
func (planProvider) Schema(context.Context, provider.SchemaRequest, *provider.SchemaResponse) {}
func (planProvider) Configure(context.Context, provider.ConfigureRequest, *provider.ConfigureResponse) {
}
func (planProvider) Resources(context.Context) []func() frameworkresource.Resource {
	return []func() frameworkresource.Resource{func() frameworkresource.Resource { return NewResource(Hooks{}) }}
}
func (planProvider) DataSources(context.Context) []func() datasource.DataSource { return nil }

func nullOf(t tftypes.Type) tftypes.Value { return tftypes.NewValue(t, nil) }

// object makes a value of the object type t with the attributes of set, and null for the others.
func object(t tftypes.Type, set map[string]tftypes.Value) tftypes.Value {
	attrs := map[string]tftypes.Value{}
	for name, at := range t.(tftypes.Object).AttributeTypes {
		attrs[name] = nullOf(at)
	}
	for name, v := range set {
		attrs[name] = v
	}
	return tftypes.NewValue(t, attrs)
}

func text(v string) tftypes.Value  { return tftypes.NewValue(tftypes.String, v) }
func flagOf(v bool) tftypes.Value  { return tftypes.NewValue(tftypes.Bool, v) }
func number(v int64) tftypes.Value { return tftypes.NewValue(tftypes.Number, v) }

// keepStep is one step of the thing: the fields of its keep object, which holds both twins.
type keepStep map[string]tftypes.Value

// fieldsComputed are the attributes of the keep object that Terraform proposes from the prior state
// when the configuration leaves them out. The others are null.
var fieldsComputed = []string{"label", "threshold", "keep_matching"}

type planner struct {
	t          *testing.T
	root, step tftypes.Type
	list       bool // the steps are a list, else a set
	server     tfprotov6.ProviderServer
}

func newPlanner(t *testing.T) *planner {
	t.Helper()
	server, err := providerserver.NewProtocol6WithError(planProvider{})()
	if err != nil {
		t.Fatal(err)
	}
	resp, err := server.GetProviderSchema(context.Background(), &tfprotov6.GetProviderSchemaRequest{})
	if err != nil {
		t.Fatal(err)
	}
	root := resp.ResourceSchemas["test_parse_thing"].ValueType()
	p := &planner{t: t, root: root, server: server}
	switch steps := root.(tftypes.Object).AttributeTypes["steps"].(type) {
	case tftypes.List:
		p.step, p.list = steps.ElementType, true
	case tftypes.Set:
		p.step = steps.ElementType
	}
	return p
}

// thing makes the resource. A thing that does not exist yet has no id.
func (p *planner) thing(exists bool, steps ...tftypes.Value) tftypes.Value {
	fields := map[string]tftypes.Value{"name": text("n"), "steps": p.steps(steps)}
	if exists {
		fields["id"] = text("thing-1")
	}
	return object(p.root, fields)
}

// needsList skips a test whose prior state holds computed values in the items. Terraform proposes
// them in the item of a set, and the framework then finds no configured item that matches the
// planned one, so it plans the default of the flag in place of the configured value.
func (p *planner) needsList() {
	p.t.Helper()
	if !p.list {
		p.t.Skip("the framework cannot match a planned set item with computed values to its configuration")
	}
}

func (p *planner) prior(prior []keepStep, steps []tftypes.Value) tftypes.Value {
	if prior == nil {
		return nullOf(p.root) // a create has no prior state
	}
	return p.thing(true, steps...)
}

func (p *planner) steps(items []tftypes.Value) tftypes.Value {
	if p.list {
		return tftypes.NewValue(tftypes.List{ElementType: p.step}, items)
	}
	return tftypes.NewValue(tftypes.Set{ElementType: p.step}, items)
}

// keepStep makes a step. The configuration has no id, which the server sets.
func (p *planner) keepStep(keep keepStep, withID bool) tftypes.Value {
	keepType := p.step.(tftypes.Object).AttributeTypes["keep"]
	fields := keepStep{"name": text("s")}
	if withID {
		fields["id"] = text("step-1")
	}
	for name, v := range keep {
		fields[name] = v
	}
	return object(p.step, map[string]tftypes.Value{"keep": object(keepType, fields)})
}

// plan plans the config steps against the prior steps, as Terraform does: the proposed state has the
// configuration, and the prior value of each computed attribute that the configuration leaves out.
func (p *planner) plan(prior, config []keepStep) []ParseStepModel {
	p.t.Helper()
	var priorSteps, configSteps, proposedSteps []tftypes.Value
	for _, k := range prior {
		priorSteps = append(priorSteps, p.keepStep(k, true))
	}
	for i, c := range config {
		configSteps = append(configSteps, p.keepStep(c, false))
		proposed := keepStep{}
		for name, v := range c {
			proposed[name] = v
		}
		// Terraform pairs a configured item with the prior item that has the same rule.
		j := i
		for k, before := range prior {
			if before["rule"].Equal(c["rule"]) {
				j = k
			}
		}
		for _, name := range fieldsComputed {
			if _, set := c[name]; set || j >= len(prior) {
				continue
			}
			if v, kept := prior[j][name]; kept {
				proposed[name] = v
			}
		}
		proposedSteps = append(proposedSteps, p.keepStep(proposed, i < len(prior)))
	}
	encode := func(v tftypes.Value) *tfprotov6.DynamicValue {
		dv, err := tfprotov6.NewDynamicValue(p.root, v)
		if err != nil {
			p.t.Fatal(err)
		}
		return &dv
	}
	configValue := object(p.root, map[string]tftypes.Value{"name": text("n"), "steps": p.steps(configSteps)})
	resp, err := p.server.PlanResourceChange(context.Background(), &tfprotov6.PlanResourceChangeRequest{
		TypeName:         "test_parse_thing",
		PriorState:       encode(p.prior(prior, priorSteps)),
		Config:           encode(configValue),
		ProposedNewState: encode(p.thing(prior != nil, proposedSteps...)),
	})
	if err != nil {
		p.t.Fatal(err)
	}
	for _, d := range resp.Diagnostics {
		if d.Severity == tfprotov6.DiagnosticSeverityError {
			p.t.Fatalf("plan: %s: %s", d.Summary, d.Detail)
		}
	}
	planned, err := resp.PlannedState.Unmarshal(p.root)
	if err != nil {
		p.t.Fatal(err)
	}
	var steps []ParseStepModel
	state := tfsdk.State{Schema: Schema(), Raw: planned}
	if diags := state.GetAttribute(context.Background(), path.Root("steps"), &steps); diags.HasError() {
		p.t.Fatal(diags)
	}
	return steps
}

func expandPlanned(m *ParseStepModel) diag.Diagnostics {
	var diags diag.Diagnostics
	expandCreateParseStep(context.Background(), path.Empty(), m, &diags)
	return diags
}

// The flag turns from true to false. The shown twin's field label kept its server default in state,
// and the plan would keep it: the expand then refused a field that the user never wrote. The plan is
// null for label now, and the expand accepts the step. The field of the folded twin is not in state
// and plans as before.
func TestFlagTurnedToFalsePlansTheShownTwinsFieldNull(t *testing.T) {
	p := newPlanner(t)
	p.needsList()
	steps := p.plan(
		[]keepStep{{"rule": text("r"), "label": text("standard"), "strict": text("s"), "keep_matching": flagOf(true)}},
		[]keepStep{{"rule": text("r"), "hard": text("h"), "keep_matching": flagOf(false)}},
	)
	keep := steps[0].KeepParams
	if !keep.Label.IsNull() {
		t.Fatalf("label = %v, want null while keep_matching is false (%+v)", keep.Label, keep)
	}
	if diags := expandPlanned(&steps[0]); diags.HasError() {
		t.Fatalf("the expand refuses the planned step: %v", diags)
	}
	if keep.KeepMatching.ValueBool() || keep.Hard.ValueString() != "h" {
		t.Fatalf("planned %+v", keep)
	}
}

// The flag turns from false to true. The folded twin's field threshold kept its server default in
// state, and plans null now.
func TestFlagTurnedToTruePlansTheFoldedTwinsFieldsNull(t *testing.T) {
	p := newPlanner(t)
	p.needsList()
	steps := p.plan(
		[]keepStep{{"rule": text("r"), "threshold": number(5), "hard": text("h"), "keep_matching": flagOf(false)}},
		[]keepStep{{"rule": text("r"), "strict": text("s"), "keep_matching": flagOf(true)}},
	)
	keep := steps[0].KeepParams
	if !keep.Threshold.IsNull() {
		t.Fatalf("threshold = %v, want null while keep_matching is true", keep.Threshold)
	}
	if diags := expandPlanned(&steps[0]); diags.HasError() {
		t.Fatalf("the expand refuses the planned step: %v", diags)
	}
}

// A field of the twin that the flag selects keeps its server default in the plan.
func TestFieldOfTheSelectedTwinKeepsItsDefault(t *testing.T) {
	p := newPlanner(t)
	p.needsList()
	shown := p.plan(
		[]keepStep{{"rule": text("r"), "label": text("standard"), "strict": text("s"), "keep_matching": flagOf(true)}},
		[]keepStep{{"rule": text("r"), "strict": text("s"), "keep_matching": flagOf(true)}},
	)
	if got := shown[0].KeepParams.Label; got.ValueString() != "standard" {
		t.Fatalf("label = %v, want the default", got)
	}
	folded := p.plan(
		[]keepStep{{"rule": text("r"), "threshold": number(5), "hard": text("h"), "keep_matching": flagOf(false)}},
		[]keepStep{{"rule": text("r"), "hard": text("h"), "keep_matching": flagOf(false)}},
	)
	if got := folded[0].KeepParams.Threshold; got.ValueInt64() != 5 {
		t.Fatalf("threshold = %v, want the default", got)
	}
}

// A field that the user configures for the other twin stays in the plan, so that the expand still
// reports it.
func TestConfiguredFieldOfTheOtherTwinStaysInThePlan(t *testing.T) {
	p := newPlanner(t)
	p.needsList()
	steps := p.plan(
		[]keepStep{{"rule": text("r"), "label": text("standard"), "strict": text("s"), "keep_matching": flagOf(true)}},
		[]keepStep{{"rule": text("r"), "label": text("mine"), "hard": text("h"), "keep_matching": flagOf(false)}},
	)
	if got := steps[0].KeepParams.Label; got.ValueString() != "mine" {
		t.Fatalf("label = %v, want the configured value", got)
	}
	if diags := expandPlanned(&steps[0]); !diags.HasError() {
		t.Fatal("the expand accepts a configured field of the other twin")
	}
}

// The plan reads the flag of the step that holds the field: one step turns to false, the next stays
// true, and each plans its own label.
func TestEachStepPlansByItsOwnFlag(t *testing.T) {
	p := newPlanner(t)
	p.needsList()
	prior := func(rule string) keepStep {
		return keepStep{"rule": text(rule), "label": text("standard"), "strict": text("s"), "keep_matching": flagOf(true)}
	}
	steps := p.plan(
		[]keepStep{prior("r1"), prior("r2")},
		[]keepStep{
			{"rule": text("r1"), "hard": text("h"), "keep_matching": flagOf(false)},
			{"rule": text("r2"), "strict": text("s"), "keep_matching": flagOf(true)},
		},
	)
	labels := map[string]string{} // the label of each step by its rule, "null" for none
	for _, step := range steps {
		keep := step.KeepParams
		labels[keep.Rule.ValueString()] = keep.Label.String()
		if diags := expandPlanned(&step); diags.HasError() {
			t.Fatalf("the expand refuses the planned step %s: %v", keep.Rule, diags)
		}
	}
	if labels["r1"] != "<null>" || labels["r2"] != `"standard"` {
		t.Fatalf("labels = %v, want null for r1 and the default for r2", labels)
	}
}

// A flag that is unknown now selects no twin yet: the plan keeps the value of the state.
func TestUnknownFlagLeavesTheFieldAsItIs(t *testing.T) {
	p := newPlanner(t)
	p.needsList()
	steps := p.plan(
		[]keepStep{{"rule": text("r"), "label": text("standard"), "strict": text("s"), "keep_matching": flagOf(true)}},
		[]keepStep{{"rule": text("r"), "strict": text("s"), "keep_matching": tftypes.NewValue(tftypes.Bool, tftypes.UnknownValue)}},
	)
	if got := steps[0].KeepParams.Label; got.IsNull() {
		t.Fatalf("label = %v, want it kept while the flag is unknown", got)
	}
}

// A create has no state to keep, and the plan of a computed field is unknown. A field of the twin that
// the flag does not select is null, and a field of the selected twin stays unknown: the server fills
// it in. The flag and the fields are those of the step of the set or the list.
func TestCreatePlansTheFieldsOfTheOtherTwinNull(t *testing.T) {
	p := newPlanner(t)
	folded := p.plan(nil, []keepStep{{"rule": text("r"), "hard": text("h"), "keep_matching": flagOf(false)}})[0].KeepParams
	if !folded.Label.IsNull() || !folded.Threshold.IsUnknown() {
		t.Fatalf("flag false: label = %v, threshold = %v, want null and unknown", folded.Label, folded.Threshold)
	}
	shown := p.plan(nil, []keepStep{{"rule": text("r"), "strict": text("s"), "keep_matching": flagOf(true)}})[0].KeepParams
	if !shown.Threshold.IsNull() || !shown.Label.IsUnknown() {
		t.Fatalf("flag true: label = %v, threshold = %v, want unknown and null", shown.Label, shown.Threshold)
	}
}
