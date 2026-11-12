package failpaths

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// The minimal config of a generated resource is read without a Terraform binary: literals, lists,
// objects, a dynamic attribute, and jsonencode.
func TestConfigFromHCLFitsTheSchemaType(t *testing.T) {
	const src = `resource "coralogix_thing" "test" {
  name = "x"
  limit = 2
  tags = ["a", "b"]
  detail = {
    mode = "fast"
  }
  parameters = {
    Key = "v"
  }
  family = jsonencode({ a = 1 })
}`
	typ := tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"name":       tftypes.String,
		"limit":      tftypes.Number,
		"tags":       tftypes.Set{ElementType: tftypes.String},
		"detail":     tftypes.Object{AttributeTypes: map[string]tftypes.Type{"mode": tftypes.String, "other": tftypes.Bool}},
		"parameters": tftypes.DynamicPseudoType,
		"family":     tftypes.String,
		"left_out":   tftypes.String,
	}}
	cfg, err := configFromHCL(src, "coralogix_thing")
	if err != nil {
		t.Fatal(err)
	}
	got, err := fromCty(cfg, typ)
	if err != nil {
		t.Fatal(err)
	}
	attrs := map[string]tftypes.Value{}
	if err := got.As(&attrs); err != nil {
		t.Fatal(err)
	}
	if !attrs["left_out"].IsNull() {
		t.Error("an attribute that the config leaves out must be null")
	}
	var family string
	if err := attrs["family"].As(&family); err != nil || family != `{"a":1}` {
		t.Errorf("family = %q, %v, want the JSON text", family, err)
	}
	var detail map[string]tftypes.Value
	if err := attrs["detail"].As(&detail); err != nil || !detail["other"].IsNull() {
		t.Errorf("detail = %v, %v, want the attribute that the config leaves out to be null", detail, err)
	}
}

// A reference to another resource is no literal: the test of such a resource skips.
func TestConfigFromHCLRefusesAReference(t *testing.T) {
	if _, err := configFromHCL("resource \"coralogix_thing\" \"test\" {\n  connector_id = coralogix_other.x.id\n}", "coralogix_thing"); err == nil {
		t.Fatal("want an error for a reference")
	}
}
