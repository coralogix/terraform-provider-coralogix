// Package enumtest is for tests only. An overrides file no longer has a values key on an enums
// line: the generator takes the accepted values from the contract. A test that has no contract
// can still write the values for brevity, and Split turns them into the contract that the line
// stands for.
package enumtest

import "go.yaml.in/yaml/v4"

// Split removes the values key of each enums line and returns, per enum, the contract
// that the line stands for.
func Split(text string) (string, map[string][]string, error) {
	var root yaml.Node
	if err := yaml.Unmarshal([]byte(text), &root); err != nil || len(root.Content) == 0 || root.Content[0].Kind != yaml.MappingNode {
		return text, nil, nil // Parse reports the error
	}
	contracts := map[string][]string{}
	top := root.Content[0]
	for i := 0; i+1 < len(top.Content); i += 2 {
		if top.Content[i].Value != "enums" || top.Content[i+1].Kind != yaml.MappingNode {
			continue
		}
		enums := top.Content[i+1]
		for j := 0; j+1 < len(enums.Content); j += 2 {
			name, line := enums.Content[j].Value, enums.Content[j+1]
			if line.Kind != yaml.MappingNode {
				continue
			}
			var contract, rejected []string
			zero, hadValues := false, false
			kept := line.Content[:0:0]
			for k := 0; k+1 < len(line.Content); k += 2 {
				key, value := line.Content[k], line.Content[k+1]
				switch key.Value {
				case "values":
					hadValues = true
					for _, v := range value.Content {
						contract = append(contract, v.Value)
					}
					continue
				case "rejected":
					for _, v := range value.Content {
						rejected = append(rejected, v.Value)
					}
				case "zero", "zeroNull":
					zero = true
				}
				kept = append(kept, key, value)
			}
			if !hadValues {
				continue
			}
			line.Content = kept
			if zero {
				contract = append([]string{"ZERO_UNSPECIFIED"}, contract...)
			}
			contracts[name] = append(contract, rejected...)
		}
	}
	if len(contracts) == 0 {
		return text, nil, nil
	}
	out, err := yaml.Marshal(&root)
	if err != nil {
		return "", nil, err
	}
	return string(out), contracts, nil
}
