---
subcategory: "Rules"
page_title: "Coralogix: coralogix_rules_group"
---

# coralogix_rules_group

Provides the Coralogix Rules Group resource. This allows Rules Group to be created, updated, and deleted.

## Example Usage

```hcl
# Create "My Group" Rules Group
resource "coralogix_rules_group" "rules_group" {
    name    = "My Group"
    enabled = true
}
```

## Argument Reference

* `name` - (Required) Rules Group name.
* `enabled` - (Optional) Rules Group state.

## Attribute Reference

* `order` - Rules Group order number.

## Import

Rules Groups can be imported using their ID.

```
$ terraform import coralogix_rules_group.rules_group e10ef9d1-36ab-11e8-af8f-02420a00070c
```