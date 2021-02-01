---
subcategory: "Rules"
page_title: "Coralogix: coralogix_rule"
---

# coralogix_rule

Use this data source to retrieve information about a Coralogix Rule.

## Example Usage

```hcl
data "coralogix_rule" "rule" {
    rule_id        = "e1a31d75-36ab-11e8-af8f-02420a00070c"
    rules_group_id = "e10ef9d1-36ab-11e8-af8f-02420a00070c"
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

---

Each `rule_matcher` block exports the following:

* `field` - Rule Matcher field.
* `constraint` - Rule Matcher constraint.