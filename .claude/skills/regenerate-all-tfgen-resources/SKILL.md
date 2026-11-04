---
name: regenerate-all-tfgen-resources
description: "Use when changing tools/iac-codegen behavior or its version. Regenerate every listed Terraform resource and verify committed output."
---

# Regenerate all Terraform resources

**Trigger:** A generator input or output changes, including `version.Current`, the pinned SDK, or a resource override or acceptance file.

**Fix:** Bump `version.Current` for a generator behavior change. Run `go generate ./internal/provider/` from the provider root. Review every generated package, `versions.yaml`, registry, example, and affected documentation page. Add a line to `internal/provider/codegen.go` for each new generated resource. Include the generated output in the same proposed change. Run `bash tools/iac-codegen/scripts/check-generated-resources.sh` after the change is committed to verify the full result.

**Why:** A generator input can change output without changing a version. CI checks each package version and compares regenerated output with committed files on every PR.
