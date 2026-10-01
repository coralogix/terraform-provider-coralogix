package aaa

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	scopess "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/scopes_service"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const scopeUnitTestID = "scope-123"

// scopeFailingRoundTripper fails before any response exists, so the generated
// client hands back a nil *http.Response alongside the error — the shape a DNS
// failure, refused connection or timeout produces.
type scopeFailingRoundTripper struct{}

func (scopeFailingRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("scope transport failure")
}

// scopeClientForHandler points a generated Scopes client at a local test server.
func scopeClientForHandler(t *testing.T, handler http.HandlerFunc) *scopess.ScopesServiceAPIService {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	cfg := scopess.NewConfiguration()
	cfg.Servers = scopess.ServerConfigurations{{URL: srv.URL}}
	cfg.HTTPClient = srv.Client()
	return scopess.NewAPIClient(cfg).ScopesServiceAPI
}

// scopeClientWithTransportFailure never reaches a server at all.
func scopeClientWithTransportFailure() *scopess.ScopesServiceAPIService {
	cfg := scopess.NewConfiguration()
	cfg.Servers = scopess.ServerConfigurations{{URL: "https://example.com"}}
	cfg.HTTPClient = &http.Client{Transport: scopeFailingRoundTripper{}}
	return scopess.NewAPIClient(cfg).ScopesServiceAPI
}

func scopeRawResponseHandler(status int, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}
}

// scopeErrorText joins the summary and detail of every error diagnostic, so a
// test can assert that the API's own message survived into what the user sees.
func scopeErrorText(errs []diag.Diagnostic) string {
	var b strings.Builder
	for _, d := range errs {
		b.WriteString(d.Summary())
		b.WriteString("\n")
		b.WriteString(d.Detail())
		b.WriteString("\n")
	}
	return b.String()
}

// scopeCapturePanic runs fn and returns the recovered panic value, or nil. Each
// case below asserts a diagnostic rather than a crash, so a nil-dereference has
// to be caught here instead of taking down the whole test binary.
func scopeCapturePanic(fn func()) (recovered any) {
	defer func() { recovered = recover() }()
	fn()
	return nil
}

func scopeResourceSchema(t *testing.T, ctx context.Context) tfsdk.State {
	t.Helper()
	schemaResp := &resource.SchemaResponse{}
	(&ScopeResource{}).Schema(ctx, resource.SchemaRequest{}, schemaResp)
	return tfsdk.State{Schema: schemaResp.Schema}
}

func scopeDataSourceSchema(t *testing.T, ctx context.Context) datasource.SchemaResponse {
	t.Helper()
	schemaResp := datasource.SchemaResponse{}
	(&ScopeDataSource{}).Schema(ctx, datasource.SchemaRequest{}, &schemaResp)
	return schemaResp
}

func scopeStateModel(id string) *ScopeResourceModel {
	return &ScopeResourceModel{
		ID:                types.StringValue(id),
		DisplayName:       types.StringValue("example scope"),
		TeamId:            types.StringValue("1"),
		Description:       types.StringNull(),
		DefaultExpression: types.StringValue("<v1>true"),
		Filters: []ScopeFilterModel{{
			EntityType: types.StringValue("logs"),
			Expression: types.StringValue("<v1>true"),
		}},
	}
}

// readScopeResource drives ScopeResource.Read against the given client with a
// fully-populated prior state.
func readScopeResource(t *testing.T, ctx context.Context, client *scopess.ScopesServiceAPIService, id string) *resource.ReadResponse {
	t.Helper()
	return readScopeResourceWithState(t, ctx, client, scopeStateModel(id))
}

// readScopeResourceWithState drives ScopeResource.Read against an arbitrary
// prior state, and reports whether the resource was removed.
func readScopeResourceWithState(t *testing.T, ctx context.Context, client *scopess.ScopesServiceAPIService, model *ScopeResourceModel) *resource.ReadResponse {
	t.Helper()

	state := scopeResourceSchema(t, ctx)
	if diags := state.Set(ctx, model); diags.HasError() {
		t.Fatalf("failed to seed state: %v", diags)
	}

	resp := &resource.ReadResponse{State: scopeResourceSchema(t, ctx)}
	if diags := resp.State.Set(ctx, model); diags.HasError() {
		t.Fatalf("failed to seed response state: %v", diags)
	}

	r := &ScopeResource{client: client}
	if rec := scopeCapturePanic(func() {
		r.Read(ctx, resource.ReadRequest{State: state}, resp)
	}); rec != nil {
		t.Fatalf("ScopeResource.Read panicked: %v", rec)
	}
	return resp
}

// readScopeDataSource drives ScopeDataSource.Read against the given client.
func readScopeDataSource(t *testing.T, ctx context.Context, client *scopess.ScopesServiceAPIService, id string) *datasource.ReadResponse {
	t.Helper()

	schemaResp := scopeDataSourceSchema(t, ctx)

	// tfsdk.Config has no Set; build the raw value through a State sharing the
	// same schema and hand it over.
	seed := tfsdk.State{Schema: schemaResp.Schema}
	if diags := seed.Set(ctx, scopeStateModel(id)); diags.HasError() {
		t.Fatalf("failed to seed config: %v", diags)
	}
	config := tfsdk.Config{Schema: schemaResp.Schema, Raw: seed.Raw}

	resp := &datasource.ReadResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	d := &ScopeDataSource{client: client}
	if rec := scopeCapturePanic(func() {
		d.Read(ctx, datasource.ReadRequest{Config: config}, resp)
	}); rec != nil {
		t.Fatalf("ScopeDataSource.Read panicked: %v", rec)
	}
	return resp
}

// TestScopeReadDoesNotPanicOnAPIFailure covers every path through the scope
// resource and data source Read methods where the generated client returns no
// usable response model. The client leaves *GetScopesResponse nil on every error
// path, and can also answer 200 with an empty scopes list, so each of these used
// to dereference nil and crash the provider process; the expected behaviour is a
// diagnostic instead, and for the resource, state removal only on a confirmed 404.
func TestScopeReadDoesNotPanicOnAPIFailure(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name string
		// handler is used unless transportFailure is set.
		handler          http.HandlerFunc
		transportFailure bool

		wantResourceWarnings int
		wantResourceError    bool
		wantResourceGone     bool
		wantDataSourceError  bool
		// wantErrorContains, when set, must appear in the error diagnostics of
		// both the resource and the data source.
		wantErrorContains string
	}{
		{
			name:                 "401_unauthorized_surfaces_the_api_message",
			handler:              scopeRawResponseHandler(http.StatusUnauthorized, `{"message":"invalid api key"}`),
			wantResourceWarnings: 0,
			wantResourceError:    true,
			wantResourceGone:     false,
			wantDataSourceError:  true,
			wantErrorContains:    "invalid api key",
		},
		{
			name:                 "403_forbidden_surfaces_the_api_error",
			handler:              scopeRawResponseHandler(http.StatusForbidden, `{"message":"permission denied"}`),
			wantResourceWarnings: 0,
			wantResourceError:    true,
			wantResourceGone:     false,
			wantDataSourceError:  true,
			wantErrorContains:    "permission denied",
		},
		{
			name:                 "404_removes_the_resource_and_errors_the_data_source",
			handler:              scopeRawResponseHandler(http.StatusNotFound, `{"message":"not found"}`),
			wantResourceWarnings: 1,
			wantResourceError:    false,
			wantResourceGone:     true,
			wantDataSourceError:  true,
		},
		{
			// An empty list is not a confirmed deletion — it can also be a
			// permission-filtered lookup — so the resource is kept in state and
			// the ambiguity is reported, rather than silently dropping a scope
			// that may still exist. Only a 404 above removes it.
			name:                 "200_with_an_empty_scopes_list_keeps_the_resource_in_state",
			handler:              scopeRawResponseHandler(http.StatusOK, `{"scopes":[]}`),
			wantResourceWarnings: 0,
			wantResourceError:    true,
			wantResourceGone:     false,
			wantDataSourceError:  true,
		},
		{
			name:                 "500_keeps_the_resource_in_state",
			handler:              scopeRawResponseHandler(http.StatusInternalServerError, `{"message":"boom"}`),
			wantResourceWarnings: 0,
			wantResourceError:    true,
			wantResourceGone:     false,
			wantDataSourceError:  true,
			wantErrorContains:    "boom",
		},
		{
			name:                 "429_too_many_requests",
			handler:              scopeRawResponseHandler(http.StatusTooManyRequests, `{"message":"rate limit exceeded"}`),
			wantResourceWarnings: 0,
			wantResourceError:    true,
			wantResourceGone:     false,
			wantDataSourceError:  true,
			wantErrorContains:    "rate limit exceeded",
		},
		{
			name:                 "transport_failure_has_no_http_response_to_inspect",
			transportFailure:     true,
			wantResourceWarnings: 0,
			wantResourceError:    true,
			wantResourceGone:     false,
			wantDataSourceError:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			newClient := func() *scopess.ScopesServiceAPIService {
				if tc.transportFailure {
					return scopeClientWithTransportFailure()
				}
				return scopeClientForHandler(t, tc.handler)
			}

			resourceResp := readScopeResource(t, ctx, newClient(), scopeUnitTestID)
			if got := resourceResp.Diagnostics.WarningsCount(); got != tc.wantResourceWarnings {
				t.Errorf("resource warnings = %d, want %d (%v)", got, tc.wantResourceWarnings, resourceResp.Diagnostics.Warnings())
			}
			if got := resourceResp.Diagnostics.HasError(); got != tc.wantResourceError {
				t.Errorf("resource hasError = %v, want %v (%v)", got, tc.wantResourceError, resourceResp.Diagnostics.Errors())
			}
			// RemoveResource sets the response state to a null object.
			if got := resourceResp.State.Raw.IsNull(); got != tc.wantResourceGone {
				t.Errorf("resource removed from state = %v, want %v", got, tc.wantResourceGone)
			}
			if tc.wantErrorContains != "" {
				if got := scopeErrorText(resourceResp.Diagnostics.Errors()); !strings.Contains(got, tc.wantErrorContains) {
					t.Errorf("resource error does not carry the API message %q; got:\n%s", tc.wantErrorContains, got)
				}
			}

			dataSourceResp := readScopeDataSource(t, ctx, newClient(), scopeUnitTestID)
			if got := dataSourceResp.Diagnostics.HasError(); got != tc.wantDataSourceError {
				t.Errorf("data source hasError = %v, want %v (%v)", got, tc.wantDataSourceError, dataSourceResp.Diagnostics.Errors())
			}
			if tc.wantErrorContains != "" {
				if got := scopeErrorText(dataSourceResp.Diagnostics.Errors()); !strings.Contains(got, tc.wantErrorContains) {
					t.Errorf("data source error does not carry the API message %q; got:\n%s", tc.wantErrorContains, got)
				}
			}
			// A data source must never remove anything from state.
			if dataSourceResp.State.Raw.IsNull() != true && dataSourceResp.Diagnostics.HasError() {
				t.Errorf("data source wrote state despite an error diagnostic")
			}
		})
	}

	t.Run("empty_id_is_not_looked_up", func(t *testing.T) {
		handler := scopeRawResponseHandler(http.StatusOK, `{"scopes":[]}`)

		resourceResp := readScopeResource(t, ctx, scopeClientForHandler(t, handler), "")
		if resourceResp.Diagnostics.HasError() {
			t.Errorf("resource hasError = true, want false (%v)", resourceResp.Diagnostics.Errors())
		}
		if !resourceResp.State.Raw.IsNull() {
			t.Error("resource with an empty id should be removed from state")
		}

		dataSourceResp := readScopeDataSource(t, ctx, scopeClientForHandler(t, handler), "")
		if !dataSourceResp.Diagnostics.HasError() {
			t.Error("data source with an empty id should produce an error diagnostic")
		}
	})
}

// TestScopeReadFlattensASuccessfulResponse pins the happy path the guards above
// must not disturb: a 200 carrying one scope still lands in state.
func TestScopeReadFlattensASuccessfulResponse(t *testing.T) {
	ctx := context.Background()

	const body = `{"scopes":[{"id":"scope-123","displayName":"example scope","description":"desc",` +
		`"defaultExpression":"<v1>true","teamId":7,` +
		`"filters":[{"entityType":"ENTITY_TYPE_LOGS","expression":"<v1>true"}]}]}`

	assertScope := func(t *testing.T, got ScopeResourceModel) {
		t.Helper()
		if got.ID.ValueString() != scopeUnitTestID {
			t.Errorf("id = %q, want %q", got.ID.ValueString(), scopeUnitTestID)
		}
		if got.DisplayName.ValueString() != "example scope" {
			t.Errorf("display_name = %q, want %q", got.DisplayName.ValueString(), "example scope")
		}
		if got.Description.ValueString() != "desc" {
			t.Errorf("description = %q, want %q", got.Description.ValueString(), "desc")
		}
		if got.DefaultExpression.ValueString() != "<v1>true" {
			t.Errorf("default_expression = %q, want %q", got.DefaultExpression.ValueString(), "<v1>true")
		}
		if got.TeamId.ValueString() != "7" {
			t.Errorf("team_id = %q, want %q", got.TeamId.ValueString(), "7")
		}
		if len(got.Filters) != 1 {
			t.Fatalf("filters = %d, want 1", len(got.Filters))
		}
		if got.Filters[0].EntityType.ValueString() != "logs" {
			t.Errorf("filters[0].entity_type = %q, want %q", got.Filters[0].EntityType.ValueString(), "logs")
		}
		if got.Filters[0].Expression.ValueString() != "<v1>true" {
			t.Errorf("filters[0].expression = %q, want %q", got.Filters[0].Expression.ValueString(), "<v1>true")
		}
	}

	t.Run("resource", func(t *testing.T) {
		resp := readScopeResource(t, ctx, scopeClientForHandler(t, scopeRawResponseHandler(http.StatusOK, body)), scopeUnitTestID)
		if resp.Diagnostics.HasError() {
			t.Fatalf("unexpected error: %v", resp.Diagnostics.Errors())
		}
		var got ScopeResourceModel
		if diags := resp.State.Get(ctx, &got); diags.HasError() {
			t.Fatalf("reading state back: %v", diags)
		}
		assertScope(t, got)
	})

	t.Run("data_source", func(t *testing.T) {
		resp := readScopeDataSource(t, ctx, scopeClientForHandler(t, scopeRawResponseHandler(http.StatusOK, body)), scopeUnitTestID)
		if resp.Diagnostics.HasError() {
			t.Fatalf("unexpected error: %v", resp.Diagnostics.Errors())
		}
		var got ScopeResourceModel
		if diags := resp.State.Get(ctx, &got); diags.HasError() {
			t.Fatalf("reading state back: %v", diags)
		}
		assertScope(t, got)
	})

	// ImportState is a passthrough of the id, so the first Read after an import
	// sees a state holding nothing else. The new id guard must let that through
	// and the read must hydrate every attribute.
	t.Run("resource_import_shape_only_id_in_state", func(t *testing.T) {
		resp := readScopeResourceWithState(t, ctx,
			scopeClientForHandler(t, scopeRawResponseHandler(http.StatusOK, body)),
			&ScopeResourceModel{
				ID:                types.StringValue(scopeUnitTestID),
				DisplayName:       types.StringNull(),
				TeamId:            types.StringNull(),
				Description:       types.StringNull(),
				DefaultExpression: types.StringNull(),
				Filters:           nil,
			})
		if resp.Diagnostics.HasError() {
			t.Fatalf("unexpected error: %v", resp.Diagnostics.Errors())
		}
		if resp.State.Raw.IsNull() {
			t.Fatal("import read removed the resource from state")
		}
		var got ScopeResourceModel
		if diags := resp.State.Get(ctx, &got); diags.HasError() {
			t.Fatalf("reading state back: %v", diags)
		}
		assertScope(t, got)
	})
}
