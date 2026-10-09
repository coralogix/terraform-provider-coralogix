package generator

import (
	"errors"
	"fmt"
	"strings"

	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/model"
)

// errUnwrapCombination reports a field line that cannot work with a wrapped value yet: the prior
// order and the document comparison read the SDK field without its wrappers.
var errUnwrapCombination = errors.New("keepPriorOrder and equality are not supported on a field whose value is inside a wrapper")

// wrappedConv sets the conversion of a value that the API holds in the wrappers of t: the
// conversion of the value, and the helper that wraps it. It returns the SDK Go type of the field,
// the outer wrapper.
func (b *convBuilder) wrappedConv(cf *convField, t *model.Type, computed bool) (string, error) {
	inner, err := b.fieldConv(cf, t.Unwrapped(), computed)
	if err != nil {
		return "", err
	}
	outer, helper, err := b.wrapHelperFor(t.Wrappers, inner)
	if err != nil {
		return "", err
	}
	if cf.Wrap == nil {
		cf.Wrap = &fieldWrap{Inner: b.qualifyType(inner)}
	}
	// The value can be a list of wrapped items: its helper runs inside this one.
	cf.Wrap.Funcs = append([]string{helper.Func}, cf.Wrap.Funcs...)
	return "*" + outer, nil
}

// wrappedItemsConv sets the conversion of a list or set whose items the API holds in wrappers. It
// converts the items as a list of the values, then wraps each item.
func (b *convBuilder) wrappedItemsConv(cf *convField, t *model.Type) (string, error) {
	plain := *t
	plain.Elem = t.Elem.Unwrapped()
	inner, err := b.collectionConv(cf, &plain)
	if err != nil {
		return "", err
	}
	value := strings.TrimPrefix(inner, "[]")
	outer, item, err := b.wrapHelperFor(t.Elem.Wrappers, "*"+value)
	if err != nil {
		return "", err
	}
	helper, err := b.addWrapHelper(&wrapHelper{
		Func: item.Func + "Items", Outer: "[]" + b.qualify(outer), Inner: b.qualifyType(inner),
		Item: item.Func, ItemValue: b.qualifyType(value),
	})
	if err != nil {
		return "", err
	}
	cf.Wrap = &fieldWrap{Inner: b.qualifyType(inner), Funcs: []string{helper.Func}}
	return "[]" + outer, nil
}

// wrapStep is one wrapper object of a value.
type wrapStep struct {
	Type  string // qualified SDK type of the wrapper
	Field string // Go field that holds the value
	Value bool   // the field is a value, not a pointer
}

// wrapHelperFor returns the SDK type of the outer wrapper and the helper that puts a value of the
// Go type inner, as the field conversion writes it, in the wrappers. Each wrapper field must have
// the type of the next wrapper, and the last one the type of the value, or its value type.
func (b *convBuilder) wrapHelperFor(wrappers []model.Wrapper, inner string) (string, *wrapHelper, error) {
	steps := make([]wrapStep, 0, len(wrappers))
	names := make([]string, 0, len(wrappers))
	var outer string
	for i, w := range wrappers {
		typ, err := b.ix.schemaRef(w.Schema)
		if err != nil {
			return "", nil, err
		}
		if i == 0 {
			outer = typ.Name
		}
		field, err := b.ix.fieldRef(typ.Path + "." + w.Field)
		if err != nil {
			return "", nil, err
		}
		want := inner
		if i+1 < len(wrappers) {
			next, err := b.ix.schemaRef(wrappers[i+1].Schema)
			if err != nil {
				return "", nil, err
			}
			want = "*" + next.Name
		}
		step := wrapStep{Type: b.qualify(typ.Name), Field: field.Name}
		switch {
		case field.Want == want:
		case strings.HasPrefix(want, "*") && field.Want == want[1:]:
			step.Value = true
		default:
			return "", nil, fmt.Errorf("SDK field %s has type %s, the wrapped value needs %s", field.sdkName(), field.Want, want)
		}
		steps = append(steps, step)
		names = append(names, camelize(w.Schema))
	}
	helper, err := b.addWrapHelper(&wrapHelper{
		Func: strings.Join(names, ""), Outer: "*" + b.qualify(outer), Inner: b.qualifyType(inner),
		Build: wrapBuild(steps), Read: wrapRead(steps, strings.HasPrefix(inner, "*")),
		NilCheck: wrapNilCheck(steps),
	})
	return outer, helper, err
}

// addWrapHelper keeps one helper per name. Two places with the same wrappers share it, and the
// value inside them has one Go type.
func (b *convBuilder) addWrapHelper(h *wrapHelper) (*wrapHelper, error) {
	for _, existing := range b.wraps {
		if existing.Func != h.Func {
			continue
		}
		if existing.Inner != h.Inner {
			return nil, fmt.Errorf("the wrappers %s hold %s in one place and %s in another", h.Func, existing.Inner, h.Inner)
		}
		return existing, nil
	}
	b.wraps = append(b.wraps, h)
	return h, nil
}

// wrapBuild returns the Go expression that puts the non-nil value v in the wrappers.
func wrapBuild(steps []wrapStep) string {
	expr := "v"
	for i := len(steps) - 1; i >= 0; i-- {
		s := steps[i]
		value := expr
		if s.Value {
			// A field that is a value takes the struct, not a pointer to it.
			value = "*v"
			if i < len(steps)-1 {
				value = strings.TrimPrefix(expr, "&")
			}
		}
		expr = "&" + s.Type + "{" + s.Field + ": " + value + "}"
	}
	return expr
}

// wrapNilCheck returns the condition under which a wrapped value w has no value.
func wrapNilCheck(steps []wrapStep) string {
	checks := []string{"w == nil"}
	at := "w"
	for _, s := range steps[:len(steps)-1] {
		at += "." + s.Field
		if !s.Value {
			checks = append(checks, at+" == nil")
		}
	}
	return strings.Join(checks, " || ")
}

// wrapRead returns the Go expression of the value inside the wrapped value w. pointer is true when
// the value is a pointer, so a value field needs its address.
func wrapRead(steps []wrapStep, pointer bool) string {
	at := "w"
	for _, s := range steps {
		at += "." + s.Field
	}
	if pointer && steps[len(steps)-1].Value {
		return "&" + at
	}
	return at
}

// goBuiltins are the Go types that an SDK field type names without the SDK package.
var goBuiltins = map[string]bool{
	"string": true, "bool": true, "int32": true, "int64": true, "float32": true, "float64": true, "interface{}": true,
}

// qualifyType writes an SDK Go type, as the field conversions return it, with the SDK package
// name: "*Thing" → "*things_service.Thing", "[]string" stays.
func (b *convBuilder) qualifyType(goType string) string {
	var prefix string
	rest := goType
	for {
		switch {
		case strings.HasPrefix(rest, "*"):
			prefix, rest = prefix+"*", rest[1:]
		case strings.HasPrefix(rest, "[]"):
			prefix, rest = prefix+"[]", rest[2:]
		case strings.HasPrefix(rest, "map[string]"):
			prefix, rest = prefix+"map[string]", rest[len("map[string]"):]
		case goBuiltins[rest] || strings.Contains(rest, "."):
			return prefix + rest
		default:
			return prefix + b.qualify(rest)
		}
	}
}
