# OpenAPI generator vs. our hand-written resources

**TL;DR — the HashiCorp OpenAPI generator does not work for `coralogix_alert` or
`coralogix_parsing_rules` as-is.** Both resources model a discriminated union with
`oneOf`, which the generator explicitly does not support, so out of the box it drops
both resources entirely and emits an empty spec. Even after manually rewriting the
specs to remove the `oneOf`, what comes out is a schema + type-plumbing only — no
CRUD, no expand/flatten, no validation, and a schema shape that is semantically
wrong for a union. It is not a viable replacement for what we maintain today; at
best it is a starting scaffold for *new, structurally simple* resources.

## What was run

- Tool: `tfplugingen-openapi` + `tfplugingen-framework` (HashiCorp code-gen pipeline).
- Input: the split OpenAPI specs from `coralogix-management-sdk/specs/`:
  - `alert_definitions_service.json` (`/alerts/alerts/v3`)
  - `rule_groups_service.json` (`/parsing-rules/rule-groups/v1`)
- Configs: `codegen/configs/*.yml` map POST/GET/PUT/DELETE → create/read/update/delete.

## Result 1 — out of the box: both resources are dropped

```
WARN skipping resource schema mapping resource=alert
     err="found 16 oneOf subschema(s), schema composition is currently not supported"
WARN skipping resource schema mapping resource=rule_group
     err="found 4 oneOf subschema(s), schema composition is currently not supported"
```

The generated Provider Code Spec contains only the provider name — **zero resources,
zero data sources**:

```json
{ "provider": { "name": "coralogix" }, "version": "0.1" }
```

Root cause: `AlertDefProperties` uses a root-level `oneOf` over its 16 alert-type
variants (logs-threshold, metric-threshold, flow, logs-anomaly, tracing, SLO, …);
`RuleGroup.ruleMatchers` uses `oneOf` over its 4 matcher variants. `oneOf`/`anyOf`/
`allOf` composition is [a documented non-feature](https://developer.hashicorp.com/terraform/plugin/code-generation/openapi-generator#schema)
of the generator. When it hits one in a resource's root schema, it discards the
whole resource, not just the field.

## Result 2 — with a manual `oneOf`-stripping workaround

After preprocessing the specs to delete the root `oneOf` (see `gen_flat_specs.py`),
the generator does produce output — but note what that output is:

| | generated `alert` | generated `rule_group` |
|---|---|---|
| Top-level attributes | 6 | 11 |
| Generated Go | **160,202 lines / 4.2 MB** | 17,533 lines / 463 KB |
| Model structs | 694 | 73 |
| Functions | 5,900 | ~500 |
| Create/Read/Update/Delete | **none** | **none** |
| expand / flatten to SDK types | **none** | **none** |
| Validators (mutual exclusion, bounds) | **none** | **none** |
| Plan modifiers | **none** | **none** |
| Client wiring / Configure / ImportState | **none** | **none** |

`tfplugingen-framework generate resources` emits only `XxxResourceSchema()` plus the
custom `attr.Type`/`attr.Value` boilerplate for every nested object. There is no
`Create`/`Read`/`Update`/`Delete`/`Metadata`/`Configure`/`ImportState` — you write all
of that, plus the mapping to/from the SDK Go types, by hand.

And the generated Go **does not even compile**. Because the flattened schema exposes
the same nested object in both the request and response shapes, the generator emits
each custom type twice:

```
codegen/_output/go_parsing_rules/rule_group_resource_gen.go:9616:6:
    RuleMatchersType redeclared in this block
    other declaration at :1909  (ApplicationNameType, and many more, likewise)
```

So even the "best case" output is not buildable without hand-editing. (The generated
files live under `codegen/_output/`, an underscore-prefixed dir Go tooling skips, so
they don't break `go build ./...` or pre-commit.)

Worse, the schema it produces is **semantically wrong**:

1. **Union collapsed to flat optionals.** All 16 alert-type variants become
   independent optional attributes at the same level. Nothing enforces that exactly
   one is set — the whole point of the union. Our hand-written schema encodes this
   with `ExactlyOneOf`/`ConflictsWith` (261 validator references in `alert_schema/`).
2. **Request and response schemas are unioned, producing duplicates.** The alert
   resource comes out with both `alert_def_properties` (from the create body) *and*
   `alert_def` (the read-response wrapper) *and* `access` — overlapping
   representations of the same data, because the generator naively merges the create
   request shape with the read response shape.
3. **Expression-language `<v1>` prefix, enum `UNSPECIFIED` handling, cross-midnight
   time windows, backend-set defaults** — none of the domain rules we rely on survive,
   because they live in behavior, not in the JSON schema.

## Side-by-side with what we ship today

| | Generated (best case) | Hand-written today |
|---|---|---|
| `coralogix_alert` | schema stub only, 160k LoC, no logic | `resource_coralogix_alert.go` 5,033 LoC + `alert_schema/` (v2 935, v3 1,006) + `alert_types/common.go` 753 |
| Alert schema versions / `UpgradeState` | not generated | v1→v2→v3 upgraders wired |
| Alert union handling | flattened, unvalidated | per-type nested blocks + `ExactlyOneOf` |
| `coralogix_parsing_rules` | schema stub only, 17k LoC, no logic | `resource_coralogix_parsing_rules.go` 1,202 LoC (framework, OpenAPI-SDK based) |
| `coralogix_rules_group` (legacy) | n/a | `resource_coralogix_rules_group.go` 1,154 LoC (SDKv2) |
| CRUD + expand/flatten | **you write it** | present |
| Validators / plan modifiers / import | **you write it** | present |

## Assessment

- **Not usable for alerts or parsing rules.** The core of both resources is a
  discriminated union, which is exactly the case the generator refuses. The manual
  `oneOf`-stripping workaround produces a schema that would mislead users (no mutual
  exclusion) and a 4.2 MB file no one wants to review or maintain.
- **The hard part is not the schema — it's the mapping and the rules.** Even where
  the generator produces a schema, it produces 0% of the CRUD, expand/flatten,
  validators, plan modifiers, state upgraders, and import logic that make these
  resources correct. That is where essentially all of our maintenance cost lives.
- **Where it could help:** brand-new, structurally simple resources (flat objects,
  no `oneOf`, no schema-version history) — as a one-time schema scaffold that a human
  then fills in. It is not a fit for regenerating the resources in this comparison.

### If we wanted to pursue this seriously

1. Get `oneOf` support upstream, or add a discriminator-aware preprocessing step that
   turns each union into a set of mutually-exclusive nested blocks (not a flat merge).
2. Split create-request vs. read-response mapping so we don't get duplicate attributes.
3. Layer our validators / plan modifiers / upgraders on top via the generator's
   `ignores` + custom-type hooks — none of which is generated for us today.

Given (1)–(3), the generator would still only produce the schema; the CRUD and
mapping stay hand-written. The cost/benefit does not favor migrating these two.
