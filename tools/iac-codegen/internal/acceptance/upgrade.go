package acceptance

import (
	"bytes"
	"fmt"

	"go.yaml.in/yaml/v4"
)

// UpgradeFileName is the name of the upgrade attributes file. The generator writes it next to the
// acceptance file. People do not edit it.
const UpgradeFileName = "upgrade-attributes.yaml"

// UpgradeAttributes are the attributes that the released provider of the upgrade test has, with
// their type and requiredness. The generator takes them from the schema when upgradeFrom is first
// set or changes, and keeps them after that. The upgrade test leaves out every attribute that the
// release does not have in the same type, because the released provider rejects it.
type UpgradeAttributes struct {
	From string `yaml:"from"`
	// Attributes map an attribute path, such as rules[].targets[].connector_id, to its shape.
	Attributes map[string]UpgradeAttribute `yaml:"attributes"`
}

// UpgradeAttribute is the shape of one attribute in the release.
type UpgradeAttribute struct {
	// Type is the Terraform kind, with the element type of a collection of plain values: String,
	// ListNested, List(String).
	Type     string `yaml:"type"`
	Required bool   `yaml:"required,omitempty"`
}

// MarshalYAML writes an attribute on one line: {type: String, required: true}.
func (a UpgradeAttribute) MarshalYAML() (any, error) {
	type plain UpgradeAttribute
	var node yaml.Node
	if err := node.Encode(plain(a)); err != nil {
		return nil, err
	}
	node.Style = yaml.FlowStyle
	return &node, nil
}

const upgradeHeader = `# Written by coralogix-iac-codegen. Do not edit.
# The attributes that the released provider of the upgrade test has. The upgrade test leaves out
# every other attribute. The generator takes the list from the schema when upgradeFrom changes.
`

// ParseUpgrade reads the upgrade attributes file.
func ParseUpgrade(data []byte) (*UpgradeAttributes, error) {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	var u UpgradeAttributes
	if err := dec.Decode(&u); err != nil {
		return nil, fmt.Errorf("%s: %w", UpgradeFileName, err)
	}
	if !version.MatchString(u.From) {
		return nil, fmt.Errorf("%s: from is %q, want a release such as 3.19.0", UpgradeFileName, u.From)
	}
	for path, a := range u.Attributes {
		if !fieldPath.MatchString(path) || a.Type == "" {
			return nil, fmt.Errorf("%s: %q needs a field path and a type", UpgradeFileName, path)
		}
	}
	return &u, nil
}

// Marshal returns the file. The attributes are in sorted order.
func (u *UpgradeAttributes) Marshal() ([]byte, error) {
	data, err := yaml.Marshal(u)
	if err != nil {
		return nil, err
	}
	return append([]byte(upgradeHeader), data...), nil
}
