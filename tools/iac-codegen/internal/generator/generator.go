// Package generator validates and generates complete Terraform resources.
package generator

import (
	"errors"
	"fmt"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/issue"
	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/model"
	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/overrides"
	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/source"
	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/version"
	"golang.org/x/tools/go/packages"
)

// Options are the explicit resource generation choices.
type Options struct {
	Resource     string
	OutputDir    string
	OperationIDs model.OperationIDs
	// OverridesPath is the behavior-overrides file of a resource that users
	// already have. "" uses the file in OutputDir, when it exists.
	OverridesPath string
	// AcceptancePath is the acceptance file. "" uses the file in OutputDir, when it exists. With a
	// file, the generator also writes acceptance_test.go.
	AcceptancePath string
	// OpenAPIPath replaces the pinned SDK OpenAPI document. Go types still come from the
	// SDK version in the provider go.mod. Use it to render a candidate contract before that
	// document is published in the SDK.
	OpenAPIPath string
}

// CheckOptions selects one resource in a local candidate OpenAPI document.
type CheckOptions struct {
	Resource     string
	OpenAPIPath  string
	OperationIDs model.OperationIDs
	// OverridesPath is the behavior-overrides file of a resource that users already have.
	OverridesPath string
	// AcceptancePath is the acceptance file. With it, Check also builds and renders the
	// acceptance test, as Generate does.
	AcceptancePath string
}

type validatedResource struct {
	resource  *model.Resource
	refs      []sdkRef
	overrides *overrides.File // nil for a new resource
	// withoutDelete is the resource validated without the Delete override of the overrides, when
	// that also passes every OpenAPI and renderer check. The override is then stale unless the SDK
	// symbol checks of generate reject it. It is nil otherwise.
	withoutDelete *validatedResource
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
	// A missing candidate is reported before the pinned SDK is read. Generation
	// against a candidate still needs that SDK, but the file check does not.
	var candidate []byte
	if options.OpenAPIPath != "" {
		data, err := os.ReadFile(options.OpenAPIPath)
		if err != nil {
			return fmt.Errorf("read candidate OpenAPI %s: %w", options.OpenAPIPath, err)
		}
		candidate = data
	}
	input, err := source.Resolve(start)
	if err != nil {
		return err
	}
	if candidate != nil {
		input.OpenAPI = candidate
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
	file, err := readOverrides(options.OverridesPath)
	if err != nil {
		return err
	}
	validated, err := validateOpenAPIWith(data, options.Resource, options.OperationIDs, "candidate.invalid/coralogix-management-sdk", source.ProviderModule, file)
	if err != nil {
		return err
	}
	if validated.withoutDelete != nil {
		// Check has no SDK. As for every SDK symbol, it assumes that the SDK matches the contract.
		return unneededDeleteOverrideError(validated)
	}
	acc, err := readAcceptance(Options{Resource: options.Resource, AcceptancePath: options.AcceptancePath})
	if err != nil || acc == nil {
		return err
	}
	prior, err := readUpgrade(filepath.Dir(options.AcceptancePath))
	if err != nil {
		return err
	}
	if _, err := renderAll(validated.resource, validated.refs, "generated", validated.overrides, acc, prior, source.ProviderModule); err != nil {
		return eligibilityIssue("ACCEPTANCE_INVALID", "renderer.acceptance", err,
			"Fix the acceptance file, or set the value that the test cannot make up.")
	}
	return nil
}

// generateFromInput is the internal test seam for synthetic OpenAPI and a
// small handwritten SDK. It is not available through the CLI.
func generateFromInput(options Options, input source.Input, loadDir string) error {
	pkg, err := validateOptions(options)
	if err != nil {
		return err
	}
	file, err := readOverrides(overridesPath(options))
	if err != nil {
		return err
	}
	validated, err := validateOpenAPIWith(input.OpenAPI, options.Resource, options.OperationIDs, input.SDKModule, input.ProviderModule, file)
	if err != nil {
		return err
	}
	if err := checkSDK(validated, input, loadDir); err != nil {
		return err
	}
	accFile, err := readAcceptance(options)
	if err != nil {
		return err
	}
	if err := checkUpgradeLocation(options, accFile); err != nil {
		return err
	}
	prior, err := readUpgrade(options.OutputDir)
	if err != nil {
		return err
	}
	files, err := renderAll(validated.resource, validated.refs, pkg, validated.overrides, accFile, prior, input.ProviderModule)
	if err != nil {
		return fmt.Errorf("render resource: %w", err)
	}
	if err := checkVersionHeaders(files); err != nil {
		return err
	}
	if err := keepOverrides(options, files); err != nil {
		return err
	}
	if err := keepAcceptance(options, files); err != nil {
		return err
	}
	return publish(options.OutputDir, files)
}

// checkSDK checks every SDK symbol that the generated code uses. A Delete override is stale when
// the resource without it also passes these checks.
func checkSDK(validated *validatedResource, input source.Input, loadDir string) error {
	refs := validated.refs
	if validated.withoutDelete != nil {
		refs = slices.Concat(refs, validated.withoutDelete.refs)
	}
	loaded, err := loadSDK(refs, loadDir)
	if err != nil {
		return err
	}
	if report := sdkIssues(validated.refs, loaded, input); len(report) != 0 {
		return &EligibilityError{Report: report.Normalize()}
	}
	if validated.withoutDelete != nil && len(sdkIssues(validated.withoutDelete.refs, loaded, input)) == 0 {
		// Without the override, the SDK also has every symbol: the DELETE can be the Delete.
		return unneededDeleteOverrideError(validated)
	}
	return nil
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
	return validateOpenAPIWith(data, resourceName, operationIDs, sdkModule, providerModule, nil)
}

// validateOpenAPIWith is validateOpenAPI for a resource with a behavior-overrides
// file. file is nil for a new resource.
func validateOpenAPIWith(data []byte, resourceName string, operationIDs model.OperationIDs, sdkModule, providerModule string, file *overrides.File) (*validatedResource, error) {
	var policy model.Policy
	if file != nil {
		policy = file.Policy()
	}
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
	if file != nil {
		if report = overrideIssues(doc, component, file); len(report) != 0 {
			return nil, &EligibilityError{Report: report}
		}
	}
	if report = model.ValidateWithPolicy(doc, component, operationIDs, policy); len(report) != 0 {
		return nil, &EligibilityError{Report: report}
	}
	resource, err := model.BuildWithPolicy(doc, component, operationIDs, policy)
	if err != nil {
		return nil, fmt.Errorf("build validated resource: %w", err)
	}
	if report = unwrapUsageIssues(resource, file); len(report) != 0 {
		return nil, &EligibilityError{Report: report}
	}
	tag, err := resourceTag(doc, resource)
	if err != nil {
		return nil, eligibilityIssue("OPERATION_TAG_INCOMPATIBLE", "paths", err, "Give every lifecycle operation exactly one matching SDK package tag.")
	}
	refs, err := resolveSDKNames(resource, tag, sdkModule, providerModule)
	if err != nil {
		return nil, eligibilityIssue("SDK_SHAPE_UNSUPPORTED", "resource", err, "Use OpenAPI shapes that have deterministic generated Go SDK names and types.")
	}
	if report = rendererIssues(resource, refs, file); len(report) != 0 {
		return nil, &EligibilityError{Report: report.Normalize()}
	}
	return &validatedResource{resource: resource, refs: refs, overrides: file,
		withoutDelete: validateWithoutDelete(data, resourceName, operationIDs, sdkModule, providerModule, file)}, nil
}

// validateWithoutDelete validates the contract again without the Delete override. It returns the
// result when every check passes: a DELETE of the API can then be the Delete. Any issue, a DELETE
// that the generator cannot use or no DELETE at all, means that the override is needed, and it
// returns nil.
func validateWithoutDelete(data []byte, resourceName string, operationIDs model.OperationIDs, sdkModule, providerModule string, file *overrides.File) *validatedResource {
	if file == nil || file.API.Delete == nil {
		return nil
	}
	without := *file
	without.API.Delete = nil
	validated, err := validateOpenAPIWith(data, resourceName, operationIDs, sdkModule, providerModule, &without)
	if err != nil {
		return nil
	}
	return validated
}

// unneededDeleteOverrideError reports a Delete override that generation does not need. A stale
// override must not hide a DELETE of the API.
func unneededDeleteOverrideError(validated *validatedResource) error {
	return &EligibilityError{Report: issue.Report{{
		Code:        "DELETE_OVERRIDE_UNNEEDED",
		Location:    overrides.FileName + ":api.delete.operation",
		Message:     fmt.Sprintf("The overrides name the Delete operation %q, but the resource is eligible without it: the API has the DELETE operation %s.", validated.overrides.API.Delete.Operation, validated.withoutDelete.resource.Delete.OperationID),
		Remediation: "Delete api.delete from the overrides: the generated resource uses the DELETE.",
	}}}
}

// rendererIssues runs the same builders that generate uses, without rendering
// or writing files. This keeps candidate checks aligned with generation when a
// valid model or deterministic SDK shape still cannot become Terraform code.
func rendererIssues(resource *model.Resource, refs []sdkRef, file *overrides.File) issue.Report {
	checks := []struct {
		location string
		run      func() error
	}{
		{"renderer.schema", func() error { _, err := buildTFResourceWith(resource, "generated", file); return err }},
		{"renderer.conversion", func() error { _, err := buildConvWith(resource, refs, file); return err }},
		{"renderer.crud", func() error { _, err := buildCRUDWith(resource, refs, file); return err }},
	}
	var report issue.Report
	for _, check := range checks {
		if err := check.run(); err != nil {
			report = append(report, shapeIssue(check.location, err))
		}
	}
	if len(report) != 0 {
		return report
	}
	if _, err := renderWith(resource, refs, "generated", file); err != nil {
		report = append(report, issue.Issue{
			Code:        "RENDERER_OUTPUT_INVALID",
			Location:    "renderer.output",
			Message:     err.Error(),
			Remediation: "Use names and shapes that render as valid formatted Go code.",
		})
	}
	return report
}

func shapeIssue(location string, err error) issue.Issue {
	if errors.Is(err, errPriorContainer) {
		return issue.Issue{
			Code:        "OVERRIDE_PRIOR_CONTAINER",
			Location:    location,
			Message:     err.Error(),
			Remediation: "Delete the equality or keepPriorOrder key from the fields of the object that the map or the computed object holds.",
		}
	}
	if errors.Is(err, errUnwrapCombination) {
		return issue.Issue{
			Code:        "UNWRAP_COMBINATION_UNSUPPORTED",
			Location:    location,
			Message:     err.Error(),
			Remediation: "Delete the keepPriorOrder or equality key, or the unwrap line of the field.",
		}
	}
	if errors.Is(err, errStaleOverrideKey) {
		return issue.Issue{
			Code:        "OVERRIDE_UNUSED",
			Location:    location,
			Message:     err.Error(),
			Remediation: "Delete only the named key from the line.",
		}
	}
	return issue.Issue{
		Code:        "RENDERER_SHAPE_UNSUPPORTED",
		Location:    location,
		Message:     err.Error(),
		Remediation: "Use a resource shape that the generic Terraform renderer supports.",
	}
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
		if filepath.Ext(name) != ".go" {
			continue
		}
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
