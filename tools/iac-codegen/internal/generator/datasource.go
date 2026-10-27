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
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
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
	// Show are the lists and sets of objects whose items get attributes that only the data source
	// has (show: true).
	Show []*dsShow
	// Descriptions replace the descriptions that the data source copies from the resource, by the
	// dotted Terraform path of the attribute; the path "" is the data source itself.
	Descriptions []dsDescription
}

// dsDescription is the description of one attribute of the data source.
type dsDescription struct{ Path, Text string }

// dsShow is a top-level list or set of objects of the resource whose items get more attributes
// in the data source: fields that the resource skips and the API returns.
type dsShow struct {
	Attr       string // Terraform name of the attribute that holds the items
	Model      string // Go field of the Terraform model
	SDK        string // SDK field of the resource that holds the items
	ItemSDK    string // SDK type of an item
	NeedsPrior bool   // flatten<Flatten> takes the prior model; the data source has none
	Collection string // "Set" or "List"
	Flatten    string // function suffix: flatten<Flatten> converts an item
	AttrTypes  string // function that returns the attribute types of an item
	Fields     []*dsShowField
}

// dsShowField is one attribute that a data source adds to the items of a dsShow.
type dsShowField struct {
	TFName, SDK string
	Kind        string // Bool or String
	Pointer     bool   // the SDK field is a pointer
	Description string
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
		if line.Show || line.ReadEmptyAs == "" {
			continue
		}
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
	if out.Show, err = buildShow(refs, conv, file); err != nil {
		return nil, err
	}
	if out.Descriptions, err = buildDescriptions(refs, conv, file); err != nil {
		return nil, err
	}
	return out, nil
}

// buildDescriptions returns the description lines: the data source itself, a top-level attribute of
// the resource, and an attribute of the items of a top-level list or set of objects.
func buildDescriptions(refs []sdkcheck.Ref, conv *convData, file *overrides.File) ([]dsDescription, error) {
	var out []dsDescription
	if file != nil && file.DataSource != nil && file.DataSource.Description != "" {
		out = append(out, dsDescription{Text: file.DataSource.Description})
	}
	for _, line := range file.DataSourceLines() {
		if line.Show || line.Description == "" {
			continue
		}
		at, err := descriptionPath(refs, conv, file, line)
		if err != nil {
			return nil, err
		}
		out = append(out, dsDescription{Path: at, Text: line.Description})
	}
	return out, nil
}

// descriptionPath returns the Terraform path of the attribute that a description line names.
func descriptionPath(refs []sdkcheck.Ref, conv *convData, file *overrides.File, line overrides.DataSourceLine) (string, error) {
	named := func(fields []*convField) *convField {
		i := slices.IndexFunc(fields, func(f *convField) bool { return f.Model == camelize(line.Field) && f.Conv != convGroup })
		if i < 0 {
			return nil
		}
		return fields[i]
	}
	if line.Component == file.Resource {
		f := named(conv.Resource.Fields)
		if f == nil {
			return "", fmt.Errorf("%s: %s is not a top-level attribute of the resource", line, line.Field)
		}
		return f.TFName, nil
	}
	ix, err := indexRefs(refs)
	if err != nil {
		return "", err
	}
	owner, err := ix.schemaRef(line.Component)
	if err != nil {
		return "", fmt.Errorf("%s: %w", line, err)
	}
	for _, holder := range conv.Resource.Fields {
		if holder.Object == nil || holder.Object.SDK != owner.Name || (holder.Conv != convObjects && holder.Conv != convObj) {
			continue
		}
		f := named(holder.Object.Fields)
		if f == nil {
			return "", fmt.Errorf("%s: %s is not an attribute of the items of %s", line, line.Field, holder.TFName)
		}
		return holder.TFName + "." + f.TFName, nil
	}
	return "", fmt.Errorf("%s: the resource has no top-level object or list of objects of component %s", line, line.Component)
}

// showPath is the path of the synthetic SDK ref of a field that a data source line shows.
func showPath(component, field string) string { return "show." + component + "." + field }

// showSDKNames adds an SDK ref for each field that a show line adds to the data source, so the SDK
// check finds out whether the SDK has it and whether it is a pointer. The model has no such field,
// because the resource skips it. The contract says its type: a bool or a string.
func showSDKNames(doc *v3.Document, refs []sdkcheck.Ref, file *overrides.File) ([]sdkcheck.Ref, error) {
	lines := showLines(file)
	if len(lines) == 0 {
		return refs, nil
	}
	ix, err := indexRefs(refs)
	if err != nil {
		return nil, err
	}
	out := slices.Clone(refs)
	for _, line := range lines {
		owner, err := ix.schemaRef(line.Component)
		if err != nil {
			return nil, fmt.Errorf("%s: the resource has no object of component %s: %w", line, line.Component, err)
		}
		kind, err := showFieldKind(doc, line)
		if err != nil {
			return nil, err
		}
		out = append(out, sdkcheck.Ref{Path: showPath(line.Component, line.Field), Kind: sdkcheck.KindField, Pkg: owner.Pkg, Owner: owner.Name,
			Name: goFieldName(line.Field), Want: "*" + strings.ToLower(kind), WantValue: strings.ToLower(kind), Rule: sdkcheck.RuleProperty})
	}
	return out, nil
}

// showFieldKind returns Bool or String for the contract type of the field of a show line.
func showFieldKind(doc *v3.Document, line overrides.DataSourceLine) (string, error) {
	if doc.Components == nil || doc.Components.Schemas == nil {
		return "", fmt.Errorf("%s: the API contract has no component schemas", line)
	}
	proxy := doc.Components.Schemas.GetOrZero(line.Component)
	if proxy == nil {
		return "", fmt.Errorf("%s: no component %q", line, line.Component)
	}
	schema, err := proxy.BuildSchema()
	if err != nil {
		return "", fmt.Errorf("%s: %w", line, err)
	}
	if schema.Properties == nil || schema.Properties.GetOrZero(line.Field) == nil {
		return "", fmt.Errorf("%s: component %s has no field %s", line, line.Component, line.Field)
	}
	field, err := schema.Properties.GetOrZero(line.Field).BuildSchema()
	if err != nil {
		return "", fmt.Errorf("%s: %w", line, err)
	}
	switch {
	case slices.Contains(field.Type, "boolean"):
		return "Bool", nil
	case slices.Contains(field.Type, "string") && field.Format == "" && len(field.Enum) == 0:
		return "String", nil
	}
	return "", fmt.Errorf("%s: show supports a boolean and a plain string, not the field %s", line, line.Field)
}

// showLines returns the lines of the data source section that show a field.
func showLines(file *overrides.File) []overrides.DataSourceLine {
	var out []overrides.DataSourceLine
	for _, line := range file.DataSourceLines() {
		if line.Show {
			out = append(out, line)
		}
	}
	return out
}

// buildShow groups the show lines by the top-level list or set of objects that holds the items.
func buildShow(refs []sdkcheck.Ref, conv *convData, file *overrides.File) ([]*dsShow, error) {
	lines := showLines(file)
	if len(lines) == 0 {
		return nil, nil
	}
	ix, err := indexRefs(refs)
	if err != nil {
		return nil, err
	}
	var out []*dsShow
	for _, line := range lines {
		owner, err := ix.schemaRef(line.Component)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", line, err)
		}
		index := slices.IndexFunc(conv.Resource.Fields, func(f *convField) bool {
			return f.Conv == convObjects && f.Object != nil && f.Object.SDK == owner.Name
		})
		if index < 0 {
			return nil, fmt.Errorf("%s: show needs a top-level list or set of objects of the resource whose items are %s", line, line.Component)
		}
		holder := conv.Resource.Fields[index]
		field, err := ix.fieldRef(showPath(line.Component, line.Field))
		if err != nil {
			return nil, err
		}
		kind, pointer := "Bool", strings.HasPrefix(field.Want, "*")
		if strings.HasSuffix(field.Want, "string") {
			kind = "String"
		}
		at := slices.IndexFunc(out, func(s *dsShow) bool { return s.Attr == holder.TFName })
		if at < 0 {
			out = append(out, &dsShow{Attr: holder.TFName, Model: holder.Model, SDK: holder.SDK, Collection: holder.Collection,
				ItemSDK: owner.Name, NeedsPrior: holder.Object.NeedsPrior, Flatten: holder.Object.Func, AttrTypes: holder.Object.AttrTypesFunc})
			at = len(out) - 1
		}
		tfName := terraformName(file.Policy(), line.Component, line.Field)
		// The shown attribute must not take the name of an attribute of the item, or of another shown
		// attribute: foo-bar and foo_bar are both foo_bar in Terraform.
		if slices.ContainsFunc(holder.Object.Fields, func(f *convField) bool { return f.TFName == tfName }) ||
			slices.ContainsFunc(out[at].Fields, func(f *dsShowField) bool { return f.TFName == tfName }) {
			return nil, fmt.Errorf("%s: the shown attribute %s has the Terraform name %s, which an attribute of %s already has", line, line.Field, tfName, holder.TFName)
		}
		out[at].Fields = append(out[at].Fields, &dsShowField{TFName: tfName, SDK: field.Name, Kind: kind,
			Pointer: pointer, Description: file.DataSource.Types[line.Component].Fields[line.Field].Description})
	}
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
		case line.Show && file.Types[line.Component].Fields[line.Field].Skip:
		case line.Show:
			report = append(report, issue.Issue{
				Code:        "OVERRIDE_INVALID",
				Location:    location,
				Message:     fmt.Sprintf("show adds a field that the resource skips, but the resource does not skip %s.%s, so the data source already has it.", line.Component, line.Field),
				Remediation: "Delete the line, or skip the field of the resource.",
			})
		case line.ReadEmptyAs != "" && line.Component != file.Resource:
			report = append(report, issue.Issue{
				Code:        "OVERRIDE_INVALID",
				Location:    location,
				Message:     fmt.Sprintf("A data source line changes a top-level attribute, so its component must be the resource component %s, not %s.", file.Resource, line.Component),
				Remediation: "Move the line to dataSource.types." + file.Resource + ", or delete it.",
			})
		case line.ReadEmptyAs != "" && file.Types[line.Component].Fields[line.Field].ReadEmptyAs != "null":
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
