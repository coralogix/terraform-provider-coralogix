---
page_title: "Migrate enrichments to coralogix_data_enrichments"
---

# Migrate enrichments to `coralogix_data_enrichments`

The `coralogix_enrichment` resource and data source are deprecated. The `coralogix_data_set` resource and data source are also deprecated. Use `coralogix_data_enrichments` instead.

The schemas are different. You must rewrite the HCL. Do not let Terraform create a second enrichment or delete the existing enrichment.

## Before you start

1. Use a provider version that contains this guide and the related import fixes.
2. Run `terraform plan`. Resolve all existing changes first.
3. Back up the state. State can contain sensitive values. Store the backup securely. Do not commit it.

```shell
terraform state pull > terraform-state-before-enrichment-migration.json
```

The API key must have the permissions for the enrichment types that you migrate:

| Enrichment type | Read or import | Create, update, or delete |
|---|---|---|
| AWS | `cloud-metadata-enrichment:ReadConfig` | `cloud-metadata-enrichment:UpdateConfig` |
| Geo IP | `geo-enrichment:ReadConfig` | `geo-enrichment:UpdateConfig` |
| Suspicious IP | `security-enrichment:ReadConfig` | `security-enrichment:UpdateConfig` |
| Custom | `team-custom-enrichment:ReadConfig` | `team-custom-enrichment:UpdateConfig`, `team-custom-enrichment:UpdateData` |

Give the key `team-custom-enrichment:ReadData` too if you use it to download custom data outside this provider.

## Complete resource field map

The old enrichment resource manages one enrichment type. The new resource can manage more than one type. Migrate each old resource to one new resource first. You can combine resources after the migration.

### `coralogix_enrichment`

| Old field | New field | Action |
|---|---|---|
| Resource `id` | Resource `id` | Do not set it. Import sets it. Standard IDs are type names. A custom ID is numeric. |
| `aws`, `geo_ip`, `suspicious_ip`, or `custom` list block | Object with the same name | Replace the block syntax with object syntax. |
| `fields` set | `fields` list | Always write the list. Use the order returned by the new data source. |
| `fields.name` | `fields.name` | Keep the value. |
| `fields.id` | `fields.id` | Do not set it. The API supplies it. |
| `aws.fields.resource` | `aws.fields.resource` | Keep the exact value. |
| `custom.custom_enrichment_id` | `custom.custom_enrichment_data.id` | Do not set it. Import the same numeric ID. The API supplies the nested value. |
| No old field | `fields.enriched_field_name` | Copy the current API value when present. Omit it when the data source returns `null`. |
| No old field | `fields.selected_columns` | Copy the current API values when present. Omit it when the data source returns `null`. |
| No old field | `geo_ip.fields.with_asn` | Copy the current API value when present. |
| `timeouts` block | No equivalent | Remove the block. |

### `coralogix_data_set`

| Old field | New field | Action |
|---|---|---|
| Resource `id` | Resource `id` and `custom.custom_enrichment_data.id` | Do not set either field. Import the old numeric ID. |
| `name` | `custom.custom_enrichment_data.name` | Keep the value. |
| `description` | `custom.custom_enrichment_data.description` | Keep the value. |
| `version` | `custom.custom_enrichment_data.version` | Do not set it. The API supplies it. |
| `file_content` | `custom.custom_enrichment_data.contents` | Keep the same string or `file(...)` expression. |
| `uploaded_file[0].path` | `custom.custom_enrichment_data.contents = file(path)` | Keep the same file path. |
| `uploaded_file[0].modification_time_uploaded` | No equivalent | Remove it. It was computed local file state. |
| `uploaded_file[0].updated_from_uploading` | No equivalent | Remove it. The new resource reads `contents` from its HCL expression. |
| `timeouts` block | No equivalent | Remove the block. |

These tables include every public field from both old resource schemas. The removed fields are local Terraform behavior. They do not contain remote enrichment configuration.

The old schemas allow an empty `fields` set. An empty standard enrichment has no remote field to import. Remove an empty standard resource instead of migrating it. A custom data set can have an empty field list. Migrate its CSV data and use `fields = []`.

Keep every string exactly as the new data source returns it. This rule includes field names, enriched field names, selected column names, and `aws.fields.resource`. Do not change letter case.

The OpenAPI service also has a `targets` field on an enrichment. Neither old Terraform resource exposed it. The current `coralogix_data_enrichments` schema does not expose it. Do not migrate an enrichment that uses targets managed outside Terraform. The provider cannot preserve that field.

The custom OpenAPI response also contains `file_name`, `file_size`, and `is_query_only`. These values are read-only API metadata. Neither old Terraform schema exposed them. They are not migration inputs.

## Read the current OpenAPI values

The old schema does not contain all values that the new schema can return. Read the current values before you rewrite the resource.

Add a temporary data source for the type that you migrate:

```hcl
data "coralogix_data_enrichments" "migration" {
  id = "geo_ip"
}
```

Valid standard IDs are `aws`, `geo_ip`, and `suspicious_ip`. Use the numeric custom-enrichment ID for a custom enrichment. You can use a comma-separated ID to read more than one standard type.

Run:

```shell
terraform apply -target='data.coralogix_data_enrichments.migration'
terraform state show 'data.coralogix_data_enrichments.migration'
```

Copy `enriched_field_name`, `selected_columns`, and `with_asn` when they are present. Omit a field when the data source returns `null`. Remove the temporary data source after the resource migration.

The custom data source returns custom metadata and field mappings. It does not download the CSV contents. Keep the same local file or string that the old `coralogix_data_set` resource used.

## Migrate a standard enrichment resource

This example migrates a Geo IP enrichment.

Before:

```hcl
resource "coralogix_enrichment" "geo_ip" {
  geo_ip {
    fields {
      name = "coralogix.metadata.IPAddress"
    }
  }
}
```

After, when the new data source returns additional values:

```hcl
resource "coralogix_data_enrichments" "geo_ip" {
  geo_ip = {
    fields = [{
      name                = "coralogix.metadata.IPAddress"
      enriched_field_name = "geo_ip"
      selected_columns    = ["city", "country"]
      with_asn            = false
    }]
  }
}
```

The values in this example are only examples. Use the values returned for your account. Omit `enriched_field_name` or `selected_columns` when its returned value is `null`.

Move the state to the new resource:

```shell
terraform state rm 'coralogix_enrichment.geo_ip'
terraform import 'coralogix_data_enrichments.geo_ip' 'geo_ip'
terraform plan
```

Use `aws` or `suspicious_ip` as the import ID for those types. Use a comma-separated ID such as `geo_ip,suspicious_ip` when one new resource manages more than one type.

The plan must not create, update, or delete the remote enrichment. Correct the HCL before you apply if it shows a resource change.

An import by standard type reads all fields for that type. Combine all old resources for that type before import. Do not import one standard type into more than one new resource.

## Migrate a custom enrichment resource

A custom enrichment usually uses two old resources:

- `coralogix_data_set` manages the CSV data.
- `coralogix_enrichment` maps log fields to that data.

The new `coralogix_data_enrichments` resource manages both parts.

Before:

```hcl
resource "coralogix_data_set" "servers" {
  name         = "servers"
  description  = "Server details"
  file_content = file("${path.module}/servers.csv")
}

resource "coralogix_enrichment" "servers" {
  custom {
    custom_enrichment_id = coralogix_data_set.servers.id

    fields {
      name = "server_id"
    }
  }
}
```

After, when the new data source returns additional values:

```hcl
resource "coralogix_data_enrichments" "servers" {
  custom = {
    custom_enrichment_data = {
      name        = "servers"
      description = "Server details"
      contents    = file("${path.module}/servers.csv")
    }

    fields = [{
      name                = "server_id"
      enriched_field_name = "server"
      selected_columns    = ["region", "owner"]
    }]
  }
}
```

Use the current API values for `enriched_field_name` and `selected_columns`. Omit either field when the data source returns `null`.

Use the complete `coralogix_data_set` field map above. Keep the same content source. If the old resource uses `uploaded_file.path`, replace that block with `contents = file(path)`.

Remove both old state addresses. Then import the numeric custom-enrichment ID into the new resource:

```shell
terraform state rm 'coralogix_enrichment.servers'
terraform state rm 'coralogix_data_set.servers'
terraform import 'coralogix_data_enrichments.servers' '12345'
terraform plan
```

Replace `12345` with the existing `coralogix_data_set` ID.

The custom API does not return CSV contents during a normal read. The first plan after import can show one in-place `contents` update. Confirm that the HCL uses the same old file or string. Apply this update once. The provider keeps existing field mappings when only custom data or metadata changes. Run `terraform plan` again. It must show no changes.

## Migrate the data sources

### `coralogix_enrichment`

Change the data-source type. Keep the lookup ID.

Before:

```hcl
data "coralogix_enrichment" "geo_ip" {
  id = "geo_ip"
}
```

After:

```hcl
data "coralogix_data_enrichments" "geo_ip" {
  id = "geo_ip"
}
```

For a custom enrichment, keep its numeric ID:

```hcl
data "coralogix_data_enrichments" "custom" {
  id = "12345"
}
```

Do not run `terraform state rm` or `terraform import` for a data source. Run `terraform plan`, review the output changes, and run `terraform apply`.

#### Complete output map

| Old output | New output | Action |
|---|---|---|
| Lookup `id` | Lookup `id` | Keep the same type name or numeric custom ID. |
| `geo_ip[0]` | `geo_ip` | Remove the `[0]` index. |
| `suspicious_ip[0]` | `suspicious_ip` | Remove the `[0]` index. |
| `aws[0]` | `aws` | Remove the `[0]` index. |
| `custom[0]` | `custom` | Remove the `[0]` index. |
| `fields` set | `fields` list | Use the API order. |
| `fields.name` | `fields.name` | Keep the value. |
| `fields.id` | `fields.id` | Keep downstream references when needed. |
| `aws.fields.resource` | `aws.fields.resource` | Keep the value. |
| No old output | `fields.enriched_field_name` | New API output. It can be `null`. |
| No old output | `fields.selected_columns` | New API output. It can be `null`. |
| No old output | `geo_ip.fields.with_asn` | New API output. |
| `custom[0].custom_enrichment_id` | `custom.custom_enrichment_data.id` | Change the expression path. |
| No old output | `custom.custom_enrichment_data.name`, `description`, and `version` | New API output. |

Remove the `[0]` index after the enrichment type in downstream expressions.

Before:

```hcl
output "first_geo_ip_field" {
  value = tolist(data.coralogix_enrichment.geo_ip.geo_ip[0].fields)[0].name
}
```

After:

```hcl
output "first_geo_ip_field" {
  value = data.coralogix_data_enrichments.geo_ip.geo_ip.fields[0].name
}
```

The custom data source leaves `custom.custom_enrichment_data.contents` empty because a normal read does not download the CSV file.

### `coralogix_data_set`

Change the data-source type. Keep the numeric lookup ID.

Before:

```hcl
data "coralogix_data_set" "servers" {
  id = "12345"
}
```

After:

```hcl
data "coralogix_data_enrichments" "servers" {
  id = "12345"
}
```

Map all outputs as follows:

| Old output | New output |
|---|---|
| `id` | `id` and `custom.custom_enrichment_data.id` |
| `name` | `custom.custom_enrichment_data.name` |
| `description` | `custom.custom_enrichment_data.description` |
| `version` | `custom.custom_enrichment_data.version` |
| `file_content` | `custom.custom_enrichment_data.contents` is empty |
| `uploaded_file[0].path` | No output. Keep the path in resource HCL if the new resource manages this data set. |
| `uploaded_file[0].modification_time_uploaded` | No equivalent. It was local computed state. |
| `uploaded_file[0].updated_from_uploading` | No equivalent. It was local computed state. |

The old data source did not download file content. The new data source also does not download it. Do not use either data source as a source for CSV contents.

## Final verification

Run:

```shell
terraform state list | grep -E 'coralogix_(enrichment|data_set)'
```

The command must return no old resource or data-source address. Search the HCL for both old type names and remove all remaining references:

```shell
grep -R -E 'coralogix_(enrichment|data_set)' --include='*.tf' .
```

Run one final plan. It must show no changes. You can then upgrade to the major provider version that removes the old types.
