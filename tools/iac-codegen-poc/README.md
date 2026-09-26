# IaC code generation POC

A proof of concept: generate a complete Terraform resource from OpenAPI, with no handwritten resource code.
It generates one resource, `ai_evaluation`, from the AI Evaluations API.

This folder is a separate Go module. It does not change the provider build. The generated resource is
not registered in the provider. The handwritten `coralogix_ai_evaluation` resource stays as it is.

**The generated files:** [`generated/aievaluation/`](generated/aievaluation/) (the real API),
[`generated/fakeboard/`](generated/fakeboard/) (a fake API, see "Fake resource"),
[`generated/fakesettings/`](generated/fakesettings/) (a fake singleton, see "Singletons"), and
[`generated/fakerule/`](generated/fakerule/) and [`generated/fakeview/`](generated/fakeview/) (fake full-replace
updates, see "Full replace").

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
fakesdk/generate.sh                              # regenerate the fake SDK (needs Docker)
go run ./cmd/tfgen --spec spec/openapi.patched.yaml --survey   # shapes that cannot be generated, for all Get resources
go run ./cmd/tfgen --spec spec/openapi.patched.yaml --survey-resources   # operations, ids, bodies, responses of writable resources
go run ./cmd/tfgen --spec spec/fake/settings.yaml --resource FakeSettings \
  --sdk-module github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen-poc/fakesdk --out generated/fakesettings
go run ./cmd/tfgen --spec spec/fake/openapi.yaml --resource FakeBoard \
  --sdk-module github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen-poc/fakesdk --out generated/fakeboard
go run ./cmd/tfgen --spec spec/fake/rules.yaml --resource FakeRule \
  --sdk-module github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen-poc/fakesdk --out generated/fakerule
go run ./cmd/tfgen --spec spec/fake/views.yaml --resource FakeView \
  --sdk-module github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen-poc/fakesdk --out generated/fakeview
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

- **Fake SDK:** [`fakesdk/generate.sh`](fakesdk/generate.sh) runs the same steps as the pinned real SDK: its sanitizer,
  `openapi-generator` 7.17.0 in Docker with the SDK templates and options, and its regex fix. Only
  [`fakesdk/go/openapi/cxsdk`](fakesdk/go/openapi/cxsdk/cxsdk.go) is handwritten. `--sdk-module` selects it.
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
[`cmd/tfgen/testdata/survey.txt`](cmd/tfgen/testdata/survey.txt): 42 of 43 resources have no issue. Left: AlertDef, which
has an enum with only the `*_UNSPECIFIED` value (F33, an API gap).

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
| F33 | Survey: 3 enums have only the `*_UNSPECIFIED` value (for example `LogsAnomalyConditionType`). No valid value can be sent. | API proto |
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
| F44 | `PolicySettings` (a singleton): Get is on `/dataplans/policy-settings/v1`, but Replace is on `/dataplans/policiy-settings/v1` (a typo). A singleton linter rule, all operations on one path, would catch it. | API proto |
