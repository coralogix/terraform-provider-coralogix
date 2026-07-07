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

`.pre-commit-config.yaml` runs Gitleaks, `gofmt`, `goimports`, `go vet`, `golangci-lint`, `go-cyclo` (limit 15), `go-mod-tidy`, `go-build`, and `go-unit-tests` on every commit.

## Adding API support / SDK considerations

The provider depends on the public Go SDK at [`coralogix/coralogix-management-sdk`](https://github.com/coralogix/coralogix-management-sdk). That SDK is the authoritative surface for "is this API field available to me?". This repo does not commit `.proto` files.

**To check whether a field exists in the pinned SDK:**

```bash
grep coralogix-management-sdk go.mod                       # find pinned version
grep -rn "FieldName" ~/go/pkg/mod/github.com/coralogix/coralogix-management-sdk@<version>/go/openapi/gen/<service>/
```

If the SDK's generated Go types don't include the field, an SDK release has to land first. If they do, wire the schema, model, extractor, and flatten in the relevant `internal/provider/<domain>/` package.

If generated SDK types are ambiguous about presence, defaults, validation, or required fields, do not guess from Go zero values alone. Verify behavior against the API before adding Terraform validators or schema defaults.

When the resource has a paired data source (`data_source_*.go`), check whether it delegates to the resource's `Schema()` method — many in this repo do, which means new attributes flow through automatically. Quick check: `grep "var r.*Resource" internal/provider/<domain>/data_source_*.go`.

**Verify empirically.** API behaviour can diverge from what the SDK's generated Go types suggest — runtime business rules (mutual exclusion, value bounds, defaults) often aren't expressed in the types. Always test against a real environment before encoding constraints in the schema. Wrong validators that block valid configs are harder to undo than missing ones. Examples that surfaced in this codebase:

- `LogRules.dpxl_expression` and `LogRules.severities` are mutually exclusive at the API even though the Go types don't enforce it.
- `UsageTier.daily_quota_percentage` is bounded to `0–100` even though the Go type is plain `float64`.

### Coralogix expression languages use a `<v1>` version prefix

Fields that take expressions — `coralogix_tco_policies_logs.dpxl_expression`, `coralogix_scope.default_expression`, `coralogix_scope.filters[*].expression` — require a version tag at the start of the string (e.g. `<v1> $d.severity == 'INFO'`, `<v1>true`). Bare expressions are rejected at API compile time. When adding a new expression-typed field, mention the prefix in `MarkdownDescription` and include it in test fixtures.

### Bumping the SDK in `go.mod`

A bump usually requires adapting many resource files to the new SDK API surface (renamed types, changed signatures). PR #506 touched 17 resource files alongside `go.mod`. That's expected — bundle the bump and the adaptations in the same PR; they're not separable. After the bump: `go mod tidy && go build ./... && go vet ./...`, then the relevant `make testacc` runs to catch behaviour drift the compiler can't.

## Changelog

Every PR must update `CHANGELOG.md`. The CI `changelog-check` workflow enforces this and will block merges if the file is untouched. Add a `skip changelog` label to the PR to bypass the check.

Exception: non-user-facing repository hygiene changes, such as pre-commit hook updates or CI-only changes, do not need a changelog entry unless the user explicitly asks for one.

**Where to add your entry:**

- **Normal PR** — add bullet points under the `# Unreleased` section at the top of the file.
- **Release PR** (the PR that bumps `internal/clientset/version.go` to a new version) — rename `# Unreleased` to `# Release X.Y.Z` matching the new version, and add a fresh empty `# Unreleased` section above it.

Keep entries concise: one line per change, prefixed with the affected resource/data-source path when relevant. Examples:

```markdown
# Unreleased

#### resource/coralogix_alert
- FEAT: Add support for `no_data_policy` condition.
- FIX: Nil pointer dereference on import when `scheduling` is unset.

#### resource/coralogix_dashboard
- FIX: Incorrect unit mapping for gauge widgets.

#### provider
- FEAT: Add support for `AP4` region.
```

## Public-repo discipline

This is a public repo (`coralogix/terraform-provider-coralogix`). Internal ticket identifiers (`BUGV2-`, `CX-`, etc.) belong in **commit messages, PR descriptions, and branch names** — not in committed code, comments, doc strings, test fixtures, or example HCL. Use descriptive names instead.

A simple grep before committing catches leakage:

```bash
git diff master.. -- internal docs examples | grep -iE "BUGV2-|CX-[0-9]" || echo "✓ none"
```

## Review instructions

- **Set/unset semantics:** Check that removing an attribute from HCL really clears it when intended. Be suspicious of `Optional+Computed` plus `UseStateForUnknown()` on fields users may remove; it can silently preserve prior state. Also verify explicit empty lists are either accepted and round-trip, or rejected with a clear validator.
- **Plan/state round-trip:** For every schema field touched, trace schema → model → extract/expand → API → flatten → state. Watch for defaults that the API ignores, backend-generated IDs that need `Optional+Computed`, proto `UNSPECIFIED` enum values leaking into state, null vs empty list drift, and schema versions that were not updated together.
- **Null, unknown, and import safety:** Validators and extractors must tolerate `null`, `unknown`, and variable-derived values without panics. Import reads often start with only an ID, so dynamic or required-looking nested values must be hydrated from the backend before conversion.
- **CRUD error paths:** After `resp.Diagnostics.AddError`, return immediately. A failed create/update must not continue into flatten/state writes, because empty IDs or zero-value state can poison later reads and plans.
- **API behavior hidden by generated types:** Do not infer semantics from Go zero values alone. Check for exact numeric/string conversions, time windows that cross midnight, host/domain routing special cases, backend-only defaults, and hard-coded request values that users cannot express in schema.
- **Regression coverage:** Prefer tests that exercise apply → read → second plan, set → change → remove for optional fields, import, unknown/variable config, and API-returned optional blocks. Many past bugs only appeared on the second plan or on update/import, not on initial create.

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
description: "Use when <concrete trigger — resource names, error strings, file paths, scenarios>. <One short clause on what it does.> Do NOT use for <obvious anti-pattern, when relevant>."
---

# <Title>

**Trigger:** <when this applies, in one line>

**Fix:** <what to do — code snippet if useful>

**Why:** <one sentence — the underlying reason, so future-you can judge edge cases>
```

**Description format** (frontmatter):
- Always quote the value (single line, in `"…"`). Strict YAML parsers choke on unquoted descriptions that contain colons, parentheses, or other punctuation.
- Start with `Use when …` so it matches the documented Anthropic Skills convention and other agents (Cursor, Codex) discover it consistently.
- Keep under ~200 characters. The description is what tools match against to decide relevance — be specific and concrete, avoid filler.

**Naming:** kebab-case, scoped to the problem, not the resource (e.g., `framework-plan-modifier-for-set-of-objects`, not `fix-alert-bug-2026-04`). A good name reads like an index entry.

**Public surface:** `.claude/skills/` is checked into a public repo and indexed by external tooling. No customer names, no internal-only ticket bodies, no non-public URLs. Treat skills like docs you'd be comfortable showing to anyone who reads the repo.

**Workflow at end of a bug-fix session:**
1. Ask: "did I just learn something a fresh Claude wouldn't?"
2. If yes, check `.claude/skills/` — is there an existing skill to extend?
3. Otherwise, create a new skill. Keep it tight; one specific lesson per skill beats a sprawling catch-all.
4. Commit the skill alongside the fix so reviewers can sanity-check the captured lesson.
