// Command tfgen generates a Terraform resource from an OpenAPI spec.
//
//	go run ./cmd/tfgen --spec spec/openapi.patched.yaml --resource AiEvaluation --acc spec/acc/AiEvaluation.yaml --out generated/aievaluation
//	go run ./cmd/tfgen --spec spec/openapi.patched.yaml --resource AiEvaluation --sdk-names
//	go run ./cmd/tfgen --spec spec/openapi.patched.yaml --survey
//	go run ./cmd/tfgen --spec spec/fake/openapi.yaml --resource FakeBoard --sdk-module github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen-poc/fakesdk --out generated/fakeboard
//
// Both check that the pinned SDK has every name the generated code uses.
// --out writes the generated files. The package name is the last element of
// the directory. --sdk-names prints the list of names.
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
	acc := flag.String("acc", "", "acceptance test values file (API shape); with --out, also writes acc_test.go")
	flag.Parse()

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
