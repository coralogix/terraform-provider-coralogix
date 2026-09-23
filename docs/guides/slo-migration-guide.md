---
page_title: "Migrate coralogix_slo to coralogix_slo_v2"
---

# Migrate `coralogix_slo` to `coralogix_slo_v2`

The `coralogix_slo` resource and data source are deprecated. A future major provider version will remove them. Use `coralogix_slo_v2` instead.

Both resources can define an APM SLO, but they use separate APIs and separate remote objects. A legacy SLO ID does not exist in the V2 API. The migration creates a new SLO with a new ID. It then deletes the legacy SLO after verification.

The new SLO does not keep the legacy SLO's history, status, or current error budget. Some old values also have no direct equivalent. Review every SLO before you create its replacement.

Perform this migration before you upgrade to the major provider version that removes `coralogix_slo`.

## Before you start

1. Use a provider version that contains both resource types and supports `sli.apm_sli` in `coralogix_slo_v2`.
2. Run `terraform plan`. Resolve all existing changes first.
3. Back up the state. The state can contain sensitive values. Store the backup securely and do not commit it.

```shell
terraform state pull > terraform.tfstate.backup
```

Use an API key with the SLO permission preset. The key needs read, create, and delete access.

## Check whether the SLO can migrate without a behavior change

The following old values do not have a direct equivalent in `coralogix_slo_v2`:

- `period = "30_days"`. The new resource has no 30-day value. Its named periods are 7, 14, 21, or 28 days.
- `filters[*].compare_type` values other than `is`. The new APM filter has no comparison operator.
- A filter with no `field_values`, an empty `field_values` set, or a field name that the new API does not return unchanged. The new filter requires at least one value and uses an API label key.
- `threshold_symbol_type`. The new APM latency SLI treats latency at or below the threshold as good.
- The new latency `time_window` and aggregation method. The old resource has neither setting.

Do not migrate an affected SLO until you choose and approve the new behavior. The migration creates a new remote SLO, even when all mapped values stay the same.

Do not use `data.coralogix_slo_v2` with a legacy ID. The V2 API returns not found for that ID. Inspect the legacy resource state and the SLO in the Coralogix UI instead:

```shell
terraform state show 'coralogix_slo.example'
```

Record every configured value. For latency SLOs, also record the approved V2 time window and aggregation method.

## Rewrite an error SLO

Before:

```hcl
resource "coralogix_slo" "example" {
  name              = "Checkout_errors"
  service_name      = "checkout"
  description       = "Successful checkout requests"
  target_percentage = 99
  type              = "error"
  period            = "7_days"

  filters = [{
    field        = "http.status_code"
    compare_type = "is"
    field_values = ["500", "503"]
  }]
}
```

The filter conversion in this example is valid only when the old comparison is `is`, the value set is not empty, and the V2 API accepts the field as a label key. Confirm the label key in the APM data before migration. If V2 cannot represent the old comparison, stop the migration.

After:

```hcl
resource "coralogix_slo_v2" "example" {
  name                        = "Checkout_errors"
  description                 = "Successful checkout requests"
  target_threshold_percentage = 99
  product_type                = "apm"

  sli = {
    apm_sli = {
      services     = ["checkout"]
      error_config = {}

      filters = [{
        key    = "http.status_code"
        values = ["500", "503"]
      }]
    }
  }

  window = {
    slo_time_frame = "7_days"
  }
}
```

## Rewrite a latency SLO

Before:

```hcl
resource "coralogix_slo" "example" {
  name                   = "Checkout_latency"
  service_name           = "checkout"
  description            = "Checkout request latency"
  target_percentage      = 99
  type                   = "latency"
  threshold_microseconds = 500000
  threshold_symbol_type  = "less_or_equal"
  period                 = "14_days"

  filters = [{
    field        = "severity"
    compare_type = "is"
    field_values = ["error", "warning"]
  }]
}
```

After, when a five-minute P95 calculation is the approved replacement:

```hcl
resource "coralogix_slo_v2" "example" {
  name                        = "Checkout_latency"
  description                 = "Checkout request latency"
  target_threshold_percentage = 99
  product_type                = "apm"

  sli = {
    apm_sli = {
      services = ["checkout"]

      filters = [{
        key    = "severity"
        values = ["error", "warning"]
      }]

      latency_config = {
        time_window = "5_minutes"
        threshold   = 500

        quantile = {
          percentile = 0.95
        }
      }
    }
  }

  window = {
    slo_time_frame = "14_days"
  }
}
```

Convert the latency threshold from microseconds to milliseconds by dividing it by 1,000. The example changes `500000` microseconds to `500` milliseconds.

The after example defines a new P95 calculation. It is not an automatic translation of `threshold_symbol_type`.

Do not copy the example's `time_window` or `quantile` values without review. Choose `1_minute` or `5_minutes`. Then choose exactly one of `quantile` or `average`. These choices have no source value in `coralogix_slo`.

## Complete resource attribute mapping

The following table covers every old resource attribute.

| Old attribute | New attribute | Required action |
|---|---|---|
| `description` | `description` | Unchanged. |
| `filters` | `sli.apm_sli.filters` | Change the set of objects to a list of objects. |
| `filters[*].compare_type` | No attribute for `is`; no equivalent for other values | Use the enum mapping below. |
| `filters[*].field` | `sli.apm_sli.filters[*].key` | Rename the attribute only when V2 accepts the field as a label key. Otherwise, choose and review a valid V2 label key. |
| `filters[*].field_values` | `sli.apm_sli.filters[*].values` | Rename the attribute and change the set to a list. The new list requires at least one value. An old null or empty set has no direct equivalent. |
| `id` | `id` | The replacement gets a new ID. Do not set it in HCL and do not import the legacy ID. |
| `name` | `name` | Unchanged. |
| `period` | `window.slo_time_frame` | Move the value into the `window` object. Use the enum mapping below. |
| `remaining_error_budget_percentage` | No equivalent | Remove downstream references. This value was computed. |
| `service_name` | `sli.apm_sli.services` | Put the old value in a one-element list. The new API currently accepts one service. |
| `status` | No equivalent | Remove downstream references. This value was computed. |
| `target_percentage` | `target_threshold_percentage` | Rename the attribute. The old integer value remains the same numeric value. The new attribute also accepts decimal values. |
| `threshold_microseconds` | `sli.apm_sli.latency_config.threshold` | Divide the value by 1,000 to convert microseconds to milliseconds. |
| `threshold_symbol_type` | No configurable equivalent | Use the enum mapping below. The new APM latency SLI counts latency at or below the threshold as good. |
| `type` | `sli.apm_sli.error_config` or `sli.apm_sli.latency_config` | Set `product_type = "apm"`. For `error`, set `error_config = {}`. For `latency`, configure `latency_config` and make the required new choices below. |

### Enum mapping

The old `filters[*].compare_type` accepts these values:

| Old value | New value | Required action |
|---|---|---|
| `is` | No operator attribute | Remove the operator only when the V2 label filter has the same exact-match behavior. |
| `starts_with` | No equivalent | Stop and redesign the filter. |
| `ends_with` | No equivalent | Stop and redesign the filter. |
| `includes` | No equivalent | Stop and redesign the filter. |

The old `threshold_symbol_type` accepts these values:

| Old value | New value | Required action |
|---|---|---|
| `greater` | No operator attribute | Review the behavior. The new APM latency SLI defines good latency as at or below the threshold. |
| `greater_or_equal` | No operator attribute | Review the behavior. The new APM latency SLI defines good latency as at or below the threshold. |
| `less` | No operator attribute | Review the behavior. The new APM latency SLI defines good latency as at or below the threshold. |
| `less_or_equal` | No operator attribute | Review the boundary behavior before you remove the operator. |
| `equal` | No operator attribute | Stop and redesign the latency SLI. |

The legacy API also defines `not_equal`, but the old Terraform schema does not accept it. It is not a migratable old HCL value.

The old `period` accepts these values:

| Old value | New value | Required action |
|---|---|---|
| `7_days` | `7_days` | Keep the value in `window.slo_time_frame`. |
| `14_days` | `14_days` | Keep the value in `window.slo_time_frame`. |
| `30_days` | No exact equivalent | Choose and approve a supported value. The closest duration is `28_days`, but it changes behavior. |

The new resource also has attributes with no old input: `labels`, `ownership_tags`, and `sli.apm_sli.grouping_keys`. Omit them unless you intend to add new behavior. The backend computes `grouping` and `apm_sli_metadata`.

The required `sli` object contains exactly one of `apm_sli`, `request_based_metric_sli`, or `window_based_metric_sli`. Use `apm_sli` for the direct migration described in this guide.

For a latency SLO, these new attributes also have no old input:

| New attribute | Required action |
|---|---|
| `sli.apm_sli.latency_config.time_window` | Choose `1_minute` or `5_minutes`. |
| `sli.apm_sli.latency_config.quantile.percentile` | Set a fraction such as `0.95` when you choose percentile aggregation. |
| `sli.apm_sli.latency_config.average` | Set the empty object `{}` when you choose mean aggregation. |

## Create and cut over to the V2 resource

Do not use `terraform state mv`, `terraform state rm`, or `terraform import` for this migration. A legacy ID is not valid in the V2 API.

For each SLO:

1. Keep the old resource block. Add the converted V2 resource block next to it. The two blocks can use the same Terraform label because their resource types differ.
2. Run `terraform plan`. The plan must contain one V2 create operation and no change to the legacy SLO.
3. Run `terraform apply`. Record the new V2 ID. Both SLOs now exist during a short verification period.
4. Verify the V2 SLO. Then update all Terraform references and data sources to use the V2 resource.
5. Remove the legacy resource block. Run `terraform plan`. The plan must contain one delete operation for the legacy SLO and no change to the V2 SLO. Apply that plan.

If V2 creation or verification fails, remove the V2 block and apply again. The legacy SLO remains managed and unchanged. Do not delete the legacy SLO until V2 verification succeeds.

## Verify the resource migration

Run:

```shell
terraform plan
```

After the cutover, the plan must show no create, update, replace, or delete operation for either SLO address. Do not apply an unexpected update.

Check the new SLO in the Coralogix UI. Confirm the service, type, filters, target, time frame, latency aggregation, and latency threshold. Confirm that consumers use the new ID. The current error budget and history start again for the new SLO.

## Migrate the data source

The V2 data source cannot read a legacy ID. First create the V2 replacement. Then change the data-source type and use the new V2 ID.

Before:

```hcl
data "coralogix_slo" "example" {
  id = coralogix_slo.example.id
}
```

After:

```hcl
data "coralogix_slo_v2" "example" {
  id = coralogix_slo_v2.example.id
}
```

Terraform reads a data source again during the next plan or apply. Do not run `terraform state rm` or `terraform import` for a data source. Run `terraform plan`, review output changes, and then run `terraform apply` to replace the data source in state.

### Complete data-source output mapping

| Old output | New output | Required action |
|---|---|---|
| `description` | `description` | Read the value from the new V2 SLO. |
| `filters` | `sli.apm_sli.filters` | Change the expression path. The collection changes from a set to a list. |
| `filters[*].compare_type` | No equivalent | Remove or replace the reference. |
| `filters[*].field` | `sli.apm_sli.filters[*].key` | Change the expression path. |
| `filters[*].field_values` | `sli.apm_sli.filters[*].values` | Change the expression path. The collection changes from a set to a list. |
| `id` | `id` | Use the new V2 ID. The legacy ID is not valid in the V2 API. |
| `name` | `name` | Read the value from the new V2 SLO. |
| `period` | `window.slo_time_frame` | Change the expression path. The value can change when the old period was 30 days. |
| `remaining_error_budget_percentage` | No equivalent | Remove or replace the reference. |
| `service_name` | `sli.apm_sli.services[0]` | Change the expression path. |
| `status` | No equivalent | Remove or replace the reference. |
| `target_percentage` | `target_threshold_percentage` | Change the expression path. |
| `threshold_microseconds` | `sli.apm_sli.latency_config.threshold` | Change the expression path. The unit changes from microseconds to milliseconds. |
| `threshold_symbol_type` | No equivalent | Remove or replace the reference. |
| `type` | `sli.apm_sli.error_config` or `sli.apm_sli.latency_config` | Test which nullable object is present. |

The new data source also returns `labels`, `grouping`, `product_type`, `ownership_tags`, and `apm_sli_metadata`. It returns the new latency time window and aggregation fields. These outputs had no old equivalent.

## Final verification

Confirm that the state has no old resource or data-source addresses:

```shell
terraform state list | grep 'coralogix_slo\.'
```

The command must return no output. Search the Terraform configuration for old type references:

```shell
grep -R -E 'coralogix_slo([^_[:alnum:]]|$)' --include='*.tf' .
```

The command must return no old type declaration or reference. The pattern excludes `coralogix_slo_v2`. Run one final plan. It must show no unapproved changes. You can then upgrade to the major provider version that removes `coralogix_slo`.
