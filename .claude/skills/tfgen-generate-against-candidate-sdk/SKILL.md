---
name: tfgen-generate-against-candidate-sdk
description: "Use when tfgen generate must run before the management SDK is synced from a cx-management-apis PR (SDK_SYMBOL_MISSING for new contract fields). Builds a candidate SDK in a private GOMODCACHE."
---

# Generate against a candidate SDK

**Trigger:** `tfgen check` passes on a candidate `apidocs.swagger.json`, but `generate` needs SDK symbols that no released SDK has yet.

**Fix:**
1. Make a private module cache: symlink every entry of `$(go env GOMODCACHE)` into it, except a real copy (`chmod -R u+w`) of `github.com/coralogix/coralogix-management-sdk@<pinned version>`. `tfgen` honors `GOMODCACHE` and loads packages with `GOPROXY=off`, so every other module must be reachable.
2. In the copy, replace the resource's paths and their reachable `components.schemas` in `openapi.yaml` with the candidate spec.
3. Rewrite `specs/<tag>.json` for the affected tags only (paths of that tag plus the schemas they reach). The Rust splitter needs a newer cargo than many machines have.
4. Run `go/scripts/sanitize_openapi_for_go_codegen.py`, then openapi-generator for each tag with the flags of `go/scripts/generate_openapi.sh`. Without Java, its Docker fallback needs `OPENAPI_TOOLS_CONFIG=openapitools.json`, and the input spec must sit inside the SDK copy (Docker mounts `$PWD`).
5. Build the SDK from the published spec (`OPENAPI_VISIBILITY_FILTER=DEPRECATED`), as the SDK sync does: a `DEV` field is not in the SDK. Use a private `GOCACHE` too, because Go caches module directories as immutable and keeps stale file lists after you regenerate the SDK.
6. `GOMODCACHE=<private> go run ./cmd/tfgen generate ...` and `GOMODCACHE=<private> GOPROXY=off go build ./...`. Never commit a `replace` or a pin to the candidate.

**Docs from a worktree:** `tfplugindocs` takes the provider name from the directory name. Run it with `--provider-name coralogix`, then keep only the changed resource pages and restore their `page_title` suffix `terraform-provider-coralogix`.

**Why:** The generator's SDK symbol checks read the pinned module from the module cache. A private cache lets you check the generated code before the SDK sync, without touching the shared cache.
