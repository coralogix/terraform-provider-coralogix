---
# generated by https://github.com/hashicorp/terraform-plugin-docs
page_title: "coralogix_group Data Source - terraform-provider-coralogix"
subcategory: ""
description: |-
  Coralogix group.
---

# coralogix_group (Data Source)

Coralogix group.



<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `id` (String) Group ID.

### Read-Only

- `display_name` (String) Group display name.
- `members` (Set of String)
- `role` (String)
- `scope_id` (String) Scope attached to the group.
