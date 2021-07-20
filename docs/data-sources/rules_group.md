---
subcategory: "Rules"
page_title: "Coralogix: coralogix_rules_group"
---

# coralogix_rules_group

Use this data source to retrieve information about a Coralogix Rules Group.

## Example Usage

```hcl
data "coralogix_rules_group" "rules_group" {
    rules_group_id = "e10ef9d1-36ab-11e8-af8f-02420a00070c"
}
```

## Argument Reference

* `rules_group_id` - (Required) Rules Group ID.

## Attribute Reference

* `name` - Rules Group name.
* `order` - Rules Group order number.
* `enabled` - Rules Group state.
* `description` - Rules Group description.
* `creator` - Rules Group creator.
* `rules` - Rules Group rules list. 