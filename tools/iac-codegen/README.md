# Coralogix IaC code generator

This beta generates new Terraform Plugin Framework resources from eligible OpenAPI contracts. Unsupported or ambiguous contracts stop with an error. A resource that users already have uses existing-resource mode (see below).

## Run the generator

Run the command from this module:

```sh
cd tools/iac-codegen

go run ./cmd/tfgen generate \
  --resource resource_name \
  --out ../../internal/provider/generated/resource_name
```

The command walks to the provider root. It reads the Management SDK version from the provider `go.mod`. It then reads `openapi.yaml` from that exact module version in the local Go module cache. SDK inspection runs with `GOPROXY=off` and `GOSUMDB=off`. The command stops if required local module data is missing. It does not fetch OpenAPI from a live service.

Check a candidate OpenAPI document before its SDK is released:

```sh
go run ./cmd/tfgen check \
  --resource resource_name \
  --openapi /path/to/candidate/openapi.yaml
```

`check` uses the same OpenAPI and renderer capability checks as `generate`. It reports all detected issues in stable order and returns a non-zero status when the resource is not eligible. It does not render or publish files. SDK symbol checks run only during `generate`, after an SDK exists. The `generate` command does not accept `--openapi`.

Use an exact operation ID only when normal discovery is ambiguous:

```sh
go run ./cmd/tfgen generate \
  --resource resource_name \
  --create-operation Service_CreateResource \
  --out ../../internal/provider/generated/resource_name
```

The equivalent flags exist for Get, Update or Replace, and Delete. The generator validates the selected method and types.

## Architecture

The command uses this data flow:

```text
generate: provider go.mod -> pinned SDK openapi.yaml -> shared validation
  -> SDK symbol checks -> Terraform renderer -> staged output publication

check: local candidate openapi.yaml -> shared validation -> eligibility report
```

The OpenAPI reader has no Terraform or SDK types. The generator completes every check and formats every file before it changes the output directory. An existing output directory must contain only Go files with the exact generator header. This rule prevents the generator from replacing a user-owned directory.

## Supported contracts

The beta supports complete new resources with one Create, Get, Update or Replace, and Delete operation. Create, Get, and Update must return the resource directly. A non-singleton Update or Replace operation must put the resource ID in its path. The ID can be a string, `int32`, or `int64`. Its type must match in every path and in the resource response. A body-only Update ID is supported only in existing-resource mode. The beta supports managed, immutable, and computed fields. A computed nested object or object collection has computed-only descendants. It supports scalar values, typed maps, ordered lists, and explicitly unordered sets. It supports the documented OpenAPI one-of form. It supports PATCH with a required `updateMask` query parameter that has a documented field-mask pattern. The pattern describes the whole comma-separated value, so it must accept a list such as `a,b`. It also supports full-replace PUT. It supports a singleton, whose Get has no path parameter, when Create, Get, Update, and Delete use one path. Its `id` attribute is computed-only, with the resource type name as a static default. A singleton that always exists, with no Create or Delete or with a Get that never returns 404, is not supported.

Create, Update, and the resource response must use separate schemas. Inline Create and Update schemas are supported when each has its own identity. Every fixed object must declare `required`. Use `required: []` when every field is optional. Required means not empty. A required string must declare `minLength: 1`, a required list `minItems: 1`, and a required map `minProperties: 1`. Optional scalar request fields must use `x-coralogix-presence: true`. A required bool or number must also use this annotation when its zero value is valid. This annotation states that omission differs from an explicit zero or empty value. For a client-owned scalar that is also optional in Get, it also requires round-trip presence: omission reads back as absent, an explicit zero reads back as present, and clearing reads back as absent. Lists, maps, and objects do not need the annotation. An empty list or map is the same as a missing one, and an empty object outside a `oneOf` arm is the same as a missing object. Generated state keeps the form that the configuration or the prior state has when both the response and that prior value are empty, so `[]`, `{}`, and a missing value do not cause a difference. A computed attribute with no prior value, for example after an import, keeps the value of the API. An empty `oneOf` arm selects that arm, so Get must return it. A top-level scalar server default must be valid for the field. Create and Update must declare the same default. The field must be optional in Create and Update and required in Get. Terraform renders this contract as `Optional + Computed`. It does not insert a static value into the request. When a user removes an override, the plan becomes unknown. Update sends the field in `updateMask` without a body value. The API resets the field, and Get returns the declared default. Unordered arrays must use `x-coralogix-collection: set`. `uniqueItems` alone does not make an array unordered.

The generator rejects incomplete lifecycles, ambiguous operations, reused request schemas, missing required declarations, missing scalar presence, required strings, lists, or maps that accept an empty value, wrapped resource responses, body-only Update IDs, an optional Get response ID, unsupported or inconsistent ID types, fields that are optional in Create but required in Update, unsupported required query, header, or cookie parameters, incompatible request and response types, invalid, undeclared, or inconsistent server defaults, lifecycle-specific root `oneOf` groups, nested `oneOf` updates without dotted mask paths, `writeOnly` fields, string patterns, exclusive numeric bounds, `uint64` fields without a maximum string length of 18 or less, property-count constraints on objects that are not maps, unsupported unions, recursive schemas, free-form maps, Terraform field-name collisions, Go field-name collisions, Go component-name collisions, and missing SDK symbols. A field can be required in Create and optional in Update because Terraform still requires its configured value and can send it on every update. Nested and non-scalar server defaults and generic write-only state handling are outside this beta. It reports all detected issues in stable order. It writes no output when any issue exists. Generated Create and Update code also stops before the API call if an unexpected unknown value remains. Generated Read warns before removing a resource that the API no longer finds. Generated response conversion maps only the enum's exact `<PREFIX>_UNSPECIFIED` zero value to Terraform null. A business value that also ends in `_UNSPECIFIED` remains a value. The generator returns diagnostics if an enum list, set, or map contains the exact zero sentinel because removing a collection element would change its meaning.

The small golden contract uses one synthetic resource. It covers a clearable optional `description`, an `enabled` server default of `true`, an immutable required `kind` enum, a mutable required `config` one-of, a computed nested `status` object, computed `create_time` and `update_time` values, an ordered `destinations` list, and an unordered `tags` set.

## Existing resources

A resource that users already have must not change when its handwritten code becomes generated code. Existing-resource mode keeps its released behavior. A file named `behavior-overrides.yaml` sits in the output directory. `generate` reads it without a flag. `check` takes `--overrides <file>`.

```yaml
resource: GlobalRouter        # the OpenAPI component; the file must match --resource
mode: existing
validators:
  inferred: false             # required: no limit of the contract becomes a validator
api:
  requestWrapper: router      # Create and Update send {"router": {...}}
  updateIDInBody: true        # Update has no id in its path
  clientSetID: true           # the user can set the id
types:
  RoutingRule:
    fields:
      targets:
        computed: true
        keepPriorOrder: true  # the API does not keep the order of the items
```

The rule is: **a field with a line keeps the released behavior that the line states. A field without a line follows the API contract**, and the contract must say how (for example with `x-coralogix-presence`). A new API field never needs a line.

The file is strict:

- An unknown key is an error.
- A line that matches nothing in the contract is an error (`OVERRIDE_UNUSED`). The contract may begin to state the same fact. The generator then asks you to delete the line.
- Every key of the file changes the generated code, or it is not a key. When a key is added to the reader, the renderer must support it in the same change. The generator never writes code that ignores a line.

Keys of a field line: `skip`, `readOnly`, `required`, `description`, `markdownDescription`, `deprecation`, `computed`, `useStateForUnknown`, `default` (a string or a bool), `readEmptyAs: "null"`, `keepPriorOrder`, and `validators` (`oneOf`, `sizeAtLeast`). Other keys: `markdownDescription` of the resource, `schema.version` and `schema.upgrade.<n>` (the frozen prior schema as `<import path>.<Func>`, upgraded by reading the resource), `types.<Type>.required: []`, and `enums.<Enum>` with `zero` and `values`.

Existing mode relaxes these rules of the contract: a response that wraps the resource, a request that wraps the resource and shares its schema, the id in the Update body, a client-set id, enum values without the zero prefix, a missing `required` list on a named object, and nested server defaults. It also turns off the presence and default checks for a field with a line. It does not generate regular-expression validators. It ignores a string pattern.

A resource in this mode exports `Flatten` for a handwritten data source. A frozen prior schema stays in its own package.

## Versioned output

Every generated Go file starts with this header:

```go
// Code generated by coralogix-iac-codegen v0.1.0-beta.2. DO NOT EDIT.
```

Find files affected by this generator version with:

```sh
rg 'Code generated by coralogix-iac-codegen v0\.1\.0-beta\.2' .
```

During beta, increment the beta identifier for each generator behavior change. After beta, use a patch version for fixes, a minor version for new supported behavior, and a major version for incompatible output changes.

## Beta boundary

This beta has no type mode. `generate` has no OpenAPI input flag. The module has no live OpenAPI fetch, custom field hook, force flag, or allow-unsupported flag. The only override is the `behavior-overrides.yaml` file of an existing resource. Data sources, provider registration, and automated delivery are separate work.

## Verify changes

```sh
go fmt ./...
go test ./...
go vet ./...
go run ./cmd/tfgen --version
git diff --exit-code -- internal/generator/testdata/golden
```

Regenerate the golden output after an intended output change:

```sh
go test ./internal/generator -run TestGoldenOutput -update
```
