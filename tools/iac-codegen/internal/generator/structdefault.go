package generator

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/overrides"
)

// A default that is not a plain scalar is a Terraform value that holds the types of the attribute:
// an object, a list of objects, or a typed null. Its types come from the attributes below it, and
// the extra attributes are added after the field lines, so resolveStructDefaults builds the
// expression last. The first pass (applyDefault) only keeps the value of the line.

// deferredDefault reports whether the default of the line is built after every attribute has its
// lines and the extra attributes: a null, an object, or an empty list of objects.
func deferredDefault(a *tfAttr, value any) bool {
	switch value.(type) {
	case map[string]any:
		return true
	case []any:
		return strings.HasSuffix(a.Kind, "Nested")
	}
	return overrides.IsNullDefault(value)
}

// hasDefault reports whether the attribute has a static default, built or not.
func hasDefault(a *tfAttr) bool { return a.Default != "" || a.StructDefault != nil }

// resolveStructDefaults builds the default of each attribute that has a deferred one.
func resolveStructDefaults(attrs []*tfAttr, file *overrides.File) error {
	for _, a := range attrs {
		if a.StructDefault != nil {
			expr, err := defaultValueExpr(a, a.StructDefault, a.Name, file)
			if err != nil {
				return fmt.Errorf("%s: default: %w", a.Component+"."+a.Property, err)
			}
			a.Default = fmt.Sprintf("%sdefault.StaticValue(%s)", strings.ToLower(a.ValueKind), expr)
			a.StructDefault = nil
		}
		if err := resolveStructDefaults(a.Attributes, file); err != nil {
			return err
		}
	}
	return nil
}

// attrTypeExpr is the Go expression of the attr.Type of the attribute, as the schema states it.
func attrTypeExpr(a *tfAttr) (string, error) {
	switch a.Kind {
	case "String", "Bool", "Int64", "Int32", "Float64", "Float32":
		if a.CustomType != "" {
			return a.CustomType, nil
		}
		return "types." + a.Kind + "Type", nil
	case "Dynamic":
		return "types.DynamicType", nil
	case "List", "Set", "Map":
		if a.ElementType == "" {
			return "", fmt.Errorf("the %s attribute %s has no element type", a.Kind, a.Name)
		}
		return fmt.Sprintf("types.%sType{ElemType: %s}", a.Kind, a.ElementType), nil
	case "SingleNested":
		return objectTypeExpr(a.Attributes)
	case "ListNested", "SetNested", "MapNested":
		elem, err := objectTypeExpr(a.Attributes)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("types.%sType{ElemType: %s}", strings.TrimSuffix(a.Kind, "Nested"), elem), nil
	}
	return "", fmt.Errorf("the attribute %s of kind %s has no type expression", a.Name, a.Kind)
}

func objectTypeExpr(attrs []*tfAttr) (string, error) {
	m, err := attrTypesMapExpr(attrs)
	if err != nil {
		return "", err
	}
	return "types.ObjectType{AttrTypes: " + m + "}", nil
}

// attrTypesMapExpr is the map of attribute types of an object, in name order.
func attrTypesMapExpr(attrs []*tfAttr) (string, error) {
	sorted := slices.Clone(attrs)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Name < sorted[j].Name })
	parts := make([]string, 0, len(sorted))
	for _, child := range sorted {
		expr, err := attrTypeExpr(child)
		if err != nil {
			return "", err
		}
		parts = append(parts, fmt.Sprintf("%q: %s", child.Name, expr))
	}
	return "map[string]attr.Type{" + strings.Join(parts, ", ") + "}", nil
}

// elementTypeExpr is the type of the elements of a collection attribute.
func elementTypeExpr(a *tfAttr) (string, error) {
	if strings.HasSuffix(a.Kind, "Nested") {
		return objectTypeExpr(a.Attributes)
	}
	if a.ElementType == "" {
		return "", fmt.Errorf("the %s attribute %s has no element type", a.Kind, a.Name)
	}
	return a.ElementType, nil
}

// nullValueExpr is the typed null of the attribute.
func nullValueExpr(a *tfAttr) (string, error) {
	switch a.Kind {
	case "String", "Bool", "Int64", "Int32", "Float64", "Float32":
		if a.CustomType == rfc3339Type {
			return "timetypes.NewRFC3339Null()", nil
		}
		if a.CustomType != "" {
			return "", fmt.Errorf("the attribute %s has the custom type %s, which has no null expression", a.Name, a.CustomType)
		}
		return "types." + a.Kind + "Null()", nil
	case "Dynamic":
		return "types.DynamicNull()", nil
	case "List", "Set", "Map", "ListNested", "SetNested", "MapNested":
		elem, err := elementTypeExpr(a)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("types.%sNull(%s)", strings.TrimSuffix(a.Kind, "Nested"), elem), nil
	case "SingleNested":
		m, err := attrTypesMapExpr(a.Attributes)
		if err != nil {
			return "", err
		}
		return "types.ObjectNull(" + m + ")", nil
	}
	return "", fmt.Errorf("the attribute %s of kind %s has no null expression", a.Name, a.Kind)
}

// defaultValueExpr is the Go expression of the Terraform value that the line gives the attribute: a
// typed null for nil and null, the value of a scalar, [], or the attributes of an object. An
// attribute that an object leaves out is null.
func defaultValueExpr(a *tfAttr, value any, path string, file *overrides.File) (string, error) {
	if value == nil {
		return nullValueExpr(a)
	}
	switch v := value.(type) {
	case map[string]any:
		return objectDefaultExpr(a, v, path, file)
	case []any:
		return emptyListDefaultExpr(a, v, path)
	}
	if overrides.IsNullDefault(value) {
		return nullDefaultExpr(a, path)
	}
	return scalarDefaultExpr(a, value, path, file)
}

// nullDefaultExpr is default: null: a list, set, map, or object that plans null.
func nullDefaultExpr(a *tfAttr, path string) (string, error) {
	switch a.Kind {
	case "List", "Set", "Map", "ListNested", "SetNested", "MapNested", "SingleNested":
		return nullValueExpr(a)
	}
	return "", fmt.Errorf("%s: null fits a list, set, map, or object attribute, not a %s: an omitted optional scalar is null without a default", path, a.Kind)
}

func emptyListDefaultExpr(a *tfAttr, v []any, path string) (string, error) {
	if a.Kind != "List" && a.Kind != "Set" && a.Kind != "ListNested" && a.Kind != "SetNested" {
		return "", fmt.Errorf("%s: [] fits a list or set attribute, not a %s", path, a.Kind)
	}
	if len(v) != 0 {
		return "", fmt.Errorf("%s: a list default must be empty: []", path)
	}
	elem, err := elementTypeExpr(a)
	if err != nil {
		return "", err
	}
	kind := strings.TrimSuffix(a.Kind, "Nested")
	return fmt.Sprintf("types.%sValueMust(%s, []attr.Value{})", kind, elem), nil
}

// objectDefaultExpr is the value of an object default: each name is an attribute of the object,
// and the attributes that it leaves out are null.
func objectDefaultExpr(a *tfAttr, v map[string]any, path string, file *overrides.File) (string, error) {
	if a.Kind != "SingleNested" {
		return "", fmt.Errorf("%s: an object default fits an object attribute, not a %s", path, a.Kind)
	}
	for name := range v {
		if !slices.ContainsFunc(a.Attributes, func(c *tfAttr) bool { return c.Name == name }) {
			names := make([]string, 0, len(a.Attributes))
			for _, c := range a.Attributes {
				names = append(names, c.Name)
			}
			slices.Sort(names)
			return "", fmt.Errorf("%s: %s is no attribute of the object (attributes: %s)", path, name, strings.Join(names, ", "))
		}
	}
	typ, err := attrTypesMapExpr(a.Attributes)
	if err != nil {
		return "", err
	}
	sorted := slices.Clone(a.Attributes)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Name < sorted[j].Name })
	parts := make([]string, 0, len(sorted))
	for _, child := range sorted {
		childPath := path + "." + child.Name
		given := v[child.Name]
		expr, err := defaultValueExpr(child, given, childPath, file)
		if err != nil {
			return "", err
		}
		if given == nil && child.Required {
			return "", fmt.Errorf("%s is required: the default must give it a value", childPath)
		}
		if err := checkChildDefault(child, expr, childPath, file); err != nil {
			return "", err
		}
		parts = append(parts, fmt.Sprintf("%q: %s", child.Name, expr))
	}
	return fmt.Sprintf("types.ObjectValueMust(%s, map[string]attr.Value{%s})", typ, strings.Join(parts, ", ")), nil
}

// checkChildDefault reports an attribute whose own default differs from the value that the default
// of its object gives it: an omitted object would plan one value, and an object without the
// attribute another.
func checkChildDefault(child *tfAttr, objectValue, path string, file *overrides.File) error {
	if child.DefaultRaw == nil {
		return nil
	}
	own, err := defaultValueExpr(child, child.DefaultRaw, path, file)
	if err != nil || own == objectValue {
		return err
	}
	return fmt.Errorf("%s has its own default (%s) that differs from the value that the default of its object gives it (%s): name the same value in both", path, own, objectValue)
}

// scalarDefaultExpr is the value of a scalar default inside an object, checked against the type of
// the attribute, the values of its enum, and the validators of its line.
func scalarDefaultExpr(a *tfAttr, value any, path string, file *overrides.File) (string, error) {
	if a.CustomType != "" {
		return "", fmt.Errorf("%s: the attribute has the custom type %s, whose default can only be null", path, a.CustomType)
	}
	line := file.Types[a.Component].Fields[a.Property]
	switch v := value.(type) {
	case string:
		if a.Kind != "String" {
			break
		}
		if err := checkStringDefault(a, v, line, path, file); err != nil {
			return "", err
		}
		return fmt.Sprintf("types.StringValue(%q)", v), nil
	case bool:
		if a.Kind != "Bool" {
			break
		}
		if a.TrueOnly && !v {
			return "", fmt.Errorf("%s: only true is a valid value of this attribute", path)
		}
		return fmt.Sprintf("types.BoolValue(%t)", v), nil
	case int:
		return intValueExpr(a, float64(v), strconv.Itoa(v), line, path)
	case float64:
		if a.Kind != "Float64" && a.Kind != "Float32" {
			break
		}
		return floatValueExpr(a, v, line, path)
	}
	return "", fmt.Errorf("%s: a default of type %T does not fit a %s attribute", path, value, a.Kind)
}

func intValueExpr(a *tfAttr, v float64, literal string, line overrides.Field, path string) (string, error) {
	switch a.Kind {
	case "Int64", "Int32":
		if a.Kind == "Int32" && (v < math.MinInt32 || v > math.MaxInt32) {
			return "", fmt.Errorf("%s: the default %s does not fit an Int32 attribute", path, literal)
		}
		if err := checkNumberDefault(v, line, path); err != nil {
			return "", err
		}
		return fmt.Sprintf("types.%sValue(%s)", a.Kind, literal), nil
	case "Float64", "Float32":
		return floatValueExpr(a, v, line, path)
	}
	return "", fmt.Errorf("%s: a default of type int does not fit a %s attribute", path, a.Kind)
}

func floatValueExpr(a *tfAttr, v float64, line overrides.Field, path string) (string, error) {
	bits := 64
	if a.Kind == "Float32" {
		bits = 32
	}
	if math.IsNaN(v) || math.IsInf(v, 0) || bits == 32 && math.Abs(v) > math.MaxFloat32 {
		return "", fmt.Errorf("%s: the default %v does not fit a %s attribute", path, v, a.Kind)
	}
	if err := checkNumberDefault(v, line, path); err != nil {
		return "", err
	}
	return fmt.Sprintf("types.%sValue(%s)", a.Kind, strconv.FormatFloat(v, 'f', -1, bits)), nil
}

func checkNumberDefault(v float64, line overrides.Field, path string) error {
	for _, validator := range line.Validators {
		if len(validator.Between) == 2 && (v < validator.Between[0] || v > validator.Between[1]) {
			return fmt.Errorf("%s: the default %v is outside the between validator of the line [%v, %v]", path, v, validator.Between[0], validator.Between[1])
		}
		if validator.AtLeast != nil && v < *validator.AtLeast {
			return fmt.Errorf("%s: the default %v is below the atLeast validator of the line (%v)", path, v, *validator.AtLeast)
		}
	}
	return nil
}

func checkStringDefault(a *tfAttr, v string, line overrides.Field, path string, file *overrides.File) error {
	if enum, ok := file.Enums[a.EnumSchema]; ok && a.EnumSchema != "" {
		var allowed []string
		if enum.Zero != "" && !enum.ZeroNull {
			allowed = append(allowed, enum.Zero)
		}
		for _, api := range enum.Values {
			allowed = append(allowed, enum.TFValue(api))
		}
		if !slices.Contains(allowed, v) {
			slices.Sort(allowed)
			return fmt.Errorf("%s: %q is not a Terraform value of the enum %s (values: %s)", path, v, a.EnumSchema, strings.Join(allowed, ", "))
		}
	}
	for _, validator := range line.Validators {
		if err := checkStringValidator(validator, v, path); err != nil {
			return err
		}
	}
	return nil
}

// checkStringValidator checks a string default against one validator of the line of its field.
func checkStringValidator(validator overrides.Validator, v, path string) error {
	length := utf8.RuneCountInString(v)
	switch {
	case len(validator.OneOf) != 0 && !slices.Contains(validator.OneOf, v):
		return fmt.Errorf("%s: %q is not in the oneOf validator of the line (%s)", path, v, strings.Join(validator.OneOf, ", "))
	case validator.LengthAtLeast != nil && length < *validator.LengthAtLeast:
		return fmt.Errorf("%s: %q is shorter than the lengthAtLeast validator of the line (%d)", path, v, *validator.LengthAtLeast)
	case validator.LengthAtMost != nil && length > *validator.LengthAtMost:
		return fmt.Errorf("%s: %q is longer than the lengthAtMost validator of the line (%d)", path, v, *validator.LengthAtMost)
	case validator.RegexMatches != nil:
		re, err := regexp.Compile(validator.RegexMatches.Pattern)
		if err != nil {
			return errors.New("the regexMatches validator of the line does not compile: " + err.Error())
		}
		if !re.MatchString(v) {
			return fmt.Errorf("%s: %q does not match the regexMatches validator of the line (%s)", path, v, validator.RegexMatches.Pattern)
		}
	}
	return nil
}
