---

# generated by https://github.com/hashicorp/terraform-plugin-docs

page_title: "coralogix_slo Resource - terraform-provider-coralogix"
subcategory: ""
description: |-
Coralogix SLO.
---

# coralogix_slo (Resource)

Coralogix SLO.

## Example Usage

```hcl
resource "coralogix_slo" "example" {
  name                   = "example"
  period                 = "30_days"
  service_name           = "example"
  target_percentage      = 99.9
  type                   = "error"
  description            = "example"
  name                   = "coralogix_slo_example"
  service_name           = "service_name"
  description            = "description"
  target_percentage      = 30
  type                   = "latency"
  threshold_microseconds = 1000000
  threshold_symbol_type  = "greater"
  period                 = "7_days"
  filters                = [
    {
      field        = "severity"
      compare_type = "is"
      field_values = ["error", "warning"]
    },
  ]
  threshold_microseconds = 1000000
  threshold_symbol_type  = "greater"
}
```

<!-- schema generated by tfplugindocs -->

## Schema

### Required

- `name` (String) SLO name.
- `period` (String) Period. This is the period of the SLO. Valid values
  are: ["30_days" "unspecified" "7_days" "14_days"]
- `service_name` (String) Service name. This is the name of the service that the SLO is associated with.
- `target_percentage` (Number) Target percentage. This is the target percentage of the SLO.
- `type` (String) Type. This is the type of the SLO. Valid values are: "error", "latency".

### Optional

- `description` (String) Optional SLO description.
- `filters` (Attributes Set) (see [below for nested schema](#nestedatt--filters))
- `threshold_microseconds` (Number) Threshold in microseconds. Required when `type` is `latency`.
- `threshold_symbol_type` (String) Threshold symbol type. Required when `type` is `latency`. Valid values
  are: ["greater" "greater_or_equal" "less" "equal"]

### Read-Only

- `id` (String) SLO ID.
- `remaining_error_budget_percentage` (Number)
- `status` (String)

<a id="nestedatt--filters"></a>

### Nested Schema for `filters`

Required:

- `compare_type` (String) Compare type. This is the compare type of the SLO. Valid values
  are: ["starts_with" "ends_with" "includes" "unspecified" "is"]
- `field` (String)

Optional:

- `field_values` (Set of String)