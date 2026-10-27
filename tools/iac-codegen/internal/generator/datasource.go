package generator

import (
	"errors"
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
	// Name is the lookup by name. nil: the data source looks up the id only.
	Name *dsNameLookup
}

// dsNameLookup finds the id of the resource with a name: it calls the List and takes the id of
// the item with that name. The data source then reads the id as a lookup by id does.
type dsNameLookup struct {
	Attr      string // Terraform attribute of the name
	Validator string // package of the ExactlyOneOf validator of the id, for example stringvalidator
	List      string // SDK method of the List
	Items     string // SDK field of the response that holds the items
	ItemID    string // SDK field of the item id
	ItemName  string // SDK field of the item name
	// IDValue and NameValue: the item field is a value, not a pointer.
	IDValue, NameValue bool
	// First takes the first item with the name (duplicateNames: first). Otherwise a name that
	// more than one item has is an error.
	First bool
}

// dsValueField is a top-level string of a readEmptyAs: "value" line.
type dsValueField struct {
	TFName, Model, SDK string
	Value              bool // the SDK field is a value, not a pointer
}

// buildDataSource returns the template data of the data source.
func buildDataSource(r *model.Resource, refs []sdkcheck.Ref, conv *convData, crud *crudData, file *overrides.File) (*dsData, error) {
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
	name, err := buildNameLookup(r, refs, conv, crud, file)
	if err != nil {
		return nil, err
	}
	out.Name = name
	return out, nil
}

// buildNameLookup returns the lookup by name, or nil. A new resource has it when the List of the
// API is usable. A released resource has it only with name in dataSource.lookup, and then the
// List must be usable.
func buildNameLookup(r *model.Resource, refs []sdkcheck.Ref, conv *convData, crud *crudData, file *overrides.File) (*dsNameLookup, error) {
	wanted := file != nil && file.DataSource.LooksUpName()
	if file != nil && !wanted {
		return nil, nil
	}
	attr, problem := nameLookupProblem(r, conv)
	switch {
	case problem == "":
	case wanted:
		return nil, fmt.Errorf("%w: dataSource.lookup names %q, but %s", errDataSourceLookup, overrides.LookupName, problem)
	default:
		return nil, nil // a new resource without a usable List: lookup by id only
	}
	ix, err := indexRefs(refs)
	if err != nil {
		return nil, err
	}
	out := &dsNameLookup{
		Attr:      attr,
		Validator: strings.ToLower(crud.IDTFType) + "validator",
		First:     file != nil && file.DataSource.DuplicateNames == overrides.DuplicateNamesFirst,
	}
	method, err := ix.methodRef("list", crud.Client)
	if err != nil {
		return nil, err
	}
	items, err := ix.fieldRef("list.response." + r.List.ItemsField)
	if err != nil {
		return nil, err
	}
	out.List, out.Items = method.Name, items.Name
	item := "list.item."
	if r.List.Full {
		item = "fields." // the item is the resource, so the resource fields name it
	}
	id, err := ix.fieldRef(item + r.List.IDField)
	if err != nil {
		return nil, err
	}
	name, err := ix.fieldRef(item + r.List.NameField)
	if err != nil {
		return nil, err
	}
	out.ItemID, out.IDValue = id.Name, id.Want == crud.IDGoType
	out.ItemName, out.NameValue = name.Name, name.Want == "string"
	return out, nil
}

// dropUnusedList removes the List from r when the data source does not look up the name, so that
// the SDK names of a List that the generated code never calls are not checked. A released
// resource keeps the List only with name in dataSource.lookup. A new resource keeps it when the
// List can serve the lookup.
func dropUnusedList(r *model.Resource, file *overrides.File) {
	if file != nil && file.DataSource.LooksUpName() {
		return
	}
	if l := r.List; file == nil && !r.Singleton && l != nil && l.NameField != "" && !l.Paged {
		return
	}
	r.List = nil
}

// errDataSourceLookup is a lookup line that the API cannot serve.
var errDataSourceLookup = errors.New("the data source cannot look up the name")

// nameLookupProblem returns the Terraform attribute of the name, or why the data source cannot find
// the resource by its name: the List must exist, have items with a string name, and return every
// item in one response.
func nameLookupProblem(r *model.Resource, conv *convData) (string, string) {
	l := r.List
	switch {
	case r.Singleton:
		return "", "a singleton has no List"
	case l == nil && r.ListProblem != "":
		return "", "the List of the API cannot be used: " + r.ListProblem
	case l == nil:
		return "", "the API has no List on the Create path; name one with api.operations.list"
	case l.NameField == "":
		return "", fmt.Sprintf("the List item %s has no string property name", l.ItemComponent)
	case l.Paged:
		return "", fmt.Sprintf("the List %s pages its result, and the data source reads only one page", l.OperationID)
	}
	attr := terraformName(r.Policy, r.Name, l.NameField)
	if !slices.ContainsFunc(conv.Resource.Fields, func(f *convField) bool { return f.TFName == attr && f.Conv == convString }) {
		return "", fmt.Sprintf("the resource has no top-level string attribute %s for the name", attr)
	}
	return attr, ""
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
	_, err = buildDataSource(r, refs, conv, crud, file)
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
