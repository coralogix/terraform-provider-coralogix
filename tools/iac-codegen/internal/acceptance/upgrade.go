package acceptance

import (
	"bytes"
	"fmt"
	"slices"

	"go.yaml.in/yaml/v4"
)

// UpgradeFileName is the name of the upgrade attributes file. The generator writes it next to the
// acceptance file. People do not edit it.
const UpgradeFileName = "upgrade-attributes.yaml"

// UpgradeAttributes are the attributes that the released provider of the upgrade test has. The
// generator takes them from the schema when upgradeFrom is first set or changes, and keeps them
// after that. The upgrade test leaves out every attribute that is not in the list, because the
// released provider rejects an attribute it does not have.
type UpgradeAttributes struct {
	From       string   `yaml:"from"`
	Attributes []string `yaml:"attributes"`
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
	return &u, nil
}

// Marshal returns the file, with the attributes in sorted order.
func (u *UpgradeAttributes) Marshal() ([]byte, error) {
	sorted := *u
	sorted.Attributes = slices.Sorted(slices.Values(u.Attributes))
	data, err := yaml.Marshal(&sorted)
	if err != nil {
		return nil, err
	}
	return append([]byte(upgradeHeader), data...), nil
}
