# IaC code generation POC

A proof of concept: generate a complete Terraform resource from OpenAPI, with no handwritten resource code.
It generates one resource, `ai_evaluation`, from the AI Evaluations API.

This folder is a separate Go module. It does not change the provider build. The generated resource is
not registered in the provider. The handwritten `coralogix_ai_evaluation` resource stays as it is.

**The generated files:** [`generated/aievaluation/`](generated/aievaluation/)

| File | Content |
|---|---|
| [`schema.go`](generated/aievaluation/schema.go) | Schema: attributes, types, validators, defaults, plan modifiers |
| [`model.go`](generated/aievaluation/model.go) | Model: Go structs for the plan and the state |
| [`convert.go`](generated/aievaluation/convert.go) | Expand (plan → SDK request) and flatten (SDK response → state) |
| [`mask.go`](generated/aievaluation/mask.go) | Update mask: the top-level fields that changed between the plan and the state |
| [`resource.go`](generated/aievaluation/resource.go) | Create, Read, Update, Delete, Import |

The `*_test.go` files in that folder are handwritten. They test the generated code.

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
go run ./cmd/tfgen --spec spec/openapi.patched.yaml --resource AiEvaluation --out generated/aievaluation
go run ./cmd/tfgen --spec spec/openapi.patched.yaml --resource AiEvaluation --sdk-names   # print the SDK names
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

[`provider/acc_test.go`](provider/acc_test.go) uses a small test provider (`provider/provider.go`) that
registers only the generated resource.

- Steps: Create (PII) → Import (verify) → Update (`config`, `threshold`) → change the `config` arm
  (PII → allowed topics) → clear `threshold` → Delete. Each update is in place and keeps the id.
  After each step, Terraform checks that a new plan is empty.
- `is_enabled` stays `true`, because the server rejects `isEnabled` in the update mask (F25).
- The test uses the first AI application with a name and a subsystem, and a target that is not in use.
  It deletes what it creates, also on failure.
- Result on EU2: all steps pass.

```zsh
printf 'Coralogix API key: '; read -rs CORALOGIX_API_KEY; echo; export CORALOGIX_API_KEY
CORALOGIX_ENV=EU2 TF_ACC=1 go test ./provider -run TestAccAiEvaluation -v -count=1 -timeout 30m
```

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
| F23 | The model does not keep the `updateMask` pattern. The generator checks each mask entry with its own copy of the contract rule (top-level name, no `*`). If the contract later allows nested paths, the generator must read the pattern from the spec. | Tooling |
| F24 | "Not found" has no machine-readable form. The SDK's `cxsdk.IsNotFound` parses the message text (`Not Found: <msg>`) to tell a missing resource from a missing route. The generated Read and Delete use only the status 404, like the handwritten resource. So a wrong route would remove the resource from the state. The contract should define a not-found reason. | Contract / Backend |
| F25 | The Update mask rejects `isEnabled`. The generator sends the API (JSON) name `isEnabled`. The gateway turns it into the proto path `is_enabled` (proto field `is_enabled = 6` in `UpdateAiEvaluationRequest`). The server answers 400 `Unknown field path "is_enabled" in update_mask`. So a documented Update field cannot be masked. No other client masks it: the SDK example masks only `config`, and the handwritten resource sends no mask. The contract must say: every field in the Update body is a valid mask path. A probe must check each one. | Backend / Contract |
