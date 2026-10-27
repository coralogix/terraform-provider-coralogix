package generator

import (
	"fmt"
	"slices"
	"strings"

	"github.com/coralogix/coralogix-management-sdk/iac/issue"
	"github.com/coralogix/coralogix-management-sdk/iac/model"
	"github.com/coralogix/coralogix-management-sdk/iac/sdkcheck"
	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/overrides"
)

// dataSourceFile is the generated file of the data source in the output directory.
const dataSourceFile = "data_source.go"

// dsData is the template data of the data source. The data source reads the resource by id with the
// schema, the model, and the conversion of the resource.
type dsData struct {
	// UtilsPkg is the import path of the provider package that copies a resource schema to a data
	// source schema.
	UtilsPkg string
	// IDSchema: the data source sets the schema of the id attribute itself, because the copy makes
	// only a string attribute named "id" the required lookup attribute.
	IDSchema bool
	// ValueFields are the top-level strings that the data source shows as the API returns them,
	// also the empty string that the resource reads as null (readEmptyAs: "value").
	ValueFields []*dsValueField
}

// dsValueField is a top-level string of a readEmptyAs: "value" line.
type dsValueField struct {
	TFName, Model, SDK string
	Value              bool // the SDK field is a value, not a pointer
}

// buildDataSource returns the template data of the data source.
func buildDataSource(conv *convData, crud *crudData, file *overrides.File) (*dsData, error) {
	out := &dsData{
		UtilsPkg: strings.TrimSuffix(crud.ProviderPkg, "/clientset") + "/utils",
		IDSchema: crud.Singleton || crud.IDAttr != "id" || crud.IDTFType != "String",
	}
	for _, line := range file.DataSourceLines() {
		field := slices.IndexFunc(conv.Resource.Fields, func(f *convField) bool {
			return f.Model == camelize(line.Field) && f.Conv != convGroup
		})
		if field < 0 {
			return nil, fmt.Errorf("%s: %s is not a top-level attribute of the resource", line, line.Field)
		}
		f := conv.Resource.Fields[field]
		if f.Conv != convString || f.Wrap != nil || !f.ReadEmptyAsNull {
			return nil, fmt.Errorf(`%s: readEmptyAs: "value" is supported for a string that the resource reads with readEmptyAs: "null", not for the %s field %s`, line, f.Conv, f.TFName)
		}
		out.ValueFields = append(out.ValueFields, &dsValueField{TFName: f.TFName, Model: f.Model, SDK: f.SDK, Value: f.Value})
	}
	return out, nil
}

// dataSourceShape runs the builders of the data source, as generate does, without rendering.
func dataSourceShape(r *model.Resource, refs []sdkcheck.Ref, file *overrides.File) error {
	if !file.HasDataSource() {
		return nil
	}
	conv, err := buildConvWith(r, refs, file)
	if err != nil {
		return err
	}
	crud, err := buildCRUDWith(r, refs, file)
	if err != nil {
		return err
	}
	_, err = buildDataSource(conv, crud, file)
	return err
}

// dataSourceIssues reports the lines of the dataSource section that change nothing or that this
// version does not support. A line can only change what the resource does to a top-level field.
func dataSourceIssues(file *overrides.File) issue.Report {
	if file.DataSource == nil {
		return nil
	}
	var report issue.Report
	if slices.Equal(file.DataSource.Lookup, []string{overrides.LookupID}) {
		report = append(report, issue.Issue{
			Code:        "OVERRIDE_UNUSED",
			Location:    overrides.FileName + ":dataSource.lookup",
			Message:     "The data source looks up the id without the line.",
			Remediation: "Delete dataSource.lookup. Write dataSource: {} when the section has no other line.",
		})
	}
	for _, line := range file.DataSourceLines() {
		location := overrides.FileName + ":" + line.String()
		switch {
		case line.Component != file.Resource:
			report = append(report, issue.Issue{
				Code:        "OVERRIDE_INVALID",
				Location:    location,
				Message:     fmt.Sprintf("A data source line changes a top-level attribute, so its component must be the resource component %s, not %s.", file.Resource, line.Component),
				Remediation: "Move the line to dataSource.types." + file.Resource + ", or delete it.",
			})
		case file.Types[line.Component].Fields[line.Field].ReadEmptyAs != "null":
			report = append(report, issue.Issue{
				Code:        "OVERRIDE_UNUSED",
				Location:    location,
				Message:     fmt.Sprintf(`The resource has no readEmptyAs: "null" line on %s, so the data source already shows the value of the API.`, line.Field),
				Remediation: "Delete the line.",
			})
		}
	}
	return report
}
