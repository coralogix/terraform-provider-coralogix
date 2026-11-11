package generator

import (
	"errors"
	"fmt"
	"strings"

	"github.com/coralogix/coralogix-management-sdk/iac/model"
	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/overrides"
)

// convDistribution is a top-level list that a distributeTo line moves into the items of a component.
// A request or a response carries the items of every component in one collector, which the root
// functions install in the context. The component adds the items of its own attribute to it on
// expand, and takes the items that name it out of it on flatten.
type convDistribution struct {
	Name    string      // Go name of the list: the collector is distributed<Name>
	SDKType string      // qualified SDK type of one list item
	Object  *convObject // the conversion of the list item
	// List is the field of the list item that holds the id of the component. It is no attribute: the
	// generated code writes it on expand and reads it on flatten.
	List *convField
	// Keep is the Go condition that a list item (the variable a) must meet to be shown: the
	// values on the path of the keep line are set. It is "" when the line has no path.
	Keep string
}

// convRootList is the list of a root, and the Go field of the root struct that holds it.
type convRootList struct {
	Dist *convDistribution
	SDK  string
}

// ReadOf returns the Go expression of the *string that the string field f holds in the SDK struct
// recv, with the wrappers of the field taken off.
func (f *convField) ReadOf(recv string) string {
	src := recv + "." + f.SDK
	if f.Value {
		src = "&" + src
	}
	if f.Wrap != nil {
		return f.Wrap.UnwrapOf(src)
	}
	return src
}

// WriteOf returns the Go expression of the SDK value of the string field f for the *string inner,
// with the wrappers of the field put on.
func (f *convField) WriteOf(inner string) string {
	expr := inner
	if f.Wrap != nil {
		expr = f.Wrap.ExpandOf(inner)
	}
	if f.Value {
		return "valueOf(" + expr + ")"
	}
	return expr
}

// ListRead is the Go expression of the id that a list item a names its component by.
func (d *convDistribution) ListRead() string { return d.List.ReadOf("a") }

// rootLists builds the lists that distributeTo lines moved out of the resource, for one root. A
// root that holds none of them has nothing to build.
func (b *convBuilder) rootLists(obj *convObject, root convRoot, fieldsPath string) error {
	if b.file == nil {
		return nil
	}
	for _, d := range b.file.Distributed {
		if !root.has(d.Field) {
			continue
		}
		cf, err := b.rootField(obj, root, fieldsPath, d.Field, rootType(root.path, d.Field))
		if err != nil {
			return err
		}
		if cf.Conv != convObjects || cf.Collection != "List" {
			return fmt.Errorf("%s.%s: distributeTo needs a list of objects", root.path, d.Field.Name)
		}
		dist, err := b.distributionOf(d, cf)
		if err != nil {
			return fmt.Errorf("%s.%s: %w", root.path, d.Field.Name, err)
		}
		obj.Lists = append(obj.Lists, convRootList{Dist: dist, SDK: cf.SDK})
	}
	return nil
}

// distributionOf returns the distribution of the list, which the first root that holds the list builds.
func (b *convBuilder) distributionOf(d *overrides.Distribution, cf *convField) (*convDistribution, error) {
	if dist, ok := b.dists[d.Field.Name]; ok {
		return dist, nil
	}
	action := d.Field.Type.Elem
	ref, err := b.ix.schemaRef(action.Schema)
	if err != nil {
		return nil, err
	}
	hidden, err := b.fieldAt(ref.Path+"."+d.Hidden.Name, action.Schema, d.Hidden.Name, d.Hidden.Type, false, d.Hidden.Attrs.Presence)
	if err != nil {
		return nil, fmt.Errorf("by.listField: %w", err)
	}
	if hidden.Conv != convString {
		return nil, fmt.Errorf("by.listField: %s is a %s, and distributeTo needs a plain string", d.Hidden.Name, hidden.Conv)
	}
	keep, err := b.keepCondition(action, d.Line.Keep)
	if err != nil {
		return nil, err
	}
	dist := &convDistribution{Name: camelize(d.Field.Name), SDKType: cf.SDKType, Object: cf.Object, List: hidden, Keep: keep}
	b.dists[d.Field.Name] = dist
	return dist, nil
}

// keepCondition returns the Go condition that the list item a has a value at each step of the keep
// path. A step that the SDK holds as a value needs no check, but the last step must be a pointer:
// only a pointer can be unset.
func (b *convBuilder) keepCondition(action *model.Type, keep string) (string, error) {
	steps := keepSteps(keep)
	var checks []string
	expr, t := "a", action
	for i, step := range steps {
		if t == nil || (t.Kind != model.Object && t.Kind != model.OneOf) {
			return "", fmt.Errorf("keep: %q goes through a value that has no fields", keep)
		}
		i1 := fieldIndex(t, step)
		if i1 < 0 {
			return "", fmt.Errorf("keep: %s has no field %s", t.Schema, step)
		}
		f := t.Fields[i1]
		if len(f.Type.Wrappers) != 0 {
			return "", fmt.Errorf("keep: %s of %s is in wrapper objects, and the path cannot go through them", step, t.Schema)
		}
		ref, err := b.ix.schemaRef(t.Schema)
		if err != nil {
			return "", err
		}
		field, err := b.ix.fieldRef(ref.Path + "." + step)
		if err != nil {
			return "", err
		}
		expr += "." + field.Name
		switch {
		case strings.HasPrefix(field.Want, "*"):
			checks = append(checks, expr+" != nil")
		case i == len(steps)-1:
			return "", fmt.Errorf("keep: %s of %s is set in every item, so the path says nothing", step, t.Schema)
		}
		t = f.Type
	}
	return strings.Join(checks, " && "), nil
}

// fieldIndex returns the index of the field of t with the API name, or -1.
func fieldIndex(t *model.Type, name string) int {
	for i, f := range t.Fields {
		if f.Name == name {
			return i
		}
	}
	return -1
}

// distribute gives the item of each distributeTo line its attribute. The item is a component that
// the resource holds, so its object already exists. A request can hold the component under another
// name, so every object that converts the model of the item gets the attribute: the request objects
// collect the list items, and the response object reads them. The attribute goes last, after the
// id, so expand knows the id when it reads it.
func (b *convBuilder) distribute(out *convData) error {
	if b.file == nil {
		return nil
	}
	for _, d := range b.file.Distributed {
		dist, first := b.dists[d.Field.Name], b.bySchema[d.Line.Item]
		if dist == nil || first == nil {
			return fmt.Errorf("%s: distributeTo found no conversion of the list or of the item %s", d.Field.Name, d.Line.Item)
		}
		for _, item := range b.objects {
			if item.Model != first.Model {
				continue
			}
			if err := b.distributeInto(d, dist, item); err != nil {
				return fmt.Errorf("%s: %w", d.Field.Name, err)
			}
		}
		out.Distributions = append(out.Distributions, dist)
	}
	return nil
}

// distributeInto adds the attribute of the distribution to one object of the item.
func (b *convBuilder) distributeInto(d *overrides.Distribution, dist *convDistribution, item *convObject) error {
	if item.Typed != nil || item.Carry != nil || item.Group {
		return fmt.Errorf("distributeTo needs an item that is a plain object, but %s is not", d.Line.Item)
	}
	id := fieldByModel(item.Fields, camelize(d.Line.By.ItemField))
	if id == nil || id.Conv != convString || (item.Expand && id.FlattenOnly) {
		return fmt.Errorf("by.itemField %s must be a string that a request sends in %s", d.Line.By.ItemField, d.Line.Item)
	}
	item.Fields = append(item.Fields, &convField{
		TFName: d.Line.As, Model: camelize(d.Line.As), Conv: convDistributed, Collection: "List",
		Object: dist.Object, SDKType: dist.SDKType, Dist: dist, ItemID: id,
	})
	for _, expand := range []bool{true, false} {
		if (expand && item.Expand) || (!expand && item.Flatten) {
			if err := b.mark(dist.Object, expand); err != nil {
				return err
			}
		}
	}
	return nil
}

// checkDistributions rejects a moved list whose items need the prior model: the flatten of an item
// that a distribution builds has no prior item to pass.
func checkDistributions(d *convData) error {
	for _, dist := range d.Distributions {
		if dist.Object.NeedsPrior {
			return errors.New("distributeTo does not support equality, keepPriorOrder, or readEmptyAs on a string in the items that it moves")
		}
	}
	return nil
}
