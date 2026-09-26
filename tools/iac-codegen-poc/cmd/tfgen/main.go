// Command tfgen generates a Terraform resource from an OpenAPI spec.
//
//	go run ./cmd/tfgen --spec spec/openapi.patched.yaml --resource AiEvaluation --acc spec/acc/AiEvaluation.yaml --out generated/aievaluation
//	go run ./cmd/tfgen --spec spec/openapi.patched.yaml --resource AiEvaluation --sdk-names
//	go run ./cmd/tfgen --spec spec/openapi.patched.yaml --survey
//	go run ./cmd/tfgen --spec spec/openapi.patched.yaml --survey-resources
//	go run ./cmd/tfgen --spec spec/fake/openapi.yaml --resource FakeBoard --sdk-module github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen-poc/fakesdk --out generated/fakeboard
//	go run ./cmd/tfgen --spec spec/fake/openapi.yaml --types Panel,Header --tag "Fake Boards Service" --sdk-module github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen-poc/fakesdk --out generated/fakepanel
//
// Both check that the pinned SDK has every name the generated code uses.
// --out writes the generated files. The package name is the last element of
// the directory. --sdk-names prints the list of names. --types writes only
// the Terraform types of component schemas, for handwritten resources (D20).
package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen-poc/internal/model"
)

func main() {
	spec := flag.String("spec", "", "patched OpenAPI YAML file")
	resource := flag.String("resource", "", "component name of the resource schema, for example AiEvaluation")
	out := flag.String("out", "", "output directory of the generated resource")
	sdkNames := flag.Bool("sdk-names", false, "check the SDK names and print them")
	sdkModule := flag.String("sdk-module", realSDK, "Go module of the SDK")
	survey := flag.Bool("survey", false, "measure the schema shapes of all Get resources that cannot be generated")
	surveyResources := flag.Bool("survey-resources", false, "measure the resource shapes (operations, ids, bodies, responses) of all Get resources")
	acc := flag.String("acc", "", "acceptance test values file (API shape); with --out, also writes acc_test.go")
	typeList := flag.String("types", "", "comma-separated component schemas: write only their Terraform types, for handwritten resources")
	enumList := flag.String("enums", "", "comma-separated enum component schemas: write their Terraform names, for handwritten resources")
	tag := flag.String("tag", "", "with --types or --enums: the operation tag whose SDK package has the types")
	overridesFile := flag.String("overrides", "", "with --types: a YAML file that makes the types match an existing handwritten resource")
	flag.Parse()

	if *typeList != "" || *enumList != "" || *overridesFile != "" {
		in := typeInputs{roots: splitList(*typeList), enums: splitList(*enumList), overrides: *overridesFile}
		if err := runTypes(*spec, in, *tag, *sdkModule, *out); err != nil {
			fmt.Fprintln(os.Stderr, "tfgen:", err)
			os.Exit(1)
		}
		return
	}

	if *surveyResources {
		if err := runResourceSurvey(*spec, os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, "tfgen:", err)
			os.Exit(1)
		}
		return
	}
	if *survey {
		if err := runSurvey(*spec, os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, "tfgen:", err)
			os.Exit(1)
		}
		return
	}

	if err := run(*spec, *resource, *out, *sdkModule, *acc, *sdkNames); err != nil {
		fmt.Fprintln(os.Stderr, "tfgen:", err)
		os.Exit(1)
	}
}

func run(spec, resource, out, sdkModule, accPath string, sdkNames bool) error {
	if spec == "" || resource == "" {
		return errors.New("--spec and --resource are required")
	}
	if (out == "") == !sdkNames {
		return errors.New("use exactly one of --out and --sdk-names")
	}
	r, refs, err := checkedSDKNames(spec, resource, sdkModule)
	if err != nil {
		return err
	}
	if sdkNames {
		return writeSDKNames(os.Stdout, refs)
	}
	var acc *accValues
	if accPath != "" {
		if acc, err = loadAccValues(accPath); err != nil {
			return err
		}
	}
	files, err := generate(r, refs, filepath.Base(out), acc)
	if err != nil {
		return err
	}
	return writeFiles(out, files)
}

// runTypes writes the Terraform types and the enum names of in to out.
func runTypes(spec string, in typeInputs, tag, sdkModule, out string) error {
	if spec == "" || out == "" {
		return errors.New("--spec and --out are required with --types and --enums")
	}
	if len(in.roots) == 0 && in.overrides != "" {
		return fmt.Errorf("%s needs --types", overridesFlag)
	}
	var ov *overrides
	if in.overrides != "" {
		var err error
		if ov, err = loadOverrides(in.overrides); err != nil {
			return err
		}
	}
	types, enums, refs, err := checkedTypeNames(spec, in, tag, sdkModule)
	if err != nil {
		return err
	}
	files, err := generateTypes(types, enums, refs, ov, filepath.Base(out), typesCommand(in, tag))
	if err != nil {
		return err
	}
	return writeFiles(out, files)
}

// typesCommand is the part of the tfgen command that the generated package
// records. It has no paths, so it is the same on every machine.
func typesCommand(in typeInputs, tag string) string {
	var parts []string
	if len(in.roots) != 0 {
		parts = append(parts, "--types "+strings.Join(in.roots, ","))
	}
	if len(in.enums) != 0 {
		parts = append(parts, "--enums "+strings.Join(in.enums, ","))
	}
	if in.overrides != "" {
		parts = append(parts, overridesFlag+" "+filepath.Base(in.overrides))
	}
	return fmt.Sprintf("%s %s %q", strings.Join(parts, " "), typeTagFlag, tag)
}

// splitList splits a comma-separated flag value. An empty value is no items.
func splitList(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, ",")
}

func writeFiles(out string, files map[string][]byte) error {
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	for name, src := range files {
		if err := os.WriteFile(filepath.Join(out, name), src, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// checkedSDKNames builds the model and returns it with its SDK names. It
// fails when the pinned SDK does not have one of the names. The generated
// code uses only these names.
func checkedSDKNames(spec, resource, sdkModule string) (*model.Resource, []sdkRef, error) {
	data, err := os.ReadFile(spec)
	if err != nil {
		return nil, nil, err
	}
	doc, err := model.Load(data)
	if err != nil {
		return nil, nil, err
	}
	r, err := model.Build(doc, resource)
	if err != nil {
		return nil, nil, err
	}
	tag, err := resourceTag(doc, r)
	if err != nil {
		return nil, nil, err
	}
	refs, err := resolveSDKNames(r, tag, sdkModule)
	if err != nil {
		return nil, nil, err
	}
	pkgs, err := loadSDK(refs)
	if err != nil {
		return nil, nil, err
	}
	if err := checkSDKNames(refs, pkgs); err != nil {
		return nil, nil, err
	}
	return r, refs, nil
}

// writeSDKNames prints one line per ref, grouped by package.
func writeSDKNames(w io.Writer, refs []sdkRef) error {
	var buf bytes.Buffer
	tw := tabwriter.NewWriter(&buf, 0, 0, 2, ' ', 0)
	pkg := ""
	for _, ref := range refs {
		if ref.Pkg != pkg {
			pkg = ref.Pkg
			fmt.Fprintf(tw, "\npackage %s\n", pkg)
			fmt.Fprintln(tw, "  PATH\tKIND\tRULE\tSDK NAME\tGO TYPE")
		}
		fmt.Fprintf(tw, "  %s\t%s\t%s\t%s\t%s\n", ref.Path, ref.Kind, ref.Rule, ref.sdkName(), ref.Want)
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	for line := range strings.Lines(buf.String()) {
		if _, err := io.WriteString(w, strings.TrimRight(line, " \n")+"\n"); err != nil {
			return err
		}
	}
	return nil
}
