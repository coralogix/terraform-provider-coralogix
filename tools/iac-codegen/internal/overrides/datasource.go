package overrides

import (
	"fmt"
	"slices"
)

// DataSource is the dataSource section of the file. A released resource gets a generated data
// source only with this section, which is {} when the data source keeps every default. A new
// resource always gets one.
type DataSource struct {
	// Lookup names the attributes that find the resource. [id] is the default, so the generator
	// reports it as a line that changes nothing. This version looks up the id only.
	Lookup []string `yaml:"lookup"`
	// Types has one entry per OpenAPI component. Only the resource component can have one: a line
	// changes a top-level attribute.
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
}

// ReadEmptyAsValue is the readEmptyAs value of a data source field line.
const ReadEmptyAsValue = "value"

// LookupID is the lookup attribute of the data source.
const LookupID = "id"

// check checks the keys that the file alone decides. The generator checks the lines against the
// resource lines and the contract.
func (d *DataSource) check() error {
	if d == nil {
		return nil
	}
	for i, name := range d.Lookup {
		if name != LookupID {
			return fmt.Errorf("dataSource.lookup: %q is not supported; the data source looks up %q", name, LookupID)
		}
		if slices.Contains(d.Lookup[:i], name) {
			return fmt.Errorf("dataSource.lookup names %q twice", name)
		}
	}
	for _, component := range sortedKeys(d.Types) {
		fields := d.Types[component].Fields
		if len(fields) == 0 {
			return fmt.Errorf("dataSource.types.%s has no override", component)
		}
		for _, field := range sortedKeys(fields) {
			if got := fields[field].ReadEmptyAs; got != ReadEmptyAsValue {
				return fmt.Errorf("dataSource.types.%s.fields.%s: readEmptyAs is %q, want %q", component, field, got, ReadEmptyAsValue)
			}
		}
	}
	return nil
}

// DataSourceLine is one line of the dataSource section, for the issues of the generator.
type DataSourceLine struct {
	Component, Field string
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
			out = append(out, DataSourceLine{Component: component, Field: field})
		}
	}
	return out
}

// HasDataSource reports whether the generator writes a data source: always for a new resource
// (f is nil), and for a released resource only with a dataSource section.
func (f *File) HasDataSource() bool {
	return f == nil || f.DataSource != nil
}
