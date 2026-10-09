package model

import (
	"fmt"
	"slices"

	"github.com/pb33f/libopenapi/datamodel/high/base"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
)

// Policy relaxes the eligibility rules for a resource that users already have
// (existing-resource mode). The zero value is the strict rule set for new
// resources.
//
// A relaxed rule does not hide a problem. The policy names the exact wrapper,
// enum, object, or field that differs from the new-resource contract. The
// generator builds a Policy from the behavior-overrides file, and it fails
// when a line of that file matches nothing.
type Policy struct {
	// Existing turns on the rules that are true for every released resource:
	//   - a response may wrap the resource in one property;
	//   - the request schema may be the resource schema (a shared payload), and
	//     the read-only fields of it are not sent;
	//   - a string pattern is not a generated validator, so it is not an error;
	//   - a nested field may declare a server default.
	Existing bool
	// RequestWrapper is the property of the Create and Update bodies that holds
	// the resource, for example "router". "" means that the body is the resource.
	RequestWrapper string
	// UpdateIDInBody means that Update has no id in its path. The id is a field of
	// the resource in the body, and Update uses the Create path.
	UpdateIDInBody bool
	// ClientSetID means that the client can send the id on Create, and the Get
	// response does not require it.
	ClientSetID bool
	// DeleteOperation is the operationId of the operation that removes the resource
	// when the API has no DELETE. It is a POST on the Get path plus one segment, for
	// example /things/{id}/archive, with the id path parameter of Get, no request
	// body, and a response that the generated code ignores. "" means a DELETE on
	// the Get path.
	DeleteOperation string
	// UpdateMaskInBody: PATCH updateMask is an optional property of the JSON
	// body, not a query parameter. The generated update sets it to the same
	// changed-field mask a query parameter would send. A full replace (PUT)
	// has no mask, so the key is an error there.
	UpdateMaskInBody bool
	// appliedUnsupported records Component.arm keys that splitUnsupported
	// dropped. Create and Update types are discarded after merge, so a walk of
	// the response cannot see a key that names only a request component.
	appliedUnsupported map[string]bool
	// RequiresReplace lists "Component.field" of top-level fields that replace
	// the resource when they change, although Update accepts them.
	RequiresReplace []string
	// Unsupported lists "Component.field" of oneOf arms the resource does not
	// configure. Flatten reports a response that selects one.
	Unsupported []string
	// UnsupportedSummary and UnsupportedDetail are the diagnostic for those
	// arms, keyed by the component name.
	UnsupportedSummary map[string]string
	UnsupportedDetail  map[string]string
	// EnumAnyPrefix lists the enum components whose business values do not use
	// the prefix of the zero value (ENTITY_TYPE_UNSPECIFIED, ALERTS).
	EnumAnyPrefix []string
	// EmptyRequired lists the object components that declare no required list
	// and mean "no field is required".
	EmptyRequired []string
	// Skip lists "Component.field" of fields that the resource does not manage.
	// The generator does not send them and does not store them.
	Skip []string
	// Released lists "Component.field" of fields that keep their released behavior.
	// The behavior-overrides file states it, so the contract need not say whether
	// omission differs from an empty value. A field that is not listed follows the
	// contract, and the contract must state it.
	Released []string
	// Defaults lists "Component.field" of top-level fields whose line replaces the server default
	// of the contract: a static default or computed: false. A field that is not
	// listed uses the server default of the contract, also when it has another line.
	Defaults []string
	// NoInferredValidators means that no limit of the contract becomes a validator.
	NoInferredValidators bool
	// ReadOnly lists "Component.field" of top-level fields that the server sets, although
	// the contract does not mark them readOnly yet. They are in Get only.
	ReadOnly []string
	// Unwrap lists the components with one property that Terraform shows as the value of that
	// property, in every place that has no field line.
	Unwrap []string
	// UnwrapFields maps "Component.field" to the choice of a field line: true shows the value
	// inside the wrapper of the field, or of its items, and false keeps the object.
	UnwrapFields map[string]bool
}

// unwraps reports whether Terraform shows the value inside component, the wrapper that holds
// the value of field of the owner component, or its items.
func (p Policy) unwraps(owner, field, component string) bool {
	if !p.Existing {
		return false
	}
	if on, ok := p.UnwrapFields[owner+"."+field]; ok {
		return on
	}
	return component != "" && slices.Contains(p.Unwrap, component)
}

// requestBody returns the schema that holds the resource in a Create or Update
// body. With a request wrapper it is the wrapper property.
func (p Policy) requestBody(op *v3.Operation) *base.SchemaProxy {
	proxy := bodyProxy(op)
	if p.RequestWrapper == "" || proxy == nil {
		return proxy
	}
	s, err := schemaOf(proxy)
	if err != nil {
		return proxy
	}
	if inner := propertyOf(s, p.RequestWrapper); inner != nil {
		return inner
	}
	return proxy // a later check reports the missing wrapper property
}

// operationIDs returns the explicit operation IDs with the Delete operation of the policy. The
// policy and an explicit ID cannot both name the Delete operation.
func (p Policy) operationIDs(ids OperationIDs) (OperationIDs, error) {
	if p.DeleteOperation == "" {
		return ids, nil
	}
	if ids.Delete != "" {
		return ids, fmt.Errorf("%s: the overrides name %s, and the flag names %s", opDelete, p.DeleteOperation, ids.Delete)
	}
	ids.Delete = p.DeleteOperation
	return ids, nil
}

// methods returns the HTTP methods that the operation of the lifecycle step may use.
func (p Policy) methods(v verb) []string {
	if v == opDelete && p.DeleteOperation != "" {
		return []string{"POST"}
	}
	return verbMethods[v]
}

func (p Policy) skips(component, field string) bool {
	return component != "" && slices.Contains(p.Skip, component+"."+field)
}

func (p Policy) requiresReplace(component, field string) bool {
	return component != "" && slices.Contains(p.RequiresReplace, component+"."+field)
}

func (p Policy) released(component, field string) bool {
	return p.Existing && component != "" && slices.Contains(p.Released, component+"."+field)
}

// OverridesDefault reports whether the behavior-overrides file replaces the default of the field.
// Then the generated resource does not use the server default of the contract.
func (p Policy) OverridesDefault(component, field string) bool {
	return p.Existing && component != "" && slices.Contains(p.Defaults, component+"."+field)
}

// EnumOverride reports whether the file states the Terraform values of the enum.
func (p Policy) EnumOverride(component string) bool { return p.enumAnyPrefix(component) }

// readOnly reports whether the policy states that the server sets the field.
func (p Policy) readOnly(component, field string) bool {
	return component != "" && slices.Contains(p.ReadOnly, component+"."+field)
}

func (p Policy) enumAnyPrefix(component string) bool {
	return component != "" && slices.Contains(p.EnumAnyPrefix, component)
}

func (p Policy) emptyRequired(component string) bool {
	return component != "" && slices.Contains(p.EmptyRequired, component)
}

// pruneSkipped removes the fields that the policy skips from the resource and from its
// nested types. The resource does not manage them: it neither sends nor stores them.
func (r *Resource) pruneSkipped() {
	if len(r.Policy.Skip) == 0 {
		return
	}
	r.Fields = slices.DeleteFunc(r.Fields, func(f *ResourceField) bool { return r.Policy.skips(r.Name, f.Name) })
	seen := map[*Type]bool{}
	for _, f := range r.Fields {
		r.Policy.pruneType(f.Type, seen)
	}
}

func (p Policy) pruneType(t *Type, seen map[*Type]bool) {
	if t == nil || seen[t] {
		return
	}
	seen[t] = true
	if t.Elem != nil {
		p.pruneType(t.Elem, seen)
	}
	if t.Kind != Object && t.Kind != OneOf {
		return
	}
	t.Fields = slices.DeleteFunc(t.Fields, func(f *Field) bool { return p.skips(t.Schema, f.Name) })
	for _, f := range t.Fields {
		p.pruneType(f.Type, seen)
	}
}
