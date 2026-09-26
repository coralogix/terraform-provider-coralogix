package main

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"

	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"

	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen-poc/internal/model"
)

// realSDK is the default SDK module. go.mod pins its version, and
// go/packages loads that version (README.md, "Pinned versions").
// --sdk-module selects another module with the same layout, for example the
// fake SDK.
const realSDK = "github.com/coralogix/coralogix-management-sdk"

// sdkGenRoot is the parent of the generated SDK packages of module.
func sdkGenRoot(module string) string { return module + "/go/openapi/gen" }

// sdkClientSet is the handwritten cxsdk package of module.
func sdkClientSet(module string) string { return module + "/go/openapi/cxsdk" }

// refKind is the kind of Go object an sdkRef names.
type refKind string

const (
	kindPackage refKind = "package"
	kindType    refKind = "type"
	kindField   refKind = "field"
	kindMethod  refKind = "method"
	kindFunc    refKind = "func"
	kindConst   refKind = "const"
)

// rule is how an SDK name is derived from the spec. The "special" rules copy
// a choice that the SDK makes and the spec does not show
// (README.md, findings F15–F18).
type rule string

const (
	ruleTag         rule = "tag"                   // the operation tag
	ruleOperationID rule = "operationId"           // the operationId
	ruleComponent   rule = "component"             // the component schema name
	ruleProperty    rule = "property"              // the property name
	ruleEnumValue   rule = "enum value"            // the component name and the enum value
	ruleInlineBody  rule = "special: inline body"  // F15
	ruleEmptyObject rule = "special: empty object" // F16
	ruleClientSet   rule = "special: cxsdk"        // F17
)

// sdkRef is one SDK name that the generated code uses.
type sdkRef struct {
	Path  string // model path, for example "fields.config.sqlLoad.joinLimit"
	Kind  refKind
	Pkg   string // import path of the package that has the name
	Owner string // field, method: the type that has it
	Name  string // package: the package name
	// Want is the Go type of a field or the signature of a method, written
	// relative to Pkg. It is "" for the other kinds.
	Want string
	// WantValue is the value type that a field may have instead of the
	// pointer type Want. checkSDKNames sets Want to it when the SDK has it.
	WantValue string
	// ByType: a method that the SDK may name another way. When Name is not
	// found, the check uses the one method of Owner with type Want (F17).
	ByType bool
	Rule   rule
	// Schema is the component name of a type ref that comes from the spec.
	// The generator uses it to find the SDK type of a model type.
	Schema string
}

// sdkName is the name as the Go code writes it, for example "SqlLoadConfig.JoinLimit".
func (s sdkRef) sdkName() string {
	if s.Owner == "" {
		return s.Name
	}
	return s.Owner + "." + s.Name
}

// resourceTag returns the one tag that all four operations share. The SDK
// makes one package per tag.
func resourceTag(doc *v3.Document, r *model.Resource) (string, error) {
	tag := ""
	for _, op := range []model.Operation{r.Create, r.Get, r.Update, r.Delete} {
		item, ok := doc.Paths.PathItems.Get(op.Path)
		if !ok {
			return "", fmt.Errorf("%s: path %s not in the spec", op.OperationID, op.Path)
		}
		o, ok := item.GetOperations().Get(strings.ToLower(op.Method))
		if !ok {
			return "", fmt.Errorf("%s: method %s not in the spec", op.OperationID, op.Method)
		}
		if len(o.Tags) != 1 {
			return "", fmt.Errorf("%s: %d tags, want 1", op.OperationID, len(o.Tags))
		}
		if tag != "" && o.Tags[0] != tag {
			return "", fmt.Errorf("%s: tag %q, want %q (as in the other operations)", op.OperationID, o.Tags[0], tag)
		}
		tag = o.Tags[0]
	}
	return tag, nil
}

// resolveSDKNames lists the SDK names that the generated code uses for the
// resource. It only applies the naming rules. checkSDKNames confirms that
// the names exist.
func resolveSDKNames(r *model.Resource, tag, module string) ([]sdkRef, error) {
	pkgName := strings.ToLower(strings.ReplaceAll(tag, " ", "_"))
	s := &resolver{pkg: sdkGenRoot(module) + "/" + pkgName, seen: map[string]bool{}, bodies: map[string]string{}}
	client := camelize(tag) + "APIService"
	s.add(sdkRef{Path: "resource", Kind: kindPackage, Name: pkgName, Rule: ruleTag})
	s.add(sdkRef{Path: "resource", Kind: kindType, Name: client, Rule: ruleTag})

	if err := s.operations(r, client); err != nil {
		return nil, err
	}

	resource := goTypeName(r.Name)
	s.add(sdkRef{Path: "fields", Kind: kindType, Name: resource, Rule: ruleComponent, Schema: r.Name})
	for _, f := range r.Fields {
		path := "fields." + f.Name
		if f.InGet {
			if err := s.field(path, resource, f.Name, f.Type); err != nil {
				return nil, err
			}
		}
		if err := s.nested(path, f.Type); err != nil {
			return nil, err
		}
	}
	for _, f := range r.Fields {
		if f.Create != nil {
			if err := s.field("create.body."+f.Name, s.bodies["create"], f.Name, f.Type); err != nil {
				return nil, err
			}
		}
	}
	for _, f := range r.Fields {
		if f.Update != nil {
			if err := s.field("update.body."+f.Name, s.bodies["update"], f.Name, f.Type); err != nil {
				return nil, err
			}
		}
	}
	if err := s.updateExtras(r); err != nil {
		return nil, err
	}
	// cxsdk is handwritten. Its accessor name usually drops " Service" from the
	// tag (F17); when it does not, the check finds the accessor by its type.
	// The provider data is a *ClientSet. The resource wraps API errors with
	// NewAPIError and reads the HTTP status with Code.
	for _, ref := range []sdkRef{
		{Path: "cxsdk", Kind: kindPackage, Name: "cxsdk"},
		{Path: "cxsdk", Kind: kindType, Name: "ClientSet"},
		{Path: "resource", Kind: kindMethod, Owner: "ClientSet", Name: camelize(strings.TrimSuffix(tag, " Service")),
			Want: "func() *" + pkgName + "." + client, ByType: true},
		{Path: "cxsdk.errors", Kind: kindFunc, Name: "NewAPIError", Want: "func(resp *http.Response, err error) error"},
		{Path: "cxsdk.errors", Kind: kindFunc, Name: "Code", Want: "func(err error) int"},
	} {
		ref.Pkg, ref.Rule = sdkClientSet(module), ruleClientSet
		s.refs = append(s.refs, ref)
	}
	return s.refs, nil
}

// updateExtras adds the Update body fields that are not resource fields: the
// update mask of a PATCH, and the id when the Update path has none.
func (s *resolver) updateExtras(r *model.Resource) error {
	str := &model.Type{Kind: model.String}
	if !r.Replace {
		if err := s.field("update.body."+r.UpdateMask, s.bodies["update"], r.UpdateMask, str); err != nil {
			return err
		}
	}
	if r.IDInBody {
		return s.field("update.body."+r.IDParam, s.bodies["update"], r.IDParam, str)
	}
	return nil
}

// operations adds the SDK names of the four operations.
func (s *resolver) operations(r *model.Resource, client string) error {
	ops := []struct {
		name string
		op   model.Operation
		id   bool // the method takes the id path parameter
	}{
		{"create", r.Create, false},
		{"get", r.Get, true},
		{"update", r.Update, !r.IDInBody},
		{"delete", r.Delete, true},
	}
	for _, o := range ops {
		// A singleton has no id in the path (D18).
		if err := s.operation(r, o.name, o.op, client, o.id && !r.Singleton); err != nil {
			return err
		}
	}

	return nil
}

type resolver struct {
	pkg    string
	refs   []sdkRef
	seen   map[string]bool   // component schemas already walked
	bodies map[string]string // operation name → request body type
}

func (s *resolver) add(ref sdkRef) {
	ref.Pkg = s.pkg
	s.refs = append(s.refs, ref)
}

// operation adds the method, the request builder, the body, and the response
// of one operation. The generated code calls, for example:
//
//	client.AiEvaluationsServiceGetAiEvaluation(ctx, id).Execute()
func (s *resolver) operation(r *model.Resource, name string, op model.Operation, client string, withID bool) error {
	path := name
	method := camelize(op.OperationID)
	builder := "Api" + method + "Request"
	params := "ctx context.Context"
	if withID {
		params += ", " + r.IDParam + " string"
	}
	s.add(sdkRef{Path: path, Kind: kindMethod, Owner: client, Name: method,
		Want: "func(" + params + ") " + builder, Rule: ruleOperationID})
	s.add(sdkRef{Path: path, Kind: kindType, Name: builder, Rule: ruleOperationID})

	switch op.Body {
	case "":
	case "inline":
		body := inlineBodyName(method, op)
		s.bodies[name] = body
		s.add(sdkRef{Path: path + ".body", Kind: kindType, Name: body, Rule: ruleInlineBody})
		s.add(sdkRef{Path: path + ".body", Kind: kindMethod, Owner: builder, Name: body,
			Want: "func(" + lowerFirst(body) + " " + body + ") " + builder, Rule: ruleInlineBody})
	default:
		return fmt.Errorf("%s.body: component body %s is not supported", path, op.Body)
	}

	resp := goTypeName(op.Response.Schema)
	if op.Response.Empty {
		// An object with no fields has no SDK type (F16).
		s.add(sdkRef{Path: path, Kind: kindMethod, Owner: builder, Name: "Execute",
			Want: "func() (map[string]interface{}, *http.Response, error)", Rule: ruleEmptyObject})
		return nil
	}
	s.add(sdkRef{Path: path + ".response", Kind: kindType, Name: resp, Rule: ruleComponent})
	s.add(sdkRef{Path: path, Kind: kindMethod, Owner: builder, Name: "Execute",
		Want: "func() (*" + resp + ", *http.Response, error)", Rule: ruleOperationID})
	if op.Response.Field != "" {
		s.add(sdkRef{Path: path + ".response." + op.Response.Field, Kind: kindField, Owner: resp,
			Name: goFieldName(op.Response.Field), Want: "*" + goTypeName(r.Name), Rule: ruleProperty})
	}
	return nil
}

// inlineBodyName is the SDK type of an inline request body. openapi-generator
// names it after its title, or after the operationId when it has none. When
// a component has the title as its name, it adds "1" (F15, F45).
func inlineBodyName(method string, op model.Operation) string {
	switch {
	case op.BodyTitle == "":
		return method + "Request"
	case op.BodyTitleIsComponent:
		return goTypeName(op.BodyTitle) + "1"
	}
	return goTypeName(op.BodyTitle)
}

// field adds the Go field for the property name of the owner type.
func (s *resolver) field(path, owner, name string, t *model.Type) error {
	goType, r, err := fieldType(t)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	ref := sdkRef{Path: path, Kind: kindField, Owner: owner, Name: goFieldName(name), Want: goType, Rule: r}
	// openapi-generator uses a value, not a pointer, for a required field.
	// Only the SDK knows which spec it was generated from (F18), so the
	// check accepts both and keeps the one that the SDK has.
	if strings.HasPrefix(goType, "*") {
		ref.WantValue = goType[1:]
	}
	s.add(ref)
	return nil
}

// nested adds the types, fields, and enum constants inside t. It walks each
// component schema once, so a path shows the first place that uses it.
func (s *resolver) nested(path string, t *model.Type) error {
	switch t.Kind {
	case model.List, model.Set, model.Map:
		return s.nested(path, t.Elem)
	case model.Enum:
		if s.seen[t.Schema] {
			return nil
		}
		s.seen[t.Schema] = true
		name := goTypeName(t.Schema)
		s.add(sdkRef{Path: path, Kind: kindType, Name: name, Rule: ruleComponent, Schema: t.Schema})
		for _, v := range t.Values {
			s.add(sdkRef{Path: path + "." + v, Kind: kindConst, Name: enumConstName(name, v), Rule: ruleEnumValue})
		}
	case model.Object, model.OneOf:
		// An object with no fields has no SDK type (F16).
		if len(t.Fields) == 0 || s.seen[t.Schema] {
			return nil
		}
		s.seen[t.Schema] = true
		name := goTypeName(t.Schema)
		s.add(sdkRef{Path: path, Kind: kindType, Name: name, Rule: ruleComponent, Schema: t.Schema})
		for _, f := range t.Fields {
			child := path + "." + f.Name
			if err := s.field(child, name, f.Name, f.Type); err != nil {
				return err
			}
			if err := s.nested(child, f.Type); err != nil {
				return err
			}
		}
	}
	return nil
}

// fieldType is the Go type of an optional SDK struct field. A required field
// can also be the value type (see field).
func fieldType(t *model.Type) (string, rule, error) {
	if (t.Kind == model.Object || t.Kind == model.OneOf) && len(t.Fields) == 0 {
		return "map[string]interface{}", ruleEmptyObject, nil
	}
	if t.Kind == model.List || t.Kind == model.Set || t.Kind == model.Map {
		elem, _, err := fieldType(t.Elem)
		if err != nil {
			return "", "", err
		}
		if !strings.HasPrefix(elem, "*") {
			return "", "", fmt.Errorf("%s of %s is not supported", t.Kind, elem)
		}
		if t.Kind == model.Map {
			return "map[string]" + elem[1:], ruleProperty, nil
		}
		return "[]" + elem[1:], ruleProperty, nil
	}
	v, err := valueType(t)
	if err != nil {
		return "", "", err
	}
	return "*" + v, ruleProperty, nil
}

func valueType(t *model.Type) (string, error) {
	switch t.Kind {
	case model.String:
		if t.Format == "date-time" {
			return "time.Time", nil
		}
		return "string", nil
	case model.Bool:
		return "bool", nil
	case model.Number:
		switch t.Format {
		case "double":
			return "float64", nil
		case "float":
			return "float32", nil
		}
	case model.Integer:
		if t.WireString {
			return "string", nil // D7: the SDK has the JSON string
		}
		switch t.Format {
		case "int32":
			return "int32", nil
		case "int64":
			return "int64", nil
		}
	case model.Enum, model.Object, model.OneOf:
		if t.Schema == "" {
			return "", fmt.Errorf("inline %s schema is not supported", t.Kind)
		}
		return goTypeName(t.Schema), nil
	}
	return "", fmt.Errorf("%s with format %q is not supported", t.Kind, t.Format)
}

// enumWordBreak finds a lower-case letter or a digit before an upper-case
// letter. openapi-generator puts "_" there in an enum constant name.
var enumWordBreak = regexp.MustCompile(`([a-z\d])([A-Z])`)

// enumConstName is the SDK constant of the enum value v of the SDK type
// typeName: "TextAlignment", "TEXT_ALIGNMENT_LEFT" → "TEXTALIGNMENT_TEXT_ALIGNMENT_LEFT".
// openapi-generator also breaks the words of the value at a digit before a
// letter: "E2M_TYPE_LOGS2METRICS" → "E2_M_TYPE_LOGS2_METRICS" (F55).
func enumConstName(typeName, v string) string {
	return strings.ToUpper(typeName) + "_" + enumWordBreak.ReplaceAllString(v, "${1}_${2}")
}

// goTypeName is the SDK type for a component name: "v3.FilterOperator" → "V3FilterOperator".
func goTypeName(component string) string { return camelize(component) }

// goFieldName is the SDK struct field for a property name: "joinLimit" → "JoinLimit".
func goFieldName(property string) string { return camelize(property) }

// camelize splits s at each character that is not a letter or a digit, and
// joins the parts with the first letter in upper case. It keeps the case of
// the other letters: "AI Evaluations Service" → "AIEvaluationsService".
func camelize(s string) string {
	var b strings.Builder
	for part := range strings.FieldsFuncSeq(s, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}) {
		b.WriteString(strings.ToUpper(part[:1]) + part[1:])
	}
	return b.String()
}

func lowerFirst(s string) string { return strings.ToLower(s[:1]) + s[1:] }
