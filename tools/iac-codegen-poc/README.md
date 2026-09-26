# IaC code generation POC

A proof of concept: generate Terraform code from OpenAPI. Two modes:

- **Resource mode:** a complete Terraform resource, with no handwritten resource code. It generates `ai_evaluation`
  from the AI Evaluations API, and fake resources for the shapes that it does not have.
- **Type mode:** only the Terraform types (schema attributes, models, expand, flatten) of API types, in their own package.
  Handwritten resources embed them, for example a new alert type in the handwritten alert resource. See "Type mode".

This folder is a separate Go module. It does not change the provider build. The generated resource is
not registered in the provider. The handwritten `coralogix_ai_evaluation` resource stays as it is.

**The generated files:** [`generated/aievaluation/`](generated/aievaluation/) (the real API),
[`generated/fakeboard/`](generated/fakeboard/) (a fake API, see "Fake resource"),
[`generated/fakesettings/`](generated/fakesettings/) (a fake singleton, see "Singletons"), and
[`generated/fakerule/`](generated/fakerule/) and [`generated/fakeview/`](generated/fakeview/) (fake full-replace
updates, see "Full replace"). Type mode: [`generated/fakepanel/`](generated/fakepanel/) (fake types) and
[`generated/dashboardwidgets/`](generated/dashboardwidgets/) (the 9 real dashboard widget types).

| File | Content |
|---|---|
| `schema.go` ([ai](generated/aievaluation/schema.go), [fake](generated/fakeboard/schema.go)) | Schema: attributes, types, validators, defaults, plan modifiers |
| `model.go` ([ai](generated/aievaluation/model.go), [fake](generated/fakeboard/model.go)) | Model: Go structs for the plan and the state |
| `convert.go` ([ai](generated/aievaluation/convert.go), [fake](generated/fakeboard/convert.go)) | Expand (plan → SDK request) and flatten (SDK response → state) |
| `mask.go` ([ai](generated/aievaluation/mask.go), [fake](generated/fakeboard/mask.go)) | Update mask: the fields that changed between the plan and the state. Top-level names for `ai_evaluation`, leaf paths for the fake. |
| `replace.go` ([rule](generated/fakerule/replace.go), [view](generated/fakeview/replace.go)) | Instead of `mask.go` when Update is a full replace (`PUT`): the whole Update body, and no request when nothing changed |
| `resource.go` ([ai](generated/aievaluation/resource.go), [fake](generated/fakeboard/resource.go)) | Create, Read, Update, Delete, Import |
| `acc_test.go` ([ai](generated/aievaluation/acc_test.go)) | The acceptance test, from the model and a values file (see "Acceptance test") |

The other `*_test.go` files in these folders are handwritten. They test the generated code.

## Architecture

```
openapi.yaml ──► 1. Read ──► model (Go structs) ──► 3. Write ──► Terraform resource (.go files)
                                   │
                     2. Check the names against the Go SDK
```

1. **Read** (`internal/model`): parse OpenAPI with `libopenapi`, and build a model of the resource.
   The model has no Terraform types and no SDK types.
2. **Check** (`cmd/tfgen`): load the pinned SDK with `go/packages`. Every type, field, and method that
   the output uses must exist. If one is missing, generation stops.
3. **Write** (`cmd/tfgen/templates`): fill `text/template` templates from the model, and format with `go/format`.

### Field location

Where a field appears in the API decides how Terraform treats it:

| Create | Update | Get | Behavior |
|---|---|---|---|
| Yes | Yes | Yes | Normal managed field |
| Yes | No | Yes | Immutable → `RequiresReplace` |
| No | No | Yes | Computed → read and store, never send |
| any other combination | | | Reject: do not generate |

## Overlay

The API does not follow the full contract yet. Its OpenAPI does not have some facts that the generator
needs, for example which Create fields are required. [`spec/overlay.yaml`](spec/overlay.yaml) adds these
facts. `cmd/overlay` applies it to `openapi.yaml` from the pinned SDK module and writes
`spec/openapi.patched.yaml`. It changes only the lines of each entry; all other lines stay byte for byte.
When the real spec has these facts, the overlay is not needed.

| # | Target | Change | Decision |
|---|---|---|---|
| O1 | Create request body | `required: [application, subsystem, config]` | D4 |
| O2 | Create `target`, `threshold` | presence marker `x-coralogix-presence: true` | D4a |
| O3 | Create `isEnabled` | `default: false` | D4b |
| O4 | Update request body | remove `application`, `subsystem`, `target` | D9 |
| O5 | Update `config`, `isEnabled`, `threshold` | presence marker | Proto `optional` in Update |
| O6 | The topic, table, competitor, and PII category lists | `uniqueItems: true`, `x-coralogix-collection: set` | D6 |
| O7 | `SqlLoadConfig.joinLimit`, `SqlLoadConfig.cteLimit`, `CustomEvaluationExample.score` | `format: uint64` | D7 |

## Pinned versions

| Item | Value |
|---|---|
| SDK Go module | `github.com/coralogix/coralogix-management-sdk v1.9.4-0.20260908121026-582cbc8f62f3` (the same as the provider) |
| Source spec | `openapi.yaml` at the root of that SDK module (`internal/sdkspec`) |
| Terraform Plugin Framework | `v1.17.0` (the same as the provider) |
| Go | 1.26 |

## Commands

Run them in this folder.

```sh
go run ./cmd/overlay --overlay spec/overlay.yaml --out spec/openapi.patched.yaml
go run ./cmd/tfgen --spec spec/openapi.patched.yaml --resource AiEvaluation --acc spec/acc/AiEvaluation.yaml --out generated/aievaluation
go run ./cmd/tfgen --spec spec/openapi.patched.yaml --resource AiEvaluation --sdk-names   # print the SDK names
testsdk/generate.sh                              # regenerate the test SDK (needs Docker)
go run ./cmd/tfgen --spec spec/openapi.patched.yaml --survey   # shapes that cannot be generated, for all Get resources
go run ./cmd/tfgen --spec spec/openapi.patched.yaml --survey-resources   # operations, ids, bodies, responses of writable resources
go run ./cmd/tfgen --spec spec/fake/settings.yaml --resource FakeSettings \
  --sdk-module github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen-poc/testsdk --out generated/fakesettings
go run ./cmd/tfgen --spec spec/fake/openapi.yaml --resource FakeBoard \
  --sdk-module github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen-poc/testsdk --out generated/fakeboard
go run ./cmd/tfgen --spec spec/fake/rules.yaml --resource FakeRule \
  --sdk-module github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen-poc/testsdk --out generated/fakerule
go run ./cmd/tfgen --spec spec/fake/views.yaml --resource FakeView \
  --sdk-module github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen-poc/testsdk --out generated/fakeview
go run ./cmd/tfgen --spec spec/fake/openapi.yaml --types Panel,Header,Interval,AbsoluteTime \
  --enums Color,Unit,Orientation,Comparison,Delivery --tag "Fake Boards Service" \
  --sdk-module github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen-poc/testsdk --out generated/fakepanel
go run ./cmd/tfgen --spec spec/openapi.patched.yaml --types Widget.Definition --tag "Dashboard service" --out generated/dashboardwidgets
integration/alertsanalytics/run.sh <provider checkout>   # plug generated alert types into a copy of the provider, run its tests
integration/enumnames/compare.sh <provider checkout>     # compare generated enum names with the provider's handwritten ones
go run ./cmd/tfgen --spec spec/fake/openapi.yaml --types Routing --tag "Fake Boards Service" \
  --sdk-module github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen-poc/testsdk \
  --overrides spec/fake/routing.overrides.yaml --out generated/fakerouting
go run ./cmd/schemacompare <handwritten dump> <generated dump>   # breaking and review differences of two schema dumps
integration/switch.sh globalrouter <provider checkout>   # switch a resource to generated types, prove no change (or connector)
STEP=added integration/switch.sh globalrouter <provider checkout>   # the same, with the API fields that Terraform did not have
CORALOGIX_ENV=EU2 integration/live.sh globalrouter <provider checkout>   # live check; prompts for the API key
go test ./...                                    # the acceptance test skips without TF_ACC
go test ./internal/model -run TestDump -update   # rewrite internal/model/testdata/ai_evaluation.golden
go test ./cmd/tfgen -run TestSDKNames -update    # rewrite cmd/tfgen/testdata/sdk_names.golden
go test ./schemadump -update                     # rewrite the generated dump and diff.golden
(cd tools/hwschema && go run . > ../../schemadump/testdata/handwritten.txt)   # dump the handwritten schema
```

A second generator run must give no diff.

## Differences from the handwritten resource

[`schemadump/testdata/diff.golden`](schemadump/testdata/diff.golden) shows exactly these differences:

- `target` and `threshold` are optional. Handwritten: required.
- `is_enabled` defaults to `false`. Handwritten: `true`.
- New computed attributes: `company_id`, `created_at`, `created_by`, `updated_at`.
- New `config` arms: `sql_load`, `custom_evaluation`.
- `target` values use API casing (`"RESPONSE"`), and allow `CONVERSATION`. Handwritten: lowercase, only `prompt` and `response`.
- Nested `config` fields are optional (F13). Handwritten: the 6 lists are required.
- `config` may have no arm: `Conflicting` instead of `ExactlyOneOf` (F20).
- `prompt_injection.additional_context` has no default. Handwritten: `""`.

## Acceptance test

The acceptance test is generated: [`generated/aievaluation/acc_test.go`](generated/aievaluation/acc_test.go). Its inputs:

- [`spec/acc/AiEvaluation.yaml`](spec/acc/AiEvaluation.yaml): the test values. The API owner fills it in, with the **API** names
  and value shapes (as in a request body), not the Terraform ones. `${name}` placeholders come from the environment.
  `skip` lists an Update field that cannot be tested, with the reason.
- [`generated/aievaluation/acc_env_test.go`](generated/aievaluation/acc_env_test.go) (handwritten): finds where the test can
  create an evaluation (the first AI application with a subsystem, and a free target), fills the placeholders, and deletes
  leftovers after the test.

The generator checks the values file against the spec: unknown fields, wrong types, enum values, `oneOf` arms, missing
required fields, and an Update field with no update value and no skip reason all fail generation. It converts the values
to HCL (`joinLimit: "10"` → `join_limit = 10`).

- Steps: Create → Import (verify) → one update step for each Update field → one clear step for each field that can be
  cleared (`threshold`) → Delete. Each update is in place and keeps the id. After each step, Terraform checks that a new
  plan is empty. At the end, the test checks that Get returns 404.
- `is_enabled` is skipped, because the server rejects `isEnabled` in the update mask (F25).
- Result on EU2: all steps pass.

```zsh
printf 'Coralogix API key: '; read -rs CORALOGIX_API_KEY; echo; export CORALOGIX_API_KEY
CORALOGIX_ENV=EU2 TF_ACC=1 go test ./generated/aievaluation -run TestAccAiEvaluation -v -count=1 -timeout 30m
```

## Fake resource

`ai_evaluation` has no maps, only two levels of nesting, and an API that accepts only top-level mask names.
[`spec/fake/openapi.yaml`](spec/fake/openapi.yaml) is a fake API that follows the full contract. It tests the other shapes.
No server implements it; the tests use a fake HTTP server.

- **Test SDK:** [`testsdk/generate.sh`](testsdk/generate.sh) generates the SDK of the fake API with the pinned SDK's own steps: its sanitizer,
  `openapi-generator` 7.17.0 in Docker with the SDK templates and options, and its regex fix. Only
  [`testsdk/go/openapi/cxsdk`](testsdk/go/openapi/cxsdk/cxsdk.go) is handwritten. `--sdk-module` selects it.
- **Nested objects:** three levels, a `oneOf` at level 1, level 3, in a list item, and in a map value, a list of objects,
  nested required fields.
- **Maps:** a map of strings, a map of objects, a map of 64-bit numbers inside a nested object. null is not sent; `{}` is sent.
- **Numbers:** `int32`, `int64` (signed), `float`, and `double` each get their own Terraform type (`Int32`, `Int64`, `Float32`,
  `Float64`), so a float keeps its digits (0.1 stays 0.1). Lists and maps of numbers and bools.
- **oneOf groups:** a `oneOf` with normal fields beside the arms, an object with several `oneOf` groups (`allOf` of `oneOf`), and a
  group at the resource root. A group in an object gets its validator on each arm, so it runs only when the object is set.
- **Real shapes from the API:** a `oneOf` with a `discriminator` field (as dashboards `SortStrategy`), and a base64 string with
  `format: byte` (as custom enrichments `File.binary`).
- **Leaf masks:** the fake mask pattern accepts dotted paths, so the generator names the changed leaves:

| Change | Mask |
|---|---|
| `header.text` | `layout.section.header.text` |
| Clear `header.color` | `layout.section.header.color`, with no value in the body |
| Font size inside the same `oneOf` arm | `layout.section.header.style.font.size` |
| `font` → `bold` | `layout.section.header.style.bold` (a new arm replaces the old one) |
| Remove `style` | `layout.section.header.style.font` (the old arm, with no value) |
| A value in a list item or a map | `layout.section.rows`, `layout.section.widths`, `labels` (replaced whole) |
| A oneOf group arm `auto` → `manual` | `layout.section.interval.manual` (only the new arm) |

The generator reads the mask pattern from the spec, and checks every mask path against it. `ai_evaluation`'s pattern accepts
only top-level names, so its `mask.go` has top-level masks.

## Survey

`go run ./cmd/tfgen --spec spec/openapi.patched.yaml --survey` measures, for the resource schema of every Get operation in the
spec, the shapes that the model or the generator cannot generate. It ignores the operations (it assumes `PATCH` with a mask).
The model reports every problem, not only the first. The last output is in
[`cmd/tfgen/testdata/survey.txt`](cmd/tfgen/testdata/survey.txt): all 43 resources have no issue. (AlertDef looked blocked by
an enum with only a `*_UNSPECIFIED` value, but those enums are `*_OR_UNSPECIFIED` enums with a real value; see F33, F50.)

## Resource shapes

`go run ./cmd/tfgen --spec spec/openapi.patched.yaml --survey-resources` groups the operations of each writable resource (a Get
and a Create or Update) and lists how its ids, request bodies, and responses look. The last output is in
[`cmd/tfgen/testdata/resource_survey.txt`](cmd/tfgen/testdata/resource_survey.txt). Most shapes that the generator does not
support are better prevented in the API, so they are linter rules, not generator features (F37–F41).

What the generator reads from OpenAPI:

- **Response form:** the 200 response `$ref` is the resource itself (for example with `response_body` in the proto), or a type
  with one field that points to the resource (`{aiEvaluation: {...}}`). Both work. Fields beside the resource are an error.
- **Empty responses:** a response object with no fields (many Delete responses) is a map in the SDK, and is accepted (F43).
- **Singletons:** a resource whose Get has no path parameter is a singleton: one per company, like
  `CompanyIpAccessSettings`. It is generated only when Create, Get, Update, and Delete are on one path. The API calls take
  no id, the Terraform `id` is a fixed, read-only value, and import accepts any id. A singleton with only Get and Update is
  not a Terraform resource, and generation stops with an error. [`spec/fake/settings.yaml`](spec/fake/settings.yaml) is the
  fake singleton; [`generated/fakesettings/resource_test.go`](generated/fakesettings/resource_test.go) tests its CRUD.

### Full replace

Most APIs that Terraform has and that change often (dashboards, alerts, quota, notification center, SLO) update with `PUT`, not
`PATCH` with a mask. The generator supports them as they are (D19):

- **Detection:** `_Replace<Name>`, or `_Update<Name>` with `PUT`, is a full replace. `_Update<Name>` with `PATCH` keeps the mask.
- **Paths:** `PUT /things/{id}` (View, TeamGroup), or `PUT /things` with the id in the body (E2M, Policy, Slo, ViewFolder).
  The generated code then puts the id from the state in the body.
- **Body:** every Update field, with no mask. A value that the plan removes is not sent, and the server clears it. Immutable
  fields are not sent. When no Update field changed, Update sends no request and reads the resource.
- **Server fields:** a full-replace body is often the whole resource, so it also has `createTime` and other server fields. A
  request property with `readOnly: true` is not sent (F46). Without it, generation stops.

```
PUT /fake/rules/v1
{"id":"r1","name":"errors","priority":5,"enabled":true,"tags":["a","b"],"condition":{"query":"level:error","threshold":2}}
```

Fakes: [`spec/fake/rules.yaml`](spec/fake/rules.yaml) (`PUT` on the collection, id in the body) and
[`spec/fake/views.yaml`](spec/fake/views.yaml) (`PUT` on `{id}`), tested in
[`generated/fakerule/resource_test.go`](generated/fakerule/resource_test.go) and
[`generated/fakeview/resource_test.go`](generated/fakeview/resource_test.go).

On the real spec, the `PUT` shape no longer stops any of the 20 full-replace resources. `ViewFolder` generates completely, with
the same SDK calls as the handwritten resource. Its only schema difference: `name` is optional, because its Create body has no
`required` list. The others stop on contract issues: a field in Update but not in Create (E2M, Policy, View), server fields
without `readOnly` (E2M, Slo), a request body that wraps the resource (Connector, GlobalRouter), or the response shape.

## Type mode

Most APIs that change often are existing, handwritten Terraform resources. Regenerating them would lose their custom code,
so the type mode generates only new parts, next to the handwritten code (D20):

```sh
tfgen --spec <spec> --types AnalyticsThresholdType,AnalyticsImmediateType --tag "Alert definitions service" --out <dir>/generated
```

It writes one package: the given types and every type inside them, each type once. For each root type `T`:

```go
generated.TAttributes() map[string]schema.Attribute  // the attribute that holds them decides Optional or Required
generated.TModel                                     // and a model, with <Model>AttrTypes(), for every type inside T
generated.ExpandT(ctx, p, m)                         // → *sdk.T, diag.Diagnostics
generated.FlattenT(ctx, p, v)                        // → *TModel, diag.Diagnostics
```

- `oneOf` rules become validators on the arms, relative to their object. They run only when the object is set, and for each list
  item or map value on its own. The handwritten code wires nothing.
- `--tag` selects the SDK package: the SDK copies a type into each package that uses it.
- The package doc records the command. A second run gives the same files.

**Real alerts.** Terraform has no `analytics_threshold` or `analytics_immediate` alert types (API types from 2026-06-15). The
generator writes them (681 lines). [`integration/alertsanalytics/run.sh`](integration/alertsanalytics/run.sh) copies the provider
(`git archive`), generates the types, applies [`provider.patch`](integration/alertsanalytics/provider.patch), and runs the alert
tests. The patch is the handwritten plug-in: about 50 changed lines in 5 files and a 94-line glue file. Most of it is existing
handwritten structure: each alert type sets the common alert properties itself, and 11 helper functions list every alert type
(F52). All provider unit tests pass, plus a round trip of analytics alerts through the handwritten expand and flatten.

**Real dashboards.** All 9 widget types of `Widget.Definition` generate
([`generated/dashboardwidgets/`](generated/dashboardwidgets/), 27,000 lines). Its
[`roundtrip_test.go`](generated/dashboardwidgets/roundtrip_test.go) reads real widget JSON with the SDK, flattens it, stores it in
a Terraform state of the generated schema, reads it back, and expands it, with no change. The fixtures are the provider's
dashboard test fixtures and example (line charts, data tables, dynamic widgets) and one hexagon with an absolute time frame.

These widgets cannot replace the handwritten ones: the handwritten schema has other names and shapes, and user configurations and
states depend on them.

| Handwritten | Generated from the API |
|---|---|
| `query.data_prime` | `query.dataprime` |
| `time_frame.absolute.start` / `end` | `time_frame.absolute_time_frame.from` / `to` |
| `threshold_type = "absolute"` | `threshold_type = "THRESHOLD_TYPE_ABSOLUTE"` |

Replacing a handwritten part needs overrides that keep the old names, and checks that prove no change: see "Overrides".

**Enum names.** Handwritten resources give API enum values their own Terraform names, in maps that a new API value does not
reach. `--enums` writes those maps from the spec (F54):

```go
generated.TextAlignmentByName   // "left" → TEXTALIGNMENT_TEXT_ALIGNMENT_LEFT, ...
generated.TextAlignmentNames    // []string{"left", "center", "right"}, for stringvalidator.OneOf(...)
generated.TextAlignmentName(v)  // the reverse; false for the value that only means "not set", and for unknown values
```

A name is the API value without the prefix that all its values share and without `_OR_UNSPECIFIED` / `_UNSPECIFIED`, in lower
case. [`integration/enumnames/compare.sh`](integration/enumnames/compare.sh) compares the rule with the provider's handwritten
maps: 293 of 327 names are the same, 20 differ only in letter case (`Debug`, `PHONE_NUMBER`), and 14 use other words (`euro` for
`EUR`, `avg` for `AVERAGE`). Those stay handwritten.

**Timestamps in requests** (`format: date-time`) are Terraform strings in RFC 3339, in UTC, as in the handwritten dashboard
resource. A generated validator accepts only the form that the API returns, so a value reads back as it was written:

```
from = "2026-09-26T08:00:00Z"        accepted
from = "2026-09-26T10:00:00+02:00"   Write "2026-09-26T10:00:00+02:00" as "2026-09-26T08:00:00Z": the API returns this form, ...
```

## Overrides

The type mode can also replace an existing handwritten resource (D21). Overrides make the generated schema the handwritten one,
so user configurations and states do not change. Later API changes then reach Terraform when the types are regenerated.

```sh
tfgen --spec <spec> --types GlobalRouter --tag "Global routers service" --overrides overrides.yaml --out <dir>/globalroutertypes
```

An overrides file is keyed by API type and API field, so one line covers every place that uses the type:

```yaml
wideNumbers: true                  # int32 and float as Int64 and Float64
types:
  GlobalRouter:
    id: {computed: true, useStateForUnknown: true, missingAsZero: true}
    fallback: {emptyAsNull: true, deprecationMessage: "Use `fallback_targets` instead."}
    createTime: {readOnly: true}   # set by the server: Computed only, never sent
  KeyInfo:
    keyPermissions: {inline: true} # the fields of the nested object in the parent
    value: {sensitive: true}
  FlowStages:
    timeframeMs: {int64: true}     # "60000" in JSON, 60000 in Terraform
  RoutingRule:
    entityType: {default: unspecified}
enums:
  notification_center.EntityType: {terraformNames: true, zero: unspecified}   # "alerts", not "ALERTS"
unwrap: [LuceneQuery]              # {"value": "..."} as the value itself
```

| Kind | Overrides |
|---|---|
| Schema | `name`, `skip`, `required`, `optional`, `computed`, `default` (also on a computed-only attribute), `defaultObject` (an object of its fields' defaults), `set`, `useStateForUnknown`, `useNonNullStateForUnknown`, `requiresReplace`, `deprecationMessage`, `sensitive`, `wideNumbers`, `wide` (one field) |
| Numbers | `int64`: a string with the pattern `^-?[0-9]+$` or `^[0-9]+$` and no format is an Int64 (F68). `string`: an int64 is a String |
| Enums | `terraformNames` (the enum name rule), `values` (names that differ), `zero` (a name for the value that only means "not set"), `acceptZero: false` (read it, but do not accept it in a configuration) |
| Reading a response | `missingAsZero` (a missing value is `""`, `false`, `0`, `[]`, an enum's proto zero value, or an empty object), `emptyAsNull` (an empty list, map, or object is null) |
| Server fields | `readOnly` (a read-only object makes everything inside it read only; the spec `readOnly` counts too) |
| Shape | `wrap`: a new object around API fields, for example the arms of a proto oneof (F62). `unwrap`: an object with one field shown as that field (F66). `inline`: the fields of a nested object or oneOf shown in the parent (F67). A wrapper can be computed |
| Handwritten parts | `custom`: one field keeps its handwritten attribute and converters; `namesArm`: an enum that names the set arm of a oneOf is filled in by expand |

An override that names a type, field, or enum value that does not exist is an error, so a stale file cannot hide an API change.
One change of shape is not supported yet: a `oneOf` written as a `type` string.

**Checks.** A switch must change nothing for users. Three checks run before the handwritten code is removed:

- [`cmd/schemacompare`](cmd/schemacompare/main.go) compares the schema dumps. A difference is breaking (a removed or renamed
  attribute, another type, `computed`, default, `RequiresReplace`, a lost `UseStateForUnknown`, an enum value no longer accepted)
  or for review (new validators from the spec, required → optional, new optional or computed attributes). An optional computed
  attribute with a null default is the same as an optional one.
- An equivalence test runs the old and the new code on the same API responses: the same Terraform state, and the same request.
  The schema cannot show how a handwritten flatten reads a response, for example a missing description as `""` (F57).
- A plan test makes every computed attribute without a default unknown, as Terraform plans an update, and reads it into the new model
  (`schemadump.PlanWithUnknowns`). States read from the API never hold unknown values, so the other checks cannot see a
  model that fails on a plan (F65).

**Scripts.** [`integration/switch.sh <name>`](integration/switch.sh) runs a switch in a copy of the provider, and
[`integration/live.sh <name>`](integration/live.sh) checks it on a real account. Each resource has its inputs in
`integration/<name>/`: `switch.env` (the root type, the tag, the packages), the overrides, `provider.patch`, the two checks, and
`live.tf`. The live check applies `live.tf` with the current provider, then plans with the switched one (no changes), imports with
both (the same state, the same plan), updates, and plans again.

**Pilot: `coralogix_global_router`.** `switch.sh globalrouter` copies the provider,
generates the types with [`overrides.yaml`](integration/globalrouter/overrides.yaml) (37 lines), runs both checks, and applies
[`provider.patch`](integration/globalrouter/provider.patch): the resource, the data source, and the schema use the generated
types, and the handwritten models, expand, and flatten are removed (+26 −658 lines). Results:

- The schema compare went from 17 breaking differences to 0, with 22 for review.
- The equivalence test found 9 differences in how the handwritten code reads responses; `missingAsZero` and `emptyAsNull` fixed
  them. 5 API responses give the same state and the same request.
- All provider unit tests pass.
- `live.sh globalrouter` on EU2: the current provider creates a connector and a router; the switched one plans no changes on
  that state and after an update, and imports the same state. It passed.
- With [`overrides-added.yaml`](integration/globalrouter/overrides-added.yaml), the fields that Terraform did not have
  (`create_time`, `update_time`, target ids) become computed attributes. The same checks and the live check pass.

**Partial switch: `coralogix_connector`.** Its `connector_config` has Terraform-only parts: write-only attributes for secrets, a
validator, and code that merges the secrets into the request and keeps them out of state. That part stays handwritten; the rest
(`id`, `name`, `description`, `type`, `config_overrides`) is generated. The resource model embeds the generated model (F61):

```go
type ConnectorResourceModel struct {
	connectortypes.ConnectorModel                             // generated
	ConnectorConfig types.Object `tfsdk:"connector_config"`   // handwritten
}
```

The checks run on the combined schema and code: 0 breaking differences, and the equivalence test (including a write-only secret)
passes. [`provider.patch`](integration/connector/provider.patch) is +63 −317 lines. The live checks pass for both steps.
`resolvedConnectorConfig` stays out of Terraform: it is read only, but it holds the secrets (F60).

**Wrappers: `coralogix_slo_v2`.** The handwritten SLO puts the arms of the proto oneofs `sli` and `window` in objects named after
them. The spec has no oneof names (F62), so the overrides add them:

```yaml
types:
  Slo:
    sli:    {wrap: [requestBasedMetricSli, windowBasedMetricSli, apmSli], required: true}
    window: {wrap: [sloTimeFrame], required: true}
```

```hcl
sli    = { request_based_metric_sli = { good_events = { ... }, total_events = { ... } } }
window = { slo_time_frame = "7_days" }
```

The compare shows no shape difference, and 0 breaking ones. The equivalence test passes on 9 API responses (each SLI kind,
ownership tags, the empty arrays that the server adds, a legacy filter value, missing values). Two rules that the API does not
describe stay handwritten, in a small file that runs before the generated read and after the generated write
([`add/`](integration/slo/add/), F64). The provider's SLO unit tests are rewritten against the generated model, with the same
assertions. The resource goes from 1,584 to 258 lines. The live checks pass for both steps. The first live run found F65, which
the plan test now covers. The request body is a separate copy of the SLO type, which stays handwritten (F63).

**Value objects: dashboards.** The dashboards API wraps many values in an object with one field (F66). The handwritten
resource shows the value itself, and `unwrap` does the same:

```json
{"logs": {"luceneQuery": {"value": "status:500"}}}
```

```hcl
logs = { lucene_query = "status:500" }
```

A null value sends no object. A missing object and an object with no value both read as null. A list of these objects is a list
of values. The value keeps its own read overrides. [`generated/fakeunwrap`](generated/fakeunwrap/unwrap_test.go) tests strings,
enums, sets, lists, objects, and the oneOf validators on unwrapped arms. On the real dashboard widgets,
`unwrap: [LuceneQuery, PromQlQuery, UUID]` makes 27 query attributes strings, and the real widget JSON files round-trip with no
change. Dashboards do not switch yet: they also need the `type` string rule.

**Nested objects in the parent: ApiKey and alerts.** Handwritten resources often show the fields of a nested API object as
fields of the parent (F67). `inline` does the same. It is on the field that holds the object, so each parent decides:

```json
{"name": "k", "keyPermissions": {"permissions": ["a"], "presets": ["p"]}}
```

```yaml
types:
  KeyInfo:
    keyPermissions: {inline: true}
```

```hcl
name        = "k"
permissions = ["a"]
presets     = ["p"]
```

Expand sends the nested object when at least one of its writable fields is set, and no object otherwise. A missing object reads
as null fields. An inlined field is required only when the object is required too. The inlined fields keep the overrides of
their own type. [`generated/fakeinline`](generated/fakeinline/inline_test.go) tests the same object inlined in two parents and
kept as an object in a third, read-only and unknown fields, and a required object.

The spec sends some 64-bit numbers as strings with no `format`, only a pattern (F68). `int64` adds the missing format, so
Terraform has an Int64. The generator now also reads `format: int64` on a string, so a fixed spec needs no override. `string`
is the reverse, for a handwritten String that holds a number (ApiKey `owner.team_id`).

**Handwritten fields and the alert type: alerts.** Some alert fields have a Terraform shape that no rule converts.
`of_the_last` is one string in Terraform and an object in the API:

```hcl
of_the_last = "10_MINUTES"   # or "1h30m"
```

```json
{"ofTheLast": {"metricTimeWindowSpecificValue": "METRIC_TIME_WINDOW_VALUE_MINUTES_10"}}
```

`custom` keeps the handwritten code for that one field. Everything around it is generated:

```yaml
customPackage: <import path of the handwritten converters>
types:
  MetricThresholdCondition:
    ofTheLast: {custom: {type: String, shape: 1dd67adb0272}}
```

The generated code calls three handwritten functions for the field: its attribute, expand, and flatten. `shape` is a
fingerprint of the API type of the field. When the API changes that type, the generator stops, shows the new type, and names
the handwritten converter to check. The converters are in their own package, which must not import the generated one.

An alert has one alert-type field set, and a `type` field that repeats its name
(`{"metricThreshold": {...}, "type": "ALERT_DEF_TYPE_METRIC_THRESHOLD"}`). Terraform has no `type`. With
`type: {namesArm: true}`, `type` is not an attribute, and expand fills it in from the set field. Every alert-type field must
pair with one `type` value by the enum name rule, and every value with a field, so a new alert type without a value stops the
generator. On the real spec, all 15 alert types pair. [`generated/fakealarm`](generated/fakealarm/alarm_test.go) tests both
rules.

**Alerts: a partial switch.** `coralogix_alert` (25 API commits in 12 months, 13 alert types) runs on generated types, with no
change for users. [`integration/alert/`](integration/alert/overrides.yaml) has the inputs:

- The overrides (about 280 lines, most of them the Terraform names of 26 enums).
- The handwritten parts ([`add/`](integration/alert/add/)): the schedule (times in the user's time zone, which the API has in
  UTC), the four `custom` converters (`of_the_last` twice, `latency_threshold_ms`, `override.priority`), and the Terraform-only
  rules: two deprecation warnings, the `group_by` validator and plan modifier, four size checks, and empty routing overrides.
- The checks. The schema compare has 0 breaking differences (302 with no overrides). The equivalence test runs the old and
  the new code on 25 API alerts (each alert type, minimal and empty ones, notifications, zero values): the same state on an
  import and on a read, and the same request. It found about 50 read rules that the spec does not describe (F72); removing one
  of them fails 16 cases. [`provider.patch`](integration/alert/provider.patch) switches the resource, the data source, and the
  state upgrades; the 11 alert unit tests are rewritten against the generated model with the same cases.
- Code size (the alerts package without the scheduler): handwritten 10,963 → 5,424 lines (the resource 5,051 → 507);
  generated 7,454 lines. The V1 and V2 schemas stay handwritten for the state upgrades.
- `live.sh alert` on a real account passed: the current provider creates 8 alerts; the switched one plans no changes, imports
  the same state, and plans no changes after its own update. The first runs found API rules that only the server checks (F75),
  and a lost `UseStateForUnknown`, which the compare now counts as breaking.

Left for later: the two analytics alert types and the new API fields (the second step), and the validators that the spec lacks
(F74).

**Next candidates.** A schema compare before and after these overrides:

| Resource | Breaking before | Breaking after | Left |
|---|---|---|---|
| `coralogix_api_key` | 13 | 2 | the `access_policy` JSON plan modifier; `presets`, which Get sends as objects and Create takes as names (F69) |
| `coralogix_tco_policies_logs`, `_traces` | 22, 18 | not measured | the request body is not the policy type, so it stays handwritten |

## Decisions

| # | Topic | Decision | Reason |
|---|---|---|---|
| D1 | Resource | `ai_evaluation` | Only 2 update operations in the whole spec have `updateMask`, and both are AI evaluations. `ai_custom_evaluation` has no Get by id. |
| D2 | Location | Local git repo, no remote | History, and `git diff` checks that a second run changes nothing. |
| D3 | Input | SDK spec + local overlay → `spec/openapi.patched.yaml` | Simulates plan steps 1–2. Nothing is published. |
| D4 | Create required | `application`, `subsystem`, `config` | A plain proto `string` with `min_length: 1` is effectively required (TF commit `ad3b08d`, "Restrict AI Evaluation Schema to match proto"). `config` defines what the evaluation does. |
| D4a | Optional | `target`, `threshold` | Proto `optional`. The TF test treats an existing evaluation with an empty `target` as a real case. |
| D4b | `is_enabled` | Optional, `default: false` | Plain proto `bool`: the server receives `false` when it is omitted. The handwritten default `true` is a Terraform-only choice. |
| D5 | Computed fields | Store all 5. Only `id` uses `UseStateForUnknown`. | Contract: read and store. OpenAPI cannot say which fields never change. If a reused value changes, Terraform fails with "inconsistent result after apply". |
| D6 | Collections | The 6 string/enum lists → Set. `customEvaluation.examples` → List. | Order has no meaning for these values. The handwritten resource uses Set. Tests both code paths. |
| D7 | 64-bit numbers | Overlay adds `format: uint64` to `joinLimit`, `cteLimit`, `score` → Terraform `Int64`, converted to and from string | Protobuf JSON sends 64-bit numbers as strings. The OpenAPI generator drops the type. The SDK has `*string`. |
| D8 | Update mask | Only changed top-level fields. A removed field goes into the mask and stays out of the body, so the server clears it. | Changed-field detection, as in the code generation plan. The mask pattern allows only top-level names. |
| D9 | Immutable | `application`, `subsystem`, `target` → `RequiresReplace`. The overlay removes them from the Update request. | The API should reject a change, so the proto should not include them in Update (F4). The handwritten resource also replaces on change, and its Update does not send them. |
| D10 | Tools | Overlay: own Go command `cmd/overlay`. Read OpenAPI: `libopenapi`. Check SDK names: `go/packages`. Write Go: `text/template` + `go/format`. | `libopenapi` keeps the property order. `kin-openapi` keeps properties in a Go map, so the order is lost. |
| D11 | Tests | Unit tests + acceptance tests on **EU2** | The acceptance test needs an API key, so it runs by hand. |
| D12 | Enum values | Use the API values as they are (`"RESPONSE"`, `"EMAIL_ADDRESS"`). No lowercase mapping. | No conversion code. Same values as the API docs. |
| D13 | `target` values | Allow all 3 (`PROMPT`, `RESPONSE`, `CONVERSATION`), as the spec says. | OpenAPI is the only input. The POC is not merged into the provider. |
| D14 | No Update field changed | Send no `PATCH`. `updateRequest` returns nil. Update then reads the resource with Get to fill the state. | The mask has `minLength: 1`, so an empty mask is invalid. `*` is not allowed. F6: no mask means "update what is in the body", which is not what we want. |
| D15 | Acceptance test values | Handwritten. The generator cannot know live values (for example a real AI application). Later, the API owner (backend developer) fills them in. | A spec example cannot know the environment. |
| D16 | Acceptance test value format | A YAML file (`spec/acc/<Resource>.yaml`) with `${name}` placeholders. A small handwritten Go hook fills the placeholders from the environment. | A backend developer edits only data. The generator checks the file. |
| D17 | Names and shapes in that YAML | The API JSON shape (`camelCase`, request-body values). The generator converts it to HCL. | Backend developers know the API, not Terraform. The same file can serve the Operator and other tools later. |
| D18 | Singletons | A singleton is a resource whose Get has no path parameter (one per company). Only a singleton with Create, Get, Update, and Delete on one path is generated. A singleton with only Get and Update is not a Terraform resource: not generated for now. | Terraform needs a real create and delete. (2026-09-25) |
| D19 | Full-replace Update (`PUT`) | Generate it as it is, with no change to the API: `PUT` on the item path, or on the Create path with the id in the body. A request property with `readOnly: true` is a server field and is not sent. `PATCH` keeps the update mask. | The frequently changed APIs that Terraform has (dashboards, alerts, quota, notification center, SLO) all use `PUT`. Forcing `PATCH` on them is a breaking change for customers and work for every team. (2026-09-26) |
| D20 | Generated parts inside handwritten resources (type mode) | A type mode, `tfgen --types A,B --tag <tag> --out <dir>`: the schema attributes, models, and expand and flatten of API types and every type inside them, in their own package (for example `dashboard_widgets/generated`). The handwritten code keeps the resource and plugs a type in with a few lines. Regenerating touches only that package. Existing handwritten code is never overwritten. | Most frequently changed APIs are existing, handwritten Terraform resources; regenerating them would lose custom code. In the last 12 months, 25 of 94 schema changes in Terraform-backed APIs added new objects to existing objects (dashboards 12, alerts 6). A separate package cannot clash with handwritten names. (2026-09-26) |
| D21 | Overrides for existing resources (option D) | A YAML file per type-mode package: `--overrides <file>`. Keyed by API component and field, so one line covers every place the type is used. Kinds: `name`, `computed` (with "keep the state value"), `default`, `set`, `skip`, `required`, enum value names (the E14 rule, one line per value that differs), and one package option for wide numbers (`Int64`, `Float64`). Added in the pilot: how a response is read (`missingAsZero`, `emptyAsNull`, F57), `readOnly` (F58), and `deprecationMessage`. Added for SLO: `wrap` (a new object around API fields, F62), a read-only object makes everything inside it read only, `useNonNullStateForUnknown`, `missingAsZero` on an enum (the proto zero value, its first value), enum `acceptZero: false`; the spec `readOnly` applies in the type mode. Added for dashboards: `unwrap`, a top-level list of objects with one field that Terraform shows as that field (`lucene_query = "..."` for `{luceneQuery: {value: "..."}}`, F66). Added for ApiKey and alerts: `inline` on the field that holds a nested object (each parent decides; the fields of the object become fields of the parent, F67), `int64` (a string with the pattern `^-?[0-9]+$` or `^[0-9]+$` is an Int64, F68), `string` (an int64 JSON number is a String, F68), and `sensitive`. Added for alerts: `custom` (one field keeps its handwritten attribute and converters; the override records a fingerprint of the field's API type, so an API change of that field stops the generator) and `namesArm` (an enum that names the set arm of a oneOf, such as the alerts `type`, is set by expand; every arm must pair with one value). Added for the alerts switch: `inline` on a oneOf, `defaultObject` (the default of an object is the object of its fields' defaults), a computed wrapper, a per-field `wide`, a default on a computed-only attribute, and `missingAsZero` on an object (a missing object reads as an empty one, and an empty one is sent as a missing one). A switch needs a schema compare and an equivalence test of the old and new flatten and expand. An override that names a missing component or field is an error. The spec of existing APIs does not change; only new APIs follow the contract. Structure changes left (a `oneOf` as a `type` string, an empty object as a bool) are out of scope until the pilot is reevaluated. | 72% of the handwritten attributes of 18 resources already match; most of the rest are flags, defaults, set vs list, enum names, and renames. The switch must not change user configs or state. Pilot: GlobalRouter. New API fields are skipped in the switch, then added in a second step. Generated validators replace the handwritten ones; the compare tool lists each one that is new. (2026-09-26) |

## Findings

Gaps in the API, the contract, or the tools.

| # | Finding | Owner area |
|---|---|---|
| F1 | The contract needs a way to mark computed fields that never change (for plan stability). | Contract |
| F2 | The proto should mark the 6 topic/table/competitor/PII lists as `UNORDERED_LIST`. | API proto |
| F3 | The OpenAPI generator must emit `format: int64/uint64` for 64-bit number fields. | OpenAPI generator |
| F4 | Remove `application`, `subsystem`, `target` from `UpdateAiEvaluationRequest`. | API proto |
| F5 | `is_enabled` in `CreateAiEvaluationRequest` must be proto `optional` (contract rule 3.3). | API proto |
| F6 | `PATCH` without `updateMask` is allowed today ("When omitted, only fields present in the body are updated"). The contract says it must return 400. | Backend |
| F7 | `CreateAiEvaluationRequest` has no `json_schema.required`. | API proto |
| F8 | The SDK example and the TF test helper can omit `subsystem`, but the TF schema requires it. One of them is wrong. | Tests |
| F9 | The spec is OpenAPI 3.1. The SDK's Rust splitter uses the `openapiv3` crate, which targets 3.0. Not a POC blocker. | Tooling |
| F10 | The spec is not valid for a strict validator. 1120 schemas have `externalDocs: {url: ''}`, and `info.title` is `''`. Both need a non-empty value. kin-openapi `Validate` passes when these two are fixed. | OpenAPI generator |
| F11 | `FilterPathAndValues` → `Filters.pathAndValues[]` → `FilterPathAndValues` is a `$ref` loop. An empty array ends it, so it is valid. libopenapi reports it as infinite by default. The generator must set `IgnoreArrayCircularReferences`. Not in the AI evaluation schemas. | Tooling |
| F12 | A YAML library cannot re-encode the spec without changes. `go.yaml.in/yaml/v4` load + dump changes about 3000 lines: it wraps `>-` folded strings at different points than the tool that wrote the spec. Any tool that edits the spec (the overlay, automated delivery) must splice text, or all tools must share one formatter. | Tooling |
| F13 | Nested `config` fields have no `required`. Example: `allowedTopics.topics` has `minItems: 1`, but it is not required. So the model marks every nested field optional. | API proto |
| F14 | Create lists two success responses, `200` and `201`, with the same schema. The model reads only `200`. | OpenAPI generator |
| F15 | Inline request bodies have no component name. openapi-generator names them `<operationId without _>Request`, for example `AiEvaluationsServiceCreateAiEvaluationRequest`. The generator copies this rule. A named component would remove the rule. | OpenAPI generator |
| F16 | An object with no properties has no SDK type. The SDK field is `map[string]interface{}`. The 10 empty `config` arms use it. | SDK generator |
| F17 | Names come from two sources with different casing. Package and client type come from the tag: `AI Evaluations Service` → `ai_evaluations_service`, `AIEvaluationsServiceAPIService`. Methods come from the operationId: `AiEvaluationsServiceGetAiEvaluation`. The accessor `cxsdk.ClientSet.AIEvaluations()` is handwritten; the generator assumes "tag without ` Service`". | SDK |
| F18 | The SDK is generated from the source spec, not the patched spec. So the overlay facts are not in the SDK types: required fields are pointers, uint64 fields are `*string`, and the Update body still has `application`, `subsystem`, `target`. The generator must use the SDK types. When steps 1–2 fix the real spec, the SDK types change (for example required → not a pointer), and `--sdk-names` fails until the rules follow. | Tooling |
| F19 | uint64 fields have `maxLength: 4` and `pattern: ^[0-9]+$` on the JSON string. The real number range is not in the spec. The generator adds only `AtLeast(0)`. | API proto |
| F20 | `EvaluationConfig` allows "no arm" (the `not: anyOf` entry). So `config = {}` is valid in the spec. The handwritten resource requires exactly one arm. If the API rejects an empty config, the proto oneof must be marked required. | API proto |
| F21 | Response presence is not in the contract. Flatten maps a missing value to null. If the server omits a zero value (for example `isEnabled: false`, `topics: []`), or sends a zero value for an unset field, the state differs from the plan, and Terraform fails with "inconsistent result after apply". The contract must say: a response has a value exactly when the request had one. The acceptance test on EU2 found no mismatch for set values, a cleared `threshold`, or a changed `config` arm. `isEnabled: false` was not checked, because Update cannot mask it (F25). | Contract / Backend |
| F22 | A uint64 value above 9223372036854775807 does not fit Terraform `Int64`. Flatten returns an error for it. A real range in the spec (F19) would let the schema reject it earlier. | API proto |
| F23 | The model does not keep the `updateMask` pattern. The generator checks each mask entry with its own copy of the contract rule (top-level name, no `*`). If the contract later allows nested paths, the generator must read the pattern from the spec. the model keeps the pattern (`UpdateMaskPattern`), and the generator checks every mask path against it. | Tooling |
| F24 | "Not found" has no machine-readable form. The SDK's `cxsdk.IsNotFound` parses the message text (`Not Found: <msg>`) to tell a missing resource from a missing route. The generated Read and Delete use only the status 404, like the handwritten resource. So a wrong route would remove the resource from the state. The contract should define a not-found reason. | Contract / Backend |
| F25 | The Update mask rejects `isEnabled`. The generator sends the API (JSON) name `isEnabled`. The gateway turns it into the proto path `is_enabled` (proto field `is_enabled = 6` in `UpdateAiEvaluationRequest`). The server answers 400 `Unknown field path "is_enabled" in update_mask`. So a documented Update field cannot be masked. No other client masks it: the SDK example masks only `config`, and the handwritten resource sends no mask. The contract must say: every field in the Update body is a valid mask path. A probe must check each one. | Backend / Contract |
| F26 | Dashboards (dry run with an assumed `PATCH` + mask): the Create body wraps the resource (`{dashboard, requestId}`), not flat fields. | API proto |
| F27 | Dashboards: Create returns only `dashboardId`, not the resource. | API proto |
| F28 | Dashboards: Get puts metadata beside the resource (`{dashboard, createdAt, authorId, isLocked, ...}`). The contract needs the resource only, with metadata as its fields. | API proto |
| F29 | Dashboards: an object has a `oneOf` and normal fields beside it (`IntervalResolution`: `auto` or `manual`, plus `useAdvancedLimit`). The model assumes that every field of a `oneOf` object is an arm. Generator limit. | Tooling |
| F30 | The SDK uses a value type (`string`, `Layout`), not a pointer, for a field that its source spec marks `required` (F18). The patched spec cannot tell which: `ai_evaluation` has `required` only in the overlay, so its SDK still has pointers. The generator now accepts `*T` or `T` and follows the SDK. | Tooling |
| F31 | A test needs a valid value for each field, and a second one for updates. The spec has `example` for few fields, and none can know the environment (a real AI application). The contract could require an `example` on every writable field; then the values file keeps only the environment placeholders. | Contract |
| F33 | Wrong, corrected on 2026-09-26 (F50). The 3 "enums with only `*_UNSPECIFIED`" (for example `LogsAnomalyConditionType`) are `*_OR_UNSPECIFIED` enums: their one value is a real choice. With F50 the survey has no issue in AlertDef's schema. | – |
| F34 | Latent generator risk: a required pure `oneOf` (no "no arm") inside an optional object gets a resource-level `ExactlyOneOf`. When the parent object is null, that validator finds no arm and fails. `ai_evaluation` is not affected (its `config` allows no arm). oneOf groups put the validator on each arm instead, which runs only when the parent is set. Pure `oneOf` should do the same; that changes the `ai_evaluation` schema.go. | Tooling |
| F35 | Server timestamps in request bodies: SLO Create, Replace, and ValidateReplace send `createTime` and `updateTime` (`date-time`). They are server-generated, so by the contract they exist only in responses. These are the only `date-time` fields in any request body. | API proto |
| F36 | A `discriminator` names a string field beside a `oneOf` (for example dashboards `SortStrategy.strategyType`: `STRATEGY_TYPE_CATEGORY` or `STRATEGY_TYPE_QUERY_VALUE`), with no mapping. The spec does not say who sets it: must a Terraform user send a value that matches the arm, or does the server derive it? The field is redundant with the arm. The contract should drop it, or mark it server-set. | API proto / Contract |
| F37 | A request body that reuses the resource message (`{connector: {...}}`) cannot show which fields are server-set, so the generator cannot classify fields. This is already linted: `cx-management-apis/scripts/lint_payload_reuse.py` runs in CI (`openapi-lint.yml`) and blocks new cases, with `.payload-reuse-baseline.txt` for existing ones. The generator does not support this shape, on purpose. | Contract (covered) |
| F38 | The id path parameter can have another name than the id field of the resource (`{key_id}` and `id`, 6 writable resources). Contract rule: the same name. | Linter |
| F39 | A response can have fields beside the resource (`{dashboard, createdAt, authorId, isLocked}`, 6 writable resources; also F28). Anti-pattern: metadata and settings belong inside the resource. Contract rule: no fields beside the resource. | Linter |
| F40 | Most responses wrap the resource in one field (`{aiEvaluation: {...}}`, 20 of 28 writable resources); only View and ViewFolder return the resource itself. The wrapper comes from gRPC (a response message per call); REST gains nothing from it. Contract rule: a resource Create, Get, or Update sets `google.api.http` `response_body` to the resource field, so REST returns the resource itself and gRPC keeps its message. The generator supports both forms. | Linter |
| F41 | Create returns only an id in 4 of 28 writable resources (`{dashboardId}`, `{folderId}`, `{id}`, `{keyId, name, value}`). Terraform needs the full resource after Create. Contract rule: Create returns the complete resource, like PATCH (rule 5.1). It extends the linter rule that PATCH returns the same resource as Get. The generator does not call Get after Create, on purpose. | Linter |
| F42 | Later: `ApiKey` returns its secret `value` only in the Create response; Get never shows it. A valid pattern (a secret seen once), but neither the contract nor the generator covers it. Terraform would need a sensitive attribute set only from the Create response. | Contract / Tooling |
| F43 | 23 of 43 Delete operations return an empty response object (for example `DeleteCompanyIpAccessSettingsResponse`). openapi-generator then returns `map[string]interface{}` from `Execute`, not a response type, and the generator stopped on it. The surveys did not show it, because they do not check SDK names. Fixed in E10: an empty response is expected as a map. | Tooling |
| F45 | openapi-generator names an inline request body after its `title` when it has one (103 of 132 inline bodies), not after the operationId (F15). When a component has that name, it adds `1` (`ViewFolder` → `ViewFolder1`). The generator now copies both rules; the SDK check confirms them. | SDK generator |
| F46 | Server fields in `PUT` bodies: when the body is the whole resource (E2M, Slo, CompanyIpAccessSettings), it also has `createTime`, `updateTime`, and other server fields, with nothing that marks them. The generator needs `readOnly: true` on them (D19). Our OpenAPI v3 generator sets `readOnly` only from `openapiv3_field.read_only`; the v2 generator also maps proto `field_behavior = OUTPUT_ONLY`, which no Coralogix proto uses today. | OpenAPI generator, API proto |
| F47 | The handwritten `cxsdk` accessor is not always the tag without " Service": tag `Folders For Views Service` → `ViewsFolders()`. The generator now falls back to the one `ClientSet` method with the client type. | SDK |
| F48 | Every dashboard chart widget (7 of 9 `Widget.Definition` arms) has a list whose items are a pure `oneOf` (the dataprime query `filters`, the logs `aggregation`). The generator supported a `oneOf` inside a list item, but not a list of `oneOf`. Fixed: lists and maps of `oneOf` generate. | Tooling |
| F49 | Generator bug, fixed: a `oneOf` inside a list item or map value got one resource validator with a wildcard (`rows[*].style.bold` conflicts with `rows[*].style.font`). It compared all items together, so row 0 bold and row 1 with a font was rejected. Every `oneOf` below the root now has validators on its arms, relative to its object, like the groups. This also fixes F34. | Tooling |
| F50 | Generator bug, fixed: 35 enum values in the spec end in `_OR_UNSPECIFIED` (for example `ANALYTICS_THRESHOLD_OPERATOR_MORE_THAN_OR_UNSPECIFIED`, `ALERT_DEF_PRIORITY_P5_OR_UNSPECIFIED`). That zero value is also a real choice, but the model dropped every `*_UNSPECIFIED`, so a user could not choose "more than" or P5. The model now drops only a plain `*_UNSPECIFIED`. Risk: without presence, the server may omit the zero value on read (F21). | Tooling, API proto |
| F51 | `date-time` in requests (F35) is needed: every dashboard chart widget has a query time frame with user-written `from` and `to` (`absoluteTimeFrame`). The handwritten resource uses RFC 3339 strings (`time.Parse(time.RFC3339, ...)`). Fixed: an RFC 3339 string in UTC, with a validator for the form that the API returns. | Tooling |
| F52 | Terraform alerts have no `analytics_threshold` or `analytics_immediate` (API types added on 2026-06-15). Plugging in a generated type costs handwritten work beyond the plug-in lines: each alert type sets the common alert properties itself, 11 helper functions list every alert type and fail on others, and a test fixture lists the type names. | Tooling (provider) |
| F53 | Generator bug, fixed: a model struct kept the dots of a component name (`widgets.GaugeModel`), which is not a Go name. Only the fake and `ai_evaluation`, which have no dots, were generated before. Now camelized like the SDK types (`WidgetsGaugeModel`). | Tooling |
| F54 | Handwritten resources map about 330 enum values to their own Terraform names (`"left": TEXT_ALIGNMENT_LEFT`), in maps that a new API value does not reach. 293 follow one rule: the value without its longest shared word prefix and without `_OR_UNSPECIFIED` / `_UNSPECIFIED`, in lower case. 20 differ only in letter case (`Debug`, `PHONE_NUMBER`, `DataMap`), 14 use other words (`euro` for `EUR`, `percent01` for `PERCENT_ZERO_ONE`, `avg` for `AVERAGE`). The rule also finds a third zero-value style: `X_UNSPECIFIED` is a real value when X is a word (`ANNOTATION_ORIENTATION_VERTICAL_UNSPECIFIED` = "vertical"); the model dropped it before. | Tooling (provider) |
| F55 | openapi-generator breaks the words of an enum constant name at a lower-case letter or a digit before an upper-case letter: `E2M_TYPE_LOGS2METRICS` → `E2MTYPE_E2_M_TYPE_LOGS2_METRICS`. The generator now copies the rule; the SDK check found it. | SDK generator |
| F56 | The type mode writes enum attributes with the API values (`"ALERTS"`); handwritten resources use their own names (`"alerts"`), in 309 of the 18 resources' enum attributes. Fixed for existing resources by the enum overrides (`terraformNames`, per-value names, a name for the zero value). | Tooling (provider) |
| F57 | A handwritten flatten also changes values: GlobalRouter reads a missing `name`, `description`, rule `name` and `condition` as `""`, a missing `disabled` as `false`, a missing `rules` as `[]`, and an empty `fallback`, `fallback_targets`, or `routing_labels` as null (the API returns empty lists). A schema compare cannot see this; the equivalence test (old and new flatten of the same API object) did. Fixed by the overrides `missingAsZero` and `emptyAsNull`. Every switch needs such a test. | Tooling (provider) |
| F58 | Server fields in the GlobalRouter types (`createTime`, `updateTime`, `RoutingTarget.id`) have no `readOnly` (F46), so the type mode would make them optional and send them. Fixed by the override `readOnly` (Computed only, not sent, no validators). | OpenAPI generator, API proto |
| F59 | The generated attribute descriptions come from the spec, not from the handwritten schema, so the provider docs change on a switch. Not a user break; the compare tool does not check descriptions. | Tooling (provider) |
| F60 | `Connector.resolvedConnectorConfig` (spec `readOnly`) is the full effective config, so it would also hold the values that the write-only attributes keep out of state. Adding it to Terraform would leak them; it stays skipped. A server field is not always safe to show. | API proto / Tooling |
| F61 | A resource with Terraform-only parts (Connector: write-only secrets beside the API fields, with their own validator and read and write logic) can switch partly: the resource model embeds the generated model (plugin framework embedded structs), and the handwritten attribute stays beside it. The checks run on the combined schema and code. | Tooling (provider) |
| F62 | The spec loses proto oneof names. Handwritten resources put the arms of a oneof in an object named after it (SLO `sli { request_based_metric_sli \| window_based_metric_sli \| apm_sli }`, `window { slo_time_frame }`), and a one-arm oneof (`window`) is not even a `oneOf` in the spec. The wrapper override restores them. If the spec carried the oneof name (for example `x-oneof: sli` on each arm), the generator could make the wrappers itself. | OpenAPI generator |
| F63 | The SLO Create and Replace body is `Slo1`, an inline copy of `Slo`. The handwritten resource copies `Slo` into it field by field (`extractSLOV2Payload`), and its own comment warns that a new field is dropped until someone adds it there. The type mode cannot target an inline body, so this copy stays handwritten, with the drift risk. A request body that reuses the component (or a named component, F15) would remove it. | API proto / OpenAPI generator |
| F64 | Some handwritten rules are not in the API and stay handwritten after a switch: SLO turns an old singular APM filter `value` into `values`, and leaves out ownership dimensions with no values. They run on the SDK value before the generated flatten and after the generated expand (`integration/slo/add/`). The provider's unit tests keep them: the switch rewrites the tests against the generated model with the same assertions. | Tooling (provider) |
| F65 | Generator bug, fixed: a computed nested object had a struct-pointer model field. Terraform plans a computed attribute with no configuration value as unknown on an update, and a pointer cannot hold an unknown value, so the update failed ("Received unknown value ... *slotypes.ApmSliModel"). The SLO live check found it (`apm_sli_metadata`, `grouping`); the resource mode had the same latent bug (a computed object, the fake `routing`). A computed single nested attribute now has a `types.Object` model field (`objectAs` / `objectValue`). The offline checks missed it: states read from the API never hold unknown values. New check: `schemadump.PlanWithUnknowns` makes every computed attribute unknown, and each switch's plan test reads that into the model and expands it (both steps). | Tooling |
| F66 | The API wraps many values in an object with one field, `value` (21 components; 50 of their 58 uses are in dashboards: `LuceneQuery`, `PromQlQuery`, `UUID`). Handwritten resources show the value itself. The `unwrap` override does the same: a null value sends no object, and a missing object and an object without a value both read as null. Alerts do not use them. | API proto / Tooling (provider) |
| F67 | Handwritten resources often show the fields of a nested API object in the parent: ApiKey `permissions`, `presets` (API `keyPermissions.{permissions, presets}`); TCO `severities` (`logRules.severities`); alerts `percentage_of_deviation` (`anomalyAlertSettings.percentageOfDeviation`), `tracing_filter.latency_threshold_ms` (`tracingFilter.simpleFilter.latencyThresholdMs`), and the routing overrides (`configOverrides.{...}`). It is the opposite of `wrap`. All three measured resources need it. Correction: `tracing_filter` needs no `inline`: `TracingFilter` has only `simpleFilter`, so `unwrap: [TracingFilter]` covers it. Fixed by the `inline` override. | Tooling (provider) |
| F68 | A signed 64-bit integer that JSON sends as a string (alerts `maxUniqueCount`, `timeframeMs`) has no `format: int64` in the spec, only `type: string` and `pattern: ^-?[0-9]+$`. So the type mode writes a String attribute; the handwritten resource has Int64, as D7 does for uint64. The OpenAPI generator should write the format; until then, an override. The reverse also occurs: ApiKey `owner.team_id` is a String in Terraform and an int64 JSON number in the API (corrected: not a uint64). Unsigned numbers have the same gap: alerts `duration` and `latencyThresholdMs` have only `pattern: ^[0-9]+$`, as the AI evaluation fields had before the overlay (D7). Fixed by the `int64` and `string` overrides; the generator now also reads `format: int64` on a string, so a fixed spec needs no override. | Tooling |
| F69 | ApiKey `presets` has another shape in Get than in Create: Get sends `PresetInfo` objects (`{name, permissions}`), Create takes names. The handwritten resource shows the names as a set of strings (the resource-shape survey case "a field with another type in Create than in Get"). The type mode reads the Get type, so no override covers it: it stays handwritten, or the API sends names in both. | API proto |
| F70 | The handwritten alert flatten panics on some API values: a flow stage without `timeframeMs`, and a tracing filter without `latencyThresholdMs` (nil dereferences). Its expand writes a latency above 10 digits as `1.23456789e+10` (`big.Float.String`); the custom converter keeps that. The generated code reads missing values as null. | Tooling (provider) |
| F71 | A float that Terraform shows as Float64: the handwritten alert code reads `percentage_of_deviation` 0.1 as 0.1000000015 (float32 to float64); the generated code reads 0.1, the shortest decimal (as E15). So a state with a non-exact value changes on the first read after the switch, to the value of the configuration. | Tooling (provider) |
| F72 | The handwritten alert code reads many values in a way the spec does not describe: 21 missing enums as their zero value, 21 empty lists and maps as null, 8 missing objects as empty ones or as defaults. The equivalence test found each one; the read overrides reproduce them. An empty-as-null value that a configuration sets fails the apply ("inconsistent result"), so the handwritten code rejects an empty value on 4 of those fields; the switch keeps these 4 checks. The generator could add the check to every `emptyAsNull` collection. | Tooling (provider) |
| F73 | Requests: the handwritten alert code sends a null list as `[]`, and an empty routing override `{}` for none. For protobuf repeated fields `[]` and a missing field are the same, so the equivalence test ignores it; `{}` and a missing message differ in presence, so the switch keeps sending `{}` (a small handwritten rule). | API proto |
| F74 | Validators the spec lacks, which the switch drops (listed for review, as for SLO): a minimum length of 1 (`name`, `data_set`, `data_space`), "exactly one" arm (`type_definition`, SLO `error_budget`/`burn_rate` and `dual`/`single`, `missing_values`, the webhook integration: the spec allows no arm), and `custom_evaluation_delay` at least 0. The API rejects such values instead. The proto should have `min_length`, required oneofs, and ranges. | API proto |
| F75 | Alert rules that only the server checks (the first live run found them; the spec and Terraform accept both): `undetected_values_management` needs at least one rule with `LESS_THAN` ("Cannot set undetected values management for alert that doesnt have at least one rule with less than as a condition"), and the rules of a metric threshold cannot have different condition types (`MORE_THAN` with `MORE_THAN_OR_EQUALS` is also rejected). The contract has no place for rules across fields; a probe could list them. | API proto / Contract |
| F44 | `PolicySettings` (a singleton): Get is on `/dataplans/policy-settings/v1`, but Replace is on `/dataplans/policiy-settings/v1` (a typo). A singleton linter rule, all operations on one path, would catch it. | API proto |
