---
name: regenerate-all-tfgen-resources
description: "Use when changing tools/iac-codegen behavior or its version. Regenerate every listed Terraform resource and verify committed output."
---

# Regenerate all Terraform resources

**Trigger:** The generator output changes, `version.Current` changes, or `versions.yaml` has older entries.

**Fix:** Bump `version.Current` for an output change. Run `go generate ./internal/provider/` from the provider root. Review every generated package, `versions.yaml`, registry, example, and affected documentation page. Add a line to `internal/provider/codegen.go` for each new generated resource. Include the generated output in the same proposed change. Run `bash tools/iac-codegen/scripts/check-generated-resources.sh <base-commit>` after the change is committed to verify the full result.

**Why:** One package can keep old generated logic after a version change. CI checks every package version and compares regenerated output with committed files when the version changes.
