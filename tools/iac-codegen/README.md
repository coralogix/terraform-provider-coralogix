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

`check` uses the same OpenAPI and renderer capability checks as `generate`. It reports all detected issues in stable order and returns a non-zero status when the resource is not eligible. It does not render or publish files. SDK symbol checks run only during `generate`, after an SDK exists.

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

The beta supports complete new resources with one Create, Get, Update or Replace, and Delete operation. Create, Get, and Update must return the resource directly. A non-singleton Update or Replace operation must put the resource ID in its path. The ID can be a string, `int32`, or `int64`. Its type must match in every path and in the resource response. A body-only Update ID is supported only in existing-resource mode. The beta supports managed, immutable, and computed fields. A computed nested object or object collection has computed-only descendants. It supports scalar values, typed maps, ordered lists, and explicitly unordered sets, including sets of objects. It supports the documented OpenAPI one-of form. It supports PATCH with a required `updateMask` query parameter that has a documented field-mask pattern. The parameter can also have its proto name, `update_mask`, as the OpenAPI fork writes it. A 200 response can also be `allOf` with one `$ref`, as the fork writes a `response_body` field that has a description. The pattern describes the whole comma-separated value, so it must accept a list such as `a,b`. It also supports full-replace PUT. In existing-resource mode, Delete can be a POST that the behavior-overrides file names, for an API that removes the resource with a call such as `/{id}/archive` instead of a DELETE (see below). In existing-resource mode, a string field can hold a YAML or JSON document that the API returns in another format; the behavior-overrides file says so (see below). It supports a singleton, whose Get has no path parameter, when Create, Get, Update, and Delete use one path. Its `id` attribute is computed-only, with the resource type name as a static default. A singleton that always exists, with no Create or Delete or with a Get that never returns 404, is not supported.

Create, Update, and the resource response must use separate schemas. Inline Create and Update schemas are supported when each has its own identity. A fixed object without a `required` list has no required field. The OpenAPI fork cannot write an empty list, so a message whose fields are all optional, such as a PATCH body, has none. Required means not empty. A required string must declare `minLength: 1`, a required list `minItems: 1`, and a required map `minProperties: 1`. Optional scalar request fields must use `x-coralogix-presence: true`. A required bool or number must also use this annotation when its zero value is valid. This annotation states that omission differs from an explicit zero or empty value. For a client-owned scalar that is also optional in Get, it also requires round-trip presence: omission reads back as absent, an explicit zero reads back as present, and clearing reads back as absent. Lists, maps, and objects do not need the annotation. An empty list or map is the same as a missing one, and an empty object outside a `oneOf` arm is the same as a missing object. Generated state keeps the form that the configuration or the prior state has when both the response and that prior value are empty, so `[]`, `{}`, and a missing value do not cause a difference. A computed attribute with no prior value, for example after an import, keeps the value of the API. An empty `oneOf` arm selects that arm, so Get must return it. A top-level scalar server default must be valid for the field. Update can omit the default, because the generated resource reads only the Create default. If Update declares one, it must equal the Create default. The field must be optional in Create and Update and required in Get. Terraform renders this contract as `Optional + Computed`. It does not insert a static value into the request. When a user removes an override, the plan becomes unknown. Update sends the field in `updateMask` without a body value. The API resets the field, and Get returns the declared default. Unordered arrays must use `x-coralogix-collection: set`. `uniqueItems` alone does not make an array unordered.

A nested object can use other components in Create, Update, and the resource response, for example `ThingSpecCreate`, `ThingSpecUpdate`, and `ThingSpec`. The generator matches the nested fields by name, in objects, list items, map values, and one-of arms. Each nested field gets the lifecycle of a top-level field:

| Create | Update | Response | Nested field |
|---|---|---|---|
| yes | yes | yes | managed |
| yes | no | yes | immutable: a change replaces the resource |
| no | no | yes | computed |

Any other combination is an error. The kind, format, limits, enum values, defaults, and one-of arms of a field must be the same in every schema. Component names, descriptions, `required`, and presence can differ. An enum must use the same component in every schema. Create decides `required`. A one-of can require an arm in Create and allow no arm in Update and the response: a PATCH that changes only another field sends no arm. Terraform then requires an arm, as Create does. The opposite is an error. A computed nested field has no plan modifier, so it shows `(known after apply)` when the resource changes. Update leaves it out of the change check and the update mask. An immutable value that holds computed fields, for example a list of items with a server-set `id`, is compared without them, so only a change that Create sends replaces the resource. A behavior-overrides line cannot make a field inside an immutable value optional and computed: an omitted one would be unknown after any change and replace the resource. An immutable field in a list item also replaces the resource when a new item sets it, because Update cannot send it. Removing an object that holds an immutable value replaces the resource too. A `readOnly` property inside a component that the request shares with the response stays unsupported. The request must use its own component.

The generator rejects incomplete lifecycles, ambiguous operations, reused request schemas, missing scalar presence, required strings, lists, or maps that accept an empty value, wrapped resource responses, body-only Update IDs, an optional Get response ID, unsupported or inconsistent ID types, fields that are optional in Create but required in Update, nested fields that only a request has or that only Update sends, unsupported required query, header, or cookie parameters, incompatible request and response types, invalid, undeclared, or inconsistent server defaults, lifecycle-specific root `oneOf` groups, nested `oneOf` updates without dotted mask paths, `writeOnly` fields, string patterns in Create or Update (a pattern that only the response has is ignored, because the configuration never sets that value), exclusive numeric bounds, `uint64` fields without a maximum string length of 18 or less, property-count constraints on objects that are not maps, unsupported unions, recursive schemas, free-form maps, Terraform field-name collisions, Go field-name collisions, Go component-name collisions, and missing SDK symbols. A field can be required in Create and optional in Update because Terraform still requires its configured value and can send it on every update. Nested and non-scalar server defaults and generic write-only state handling are outside this beta. It reports all detected issues in stable order. It writes no output when any issue exists. Generated Create and Update code also stops before the API call if an unexpected unknown value remains. Generated Read warns before removing a resource that the API no longer finds. Generated response conversion maps only the enum's exact `<PREFIX>_UNSPECIFIED` zero value to Terraform null. A business value that also ends in `_UNSPECIFIED` remains a value. The generator returns diagnostics if an enum list, set, or map contains the exact zero sentinel because removing a collection element would change its meaning.

The small golden contract uses one synthetic resource. It covers a clearable optional `description`, an `enabled` server default of `true`, an immutable required `kind` enum, a mutable required `config` one-of, a computed nested `status` object, a `spec` object with other request components and managed, immutable, and computed nested fields, also in list items, computed `create_time` and `update_time` values, an ordered `destinations` list, and an unordered `tags` set.

A second small golden contract, `archivedthing`, is an existing resource whose API has no DELETE. Its behavior-overrides file names the archive call as the Delete.

A third small golden contract, `configthing`, is an existing resource with JSON fields, one of them immutable, and a YAML field in a list of objects that keeps its prior order. Its behavior-overrides file sets `equality` on both.

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
  delete:
    operation: Service_ArchiveRouter # the API has no DELETE; this POST removes the resource
types:
  RoutingRule:
    fields:
      targets:
        computed: true
        keepPriorOrder: true  # the API does not keep the order of the items
  ConnectorConfig:
    extraAttributes:          # Terraform-only maps; expand does not send them
      field_values_wo:
        elementType: string
        writeOnly: true
        markdownDescription: Secret values keyed by field name.
```

The rule is: **a field with a line keeps the released behavior that the line states. A field without a line follows the API contract**, and the contract must say how (for example with `x-coralogix-presence`). A new optional scalar API field needs no line only when the contract states its presence. Without that annotation the generator stops with `FIELD_PRESENCE_UNKNOWN`. A list, map, or object needs no annotation. Then ask for the annotation in the API contract (a proto3 `optional` field), or probe what a read returns for an omitted value and write a line. A new value of an enum also stops the generator (`ENUM_VALUE_UNDECIDED`) until the value is in `values` or in `rejected`.

The file is strict:

- An unknown key is an error.
- A line that matches nothing in the contract is an error (`OVERRIDE_UNUSED`). The contract may begin to state the same fact (`readOnly` or `required`). The generator then names the stale key. If the line sets other keys, keep them: the contract does not state them.
- Every key of the file changes the generated code, or it is not a key. When a key is added to the reader, the renderer must support it in the same change. The generator never writes code that ignores a line.

Keys of a field line: `skip`, `readOnly`, `required`, `description`, `markdownDescription`, `deprecation`, `computed`, `useStateForUnknown`, `default` (a string, a bool, or a number), `readEmptyAs: "null"`, `keepPriorOrder`, `equality` (`yaml` or `json`), and `validators` (`oneOf`, `sizeAtLeast`, `lengthAtLeast`, `enum: true`). `enum: true` accepts the values of the field's enum as the `enums` line states them, so the validator and the conversion maps cannot disagree. Other keys: `markdownDescription` of the resource, `schema.version` and `schema.upgrade.<n>` (the frozen prior schema as `<import path>.<Func>`, upgraded by reading the resource), `types.<Type>.required: []`, `types.<Type>.extraAttributes` (Terraform-only map attributes that the contract does not have; `elementType` is `string` or `int64`, `writeOnly: true` emits `WriteOnly: true`, flatten writes a typed null, expand does not send them), and `enums.<Enum>` with `zero`, `values` (accepted), and `rejected` (in the contract, not accepted). Every value of the contract must be in one of the two lists. An `enums` line applies to a single enum field. The generator reports a list, set, or map of the enum. When `clientSetID` is on, flatten returns an error for a response without an id, because a proto3 optional id cannot be required in the contract.

`keepPriorOrder` on a list of objects keeps the order of the prior list, the plan after Create or Update or the state before Read, when the API returns the items in another order. Each API item pairs with the first prior item that it equals, compared on the fields that the request sends, with equality lines applied. When every item pairs, the state keeps the prior order. When an item changed, or the API added or dropped one, the API order stays, so the plan shows the change. When the request sends the items with their own component, for example a Create item without the id and the hash that the response adds, the generated code converts each API item to the model and both it and each prior item to that request component, so a field that only the server sets never decides the pairing. Create and Update must then send the same fields of the item. The generator rejects the key on a list that no request sends.

`equality: yaml` or `equality: json` compares a string field as a YAML or JSON document, for a field that the API returns normalized: in another format, key order, or whitespace. Two values are equal when they parse to the same value. Numbers compare by their exact value, so two numbers that round to one float64 stay different, and a YAML integer never equals a YAML float. Merge keys (`<<`) expand as `yaml.Unmarshal` expands them, because an API that normalizes YAML returns the merged map without its anchor: a key of the map itself wins, and an earlier map of a merge list wins over a later one. A value that does not parse equals only the same text, so it never panics and never hides a change, and neither does a YAML document whose aliases hold the document itself or expand to more than about a million values. A plan modifier keeps the state when the configured document equals it. It runs before the other plan modifiers of the field, so an equal document in an immutable field does not replace the resource, and flatten keeps the text of the plan or the state when the API returns an equal document. Without a prior value, after an import for example, the state takes the value of the API. A resource with an equality line also gets a `ModifyPlan`: Terraform marks every computed attribute that the configuration does not set unknown when the configuration text changes, before the plan modifiers run, so a change of format alone would still show `(known after apply)`. When every known planned value is the state, and the configuration sets none of the unknown ones, the plan is the state. With `keepPriorOrder` on the list, the items match the prior items with the same document comparison, so a list that the API reorders and reformats keeps its order and text. The key works in nested objects and in lists of objects, but not in the objects of a map or set or in a computed object: their flatten has no prior value, or no order to pair prior items by, so the generator reports `OVERRIDE_PRIOR_CONTAINER` there, for `keepPriorOrder` too. The generator reports it on a field that is not a plain string (`OVERRIDE_EQUALITY_NOT_STRING`: a list, an object, a number, an enum, or a string with a `format`) and on a field that only the server sets (`OVERRIDE_EQUALITY_COMPUTED`). A resource with an equality line cannot have a server default, because that default plans unknown on purpose.

`api.delete.operation` names the operation that removes the resource when the API has no DELETE. It must be a POST whose path is the Get path plus one segment, for example `/v1/things/{id}/archive`, with the id path parameter of Get as its only parameter, no request body, and a 200 response. The generated Delete calls it with the id, ignores the response, and treats a 404 as deleted, as for a DELETE. The generator reports an operation that does not exist (`DELETE_OVERRIDE_OPERATION_NOT_FOUND`), another method (`DELETE_OVERRIDE_METHOD_INCOMPATIBLE`), another path (`DELETE_OVERRIDE_PATH_INCOMPATIBLE`), another or an extra path parameter (`DELETE_OVERRIDE_ID_INCOMPATIBLE`), a required query, header, or cookie parameter (`REQUIRED_PARAMETER_UNSUPPORTED`), and a request body, even an empty one (`DELETE_OVERRIDE_BODY_UNSUPPORTED`): the generated call sends only the id. It also reports the key when the resource passes every check without it, so that a DELETE of the API is the Delete (`DELETE_OVERRIDE_UNNEEDED`). `generate` includes the SDK symbol checks in that decision; `check` has no SDK and assumes, as for every SDK symbol, that the SDK matches the contract. A DELETE that the generator cannot use, or any other issue without the key, leaves the override needed, and when `--delete-operation` also names an operation (`DELETE_OVERRIDE_CONFLICT`). `api.delete` is an object, so that a later key can add a step before the call. No such step exists yet.

`validators.inferred: false` removes the limits of the contract. A `oneOf` group validator states the structure of the request, so it stays. `clientSetID` needs the id property in the Create body, and `updateIDInBody` needs it in the Update body. Otherwise the generator reports an issue.

`skip` cannot name the resource id or an arm of a `oneOf` group. `readEmptyAs: "null"` works for an object and for a list or set of objects. The generator reports any other use, because the key would do nothing.

Existing mode relaxes these rules of the contract: a response that wraps the resource, a request that wraps the resource and shares its schema, the id in the Update body, a client-set id, enum values without the zero prefix, a missing `required` list on a named object, and nested server defaults. It also turns off the presence check for a field with a line. A top-level field with a server default in the contract uses that default, as in a new resource. A `default` line replaces it: the field gets that static default, and the generator does not check the contract default. A `computed: false` line replaces it too, because a server default needs a computed attribute. It does not generate regular-expression validators. It ignores a string pattern.

Existing mode also keeps the prior form of an empty value, as new resources do. The API returns an unset list or map as an empty one, because proto3 has no presence for them, so without it a configuration that omits one fails the apply with `was null, but now` an empty value. `flatten` and the field lines (`readEmptyAs`, `computed`) decide what a read writes before that rule applies. A state upgrade has no prior of the new schema version, so its state takes the value of the API.

A resource in this mode exports `Flatten` for a handwritten data source. A frozen prior schema stays in its own package.

## Acceptance tests

With an `acceptance.yaml` in the output directory, `generate` also writes `acceptance_test.go`: an external test package (`<package>_test`) that runs against a real tenant. It serves the provider through `provider.MuxServer`, the SDKv2 and the framework providers behind one mux as in the binary, so a prerequisite can be any resource of the provider. It needs no helper from the provider tests. Without the file, the generator writes no test. `--acceptance <file>` names another file. `check --acceptance <file>` builds and renders the same test without writing it, so a wrong file fails before the SDK exists.

```yaml
resource: GlobalRouter          # must match --resource
env: [SLACK_INTEGRATION_ID]     # variables the config reads; the test stops if one is missing
prerequisites: |                # HCL that goes before the resource; @{run} and @{env.NAME} work
  resource "coralogix_connector" "http" { id = "@{run}-http" ... }
values:                         # an HCL expression for a field, instead of a made-up value
  rules[].targets[].connector_id: coralogix_connector.http.id
skip: [fallback]                # fields that no config sets (a deprecated field, for example)
minimal: [routing_labels]       # optional fields that the minimal config must set
upgradeMinimal: [description]    # optional fields the released provider needs to create the resource (needs upgradeFrom)
upgradeFrom: "3.19.0"           # a released provider for the upgrade test
```

The test makes its configs from the final schema, after the behavior overrides. Each lifecycle is a subtest on its own resource, so a failure in one does not hide the others:

- `full-lifecycle`: the full config sets every attribute the user can set, and the test checks each value. Then an import step compares the imported state with the state (`ImportStateVerify`). The update config changes the strings, numbers, and bools. An immutable attribute and an enum keep their values, because the valid values of other fields can depend on an enum. Then another import.
- `minimal-lifecycle`: the minimal config sets only the required attributes, and the ones listed in `minimal`. Optional attributes that the server does not fill must leave the state. Then an import, the full config, and the minimal config again.

After every apply, the framework plans again and fails on a non-empty plan. The made-up values are: the first accepted value of an enum (not `unspecified`), `true` and `false`, `1` and `2` (or the start of a range validator and the next number), and `@{run}-<attribute>` for a string (cut or padded with `x` to fit a length validator). One arm of each `oneOf` group is set. A made-up collection has one element, made by the same rules and the validators of the elements. The generator fails on a kind of attribute it cannot make, or on a collection whose size validator rejects one element, and asks for a value in `acceptance.yaml`. A path in `values`, `skip`, or `minimal` that matches no attribute is an error. A nested path in `minimal` (or `upgradeMinimal`) also sets its optional parents, with only their required fields. A required attribute in `skip` is an error too: no config can leave it out. After the minimal config removes an `Optional` and `Computed` attribute, the test checks the value that the schema states: a static or server default, or, with `UseStateForUnknown`, the value of the full config. Without one, the API decides, and only the empty plan and the import check it. A `values` path names the attributes from the resource down, with `[]` for each list or set element: `rules[].targets[].connector_id`.

With `upgradeFrom`, a second test has two subtests. `upgrade-full` creates the full config with that released provider, plans it with this build (no change expected), applies the updated config, and imports. The apply checks the plan action only where it is sure: no change for the same config, and replace when a top-level immutable attribute surely differs. Otherwise the plan must not replace the resource, and may update it or not, because a default or an API value in state can equal what the next config sets; the value checks after the apply catch a missing update. When an immutable `Optional` and `Computed` attribute without a known default is left out of the first config, the test checks no action and keeps the empty-plan check. The released provider rejects an attribute that it does not have, so the generator writes `upgrade-attributes.yaml` next to the test and the acceptance file, which must then sit in the output directory, so that `check` and `generate` read the same list: every attribute path of the schema (nested ones and every `oneOf` arm included), with its type and requiredness, when `upgradeFrom` is first set or changes. It keeps the file on later runs. The first config of each upgrade subtest leaves out every attribute that the release does not have in the same type, so a field added or retyped after the release is still tested by the normal lifecycle. It sets every attribute that the release requires, even one that is optional now. No config works with both providers when a new attribute is required, or when a required attribute of the release is gone or retyped, inside an object that the release has: that is an error. Set `upgradeFrom` to the latest release when you change it, because the list comes from the schema of that moment. `upgrade-minimal` does the same from the minimal config plus the fields in `upgradeMinimal`, and updates to the full config. Set `CORALOGIX_GENERATED_UPGRADE_ACC=1` and `TF_ACC_PROVIDER_NAMESPACE=coralogix` to run it, for every generated resource at once (`make testacc-generated-upgrade`). It is skipped otherwise. The `provider-migration` job of the acceptance workflow runs every test whose name ends in `GeneratedUpgrade`, so a new generated resource needs no workflow change.

Not covered: a check that the resource is gone after destroy, a oneOf group among the top-level fields, and checks on the elements of a set.

## Versioned output

Every generated Go file starts with this header:

```go
// Code generated by coralogix-iac-codegen v0.1.0-beta.10. DO NOT EDIT.
```

Find files affected by this generator version with:

```sh
rg 'Code generated by coralogix-iac-codegen v0\.1\.0-beta\.10' .
```

During beta, increment the beta identifier for each generator behavior change. After beta, use a patch version for fixes, a minor version for new supported behavior, and a major version for incompatible output changes.

## Beta boundary

This beta has no type mode. `generate` has no OpenAPI input flag. The module has no live OpenAPI fetch, custom field hook, force flag, or allow-unsupported flag. The only override is the `behavior-overrides.yaml` file of an existing resource. A Delete override makes one call: it cannot run a step before it, such as an update that an API needs before it archives, and it cannot send a request body. Data sources, provider registration, and automated delivery are separate work.

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
