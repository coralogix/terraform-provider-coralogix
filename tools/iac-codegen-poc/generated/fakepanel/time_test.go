// This file is handwritten. It tests the generated timestamp attributes
// (F51) through the host resource of host_test.go.

package fakepanel_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	sdk "github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen-poc/fakesdk/go/openapi/gen/fake_boards_service"
	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen-poc/generated/fakepanel"
)

// TestTimeRoundTrip checks that an API time reads back in UTC, in the form
// that the validator accepts, and expands to the same instant.
func TestTimeRoundTrip(t *testing.T) {
	ctx := context.Background()
	from := time.Date(2026, 9, 26, 10, 0, 0, 500_000_000, time.FixedZone("CEST", 2*3600))
	m, diags := fakepanel.FlattenAbsoluteTime(ctx, path.Root("range"), &sdk.AbsoluteTime{From: &from})
	if diags.HasError() {
		t.Fatal(diags)
	}
	if got := m.From.ValueString(); got != "2026-09-26T08:00:00.5Z" {
		t.Errorf("from = %q, want the UTC form 2026-09-26T08:00:00.5Z", got)
	}
	if !m.To.IsNull() {
		t.Errorf("to = %s, want null", m.To)
	}
	back, diags := fakepanel.ExpandAbsoluteTime(ctx, path.Root("range"), m)
	if diags.HasError() {
		t.Fatal(diags)
	}
	if back.From == nil || !back.From.Equal(from) || back.To != nil {
		t.Errorf("expand = %v, %v; want %v and nil", back.From, back.To, from)
	}
}

// TestTimeValidator runs the timestamp validator through a provider server.
func TestTimeValidator(t *testing.T) {
	for _, c := range []struct{ value, want string }{
		{"2026-09-26T08:00:00Z", ""},
		{"2026-09-26T08:00:00.5Z", ""},
		{"2026-09-26T10:00:00+02:00", `as "2026-09-26T08:00:00Z"`},
		{"2026-09-26T08:00:00.500Z", `as "2026-09-26T08:00:00.5Z"`},
		{"yesterday", "is not an RFC 3339 timestamp"},
	} {
		t.Run(c.value, func(t *testing.T) {
			m := validModel(t)
			m.Range = &fakepanel.AbsoluteTimeModel{From: types.StringValue(c.value), To: types.StringNull()}
			got := diagText(validate(t, m))
			t.Logf("errors: %s", got)
			switch {
			case c.want == "" && got != "":
				t.Errorf("errors: %s", got)
			case c.want != "" && (!strings.Contains(got, c.want) || !strings.Contains(got, `AttributeName("from")`)):
				t.Errorf("errors = %q, want %q at range.from", got, c.want)
			}
		})
	}
}

// rangeAttribute is the host attribute that holds the generated AbsoluteTime.
func rangeAttribute() schema.Attribute {
	return schema.SingleNestedAttribute{Optional: true, Attributes: fakepanel.AbsoluteTimeAttributes()}
}
