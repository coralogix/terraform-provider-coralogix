// Command tfgen generates complete Terraform Plugin Framework resources.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/generator"
	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/model"
	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/version"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 1 && (args[0] == "--version" || args[0] == "version") {
		if _, err := fmt.Fprintln(os.Stdout, version.Current); err != nil {
			return 1
		}
		return 0
	}
	if len(args) == 0 {
		printUsage()
		return 2
	}
	switch args[0] {
	case "generate":
		return runGenerate(args[1:])
	case "check":
		return runCheck(args[1:])
	default:
		printUsage()
		return 2
	}
}

func runGenerate(args []string) int {
	set := flag.NewFlagSet("tfgen generate", flag.ContinueOnError)
	set.SetOutput(os.Stderr)
	resource := set.String("resource", "", "OpenAPI component name of the complete resource")
	out := set.String("out", "", "generator-owned output directory")
	overridesFile := set.String("overrides", "", "behavior-overrides file of a resource that users already have (default: the file in --out)")
	acceptanceFile := set.String("acceptance", "", "acceptance file; with it the generator also writes acceptance_test.go (default: the file in --out)")
	operations := addOperationFlags(set)
	if err := set.Parse(args); err != nil {
		return 2
	}
	if set.NArg() != 0 {
		fmt.Fprintf(os.Stderr, "tfgen: unexpected arguments: %v\n", set.Args())
		return 2
	}
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "tfgen: find current directory: %s\n", err)
		return 1
	}
	err = generator.Generate(cwd, generator.Options{
		Resource:       *resource,
		OutputDir:      *out,
		OperationIDs:   operations.ids(),
		OverridesPath:  *overridesFile,
		AcceptancePath: *acceptanceFile,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "tfgen: %s\n", err)
		return 1
	}
	return 0
}

func runCheck(args []string) int {
	set := flag.NewFlagSet("tfgen check", flag.ContinueOnError)
	set.SetOutput(os.Stderr)
	resource := set.String("resource", "", "OpenAPI component name of the complete resource")
	openAPI := set.String("openapi", "", "local candidate OpenAPI file")
	overridesFile := set.String("overrides", "", "behavior-overrides file of a resource that users already have")
	acceptanceFile := set.String("acceptance", "", "acceptance file; with it the check also builds the acceptance test")
	operations := addOperationFlags(set)
	if err := set.Parse(args); err != nil {
		return 2
	}
	if set.NArg() != 0 {
		fmt.Fprintf(os.Stderr, "tfgen: unexpected arguments: %v\n", set.Args())
		return 2
	}
	if err := generator.Check(generator.CheckOptions{
		Resource:       *resource,
		OpenAPIPath:    *openAPI,
		OperationIDs:   operations.ids(),
		OverridesPath:  *overridesFile,
		AcceptancePath: *acceptanceFile,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "tfgen: %s\n", err)
		return 1
	}
	return 0
}

type operationFlags struct {
	create *string
	get    *string
	update *string
	delete *string
}

func addOperationFlags(set *flag.FlagSet) operationFlags {
	return operationFlags{
		create: set.String("create-operation", "", "exact Create operationId when discovery is ambiguous"),
		get:    set.String("get-operation", "", "exact Get operationId when discovery is ambiguous"),
		update: set.String("update-operation", "", "exact Update or Replace operationId when discovery is ambiguous"),
		delete: set.String("delete-operation", "", "exact Delete operationId when discovery is ambiguous"),
	}
}

func (f operationFlags) ids() model.OperationIDs {
	return model.OperationIDs{Create: *f.create, Get: *f.get, Update: *f.update, Delete: *f.delete}
}

func printUsage() {
	fmt.Fprintln(os.Stderr, "usage: tfgen generate --resource <resource_name> --out <generated_output_directory> [--overrides <file>] [--acceptance <file>]")
	fmt.Fprintln(os.Stderr, "       tfgen check --resource <resource_name> --openapi <candidate_openapi_file> [--overrides <file>]")
	fmt.Fprintln(os.Stderr, "       tfgen --version")
}
