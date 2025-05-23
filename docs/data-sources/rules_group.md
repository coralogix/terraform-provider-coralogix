---
# generated by https://github.com/hashicorp/terraform-plugin-docs
page_title: "coralogix_rules_group Data Source - terraform-provider-coralogix"
subcategory: ""
description: |-
  
---

# coralogix_rules_group (Data Source)



## Example Usage

```terraform
data "coralogix_rules_group" "imported_rules_group_example" {
  id = coralogix_rules_group.rules_group_example.id
}
```

<!-- schema generated by tfplugindocs -->
## Schema

### Read-Only

- `active` (Boolean) Determines whether the rule-group will be active.
- `applications` (Set of String) Rules will execute on logs that match the following applications.
- `creator` (String) Rule-group creator.
- `description` (String) Rule-group description
- `hidden` (Boolean)
- `id` (String) The ID of this resource.
- `name` (String) Rule-group name
- `order` (Number) Determines the index of the rule-group between the other rule-groups. By default, will be added last. (1 based indexing).
- `rule_subgroups` (List of Object) List of rule-subgroups. Every rule-subgroup is a list of rules linked with a logical 'OR' (||) operation. (see [below for nested schema](#nestedatt--rule_subgroups))
- `severities` (Set of String) Rules will execute on logs that match the these severities. Can be one of ["Critical" "Debug" "Error" "Info" "Verbose" "Warning"]
- `subsystems` (Set of String) Rules will execute on logs that match the following subsystems.

<a id="nestedatt--rule_subgroups"></a>
### Nested Schema for `rule_subgroups`

Read-Only:

- `active` (Boolean)
- `id` (String)
- `order` (Number)
- `rules` (List of Object) (see [below for nested schema](#nestedobjatt--rule_subgroups--rules))

<a id="nestedobjatt--rule_subgroups--rules"></a>
### Nested Schema for `rule_subgroups.rules`

Read-Only:

- `block` (List of Object) (see [below for nested schema](#nestedobjatt--rule_subgroups--rules--block))
- `extract` (List of Object) (see [below for nested schema](#nestedobjatt--rule_subgroups--rules--extract))
- `extract_timestamp` (List of Object) (see [below for nested schema](#nestedobjatt--rule_subgroups--rules--extract_timestamp))
- `json_extract` (List of Object) (see [below for nested schema](#nestedobjatt--rule_subgroups--rules--json_extract))
- `json_stringify` (List of Object) (see [below for nested schema](#nestedobjatt--rule_subgroups--rules--json_stringify))
- `parse` (List of Object) (see [below for nested schema](#nestedobjatt--rule_subgroups--rules--parse))
- `parse_json_field` (List of Object) (see [below for nested schema](#nestedobjatt--rule_subgroups--rules--parse_json_field))
- `remove_fields` (List of Object) (see [below for nested schema](#nestedobjatt--rule_subgroups--rules--remove_fields))
- `replace` (List of Object) (see [below for nested schema](#nestedobjatt--rule_subgroups--rules--replace))

<a id="nestedobjatt--rule_subgroups--rules--block"></a>
### Nested Schema for `rule_subgroups.rules.block`

Read-Only:

- `active` (Boolean)
- `blocking_all_matching_blocks` (Boolean)
- `description` (String)
- `id` (String)
- `keep_blocked_logs` (Boolean)
- `name` (String)
- `order` (Number)
- `regular_expression` (String)
- `source_field` (String)


<a id="nestedobjatt--rule_subgroups--rules--extract"></a>
### Nested Schema for `rule_subgroups.rules.extract`

Read-Only:

- `active` (Boolean)
- `description` (String)
- `id` (String)
- `name` (String)
- `order` (Number)
- `regular_expression` (String)
- `source_field` (String)


<a id="nestedobjatt--rule_subgroups--rules--extract_timestamp"></a>
### Nested Schema for `rule_subgroups.rules.extract_timestamp`

Read-Only:

- `active` (Boolean)
- `description` (String)
- `field_format_standard` (String)
- `id` (String)
- `name` (String)
- `order` (Number)
- `source_field` (String)
- `time_format` (String)


<a id="nestedobjatt--rule_subgroups--rules--json_extract"></a>
### Nested Schema for `rule_subgroups.rules.json_extract`

Read-Only:

- `active` (Boolean)
- `description` (String)
- `destination_field` (String)
- `destination_field_text` (String)
- `id` (String)
- `json_key` (String)
- `name` (String)
- `order` (Number)


<a id="nestedobjatt--rule_subgroups--rules--json_stringify"></a>
### Nested Schema for `rule_subgroups.rules.json_stringify`

Read-Only:

- `active` (Boolean)
- `description` (String)
- `destination_field` (String)
- `id` (String)
- `keep_source_field` (Boolean)
- `name` (String)
- `order` (Number)
- `source_field` (String)


<a id="nestedobjatt--rule_subgroups--rules--parse"></a>
### Nested Schema for `rule_subgroups.rules.parse`

Read-Only:

- `active` (Boolean)
- `description` (String)
- `destination_field` (String)
- `id` (String)
- `name` (String)
- `order` (Number)
- `regular_expression` (String)
- `source_field` (String)


<a id="nestedobjatt--rule_subgroups--rules--parse_json_field"></a>
### Nested Schema for `rule_subgroups.rules.parse_json_field`

Read-Only:

- `active` (Boolean)
- `description` (String)
- `destination_field` (String)
- `id` (String)
- `keep_destination_field` (Boolean)
- `keep_source_field` (Boolean)
- `name` (String)
- `order` (Number)
- `source_field` (String)


<a id="nestedobjatt--rule_subgroups--rules--remove_fields"></a>
### Nested Schema for `rule_subgroups.rules.remove_fields`

Read-Only:

- `active` (Boolean)
- `description` (String)
- `excluded_fields` (List of String)
- `id` (String)
- `name` (String)
- `order` (Number)


<a id="nestedobjatt--rule_subgroups--rules--replace"></a>
### Nested Schema for `rule_subgroups.rules.replace`

Read-Only:

- `active` (Boolean)
- `description` (String)
- `destination_field` (String)
- `id` (String)
- `name` (String)
- `order` (Number)
- `regular_expression` (String)
- `replacement_string` (String)
- `source_field` (String)
