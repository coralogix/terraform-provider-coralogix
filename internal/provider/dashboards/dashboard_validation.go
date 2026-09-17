// Copyright 2026 Coralogix Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package dashboards

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode"

	dashboardservice "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/dashboard_service"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	dashboardschema "github.com/coralogix/terraform-provider-coralogix/internal/provider/dashboards/dashboard_schema"
)

const (
	// dashboardValidationEnabledEnvVar turns plan-time dashboard validation off
	// when set to a false value. Validation is on by default.
	dashboardValidationEnabledEnvVar = "CORALOGIX_DASHBOARD_VALIDATION"
	// dashboardValidationTimeoutEnvVar overrides how long the validation call may
	// take before the plan gives up on it and continues.
	dashboardValidationTimeoutEnvVar = "CORALOGIX_DASHBOARD_VALIDATION_TIMEOUT"

	dashboardValidationDefaultTimeout = 5 * time.Second

	// dashboardValidationMaxReportedIssues caps how many issues become
	// warnings, so one badly broken dashboard cannot bury the rest of the plan.
	dashboardValidationMaxReportedIssues = 20
)

// dashboardValidationSettings is the plan-time validation configuration. It is
// read from the environment rather than from provider schema because the
// provider is muxed, and terraform-plugin-mux requires the provider schema to
// be identical across the SDKv2 and framework servers.
type dashboardValidationSettings struct {
	Enabled bool
	Timeout time.Duration
}

func dashboardValidationSettingsFromEnv() dashboardValidationSettings {
	settings := dashboardValidationSettings{
		Enabled: true,
		Timeout: dashboardValidationDefaultTimeout,
	}

	if raw := os.Getenv(dashboardValidationEnabledEnvVar); raw != "" {
		if enabled, err := strconv.ParseBool(raw); err == nil {
			settings.Enabled = enabled
		}
	}

	if raw := os.Getenv(dashboardValidationTimeoutEnvVar); raw != "" {
		if timeout, err := time.ParseDuration(raw); err == nil && timeout > 0 {
			settings.Timeout = timeout
		}
	}

	return settings
}

var _ resource.ResourceWithModifyPlan = &DashboardResource{}

// ModifyPlan reports backend validation issues as plan-time warnings.
//
// The create and replace endpoints apply structural validation only, so a
// dashboard that carries a stale variable reference or a duplicate widget id is
// stored without complaint and renders broken. The check endpoint reports those
// problems and persists nothing. Running it during the plan puts them in front
// of the user while the change is still free to fix.
//
// Validation is advisory. Every failure path warns and continues, so a plan is
// never blocked because validation was disabled, unavailable, slow, or unhappy.
func (r *DashboardResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.Plan.Raw.IsNull() {
		// Destroy plan. There is no dashboard left to validate.
		return
	}
	if !req.State.Raw.IsNull() && req.State.Raw.Equal(req.Plan.Raw) {
		// Nothing is changing, so there is nothing new to tell the user. This
		// keeps a plan over unchanged dashboards free of validation calls.
		return
	}
	if r.openAPIClient == nil {
		return
	}

	settings := dashboardValidationSettingsFromEnv()
	if !settings.Enabled {
		return
	}

	planSchema := dashboardschema.V4()
	isComputed := func(attributePath *tftypes.AttributePath) bool {
		attribute, err := planSchema.AttributeAtTerraformPath(ctx, attributePath)
		if err != nil {
			return false
		}
		return attribute.IsComputed()
	}
	resolves := func(attributePath *tftypes.AttributePath) bool {
		_, err := planSchema.AttributeAtTerraformPath(ctx, attributePath)
		return err == nil
	}
	unknownPaths, convertible := unknownUserValuePaths(req.Plan.Raw, isComputed)
	if !convertible {
		log.Print("[DEBUG] Skipping Dashboard validation: an unknown value sits at a path issues cannot be matched against")
		return
	}

	var plan DashboardResourceModel
	if diags := req.Plan.Get(ctx, &plan); diags.HasError() {
		log.Printf("[DEBUG] Skipping Dashboard validation: %v", diags.Errors())
		return
	}

	dashboard, diags := extractDashboard(ctx, plan)
	if diags.HasError() {
		// Create and Update report extraction failures as errors. Doing the
		// same here would fail the plan, so this only logs.
		log.Printf("[DEBUG] Skipping Dashboard validation: %v", diags.Errors())
		return
	}

	checkCtx, cancel := context.WithTimeout(ctx, settings.Timeout)
	defer cancel()

	issues, err := r.openAPIClient.Check(checkCtx, dashboard)
	if err != nil {
		log.Printf("[WARN] Dashboard validation failed: %s", err)
		resp.Diagnostics.AddWarning(
			"Could not validate Dashboard",
			fmt.Sprintf("The dashboard was not validated and the plan continues unchanged: %s\n\n"+
				"Set %s=false to stop validating dashboards, or %s to allow more time.",
				err, dashboardValidationEnabledEnvVar, dashboardValidationTimeoutEnvVar),
		)
		return
	}

	addDashboardIssueWarnings(&resp.Diagnostics, issues, unknownPaths, resolves)
}

// addDashboardIssueWarnings turns validation issues into plan warnings.
//
// Both severities become warnings. The backend reports issues that create and
// replace accept, so failing the plan on them would reject configurations that
// apply cleanly today, and a new backend check would break plans that nobody
// changed. The severity is kept in the summary so users can still tell the two
// apart.
//
// unknownPaths lists attributes Terraform could not resolve at plan time. An
// issue that touches one of them is dropped, because the value the backend
// judged is not the value the user wrote.
func addDashboardIssueWarnings(diagnostics *diag.Diagnostics, issues []dashboardservice.Issue, unknownPaths []path.Path, resolves func(*tftypes.AttributePath) bool) {
	reported := 0
	skipped := 0

	for _, issue := range issues {
		message := issue.GetMessage()
		if message == "" {
			continue
		}

		location := issue.GetLocation()
		attributePath, mapped := attributePathFromPointer(location, resolves)
		if issueTouchesUnknown(attributePath, mapped, unknownPaths) {
			log.Printf("[DEBUG] Dropping Dashboard validation issue at %q: it covers a value that is not known yet", location)
			skipped++
			continue
		}

		if reported == dashboardValidationMaxReportedIssues {
			diagnostics.AddWarning(
				"Dashboard validation issues omitted",
				fmt.Sprintf("%d more validation issues were found and are not listed here.", len(issues)-reported-skipped),
			)
			return
		}
		reported++

		summary := dashboardIssueSummary(issue.GetSeverity())
		if mapped {
			diagnostics.AddAttributeWarning(attributePath, summary, message)
			continue
		}

		if location != "" {
			message = fmt.Sprintf("%s: %s", location, message)
		}
		diagnostics.AddWarning(summary, message)
	}
}

func issueTouchesUnknown(attributePath path.Path, mapped bool, unknownPaths []path.Path) bool {
	if len(unknownPaths) == 0 {
		return false
	}
	if !mapped {
		// A dashboard level issue cannot be tied to an attribute, so there is
		// no way to tell whether an unknown value caused it.
		return true
	}

	for _, unknownPath := range unknownPaths {
		if pathsOverlap(attributePath, unknownPath) {
			return true
		}
	}

	return false
}

func dashboardIssueSummary(severity dashboardservice.IssueSeverity) string {
	if severity == dashboardservice.ISSUESEVERITY_SEVERITY_ERROR {
		return "Dashboard validation error"
	}
	return "Dashboard validation warning"
}

// unknownUserValuePaths lists the attributes the user controls whose value is
// not known yet, usually because they reference another resource that does not
// exist. A dashboard is still worth validating when some of them are unknown,
// but an issue reported against one of them may be about a value the user did
// set and Terraform simply cannot see, so those issues are dropped later.
//
// Unknowns on Computed attributes are not collected. Every dashboard, section,
// row and widget id is Computed and unknown on create, and expansion fills in
// the ones that matter.
//
// The second return value is false when a path cannot be expressed as a
// framework path. The caller then has no way to match issues against it and
// skips validation instead of reporting something it cannot vouch for.
func unknownUserValuePaths(plan tftypes.Value, isComputed func(*tftypes.AttributePath) bool) ([]path.Path, bool) {
	var unknown []path.Path
	convertible := true

	// The walk only errors when the callback errors, and this one never does.
	_ = tftypes.Walk(plan, func(attrPath *tftypes.AttributePath, value tftypes.Value) (bool, error) {
		if value.IsKnown() {
			return true, nil
		}
		if isComputed(attrPath) {
			// Skip the subtree: everything under a computed attribute is
			// unknown for the same reason.
			return false, nil
		}

		converted, ok := frameworkPathFromTerraformPath(attrPath)
		if !ok {
			convertible = false
			return false, nil
		}
		unknown = append(unknown, converted)

		return false, nil
	})

	return unknown, convertible
}

// frameworkPathFromTerraformPath converts a path from the low level plan value
// to the path type diagnostics use. It reports false for set elements, which
// are addressed by value and have no equivalent this code can build.
func frameworkPathFromTerraformPath(attrPath *tftypes.AttributePath) (path.Path, bool) {
	converted := path.Empty()
	for _, step := range attrPath.Steps() {
		switch typedStep := step.(type) {
		case tftypes.AttributeName:
			converted = converted.AtName(string(typedStep))
		case tftypes.ElementKeyInt:
			converted = converted.AtListIndex(int(typedStep))
		case tftypes.ElementKeyString:
			converted = converted.AtMapKey(string(typedStep))
		default:
			return path.Empty(), false
		}
	}

	return converted, true
}

// pathsOverlap reports whether two attribute paths refer to the same value or
// to values that contain one another. A widget whose reference is unknown makes
// an issue about that widget suspect; an issue about its sibling does not.
func pathsOverlap(left, right path.Path) bool {
	leftString, rightString := left.String(), right.String()
	if len(leftString) > len(rightString) {
		leftString, rightString = rightString, leftString
	}
	if leftString == rightString {
		return true
	}

	if !strings.HasPrefix(rightString, leftString) {
		return false
	}

	// Guard against "widgets[1]" matching the prefix of "widgets[10]".
	switch rightString[len(leftString)] {
	case '.', '[':
		return true
	default:
		return false
	}
}

// dashboardPointerAliases maps API field names onto the path the Terraform
// schema uses, for the fields whose shape differs between the two. Only the
// start of a pointer is aliased, which is where the reshaping happens.
//
// Anything else that diverges - dashboard level actions, which Terraform keeps
// per widget, or the auto refresh interval, which the API spells as a set of
// mutually exclusive fields - produces a path that is not in the schema and is
// rejected by the resolver, so the issue becomes a resource level warning.
var dashboardPointerAliases = map[string][]string{
	"folderId":   {"folder", "id"},
	"folderPath": {"folder", "path"},
}

// attributePathFromPointer converts an RFC 6901 JSON Pointer returned by the
// validation endpoint into a Terraform attribute path, so a warning lands on
// the offending block instead of on the resource as a whole.
//
//	/layout/sections/0/rows/0/widgets/1 -> layout.sections[0].rows[0].widgets[1]
//
// resolves reports whether a path exists in the resource schema. The API body
// and the Terraform schema do not always agree on shape, and a path that only
// looks plausible is worse than no path: it would neither point at anything a
// user can edit nor match an unknown value the issue should have been filtered
// against.
//
// It reports false for a pointer it cannot map, including the empty pointer the
// API uses for dashboard-level issues. The caller then warns without a path.
func attributePathFromPointer(pointer string, resolves func(*tftypes.AttributePath) bool) (path.Path, bool) {
	trimmed := strings.TrimPrefix(pointer, "/")
	if trimmed == "" {
		return path.Empty(), false
	}

	segments := strings.Split(trimmed, "/")
	if aliased, ok := dashboardPointerAliases[unescapePointerSegment(segments[0])]; ok {
		segments = append(append([]string{}, aliased...), segments[1:]...)
	}

	attributePath := path.Empty()
	terraformPath := tftypes.NewAttributePath()
	for _, segment := range segments {
		segment = unescapePointerSegment(segment)
		if segment == "" {
			return path.Empty(), false
		}

		if index, err := strconv.Atoi(segment); err == nil {
			if index < 0 {
				return path.Empty(), false
			}
			attributePath = attributePath.AtListIndex(index)
			terraformPath = terraformPath.WithElementKeyInt(index)
			continue
		}

		name := camelToSnake(segment)
		attributePath = attributePath.AtName(name)
		terraformPath = terraformPath.WithAttributeName(name)
	}

	if !resolves(trimTrailingElementKeys(terraformPath)) {
		return path.Empty(), false
	}

	return attributePath, true
}

// trimTrailingElementKeys drops trailing collection keys from a path. A list
// element has no schema of its own, so "layout.sections[0]" cannot be looked
// up, while the "layout.sections" it belongs to can.
func trimTrailingElementKeys(terraformPath *tftypes.AttributePath) *tftypes.AttributePath {
	steps := terraformPath.Steps()
	for len(steps) > 0 {
		if _, ok := steps[len(steps)-1].(tftypes.AttributeName); ok {
			break
		}
		steps = steps[:len(steps)-1]
	}

	return tftypes.NewAttributePathWithSteps(steps)
}

// unescapePointerSegment decodes the two escapes RFC 6901 defines, in the order
// the RFC requires: ~1 before ~0, so that an encoded tilde is not re-read.
func unescapePointerSegment(segment string) string {
	segment = strings.ReplaceAll(segment, "~1", "/")
	return strings.ReplaceAll(segment, "~0", "~")
}

// camelToSnake converts an API field name to the Terraform attribute name, for
// example variablesV2 to variables_v2 and folderId to folder_id.
func camelToSnake(name string) string {
	var builder strings.Builder
	builder.Grow(len(name) + 4)

	runes := []rune(name)
	for i, r := range runes {
		if unicode.IsUpper(r) {
			// A boundary is a lower-to-upper change, or the end of an acronym
			// such as the "ID" in "widgetIDs".
			previousIsLower := i > 0 && !unicode.IsUpper(runes[i-1]) && runes[i-1] != '_'
			nextIsLower := i+1 < len(runes) && unicode.IsLower(runes[i+1])
			if i > 0 && (previousIsLower || nextIsLower) {
				builder.WriteRune('_')
			}
			builder.WriteRune(unicode.ToLower(r))
			continue
		}

		builder.WriteRune(r)
	}

	return builder.String()
}
