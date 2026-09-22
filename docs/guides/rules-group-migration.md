---
page_title: "Migrate coralogix_rules_group to coralogix_parsing_rules"
---

# Migrate `coralogix_rules_group` to `coralogix_parsing_rules`

The `coralogix_rules_group` resource and data source are deprecated. A future major provider version will remove them. Use the `coralogix_parsing_rules` resource and data source instead.

Both resources manage the same Coralogix parsing rule group. They use different Terraform schemas. You must rewrite the resource HCL and move each existing rule group to the new Terraform state address. You must also replace each deprecated data source and update expressions that use its changed output shape.

Perform this migration before you upgrade to the major provider version that removes `coralogix_rules_group`.

## Before you start

1. Use a provider version that contains both resource types.
2. Run `terraform plan`. Resolve all existing changes first.
3. Back up the state. The state can contain sensitive values. Store the backup securely and do not commit it.

```shell
terraform state pull > terraform.tfstate.backup
```

The API key needs `parsing-rules:ReadConfig` to import and read a rule group. It needs `parsing-rules:UpdateConfig` to update or delete one. The OpenAPI resource does not make an extra lookup in another service.

## Rewrite the HCL

The following example shows the main schema changes.

Before:

```hcl
resource "coralogix_rules_group" "example" {
  name       = "Application rules"
  severities = ["Warning", "Error"]

  rule_subgroups {
    order = 1

    rules {
      block {
        order                        = 1
        name                         = "Block health checks"
        source_field                 = "text"
        regular_expression           = "/health"
        blocking_all_matching_blocks = true
      }
    }

    rules {
      extract_timestamp {
        order                 = 2
        name                  = "Read timestamp"
        source_field          = "text"
        field_format_standard = "Strftime"
        time_format           = "%Y-%m-%dT%H:%M:%S%z"
      }
    }
  }
}
```

After:

```hcl
resource "coralogix_parsing_rules" "example" {
  name       = "Application rules"
  severities = ["warning", "error"]

  rule_subgroups = [{
    rules = [
      {
        block = {
          name                      = "Block health checks"
          source_field              = "text"
          regular_expression        = "/health"
          block_all_matching_blocks = true
        }
      },
      {
        extract_timestamp = {
          name                  = "Read timestamp"
          source_field          = "text"
          field_format_standard = "strftime"
          time_format           = "%Y-%m-%dT%H:%M:%S%z"
        }
      }
    ]
  }]
}
```

Apply these changes to each resource:

| Old schema | New schema |
|---|---|
| `coralogix_rules_group` | `coralogix_parsing_rules` |
| `rule_subgroups { ... }` | `rule_subgroups = [{ ... }]` |
| `rules { ... }` | `rules = [{ ... }]` |
| A rule block such as `parse { ... }` | An object attribute such as `parse = { ... }` |
| `blocking_all_matching_blocks` | `block_all_matching_blocks` |
| Configured subgroup and rule `order` values | List position. The provider computes nested `order` values. |
| A custom `timeouts` block | No equivalent. Remove the block. |

If the old HCL sets nested `order` values, sort the subgroups and rules by those values before you remove them. The first list item has order `1`. This preserves rule evaluation order.

The top-level `order` attribute is still configurable. It controls the position of the rule group among all rule groups.

### Complete resource attribute mapping

The following tables cover every resource attribute. “Unchanged” means that the attribute keeps the same name and value type. It still moves into the new list/object syntax when it is nested.

Top-level attributes:

| Old attribute | New attribute | Required action |
|---|---|---|
| `id` | `id` | Computed in both resources. Do not set it in HCL. |
| `name` | `name` | Unchanged. Required. |
| `description` | `description` | Unchanged. Optional. |
| `active` | `active` | Unchanged. Optional. |
| `applications` | `applications` | Unchanged set of strings. Optional. |
| `subsystems` | `subsystems` | Unchanged set of strings. Optional. |
| `severities` | `severities` | Keep the attribute. Change every value to lowercase as shown below. |
| `hidden` | `hidden` | Unchanged. Optional. |
| `creator` | `creator` | Unchanged. Optional. |
| `order` | `order` | Unchanged. Optional and computed. This remains the rule-group order. |
| `rule_subgroups` | `rule_subgroups` | Change block syntax to a list of objects. |
| `timeouts.create`, `timeouts.read`, `timeouts.update`, `timeouts.delete` | No equivalent | Remove the complete `timeouts` block. |

Each `rule_subgroups` element:

| Old attribute | New attribute | Required action |
|---|---|---|
| `id` | `id` | Computed in both resources. Do not set it. |
| `active` | `active` | Unchanged. Optional. |
| `order` | `order` | Changes from configurable to computed. Sort the list by the old values, then remove this attribute. |
| `rules` | `rules` | Change block syntax to a list of objects. Keep one rule type in each list element. |

Every rule type has these common attributes:

| Old attribute | New attribute | Required action |
|---|---|---|
| `id` | `id` | Computed in both resources. Do not set it. |
| `name` | `name` | Unchanged. Required. |
| `description` | `description` | Unchanged. Optional. |
| `active` | `active` | Unchanged. Optional. |
| `order` | `order` | Changes from configurable to computed. Sort the `rules` list by the old values, then remove this attribute. |

The new schema requires exactly one rule type in each `rules` element. Keep `parse`, `block`, `json_extract`, `replace`, `extract_timestamp`, `remove_fields`, `json_stringify`, `extract`, or `parse_json_field`. Put two rule types in two separate list elements.

Rule-specific attributes:

| Rule type | Attributes | Required action |
|---|---|---|
| `parse` | `source_field`, `destination_field`, `regular_expression` | All unchanged and required. |
| `block` | `source_field`, `regular_expression`, `keep_blocked_logs` | Unchanged. `source_field` and `regular_expression` are required. `keep_blocked_logs` is optional. |
| `block` | `blocking_all_matching_blocks` | Rename to `block_all_matching_blocks`. It remains optional and defaults to `true`. |
| `json_extract` | `destination_field`, `json_key`, `destination_field_text` | Keep all attributes. `destination_field` and `json_key` are required. `destination_field_text` remains optional and is used with the `text` destination. Change `destination_field` to its canonical value as shown below. |
| `replace` | `source_field`, `destination_field`, `regular_expression`, `replacement_string` | Names are unchanged. The first three are required. `replacement_string` remains optional. |
| `extract_timestamp` | `source_field`, `field_format_standard`, `time_format` | Keep all attributes. All are required. Change `field_format_standard` as shown below. |
| `remove_fields` | `excluded_fields` | Unchanged. Required and must contain at least one string. |
| `json_stringify` | `source_field`, `destination_field`, `keep_source_field` | Names are unchanged. The first two are required. `keep_source_field` remains optional and defaults to `false`. |
| `extract` | `source_field`, `regular_expression` | Both unchanged and required. |
| `parse_json_field` | `source_field`, `destination_field`, `keep_source_field`, `keep_destination_field` | Names are unchanged. The first two are required. `keep_source_field` remains optional and defaults to `false`. `keep_destination_field` remains optional and defaults to `true`. |

### Optional and computed attributes

The OpenAPI resource reads more omitted values from the API. These attributes change from optional to optional and computed:

- Top level: `description`, `active`, `applications`, `subsystems`, `severities`, `hidden`, and `creator`.
- Subgroup: `active`.
- Every rule type: `description` and `active`.
- `block.block_all_matching_blocks`, `json_stringify.keep_source_field`, `parse_json_field.keep_source_field`, and `parse_json_field.keep_destination_field`.

You can keep an explicit value or continue to omit these attributes. When you omit one, the API value is stored in state.

Two old provider-side defaults are no longer schema defaults. `block.keep_blocked_logs` no longer has a provider-side `false` default. `replace.replacement_string` no longer has a provider-side empty-string default. Keep an explicit value when the old HCL set one. If the old HCL omitted either attribute, set `keep_blocked_logs = false` or `replacement_string = ""` in the new HCL. This preserves the old request value and avoids depending on an API default.

The resource output shape also changes for rule types. An old rule type was a list containing zero or one object. A new rule type is a nullable object. Remove the `[0]` index after `parse`, `block`, or another rule-type name in expressions that read the resource.

### Change enum values

Severity values are lowercase in the new resource:

| Old value | New value |
|---|---|
| `Debug` | `debug` |
| `Verbose` | `verbose` |
| `Info` | `info` |
| `Warning` | `warning` |
| `Error` | `error` |
| `Critical` | `critical` |

The `extract_timestamp.field_format_standard` values also change:

| Old value | New value |
|---|---|
| `Strftime` | `strftime` |
| `JavaSDF` | `javaSDF` |
| `Golang` | `golang` |
| `SecondTS` | `secondTS` |
| `MilliTS` | `milliTS` |
| `MicroTS` | `microTS` |
| `NanoTS` | `nanoTS` |

The new resource accepts `json_extract.destination_field` values without regard to case. However, an import reads the canonical value from the API. Use the canonical value in HCL to prevent a plan diff:

| Old value | Canonical new value |
|---|---|
| `Category` | `category` |
| `Class` | `class` |
| `Method` | `method` |
| `ThreadID` | `threadID` |
| `Severity` | `severity` |
| `Text` | `text` |

## Move the resource in state

Do not use `terraform state mv`. The two resource types have different state schemas, and the provider does not define a direct state conversion.

For each rule group:

1. Get its current ID.

```shell
terraform state show 'coralogix_rules_group.example'
```

2. Remove only the old Terraform state address. This command does not delete the rule group in Coralogix.

```shell
terraform state rm 'coralogix_rules_group.example'
```

3. Import the same ID at the new address.

```shell
terraform import 'coralogix_parsing_rules.example' '<rule-group-id>'
```

Use the full address for a resource in a module or a resource that uses `count` or `for_each`. Keep the address in single quotes. For example:

```shell
terraform state rm 'module.logs.coralogix_rules_group.example["api"]'
terraform import 'module.logs.coralogix_parsing_rules.example["api"]' '<rule-group-id>'
```

If the import fails, the remote rule group is unchanged. Fix the error and run the import again. Use the state backup only if you must restore the old state address.

## Verify the migration

Run:

```shell
terraform plan
```

The plan must show no create, update, or delete operation for the migrated rule group. Do not apply a plan that replaces the rule group. Correct the HCL until the plan has no changes.

## Migrate the data source

The deprecated and new data sources both accept only `id` as input. All other attributes are read-only. Change the data source type and any resource reference used to supply the ID.

Before:

```hcl
data "coralogix_rules_group" "example" {
  id = coralogix_rules_group.example.id
}
```

After:

```hcl
data "coralogix_parsing_rules" "example" {
  id = coralogix_parsing_rules.example.id
}
```

Terraform reads a data source again during the next plan or apply. Do not run `terraform state rm` or `terraform import` for a data source. Run `terraform plan`, review any output value changes, and then run `terraform apply` to replace the data source in state.

### Complete data-source output mapping

The following top-level outputs keep the same names and value types:

| Output | Migration action |
|---|---|
| `id`, `name`, `description`, `creator` | No expression change, except for the data-source type in the address. |
| `active`, `hidden`, `order` | No expression change, except for the data-source type in the address. |
| `applications`, `subsystems` | No expression change, except for the data-source type in the address. |
| `severities` | Values change to the lowercase forms in the severity mapping above. Update exact string comparisons. |
| `rule_subgroups` | Remains a list. Its nested output shape changes as described below. |

Within `rule_subgroups`, `id`, `active`, `order`, and `rules` keep their names. `rules` remains a list. Each rule type changes from a list containing zero or one object to a nullable object. Remove the `[0]` index after the rule type in downstream expressions.

Before:

```hcl
output "first_rule_name" {
  value = data.coralogix_rules_group.example.rule_subgroups[0].rules[0].parse[0].name
}
```

After:

```hcl
output "first_rule_name" {
  value = data.coralogix_parsing_rules.example.rule_subgroups[0].rules[0].parse.name
}
```

All nine rule-type outputs are present in the new data source: `parse`, `block`, `json_extract`, `replace`, `extract_timestamp`, `remove_fields`, `json_stringify`, `extract`, and `parse_json_field`. Their leaf attributes follow the complete rule-specific mapping above. In addition:

- `block.blocking_all_matching_blocks` becomes `block.block_all_matching_blocks`.
- `extract_timestamp.field_format_standard` returns the new canonical value.
- `json_extract.destination_field` returns the new canonical value.
- Common `id`, `name`, `description`, `active`, and `order` outputs keep their names.

## Final verification

Confirm that the state has no old resource or data-source addresses:

```shell
terraform state list | grep 'coralogix_rules_group'
```

The command must return no output. Search the Terraform configuration for `coralogix_rules_group` and remove any remaining references. You can then upgrade to the major provider version that removes it.
