package generator

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
)

// The keys of the docs files in the rendered files. They are not files of the output directory:
// a new resource has them in examples/resources/coralogix_<type>/ of the provider.
const (
	exampleKey = "examples/resource.tf"
	importKey  = "examples/import.sh"
)

// exampleProviders is the provider block of an example. It has no version: the example works
// with every release that has the resource.
const exampleProviders = `terraform {
  required_providers {
    coralogix = {
      source = "coralogix/coralogix"
    }
  }
}`

// exampleRun is @{run} in the docs example. Made-up strings fit their validators with it too.
const exampleRun = "example"

var envPlaceholder = regexp.MustCompile(`@\{env\.([A-Z][A-Z0-9_]*)\}`)

// renderExample returns the example of the docs: the prerequisites and the full config of the
// acceptance test. The id of the test run becomes "example", and an environment variable becomes
// a placeholder with its name, for example "<SLACK_INTEGRATION_ID>". No value of the test
// environment reaches the docs.
func renderExample(acc *acceptanceData, typeName string) ([]byte, error) {
	resourceType := "coralogix_" + typeName
	testLabel := fmt.Sprintf("resource %q \"test\"", resourceType)
	if !strings.HasPrefix(acc.Full.Config, testLabel) {
		return nil, fmt.Errorf("example: the full config does not start with %s", testLabel)
	}
	config := fmt.Sprintf("resource %q \"example\"", resourceType) + strings.TrimPrefix(acc.Full.Config, testLabel)
	parts := []string{exampleProviders}
	if acc.Prerequisites != "" {
		parts = append(parts, acc.Prerequisites)
	}
	parts = append(parts, config)
	text := envPlaceholder.ReplaceAllString(strings.Join(parts, "\n\n")+"\n", "<$1>")
	text = strings.ReplaceAll(text, "@{run}", exampleRun)
	if _, diags := hclsyntax.ParseConfig([]byte(text), "resource.tf", hcl.InitialPos); diags.HasErrors() {
		return nil, fmt.Errorf("example: %s", diags.Error())
	}
	return hclwrite.Format([]byte(text)), nil
}

// renderImport returns the import command of the docs. A singleton has one id per company: the
// resource type name. Other ids are a quoted placeholder: unquoted, a shell reads < and > as
// redirections.
func renderImport(crud *crudData) []byte {
	id := `"<` + crud.IDAttr + `>"`
	if crud.Singleton {
		id = crud.TypeName
	}
	return fmt.Appendf(nil, "terraform import coralogix_%s.example %s\n", crud.TypeName, id)
}
