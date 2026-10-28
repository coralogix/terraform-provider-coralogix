package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/coralogix/coralogix-management-sdk/iac/issue"
	"github.com/coralogix/coralogix-management-sdk/iac/model"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
)

func TestResolveResourceName(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "openapi.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	doc, err := model.Load(data)
	if err != nil {
		t.Fatal(err)
	}
	for _, selection := range []string{"Thing", "thing"} {
		name, report := resolveResource(doc, selection)
		if len(report) != 0 || name != "Thing" {
			t.Errorf("resolveResource(%q) = %q, %v", selection, name, report)
		}
	}
}

func TestTerraformNameKeepsAcronymsTogether(t *testing.T) {
	for input, want := range map[string]string{
		"HTTPServer":  "http_server",
		"httpServer":  "http_server",
		"sqlReadOnly": "sql_read_only",
		"foo-bar":     "foo_bar",
		"foo.bar":     "foo_bar",
		"1st":         "_1st",
		"_private":    "_private",
		"café":        "caf_",
	} {
		if got := TerraformName(input); got != want {
			t.Errorf("TerraformName(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestTerraformNameCollisionIsIneligible(t *testing.T) {
	for name, properties := range map[string]string{
		"separator": "        foo-bar: {type: string}\n        foo_bar: {type: string}\n",
		"acronym":   "        HTTPServer: {type: string}\n        httpServer: {type: string}\n",
	} {
		t.Run(name, func(t *testing.T) {
			doc := loadComponents(t, "    Collision:\n      type: object\n      properties:\n"+properties)
			if codes := issueCodes(terraformContractIssues(doc, "Collision", model.OperationIDs{}, model.Policy{}, nil)); !slices.Contains(codes, "TERRAFORM_NAME_COLLISION") {
				t.Fatalf("codes %v do not contain TERRAFORM_NAME_COLLISION", codes)
			}
		})
	}
}

func TestUint64RequiresTerraformInt64Bound(t *testing.T) {
	tests := map[string]struct {
		schema   string
		eligible bool
	}{
		"missing bound":  {schema: "type: string, format: uint64"},
		"18 digits":      {schema: "type: string, format: uint64, maxLength: 18", eligible: true},
		"19 digits":      {schema: "type: string, format: uint64, maxLength: 19"},
		"integer number": {schema: "type: integer, format: uint64"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			doc := loadComponents(t, fmt.Sprintf("    Holder:\n      type: object\n      properties:\n        counter: {%s}\n", test.schema))
			report := terraformContractIssues(doc, "Holder", model.OperationIDs{}, model.Policy{}, nil)
			if slices.Contains(issueCodes(report), "UINT64_RANGE_UNSUPPORTED") == test.eligible {
				t.Fatalf("report = %v, eligible = %t", report, test.eligible)
			}
		})
	}
}

// The Terraform checks join the report of the contract checks, so one report lists every problem.
func TestTerraformChecksJoinTheContractReport(t *testing.T) {
	doc := loadComponents(t, `    Broken:
      type: object
      properties:
        fooBar: {type: string}
        foo_bar: {type: string}
        children:
          type: array
          items: {$ref: '#/components/schemas/Broken'}
`)
	report := terraformContractIssues(doc, "Broken", model.OperationIDs{}, model.Policy{}, nil)
	codes := issueCodes(report)
	for _, want := range []string{"GO_NAME_COLLISION", "SCHEMA_RECURSIVE", "TERRAFORM_NAME_COLLISION"} {
		if !slices.Contains(codes, want) {
			t.Errorf("codes %v do not contain %s", codes, want)
		}
	}
	if got := report.Normalize(); !slices.Equal(report, got) {
		t.Fatalf("report is not normalized:\n%v\nwant:\n%v", report, got)
	}
}

func loadComponents(t *testing.T, schemas string) *v3.Document {
	t.Helper()
	doc, err := model.Load([]byte("openapi: 3.0.3\ninfo: {title: test, version: 1.0.0}\npaths: {}\ncomponents:\n  schemas:\n" + schemas))
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func issueCodes(report issue.Report) []string {
	codes := make([]string, 0, len(report))
	for _, item := range report.Normalize() {
		codes = append(codes, item.Code)
	}
	return codes
}

// A name line renames the Terraform attribute of one field of one component.
func TestTerraformNameUsesTheNameLine(t *testing.T) {
	p := model.Policy{Names: map[string]string{"R.scheduleOperation": "operation"}}
	if got := terraformName(p, "R", "scheduleOperation"); got != "operation" {
		t.Fatalf("terraformName = %q, want operation", got)
	}
	if got := terraformName(p, "Other", "scheduleOperation"); got != "schedule_operation" {
		t.Fatalf("terraformName of another component = %q, want schedule_operation", got)
	}
}

// A terraformType: Int64 line keeps an int32 field as Int64.
func TestTerraformTypeUsesTheInt64Line(t *testing.T) {
	p := model.Policy{Int64: []string{"R.count"}}
	if terraformType(p, "R", "count") != "Int64" || terraformType(p, "R", "other") != "" || terraformType(p, "", "count") != "" {
		t.Fatal("terraformType does not follow the Int64 list")
	}
}
