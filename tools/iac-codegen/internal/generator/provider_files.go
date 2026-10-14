package generator

import (
	"bytes"
	"errors"
	"fmt"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/overrides"
	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/version"
)

// registryFileName is the file next to the generated packages that registers the new resources.
// A resource that users already had keeps its handwritten registration.
const registryFileName = "registry.go"

// providerFile is a file that the generator writes outside the output directory.
type providerFile struct {
	path string
	data []byte
}

// providerFiles returns the files of the provider that go with the output directory: the docs
// example and import command of a new resource, and the registry. The registry is last, so
// publication registers a package only after the package is in place.
func providerFiles(options Options, providerRoot, providerModule string, isNew bool, typeName string, files map[string][]byte) ([]providerFile, error) {
	output, err := filepath.Abs(options.OutputDir)
	if err != nil {
		return nil, fmt.Errorf("resolve output directory: %w", err)
	}
	if !isNew {
		if err := checkOverridesLocation(options); err != nil {
			return nil, err
		}
	}
	var out []providerFile
	if isNew {
		dir := filepath.Join(providerRoot, "examples", "resources", "coralogix_"+typeName)
		if err := checkExampleOwned(dir, output, typeName); err != nil {
			return nil, err
		}
		out = append(out,
			providerFile{filepath.Join(dir, "resource.tf"), files[exampleKey]},
			providerFile{filepath.Join(dir, "import.sh"), files[importKey]})
	}
	registry, err := renderRegistry(output, providerRoot, providerModule, isNew)
	if err != nil {
		return nil, err
	}
	return append(out, registry), nil
}

// checkOverridesLocation requires the behavior-overrides file of a resource in the provider to be
// in its output directory. The registry reads that file to tell a new resource from an existing one.
func checkOverridesLocation(options Options) error {
	same, err := sameDirectory(filepath.Dir(overridesPath(options)), options.OutputDir)
	if err != nil {
		return err
	}
	if !same {
		return fmt.Errorf("the output directory %s is in the provider, so %s must be in it: the registry reads it to tell a new resource from an existing one",
			options.OutputDir, overrides.FileName)
	}
	return nil
}

// checkExampleOwned allows the generator to write the example directory of a new resource only
// when the directory does not exist, or when an earlier run generated the same resource in output.
// A handwritten example stays.
func checkExampleOwned(dir, output, typeName string) error {
	if _, err := os.Lstat(dir); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return fmt.Errorf("inspect example directory: %w", err)
	}
	if newResourcePackage(output) && generatedTypeName(output) == typeName {
		return nil
	}
	return fmt.Errorf("%s exists, but tfgen did not write it: a new resource does not replace a handwritten example; delete the directory", dir)
}

var typeNameConst = regexp.MustCompile(`(?m)^const TypeName = "([^"]*)"$`)

// generatedTypeName returns the resource type name of the generated package in dir, or "".
func generatedTypeName(dir string) string {
	data, err := os.ReadFile(filepath.Join(dir, "resource.go"))
	if err != nil {
		return ""
	}
	if m := typeNameConst.FindSubmatch(data); m != nil {
		return string(m[1])
	}
	return ""
}

// newResourcePackage reports whether dir holds a generated new resource: a generator-owned package
// without a behavior-overrides file.
func newResourcePackage(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) == 0 || checkOwned(dir) != nil {
		return false
	}
	_, err = os.Stat(filepath.Join(dir, overrides.FileName))
	return errors.Is(err, os.ErrNotExist)
}

// renderRegistry returns the registry of the new resources next to output. It lists the new
// resources on disk and, when isNew, the output package, which is not on disk yet.
func renderRegistry(output, providerRoot, providerModule string, isNew bool) (providerFile, error) {
	parent := filepath.Dir(output)
	pkg := filepath.Base(parent)
	if !token.IsIdentifier(pkg) || token.Lookup(pkg).IsKeyword() {
		return providerFile{}, fmt.Errorf("registry: directory %q is not a valid Go package name", pkg)
	}
	rel, err := filepath.Rel(providerRoot, parent)
	if err != nil {
		return providerFile{}, fmt.Errorf("registry: %w", err)
	}
	path := filepath.Join(parent, registryFileName)
	if err := checkRegistryOwned(path); err != nil {
		return providerFile{}, err
	}
	names, err := newResourcePackages(parent, filepath.Base(output), isNew)
	if err != nil {
		return providerFile{}, err
	}
	importBase := providerModule + "/" + filepath.ToSlash(rel)
	var buf bytes.Buffer
	if err := templates.ExecuteTemplate(&buf, "registry.go.tmpl", map[string]any{
		"VersionHeader": version.Header, "Package": pkg, "ImportBase": importBase, "Names": names,
	}); err != nil {
		return providerFile{}, fmt.Errorf("%s: %w", registryFileName, err)
	}
	src, err := formatGenerated(registryFileName, buf.Bytes())
	if err != nil {
		return providerFile{}, err
	}
	return providerFile{path, src}, nil
}

// checkRegistryOwned allows the generator to replace the registry only when it wrote it.
func checkRegistryOwned(path string) error {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect registry: %w", err)
	}
	if !ownedHeader(data) {
		return fmt.Errorf("%s is not generator-owned: it lacks a valid generator header", path)
	}
	return nil
}

// newResourcePackages returns the names of the new resource packages in parent, sorted. current
// is the package of this run.
func newResourcePackages(parent, current string, isNew bool) ([]string, error) {
	entries, err := os.ReadDir(parent)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("registry: read %s: %w", parent, err)
	}
	var names []string
	for _, entry := range entries {
		name := entry.Name()
		if !entry.IsDir() || strings.HasPrefix(name, ".") || name == current {
			continue
		}
		if newResourcePackage(filepath.Join(parent, name)) {
			names = append(names, name)
		}
	}
	if isNew {
		names = append(names, current)
	}
	slices.Sort(names)
	return names, nil
}

// publishAll publishes the output directory and the provider files. It writes every file to a
// staging place first, so a write error changes nothing. Then it moves the output directory, and
// then each provider file; the registry is last. A failed run removes the directories that it
// created and left empty, so the next run does not take them for handwritten ones.
func publishAll(output string, files map[string][]byte, extras []providerFile) (err error) {
	var staged, created []string
	defer func() {
		for _, path := range staged {
			_ = os.Remove(path)
		}
		if err != nil {
			for i := len(created) - 1; i >= 0; i-- {
				_ = os.Remove(created[i]) // only an empty directory goes
			}
		}
	}()
	for _, extra := range extras {
		dirs, err := createDirs(filepath.Dir(extra.path))
		created = append(created, dirs...)
		if err != nil {
			return err
		}
		path, err := stageFile(extra)
		if err != nil {
			return err
		}
		staged = append(staged, path)
	}
	if err := publish(output, files); err != nil {
		return err
	}
	for i, extra := range extras {
		if err := renamePath(staged[i], extra.path); err != nil {
			return fmt.Errorf("publish %s: %w", extra.path, err)
		}
	}
	return nil
}

// createDirs creates dir and its missing parents. It returns the directories that it created,
// parents first.
func createDirs(dir string) ([]string, error) {
	var missing []string
	for d := filepath.Clean(dir); ; d = filepath.Dir(d) {
		if _, err := os.Lstat(d); err == nil || filepath.Dir(d) == d {
			break
		}
		missing = append([]string{d}, missing...)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return missing, fmt.Errorf("create %s: %w", dir, err)
	}
	return missing, nil
}

func stageFile(file providerFile) (string, error) {
	stage, err := os.CreateTemp(filepath.Dir(file.path), "."+filepath.Base(file.path)+".stage-")
	if err != nil {
		return "", fmt.Errorf("stage %s: %w", file.path, err)
	}
	_, writeErr := stage.Write(file.data)
	closeErr := stage.Close()
	if err := errors.Join(writeErr, closeErr, os.Chmod(stage.Name(), 0o644)); err != nil {
		_ = os.Remove(stage.Name())
		return "", fmt.Errorf("stage %s: %w", file.path, err)
	}
	return stage.Name(), nil
}

// generatedDir is the directory of the generated packages, relative to the provider root. Its
// registry is the one that provider.go imports.
var generatedDir = filepath.Join("internal", "provider", "generated")

// checkOutputLocation requires the output directory to be a direct child of generatedDir. The
// registry sits next to the output, and provider.go imports only the registry in generatedDir.
func checkOutputLocation(providerRoot, output string) error {
	abs, err := filepath.Abs(output)
	if err != nil {
		return fmt.Errorf("resolve output directory: %w", err)
	}
	if !inGeneratedDir(providerRoot, abs) {
		return fmt.Errorf("output directory %s must be a directory in %s: provider.go registers only the resources of its registry",
			abs, filepath.Join(providerRoot, generatedDir))
	}
	return nil
}

// inGeneratedDir reports whether output is a direct child of generatedDir in the provider.
func inGeneratedDir(providerRoot, output string) bool {
	return filepath.Dir(filepath.Clean(output)) == filepath.Join(providerRoot, generatedDir)
}
