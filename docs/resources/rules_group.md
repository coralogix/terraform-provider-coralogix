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
* `description` - (Optional) Rules Group description.
* `creator` - (Optional) Rules Group creator.
* `rule_matcher` - (Optional) A `rule_matcher` block as documented below, more than one can be configured.

---

Each `rule_matcher` block should contains the following:

* `field` - (Required) The rule matcher key, must be one of the following: `severity`, `applicationName`, `subsystemName`.
* `constraint` - (Required) The value of the constraint. for key 'severity' the allowed values are: `debug`, `verbose`, `info`, `warning`, `error`, `critical`

## Attribute Reference

* `order` - Rules Group order number.
* `created_at` - Rules Group creation date.
* `updated_at` - Rules Group last update date.
* `rules` - The rules inside the rule group. can access different rule groups with syntax : .rules.0.group , .rules.1.group.

## Import

Rules Groups can be imported using their ID.

First create a new rules_group block:

```hcl
resource "coralogix_rules_group" "my_rules_group" {
}
```

And then import it:

```
$ terraform import coralogix_rules_group.rules_group <rules_group_id>
```

After that go to your .tfstate file and implement the data for your rules_group inside the resource block.

the id can be retrieved from the API with a GET request to all rules group or using the UI,
for more information regarding the API - https://coralogix.com/docs/rules-api/