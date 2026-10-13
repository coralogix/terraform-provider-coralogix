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
	// the resource in the body. Update uses the Create path, unless CustomMethods
	// names a different path.
	UpdateIDInBody bool
	// CustomMethods means Get, Update, and Delete are not the Create path with an id.
	// Get and Delete each carry the same {id} path parameter. Update has no path
	// parameter. New resources leave this false.
	CustomMethods bool
	// ClientSetID means that the client can send the id on Create, and the Get
	// response does not require it.
	ClientSetID bool
	// ServerSetID means that the server sets the id and returns it in every response, and
	// the Get response does not require it: the request shares the resource schema.
	ServerSetID bool
	// Operations names the lifecycle operationIds from behavior-overrides. Flags
	// still override an empty slot. A flag and the file cannot name different ids.
	Operations OperationIDs
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
	// EnumAnyPrefix lists the enum components whose business values do not use
	// the prefix of the zero value (ENTITY_TYPE_UNSPECIFIED, ALERTS).
	EnumAnyPrefix []string
	// EmptyRequired lists the object components that declare no required list
	// and mean "no field is required".
	EmptyRequired []string
	// Optional lists "Component.field" of fields that a request of the contract requires and the
	// released resource treats as optional.
	Optional []string
	// Computed lists "Component.field" of fields that a computed: true line makes computed.
	Computed []string
	// Sets lists "Component.field" of ordered lists of the contract that the resource shows as sets.
	Sets []string
	// Names maps "Component.field" to the Terraform attribute name of the field.
	Names map[string]string
	// KeyedBy maps "Component.field" of a list of objects to the keyedBy line of the field.
	KeyedBy map[string]KeyedLine
	// EnumValues are the accepted values of each enum that has an enums line, with their
	// Terraform values, in the order of the line.
	EnumValues map[string][]EnumValue
	// CreateAs maps "Component.field" of a top-level object field to the flat Create fields that
	// hold its properties: property name to Create field name.
	CreateAs map[string]map[string]string
	// Skip lists "Component.field" of fields that the resource does not manage.
	// The generator does not send them and does not store them.
	Skip []string
	// FixedRequest lists "Component.field" that Create and Update always send as a
	// fixed API value. A nested read-only field in this list is sent, so it is not
	// a server-only field the renderer has to drop.
	FixedRequest []string
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
	// RejectUnknown: an existing resource stops before the API call when a request value is
	// unknown, as a new resource does.
	RejectUnknown bool
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
		return referencedSchema(proxy)
	}
	s, err := schemaOf(proxy)
	if err != nil {
		return proxy
	}
	if inner := propertyOf(s, p.RequestWrapper); inner != nil {
		return referencedSchema(inner)
	}
	return proxy // a later check reports the missing wrapper property
}

// referencedSchema returns the schema that allOf: [$ref] refers to, or proxy itself. The OpenAPI
// fork writes a reference with a description in that form. Validation and Build both read the
// request body through requestBody, so both see the fields of the referenced schema.
func referencedSchema(proxy *base.SchemaProxy) *base.SchemaProxy {
	seen := map[*base.SchemaProxy]bool{}
	for proxy != nil && !seen[proxy] {
		seen[proxy] = true
		s, err := schemaOf(proxy)
		if err != nil {
			return proxy
		}
		inner, err := singleAllOf(s, "request body")
		if err != nil || !inner.IsReference() {
			return proxy
		}
		proxy = inner
	}
	return proxy
}

// operationIDs returns the lifecycle operation IDs. A flag fills a slot the file
// leaves empty. The file and a flag cannot name different ids for one step.
// api.delete.operation still names a POST delete when the file has no operations.delete.
func (p Policy) operationIDs(ids OperationIDs) (OperationIDs, error) {
	var err error
	ids.Create, err = oneOperationID(opCreate, p.Operations.Create, ids.Create)
	if err != nil {
		return ids, err
	}
	ids.Get, err = oneOperationID(opGet, p.Operations.Get, ids.Get)
	if err != nil {
		return ids, err
	}
	ids.Update, err = oneOperationID(opUpdate, p.Operations.Update, ids.Update)
	if err != nil {
		return ids, err
	}
	fromFile := p.Operations.Delete
	if fromFile == "" {
		fromFile = p.DeleteOperation
	}
	ids.Delete, err = oneOperationID(opDelete, fromFile, ids.Delete)
	return ids, err
}

func oneOperationID(v verb, fromFile, flag string) (string, error) {
	if flag != "" && fromFile != "" && flag != fromFile {
		return "", fmt.Errorf("%s: the overrides name %s, and the flag names %s", v, fromFile, flag)
	}
	if flag != "" {
		return flag, nil
	}
	return fromFile, nil
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

func (p Policy) fixedRequest(component, field string) bool {
	return component != "" && slices.Contains(p.FixedRequest, component+"."+field)
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
	for i := range t.Groups {
		// A skipped arm is one that skippedArmIssues allows: the group allows no arm.
		t.Groups[i].Arms = slices.DeleteFunc(slices.Clone(t.Groups[i].Arms), func(arm string) bool { return p.skips(t.Schema, arm) })
	}
	for _, f := range t.Fields {
		p.pruneType(f.Type, seen)
	}
}

// requiredOf returns the required list of a component without the fields that the policy
// makes optional.
func (p *Policy) requiredOf(component string, required []string) []string {
	if p == nil || component == "" || len(p.Optional) == 0 {
		return required
	}
	return slices.DeleteFunc(slices.Clone(required), func(field string) bool {
		return slices.Contains(p.Optional, component+"."+field)
	})
}

// withOptional returns s with the required list of the component after the policy, so the
// contract checks treat the fields that the policy makes optional as optional.
func (p *Policy) withOptional(component string, s *base.Schema) *base.Schema {
	if s == nil {
		return nil
	}
	required := p.requiredOf(component, s.Required)
	if len(required) == len(s.Required) {
		return s
	}
	out := *s
	out.Required = required
	return &out
}

// ComputedLine reports whether a computed: true line makes component.field computed. The
// model holds such an object as a types.Object, which can be unknown in a plan.
func (p Policy) ComputedLine(component, field string) bool { return p.computed(component, field) }

func (p *Policy) computed(component, field string) bool {
	return p != nil && component != "" && slices.Contains(p.Computed, component+"."+field)
}

// asSet makes t, the type of component.field, a set when a collection: set line says so, and a
// map when a keyedBy line says so.
func (p *Policy) asSet(component, field string, t *Type) {
	if p != nil && t != nil && t.Kind == List && component != "" && slices.Contains(p.Sets, component+"."+field) {
		t.Kind = Set
	}
	p.asMap(component, field, t)
}

// KeyedLine is a keyedBy line: the property that keys the list, and the property that is the
// value ("" for the rest of the item). Object shows the list as an object with an attribute per
// value of an enum key. Arms gives a value the oneOf arm that its item holds. KeyAs is the string
// property that expand sets to the attribute name.
type KeyedLine struct {
	Key, Value string
	Object     bool
	Arms       map[string]string
	KeyAs      string
}

// EnumValue is an accepted API value of an enum and its Terraform value.
type EnumValue struct {
	API, TF string
}

// asMap makes t, a list of objects, a map keyed by a string property of the object when a
// keyedBy line says so. The map value is the rest of the object, or one property of it. A line
// that does not fit leaves t a list: the generator reports the line.
func (p *Policy) asMap(component, field string, t *Type) {
	if p == nil || t == nil || component == "" || t.Kind != List || t.Elem == nil || t.Elem.Kind != Object {
		return
	}
	line, ok := p.KeyedBy[component+"."+field]
	if !ok {
		return
	}
	if line.Object {
		p.asKeyedObject(line, t)
		return
	}
	keyed := [2]string{line.Key, line.Value}
	item := t.Elem
	key := slices.IndexFunc(item.Fields, func(f *Field) bool { return f.Name == keyed[0] })
	if key < 0 || item.Fields[key].Type.Kind != String {
		return
	}
	elem := *item
	elem.Fields = slices.Delete(slices.Clone(item.Fields), key, key+1)
	t.Elem = &elem
	if keyed[1] != "" {
		value := slices.IndexFunc(item.Fields, func(f *Field) bool { return f.Name == keyed[1] })
		if value < 0 || len(item.Fields) != 2 {
			t.Elem = item
			return
		}
		t.Elem = item.Fields[value].Type
	}
	t.Kind, t.Keyed = Map, &Keyed{Key: keyed[0], Value: keyed[1], Item: item}
}

// TerraformName returns the Terraform attribute name of field of component: the name of a
// name line, or the name derived from the JSON name.
func (p Policy) TerraformName(component, field string) string {
	if n, ok := p.Names[component+"."+field]; ok {
		return n
	}
	return TerraformName(field)
}

// ComponentOf returns the response component of a type: a request view names its request
// component and keeps the response component in Model. Field lines name the response component.
func ComponentOf(t *Type) string {
	if t.Model != "" {
		return t.Model
	}
	return t.Schema
}

// asKeyedObject makes t, a list of objects keyed by an enum property, an object with one
// attribute per accepted value of the enum. Each attribute is the item without the key, and with
// only the oneOf arm that the line gives the value, which is required there. The attributes are
// optional and computed: the API can return an item that the configuration does not have.
func (p *Policy) asKeyedObject(line KeyedLine, t *Type) {
	item := t.Elem
	key := slices.IndexFunc(item.Fields, func(f *Field) bool { return f.Name == line.Key })
	if key < 0 || item.Fields[key].Type.Kind != Enum {
		return
	}
	values := p.EnumValues[item.Fields[key].Type.Schema]
	if len(values) == 0 {
		return
	}
	arms := map[string]bool{}
	for _, arm := range line.Arms {
		arms[arm] = true
	}
	// The object has its own component name: its attributes are named after the enum values, and
	// must not meet the lines of the item fields.
	container := Type{Kind: Object, Schema: item.Schema + ".keyed"}
	keyed := &Keyed{Key: line.Key, Item: item, Object: true, KeyAs: line.KeyAs}
	for _, v := range values {
		variant := *item
		variant.Variant, variant.Groups, variant.AllowNone, variant.Fields = GoName(v.TF), nil, false, nil
		for i, f := range item.Fields {
			switch {
			case i == key:
				continue
			case f.Name == line.Arms[v.API]:
				own := *f
				own.Attrs.Required = true
				variant.Fields = append(variant.Fields, &own)
			case arms[f.Name] || inGroup(item.Groups, f.Name):
				continue
			default:
				variant.Fields = append(variant.Fields, f)
			}
		}
		container.Fields = append(container.Fields, &Field{Name: v.TF, Type: &variant, Attrs: Attrs{Computed: true}})
		keyed.Values = append(keyed.Values, KeyedValue{API: v.API, Field: v.TF})
	}
	container.Keyed = keyed
	*t = container
}

// inGroup reports whether name is an arm of a oneOf group.
func inGroup(groups []OneOfGroup, name string) bool {
	for _, g := range groups {
		if slices.Contains(g.Arms, name) {
			return true
		}
	}
	return false
}
