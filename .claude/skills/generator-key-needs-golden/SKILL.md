---
name: generator-key-needs-golden
description: "Use when adding a generator key or branch in tools/iac-codegen. Cover it with a golden fixture. Do NOT use for a resource-only override line."
---

# A new generator key needs a golden

**Trigger:** A new key or branch in `tools/iac-codegen` (a field line, an enum line, or a convert or schema branch).

**Fix:** Add or extend a fixture under `internal/generator/testdata`. `customthing` covers custom methods, root and nested `requestValue`, `readNullAs`, string `readEmptyAs`, `requireOne`, `readRejected`, unwrap, and a business-first enum. Run `go test ./internal/generator -run TestGoldenOutput<Name> -update` and commit the golden. A second run without `-update` must pass.

**Why:** A key that only a resource's generated files show can change without a test noticing.
