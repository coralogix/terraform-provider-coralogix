package ipaccess

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestExpandSetAsWriteList(t *testing.T) {
	ctx := context.Background()
	entry, diags := types.ObjectValue(ipAccessAttrTypes(), map[string]attr.Value{
		"enabled":  types.BoolValue(true),
		"ip_range": types.StringValue("10.0.0.0/8"),
		"name":     types.StringValue("office"),
	})
	if diags.HasError() {
		t.Fatalf("create set element: %v", diags)
	}
	entries, diags := types.SetValue(types.ObjectType{AttrTypes: ipAccessAttrTypes()}, []attr.Value{entry})
	if diags.HasError() {
		t.Fatalf("create set: %v", diags)
	}
	model := &CompanyIpAccessSettingsModel{
		EnableCoralogixCustomerSupportAccess: types.StringValue("enabled"),
		Id:                                   types.StringValue("company-123"),
		IpAccess:                             entries,
	}

	create, diags := expandCreate(ctx, model)
	if diags.HasError() {
		t.Fatalf("expand create: %v", diags)
	}
	if len(create.IpAccess) != 1 || create.IpAccess[0].IpRange == nil || *create.IpAccess[0].IpRange != "10.0.0.0/8" {
		t.Fatalf("create ip_access = %#v, want one API list entry", create.IpAccess)
	}

	update, diags := expandUpdate(ctx, model)
	if diags.HasError() {
		t.Fatalf("expand replace: %v", diags)
	}
	if update.Id == nil || *update.Id != "company-123" {
		t.Fatalf("replace id = %v, want company-123", update.Id)
	}
	if len(update.IpAccess) != 1 || update.IpAccess[0].IpRange == nil || *update.IpAccess[0].IpRange != "10.0.0.0/8" {
		t.Fatalf("replace ip_access = %#v, want one API list entry", update.IpAccess)
	}
}
