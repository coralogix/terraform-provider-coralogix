package generator

import (
	"fmt"
	"slices"
	"strings"

	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/overrides"
)

// uniqueCheck says that the items of a top-level list or set of objects must differ in one
// attribute (the uniqueBy key).
type uniqueCheck struct {
	Attr string // Terraform name of the collection
	Key  string // Terraform name of the attribute of an item
}

// enumAlias is the plan rewrite of one string attribute: each From value becomes its To value.
type enumAlias struct {
	Path  string // dotted Terraform path, with the steps of collections left out
	Pairs []aliasPair
}

type aliasPair struct{ From, To string }

// applyConfigChecks collects what the generated resource checks or rewrites with generic keys:
// uniqueBy, the float32 precision of the attributes that the contract types as float (a limit of
// the contract, so validators.inferred: false leaves it out), and the aliases of an enum.
func applyConfigChecks(out *tfResource, file *overrides.File) error {
	used := map[string]bool{}
	for _, a := range out.Attributes {
		line := file.Types[a.Component].Fields[a.Property]
		if line.UniqueBy == "" {
			continue
		}
		check, err := uniqueCheckOf(a, line.UniqueBy)
		if err != nil {
			return fmt.Errorf("%s.%s: uniqueBy: %w", a.Component, a.Property, err)
		}
		used[a.Component+"."+a.Property] = true
		out.UniqueChecks = append(out.UniqueChecks, check)
	}
	for component, t := range file.Types {
		for field, line := range t.Fields {
			if line.UniqueBy != "" && !used[component+"."+field] {
				return fmt.Errorf("types.%s.fields.%s: uniqueBy works on a top-level list or set of objects only", component, field)
			}
		}
	}
	if file.Validators.Inferred != nil && *file.Validators.Inferred {
		out.Float32Paths = float32Paths(nil, "", out.Attributes)
	}
	aliases, err := enumAliases(nil, "", out.Attributes, file)
	if err != nil {
		return err
	}
	out.EnumAliases = aliases
	return nil
}

func uniqueCheckOf(a *tfAttr, property string) (uniqueCheck, error) {
	if (a.ValueKind != "List" && a.ValueKind != "Set") || len(a.Attributes) == 0 {
		return uniqueCheck{}, fmt.Errorf("%s is no list or set of objects", a.Name)
	}
	if computedOnly(a) {
		return uniqueCheck{}, fmt.Errorf("%s is computed-only, so the configuration never sets it", a.Name)
	}
	for _, item := range a.Attributes {
		if item.Property != property {
			continue
		}
		if item.ValueKind != "String" || item.CustomType != "" {
			return uniqueCheck{}, fmt.Errorf("%s of %s is no plain string", item.Name, a.Name)
		}
		return uniqueCheck{Attr: a.Name, Key: item.Name}, nil
	}
	return uniqueCheck{}, fmt.Errorf("the items of %s have no property %q", a.Name, property)
}

func float32Paths(out []string, parent string, attrs []*tfAttr) []string {
	for _, a := range attrs {
		if computedOnly(a) {
			continue
		}
		at := a.Name
		if parent != "" {
			at = parent + "." + a.Name
		}
		if a.ValueKind == "Float32" {
			out = append(out, at)
		}
		out = float32Paths(out, at, a.Attributes)
	}
	return out
}

func enumAliases(out []enumAlias, parent string, attrs []*tfAttr, file *overrides.File) ([]enumAlias, error) {
	for _, a := range attrs {
		at := a.Name
		if parent != "" {
			at = parent + "." + a.Name
		}
		if enum, ok := file.Enums[a.EnumSchema]; ok && a.EnumSchema != "" && len(enum.Aliases) != 0 && !computedOnly(a) {
			if a.ValueKind != "String" {
				return nil, fmt.Errorf("%s: enums.%s.aliases works on a string attribute, not on a list or set of an enum", at, a.EnumSchema)
			}
			alias := enumAlias{Path: at}
			for from, to := range enum.Aliases {
				alias.Pairs = append(alias.Pairs, aliasPair{From: from, To: to})
			}
			slices.SortFunc(alias.Pairs, func(x, y aliasPair) int { return strings.Compare(x.From, y.From) })
			out = append(out, alias)
		}
		var err error
		if out, err = enumAliases(out, at, a.Attributes, file); err != nil {
			return nil, err
		}
	}
	return out, nil
}
