package model

import (
	"fmt"
	"slices"

	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/issue"
)

// contractError is a field that breaks the lifecycle contract. Validate and
// Build report it as the same issue.
type contractError struct {
	code, location, message, remediation string
}

func (e *contractError) Error() string { return e.location + ": " + e.message }

func (e *contractError) issue() issue.Issue {
	return issue.Issue{Code: e.code, Location: e.location, Message: e.message, Remediation: e.remediation}
}

// mergeRequests merges the Create and the Update types of a top-level field
// into get, its type in the resource response, and classifies the nested
// fields. create or update is nil when that request does not have the field.
// location names the field in an error.
//
// The request types can use other components than the response, for example
// ThingSpecCreate for ThingSpec. Nested fields match by name. A nested field
// gets the lifecycle of a top-level field: Create, Update, and Get make it
// normal, Create and Get immutable, and Get only computed.
func mergeRequests(location string, get, create, update *Type) error {
	if create != nil {
		if err := mergeRequest(location, get, create, opCreate); err != nil {
			return err
		}
	}
	if update != nil {
		if err := mergeRequest(location, get, update, opUpdate); err != nil {
			return err
		}
	}
	return classifyFields(location, get, create != nil, update != nil)
}

// mergeRequest checks that req, the type of a value in the op request, is the
// type of the same value in the resource response. It records on get the
// request components and the request attributes of the nested fields.
func mergeRequest(location string, get, req *Type, op verb) error {
	if detail := typeDifference(get, req, op); detail != "" {
		return &contractError{
			code:        "FIELD_TYPE_INCONSISTENT",
			location:    location,
			message:     fmt.Sprintf("The %s request type differs from the resource response type: %s.", op, detail),
			remediation: "Use the same type, limits, enum values, and oneOf arms in the Create, Update, and resource response schemas. Component names and descriptions can differ.",
		}
	}
	if op == opCreate {
		adoptCreateRule(get, req)
	}
	switch get.Kind {
	case List, Set:
		return mergeRequest(location+"[]", get.Elem, req.Elem, op)
	case Map:
		return mergeRequest(location+"{}", get.Elem, req.Elem, op)
	case Object, OneOf:
		return mergeFields(location, get, req, op)
	}
	return nil
}

func mergeFields(location string, get, req *Type, op verb) error {
	if op == opCreate {
		get.CreateSchema = req.Schema
	} else {
		get.UpdateSchema = req.Schema
	}
	for _, rf := range req.Fields {
		at := location + "." + rf.Name
		gf := fieldNamed(get.Fields, rf.Name)
		if gf == nil {
			return &contractError{
				code:        "FIELD_LIFECYCLE_UNSUPPORTED",
				location:    at,
				message:     fmt.Sprintf("The field is in the %s request but not in the resource response.", op),
				remediation: "Return the field in the resource response, or remove it from the request.",
			}
		}
		if !samePointer(gf.Attrs.Default, rf.Attrs.Default) {
			return &contractError{
				code:        "FIELD_TYPE_INCONSISTENT",
				location:    at,
				message:     fmt.Sprintf("The %s request and the resource response declare different defaults.", op),
				remediation: "Declare the same default in every schema of the field.",
			}
		}
		attrs := rf.Attrs
		if op == opCreate {
			gf.Create = &attrs
		} else {
			gf.Update = &attrs
		}
		if err := mergeRequest(at, gf.Type, rf.Type, op); err != nil {
			return err
		}
	}
	return nil
}

// typeDifference returns how req differs from get, or "". It does not compare
// component names, descriptions, or the nested fields of an object.
func typeDifference(get, req *Type, op verb) string {
	switch {
	case get.Kind != req.Kind:
		return fmt.Sprintf("kind %s, want %s", req.Kind, get.Kind)
	case get.Format != req.Format || get.WireString != req.WireString:
		return fmt.Sprintf("format %q, want %q", req.Format, get.Format)
	case get.Kind == Enum && get.Schema != req.Schema:
		return fmt.Sprintf("enum %s, want %s", req.Schema, get.Schema)
	case !slices.Equal(get.Values, req.Values) || get.EnumZero != req.EnumZero:
		return "different enum values"
	case !sameLimits(get, req):
		return "different limits"
	}
	return oneOfDifference(get, req, op)
}

func sameLimits(a, b *Type) bool {
	return samePointer(a.MinLength, b.MinLength) && samePointer(a.MaxLength, b.MaxLength) &&
		samePointer(a.Minimum, b.Minimum) && samePointer(a.Maximum, b.Maximum) &&
		samePointer(a.MinItems, b.MinItems) && samePointer(a.MaxItems, b.MaxItems)
}

// oneOfDifference returns how the oneOf arms of req differ from get, or "".
// The arms must be the same. Whether a oneOf can have no arm can differ in one
// way: Create can require an arm where Update and the response allow none. A
// PATCH that changes only another field, for example family.active, sends no
// arm.
func oneOfDifference(get, req *Type, op verb) string {
	switch {
	case get.Discriminator != req.Discriminator:
		return fmt.Sprintf("discriminator %q, want %q", req.Discriminator, get.Discriminator)
	case get.Kind == OneOf && !sameSet(fieldNames(get), fieldNames(req)):
		return fmt.Sprintf("oneOf arms %v, want %v", fieldNames(req), fieldNames(get))
	case get.Kind == OneOf && noArmRuleBroken(get.AllowNone, req.AllowNone, op):
		return noArmDifference(op)
	case !sameGroupArms(get.Groups, req.Groups):
		return "different oneOf groups"
	}
	for _, g := range get.Groups {
		if noArmRuleBroken(g.AllowNone, matchingGroup(req.Groups, g).AllowNone, op) {
			return noArmDifference(op)
		}
	}
	return ""
}

// noArmRuleBroken reports whether a oneOf requires an arm that Create can
// leave out. In the Create merge, get is the response. In the Update merge,
// get has the Create rule (adoptCreateRule).
func noArmRuleBroken(getAllowsNone, reqAllowsNone bool, op verb) bool {
	if op == opCreate {
		return reqAllowsNone && !getAllowsNone
	}
	return getAllowsNone && !reqAllowsNone
}

func noArmDifference(op verb) string {
	if op == opCreate {
		return "Create allows a oneOf with no arm, but the resource response requires one"
	}
	return "Create allows a oneOf with no arm, but the Update request requires one"
}

// adoptCreateRule gives get the Create rule for a oneOf with no arm. The
// Terraform configuration follows Create, so it requires an arm when Create
// does, also when Update and the response allow none.
func adoptCreateRule(get, create *Type) {
	get.AllowNone = create.AllowNone
	for i, g := range get.Groups {
		get.Groups[i].AllowNone = matchingGroup(create.Groups, g).AllowNone
	}
}

// classifyFields sets the Behavior of the nested fields of t. create and
// update tell whether the Create and the Update requests send the value of t.
func classifyFields(location string, t *Type, create, update bool) error {
	if t == nil {
		return nil
	}
	switch t.Kind {
	case List, Set:
		return classifyFields(location+"[]", t.Elem, create, update)
	case Map:
		return classifyFields(location+"{}", t.Elem, create, update)
	case Object, OneOf:
	default:
		return nil
	}
	for _, f := range t.Fields {
		at := location + "." + f.Name
		if err := classifyField(at, f, create, update); err != nil {
			return err
		}
		if err := classifyFields(at, f.Type, create && f.Create != nil, update && f.Update != nil); err != nil {
			return err
		}
	}
	return nil
}

// classifyField sets the Behavior of f, a field of an object that the
// Create and the Update requests send when create and update are true.
func classifyField(at string, f *Field, create, update bool) error {
	inCreate, inUpdate := create && f.Create != nil, update && f.Update != nil
	switch {
	case !create && !update:
		// The value is computed. Its fields have no lifecycle of their own.
		return nil
	case !update:
		// Update does not send the parent. A change of the parent already
		// replaces the resource, so a Create field is normal inside it.
		f.Behavior = Computed
		if inCreate {
			f.Behavior = Normal
		}
		return nil
	}
	behavior, err := Classify(inCreate, inUpdate, true)
	if err != nil {
		return &contractError{
			code:        "FIELD_LIFECYCLE_UNSUPPORTED",
			location:    at,
			message:     fmt.Sprintf("The field locations are Create=%t, Update=%t, Get=true.", inCreate, inUpdate),
			remediation: "Use a managed, immutable, or computed field lifecycle.",
		}
	}
	f.Behavior = behavior
	if inCreate && inUpdate && !f.Create.Required && f.Update.Required {
		return &contractError{
			code:        "FIELD_REQUIREDNESS_UNSUPPORTED",
			location:    at,
			message:     "The field is optional in Create but required in Update.",
			remediation: "Make the field optional in Update, or require it in Create so Terraform always has a value to send.",
		}
	}
	return nil
}

// CreateType returns the type of the value in the Create request. Its objects
// name the Create components, and have the fields of the Create request with
// their Create attributes. Model names the response component of an object.
func (t *Type) CreateType() *Type { return t.requestType(opCreate) }

// UpdateType is CreateType for the Update request.
func (t *Type) UpdateType() *Type { return t.requestType(opUpdate) }

func (t *Type) requestType(op verb) *Type {
	if t == nil {
		return nil
	}
	out := *t
	out.Elem = t.Elem.requestType(op)
	if t.Kind != Object && t.Kind != OneOf {
		return &out
	}
	out.Model, out.Schema = t.Schema, t.CreateSchema
	if op == opUpdate {
		out.Schema = t.UpdateSchema
	}
	out.Fields = nil
	for _, f := range t.Fields {
		attrs := f.Create
		if op == opUpdate {
			attrs = f.Update
		}
		if attrs == nil {
			continue
		}
		out.Fields = append(out.Fields, &Field{Name: f.Name, Description: f.Description, Attrs: *attrs, Type: f.Type.requestType(op), Behavior: f.Behavior})
	}
	return &out
}

func fieldNamed(fields []*Field, name string) *Field {
	for _, f := range fields {
		if f.Name == name {
			return f
		}
	}
	return nil
}

func fieldNames(t *Type) []string {
	names := make([]string, 0, len(t.Fields))
	for _, f := range t.Fields {
		names = append(names, f.Name)
	}
	return names
}

// sameGroupArms reports whether a and b have groups with the same arms, in
// any order.
func sameGroupArms(a, b []OneOfGroup) bool {
	if len(a) != len(b) {
		return false
	}
	for _, g := range a {
		if !slices.ContainsFunc(b, func(h OneOfGroup) bool { return sameSet(g.Arms, h.Arms) }) {
			return false
		}
	}
	return true
}

// matchingGroup returns the group of groups with the arms of g.
func matchingGroup(groups []OneOfGroup, g OneOfGroup) OneOfGroup {
	i := slices.IndexFunc(groups, func(h OneOfGroup) bool { return sameSet(g.Arms, h.Arms) })
	return groups[i]
}

func samePointer[T comparable](a, b *T) bool {
	return a == nil && b == nil || a != nil && b != nil && *a == *b
}
