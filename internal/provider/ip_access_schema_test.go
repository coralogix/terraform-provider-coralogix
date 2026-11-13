package provider

import (
	"context"
	"testing"

	ipaccesssdk "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/ip_access_service"
	"github.com/coralogix/terraform-provider-coralogix/internal/provider/generated/ipaccess"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

func TestIpAccessGeneratedSchemaCompatibility(t *testing.T) {
	got := ipaccess.Schema()

	support, ok := got.Attributes["enable_coralogix_customer_support_access"].(schema.StringAttribute)
	if !ok || !support.Optional || !support.Computed || support.Default != nil || len(support.Validators) == 0 {
		t.Fatalf("customer support access schema = %#v; want optional+computed without a static default and with enum validation", got.Attributes["enable_coralogix_customer_support_access"])
	}

	entries, ok := got.Attributes["ip_access"].(schema.SetNestedAttribute)
	if !ok || !entries.Optional || !entries.Computed {
		t.Fatalf("ip_access schema = %#v; want an optional+computed set", got.Attributes["ip_access"])
	}
	if !entries.NestedObject.Attributes["enabled"].(schema.BoolAttribute).Required || !entries.NestedObject.Attributes["ip_range"].(schema.StringAttribute).Required {
		t.Fatal("ip_access.enabled and ip_access.ip_range must remain required")
	}
	name := entries.NestedObject.Attributes["name"].(schema.StringAttribute)
	if !name.Optional || !name.Computed {
		t.Fatal("ip_access.name must remain optional+computed to track the API's empty-string default")
	}

	id, ok := got.Attributes["id"].(schema.StringAttribute)
	if !ok || !id.Computed {
		t.Fatalf("id schema = %#v; want computed string", got.Attributes["id"])
	}
}

func TestIpAccessFlattenUsesMapValuesAsSet(t *testing.T) {
	ctx := context.Background()
	id := "company-123"
	enabled, disabled := true, false
	ipOne, ipTwo := "10.0.0.0/8", "192.168.0.0/16"
	nameOne, nameTwo := "office", "vpn"
	access := ipaccesssdk.CORALOGIXCUSTOMERSUPPORTACCESS_CORALOGIX_CUSTOMER_SUPPORT_ACCESS_ENABLED
	got, diags := ipaccess.Flatten(ctx, &ipaccesssdk.CompanyIpAccessSettings{
		Id:                                   &id,
		EnableCoralogixCustomerSupportAccess: &access,
		IpAccess: map[string]ipaccesssdk.IpAccess{
			"server-key-one": {Enabled: &enabled, IpRange: &ipOne, Name: &nameOne},
			"server-key-two": {Enabled: &disabled, IpRange: &ipTwo, Name: &nameTwo},
		},
	})
	if diags.HasError() {
		t.Fatalf("Flatten diagnostics: %v", diags)
	}
	var rules []ipaccess.IpAccessModel
	if d := got.IpAccess.ElementsAs(ctx, &rules, false); d.HasError() {
		t.Fatalf("decode flattened set: %v", d)
	}
	if len(rules) != 2 {
		t.Fatalf("flattened set has %d entries, want 2", len(rules))
	}
	seen := map[string]bool{}
	for _, rule := range rules {
		seen[rule.IpRange.ValueString()] = true
	}
	if !seen[ipOne] || !seen[ipTwo] {
		t.Fatalf("map values were not preserved in the set: %#v", seen)
	}
}
