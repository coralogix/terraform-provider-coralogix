# Coralogix IaC code generator

This beta generates new Terraform Plugin Framework resources from eligible OpenAPI contracts. Unsupported or ambiguous contracts stop with an error. Existing-resource migration is not part of this beta.

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

The beta supports complete new resources with one Create, Get, Update or Replace, and Delete operation. Create, Get, and Update must return the resource directly. A non-singleton Update or Replace operation must put the resource ID in its path. The ID can be a string, `int32`, or `int64`. Its type must match in every path and in the resource response. Body-only Update IDs are reserved for later existing-resource migration support. The beta supports managed, immutable, and computed fields. A computed nested object or object collection has computed-only descendants. It supports scalar values, typed maps, ordered lists, and explicitly unordered sets. It supports the documented OpenAPI one-of form. It supports PATCH with a required `updateMask` query parameter that has a documented field-mask pattern. It also supports full-replace PUT.

Optional request fields, including fields inside request objects, must use `x-coralogix-presence: true`. This annotation states that omission differs from an explicit zero or empty value. For a client-owned field that is also optional in Get, it also requires round-trip presence: omission reads back as absent, an explicit zero reads back as present, and clearing reads back as absent. Nested `oneOf` arms use union presence and do not need the annotation. A top-level scalar server default must be valid for the field. Create and Update must declare the same default. The field must be optional in Create and Update and required in Get. Terraform renders this contract as `Optional + Computed`. It does not insert a static value into the request. When a user removes an override, the plan becomes unknown. Update sends the field in `updateMask` without a body value. The API resets the field, and Get returns the declared default. Unordered arrays must use `x-coralogix-collection: set`. `uniqueItems` alone does not make an array unordered.

The generator rejects incomplete lifecycles, ambiguous operations, wrapped resource responses, body-only Update IDs, an optional Get response ID, unsupported or inconsistent ID types, unsupported required query, header, or cookie parameters, incompatible request and response types, invalid, undeclared, or inconsistent server defaults, lifecycle-specific root `oneOf` groups, `writeOnly` fields, string patterns, exclusive numeric bounds, object property-count constraints, unsupported unions, recursive schemas, free-form maps, Terraform name collisions, and missing SDK symbols. Nested and non-scalar server defaults and generic write-only state handling are outside this beta. It reports all detected issues in stable order. It writes no output when any issue exists. Generated Create and Update code also stops before the API call if an unexpected unknown value remains. Generated Read warns before removing a resource that the API no longer finds. Generated response conversion maps a scalar protobuf `*_UNSPECIFIED` enum zero value to Terraform null. It returns diagnostics if an enum list, set, or map contains such a sentinel because removing a collection element would change its meaning.

## Versioned output

Every generated Go file starts with this header:

```go
// Code generated by coralogix-iac-codegen v0.1.0-beta.13. DO NOT EDIT.
```

Find files affected by this generator version with:

```sh
rg 'Code generated by coralogix-iac-codegen v0\.1\.0-beta\.13' .
```

During beta, increment the beta identifier for each generator behavior change. After beta, use a patch version for fixes, a minor version for new supported behavior, and a major version for incompatible output changes.

## Beta boundary

This beta has no type mode. `generate` has no OpenAPI input flag. The module has no live OpenAPI fetch, overlay, compatibility override, custom field hook, force flag, or allow-unsupported flag. Existing-resource migration, data sources, provider registration, and automated delivery are separate work.

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
