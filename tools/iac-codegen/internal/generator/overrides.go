package generator

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/issue"
	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/overrides"
	"github.com/pb33f/libopenapi/datamodel/high/base"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"go.yaml.in/yaml/v4"
)

// overridesPath returns the file to read: the explicit path, else the file in the
// output directory when it exists, else "".
func overridesPath(options Options) string {
	if options.OverridesPath != "" {
		return options.OverridesPath
	}
	if options.OutputDir == "" {
		return ""
	}
	candidate := filepath.Join(options.OutputDir, overrides.FileName)
	if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
		return candidate
	}
	return ""
}

// keepOverrides adds the behavior-overrides file of the output directory to the files
// to publish. Publish replaces the whole directory, so the file must go with the new files.
// A file at another path stays where it is.
func keepOverrides(options Options, files map[string][]byte) error {
	path := overridesPath(options)
	if path == "" || filepath.Base(path) != overrides.FileName {
		return nil
	}
	inOutput, err := sameDirectory(filepath.Dir(path), options.OutputDir)
	if err != nil || !inOutput {
		return err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read overrides %s: %w", path, err)
	}
	files[overrides.FileName] = data
	return nil
}

func sameDirectory(a, b string) (bool, error) {
	absA, err := filepath.Abs(a)
	if err != nil {
		return false, err
	}
	absB, err := filepath.Abs(b)
	if err != nil {
		return false, err
	}
	return absA == absB, nil
}

// readOverrides reads the file. It returns nil for the empty path: a new resource.
func readOverrides(path string) (*overrides.File, error) {
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read overrides %s: %w", path, err)
	}
	file, err := overrides.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("overrides %s: %w", path, err)
	}
	return file, nil
}

// overrideIssues reports every line of the file that does not match the contract.
// A line that matches nothing is an error, so a stale line cannot stay in the file.
func overrideIssues(doc *v3.Document, component string, file *overrides.File) issue.Report {
	var report issue.Report
	if file.Resource != component {
		report = append(report, issue.Issue{
			Code:        "OVERRIDE_RESOURCE_MISMATCH",
			Location:    overrides.FileName,
			Message:     fmt.Sprintf("The file is for resource %q, but the selected resource is %q.", file.Resource, component),
			Remediation: "Use the behavior-overrides file of the selected resource.",
		})
	}
	for _, line := range file.Lines() {
		if problem := lineProblem(doc, line); problem != nil {
			report = append(report, issue.Issue{
				Code:        "OVERRIDE_UNUSED",
				Location:    overrides.FileName + ":" + line.String(),
				Message:     problem.Error(),
				Remediation: "Delete the line: the API contract no longer differs here, or fix the name.",
			})
		}
	}
	return report.Normalize()
}

func lineProblem(doc *v3.Document, line overrides.Line) error {
	if doc.Components == nil || doc.Components.Schemas == nil {
		return errors.New("the API contract has no component schemas")
	}
	proxy := doc.Components.Schemas.GetOrZero(line.Component)
	if proxy == nil {
		return fmt.Errorf("no component %q", line.Component)
	}
	schema, err := proxy.BuildSchema()
	if err != nil {
		return fmt.Errorf("component %q: %w", line.Component, err)
	}
	switch line.Kind {
	case overrides.KindEnum:
		return enumLineProblem(schema, line)
	case overrides.KindEmptyRequired:
		if !hasNoRequiredList(schema) {
			return fmt.Errorf("component %q declares a required list, so no override is needed", line.Component)
		}
	case overrides.KindField:
		return fieldLineProblem(schema, line)
	}
	return nil
}

func enumLineProblem(schema *base.Schema, line overrides.Line) error {
	if len(schema.Enum) == 0 {
		return fmt.Errorf("component %q is not an enum", line.Component)
	}
	for _, value := range line.EnumValues {
		if !slices.ContainsFunc(schema.Enum, func(n *yaml.Node) bool { return n.Value == value }) {
			return fmt.Errorf("enum %q has no value %q", line.Component, value)
		}
	}
	return nil
}

func fieldLineProblem(schema *base.Schema, line overrides.Line) error {
	if schema.Properties == nil || schema.Properties.GetOrZero(line.Field) == nil {
		return fmt.Errorf("component %q has no field %q", line.Component, line.Field)
	}
	if !line.ReadOnly {
		return nil
	}
	field, err := schema.Properties.GetOrZero(line.Field).BuildSchema()
	if err == nil && field.ReadOnly != nil && *field.ReadOnly {
		return fmt.Errorf("the contract already marks %q readOnly", line.Field)
	}
	return nil
}

func hasNoRequiredList(schema *base.Schema) bool {
	return schema.GoLow() == nil || schema.GoLow().Required.IsEmpty()
}
