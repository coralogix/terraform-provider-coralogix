// Package generator validates and generates complete Terraform resources.
package generator

import (
	"errors"
	"fmt"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/issue"
	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/model"
	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/source"
	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/version"
	"golang.org/x/tools/go/packages"
)

// Options are the explicit resource generation choices.
type Options struct {
	Resource     string
	OutputDir    string
	OperationIDs model.OperationIDs
}

// CheckOptions selects one resource in a local candidate OpenAPI document.
type CheckOptions struct {
	Resource     string
	OpenAPIPath  string
	OperationIDs model.OperationIDs
}

type validatedResource struct {
	resource *model.Resource
	refs     []sdkRef
}

// EligibilityError reports all reasons that a resource is not eligible.
type EligibilityError struct {
	Report issue.Report
}

func (e *EligibilityError) Error() string { return e.Report.Error() }

// Generate resolves the provider's pinned SDK and publishes one resource.
func Generate(start string, options Options) error {
	if _, err := validateOptions(options); err != nil {
		return err
	}
	input, err := source.Resolve(start)
	if err != nil {
		return err
	}
	return generateFromInput(options, input, input.ProviderRoot)
}

// Check validates a local candidate OpenAPI document without rendering files.
func Check(options CheckOptions) error {
	if options.Resource == "" {
		return errors.New("resource name is required")
	}
	if options.OpenAPIPath == "" {
		return errors.New("candidate OpenAPI path is required")
	}
	data, err := os.ReadFile(options.OpenAPIPath)
	if err != nil {
		return fmt.Errorf("read candidate OpenAPI %s: %w", options.OpenAPIPath, err)
	}
	_, err = validateOpenAPI(data, options.Resource, options.OperationIDs, "candidate.invalid/coralogix-management-sdk", source.ProviderModule)
	return err
}

// generateFromInput is the internal test seam for synthetic OpenAPI and a
// small handwritten SDK. It is not available through the CLI.
func generateFromInput(options Options, input source.Input, loadDir string) error {
	pkg, err := validateOptions(options)
	if err != nil {
		return err
	}
	validated, err := validateOpenAPI(input.OpenAPI, options.Resource, options.OperationIDs, input.SDKModule, input.ProviderModule)
	if err != nil {
		return err
	}
	loaded, err := loadSDK(validated.refs, loadDir)
	if err != nil {
		return err
	}
	if report := sdkIssues(validated.refs, loaded, input); len(report) != 0 {
		return &EligibilityError{Report: report.Normalize()}
	}
	files, err := render(validated.resource, validated.refs, pkg)
	if err != nil {
		return fmt.Errorf("render resource: %w", err)
	}
	if err := checkVersionHeaders(files); err != nil {
		return err
	}
	return publish(options.OutputDir, files)
}

func validateOptions(options Options) (string, error) {
	if options.Resource == "" {
		return "", errors.New("resource name is required")
	}
	if options.OutputDir == "" {
		return "", errors.New("output directory is required")
	}
	pkg := filepath.Base(filepath.Clean(options.OutputDir))
	if !token.IsIdentifier(pkg) || token.Lookup(pkg).IsKeyword() {
		return "", fmt.Errorf("output directory base %q is not a valid Go package name", pkg)
	}
	return pkg, nil
}

// validateOpenAPI is the single OpenAPI eligibility path for Check and Generate.
// sdkModule only supplies deterministic expected SDK import names. This function
// does not load Go packages.
func validateOpenAPI(data []byte, resourceName string, operationIDs model.OperationIDs, sdkModule, providerModule string) (*validatedResource, error) {
	doc, err := model.Load(data)
	if err != nil {
		code := "OPENAPI_INVALID"
		if strings.Contains(err.Error(), "$ref") || strings.Contains(err.Error(), "reference") {
			code = "REFERENCE_UNRESOLVED"
		}
		return nil, &EligibilityError{Report: issue.Report{{Code: code, Location: "openapi", Message: err.Error(), Remediation: "Correct the source OpenAPI document."}}}
	}
	component, report := model.ResolveResource(doc, resourceName)
	if len(report) != 0 {
		return nil, &EligibilityError{Report: report}
	}
	if report = model.Validate(doc, component, operationIDs); len(report) != 0 {
		return nil, &EligibilityError{Report: report}
	}
	resource, err := model.BuildWithOperationIDs(doc, component, operationIDs)
	if err != nil {
		return nil, fmt.Errorf("build validated resource: %w", err)
	}
	tag, err := resourceTag(doc, resource)
	if err != nil {
		return nil, eligibilityIssue("OPERATION_TAG_INCOMPATIBLE", "paths", err, "Give every lifecycle operation exactly one matching SDK package tag.")
	}
	refs, err := resolveSDKNames(resource, tag, sdkModule, providerModule)
	if err != nil {
		return nil, eligibilityIssue("SDK_SHAPE_UNSUPPORTED", "resource", err, "Use OpenAPI shapes that have deterministic generated Go SDK names and types.")
	}
	if report = rendererIssues(resource, refs); len(report) != 0 {
		return nil, &EligibilityError{Report: report.Normalize()}
	}
	return &validatedResource{resource: resource, refs: refs}, nil
}

// rendererIssues runs the same builders that generate uses, without rendering
// or writing files. This keeps candidate checks aligned with generation when a
// valid model or deterministic SDK shape still cannot become Terraform code.
func rendererIssues(resource *model.Resource, refs []sdkRef) issue.Report {
	checks := []struct {
		location string
		run      func() error
	}{
		{"renderer.schema", func() error { _, err := buildTFResource(resource, "generated"); return err }},
		{"renderer.conversion", func() error { _, err := buildConv(resource, refs); return err }},
		{"renderer.crud", func() error { _, err := buildCRUD(resource, refs); return err }},
	}
	var report issue.Report
	for _, check := range checks {
		if err := check.run(); err != nil {
			report = append(report, issue.Issue{
				Code:        "RENDERER_SHAPE_UNSUPPORTED",
				Location:    check.location,
				Message:     err.Error(),
				Remediation: "Use a resource shape that the generic Terraform renderer supports.",
			})
		}
	}
	if len(report) != 0 {
		return report
	}
	if _, err := render(resource, refs, "generated"); err != nil {
		report = append(report, issue.Issue{
			Code:        "RENDERER_OUTPUT_INVALID",
			Location:    "renderer.output",
			Message:     err.Error(),
			Remediation: "Use names and shapes that render as valid formatted Go code.",
		})
	}
	return report
}

func eligibilityIssue(code, location string, err error, remediation string) error {
	return &EligibilityError{Report: issue.Report{{Code: code, Location: location, Message: err.Error(), Remediation: remediation}}}
}

func sdkIssues(refs []sdkRef, loaded map[string]*packages.Package, input source.Input) issue.Report {
	var report issue.Report
	seenPackages := map[string]bool{}
	for _, ref := range refs {
		if seenPackages[ref.Pkg] {
			continue
		}
		seenPackages[ref.Pkg] = true
		pkg := loaded[ref.Pkg]
		if pkg == nil || pkg.Module == nil {
			continue
		}
		expectedModule, expectedVersion, expectedDir := input.SDKModule, input.SDKVersion, input.SDKDir
		remediation := "Load the Go SDK and OpenAPI from the same provider-pinned module version."
		if ref.Rule == ruleProviderClientSet {
			expectedModule, expectedVersion, expectedDir = input.ProviderModule, "", input.ProviderRoot
			remediation = "Load the provider clientset from the same provider checkout."
		}
		if pkg.Module.Path != expectedModule || expectedVersion != "" && pkg.Module.Version != expectedVersion || expectedDir != "" && !samePath(pkg.Module.Dir, expectedDir) {
			report = append(report, issue.Issue{Code: "SDK_SOURCE_MISMATCH", Location: ref.Pkg, Message: fmt.Sprintf("The loaded package comes from module %s %s at %s; expected %s %s at %s.", pkg.Module.Path, pkg.Module.Version, pkg.Module.Dir, expectedModule, expectedVersion, expectedDir), Remediation: remediation})
		}
	}
	for index := range refs {
		ref := &refs[index]
		if err := checkRef(ref, loaded[ref.Pkg]); err != nil {
			report = append(report, issue.Issue{Code: "SDK_SYMBOL_MISSING", Location: ref.Path, Message: fmt.Sprintf("SDK %s %s: %s", ref.Kind, ref.sdkName(), err), Remediation: "Regenerate and publish the SDK from the same OpenAPI contract, then pin that SDK version."})
		}
	}
	return report
}

func checkVersionHeaders(files map[string][]byte) error {
	for name, data := range files {
		if len(data) < len(version.Header) || string(data[:len(version.Header)]) != version.Header {
			return fmt.Errorf("render resource: %s lacks generator version header", name)
		}
	}
	return nil
}

func samePath(a, b string) bool {
	a, aErr := filepath.EvalSymlinks(a)
	b, bErr := filepath.EvalSymlinks(b)
	return aErr == nil && bErr == nil && filepath.Clean(a) == filepath.Clean(b)
}
