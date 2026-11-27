package generator

import (
	"maps"
	"slices"

	"github.com/coralogix/coralogix-management-sdk/iac/model"

	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/overrides"
)

// A carrySiblings fold shows two twin arms of a oneOf as one Terraform object: the target arm and
// a flag. The twins need not have the same fields. A field that both arms have (Common) is copied
// from one SDK struct to the other. A field that only the target arm has (TargetOnly) is valid
// when the flag selects the target arm. A field that only the folded arm has (FoldedOnly) is
// valid when the flag selects the folded arm. Terraform shows the union of the fields.

// foldSplit are the fields of the two twin arms of a fold, by name.
type foldSplit struct {
	Common     []*model.Field // fields of both arms, in the order of the target arm
	TargetOnly []*model.Field // fields of the target arm only
	FoldedOnly []*model.Field // fields of the folded arm only
}

// splitFold sorts the fields of two twin arms by the arms that have them. A field is common when
// the two arms have a field of that name and the same wire type.
func splitFold(target, folded *model.Type) foldSplit {
	var s foldSplit
	for _, f := range target.Fields {
		if other := typeFieldNamed(folded, f.Name); other != nil && sameWireType(f.Type, other.Type) {
			s.Common = append(s.Common, f)
		} else if other == nil {
			s.TargetOnly = append(s.TargetOnly, f)
		}
	}
	for _, f := range folded.Fields {
		if typeFieldNamed(target, f.Name) == nil {
			s.FoldedOnly = append(s.FoldedOnly, f)
		}
	}
	return s
}

// typeFieldNamed returns the field of t with the API name, or nil.
func typeFieldNamed(t *model.Type, name string) *model.Field {
	if t == nil {
		return nil
	}
	for _, f := range t.Fields {
		if f.Name == name {
			return f
		}
	}
	return nil
}

// foldable reports whether two arms can be twins of a fold: no field has the same name and a
// different type in the two arms, and the arms have a common field, or no field at all. A field
// with two types is a real conflict, and arms without a common field are no twins.
func foldable(a, b *model.Type) bool {
	if a == nil || b == nil {
		return false
	}
	for _, f := range a.Fields {
		if other := typeFieldNamed(b, f.Name); other != nil && !sameWireType(f.Type, other.Type) {
			return false
		}
	}
	return len(splitFold(a, b).Common) != 0 || len(a.Fields)+len(b.Fields) == 0
}

// foldedArms returns the names of the arms that fold into arm, sorted.
func foldedArms(c *overrides.Carry, arm string) []string {
	var out []string
	for _, folded := range slices.Sorted(maps.Keys(c.Fold)) {
		if c.Fold[folded].Into == arm {
			out = append(out, folded)
		}
	}
	return out
}

// armFieldsWithFolds are the fields that the attribute of arm shows: the fields of the arm, and
// the fields of the arms that fold into it. A field that only one of the twins has is optional,
// because the flag may select the twin that lacks it. A folded-only field is a carried field of
// the component of its own arm, so its lines apply.
func (b *tfBuilder) armFieldsWithFolds(carry, arm *model.Field, c *overrides.Carry) []*model.Field {
	fields := slices.Clone(arm.Type.Fields)
	for _, name := range foldedArms(c, arm.Name) {
		from := typeFieldNamed(carry.Type, name)
		split := splitFold(arm.Type, from.Type)
		for i, f := range fields {
			if slices.Contains(split.TargetOnly, f) {
				fields[i] = optionalField(f)
			}
		}
		for _, f := range split.FoldedOnly {
			copied := *optionalField(f)
			b.carried[&copied] = from.Type.Schema
			fields = append(fields, &copied)
		}
	}
	return fields
}

// optionalField returns f as an optional field. A field that one twin only has cannot be required:
// the flag may select the other twin.
func optionalField(f *model.Field) *model.Field {
	if !f.Attrs.Required && (f.Create == nil || !f.Create.Required) && (f.Update == nil || !f.Update.Required) {
		return f
	}
	c := *f
	c.Attrs.Required = false
	if f.Create != nil {
		create := *f.Create
		create.Required = false
		c.Create = &create
	}
	if f.Update != nil {
		update := *f.Update
		update.Required = false
		c.Update = &update
	}
	return &c
}
