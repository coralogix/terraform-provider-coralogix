package generator

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/acceptance"
	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/version"
)

// accCheck is one attribute check of an acceptance step.
type accCheck struct{ Path, Value string }

// Expr is the Go expression of the expected value. A value with a placeholder is rendered at run time.
func (c accCheck) Expr() string {
	if strings.Contains(c.Value, "@{") {
		return "render(run, " + strconv.Quote(c.Value) + ")"
	}
	return strconv.Quote(c.Value)
}

// accStep is one config of the acceptance test and what the test checks after the apply.
type accStep struct {
	Config string     // the resource block, HCL with @{run} and @{env.NAME}
	Checks []accCheck // attributes that must have the value
	Absent []string   // top-level attributes that must not be in the state
}

// acceptanceData is the template data of the generated acceptance test.
type acceptanceData struct {
	Package, VersionHeader string
	ProviderImport         string // import path of the provider package
	Name                   string // resource name, for the names of the tests
	Address                string // address of the resource in the config
	IDAttr                 string
	IDCustom               bool // the id attribute is not "id": the import step names it
	Env                    []string
	Prerequisites          string
	Full, Updated, Minimal *accStep
	UpgradeFrom            string
	UpgradeEnv             string
}

type accMode int

const (
	accFull accMode = iota
	accUpdated
	accMinimal
)

// accSynth makes the configs from the schema of the resource. A value comes from the acceptance
// file, or the synthesizer makes a plain one from the kind of the attribute.
type accSynth struct {
	file   *acceptance.File
	idAttr string
	known  map[string]bool // every attribute path that the walk saw
	used   map[string]bool // every value path that a config used
}

var quoted = regexp.MustCompile(`"([^"]*)"`)

// buildAcceptance makes the data of the acceptance test of a resource.
func buildAcceptance(res *tfResource, providerModule string, file *acceptance.File) (*acceptanceData, error) {
	if len(res.ConfigValidators) != 0 {
		return nil, errors.New("acceptance test: a oneOf group among the top-level fields is not supported")
	}
	typeName := res.CRUD.TypeName
	s := &accSynth{file: file, idAttr: res.CRUD.IDAttr, known: map[string]bool{}, used: map[string]bool{}}
	out := &acceptanceData{
		Package:        res.Package,
		VersionHeader:  version.Header,
		ProviderImport: providerModule + "/internal/provider",
		Name:           res.CRUD.Resource,
		Address:        "coralogix_" + typeName + ".test",
		IDAttr:         res.CRUD.IDAttr,
		IDCustom:       res.CRUD.IDAttr != "id",
		Env:            file.Env,
		Prerequisites:  strings.TrimSpace(file.Prerequisites),
		UpgradeFrom:    file.UpgradeFrom,
		UpgradeEnv:     "CORALOGIX_" + strings.ToUpper(typeName) + "_UPGRADE_ACC",
	}
	for _, step := range []struct {
		mode accMode
		into **accStep
	}{{accFull, &out.Full}, {accUpdated, &out.Updated}, {accMinimal, &out.Minimal}} {
		body, checks, err := s.attrs(res.Attributes, "", "", step.mode, true)
		if err != nil {
			return nil, fmt.Errorf("acceptance test: %w", err)
		}
		*step.into = &accStep{Config: resourceBlock("coralogix_"+typeName, body), Checks: checks}
	}
	out.Minimal.Absent = s.absent(res.Attributes)
	if err := s.checkFile(); err != nil {
		return nil, err
	}
	return out, nil
}

// checkFile fails when the acceptance file names a field that the walk did not see, or a value
// that no config used. A stale line cannot stay.
func (s *accSynth) checkFile() error {
	for name, paths := range map[string][]string{"skip": s.file.Skip, "minimal": s.file.Minimal} {
		for _, path := range paths {
			if !s.known[path] {
				return fmt.Errorf("%s: %q is not an attribute of the resource", name, path)
			}
		}
	}
	for _, path := range s.file.ValuePaths() {
		if !s.used[path] {
			return fmt.Errorf("values: %q is not set by the full config: it names no attribute, or the attribute is skipped, computed, or in a oneOf arm that the test leaves out", path)
		}
	}
	return nil
}

func resourceBlock(resourceType, body string) string {
	if body == "" {
		return fmt.Sprintf("resource %q \"test\" {\n}", resourceType)
	}
	return fmt.Sprintf("resource %q \"test\" {\n%s\n}", resourceType, indentLines(body))
}

func indentLines(text string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if line != "" {
			lines[i] = "  " + line
		}
	}
	return strings.Join(lines, "\n")
}

func joinPath(parent, name string) string {
	if parent == "" {
		return name
	}
	return parent + "." + name
}

// include says whether the config of the mode sets the attribute.
func (s *accSynth) include(a *tfAttr, key string, mode accMode, top bool) bool {
	switch {
	case slices.Contains(s.file.Skip, key):
		return false
	case !a.Required && !a.Optional:
		return false // the server sets it
	case top && a.Name == s.idAttr && !a.Required:
		return false // the server makes the id; a changed id would replace the resource
	case a.Required:
		return true
	}
	return mode != accMinimal || slices.Contains(s.file.Minimal, key)
}

// attrs makes the "name = value" lines of the attributes of one object, and the checks.
func (s *accSynth) attrs(attrs []*tfAttr, tfPath, key string, mode accMode, top bool) (string, []accCheck, error) {
	var lines []string
	var checks []accCheck
	groupArm := false
	for _, a := range attrs {
		attrKey := joinPath(key, a.Name)
		s.known[attrKey] = true
		if !s.include(a, attrKey, mode, top) {
			continue
		}
		if len(a.GroupValidators) != 0 && !a.Required {
			// A oneOf group takes one arm. The test sets the first.
			if groupArm {
				continue
			}
			groupArm = true
		}
		value, c, err := s.value(a, joinPath(tfPath, a.Name), attrKey, mode)
		if err != nil {
			return "", nil, fmt.Errorf("%s: %w", attrKey, err)
		}
		lines = append(lines, a.Name+" = "+value)
		checks = append(checks, c...)
	}
	return strings.Join(lines, "\n"), checks, nil
}

// absent lists the top-level attributes that the minimal config leaves out and that the server
// does not set: the state must not have them.
func (s *accSynth) absent(attrs []*tfAttr) []string {
	var out []string
	for _, a := range attrs {
		scalar := slices.Contains([]string{"String", "Bool", "Int64", "Int32", "Float64", "Float32"}, a.Kind)
		if scalar && a.Optional && !a.Computed && !s.include(a, a.Name, accMinimal, true) {
			out = append(out, a.Name)
		}
	}
	return out
}

func immutable(a *tfAttr) bool {
	return slices.ContainsFunc(a.Modifiers, func(m string) bool { return strings.Contains(m, "RequiresReplace") })
}

// value makes the HCL value of an attribute and the checks on it.
func (s *accSynth) value(a *tfAttr, tfPath, key string, mode accMode) (string, []accCheck, error) {
	if mode == accMinimal && slices.Contains(s.file.Minimal, key) {
		mode = accFull
	}
	if immutable(a) && mode == accUpdated {
		mode = accFull // a changed immutable attribute replaces the resource
	}
	if v, ok := s.file.Values[key]; ok {
		s.used[key] = true
		return strings.TrimSpace(v), nil, nil
	}
	switch a.Kind {
	case "String", "Bool", "Int64", "Int32", "Float64", "Float32":
		v, check := scalarValue(a, mode)
		return v, []accCheck{{tfPath, check}}, nil
	case "Map", "List", "Set":
		return s.collectionValue(a, tfPath, mode)
	case "SingleNested":
		body, checks, err := s.attrs(a.Attributes, tfPath, key, mode, false)
		if err != nil {
			return "", nil, err
		}
		return objectHCL(body), checks, nil
	case "ListNested", "SetNested":
		return s.nestedCollection(a, tfPath, key, mode)
	}
	return "", nil, fmt.Errorf("the %s attribute kind is not supported: set the value in %s", a.Kind, acceptance.FileName)
}

func objectHCL(body string) string {
	if body == "" {
		return "{}"
	}
	return "{\n" + indentLines(body) + "\n}"
}

func (s *accSynth) nestedCollection(a *tfAttr, tfPath, key string, mode accMode) (string, []accCheck, error) {
	element := tfPath + ".0"
	if a.Kind == "SetNested" {
		element = tfPath + ".*" // the position of a set element is not known: no element checks
	}
	body, checks, err := s.attrs(a.Attributes, element, key+"[]", mode, false)
	if err != nil {
		return "", nil, err
	}
	if a.Kind == "SetNested" {
		checks = nil
	}
	checks = append([]accCheck{{tfPath + ".#", "1"}}, checks...)
	return "[\n" + indentLines(objectHCL(body)+",") + "\n]", checks, nil
}

// scalarValue makes a plain value of a scalar attribute and the string that the state holds.
func scalarValue(a *tfAttr, mode accMode) (hcl, state string) {
	updated := mode == accUpdated
	switch a.Kind {
	case "Bool":
		v := !updated
		return strconv.FormatBool(v), strconv.FormatBool(v)
	case "Int64", "Int32":
		n := 1
		if updated {
			n = 2
		}
		return strconv.Itoa(n), strconv.Itoa(n)
	case "Float64", "Float32":
		if updated {
			return "2.5", "2.5"
		}
		return "1.5", "1.5"
	}
	v := stringValue(a, updated)
	return strconv.Quote(v), v
}

// stringValue is the first accepted value of an enum, or a plain unique string. An enum keeps its
// value in the update config: the valid values of other fields can depend on it (a rule condition
// depends on the entity type), and the test cannot know how.
func stringValue(a *tfAttr, updated bool) string {
	if values := enumValues(a); len(values) != 0 {
		return values[0]
	}
	if updated {
		return "@{run}-" + a.Name + "-updated"
	}
	return "@{run}-" + a.Name
}

// enumValues returns the values of the OneOf validator of a string attribute, without the zero
// value "unspecified" when others exist.
func enumValues(a *tfAttr) []string {
	var values []string
	for _, v := range a.Validators {
		if !strings.HasPrefix(v, "stringvalidator.OneOf(") {
			continue
		}
		for _, m := range quoted.FindAllStringSubmatch(v, -1) {
			values = append(values, m[1])
		}
	}
	if len(values) > 1 {
		values = slices.DeleteFunc(values, func(v string) bool { return v == "unspecified" })
	}
	return values
}

// collectionValue makes a map, list, or set of plain values. Only string, number, and bool
// elements are made up.
func (s *accSynth) collectionValue(a *tfAttr, tfPath string, mode accMode) (string, []accCheck, error) {
	elem, ok := map[string]string{
		"types.StringType": "String", "types.BoolType": "Bool", "types.Int64Type": "Int64",
		"types.Int32Type": "Int32", "types.Float64Type": "Float64", "types.Float32Type": "Float32",
	}[a.ElementType]
	if !ok {
		return "", nil, fmt.Errorf("elements of type %s are not supported: set the value in %s", a.ElementType, acceptance.FileName)
	}
	item, state := scalarValue(&tfAttr{Name: a.Name, Kind: elem}, mode)
	switch a.Kind {
	case "Map":
		return "{ key = " + item + " }", []accCheck{{tfPath + ".key", state}}, nil
	case "List":
		return "[" + item + "]", []accCheck{{tfPath + ".#", "1"}, {tfPath + ".0", state}}, nil
	}
	return "[" + item + "]", []accCheck{{tfPath + ".#", "1"}}, nil
}

// renderAcceptance returns the generated acceptance test.
func renderAcceptance(data *acceptanceData) ([]byte, error) {
	var buf strings.Builder
	if err := templates.ExecuteTemplate(&buf, "acceptance_test.go.tmpl", data); err != nil {
		return nil, fmt.Errorf("acceptance_test.go: %w", err)
	}
	return formatGenerated("acceptance_test.go", []byte(buf.String()))
}

// readAcceptance reads the acceptance file. It returns nil when there is none.
func readAcceptance(options Options) (*acceptance.File, error) {
	path := acceptancePath(options)
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read acceptance file %s: %w", path, err)
	}
	file, err := acceptance.Parse(data)
	if err != nil {
		return nil, err
	}
	if file.Resource != options.Resource {
		return nil, fmt.Errorf("%s is for resource %q, but the selected resource is %q", acceptance.FileName, file.Resource, options.Resource)
	}
	return file, nil
}

func acceptancePath(options Options) string {
	if options.AcceptancePath != "" {
		return options.AcceptancePath
	}
	if options.OutputDir == "" {
		return ""
	}
	candidate := filepath.Join(options.OutputDir, acceptance.FileName)
	if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
		return candidate
	}
	return ""
}

// keepAcceptance puts the acceptance file of the output directory back, like keepOverrides.
func keepAcceptance(options Options, files map[string][]byte) error {
	path := acceptancePath(options)
	if path == "" || filepath.Base(path) != acceptance.FileName {
		return nil
	}
	inOutput, err := sameDirectory(filepath.Dir(path), options.OutputDir)
	if err != nil || !inOutput {
		return err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read acceptance file %s: %w", path, err)
	}
	files[acceptance.FileName] = data
	return nil
}
