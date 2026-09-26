package schemadump

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// PlanWithUnknowns returns the state value v of schema s with every computed
// attribute unknown. Terraform plans a computed attribute with no
// configuration value as unknown, for example on an update, so a model must
// read such a plan: a struct pointer cannot hold an unknown object. This is
// the worst case: every computed attribute at once.
func PlanWithUnknowns(ctx context.Context, s schema.Schema, v tftypes.Value) (tftypes.Value, error) {
	return tftypes.Transform(v, func(p *tftypes.AttributePath, v tftypes.Value) (tftypes.Value, error) {
		if len(p.Steps()) == 0 {
			return v, nil
		}
		a, err := s.AttributeAtTerraformPath(ctx, p)
		if err != nil || !a.IsComputed() {
			return v, nil // a list element or a set value, not an attribute
		}
		return tftypes.NewValue(v.Type(), tftypes.UnknownValue), nil
	})
}
