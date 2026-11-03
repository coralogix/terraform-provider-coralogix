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

package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

const grTypeName = "coralogix_global_router"
const cxTypeName = "coralogix_connector"
const presetTypeName = "coralogix_preset"

type requestLog interface {
	takeRequests() []grRequest
}

// grStep is the record of one operation. The golden files hold these records.
type grStep struct {
	Step          string   `json:"step"`
	Config        any      `json:"config,omitempty"`
	ValidateDiags []string `json:"validateDiagnostics,omitempty"`
	Plan          any      `json:"plan,omitempty"`
	// NoChange is true when the plan equals the state. Terraform core then calls no Update.
	NoChange        bool        `json:"noChange,omitempty"`
	PlanDiags       []string    `json:"planDiagnostics,omitempty"`
	RequiresReplace []string    `json:"requiresReplace,omitempty"`
	Requests        []grRequest `json:"requests,omitempty"`
	ApplyDiags      []string    `json:"applyDiagnostics,omitempty"`
	// Inconsistent lists the paths where Terraform core would fail the apply
	// with "Provider produced inconsistent result after apply".
	Inconsistent []string `json:"inconsistentResult,omitempty"`
	// State is the state after the step. It is null when the resource is gone.
	State any `json:"state"`
	// Drift lists the paths that changed between the state before and after a refresh.
	Drift []string `json:"drift,omitempty"`
	// ReplanEmpty is true when planning the same config on the new state gives no change.
	ReplanEmpty *bool    `json:"replanEmpty,omitempty"`
	ReplanDiff  []string `json:"replanDiff,omitempty"`
}

// grHarness drives the provider through the Terraform plugin protocol, the
// way Terraform core does, against the fake backend.
type grHarness struct {
	t        *testing.T
	ctx      context.Context
	server   tfprotov6.ProviderServer
	schema   *tfprotov6.Schema
	typ      tftypes.Object
	api      requestLog
	typeName string
	// dsSchema is the schema of the data source with the same type name, or nil.
	dsSchema *tfprotov6.Schema
	steps    []grStep
	// applied is the config of the last apply that succeeded. RefreshPlan plans it again.
	applied map[string]any
	// failedCreate is the state that the last Create that failed returned, or null. Terraform core
	// saves it tainted, so the next apply replaces it.
	failedCreate tftypes.Value
}

func newGRHarness(t *testing.T) *grHarness {
	return newProtocolHarness(t, grTypeName, newGRFake(t))
}

func newCXHarness(t *testing.T) *grHarness {
	return newProtocolHarness(t, cxTypeName, newCXFake(t))
}

func newPresetHarness(t *testing.T) *grHarness {
	return newProtocolHarness(t, presetTypeName, newPresetFake(t))
}

func newAIEvalHarness(t *testing.T) *grHarness {
	return newProtocolHarness(t, aiEvalTypeName, newAIEvalFake(t))
}

func (h *grHarness) aiEvals() *aiEvalFake {
	h.t.Helper()
	f, ok := h.api.(*aiEvalFake)
	if !ok {
		h.t.Fatal("expected *aiEvalFake")
	}
	return f
}

func (h *grHarness) routers() *grFake {
	h.t.Helper()
	f, ok := h.api.(*grFake)
	if !ok {
		h.t.Fatal("expected *grFake")
	}
	return f
}

func (h *grHarness) connectors() *cxFake {
	h.t.Helper()
	f, ok := h.api.(*cxFake)
	if !ok {
		h.t.Fatal("expected *cxFake")
	}
	return f
}

func (h *grHarness) presets() *presetFake {
	h.t.Helper()
	f, ok := h.api.(*presetFake)
	if !ok {
		h.t.Fatal("expected *presetFake")
	}
	return f
}

func newProtocolHarness(t *testing.T, typeName string, api requestLog) *grHarness {
	t.Helper()
	// Placeholder credentials. The fake backend accepts any key.
	t.Setenv("CORALOGIX_API_KEY", "baseline-test-key")
	t.Setenv("CORALOGIX_DOMAIN", "eu2.coralogix.com")
	t.Setenv("CORALOGIX_ENV", "")

	h := &grHarness{t: t, ctx: context.Background(), api: api, typeName: typeName}
	var err error
	if h.server, err = testAccProtoV6ProviderFactories["coralogix"](); err != nil {
		t.Fatal(err)
	}
	schemas, err := h.server.GetProviderSchema(h.ctx, &tfprotov6.GetProviderSchemaRequest{})
	h.must(err)
	h.fatalOnError(schemas.Diagnostics)
	h.schema = schemas.ResourceSchemas[typeName]
	if h.schema == nil {
		t.Fatalf("%s is not registered", typeName)
	}
	h.typ = h.schema.ValueType().(tftypes.Object)
	h.dsSchema = schemas.DataSourceSchemas[typeName]

	providerType := schemas.Provider.ValueType().(tftypes.Object)
	nulls := map[string]tftypes.Value{}
	for name, at := range providerType.AttributeTypes {
		nulls[name] = tftypes.NewValue(at, nil)
	}
	cfg := h.dynamic(providerType, tftypes.NewValue(providerType, nulls))
	configured, err := h.server.ConfigureProvider(h.ctx, &tfprotov6.ConfigureProviderRequest{
		TerraformVersion: "1.11.0",
		Config:           cfg,
	})
	h.must(err)
	h.fatalOnError(configured.Diagnostics)
	return h
}

func (h *grHarness) must(err error) {
	h.t.Helper()
	if err != nil {
		h.t.Fatal(err)
	}
}

func (h *grHarness) fatalOnError(ds []*tfprotov6.Diagnostic) {
	h.t.Helper()
	if hasError(ds) {
		h.t.Fatalf("unexpected diagnostics: %v", diagStrings(ds))
	}
}

func (h *grHarness) dynamic(typ tftypes.Type, v tftypes.Value) *tfprotov6.DynamicValue {
	h.t.Helper()
	dv, err := tfprotov6.NewDynamicValue(typ, v)
	h.must(err)
	return &dv
}

func (h *grHarness) decode(dv *tfprotov6.DynamicValue) tftypes.Value {
	h.t.Helper()
	if dv == nil {
		return tftypes.NewValue(h.typ, nil)
	}
	v, err := dv.Unmarshal(h.typ)
	h.must(err)
	return v
}

func (h *grHarness) null() tftypes.Value { return tftypes.NewValue(h.typ, nil) }

func (h *grHarness) config(j map[string]any) tftypes.Value { return fromJSON(h.t, h.typ, j) }

func (h *grHarness) record(s grStep) { h.steps = append(h.steps, s) }

func (h *grHarness) validate(cfg tftypes.Value) []*tfprotov6.Diagnostic {
	resp, err := h.server.ValidateResourceConfig(h.ctx, &tfprotov6.ValidateResourceConfigRequest{
		TypeName:           h.typeName,
		Config:             h.dynamic(h.typ, cfg),
		ClientCapabilities: &tfprotov6.ValidateResourceConfigClientCapabilities{WriteOnlyAttributesAllowed: true},
	})
	h.must(err)
	return resp.Diagnostics
}

// Validate records the diagnostics of a config without planning it.
func (h *grHarness) Validate(name string, cfg map[string]any) {
	ds := h.validate(h.config(cfg))
	// The framework runs the validators of nested attributes in map order, so
	// several diagnostics of one config come in a different order on every run.
	diags := diagStrings(ds)
	slices.Sort(diags)
	h.record(grStep{Step: name, Config: cfg, ValidateDiags: diags, State: nil})
}

func (h *grHarness) plan(prior, cfg tftypes.Value) (tftypes.Value, []string, []*tfprotov6.Diagnostic) {
	proposed := proposedNew(h.schema.Block.Attributes, prior, cfg)
	resp, err := h.server.PlanResourceChange(h.ctx, &tfprotov6.PlanResourceChangeRequest{
		TypeName:         h.typeName,
		PriorState:       h.dynamic(h.typ, prior),
		ProposedNewState: h.dynamic(h.typ, proposed),
		Config:           h.dynamic(h.typ, cfg),
	})
	h.must(err)
	var replace []string
	for _, p := range resp.RequiresReplace {
		replace = append(replace, p.String())
	}
	if hasError(resp.Diagnostics) {
		return tftypes.Value{}, replace, resp.Diagnostics
	}
	return h.decode(resp.PlannedState), replace, resp.Diagnostics
}

// Plan validates and plans a config on top of prior state, and applies nothing. Use it for a config
// with unknown values, which Terraform plans but cannot apply as is.
func (h *grHarness) Plan(name string, prior tftypes.Value, cfgJSON map[string]any) {
	h.t.Helper()
	step := grStep{Step: name, Config: cfgJSON, State: toJSON(prior)}
	cfg := h.config(cfgJSON)
	vd := h.validate(cfg)
	step.ValidateDiags = diagStrings(vd)
	if !hasError(vd) {
		planned, replace, pd := h.plan(prior, cfg)
		step.PlanDiags = diagStrings(pd)
		step.RequiresReplace = replace
		if !hasError(pd) {
			step.Plan = toJSON(planned)
		}
	}
	h.record(step)
}

// Apply validates, plans, and applies a config on top of prior state, then plans again.
// It returns the new state, or prior when a step fails.
func (h *grHarness) Apply(name string, prior tftypes.Value, cfgJSON map[string]any) tftypes.Value {
	h.t.Helper()
	step := grStep{Step: name, Config: cfgJSON, State: toJSON(prior)}
	cfg := h.config(cfgJSON)

	vd := h.validate(cfg)
	step.ValidateDiags = diagStrings(vd)
	if hasError(vd) {
		h.record(step)
		return prior
	}

	planned, replace, pd := h.plan(prior, cfg)
	step.PlanDiags = diagStrings(pd)
	step.RequiresReplace = replace
	if hasError(pd) {
		h.record(step)
		return prior
	}
	step.Plan = toJSON(planned)
	if len(replace) > 0 {
		// Terraform deletes and creates the resource. The harness stops at the plan.
		h.record(step)
		return prior
	}
	if !prior.IsNull() && planned.Equal(prior) {
		step.NoChange = true
		h.record(step)
		return prior
	}

	resp, err := h.server.ApplyResourceChange(h.ctx, &tfprotov6.ApplyResourceChangeRequest{
		TypeName:     h.typeName,
		PriorState:   h.dynamic(h.typ, prior),
		PlannedState: h.dynamic(h.typ, planned),
		Config:       h.dynamic(h.typ, cfg),
	})
	h.must(err)
	step.Requests = h.api.takeRequests()
	step.ApplyDiags = diagStrings(resp.Diagnostics)
	state := h.decode(resp.NewState)
	if hasError(resp.Diagnostics) {
		h.applied = cfgJSON
		// Terraform core saves the state that a failed apply returns. After a Create it is tainted,
		// and the next apply replaces it; the harness does not model that, so it goes on from null.
		step.State = toJSON(state)
		h.record(step)
		if prior.IsNull() {
			h.failedCreate = state
			return prior
		}
		return state
	}

	step.State = toJSON(state)
	inconsistent("", planned, state, &step.Inconsistent)
	h.applied = cfgJSON

	replanned, _, rd := h.plan(state, cfg)
	if hasError(rd) {
		step.ReplanDiff = diagStrings(rd)
	} else {
		empty := replanned.Equal(state)
		step.ReplanEmpty = &empty
		if !empty {
			step.ReplanDiff = diffPaths(state, replanned)
		}
	}
	h.record(step)
	return state
}

// Refresh reads the resource. It returns the new state, or a null value when the resource is gone.
// ApplyAfterFailedCreate is the next apply after a Create that failed. Terraform core saved the
// state that the Create returned as tainted, so it deletes that resource and creates a new one.
// Without such a state, it creates the resource again.
func (h *grHarness) ApplyAfterFailedCreate(name string, cfgJSON map[string]any) tftypes.Value {
	h.t.Helper()
	if !h.failedCreate.IsNull() && h.failedCreate.IsKnown() {
		h.Destroy(name+": delete the tainted resource", h.failedCreate)
		h.failedCreate = h.null()
		return h.Apply(name+": create it again", h.null(), cfgJSON)
	}
	return h.Apply(name+": no state, so create again", h.null(), cfgJSON)
}

func (h *grHarness) Refresh(name string, state tftypes.Value) tftypes.Value {
	h.t.Helper()
	step := grStep{Step: name}
	resp, err := h.server.ReadResource(h.ctx, &tfprotov6.ReadResourceRequest{
		TypeName: h.typeName, CurrentState: h.dynamic(h.typ, state),
	})
	h.must(err)
	step.Requests = h.api.takeRequests()
	step.ApplyDiags = diagStrings(resp.Diagnostics)
	if hasError(resp.Diagnostics) {
		step.State = toJSON(state)
		h.record(step)
		return state
	}
	next := h.decode(resp.NewState)
	step.State = toJSON(next)
	if !next.IsNull() {
		step.Drift = diffPaths(state, next)
	}
	h.record(step)
	return next
}

// RefreshPlan reads the resource, then plans the config of the last apply on the refreshed state,
// as terraform plan does after a refresh. A value that a read gives in another form than the
// config then shows as a diff.
func (h *grHarness) RefreshPlan(name string, state tftypes.Value) tftypes.Value {
	h.t.Helper()
	next := h.Refresh(name, state)
	if next.IsNull() || h.applied == nil {
		return next
	}
	step := &h.steps[len(h.steps)-1]
	replanned, _, rd := h.plan(next, h.config(h.applied))
	if hasError(rd) {
		step.ReplanDiff = diagStrings(rd)
		return next
	}
	empty := replanned.Equal(next)
	step.ReplanEmpty = &empty
	if !empty {
		step.ReplanDiff = diffPaths(next, replanned)
	}
	return next
}

// Destroy plans and applies the deletion.
func (h *grHarness) Destroy(name string, state tftypes.Value) {
	h.t.Helper()
	step := grStep{Step: name}
	if state.IsNull() {
		step.ApplyDiags = []string{"error: prior state is null"}
		step.State = nil
		h.record(step)
		return
	}
	null := h.null()
	plan, err := h.server.PlanResourceChange(h.ctx, &tfprotov6.PlanResourceChangeRequest{
		TypeName:         h.typeName,
		PriorState:       h.dynamic(h.typ, state),
		ProposedNewState: h.dynamic(h.typ, null),
		Config:           h.dynamic(h.typ, null),
	})
	h.must(err)
	step.PlanDiags = diagStrings(plan.Diagnostics)
	if hasError(plan.Diagnostics) {
		step.State = toJSON(state)
		h.record(step)
		return
	}
	resp, err := h.server.ApplyResourceChange(h.ctx, &tfprotov6.ApplyResourceChangeRequest{
		TypeName:       h.typeName,
		PriorState:     h.dynamic(h.typ, state),
		PlannedState:   plan.PlannedState,
		Config:         h.dynamic(h.typ, null),
		PlannedPrivate: plan.PlannedPrivate,
	})
	h.must(err)
	step.Requests = h.api.takeRequests()
	step.ApplyDiags = diagStrings(resp.Diagnostics)
	step.State = toJSON(h.decode(resp.NewState))
	h.record(step)
}

// Import imports an id, then refreshes it, as `terraform import` does.
func (h *grHarness) Import(name, id string) tftypes.Value {
	h.t.Helper()
	step := grStep{Step: name}
	resp, err := h.server.ImportResourceState(h.ctx, &tfprotov6.ImportResourceStateRequest{TypeName: h.typeName, ID: id})
	h.must(err)
	step.ApplyDiags = diagStrings(resp.Diagnostics)
	if hasError(resp.Diagnostics) || len(resp.ImportedResources) != 1 {
		h.record(step)
		return h.null()
	}
	h.record(step)
	return h.Refresh(name+" (refresh)", h.decode(resp.ImportedResources[0].State))
}

// UpgradeV0 upgrades a raw state that was written by schema version 0.
func (h *grHarness) UpgradeV0(name, rawJSON string) tftypes.Value {
	h.t.Helper()
	step := grStep{Step: name, Config: json.RawMessage(rawJSON)}
	resp, err := h.server.UpgradeResourceState(h.ctx, &tfprotov6.UpgradeResourceStateRequest{
		TypeName: h.typeName, Version: 0, RawState: &tfprotov6.RawState{JSON: []byte(rawJSON)},
	})
	h.must(err)
	step.Requests = h.api.takeRequests()
	step.ApplyDiags = diagStrings(resp.Diagnostics)
	if hasError(resp.Diagnostics) {
		h.record(step)
		return h.null()
	}
	state := h.decode(resp.UpgradedState)
	step.State = toJSON(state)
	h.record(step)
	return state
}

// ReadDataSource validates the config of the data source with the type name of the harness, and
// reads it, as Terraform core does during a plan. The state of the step is the data source result.
func (h *grHarness) ReadDataSource(name string, cfgJSON map[string]any) {
	h.t.Helper()
	if h.dsSchema == nil {
		h.t.Fatalf("%s has no data source", h.typeName)
	}
	typ := h.dsSchema.ValueType().(tftypes.Object)
	step := grStep{Step: name, Config: cfgJSON}
	cfg := h.dynamic(typ, fromJSON(h.t, typ, cfgJSON))

	validated, err := h.server.ValidateDataResourceConfig(h.ctx, &tfprotov6.ValidateDataResourceConfigRequest{
		TypeName: h.typeName, Config: cfg,
	})
	h.must(err)
	step.ValidateDiags = diagStrings(validated.Diagnostics)
	slices.Sort(step.ValidateDiags)
	if hasError(validated.Diagnostics) {
		h.record(step)
		return
	}

	resp, err := h.server.ReadDataSource(h.ctx, &tfprotov6.ReadDataSourceRequest{TypeName: h.typeName, Config: cfg})
	h.must(err)
	step.Requests = h.api.takeRequests()
	step.ApplyDiags = diagStrings(resp.Diagnostics)
	if resp.State != nil {
		state, err := resp.State.Unmarshal(typ)
		h.must(err)
		step.State = toJSON(state)
	}
	h.record(step)
}

// dataSourceSchemaText returns the data source schema as Terraform core receives it: one line per
// attribute, sorted by path, with its type, its flags, and its descriptions.
func dataSourceSchemaText(s *tfprotov6.Schema) string {
	var lines []string
	var walk func(prefix string, attrs []*tfprotov6.SchemaAttribute)
	walk = func(prefix string, attrs []*tfprotov6.SchemaAttribute) {
		for _, a := range attrs {
			p := prefix + a.Name
			kind := "nested"
			if a.Type != nil {
				kind = a.Type.String()
			} else if a.NestedType != nil {
				kind = "nested " + a.NestedType.Nesting.String()
			}
			var flags []string
			for _, f := range []struct {
				on   bool
				name string
			}{{a.Required, "required"}, {a.Optional, "optional"}, {a.Computed, "computed"}, {a.Sensitive, "sensitive"}, {a.Deprecated, "deprecated"}, {a.WriteOnly, "write-only"}} {
				if f.on {
					flags = append(flags, f.name)
				}
			}
			line := p + "  " + kind + " " + strings.Join(flags, ",")
			if a.Description != "" {
				line += "\n    description: " + strings.ReplaceAll(a.Description, "\n", "\\n")
			}
			lines = append(lines, line)
			if a.NestedType != nil {
				walk(p+".", a.NestedType.Attributes)
			}
		}
	}
	walk("", s.Block.Attributes)
	slices.Sort(lines)
	header := "(data source schema)"
	if s.Block.Description != "" {
		header += "\n    description: " + strings.ReplaceAll(s.Block.Description, "\n", "\\n")
	}
	if s.Block.Deprecated {
		header += "\n    deprecated"
	}
	return header + "\n" + strings.Join(lines, "\n") + "\n"
}

// --- values ---

// idOf returns the id in a state.
func idOf(state tftypes.Value) string {
	var m map[string]tftypes.Value
	var id string
	if state.As(&m) != nil || m["id"].As(&id) != nil {
		return ""
	}
	return id
}

// fromJSON converts JSON-like Go values to a value of the given type. A missing object key is null.
// unknownJSON in a config is a value that Terraform does not know during the plan, such as one
// from a resource that does not exist yet. A golden shows it as "(unknown)".
type unknownJSON struct{}

func (unknownJSON) MarshalJSON() ([]byte, error) { return []byte(`"(unknown)"`), nil }

func fromJSON(t *testing.T, typ tftypes.Type, v any) tftypes.Value {
	t.Helper()
	if v == nil {
		return tftypes.NewValue(typ, nil)
	}
	if _, ok := v.(unknownJSON); ok {
		return tftypes.NewValue(typ, tftypes.UnknownValue)
	}
	switch {
	case typ.Is(tftypes.DynamicPseudoType):
		return dynamicFromJSON(t, v)
	case typ.Is(tftypes.String):
		return tftypes.NewValue(typ, v.(string))
	case typ.Is(tftypes.Bool):
		return tftypes.NewValue(typ, v.(bool))
	case typ.Is(tftypes.Number):
		return tftypes.NewValue(typ, big.NewFloat(v.(float64)))
	case typ.Is(tftypes.Object{}):
		m := v.(map[string]any)
		out := map[string]tftypes.Value{}
		for name, at := range typ.(tftypes.Object).AttributeTypes {
			out[name] = fromJSON(t, at, m[name])
		}
		for name := range m {
			if _, ok := out[name]; !ok {
				t.Fatalf("config has %q, which is not in the schema", name)
			}
		}
		return tftypes.NewValue(typ, out)
	case typ.Is(tftypes.List{}):
		return tftypes.NewValue(typ, elems(t, typ.(tftypes.List).ElementType, v))
	case typ.Is(tftypes.Set{}):
		return tftypes.NewValue(typ, elems(t, typ.(tftypes.Set).ElementType, v))
	case typ.Is(tftypes.Map{}):
		out := map[string]tftypes.Value{}
		for k, e := range v.(map[string]any) {
			out[k] = fromJSON(t, typ.(tftypes.Map).ElementType, e)
		}
		return tftypes.NewValue(typ, out)
	}
	t.Fatalf("unsupported type %s", typ)
	return tftypes.Value{}
}

// dynamicFromJSON gives a value of a dynamic attribute the type that Terraform infers from HCL: a
// JSON object is an object, a JSON array is a tuple, and the scalars are string, number, and bool.
func dynamicFromJSON(t *testing.T, v any) tftypes.Value {
	t.Helper()
	switch x := v.(type) {
	case string:
		return tftypes.NewValue(tftypes.String, x)
	case bool:
		return tftypes.NewValue(tftypes.Bool, x)
	case float64:
		return tftypes.NewValue(tftypes.Number, big.NewFloat(x))
	case []any:
		types := make([]tftypes.Type, len(x))
		values := make([]tftypes.Value, len(x))
		for i, e := range x {
			values[i] = dynamicFromJSON(t, e)
			types[i] = values[i].Type()
		}
		return tftypes.NewValue(tftypes.Tuple{ElementTypes: types}, values)
	case map[string]any:
		types := map[string]tftypes.Type{}
		values := map[string]tftypes.Value{}
		for k, e := range x {
			values[k] = dynamicFromJSON(t, e)
			types[k] = values[k].Type()
		}
		return tftypes.NewValue(tftypes.Object{AttributeTypes: types}, values)
	}
	t.Fatalf("unsupported dynamic value %T", v)
	return tftypes.Value{}
}

func elems(t *testing.T, elem tftypes.Type, v any) []tftypes.Value {
	out := make([]tftypes.Value, 0)
	for _, e := range v.([]any) {
		out = append(out, fromJSON(t, elem, e))
	}
	return out
}

// toJSON converts a value to JSON-like Go values. An unknown value is the string "(unknown)".
func toJSON(v tftypes.Value) any {
	switch {
	case !v.IsKnown():
		return "(unknown)"
	case v.IsNull():
		return nil
	}
	switch {
	case v.Type().Is(tftypes.String):
		var s string
		_ = v.As(&s)
		return s
	case v.Type().Is(tftypes.Bool):
		var b bool
		_ = v.As(&b)
		return b
	case v.Type().Is(tftypes.Number):
		var n big.Float
		_ = v.As(&n)
		f, _ := n.Float64()
		return f
	case v.Type().Is(tftypes.List{}), v.Type().Is(tftypes.Set{}), v.Type().Is(tftypes.Tuple{}):
		var l []tftypes.Value
		_ = v.As(&l)
		out := make([]any, 0, len(l))
		for _, e := range l {
			out = append(out, toJSON(e))
		}
		return out
	default: // object, map
		var m map[string]tftypes.Value
		_ = v.As(&m)
		out := map[string]any{}
		for k, e := range m {
			out[k] = toJSON(e)
		}
		return out
	}
}

// proposedNew emulates the proposed new state that Terraform core builds: the
// config value where it is set, else the prior value for a computed attribute. An
// optional nested attribute whose prior value holds a non-computed value takes the
// null config value instead, because that value must have come from the config
// (optionalValueNotComputable).
func proposedNew(attrs []*tfprotov6.SchemaAttribute, prior, cfg tftypes.Value) tftypes.Value {
	if !cfg.IsKnown() {
		// Terraform core proposes an unknown config value as it is.
		return cfg
	}
	var c, p map[string]tftypes.Value
	_ = cfg.As(&c)
	if !prior.IsNull() {
		_ = prior.As(&p)
	}
	out := map[string]tftypes.Value{}
	for _, a := range attrs {
		cv := c[a.Name]
		pv, hasPrior := p[a.Name]
		switch {
		case a.WriteOnly:
			// Terraform core keeps write-only values out of proposed state.
			out[a.Name] = tftypes.NewValue(cv.Type(), nil)
		case !cv.IsKnown():
			out[a.Name] = cv
		case !cv.IsNull() && a.NestedType != nil:
			out[a.Name] = proposedNested(a.NestedType, pv, hasPrior, cv)
		case !cv.IsNull():
			out[a.Name] = cv
		case a.Computed && hasPrior && !(a.Optional && a.NestedType != nil && hasNonComputedValue(a.NestedType, pv)):
			out[a.Name] = pv
		default:
			out[a.Name] = cv
		}
	}
	return tftypes.NewValue(cfg.Type(), out)
}

// hasNonComputedValue reports whether v, a value of the nested type o, holds a
// non-null value of an attribute that is not computed, at any depth.
func hasNonComputedValue(o *tfprotov6.SchemaObject, v tftypes.Value) bool {
	if v.IsNull() || !v.IsKnown() {
		return false
	}
	var items []tftypes.Value
	switch o.Nesting {
	case tfprotov6.SchemaObjectNestingModeList, tfprotov6.SchemaObjectNestingModeSet:
		_ = v.As(&items)
	case tfprotov6.SchemaObjectNestingModeMap:
		var m map[string]tftypes.Value
		_ = v.As(&m)
		for _, e := range m {
			items = append(items, e)
		}
	default:
		items = []tftypes.Value{v}
	}
	for _, item := range items {
		if item.IsNull() || !item.IsKnown() {
			continue
		}
		var m map[string]tftypes.Value
		_ = item.As(&m)
		for _, a := range o.Attributes {
			av, ok := m[a.Name]
			if !ok || av.IsNull() {
				continue
			}
			if !a.Computed {
				return true
			}
			if a.NestedType != nil && hasNonComputedValue(a.NestedType, av) {
				return true
			}
		}
	}
	return false
}

func proposedNested(o *tfprotov6.SchemaObject, prior tftypes.Value, hasPrior bool, cfg tftypes.Value) tftypes.Value {
	if !cfg.IsKnown() {
		// Terraform core keeps a configuration that is not known at all, such as a variable.
		return cfg
	}
	if !hasPrior {
		prior = tftypes.NewValue(cfg.Type(), nil)
	}
	switch o.Nesting {
	case tfprotov6.SchemaObjectNestingModeSingle:
		return proposedNew(o.Attributes, prior, cfg)
	case tfprotov6.SchemaObjectNestingModeList:
		var cs, ps []tftypes.Value
		_ = cfg.As(&cs)
		if !prior.IsNull() {
			_ = prior.As(&ps)
		}
		out := make([]tftypes.Value, len(cs))
		for i := range cs {
			pi := tftypes.NewValue(cs[i].Type(), nil)
			if i < len(ps) {
				pi = ps[i]
			}
			out[i] = proposedNew(o.Attributes, pi, cs[i])
		}
		return tftypes.NewValue(cfg.Type(), out)
	case tfprotov6.SchemaObjectNestingModeMap:
		// Terraform core pairs map elements by key.
		var cs, ps map[string]tftypes.Value
		_ = cfg.As(&cs)
		if !prior.IsNull() {
			_ = prior.As(&ps)
		}
		out := make(map[string]tftypes.Value, len(cs))
		for k, c := range cs {
			pi, ok := ps[k]
			if !ok {
				pi = tftypes.NewValue(c.Type(), nil)
			}
			out[k] = proposedNew(o.Attributes, pi, c)
		}
		return tftypes.NewValue(cfg.Type(), out)
	case tfprotov6.SchemaObjectNestingModeSet:
		// Terraform core pairs each configured item with an unused prior item that differs only in
		// computed values that the config leaves null, and takes those values from it
		// (proposedNewNestingSet).
		var cs, ps []tftypes.Value
		_ = cfg.As(&cs)
		if !prior.IsNull() && prior.IsKnown() {
			_ = prior.As(&ps)
		}
		used := make([]bool, len(ps))
		out := make([]tftypes.Value, len(cs))
		for i, c := range cs {
			pi := tftypes.NewValue(c.Type(), nil)
			for k, p := range ps {
				if !used[k] && validPriorFromConfig(o, p, c) {
					pi, used[k] = p, true
					break
				}
			}
			out[i] = proposedNew(o.Attributes, pi, c)
		}
		return tftypes.NewValue(cfg.Type(), out)
	}
	return cfg
}

// validPriorFromConfig reports whether prior can be the prior value of the configured set item
// cfg, as Terraform core decides it: every value that differs is a computed leaf that the config
// leaves null, and no differing value is a set.
func validPriorFromConfig(o *tfprotov6.SchemaObject, prior, cfg tftypes.Value) bool {
	if prior.Equal(cfg) {
		return true
	}
	if prior.IsNull() || !prior.IsKnown() || cfg.IsNull() || !cfg.IsKnown() {
		return prior.IsNull()
	}
	var p, c map[string]tftypes.Value
	_ = prior.As(&p)
	_ = cfg.As(&c)
	for _, a := range o.Attributes {
		pv, cv := p[a.Name], c[a.Name]
		if pv.Equal(cv) {
			continue
		}
		if cv.Type().Is(tftypes.Set{}) {
			return false
		}
		if a.NestedType != nil {
			if !validPriorNested(a.NestedType, pv, cv) {
				return false
			}
			continue
		}
		if !a.Computed || !cv.IsNull() {
			return false
		}
	}
	return true
}

func validPriorNested(o *tfprotov6.SchemaObject, prior, cfg tftypes.Value) bool {
	if prior.IsNull() {
		return true
	}
	if cfg.IsNull() || !cfg.IsKnown() {
		return false
	}
	switch o.Nesting {
	case tfprotov6.SchemaObjectNestingModeList:
		var ps, cs []tftypes.Value
		_ = prior.As(&ps)
		_ = cfg.As(&cs)
		if len(ps) > len(cs) {
			return false
		}
		for i := range ps {
			if !validPriorFromConfig(o, ps[i], cs[i]) {
				return false
			}
		}
		return true
	case tfprotov6.SchemaObjectNestingModeMap:
		var ps, cs map[string]tftypes.Value
		_ = prior.As(&ps)
		_ = cfg.As(&cs)
		for k, pv := range ps {
			cv, ok := cs[k]
			if !ok || !validPriorFromConfig(o, pv, cv) {
				return false
			}
		}
		return true
	}
	return validPriorFromConfig(o, prior, cfg)
}

// inconsistent appends the paths where a known planned value differs from the applied value.
func inconsistent(path string, planned, actual tftypes.Value, out *[]string) {
	if !planned.IsKnown() {
		return
	}
	at := path
	if at == "" {
		at = "(root)"
	}
	if !actual.IsKnown() {
		*out = append(*out, at+" (unknown after apply)")
		return
	}
	if planned.IsNull() || actual.IsNull() {
		if planned.IsNull() != actual.IsNull() {
			*out = append(*out, at)
		}
		return
	}
	switch {
	case planned.Type().Is(tftypes.Object{}), planned.Type().Is(tftypes.Map{}):
		var pm, am map[string]tftypes.Value
		_ = planned.As(&pm)
		_ = actual.As(&am)
		for _, k := range sortedKeys(pm) {
			sub := k
			if path != "" {
				sub = path + "." + k
			}
			inconsistent(sub, pm[k], am[k], out)
		}
		for _, k := range sortedKeys(am) {
			if _, ok := pm[k]; !ok {
				*out = append(*out, at+"."+k+" (not planned)")
			}
		}
	case planned.Type().Is(tftypes.List{}):
		var pl, al []tftypes.Value
		_ = planned.As(&pl)
		_ = actual.As(&al)
		if len(pl) != len(al) {
			*out = append(*out, fmt.Sprintf("%s (length %d, planned %d)", at, len(al), len(pl)))
			return
		}
		for i := range pl {
			inconsistent(fmt.Sprintf("%s[%d]", path, i), pl[i], al[i], out)
		}
	case planned.Type().Is(tftypes.Set{}) && !planned.IsFullyKnown():
		// As Terraform core does, the planned items, which have unknown values, and the items of the
		// result must pair one to one.
		var ps, as []tftypes.Value
		_ = planned.As(&ps)
		_ = actual.As(&as)
		if len(ps) != len(as) || !pairSetItems(ps, as) {
			*out = append(*out, at)
		}
	default:
		if !planned.Equal(actual) {
			*out = append(*out, at)
		}
	}
}

// pairSetItems reports whether every planned item pairs with its own item of the result, where an
// unknown planned value matches any value (a bipartite matching by augmenting paths).
func pairSetItems(planned, actual []tftypes.Value) bool {
	matches := func(p, a tftypes.Value) bool {
		var found []string
		inconsistent("", p, a, &found)
		return len(found) == 0
	}
	owner := make([]int, len(actual))
	for i := range owner {
		owner[i] = -1
	}
	var assign func(p int, seen []bool) bool
	assign = func(p int, seen []bool) bool {
		for a := range actual {
			if seen[a] || !matches(planned[p], actual[a]) {
				continue
			}
			seen[a] = true
			if owner[a] < 0 || assign(owner[a], seen) {
				owner[a] = p
				return true
			}
		}
		return false
	}
	for p := range planned {
		if !assign(p, make([]bool, len(actual))) {
			return false
		}
	}
	return true
}

func sortedKeys(m map[string]tftypes.Value) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

func diffPaths(a, b tftypes.Value) []string {
	diffs, err := a.Diff(b)
	if err != nil {
		return []string{"diff error: " + err.Error()}
	}
	var out []string
	for _, d := range diffs {
		out = append(out, d.Path.String())
	}
	slices.Sort(out)
	return out
}

// --- diagnostics ---

var pointerPattern = regexp.MustCompile(`0x[0-9a-f]{6,}`)

func diagStrings(ds []*tfprotov6.Diagnostic) []string {
	var out []string
	for _, d := range ds {
		severity := "error"
		if d.Severity == tfprotov6.DiagnosticSeverityWarning {
			severity = "warning"
		}
		s := severity + ": " + pointerPattern.ReplaceAllString(d.Summary, "0xPTR")
		if d.Attribute != nil {
			s += " [" + d.Attribute.String() + "]"
		}
		if d.Detail != "" {
			s += " | " + pointerPattern.ReplaceAllString(d.Detail, "0xPTR")
		}
		out = append(out, s)
	}
	return out
}

func hasError(ds []*tfprotov6.Diagnostic) bool {
	for _, d := range ds {
		if d.Severity == tfprotov6.DiagnosticSeverityError {
			return true
		}
	}
	return false
}

// --- golden files ---

const grGoldenDir = "testdata/global_router_baseline"

// goldenDir is the folder of the golden files. GR_GOLDEN_DIR names another folder, so that a run
// can write its files next to the committed ones and a person can compare them.
func goldenDir() string {
	if dir := os.Getenv("GR_GOLDEN_DIR"); dir != "" {
		return dir
	}
	return grGoldenDir
}

// checkGolden compares got with the golden file. UPDATE_GOLDEN=1 rewrites the file.
func checkGolden(t *testing.T, name string, got []byte) {
	checkGoldenAt(t, goldenDir(), name, got)
}

func checkGoldenAt(t *testing.T, dir, name string, got []byte) {
	t.Helper()
	file := filepath.Join(dir, name)
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("%v\nrun with UPDATE_GOLDEN=1 to create it", err)
	}
	if !bytes.Equal(want, got) {
		t.Errorf("%s differs from the golden file (first difference below).\n%s\nrun with UPDATE_GOLDEN=1 to accept the change", name, firstDifference(string(want), string(got)))
	}
}

func firstDifference(want, got string) string {
	w, g := strings.Split(want, "\n"), strings.Split(got, "\n")
	for i := 0; i < len(w) || i < len(g); i++ {
		var wl, gl string
		if i < len(w) {
			wl = w[i]
		}
		if i < len(g) {
			gl = g[i]
		}
		if wl != gl {
			return fmt.Sprintf("line %d\n  golden: %s\n  actual: %s", i+1, wl, gl)
		}
	}
	return ""
}

func marshalGolden(t *testing.T, v any) []byte {
	t.Helper()
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// A planned set with unknown values pairs one to one with the result: one planned item cannot
// stand for two items of the result, and two planned items cannot share one.
func TestInconsistentSetPairsItemsOneToOne(t *testing.T) {
	obj := tftypes.Object{AttributeTypes: map[string]tftypes.Type{"name": tftypes.String, "n": tftypes.Number}}
	set := tftypes.Set{ElementType: obj}
	item := func(name string, n any) tftypes.Value {
		return tftypes.NewValue(obj, map[string]tftypes.Value{"name": tftypes.NewValue(tftypes.String, name), "n": tftypes.NewValue(tftypes.Number, n)})
	}
	known := func(n float64) any { return big.NewFloat(n) }
	for _, test := range []struct {
		name            string
		planned, actual []tftypes.Value
		want            bool
	}{
		{"pairs", []tftypes.Value{item("a", tftypes.UnknownValue), item("b", known(2))}, []tftypes.Value{item("b", known(2)), item("a", known(1))}, false},
		{"an extra item in the result", []tftypes.Value{item("a", tftypes.UnknownValue)}, []tftypes.Value{item("a", known(1)), item("a", known(2))}, true},
		{"two planned items for one item", []tftypes.Value{item("a", tftypes.UnknownValue), item("a", known(1))}, []tftypes.Value{item("a", known(1)), item("b", known(1))}, true},
	} {
		var out []string
		inconsistent("", tftypes.NewValue(set, test.planned), tftypes.NewValue(set, test.actual), &out)
		if got := len(out) != 0; got != test.want {
			t.Errorf("%s: inconsistent = %v, want %t", test.name, out, test.want)
		}
	}
}

func TestProposedNewNullsAnOmittedNestedValueWithConfiguredParts(t *testing.T) {
	obj := tftypes.Object{AttributeTypes: map[string]tftypes.Type{"name": tftypes.String, "id": tftypes.String}}
	set := tftypes.Set{ElementType: obj}
	root := tftypes.Object{AttributeTypes: map[string]tftypes.Type{"items": set}}
	nested := &tfprotov6.SchemaObject{Nesting: tfprotov6.SchemaObjectNestingModeSet, Attributes: []*tfprotov6.SchemaAttribute{
		{Name: "name", Optional: true},
		{Name: "id", Computed: true},
	}}
	attrs := []*tfprotov6.SchemaAttribute{{Name: "items", Optional: true, Computed: true, NestedType: nested}}
	item := func(name, id any) tftypes.Value {
		return tftypes.NewValue(obj, map[string]tftypes.Value{"name": tftypes.NewValue(tftypes.String, name), "id": tftypes.NewValue(tftypes.String, id)})
	}
	cfg := tftypes.NewValue(root, map[string]tftypes.Value{"items": tftypes.NewValue(set, nil)})
	for _, test := range []struct {
		name     string
		prior    tftypes.Value
		wantNull bool
	}{
		{"a prior item has a configured name", item("a", "1"), true},
		{"a prior item has only computed values", item(nil, "1"), false},
	} {
		prior := tftypes.NewValue(root, map[string]tftypes.Value{"items": tftypes.NewValue(set, []tftypes.Value{test.prior})})
		var got map[string]tftypes.Value
		_ = proposedNew(attrs, prior, cfg).As(&got)
		if got["items"].IsNull() != test.wantNull {
			t.Errorf("%s: proposed items = %v, want null %t", test.name, got["items"], test.wantNull)
		}
	}
}
