# Coralogix IaC code generator

This beta generates new Terraform Plugin Framework resources from eligible OpenAPI contracts. Unsupported or ambiguous contracts stop with an error. A resource that users already have uses existing-resource mode (see below).

## Run the generator

`make codegen` in the provider root runs the generator and then the docs (see [Docs and registration of a new resource](#docs-and-registration-of-a-new-resource)). To run only the generator, run the command from this module:

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

The shared parts are the Go module `github.com/coralogix/coralogix-management-sdk/iac`, in the SDK repository. Every IaC tool reads the API the same way through it:

| `iac` package | Job |
|---|---|
| `source` | Finds the `openapi.yaml` of the SDK version that the provider `go.mod` pins. |
| `model` | Loads the OpenAPI, builds the model of one resource, and runs the contract checks. |
| `overrides` | The API keys of `behavior-overrides.yaml`: `api.*`, `unwrap`, and `skip`, `readOnly`, `optional`, `createAs`, `collection`, `keyedBy`, and `unwrap` on a field line. |
| `sdkcheck` | Lists the SDK names that the generated code uses, and checks that the pinned SDK has them. |
| `issue` | The eligibility report. |

This module adds everything that only Terraform needs: the Terraform keys of `behavior-overrides.yaml`, the Terraform checks (`TERRAFORM_NAME_COLLISION`, `UINT64_RANGE_UNSUPPORTED`), the provider clientset names, and the renderer. A fact or check about the API goes into `iac`. A fact or check about Terraform stays here. The two reports join, so `check` lists every problem at once.

The OpenAPI reader has no Terraform or SDK types. The generator completes every check and formats every file before it changes the output directory. An existing output directory must contain only Go files with the exact generator header. This rule prevents the generator from replacing a user-owned directory.

## Supported contracts

The beta supports complete new resources with one Create, Get, Update or Replace, and Delete operation. Create, Get, and Update must return the resource directly. A non-singleton Update or Replace operation must put the resource ID in its path. The ID can be a string, `int32`, or `int64`. Its type must match in every path and in the resource response. A body-only Update ID is supported only in existing-resource mode. The beta supports managed, immutable, and computed fields. A computed nested object or object collection has computed-only descendants. It supports scalar values, typed maps, ordered lists, and explicitly unordered sets, including sets of objects. It supports the documented OpenAPI one-of form. It supports PATCH with a required `updateMask` query parameter that has a documented field-mask pattern. The parameter can also have its proto name, `update_mask`, as the OpenAPI fork writes it. A 200 response can also be `allOf` with one `$ref`, as the fork writes a `response_body` field that has a description. In existing-resource mode, the response property that wraps the resource can be written the same way, `{"thing": {"allOf": [$ref]}}`. The pattern describes the whole comma-separated value, so it must accept a list such as `a,b`. It also supports full-replace PUT. In existing-resource mode, Delete can be a POST that the behavior-overrides file names, for an API that removes the resource with a call such as `/{id}/archive` instead of a DELETE (see below). In existing-resource mode, a string field can hold a YAML or JSON document that the API returns in another format; the behavior-overrides file says so (see below). A `date-time` string that a request sends, alone or as the items of a list or set, is a `timetypes.RFC3339` attribute with a validator that requires UTC (`Z`): the API returns every time in UTC, so a configured offset would read back as another text. A value that differs only in fractional seconds plans no change. A `date-time` that only the response has stays a plain string, with no validator. A component has one type for a time field in every place: when a request sends the field in one place, it is `timetypes.RFC3339` also where only the response has it, for example inside a computed value. The acceptance test uses `2030-01-01T00:00:00Z`, and `2030-01-02T00:00:00Z` in the update config. A Delete 200 response can be a component or an inline object with no fields, as the fork writes `google.protobuf.Empty`. A string pattern in Create, Update, and the resource response becomes a `stringvalidator.RegexMatches` validator, also on list items and map values. The three schemas must declare the same pattern. The free-text pattern `^[\s\S]*$` is no validator. The generated acceptance test keeps its made-up string when it matches the pattern. Otherwise the test value is built from the pattern, for example `00000000-0000-0000-0000-000000000000` for a UUID, and is not unique per run. When the built value does not fit the length limits, generation stops and asks for the value in `acceptance.yaml`. It supports a singleton, whose Get has no path parameter, when Create, Get, Update, and Delete use one path. Its `id` attribute is computed-only, with the resource type name as a static default. A singleton that always exists, with no Create or Delete or with a Get that never returns 404, is not supported.

Create, Update, and the resource response must use separate schemas. Inline Create and Update schemas are supported when each has its own identity. A fixed object without a `required` list has no required field. The OpenAPI fork cannot write an empty list, so a message whose fields are all optional, such as a PATCH body, has none. Required means not empty. A required string must declare `minLength: 1`, a required list `minItems: 1`, and a required map `minProperties: 1`. An optional scalar request field must use `x-coralogix-presence: true` only when it declares a default that is not its zero value and the zero value is valid, for example `false` next to a default of `true`: proto3 does not send a zero, so the server would store the default. An enum never needs it. The API returns a field without presence as its zero value (`false`, `0`, `""`). So a scalar without presence and without a declared default is read as if it declared its zero value as the server default, in Create, Update, and Get, when that zero value is valid: it gets the server-default handling below. A string with no format whose pattern is exactly `^[0-9]+$` or `^-?[0-9]+$` is a 64-bit number, so its zero value is `"0"`: these are the only patterns that the OpenAPI generator writes for one. Another pattern that accepts `"0"`, such as `^[A-Z0-9]+$`, is a plain string, and its zero value stays `""`. When the zero value is not valid, for example `minimum: 1` or `minLength: 1`, a configuration never holds it, so the attribute is plain `Optional` and a read of zero is null. A field with a line in the behavior-overrides file keeps its released behavior. A field that Create requires needs no annotation in Update: Terraform requires it, and a required value is never empty, so Update always sends a value. A field that a `skip` line removes is never sent, so the fields inside it need no annotation. A required bool or number must also use this annotation when its zero value is valid. The annotation states that omission differs from an explicit zero or empty value. For a client-owned scalar that is also optional in Get, it also requires round-trip presence: omission reads back as absent, an explicit zero reads back as present, and clearing reads back as absent. Lists, maps, and objects do not need the annotation. An empty list or map is the same as a missing one, and an empty object outside a `oneOf` arm is the same as a missing object. Generated state keeps the form that the configuration or the prior state has when both the response and that prior value are empty, so `[]`, `{}`, and a missing value do not cause a difference. A computed attribute with no prior value, for example after an import, keeps the value of the API. An empty `oneOf` arm selects that arm, so Get must return it. A top-level scalar server default must be valid for the field. Update can omit the default, because the generated resource reads only the Create default. If Update declares one, it must equal the Create default. The field must be optional in Create and Update, and Get must return it. Get may require it or declare the same default, and may also declare none: a response schema that a request shares should not claim that an omitted value is the default when older records read back without it. A read without the value is null in state, unless the prior value equals the default: the API can leave out a value that equals its default. A field with a server default needs no presence marker when omitting it cannot look like a configured value: an enum never sends its zero value, a zero value that is not valid is never sent, and a default that is the zero value is what an omitted zero stores. Any other zero, such as `false` next to a default of `true`, needs the marker. A default may sit next to an enum `$ref` (OpenAPI 3.1). Only `default`, descriptive keywords (`description`, `title`, `readOnly`, `deprecated`, `example`, `examples`, `$comment`), `x-coralogix-presence`, and foreign `x-` extensions can sit next to a `$ref`: the type comes from the target, so any other sibling, such as `pattern`, `prefixItems`, or `x-coralogix-collection`, is an error instead of a lost constraint. The default must fit the type and the limits of the target. A field that only Update and Get have, with a declared Update default, is managed too: Create stores the default, and the generated Create follows with an Update, which sends a server-set id in the body as Update does, when the configuration sets another value, or omits the field and Create stored another value than the default. In a new resource, a configured value that is still unknown stops Create before the API call. A new resource also supports a scalar server default on an optional nested field: the attribute is `Optional + Computed` with the same plan modifier, a request does not send its unknown value when the configuration omits the field (a configured unknown value stops the call), and an object that holds such a field reads as the omitted object when its attributes are empty, `""`, or their declared default. Other objects keep `""` as a value. A read that leaves out such a field keeps a prior value that equals the default. A nested server default inside a set is an error: a set element that a read returns without the default cannot be matched to the configured element. Terraform renders this contract as `Optional + Computed`. It does not insert a static value into the request. When a user removes an override, the plan becomes unknown. Update sends the field in `updateMask` without a body value. The API resets the field, and Get returns the declared default. Unordered arrays must use `x-coralogix-collection: set`. `uniqueItems` alone does not make an array unordered. A schema that is only `allOf` of one `$ref` uses the inner properties for these checks. Required fields are the union of the required lists. The resource model still reads `properties` on the schema itself, so a nested server default through `allOf` is unsupported.

A nested object can use other components in Create, Update, and the resource response, for example `ThingSpecCreate`, `ThingSpecUpdate`, and `ThingSpec`. The generator matches the nested fields by name, in objects, list items, map values, and one-of arms. Each nested field gets the lifecycle of a top-level field:

| Create | Update | Response | Nested field |
|---|---|---|---|
| yes | yes | yes | managed |
| yes | no | yes | immutable: a change replaces the resource |
| no | no | yes | computed |

Any other combination is an error. The kind, format, limits, enum values, defaults, and one-of arms of a field must be the same in every schema. Component names, descriptions, `required`, and presence can differ. An enum must use the same component in every schema. Create decides `required`. A one-of can require an arm in Create and allow no arm in Update and the response: a PATCH that changes only another field sends no arm. Terraform then requires an arm, as Create does. The opposite is an error. A computed nested field has no plan modifier, so it shows `(known after apply)` when the resource changes. A computed-only attribute, and every attribute inside it, has no validators and no document plan modifier, including checks that a field line adds for another place of the same component. A computed `id` still keeps `UseStateForUnknown`. Update leaves it out of the change check and the update mask. An immutable value that holds computed fields, for example a list of items with a server-set `id`, is compared without them, so only a change that Create sends replaces the resource. A behavior-overrides line cannot make a field inside an immutable value optional and computed: an omitted one would be unknown after any change and replace the resource. An immutable field in a list item also replaces the resource when a new item sets it, because Update cannot send it. Removing an object that holds an immutable value replaces the resource too. A `readOnly` property inside a component that the request shares with the response stays unsupported. The request must use its own component. Existing mode accepts it: the request does not send the field, so it is computed.

The generator rejects incomplete lifecycles, ambiguous operations, reused request schemas, missing scalar presence next to a default that is not zero, required strings, lists, or maps that accept an empty value, wrapped resource responses, body-only Update IDs, an optional Get response ID, unsupported or inconsistent ID types, fields that are optional in Create but required in Update, nested fields that only a request has or that only Update sends, unsupported required query, header, or cookie parameters, incompatible request and response types, invalid, undeclared, or inconsistent server defaults, lifecycle-specific root `oneOf` groups, nested `oneOf` updates without dotted mask paths, `writeOnly` fields, string patterns in Create or Update that Go regular expressions (RE2) cannot compile, such as a lookahead, or that an enum declares (a pattern that only the response has is ignored, because the configuration never sets that value), exclusive numeric bounds, `uint64` fields without a maximum string length of 18 or less, property-count constraints on objects that are not maps, unsupported unions, recursive schemas, free-form maps, Terraform field-name collisions, Go field-name collisions, Go component-name collisions, and missing SDK symbols. A field can be required in Create and optional in Update because Terraform still requires its configured value and can send it on every update. Nested server defaults of an existing resource, non-scalar server defaults, and generic write-only state handling are outside this beta. It reports all detected issues in stable order. It writes no output when any issue exists. Generated Create and Update code also stops before the API call if an unexpected unknown value remains. Generated Read warns before removing a resource that the API no longer finds. Read, Update, and Delete use the SDK `IsNotFound`, so a 404 of a route that does not exist, for example in a region without the service, is an error and does not remove the resource from state. Generated response conversion maps only the enum's exact `<PREFIX>_UNSPECIFIED` zero value to Terraform null. A business value that also ends in `_UNSPECIFIED` remains a value. The generator returns diagnostics if an enum list, set, or map contains the exact zero sentinel because removing a collection element would change its meaning.

The small golden contract uses one synthetic resource. It covers a clearable optional `description`, an `enabled` server default of `true`, an immutable required `kind` enum, a mutable required `config` one-of, a computed nested `status` object, a `spec` object with other request components and managed, immutable, and computed nested fields, also in list items, computed `create_time` and `update_time` values, an ordered `destinations` list, and an unordered `tags` set.

A second small golden contract, `archivedthing`, is an existing resource whose API has no DELETE. Its behavior-overrides file names the archive call as the Delete.

A third small golden contract, `configthing`, is an existing resource with JSON fields, one of them immutable, and a YAML field in a list of objects that keeps its prior order. Its behavior-overrides file sets `equality` on both.

A fourth small golden contract, `wrapthing`, is an existing resource whose API holds values in wrapper objects: a string, an enum, a required number, a list of wrapped items, an object, and a list inside two wrappers. One component collapses in one place and stays an object in another. Its behavior-overrides file has an `unwrap` list and `unwrap` field lines.

A fifth small golden contract, `echothing`, is an existing resource as an older API has it: Create, Get, and Replace wrap it as `{"thing": {"allOf": [$ref]}}`, Get and Replace hold it as a value field, Replace sends the server-set id in its body, and Replace echoes a server time and an internal flag that the resource skips. Create takes a quota limit flat while Replace and Get hold it in an object (`createAs`), each item requires a list that may be empty (`optional`), and its `oneOf` has an empty marker arm that the resource skips.

A further small golden contract, `suppressedthing`, is an existing resource whose path parameter is `thing_id` while its id is the property `uniqueIdentifier`, next to a version `id` that the file skips. Its behavior-overrides file has `api.idProperty`, `serverSetID`, `name` lines on a top-level and a nested field, an enum whose values have `names` (`mute` and `active`), a set of weekday numbers with `valueNames`, an empty oneOf arm with `asBool`, a filter whose required oneOf has a `defaultArm`, and a nested `frequency` group.

## Existing resources

A resource that users already have must not change when its handwritten code becomes generated code. Existing-resource mode keeps its released behavior. A file named `behavior-overrides.yaml` sits in the output directory. `generate` reads it without a flag. `check` takes `--overrides <file>`.

```yaml
resource: GlobalRouter        # the OpenAPI component; the file must match --resource
mode: existing
validators:
  inferred: false             # required: false drops contract limits, true keeps them
api:
  requestWrapper: router      # Create and Update send {"router": {...}}
  updateIDInBody: true        # Update has no id in its path
  clientSetID: true           # the user can set the id
  customMethods: true         # Get, Update, and Delete use different paths
  delete:
    operation: Service_ArchiveRouter # the API has no DELETE; this POST removes the resource
unwrap: [LuceneQuery]         # {"query": {"value": "x"}} is query = "x" in Terraform
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

A `required: true` key on a field that a nested component already requires is not stale when the field is a bool or number whose zero value is valid and that has no presence marker and no default: Terraform sends it every time, and the line states the presence that the contract cannot. The rule is: **a field with a line keeps the released behavior that the line states. A field without a line follows the API contract**, and the contract must say how (for example with `x-coralogix-presence`). A new optional scalar API field needs no line unless it declares a default that is not its zero value and does not state its presence. Then the generator stops with `FIELD_PRESENCE_UNKNOWN`. A new scalar without presence or a default gets the zero-value handling of a new resource. A list, map, or object needs no annotation. Then ask for the annotation in the API contract (a proto3 `optional` field), or probe what a read returns for an omitted value and write a line. A new value of an enum also stops the generator (`ENUM_VALUE_UNDECIDED`) until the value is in `values` or in `rejected`.

The file is strict:

- An unknown key is an error.
- A line that matches nothing in the contract is an error (`OVERRIDE_UNUSED`). The contract may begin to state the same fact (`readOnly` or `required`). The generator then names the stale key. If the line sets other keys, keep them: the contract does not state them.
- Every key of the file changes the generated code, or it is not a key. When a key is added to the reader, the renderer must support it in the same change. The generator never writes code that ignores a line.

Keys of a field line: `skip`, `readOnly`, `required`, `optional`, `createAs`, `collection: set`, `keyedBy`, `name`, `requiresReplace`, `uniqueBy`, `description`, `markdownDescription`, `deprecation`, `computed`, `useStateForUnknown`, `useNonNullStateForUnknown` (keeps a non-null state value; a null one stays unknown, so the server can fill it in), `default` (a string, a bool, or a number), `readEmptyAs: "null"`, `readNullAs: "empty"`, `requireOne` (on the field that holds a oneOf, or on an arm of a top-level oneOf: ExactlyOneOf instead of ConflictsWith or Conflicting, for a released resource that required an arm; a field that is neither is an error), `requestValue` (an API value Create and Update always send; the field is not a Terraform attribute), `requestValueBy` (a map from an object field of the component to the API value that Create and Update send when that field is set, such as `type: {requestValueBy: {logsQuery: E2M_TYPE_LOGS2METRICS, spansQuery: E2M_TYPE_SPANS2METRICS}}`; the field is not a Terraform attribute, each key must be an object field, and each value a value of the enum), `keepPriorOrder`, `equality` (`yaml` or `json`), `unwrap` (`true` or `false`), and `validators` (`oneOf`, `sizeAtLeast`, `lengthAtLeast`, `enum: true`, `enumExceptZero: true`, `between: [min, max]` on a number (on an integer, each bound is a whole number of at most 2^53 in size, an `Int32` one within its range, so the validator states it exactly), `exactlyOneOf` or `atLeastOneOf` with the Terraform names of sibling attributes, `atLeast` (a number: `int32validator`, `int64validator`, or `float64validator`), and `regexMatches: {pattern, message}`). `regexMatches` keeps a released pattern that the API does not check, with the released message; the pattern must compile in Go. `enum: true` accepts the values of the field's enum as the `enums` line states them, including the zero value, so the validator and the conversion maps cannot disagree. `enumExceptZero: true` accepts those values except the zero value. The conversion maps still contain the zero value, so a response that has it still converts, and a user cannot configure it. Other keys: `markdownDescription` of the resource, `unwrap` (a list of wrapper components), `schema.version` and `schema.upgrade.<n>` (the frozen prior schema as `<import path>.<Func>`, upgraded by reading the resource), `types.<Type>.required: []`, `types.<Type>.extraAttributes` (Terraform-only map attributes that the contract does not have; `elementType` is `string` or `int64`, `writeOnly: true` emits `WriteOnly: true`, flatten writes a typed null, expand does not send them; the generated acceptance test leaves them out unless `values` or an `upgradeCases` entry supplies the map, because a made-up map key is not an API field), and `enums.<Enum>` with `zero`, `values` (accepted), `rejected` (in the contract, not accepted), `verbatim: true` (keep the contract's spelling; the default lowercases each value), `trimPrefix` (removed from each value first: `SLO_TIME_FRAME_7_DAYS` with `trimPrefix: SLO_TIME_FRAME_` is `7_days`), and `names` (the Terraform value of an accepted value, for a released spelling such as `SEVERITY_DEBUG: Debug`; it takes precedence over `trimPrefix` and the case; two values cannot share a name), and `aliases` (a Terraform value that the resource accepts, the zero value or a value, and the value that the plan replaces it with, such as `unspecified: percentage` for a type that the API stores as percentage when it is unspecified; a plan of the alias that equals the state plans no change, and the generated resource then keeps the state's unset computed values). `typeName` is the released resource type name without the provider prefix, when it is not the name of the component (`slo_v2` for the `Slo` component). Every value of the contract must be in one of the two lists. An `enums` line applies to a single enum field and to a list or set of the enum: each element is mapped, the zero value too when the line names it, and `enum: true` on the list or set validates each element. The generator reports a map of the enum. When `clientSetID` is on, flatten returns an error for a response without an id, because a proto3 optional id cannot be required in the contract. A client-set id has `UseStateForUnknown` and `RequiresReplace`: a new id replaces the resource, and a removed id keeps its value. A `useStateForUnknown` key on its line changes nothing, so the generator reports it (`OVERRIDE_UNUSED`). `zero` names the Terraform value of a `*_UNSPECIFIED` sentinel. A first value that is a business value, such as `AUTO`, goes in `values`.

`keepPriorOrder` on a list of objects keeps the order of the prior list, the plan after Create or Update or the state before Read, when the API returns the items in another order. Each API item pairs with the first prior item that it equals, compared on the fields that the request sends, with equality lines applied. When every item pairs, the state keeps the prior order. When an item changed, or the API added or dropped one, the API order stays, so the plan shows the change. When the request sends the items with their own component, for example a Create item without the id and the hash that the response adds, the generated code converts each API item to the model and both it and each prior item to that request component, so a field that only the server sets never decides the pairing. Create and Update must then send the same fields of the item. The generator rejects the key on a list that no request sends.

`equality: yaml` or `equality: json` compares a string field as a YAML or JSON document, for a field that the API returns normalized: in another format, key order, or whitespace. Two values are equal when they parse to the same value. Numbers compare by their exact value, so two numbers that round to one float64 stay different, and a YAML integer never equals a YAML float. Merge keys (`<<`) expand as `yaml.Unmarshal` expands them, because an API that normalizes YAML returns the merged map without its anchor: a key of the map itself wins, and an earlier map of a merge list wins over a later one. A value that does not parse equals only the same text, so it never panics and never hides a change, and neither does a YAML document whose aliases hold the document itself or expand to more than about a million values. A plan modifier keeps the state when the configured document equals it. It runs before the other plan modifiers of the field, so an equal document in an immutable field does not replace the resource, and flatten keeps the text of the plan or the state when the API returns an equal document. Without a prior value, after an import for example, the state takes the value of the API. A resource with an equality line also gets a `ModifyPlan`: Terraform marks every computed attribute that the configuration does not set unknown when the configuration text changes, before the plan modifiers run, so a change of format alone would still show `(known after apply)`. When every known planned value is the state, and the configuration sets none of the unknown ones, the plan is the state. With `keepPriorOrder` on the list, the items match the prior items with the same document comparison, so a list that the API reorders and reformats keeps its order and text. The key works in nested objects and in lists of objects, but not in the objects of a map or set or in a computed object: their flatten has no prior value, or no order to pair prior items by, so the generator reports `OVERRIDE_PRIOR_CONTAINER` there, for `keepPriorOrder` too. The generator reports it on a field that is not a plain string (`OVERRIDE_EQUALITY_NOT_STRING`: a list, an object, a number, an enum, or a string with a `format`) and on a field that only the server sets (`OVERRIDE_EQUALITY_COMPUTED`). A resource with an equality line cannot have a server default, because that default plans unknown on purpose.

`api.delete.operation` names the operation that removes the resource when the API has no DELETE. It must be a POST whose path is the Get path plus one segment, for example `/v1/things/{id}/archive`, with the id path parameter of Get as its only parameter, no request body, and a 200 response. The generated Delete calls it with the id, ignores the response, and treats a 404 as deleted, as for a DELETE. The generator reports an operation that does not exist (`DELETE_OVERRIDE_OPERATION_NOT_FOUND`), another method (`DELETE_OVERRIDE_METHOD_INCOMPATIBLE`), another path (`DELETE_OVERRIDE_PATH_INCOMPATIBLE`), another or an extra path parameter (`DELETE_OVERRIDE_ID_INCOMPATIBLE`), a required query, header, or cookie parameter (`REQUIRED_PARAMETER_UNSUPPORTED`), and a request body, even an empty one (`DELETE_OVERRIDE_BODY_UNSUPPORTED`): the generated call sends only the id. It also reports the key when the resource passes every check without it, so that a DELETE of the API is the Delete (`DELETE_OVERRIDE_UNNEEDED`). `generate` includes the SDK symbol checks in that decision; `check` has no SDK and assumes, as for every SDK symbol, that the SDK matches the contract. A DELETE that the generator cannot use, or any other issue without the key, leaves the override needed, and when `--delete-operation` also names an operation (`DELETE_OVERRIDE_CONFLICT`). `api.delete` is an object, so that a later key can add a step before the call. No such step exists yet.

`optional: true` makes a field optional that a request of the contract requires, for a released resource that lets the user leave it out. The API accepts that, but the contract cannot say so, typically because the request shares the component with a response that always has the field. The field must be a list or a map: the SDK holds a required scalar or object as a value, which Create would send as its zero value. A list that the configuration leaves out is sent as `[]`, not `null`: the request still requires the key. The generator reports the line when no request requires the field: for a nested component, its own required list; for the resource component, the Create and Update requests (`OVERRIDE_UNUSED`).

`collection: set` makes a list a set, for a released resource whose attribute is a set while the contract has a list. The generator reports the line on a field that is not a list, and when the contract already marks the list `x-coralogix-collection: set` (`OVERRIDE_UNUSED`).

`keyedBy: {key, value}` shows a list of objects as a map keyed by a required string property of the object, for a released resource that has the map (`metricFields: {keyedBy: {key: targetBaseMetricName}}`). The map value is the rest of the object. With `value`, it is that one string property, and the object must have only the key and the value (`metricLabels: {keyedBy: {key: targetLabel, value: sourceField}}` is a map of strings). Create and Update send the items in the order of the keys. Read builds the map from the items in any order, so the order of the API does not show as a change. The generator reports a field that is not a list of objects, a key that is not a required string, and a value next to other properties (`OVERRIDE_UNUSED`).

`keyedBy: {key, object: true, arms, keyAs}` shows a list of objects keyed by a required enum property as an object with one attribute per accepted value of the enum, named by its `enums` line (`names` gives the released spelling). Each attribute is the item without the key, optional and computed, because the API can return an item that the configuration does not have. `arms` gives an enum value the oneOf arm of the item that it holds: that attribute has the arm, required, and the others have no arm. `keyAs` names a string property that Create and Update set to the attribute name. The lines of the item component apply to each attribute, so `unwrap` and `name` can show the arm as one attribute:

```yaml
types:
  v2.MetricField:
    fields:
      aggregations:
        keyedBy: {key: aggType, object: true, arms: {AGG_TYPE_SAMPLES: samples, AGG_TYPE_HISTOGRAM: histogram}, keyAs: targetMetricName}
  v2.Aggregation:
    fields:
      targetMetricName: {readOnly: true}
      samples: {unwrap: true, name: type}      # aggregations.samples.type
      histogram: {unwrap: true, name: buckets} # aggregations.histogram.buckets
```

Read puts each API item in the attribute of its key, in any order, and leaves out an item of another value. The attributes take the `useStateForUnknown` or `useNonNullStateForUnknown` of the line, as the object does: the API can fill in every item. The generator reports a key that is not a required enum, an arm of a value that the enum does not have or of a property that the item does not have, and a `keyAs` that is not a string (`OVERRIDE_UNUSED`).

`name` sets the Terraform attribute name of a field, for a released resource whose attribute name differs from the name that the JSON name gives (`applicationnameFilters: {name: applications}`). The requests and the responses keep the JSON name, and issues name the contract field. The name is lower case letters, digits, and `_`. The generator reports a name that the JSON name already gives, and a name that another field of the component has (`OVERRIDE_UNUSED`); a skipped field and a `requestValue` field have no attribute, so a name can take their name. A `name` line on a `requestValue` field is an error.

`createAs` folds top-level Create fields into an object field: `quota: {createAs: {limit: limitValue}}` says that Create sends `quota.limit` as the Create field `limitValue`, while Update and Get hold the `quota` object. Validation and the model read the Create body with the flat fields moved into the object, so the object has the normal lifecycle. The generated Create writes each flat field from the object, and sends nothing for a null object. A folded property is a scalar, a list of scalars, or an enum. The generator reports a Create field that does not exist (`CREATE_AS_UNSUPPORTED`). A property of the object that only Update and Get have must be read-only, by the contract or a `readOnly: true` line.

A `readOnly: true` line works on a field of a nested component as well: the requests that share the component do not send it, and it is computed. A `computed: true` line with `useStateForUnknown` (or `useNonNullStateForUnknown`) on a field that Get requires allows a field that Create leaves optional and Update requires: state always holds the value that Get returned, and the plan keeps it, so Update can send it. Without the plan modifier, a change of another attribute plans the field unknown, and Update would leave it out.

`validators.inferred: false` removes the limits of the contract. `validators.inferred: true` keeps them as validators, for a resource without a released version or one whose released validators are exactly the limits of the contract; the key is required. `validators.rejectUnknown: true` stops Create and Update before the API call when a request value is still unknown, as for a new resource; a released resource left an unknown computed value out of the request, so only a resource without a released version sets it. A `oneOf` group validator states the structure of the request, so it stays. `clientSetID` needs the id property in the Create body, and `updateIDInBody` needs it in the Update body. When the Create body does not have the id, the server assigns it: the attribute is computed, and Update sends the value from state. Otherwise the generator reports an issue. A server-set id, one that the contract or a `readOnly` line makes read-only, is no Update field, so the generated Update copies the state id into the body: into the resource that the body wraps (`requestWrapper`), or into the id field of a body that is the resource itself. Create and Update can take one body type, for example an inline copy of the resource that the SDK names `Thing1`; each gets its own expand function. A response field that holds the resource can be a value, as openapi-generator writes a required field.

`singletonID: true` (a top-level key, because only Terraform shows the id) is for a singleton (Get has no path parameter) whose API resource has an `id` field that the server sets. The `id` attribute then holds that value, as for a resource with an id path, instead of the fixed type name; Create, Get, Update, and Delete still take no id, and Update does not send the id. Flatten falls back to the type name when the API returns no id. Mark the field `readOnly: true`: the id must be a string, and a Create or Update body that lets the user set it is refused, because flatten would replace the value. The generator also stops when the resource has an id path parameter or no id field, and when the key is combined with `api.clientSetID`, `api.serverSetID`, or `api.updateIDInBody`.

`api.serverSetID: true` states that the server sets the id and returns it in every response, although the Get response does not require it: a schema that the request shares cannot require a field that only the server sets. Flatten returns an error for a response without an id, as for `clientSetID`. A Create whose response has no id then fails after the API created the resource, so use the key only for an API that always returns the id. The id must be read-only in the Create and Update bodies, by the contract or by a `readOnly: true` line, or absent from them. Otherwise the user could set it, and a body-only Update would send the planned id, so the generator reports `SERVER_SET_ID_IN_REQUEST`. The generator reports the key when Get requires the id (`SERVER_SET_ID_UNNEEDED`). `clientSetID` and `serverSetID` exclude each other.

`api.idProperty` names the property that holds the id when its name is not the name of the path parameter, for example `uniqueIdentifier` for `/rules/{rule_id}` next to a version `id` that every Update replaces. Get and Delete pass its value as the path parameter, every id rule above applies to it (`clientSetID`, `serverSetID`, `updateIDInBody`, a `readOnly` line), and the generated Update copies it into the body. The Terraform attribute takes the name of the property, so a released `id` needs a `name: id` line, and a field that already has that name must be skipped. The Go SDK names a path parameter in lower camel case (`alertSchedulerRuleId`), and the SDK check expects that name. With `updateIDInBody`, an id property that Create does not send and that Update and Get have is the server-set id, and is computed. An `idProperty` that names the Get path parameter changes nothing, so the generator reports it (`ID_PROPERTY_UNNEEDED`).

`valueNames` on a field line gives the numbers of an integer list or set a released name each, for example `{1: Sunday, 7: Saturday}` for days that the API numbers. The attribute is a list or set of strings with a `OneOf` validator of the names. Expand sends the numbers, and flatten returns the names. A name that is not in the line, or a number that the API returns and the line does not name, is an error, not a zero or an empty name. Names must be unique and not empty. On any other field the key is an error.

`terraformType: Int64` on a field line keeps the released Terraform type of an int32 field, or of a list or set of int32. The attribute and the model are Int64. Expand converts each value to int32 and reports a value outside the int32 range at apply, because the API holds int32; flatten widens the API value. On any other field the key is an error.

`asBool: true` on a field line shows an object without fields as a bool, for example a oneOf arm `alwaysActive: {}`. `true` sends the empty object. `false` is an error at apply, as in the released resources; remove the attribute instead. A response that has the object reads as `true`, and one without it reads as null. A oneOf group validator of the arm is a `boolvalidator`. On an object with fields the key is an error.

`defaultArm: true` on a field line marks the arm of a oneOf group that Create and Update send empty when the configuration sets no arm, for an API that requires an arm and reads an empty one as "all", for example `alertUniqueIds: {}`. The group allows no arm in Terraform (`ConflictsWith`). The arm must be a list of strings in wrapper objects (`unwrap`) with `readEmptyAs: "null"`: it is sent as the empty wrapper (`{}`), and the API's empty arm reads back as the null that the plan has. An object arm would read back as an object of nulls, which no plan has, so the apply would fail. One arm of a group can have the key, only in a nested object, and only with a full-replace Update (PUT): a PATCH mask names the arm that the plan selects, not the arm that expand adds. The API reads the arm back empty; the state keeps the form of the configuration, as for every empty value.

A component can hold a field that the user sets in one place and that only the server sets in another, for example a read-only copy of the request value. The line states how the user sets the field. The read-only copy stays computed and takes only the text of the line (`description`, `markdownDescription`, `deprecation`): a mode, a default, a validator, or a plan modifier there would plan a value that the server does not set.

`groups` makes required Terraform objects that the API does not have, each holding top-level fields of the resource:

```yaml
groups:
  sli:
    fields: [requestBasedMetricSli, windowBasedMetricSli, apmSli]
    requireOne: true          # the fields are the arms of a top-level oneOf; ExactlyOneOf instead of ConflictsWith
    markdownDescription: "..."
  window:
    fields: [sloTimeFrame]
```

Expand writes the fields of a group into the API object that holds them, and flatten reads them from it. The arms of a top-level oneOf are in one group or in none; in a group they check each other as the arms of a nested oneOf. A group holds fields that the user sets and that Update sends, so it needs a full-replace Update (a PATCH mask names the API fields), and it cannot hold the id, an immutable field, or a field with a server default. The generator reports a group field that is no top-level field.

`types.<Type>.groups` does the same for the fields of a nested component, for example `frequency = {daily = {}}` for the `daily` arm of a recurring schedule:

```yaml
types:
  Recurring.Dynamic:
    groups:
      frequency:
        fields: [daily, weekly, monthly]
        markdownDescription: "..."
```

The rules are those of a top-level group: the group object is required, holds every arm of a oneOf of the component or none, needs a full-replace Update, and holds only fields that the user sets and that have no server default. Its name cannot be the name of another attribute of the component. Its model struct is `<Component><Group>Model`; a component, the resource, or another group with that model name is an error. `requireOne: true` needs a group that holds the arms of one oneOf and no other field. A `<Type>` that is no component of the contract, or no object of the resource, is an error, as is a group field that the component does not have.

A read-only field inside a component that the request shares with the response is computed in existing mode: the request does not send it, and flatten reads it. `readEmptyAs: "null"` also works on a list or set of strings, also inside wrapper objects (`unwrap`), for a list that the API returns as `[]` when the request did not send it.

`api.customMethods: true` is existing-mode only. Get and Delete may use different paths when each path has the same `{id}` parameter. Update has no path parameter, and the id stays in the body. New resources keep the rule that Get is Create plus `/{id}`, Update is the Create path, and Delete is the Get path. `api.operations` names the four operationIds, so `tfgen generate` needs no operation flags.

`enums.<Enum>.readRejected: true` reads a rejected value back as the API spelling. Without it, that value fails the read. `readRejected: lower` reads it in lower case instead, for a released resource that lowercased an unknown API value; it excludes `verbatim`. A missing enum reads as null. `x-coralogix-presence` means the same thing: the value was not set. A string `default` on the field is the reading of a missing value for that resource, including a missing unwrapped value.

`enums.<Enum>.names` gives an accepted value its released Terraform spelling when `trimPrefix`, `verbatim`, and the lower case do not make it. A name wins over `trimPrefix`, for example `SCHEDULE_OPERATION_ACTIVATE: active` or `DURATION_FREQUENCY_MINUTE: minutes`. The conversion maps and the `enum: true` validator read the same spelling, so they cannot disagree. A name for a value that is not in `values`, an empty name, a name that is the spelling the value has anyway, and two values (the zero value included) with one Terraform spelling are errors. With `readRejected`, a rejected value reads back as its API spelling, so a name cannot be that spelling.

`requiresReplace: true` on a top-level field that Create, Update, and Get all have makes a change of the field replace the resource, as a released resource did, although Update accepts it. A PATCH Update does not send the field. A PUT Update replaces the whole resource, so it still sends the field: without it, an update of another field would clear it. It cannot be combined with `computed: true` or with `requestValue` or `requestValueBy` (such a field is no attribute, and Update always sends it). A PATCH Update body cannot require the field: the SDK holds a required field as a value and would send its zero value. With PATCH, at least one field must stay that Update can change: a resource without an in-place update is not supported.

`types.<Type>.unsupportedArms` names oneOf arms that the released resource does not configure, with `unsupportedSummary` and `unsupportedDetail`; the three keys are set together. The generator removes the arms from the model after the iac model is built, so they stay out of the schema and out of every request, and flatten reports the summary and the detail when a response selects one. An arm must still be a shape that the iac model reads. An unsupported arm cannot have a field line, and a oneOf group that requires an arm must keep one. The arms must belong to a nested object, not to the resource component itself, and the discriminator of a oneOf is no arm. A `types` line or an `unwrap` entry of a component that only the removed arms reach is an error, because it would change nothing. An `enums` line of such an enum stays required: the contract check runs before the arms are removed.

`updateMask: omit` (top level, with `api.updateMaskInBody`) sends no update mask, as a released resource that never sent one did: the server updates every field present in the body. The key also states that the server replaces the whole value of each top-level field in the body, nested objects included, so a nested attribute that the configuration removes is cleared. A server that merged nested objects would keep it and show a diff on every plan; the generator cannot check that, so record it before using the key (the AI evaluation fidelity case "update without a mask replaces a nested object" does). The generated Update then needs no mask path for a changed oneOf arm, so the generator does not report `UPDATE_MASK_NESTED_ONEOF_UNSUPPORTED` and does not check the mask paths against the mask pattern. A `requestValueBy` field is allowed, because the Update body holds its value. An optional field that Update sends must be required, have a default (a zero default, such as `false`, `0`, or `""`, only with `x-coralogix-presence` in Update, which otherwise does not send it), or keep its state value (`useStateForUnknown`, or `useNonNullStateForUnknown`: a null prior value has nothing to clear), because removing it from the configuration cannot clear it. The check uses the Terraform name, so a `name` line is covered.

`unwrap` removes wrapper objects that the API has and the released resource does not show. A wrapper is an object component with exactly one property, for example `LuceneQuery` in `{"query": {"value": "error"}}`. Terraform shows `query = "error"`.

```yaml
unwrap: [LuceneQuery, UUID]   # collapse these components in every place
types:
  Variable:
    fields:
      single: {unwrap: true}  # collapse the wrapper of this field only
      raw: {unwrap: false}    # keep the object here, although the list names its component
```

The top-level list names components. A field line names one place: the value of the field, or the items of a list or set. The attribute keeps the name of the field that holds the wrapper and gets the type of the value inside it: a scalar, an enum, a list, an object, or a oneOf. When that value is also a wrapper that collapses, both layers go (`selection: {list: {values: [..]}}` is `selection = [..]` when the list names both components). A list of wrapped items is a list of the values (`widgetIds: [{value: "a"}]` is `widget_ids = ["a"]`). Expand sends no wrapper for null, and a wrapper with the value for any other value, `""` and `0` too. Flatten reads a missing wrapper, and a wrapper without its value, as null. An item of a list without its value is the zero value, so the list keeps its length. A wrapper states the presence of its property, so that property needs no `x-coralogix-presence`. The same property in a kept object still needs it when the rules above ask for it. The update mask names the wrapper fields of a value inside a wrapped object. The field lines of the attribute (`description`, `computed`, `default`, `readEmptyAs`, `validators`) work as for any field. The generator reports a component that is not an object with one property, that has a oneOf, whose property has a server default, or that a request sends without its property, a map of wrapped values, and a wrapped resource id (`UNWRAP_COMPONENT_UNSUPPORTED`); an `unwrap: true` line on a field that holds no object component (`UNWRAP_FIELD_UNSUPPORTED`); `keepPriorOrder` or `equality` on a wrapped field (`UNWRAP_COMBINATION_UNSUPPORTED`); a list entry that no field holds, an `unwrap: false` line whose component the list does not name, and a line with other keys on the property of a component that collapses in every place, because Terraform has no attribute for it (`OVERRIDE_UNUSED`). Put those keys on the field that holds the wrapper. A new resource has no wrappers: its API returns the value itself.

`api.updateMaskInBody: true` is for a released PATCH whose `updateMask` is an optional string on the JSON body, not a query parameter. It is an error on a full replace (PUT). The generated schema has no mask attribute. Update builds the same body and the same changed-field mask as a query-parameter mask, then sets that string on the body. The server ignores a field that is not in the mask. A mask that is also a query parameter is an error. A mask pattern that names only top-level fields cannot change a nested oneOf (`UPDATE_MASK_NESTED_ONEOF_UNSUPPORTED`): the selected arm has to be a dotted path such as `config.sqlLoad`, because a parent path such as `config` can leave the old arm in place.

`skip` cannot name the resource id or an arm of a `oneOf` group, except an empty object arm of a group that allows no arm, such as an `Empty` marker (`none: {}`): it holds no value, so Terraform never selecting it leaves the group as the contract allows. A skipped top-level field has no lifecycle: it can be a value that Replace echoes in its body but Create does not have, such as an internal flag, because the generated requests never send it. A request that requires a skipped field is an error (`SKIP_REQUIRED_FIELD`). `readEmptyAs: "null"` works for a string, an object, and a list or set of objects. An omitted string reads as null. A configured `""` stays `""`, because the API returns an empty string for both. The generator reports any other use, because the key would do nothing. `requestValue` on a nested field that the request schema omits because it is read-only is still sent: the SDK struct has the field. A `requestValue` field is not in the Create or Update check lists. Those lists read Terraform attributes, and this field is not one. The same is true when the request schema still contains the field.

Existing mode relaxes these rules of the contract: a response that wraps the resource, also with other values next to it (the generated code reads only the resource), a request that wraps the resource and shares its schema, the id in the Update body, a client-set id, a server-set id that Get does not require when `api.serverSetID` says so, enum values without the zero prefix, a missing `required` list on a named object, and nested server defaults. It also turns off the presence check for a field with a line. A top-level field with a server default in the contract uses that default, as in a new resource. A `default` line replaces it: the field gets that static default, and the generator does not check the contract default. A `computed: false` line replaces it too, because a server default needs a computed attribute. It does not generate regular-expression validators. It ignores a string pattern.

Existing mode also keeps the prior form of an empty value, as new resources do. The API returns an unset list or map as an empty one, because proto3 has no presence for them, so without it a configuration that omits one fails the apply with `was null, but now` an empty value. `flatten` and the field lines (`readEmptyAs`, `computed`) decide what a read writes before that rule applies. A state upgrade has no prior of the new schema version, so its state takes the value of the API.

`uniqueBy: <property>` on a top-level list or set of objects makes the generated resource report an item that repeats the string property of an earlier one, at validation and again before the write (a value from another resource is unknown while Terraform validates). With `validators.inferred: true`, the generated resource also reports a float attribute that the API would return as another float32 (`0.1234567891` is stored as `0.12345679`), because the state would then differ from the configuration; `inferred: false` leaves the check out, as it does every limit of the contract.

`Hooks` also has `ValidateConfig` (checks over several values that no key expresses, such as a rule that depends on the value of another attribute). A write runs the generated checks, then `BeforeWrite`.

A resource in this mode exports `Flatten` for a handwritten data source. `NewResource` takes `Hooks`, handwritten steps around the generated CRUD: `BeforeWrite` changes the request body, `BeforeFlatten` changes the API value before flatten (for a value that only an older record has, such as a deprecated field that a newer one replaces), and `AfterRead` changes the state after flatten. A data source that calls `Flatten` calls the same `BeforeFlatten` step itself. A frozen prior schema stays in its own package.

## Acceptance tests

A new resource always gets `acceptance_test.go`. An existing resource gets it only with an `acceptance.yaml` in the output directory. The test is an external test package (`<package>_test`) that runs against a real tenant. It serves the provider through `provider.MuxServer`, the SDKv2 and the framework providers behind one mux as in the binary, so a prerequisite can be any resource of the provider. It needs no helper from the provider tests. A new resource needs the file only for what the schema cannot state, such as a prerequisite resource or a real value; without it, the test makes up every value. When the generator cannot make the test of a new resource, the resource is not generated: its docs example is the full config of the test. `--acceptance <file>` names another file. `check --acceptance <file>` builds and renders the same test without writing it, so a wrong file fails before the SDK exists.

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
upgradeCases:                   # extra upgrade subtests (needs upgradeFrom)
  - name: write-only
    values:
      token_wo: '{ secret = "x" }'  # extraAttributes need a real map key, not a made-up `key`
```

The test makes its configs from the final schema, after the behavior overrides. Each lifecycle is a subtest on its own resource, so a failure in one does not hide the others:

- `full-lifecycle`: the full config sets every attribute the user can set, and the test checks each value. Then an import step compares the imported state with the state (`ImportStateVerify`). The update config changes the strings, numbers, and bools. An immutable attribute and an enum keep their values, because the valid values of other fields can depend on an enum. Then another import.
- `minimal-lifecycle`: the minimal config sets only the required attributes, and the ones listed in `minimal`. Optional attributes that the server does not fill must leave the state. Then an import, the full config, and the minimal config again.

After every apply, the framework plans again and fails on a non-empty plan. The made-up values are: the first accepted value of an enum (not `unspecified`), `true` and `false`, `1` and `2` (or the start of a range validator and the next number), and `@{run}-<attribute>` for a string (cut or padded with `x` to fit a length validator). One arm of each `oneOf` group is set. A made-up collection has one element, made by the same rules and the validators of the elements. The generator fails on a kind of attribute it cannot make, or on a collection whose size validator rejects one element, and asks for a value in `acceptance.yaml`. A path in `values`, `skip`, `minimal`, or `upgradeCases[].values` that matches no attribute is an error. A nested path in `minimal` (or `upgradeMinimal`) also sets its optional parents, with only their required fields. A required attribute in `skip` is an error too: no config can leave it out. After the minimal config removes an `Optional` and `Computed` attribute, the test checks the value that the schema states: a static or server default, or, with `UseStateForUnknown`, the value of the full config. Without one, the API decides, and only the empty plan and the import check it. A `values` path names the attributes from the resource down, with `[]` for each list or set element: `rules[].targets[].connector_id`.

With `upgradeFrom`, a second test has two subtests. `upgrade-full` creates the full config with that released provider, plans it with this build (no change expected), applies the updated config, and imports. The apply checks the plan action only where it is sure: no change for the same config, and replace when an immutable value surely differs, also inside an object or a list item. It compares the immutable values of the real configs, so a `oneOf` keeps the arm that the config takes. When only one of the two configs sets a nested immutable value, the test checks no action. Otherwise the plan must not replace the resource, and may update it or not, because a default or an API value in state can equal what the next config sets; the value checks after the apply catch a missing update. When an immutable `Optional` and `Computed` attribute without a known default is left out of the first config, the test checks no action and keeps the empty-plan check. The released provider rejects an attribute that it does not have, so the generator writes `upgrade-attributes.yaml` next to the test and the acceptance file, which must then sit in the output directory, so that `check` and `generate` read the same list: every attribute path of the schema (nested ones and every `oneOf` arm included), with its type and requiredness, when `upgradeFrom` is first set or changes. It keeps the file on later runs. The first config of each upgrade subtest leaves out every attribute that the release does not have in the same type, so a field added or retyped after the release is still tested by the normal lifecycle. It sets every attribute that the release requires, even one that is optional now. No config works with both providers when a new attribute is required, or when a required attribute of the release is gone or retyped, inside an object that the release has: that is an error. Set `upgradeFrom` to the latest release when you change it, because the list comes from the schema of that moment. `upgrade-minimal` does the same from the minimal config plus the fields in `upgradeMinimal`, and updates to the full config. `upgradeCases` adds further subtests: each is the full upgrade config plus the case `values`, created with the released provider and planned with this build (no later update or import). Use a case for extraAttributes that need a real map key, and list those attributes in `upgrade-attributes.yaml` when the release has them. Set `CORALOGIX_GENERATED_UPGRADE_ACC=1` and `TF_ACC_PROVIDER_NAMESPACE=coralogix` to run it, for every generated resource at once (`make testacc-generated-upgrade`). It is skipped otherwise. The `provider-migration` job of the acceptance workflow runs every test whose name ends in `GeneratedUpgrade`, so a new generated resource needs no workflow change.

A oneOf group among the top-level fields gets one arm, as a nested group does: the first arm that the file does not skip. Not covered: a check that the resource is gone after destroy, and checks on the elements of a set.

## Docs and registration of a new resource

Run the generator and the docs in one command from the provider root:

```sh
make codegen RESOURCE=resource_name OUT=resource_name
```

`--out` must be a directory in `internal/provider/generated`, because `provider.go` imports only the registry there. For a new resource, `generate` also writes three files outside the output directory:

| File | What it holds |
|---|---|
| `internal/provider/generated/registry.go` | `Resources()`, the constructors of every new resource. `provider.go` appends them. |
| `examples/resources/coralogix_<type>/resource.tf` | The docs example: the prerequisites and the full config of the acceptance test. `@{run}` becomes `example`, and `@{env.NAME}` becomes `<NAME>`. |
| `examples/resources/coralogix_<type>/import.sh` | `terraform import coralogix_<type>.example "<id>"`. A singleton uses its type name as the id. |

`make generate` then renders `docs/resources/<type>.md` from the schema and these files.

A resource that users already have keeps its handwritten registration and examples. The generator tells the two apart by `behavior-overrides.yaml` in the output directory, so an existing resource in the provider must keep the file there. The registry lists every generator-owned package without that file.

The generator does not replace a file that it did not write. An existing example directory stops a new resource, unless an earlier run generated the same resource in the same output directory. A `registry.go` without the generator header stops every run. The generator writes every file to a staging place first. It then moves the output directory, the examples, and the registry last, so a package is in place before the registry names it.

## Versioned output

Every generated Go file starts with this header:

```go
// Code generated by coralogix-iac-codegen v0.1.0-beta.36. DO NOT EDIT.
```

Find files affected by this generator version with:

```sh
rg 'Code generated by coralogix-iac-codegen v0\.1\.0-beta\.10' .
```

During beta, increment the beta identifier for each generator behavior change. After beta, use a patch version for fixes, a minor version for new supported behavior, and a major version for incompatible output changes.

A behavior change is a change of what the generator produces, whatever caused it: a change here, or a new version of the `iac` module. A refactor that produces the same output needs no new version. CI enforces this on every pull request that changes this module (`scripts/check-version-change.sh`). It builds `tfgen` at the base commit and at the pull request, runs both on the same input, and fails when their output differs and the version did not change. The input is every resource that `internal/provider/codegen.go` lists, the `check` report of every component of the pinned spec, and the golden files of the generator tests.

`internal/provider/codegen.go` lists every generated resource with the name that its acceptance file uses. Add a line when you generate a new resource. `go generate ./internal/provider/` regenerates them all.

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
