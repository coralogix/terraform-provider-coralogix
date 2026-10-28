package overrides

import (
	"fmt"
	"slices"
)

// DataSource is the dataSource section of the file. A released resource gets a generated data
// source only with this section, which is {} when the data source keeps every default. A new
// resource always gets one.
type DataSource struct {
	// QueryID lets a released singleton data source pass an optional id input to the Get
	// operation's optional string query parameter named id. Without it, a singleton data
	// source takes no input.
	QueryID bool `yaml:"queryID"`
	// Lookup names the attributes that find the resource. [id] is the default, so the generator
	// reports it as a line that changes nothing. [id, name] also finds the resource by its name,
	// through the List of the API.
	Lookup []string `yaml:"lookup"`
	// DuplicateNames: first takes the first item of the List that has the name, in the order of
	// the API, for a released data source that did so. Without the line, a name that more than
	// one item has is an error. It needs name in Lookup.
	DuplicateNames string `yaml:"duplicateNames"`
	// Description is the markdown description of the data source. Without it, the data source has
	// the description of the resource, which describes what the resource manages.
	Description string `yaml:"description"`
	// Types has one entry per OpenAPI component. A readEmptyAs line changes a top-level attribute,
	// so its component is the resource component. A show line and a description line also name an
	// attribute of the items of a top-level list or set of objects, by the component of the items.
	Types map[string]DataSourceType `yaml:"types"`
}

// DataSourceType overrides the data source attributes of one OpenAPI component.
type DataSourceType struct {
	Fields map[string]DataSourceField `yaml:"fields"`
}

// DataSourceField is one field line of the data source.
type DataSourceField struct {
	// ReadEmptyAs: "value" shows the string that the API returns, also the empty one that the
	// resource reads as null (readEmptyAs: "null"), for a released data source that showed "".
	ReadEmptyAs string `yaml:"readEmptyAs"`
	// Show adds a computed attribute that the resource does not have: a field that the resource
	// skips (skip: true) and the contract has on an item of a list or set of objects of the
	// resource, for a released data source that showed it. The data source reads it from the API.
	// A bool and a string are supported.
	Show bool `yaml:"show"`
	// Description is the markdown description of the attribute in the data source. A show line needs
	// it. On another line it replaces the description that the data source copies from the
	// resource, for a released data source that described its attributes in another way.
	Description string `yaml:"description"`
}

// ReadEmptyAsValue is the readEmptyAs value of a data source field line.
const ReadEmptyAsValue = "value"

// The lookup attributes of the data source, and the duplicateNames value.
const (
	LookupID            = "id"
	LookupName          = "name"
	DuplicateNamesFirst = "first"
)

// check checks the keys that the file alone decides. The generator checks the lines against the
// resource lines and the contract.
func (d *DataSource) check() error {
	if d == nil {
		return nil
	}
	if err := d.checkLookup(); err != nil {
		return err
	}
	for _, component := range sortedKeys(d.Types) {
		fields := d.Types[component].Fields
		if len(fields) == 0 {
			return fmt.Errorf("dataSource.types.%s has no override", component)
		}
		for _, field := range sortedKeys(fields) {
			line := fields[field]
			switch {
			case line.Show && line.ReadEmptyAs != "":
				return fmt.Errorf("dataSource.types.%s.fields.%s: show and readEmptyAs exclude each other", component, field)
			case line.Show && line.Description == "":
				return fmt.Errorf("dataSource.types.%s.fields.%s: show needs a description", component, field)
			case line.Show:
			case line.ReadEmptyAs == "" && line.Description == "":
				return fmt.Errorf("dataSource.types.%s.fields.%s has no override: set readEmptyAs, show, or description", component, field)
			case line.ReadEmptyAs != "" && line.ReadEmptyAs != ReadEmptyAsValue:
				return fmt.Errorf("dataSource.types.%s.fields.%s: readEmptyAs is %q, want %q", component, field, line.ReadEmptyAs, ReadEmptyAsValue)
			}
		}
	}
	return nil
}

// checkLookup checks lookup and duplicateNames.
func (d *DataSource) checkLookup() error {
	for i, name := range d.Lookup {
		if name != LookupID && name != LookupName {
			return fmt.Errorf("dataSource.lookup: %q is not supported; the data source looks up %q or %q", name, LookupID, LookupName)
		}
		if slices.Contains(d.Lookup[:i], name) {
			return fmt.Errorf("dataSource.lookup names %q twice", name)
		}
	}
	if len(d.Lookup) != 0 && !slices.Contains(d.Lookup, LookupID) {
		return fmt.Errorf("dataSource.lookup must name %q: the data source always looks up the id", LookupID)
	}
	switch {
	case d.DuplicateNames != "" && d.DuplicateNames != DuplicateNamesFirst:
		return fmt.Errorf("dataSource.duplicateNames is %q, want %q", d.DuplicateNames, DuplicateNamesFirst)
	case d.DuplicateNames != "" && !d.LooksUpName():
		return fmt.Errorf("dataSource.duplicateNames needs %q in dataSource.lookup", LookupName)
	}
	return nil
}

// LooksUpName reports whether the section names name in lookup.
func (d *DataSource) LooksUpName() bool {
	return d != nil && slices.Contains(d.Lookup, LookupName)
}

// DataSourceLine is one line of the dataSource section, for the issues of the generator.
type DataSourceLine struct {
	Component, Field string
	Show             bool // the line shows a field that the resource skips
	ReadEmptyAs      string
	Description      string
}

// String is the location of the line in the file.
func (l DataSourceLine) String() string {
	return "dataSource.types." + l.Component + ".fields." + l.Field
}

// DataSourceLines returns the field lines of the dataSource section, sorted.
func (f *File) DataSourceLines() []DataSourceLine {
	if f == nil || f.DataSource == nil {
		return nil
	}
	var out []DataSourceLine
	for _, component := range sortedKeys(f.DataSource.Types) {
		for _, field := range sortedKeys(f.DataSource.Types[component].Fields) {
			out = append(out, DataSourceLine{Component: component, Field: field, Show: f.DataSource.Types[component].Fields[field].Show,
				ReadEmptyAs: f.DataSource.Types[component].Fields[field].ReadEmptyAs, Description: f.DataSource.Types[component].Fields[field].Description})
		}
	}
	return out
}

// HasDataSource reports whether the generator writes a data source: always for a new resource
// (f is nil), and for a released resource only with a dataSource section.
func (f *File) HasDataSource() bool {
	return f == nil || f.DataSource != nil
}
