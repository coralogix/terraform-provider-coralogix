---
subcategory: "Rules"
page_title: "Coralogix: coralogix_rule"
---

# coralogix_rule

Use this data source to retrieve information about a Coralogix Rule.

## Example Usage
Once a rule group is created it can be refered to in your config.
This example shows how to use the data structure
```hcl
data "coralogix_rule" "my_rule" {
    rule_id        = "e1a31d75-36ab-11e8-af8f-02420a00070c"
    rules_group_id = "7894afca-66d7-ae7e-24da-b7031c38169b"
}
```

Using a code example like this will output the name of the rule:
```hcl
output "name" {
  value       = data.coralogix_rule.my_rule.name
  description = "Rule name."
}
```

## Argument Reference

* `rule_id` - (Required) Rule ID.
* `rules_group_id` - (Required) Rules Group ID.

## Attribute Reference

* `name` - Rule name.
* `type` - Rule type.
* `description` - Rule description.
* `order` - Rule order number.
* `enabled` - Rule state.
* `rule_matcher` - A `rule_matcher` block as documented below.
* `expression` - Rule expression.
* `source_field` - Rule source field.
* `destination_field` - Rule destination field.
* `replace_value` - Rule replace value.
* `keep_blocked_logs` - Indicate if the rule keep the blocked logs in the archive and LiveTail.
* `delete_source` - Indicate if the rule delete the source field.
* `overwrite_destination` - Indicate if the rule overwrite the destination field.
* `escaped_value` - Indicate if the value is escaped.

---

Each `rule_matcher` block exports the following:

* `field` - Rule Matcher field.
* `constraint` - Rule Matcher constraint.