package generator

import (
	"fmt"
	"maps"
	"slices"

	"github.com/coralogix/coralogix-management-sdk/iac/model"
	"github.com/coralogix/coralogix-management-sdk/iac/sdkcheck"

	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/overrides"
)

// applyReleasedShape changes the built model by the Terraform keys of a released resource that
// the iac model has no field for. requiresReplace makes a top-level field immutable: a change
// replaces the resource, and Update does not send the field. unsupportedArms removes oneOf arms
// from the schema. full are the SDK names of the model before the change; the result adds the
// SDK fields of the removed arms, which flatten reads to report a response that selects one.
func applyReleasedShape(r *model.Resource, file *overrides.File, full []sdkcheck.Ref) ([]sdkcheck.Ref, error) {
	if file == nil {
		return nil, nil
	}
	if err := applyRequiresReplace(r, file); err != nil {
		return nil, err
	}
	before := reachableComponents(r)
	paths, err := removeUnsupportedArms(r, file)
	if err != nil {
		return nil, err
	}
	if err := checkReachableLines(file, before, reachableComponents(r)); err != nil {
		return nil, err
	}
	var refs []sdkcheck.Ref
	for _, ref := range full {
		if ref.Kind == sdkcheck.KindField && slices.Contains(paths, ref.Path) {
			refs = append(refs, ref)
		}
	}
	if len(refs) != len(paths) {
		return nil, fmt.Errorf("unsupportedArms: found the SDK fields of %d of %d arms", len(refs), len(paths))
	}
	return refs, nil
}

// applyRequiresReplace marks the requiresReplace fields immutable. Only a top-level field that
// Create, Update, and Get all have can replace the resource while Update accepts it. A PATCH
// leaves the field out of the Update body, so the field the Update body requires cannot be one:
// the SDK holds it as a value, so the body would send its zero value. A PUT replaces the whole
// resource, so its body keeps the field: without it, an update of another field would clear it.
func applyRequiresReplace(r *model.Resource, file *overrides.File) error {
	for _, component := range slices.Sorted(maps.Keys(file.Types)) {
		for _, name := range slices.Sorted(maps.Keys(file.Types[component].Fields)) {
			if !file.Types[component].Fields[name].RequiresReplace {
				continue
			}
			i := slices.IndexFunc(r.Fields, func(f *model.ResourceField) bool { return f.Name == name })
			if component != r.Name || i < 0 {
				return fmt.Errorf("types.%s.fields.%s: requiresReplace works on a top-level field of %s only", component, name, r.Name)
			}
			f := r.Fields[i]
			if f.Behavior != model.Normal || f.Create == nil || f.Update == nil || f.Get == nil {
				return fmt.Errorf("%s: requiresReplace needs a field that Create, Update, and Get all have", tfName(name))
			}
			if r.Replace {
				f.Behavior = model.Immutable
				continue
			}
			if f.Update.Required {
				return fmt.Errorf("%s: requiresReplace leaves the field out of the Update body, but the Update body requires it", tfName(name))
			}
			f.Behavior, f.Update = model.Immutable, nil
		}
	}
	if !r.Replace && !slices.ContainsFunc(r.Fields, func(f *model.ResourceField) bool {
		return f.Update != nil && !file.Types[r.Name].Fields[f.Name].Fixed()
	}) {
		return fmt.Errorf("requiresReplace leaves no field that Update can change: a resource without an in-place update is not supported")
	}
	return nil
}

// removeUnsupportedArms removes the unsupportedArms of each object from the model, and returns
// the SDK paths of the removed arms. Each listed name must be a oneOf arm, and a oneOf group that
// requires an arm must keep one.
func removeUnsupportedArms(r *model.Resource, file *overrides.File) ([]string, error) {
	found := map[string]bool{}
	var paths []string
	seen := map[*model.Type]bool{}
	var walk func(at string, t *model.Type) error
	walk = func(at string, t *model.Type) error {
		if t == nil || seen[t] {
			return nil
		}
		seen[t] = true
		if t.Elem != nil {
			return walk(at, t.Elem)
		}
		if arms := file.Types[t.Schema].UnsupportedArms; t.Schema != "" && len(arms) != 0 {
			removed, err := dropArms(t, arms)
			if err != nil {
				return fmt.Errorf("types.%s.unsupportedArms: %w", t.Schema, err)
			}
			for _, arm := range removed {
				found[t.Schema+"."+arm] = true
				paths = append(paths, at+"."+arm)
			}
		}
		for _, f := range t.Fields {
			if err := walk(at+"."+f.Name, f.Type); err != nil {
				return err
			}
		}
		return nil
	}
	if arms := file.Types[r.Name].UnsupportedArms; len(arms) != 0 {
		// Flatten checks the arms of a nested object only: the resource's own fields have no
		// object to report from.
		return nil, fmt.Errorf("types.%s.unsupportedArms: the oneOf arms of the resource itself are not supported, only those of a nested object", r.Name)
	}
	for _, f := range r.Fields {
		if err := walk("fields."+f.Name, f.Type); err != nil {
			return nil, err
		}
	}
	for _, component := range slices.Sorted(maps.Keys(file.Types)) {
		for _, arm := range file.Types[component].UnsupportedArms {
			if !found[component+"."+arm] {
				return nil, fmt.Errorf("types.%s.unsupportedArms: %q is not a oneOf arm of a field of the resource", component, arm)
			}
		}
	}
	return paths, nil
}

// dropArms removes the arms from t: an object whose oneOf groups hold them, or a oneOf. It
// returns the removed arms in the order of the fields.
func dropArms(t *model.Type, arms []string) ([]string, error) {
	var groupArms []string
	for _, g := range t.Groups {
		groupArms = append(groupArms, g.Arms...)
	}
	isArm := func(name string) bool {
		if t.Kind == model.OneOf {
			return name != t.Discriminator
		}
		return slices.Contains(groupArms, name)
	}
	for _, arm := range arms {
		if !slices.ContainsFunc(t.Fields, func(f *model.Field) bool { return f.Name == arm }) || !isArm(arm) {
			return nil, fmt.Errorf("%q is not a oneOf arm", arm)
		}
	}
	var removed []string
	t.Fields = slices.DeleteFunc(t.Fields, func(f *model.Field) bool {
		if slices.Contains(arms, f.Name) {
			removed = append(removed, f.Name)
			return true
		}
		return false
	})
	if t.Kind == model.OneOf && !slices.ContainsFunc(t.Fields, func(f *model.Field) bool { return f.Name != t.Discriminator }) {
		return nil, fmt.Errorf("it removes every arm")
	}
	groups := t.Groups[:0]
	for _, g := range t.Groups {
		kept := slices.DeleteFunc(slices.Clone(g.Arms), func(a string) bool { return slices.Contains(arms, a) })
		if len(kept) == 0 && !g.AllowNone {
			return nil, fmt.Errorf("it removes every arm of a oneOf group that requires one")
		}
		if len(kept) != 0 {
			g.Arms = kept
			groups = append(groups, g)
		}
	}
	t.Groups = groups
	return removed, nil
}

// unsupportedArms returns the flatten checks of the removed arms of t, in Go field order.
func (b *convBuilder) unsupportedArms(t *model.Type, at string) ([]unsupportedArm, error) {
	if b.file == nil || t.Schema == "" {
		return nil, nil
	}
	line := b.file.Types[t.Schema]
	var out []unsupportedArm
	for _, arm := range line.UnsupportedArms {
		ref, err := b.ix.fieldRef(at + "." + arm)
		if err != nil {
			return nil, fmt.Errorf("unsupported arm %s: %w", arm, err)
		}
		out = append(out, unsupportedArm{SDK: ref.Name, Summary: line.UnsupportedSummary, Detail: line.UnsupportedDetail})
	}
	slices.SortFunc(out, func(a, b unsupportedArm) int {
		if a.SDK < b.SDK {
			return -1
		}
		if a.SDK > b.SDK {
			return 1
		}
		return 0
	})
	return out, nil
}

// unsupportedArm is the SDK field of a oneOf arm that the resource does not configure, and the
// diagnostic that flatten reports when a response selects it.
type unsupportedArm struct {
	SDK, Summary, Detail string
}

// reachableComponents returns the components that the fields of r reach, the resource included,
// with the wrapper components that an unwrap line collapses.
func reachableComponents(r *model.Resource) map[string]bool {
	out := map[string]bool{r.Name: true}
	seen := map[*model.Type]bool{}
	var walk func(t *model.Type)
	walk = func(t *model.Type) {
		if t == nil || seen[t] {
			return
		}
		seen[t] = true
		names := []string{t.Schema, t.CreateSchema, t.UpdateSchema}
		for _, w := range t.Wrappers {
			names = append(names, w.Schema, w.CreateSchema, w.UpdateSchema)
		}
		for _, name := range names {
			if name != "" {
				out[name] = true
			}
		}
		walk(t.Elem)
		for _, f := range t.Fields {
			walk(f.Type)
		}
	}
	for _, f := range r.Fields {
		walk(f.Type)
	}
	return out
}

// checkReachableLines reports a types or unwrap line whose component only removed unsupported
// arms reached: the line would change nothing. An enums line is not checked: the contract check
// runs before the arms are removed and requires a decision for every value of every enum.
func checkReachableLines(file *overrides.File, before, after map[string]bool) error {
	for _, component := range slices.Sorted(maps.Keys(file.Types)) {
		if before[component] && !after[component] {
			return fmt.Errorf("types.%s: only unsupported arms reach the component, so the line changes nothing", component)
		}
	}
	for _, wrapper := range file.Unwrap {
		if before[wrapper] && !after[wrapper] {
			return fmt.Errorf("unwrap.%s: only unsupported arms reach the wrapper, so the entry changes nothing", wrapper)
		}
	}
	return nil
}
