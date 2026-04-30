# CLAUDE.md

Project guidance for AI coding agents (Claude Code, Codex, Cursor, etc.) working in this repository. `AGENTS.md` is a symlink to this file so vendor-neutral discovery works alongside Claude Code's documented entry point.

## Common commands

```bash
make build       # Compile the provider binary
make install     # Build and install into ~/.terraform.d/plugins/locally/debug/coralogix/1.5/<os_arch>
make test        # Unit tests (parallel, 30s timeout)
make testacc     # Acceptance tests (TF_ACC=1, 120m timeout) — hits real Coralogix APIs
make generate    # Regenerate docs/ via terraform-plugin-docs (note: `git checkout -- docs/guides` runs after)
```

Run a single test:
```bash
go test ./internal/provider/... -run TestAccCoralogixResourceAlert -v -timeout 30m
TF_ACC=1 go test ./internal/provider/alerts/... -run TestAccAlert -v
```

Local end-to-end smoke against examples: `./exec_examples_locally.sh` (rewrites every `examples/resources/*.tf` to point at `locally/debug/coralogix` v1.5, runs `terraform plan`, then reverts).

The Makefile's `OS_ARCH=darwin_arm64` may need editing for non-Apple-Silicon dev machines. Pair `make install` with a `~/.terraformrc` dev-overrides block pointing at `~/.terraform.d/plugins/locally/debug/coralogix`.

## Required environment for running the provider

- `CORALOGIX_API_KEY` — required.
- Either `CORALOGIX_ENV` (e.g. `EU1`, `US1`, `AP3`, etc.) **or** `CORALOGIX_DOMAIN` (e.g. `coralogix.com`) — exactly one. Setting both is a configuration error. The full env→gRPC URL map lives in `internal/provider/provider.go`.

## Architecture

**Mux server (terraform-plugin-mux).** `main.go` combines two provider implementations behind one binary:
- `provider.OldProvider()` — legacy `terraform-plugin-sdk/v2` provider, upgraded via `tf5to6server`. Hosts the older resources still on SDKv2 (`coralogix_rules_group`, `coralogix_enrichment`, `coralogix_data_set`, `coralogix_hosted_dashboard`, `coralogix_grafana_folder`).
- `provider.NewCoralogixProvider()` — `terraform-plugin-framework` provider. Hosts everything else and is where new resources should be added.

Both implementations live in `internal/provider/provider.go`. To add a new resource/data source on the framework path, register it in the `Resources()` / `DataSources()` slices there.

**Resource packages.** Each Coralogix domain lives in its own subpackage under `internal/provider/<domain>/`, with `resource_*.go`, `data_source_*.go`, and `*_test.go` files. The cross-cutting `*_test.go` files at `internal/provider/` root are the acceptance-test entry points that import resource-specific fixtures from `examples/`.

**Client layer.** `internal/clientset/` is the single factory for backend clients. `clientset.NewClientSet(env, apiKey, grpcURL)` returns a struct holding ~30 typed clients (alerts, dashboards, SLOs, TCO, AAA, notifications, …). Most are gRPC clients from `coralogix-management-sdk`; a few use REST (`rest/client.go`, `grafana-client.go`, `groups-client.go`). `callPropertiesCreator.go` builds the auth/metadata for outgoing gRPC calls. The clientset is what `Configure` hands to every resource as `ResourceData`/`DataSourceData`.

**Shared helpers.** `internal/utils/` has constants and generic helpers (map/slice utilities, schema helpers) used across all resource packages.

**Docs are generated.** `docs/` is produced by `tfplugindocs` (`make generate`) from schema descriptions and `examples/`. Don't hand-edit files under `docs/resources/` or `docs/data-sources/` — edit the schema and example, then regenerate. `docs/guides/` is hand-written and explicitly preserved by the Makefile's `git checkout -- docs/guides`.

**Examples are public contract.** `examples/resources/coralogix_<name>/resource.tf` and `examples/data-sources/coralogix_<name>/data-source.tf` are pulled into generated docs and are smoke-tested by `exec_examples_locally.sh`. Keep them runnable.

## Pre-commit

`.pre-commit-config.yaml` runs `gofmt`, `goimports`, `go vet`, `golangci-lint`, `go-cyclo` (limit 15), `go-mod-tidy`, `go-build`, and `go-unit-tests` on every commit.

## API source of truth (for feature work)

When adding support for a new API field or method, three GitHub repos are in play, listed in flow order:

- **[`coralogix/cx-management-apis`](https://github.com/coralogix/cx-management-apis)** — canonical gRPC service definitions. All released APIs originate here. Feature tickets typically link directly to a `proto` file in this repo.
- **[`coralogix/openapi-facade`](https://github.com/coralogix/openapi-facade)** — HTTP/REST proxy that translates REST clients to the gRPC backends. Versioned by quarter (`dec25`, `jan26`, `may26`, …). Useful for verifying how an API surfaces over REST.
- **[`coralogix/coralogix-management-sdk`](https://github.com/coralogix/coralogix-management-sdk)** — multi-language SDK consumed by this provider. Go bindings live under `go/openapi/gen/<service>/`. The exact version is pinned in this provider's `go.mod` (often a date-stamped pseudo-version like `v1.9.4-0.20260419...`).

The provider depends only on the Go SDK. The SDK's own `proto/` directory can lag upstream; treat the **generated Go types at the pinned version** as authoritative for "is this field available to me?".

### Workflow when the ticket asks for a new field X

1. **Read the proto** linked in the ticket (in `cx-management-apis`) to confirm the field's shape and parent type.
2. **Verify the pinned SDK has the generated Go type:**
   ```bash
   grep coralogix-management-sdk go.mod                       # find pinned version
   grep -rn "FieldName" ~/go/pkg/mod/github.com/coralogix/coralogix-management-sdk@<version>/go/openapi/gen/<service>/
   ```
   Found → no SDK bump needed; jump to step 4.
   Absent → step 3 first.
3. **The SDK type needs to land upstream first.** One observed precedent (PR #491):
    - Identify or open a paired SDK PR in `coralogix/coralogix-management-sdk` adding the proto/Go type.
    - Open the provider PR in parallel; cite the SDK PR URL in the description with a "merge only after" note.
    - After the SDK PR merges and a new pseudo-version is published, bump `go.mod` on the provider PR and rebuild.
4. **Add the schema attribute, model field, extractor, and flatten** in `internal/provider/<domain>/`.
5. **Verify empirically** against an env (e.g. EU2) — the proto and the API can diverge in subtle ways (validators, mutual-exclusion rules, default behaviour). Don't trust schema-vs-API alignment without a roundtrip.

### Bumping the SDK in `go.mod` (standalone bumps)

Sometimes a bump is needed independent of feature work — to pull in fixes or to track a quarterly LTS release.

- Title convention: `Bump SDK` or `Bump SDK for <month> release` (precedents: #506, #461).
- A bump usually requires **adapting many resource files** to the new SDK API surface (renamed types, changed signatures). #506 touched 17 resource files alongside `go.mod`. That's expected; bundle the bump and the adaptations in the same PR — they're not separable.
- Prefer LTS releases (the SDK uses `x.6.x` in June each year) for stable shipping.
- After the bump: `go mod tidy && go build ./... && go vet ./...`, then the relevant `make testacc` runs to catch behaviour drift the compiler can't.

## Skill maintenance (dynamic)

Project skills live at `.claude/skills/<skill-name>/SKILL.md`. The directory is symlinked from `.cursor/skills` so Cursor users discover the same files; if a future tool needs them, add a similar symlink for that tool's convention rather than duplicating files.

Treat the skills directory as a living knowledge base — when a bug fix or issue reveals non-obvious, repeat-able knowledge, capture it as a skill so future sessions inherit it. Don't capture things already derivable from the code, git history, or this file.

**When to create a new skill** (after finishing the actual fix):
- A bug fix uncovered a footgun that's likely to bite again (e.g., a Plan-Modifier that *must* be set to avoid spurious diffs on a specific attribute type, an SDKv2-vs-framework gotcha, a gRPC enum mapping that breaks silently).
- A repeatable diagnostic procedure emerged (e.g., "to debug a `coralogix_dashboard` import panic, check X, then Y").
- A migration recipe applies to >1 resource (e.g., porting a resource from SDKv2 to plugin-framework).

**When to update an existing skill** instead of creating a new one:
- The fix is a new edge case of an already-documented pattern → append to the existing SKILL.md.
- A documented assumption is now wrong → correct it in place; don't leave stale guidance.

**When NOT to create a skill:**
- One-off fixes with no generalizable lesson.
- Anything covered by Go/Terraform docs or already in this file.
- Vague "be careful with X" advice without a concrete trigger and action.

**Keep skills short** — ~20 lines, index-entry sized. A fresh Claude should read it in 30 seconds and know what to do. If it needs >30 lines, it's probably two skills. Don't include current-state inventories ("attribute X still has the bug") — those rot fast and belong in PR descriptions.

**Skill file layout:**

```markdown
---
name: <kebab-case-name>
description: <one sentence — concrete triggers Claude matches against (resource names, error strings, file paths)>
---

# <Title>

**Trigger:** <when this applies, in one line>

**Fix:** <what to do — code snippet if useful>

**Why:** <one sentence — the underlying reason, so future-you can judge edge cases>
```

**Naming:** kebab-case, scoped to the problem, not the resource (e.g., `framework-plan-modifier-for-set-of-objects`, not `fix-alert-bug-2026-04`). A good name reads like an index entry.

**Workflow at end of a bug-fix session:**
1. Ask: "did I just learn something a fresh Claude wouldn't?"
2. If yes, check `.claude/skills/` — is there an existing skill to extend?
3. Otherwise, create a new skill. Keep it tight; one specific lesson per skill beats a sprawling catch-all.
4. Commit the skill alongside the fix so reviewers can sanity-check the captured lesson.
