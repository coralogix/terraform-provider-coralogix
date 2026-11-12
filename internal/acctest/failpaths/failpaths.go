// Package failpaths checks what a generated resource does when the API fails, without a tenant.
package failpaths

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
	"github.com/zclconf/go-cty/cty/function/stdlib"
)

// Spec describes a generated resource for Run.
type Spec struct {
	// ProviderServer serves the provider, as the binary does.
	ProviderServer func() (tfprotov6.ProviderServer, error)
	// TypeName is the Terraform type, such as coralogix_alert.
	TypeName string
	// IDAttr is the attribute that holds the id of the resource. The prior state of a test has a made-up
	// id there.
	IDAttr string
	// Singleton: the API holds one resource of this kind and has no "gone" answer for it, so a missing
	// resource is not a case to check on update and delete.
	Singleton bool
	// NoDelete: the API cannot remove the resource. Delete makes no call, warns, and removes the
	// resource from the state.
	NoDelete bool
	// Config is the minimal config of the resource: one resource block of HCL that holds literal values
	// only. A config with a reference to another resource skips the test.
	Config string
}

// A reply of the fake API. A 404 that names a missing resource is the API saying that the resource is
// gone. A 404 of a route that does not exist is an error: the service can be missing in the region.
const (
	replyGone  = `{"code":404,"message":"Not Found: The requested resource was not found"}`
	replyRoute = `{"code":404,"message":"Not Found: Not Found"}`
	replyFail  = `{"code":500,"message":"Internal Server Error: the service failed"}`
)

type failureReply struct {
	status int
	body   string
}

var (
	failGone  = failureReply{http.StatusNotFound, replyGone}
	failRoute = failureReply{http.StatusNotFound, replyRoute}
	failError = failureReply{http.StatusInternalServerError, replyFail}
)

// failureAPI answers every request with the same reply, which a test changes.
type failureAPI struct {
	mu       sync.Mutex
	reply    failureReply
	requests int
}

func (f *failureAPI) set(reply failureReply) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reply, f.requests = reply, 0
}

func (f *failureAPI) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.requests
}

func (f *failureAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	_, _ = io.Copy(io.Discard, r.Body)
	f.mu.Lock()
	f.requests++
	reply := f.reply
	f.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(reply.status)
	_, _ = io.WriteString(w, reply.body)
}

type redirect struct {
	target *url.URL
	next   http.RoundTripper
}

func (r redirect) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.URL.Scheme = r.target.Scheme
	req.URL.Host = r.target.Host
	req.Host = r.target.Host
	return r.next.RoundTrip(req)
}

// failureHarness drives the resource through the plugin protocol, as Terraform does.
type failureHarness struct {
	t      *testing.T
	ctx    context.Context
	server tfprotov6.ProviderServer
	api    *failureAPI
	typ    tftypes.Object
	spec   Spec
	config tftypes.Value
}

// Run checks what a generated resource does when the API fails, without a tenant: the API
// is a local server that answers every request with the same error.
//
//   - Create fails: an error, and no state, so that no resource with an empty id is kept.
//   - Read of a resource that the API says is gone: no error, and the resource leaves the state.
//   - Read, Update, and Delete of a 500, and of a 404 of a route that does not exist: an error.
//   - Update of a resource that the API says is gone: an error that says that the resource no longer
//     exists, and not a state with an empty id.
//   - Delete of a resource that the API says is gone: no error. A singleton has no such answer, and a
//     resource that the API cannot delete only leaves the state.
func Run(t *testing.T, spec Spec) {
	t.Helper()
	h := newFailureHarness(t, spec)
	prior := h.priorState()

	t.Run("create", func(t *testing.T) {
		for name, reply := range map[string]failureReply{"500": failError, "gone": failGone, "route": failRoute} {
			t.Run(name, func(t *testing.T) {
				h.api.set(reply)
				planned := h.plan(h.null(), h.config)
				resp := h.apply(h.null(), planned, h.config)
				h.requireCall(t, "create")
				requireError(t, "create", resp.Diagnostics)
				if state := h.decode(resp.NewState); !state.IsNull() {
					t.Errorf("create failed, so the state must be null, but it holds %d attributes", len(attributesOf(state)))
				}
			})
		}
	})
	t.Run("read", func(t *testing.T) {
		h.api.set(failGone)
		resp := h.read(prior)
		h.requireCall(t, "read")
		requireNoError(t, "read of a resource that is gone", resp.Diagnostics)
		if state := h.decode(resp.NewState); !state.IsNull() {
			t.Errorf("the resource is gone, so it must leave the state, but it holds %d attributes", len(attributesOf(state)))
		}
		for name, reply := range map[string]failureReply{"500": failError, "route": failRoute} {
			t.Run(name, func(t *testing.T) {
				h.api.set(reply)
				diags := h.read(prior).Diagnostics
				h.requireCall(t, "read")
				requireError(t, "read", diags)
			})
		}
	})
	t.Run("update", func(t *testing.T) {
		h.api.set(failGone)
		planned := h.plan(prior, h.config)
		resp := h.apply(prior, planned, h.config)
		h.requireCall(t, "update")
		requireError(t, "update of a resource that is gone", resp.Diagnostics)
		if !h.spec.Singleton && !strings.Contains(strings.ToLower(diagText(resp.Diagnostics)), "no longer exists") {
			t.Errorf("update of a resource that is gone must say that it no longer exists, got: %s", diagText(resp.Diagnostics))
		}
		for name, reply := range map[string]failureReply{"500": failError, "route": failRoute} {
			t.Run(name, func(t *testing.T) {
				h.api.set(reply)
				planned := h.plan(prior, h.config)
				diags := h.apply(prior, planned, h.config).Diagnostics
				h.requireCall(t, "update")
				requireError(t, "update", diags)
			})
		}
	})
	t.Run("delete", func(t *testing.T) {
		if spec.NoDelete {
			for name, reply := range map[string]failureReply{"gone": failGone, "500": failError, "route": failRoute} {
				h.api.set(reply)
				requireNoError(t, "delete of "+name+", which the API cannot do (it only leaves the state)", h.apply(prior, h.null(), h.null()).Diagnostics)
			}
			return
		}
		if !spec.Singleton {
			h.api.set(failGone)
			resp := h.apply(prior, h.null(), h.null())
			requireNoError(t, "delete of a resource that is gone", resp.Diagnostics)
		}
		for name, reply := range map[string]failureReply{"500": failError, "route": failRoute} {
			t.Run(name, func(t *testing.T) {
				h.api.set(reply)
				diags := h.apply(prior, h.null(), h.null()).Diagnostics
				h.requireCall(t, "delete")
				requireError(t, "delete", diags)
			})
		}
	})
}

func newFailureHarness(t *testing.T, spec Spec) *failureHarness {
	t.Helper()
	cfgCty, err := configFromHCL(spec.Config, spec.TypeName)
	if err != nil {
		t.Skipf("%s has no config that this test can read: %v", spec.TypeName, err)
	}
	t.Setenv("CORALOGIX_API_KEY", "failure-paths-key")
	t.Setenv("CORALOGIX_DOMAIN", "eu2.coralogix.com")
	t.Setenv("CORALOGIX_ENV", "")

	api := &failureAPI{reply: failError}
	srv := httptest.NewServer(api)
	t.Cleanup(srv.Close)
	target, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	original := http.DefaultTransport
	http.DefaultTransport = redirect{target: target, next: original}
	t.Cleanup(func() { http.DefaultTransport = original })

	h := &failureHarness{t: t, ctx: context.Background(), api: api, spec: spec}
	if h.server, err = spec.ProviderServer(); err != nil {
		t.Fatal(err)
	}
	schemas, err := h.server.GetProviderSchema(h.ctx, &tfprotov6.GetProviderSchemaRequest{})
	if err != nil {
		t.Fatal(err)
	}
	fatalOnError(t, schemas.Diagnostics)
	schema := schemas.ResourceSchemas[spec.TypeName]
	if schema == nil {
		t.Fatalf("%s is not registered", spec.TypeName)
	}
	h.typ = schema.ValueType().(tftypes.Object)

	providerType := schemas.Provider.ValueType().(tftypes.Object)
	nulls := map[string]tftypes.Value{}
	for name, at := range providerType.AttributeTypes {
		nulls[name] = tftypes.NewValue(at, nil)
	}
	providerConfig := h.dynamic(providerType, tftypes.NewValue(providerType, nulls))
	configured, err := h.server.ConfigureProvider(h.ctx, &tfprotov6.ConfigureProviderRequest{TerraformVersion: "1.11.0", Config: providerConfig})
	if err != nil {
		t.Fatal(err)
	}
	fatalOnError(t, configured.Diagnostics)

	if h.config, err = fromCty(cfgCty, h.typ); err != nil {
		t.Skipf("the minimal config of %s does not fit its schema: %v", spec.TypeName, err)
	}
	// A config that Terraform would refuse never reaches the provider.
	valid, err := h.server.ValidateResourceConfig(h.ctx, &tfprotov6.ValidateResourceConfigRequest{TypeName: spec.TypeName, Config: h.dynamic(h.typ, h.config)})
	if err != nil {
		t.Fatal(err)
	}
	if hasError(valid.Diagnostics) {
		t.Skipf("the minimal config of %s does not validate offline: %s", spec.TypeName, diagText(valid.Diagnostics))
	}
	return h
}

// configFromHCL reads the resource block and returns its attributes as one object value. It fails on
// an attribute that is not a literal, such as a reference to another resource. The functions that a
// minimal config uses (jsonencode) work.
func configFromHCL(src, typeName string) (cty.Value, error) {
	file, diags := hclsyntax.ParseConfig([]byte(src), "config.tf", hcl.InitialPos)
	if diags.HasErrors() {
		return cty.NilVal, diags
	}
	body, ok := file.Body.(*hclsyntax.Body)
	if !ok {
		return cty.NilVal, fmt.Errorf("no body")
	}
	var found *hclsyntax.Block
	for _, block := range body.Blocks {
		if block.Type == "resource" && len(block.Labels) == 2 && block.Labels[0] == typeName {
			found = block
			break
		}
	}
	if found == nil {
		return cty.NilVal, fmt.Errorf("no resource block of %s", typeName)
	}
	ctx := &hcl.EvalContext{Functions: map[string]function.Function{
		"jsonencode": stdlib.JSONEncodeFunc,
		"tolist":     stdlib.MakeToFunc(cty.List(cty.DynamicPseudoType)),
		"toset":      stdlib.MakeToFunc(cty.Set(cty.DynamicPseudoType)),
		"tomap":      stdlib.MakeToFunc(cty.Map(cty.DynamicPseudoType)),
	}}
	values := map[string]cty.Value{}
	for name, attr := range found.Body.Attributes {
		v, diags := attr.Expr.Value(ctx)
		if diags.HasErrors() {
			return cty.NilVal, fmt.Errorf("attribute %s is not a literal: %w", name, diags)
		}
		values[name] = v
	}
	return cty.ObjectVal(values), nil
}

// fromCty converts a value of the config to the value of the schema type. An attribute that the config
// leaves out is null. A dynamic attribute takes the type of its value.
func fromCty(v cty.Value, typ tftypes.Type) (tftypes.Value, error) {
	if v.IsNull() || !v.IsKnown() {
		return tftypes.NewValue(typ, nil), nil
	}
	if typ.Is(tftypes.DynamicPseudoType) {
		inner, err := tftypesType(v.Type())
		if err != nil {
			return tftypes.Value{}, err
		}
		return fromCty(v, inner)
	}
	switch {
	case typ.Is(tftypes.String):
		return tftypes.NewValue(typ, v.AsString()), nil
	case typ.Is(tftypes.Bool):
		return tftypes.NewValue(typ, v.True()), nil
	case typ.Is(tftypes.Number):
		return tftypes.NewValue(typ, v.AsBigFloat()), nil
	}
	switch t := typ.(type) {
	case tftypes.List:
		return collectionFromCty(v, t, t.ElementType)
	case tftypes.Set:
		return collectionFromCty(v, t, t.ElementType)
	case tftypes.Map:
		out := map[string]tftypes.Value{}
		for it := v.ElementIterator(); it.Next(); {
			key, elem := it.Element()
			item, err := fromCty(elem, t.ElementType)
			if err != nil {
				return tftypes.Value{}, err
			}
			out[key.AsString()] = item
		}
		return tftypes.NewValue(t, out), nil
	case tftypes.Object:
		out := map[string]tftypes.Value{}
		for name, at := range t.AttributeTypes {
			if v.Type().IsObjectType() && v.Type().HasAttribute(name) {
				item, err := fromCty(v.GetAttr(name), at)
				if err != nil {
					return tftypes.Value{}, fmt.Errorf("%s: %w", name, err)
				}
				out[name] = item
			} else {
				out[name] = tftypes.NewValue(at, nil)
			}
		}
		return tftypes.NewValue(t, out), nil
	case tftypes.Tuple:
		var out []tftypes.Value
		i := 0
		for it := v.ElementIterator(); it.Next(); i++ {
			_, elem := it.Element()
			item, err := fromCty(elem, t.ElementTypes[i])
			if err != nil {
				return tftypes.Value{}, err
			}
			out = append(out, item)
		}
		return tftypes.NewValue(t, out), nil
	}
	return tftypes.Value{}, fmt.Errorf("the type %s is not supported", typ)
}

func collectionFromCty(v cty.Value, typ tftypes.Type, elem tftypes.Type) (tftypes.Value, error) {
	var out []tftypes.Value
	for it := v.ElementIterator(); it.Next(); {
		_, e := it.Element()
		item, err := fromCty(e, elem)
		if err != nil {
			return tftypes.Value{}, err
		}
		out = append(out, item)
	}
	return tftypes.NewValue(typ, out), nil
}

// tftypesType is the Terraform type of the value of a dynamic attribute.
func tftypesType(t cty.Type) (tftypes.Type, error) {
	switch {
	case t == cty.String:
		return tftypes.String, nil
	case t == cty.Number:
		return tftypes.Number, nil
	case t == cty.Bool:
		return tftypes.Bool, nil
	case t.IsObjectType():
		attrs := map[string]tftypes.Type{}
		for name, at := range t.AttributeTypes() {
			inner, err := tftypesType(at)
			if err != nil {
				return nil, err
			}
			attrs[name] = inner
		}
		return tftypes.Object{AttributeTypes: attrs}, nil
	case t.IsListType() || t.IsTupleType():
		if t.IsListType() {
			inner, err := tftypesType(t.ElementType())
			if err != nil {
				return nil, err
			}
			return tftypes.List{ElementType: inner}, nil
		}
		var elems []tftypes.Type
		for _, et := range t.TupleElementTypes() {
			inner, err := tftypesType(et)
			if err != nil {
				return nil, err
			}
			elems = append(elems, inner)
		}
		return tftypes.Tuple{ElementTypes: elems}, nil
	}
	return nil, fmt.Errorf("the type %s is not supported", t.FriendlyName())
}

func (h *failureHarness) dynamic(typ tftypes.Type, v tftypes.Value) *tfprotov6.DynamicValue {
	h.t.Helper()
	dv, err := tfprotov6.NewDynamicValue(typ, v)
	if err != nil {
		h.t.Fatal(err)
	}
	return &dv
}

func (h *failureHarness) decode(dv *tfprotov6.DynamicValue) tftypes.Value {
	h.t.Helper()
	if dv == nil {
		return tftypes.NewValue(h.typ, nil)
	}
	v, err := dv.Unmarshal(h.typ)
	if err != nil {
		h.t.Fatal(err)
	}
	return v
}

func (h *failureHarness) null() tftypes.Value { return tftypes.NewValue(h.typ, nil) }

// plan returns the planned state of a change from prior to cfg. A prior state that has values keeps
// them for an attribute that the config leaves out, as Terraform proposes it.
func (h *failureHarness) plan(prior, cfg tftypes.Value) tftypes.Value {
	h.t.Helper()
	resp, err := h.server.PlanResourceChange(h.ctx, &tfprotov6.PlanResourceChangeRequest{
		TypeName:         h.spec.TypeName,
		PriorState:       h.dynamic(h.typ, prior),
		ProposedNewState: h.dynamic(h.typ, proposedNew(h.typ, prior, cfg)),
		Config:           h.dynamic(h.typ, cfg),
	})
	if err != nil {
		h.t.Fatal(err)
	}
	if hasError(resp.Diagnostics) {
		h.t.Skipf("the minimal config of %s does not plan offline: %s", h.spec.TypeName, diagText(resp.Diagnostics))
	}
	return h.decode(resp.PlannedState)
}

// proposedNew is the state that Terraform proposes: the config, with the prior value for an
// attribute that the config leaves null.
func proposedNew(typ tftypes.Object, prior, cfg tftypes.Value) tftypes.Value {
	if prior.IsNull() {
		return cfg
	}
	var priorAttrs, cfgAttrs map[string]tftypes.Value
	if prior.As(&priorAttrs) != nil || cfg.As(&cfgAttrs) != nil {
		return cfg
	}
	out := map[string]tftypes.Value{}
	for name := range typ.AttributeTypes {
		if v := cfgAttrs[name]; v.IsNull() && !priorAttrs[name].IsNull() {
			out[name] = priorAttrs[name]
		} else {
			out[name] = v
		}
	}
	return tftypes.NewValue(typ, out)
}

// Made-up ids in the forms that a client checks before it sends a request: a UUID, a number, a text
// of 21 characters (a dashboard id), and a short text. The test takes the first that a read of a
// missing resource accepts.
var (
	stringIDs = []string{"00000000-0000-4000-8000-000000000001", "1", "abcdefghijklmnopqrstu", "failure-paths-id"}
	numberIDs = []int{2, 1000, 1}
)

// priorState is a state that an earlier create could have written: the planned state of the config,
// with a null for each unknown value, and a made-up id.
func (h *failureHarness) priorState() tftypes.Value {
	h.t.Helper()
	planned := h.plan(h.null(), h.config)
	state, err := tftypes.Transform(planned, func(p *tftypes.AttributePath, v tftypes.Value) (tftypes.Value, error) {
		if !v.IsKnown() {
			return tftypes.NewValue(v.Type(), nil), nil
		}
		return v, nil
	})
	if err != nil {
		h.t.Fatal(err)
	}
	attrs := map[string]tftypes.Value{}
	if err := state.As(&attrs); err != nil {
		h.t.Fatal(err)
	}
	at := h.typ.AttributeTypes[h.spec.IDAttr]
	var ids []tftypes.Value
	switch {
	case at == nil || h.spec.Singleton:
		return tftypes.NewValue(h.typ, attrs)
	case at.Is(tftypes.String):
		for _, id := range stringIDs {
			ids = append(ids, tftypes.NewValue(tftypes.String, id))
		}
	case at.Is(tftypes.Number):
		for _, id := range numberIDs {
			ids = append(ids, tftypes.NewValue(tftypes.Number, id))
		}
	default:
		return tftypes.NewValue(h.typ, attrs)
	}
	h.api.set(failGone)
	var first tftypes.Value
	for i, id := range ids {
		attrs[h.spec.IDAttr] = id
		candidate := tftypes.NewValue(h.typ, attrs)
		if i == 0 {
			first = candidate
		}
		if !hasError(h.read(candidate).Diagnostics) {
			return candidate
		}
	}
	return first
}

func (h *failureHarness) apply(prior, planned, cfg tftypes.Value) *tfprotov6.ApplyResourceChangeResponse {
	h.t.Helper()
	resp, err := h.server.ApplyResourceChange(h.ctx, &tfprotov6.ApplyResourceChangeRequest{
		TypeName:     h.spec.TypeName,
		PriorState:   h.dynamic(h.typ, prior),
		PlannedState: h.dynamic(h.typ, planned),
		Config:       h.dynamic(h.typ, cfg),
	})
	if err != nil {
		h.t.Fatal(err)
	}
	return resp
}

func (h *failureHarness) read(state tftypes.Value) *tfprotov6.ReadResourceResponse {
	h.t.Helper()
	resp, err := h.server.ReadResource(h.ctx, &tfprotov6.ReadResourceRequest{TypeName: h.spec.TypeName, CurrentState: h.dynamic(h.typ, state)})
	if err != nil {
		h.t.Fatal(err)
	}
	return resp
}

// requireCall fails when the last operation made no call to the API: the test would then check a
// path that does not run.
func (h *failureHarness) requireCall(t *testing.T, what string) {
	t.Helper()
	if h.api.count() == 0 {
		t.Errorf("%s made no call to the API, so this test checks nothing", what)
	}
}

func hasError(diags []*tfprotov6.Diagnostic) bool {
	for _, d := range diags {
		if d.Severity == tfprotov6.DiagnosticSeverityError {
			return true
		}
	}
	return false
}

func diagText(diags []*tfprotov6.Diagnostic) string {
	var parts []string
	for _, d := range diags {
		parts = append(parts, fmt.Sprintf("%s: %s %s", d.Severity, d.Summary, d.Detail))
	}
	return strings.Join(parts, "; ")
}

func fatalOnError(t *testing.T, diags []*tfprotov6.Diagnostic) {
	t.Helper()
	if hasError(diags) {
		t.Fatalf("unexpected diagnostics: %s", diagText(diags))
	}
}

func requireError(t *testing.T, what string, diags []*tfprotov6.Diagnostic) {
	t.Helper()
	if !hasError(diags) {
		t.Errorf("%s must fail with an error, got: %s", what, diagText(diags))
	}
}

func requireNoError(t *testing.T, what string, diags []*tfprotov6.Diagnostic) {
	t.Helper()
	if hasError(diags) {
		t.Errorf("%s must not fail, got: %s", what, diagText(diags))
	}
}

func attributesOf(v tftypes.Value) map[string]tftypes.Value {
	attrs := map[string]tftypes.Value{}
	_ = v.As(&attrs)
	return attrs
}
