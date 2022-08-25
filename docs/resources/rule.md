---
subcategory: "Rules"
page_title: "Coralogix: coralogix_rule"
---

# coralogix_rule

Provides the Coralogix Rule resource. This allows Rule to be created, updated, and deleted.

## Example Usage

```hcl
# Create "My Group" Rules Group
resource "coralogix_rules_group" "rules_group" {
    name    = "My Group"
    enabled = true
}

# Create "Parse Rule" Rule
resource "coralogix_rule" "parse_rule_example" {
    rules_group_id = coralogix_rules_group.rules_group.id
    name              = "My Parse Rule"
    type              = "parse"
    description       = "My Rule created with Terraform"
    expression        = "(?:^|[\\s\"'.:\\-\\[\\]\\(\\)\\{\\}])(?P<severity>DEBUG|TRACE|INFO|WARN|WARNING|ERROR|FATAL|EXCEPTION|[I|i]nfo|[W|w]arn|[E|e]rror|[E|e]xception)(?:$|[\\s\"'.:\\-\\[\\]\\(\\)\\{\\}])"
    source_field      = "text"
    destination_field = "text"
}

# Create "Extract Rule" Rule
resource "coralogix_rule" "extract_rule_example" {
    rules_group_id = coralogix_rules_group.rules_group.id
    name           = "My Extract Rule"
    type           = "extract"
    description    = "My Rule created with Terraform"
    expression     = "message\"\\s*:\\s*\"(?P<bytes>\\d+)\\s*.*?status\\sis\\s(?P<status>[^\"]+)"
    source_field   = "text"
}

# Create "Extract JSON Rule" Rule
resource "coralogix_rule" "extract_json_rule_example" {
    rules_group_id    = coralogix_rules_group.rules_group.id
    name              = "My Extract JSON Rule"
    type              = "jsonextract"
    description       = "My Rule created with Terraform"
    source_field      = "text"
    expression        = "worker"
    destination_field = "category"
}

# Create "Replace Rule" Rule
resource "coralogix_rule" "replace_rule_example" {
    rules_group_id    = coralogix_rules_group.rules_group.id
    name              = "My Replace Rule"
    type              = "replace"
    description       = "My Rule created with Terraform"
    source_field      = "text"
    destination_field = "text"
    expression        = "(.*user\"):\"([^-]*)-([^-]*)-([^-]*)-([^-]*)-([^-]*)\",([^$]*)"
    replace_value     = "$1:{\"name\":\"$2\",\"address\":\"$3\",\"city\":\"$4\",\"state\":\"$5\",\"zip\":\"$6\"},$7"
}

# Create "Block Rule" Rule
resource "coralogix_rule" "block_rule_example" {
    rules_group_id    = coralogix_rules_group.rules_group.id
    name              = "My Block Rule"
    type              = "block"
    description       = "My Rule created with Terraform"
    source_field      = "text"
    expression        = "sql_error_code\\s*=\\s*28000"
    keep_blocked_logs = false
}

# Create "Allow Rule" Rule
resource "coralogix_rule" "allow_rule_example" {
    rules_group_id    = coralogix_rules_group.rules_group.id
    name              = "My Allow Rule"
    type              = "allow"
    description       = "My Rule created with Terraform"
    source_field      = "text"
    expression        = "sql_error_code\\s*=\\s*28000"
    keep_blocked_logs = true
}

# Create "Timestamp Extract Rule" Rule
resource "coralogix_rule" "timestamp_extract_rule_example" {
    rules_group_id    = coralogix_rules_group.rules_group.id
    name              = "My Timestamp Extract Rule"
    type              = "timestampextract"
    description       = "My Rule created with Terraform"
    source_field      = "text.time"
    format_standard   = "golang"
    time_format       = "%Y-%m-%dT%H:%M:%S.%f%z"
}

# Create "Remove Fields Rule" Rule
resource "coralogix_rule" "remove_fields_rule_example" {
    rules_group_id    = coralogix_rules_group.rules_group.id
    name              = "My Remove Fields Rule"
    type              = "removefields"
    description       = "My Rule created with Terraform"
    expression        = "kubernetes.pod_id,metadata.not_needed_field"
}

# Create "Stringify JSON Rule" Rule
resource "coralogix_rule" "stringify_json_rule_example" {
    rules_group_id    = coralogix_rules_group.rules_group.id
    name              = "My Stringify JSON Rule"
    type              = "jsonstringify"
    description       = "My Rule created with Terraform"
    source_field      = "text.inner_json"
    destination_field = "text.stringify_json"
    delete_source     = false
}

# Create "JSON Parse Rule" Rule
resource "coralogix_rule" "json_parse_rule_example" {
    rules_group_id    = coralogix_rules_group.rules_group.id
    name                = "My JSON Parse Rule"
    type                = "jsonparse"
    description         = "My Rule created with Terraform"
    source_field        = "text.stringify_json"
    destination_field   = "text"
    delete_source       = false
    overwrite_destinaton = false
}
```

## Argument Reference

* `rules_group_id` - (Required) Rules Group ID.
* `name` - (Required) Rule name.
* `type` - (Required) Rule type, one of the following: `extract`, `jsonextract`, `parse`, `replace`, `timestampextract`, `removefields`, `block`, `allow`, `jsonparse`, `jsonstringify`.
* `description` - (Optional) Rule description.
* `enabled` - (Optional) Rule state.
* `rule_matcher` - (Optional) A `rule_matcher` block as documented below.
* `expression` - (Required) Rule expression. Should be valid regular expression.
* `source_field` - (Optional) Rule source field.
* `destination_field` - (Optional) Rule destination field.
* `replace_value` - (Optional) Rule replace value.
* `format_standard` - (Optional) Format standard for `timestampextract` rule type, one of the following: `javasdf`, `golang`, `strftime`, `secondsts`, `millits`, `microts`, `nanots`.
* `time_format` - (Optional) Time format for `timestampextract` rule type.
* `keep_blocked_logs` - (Optional) Should the rule keep the blocked logs in the archive and LiveTail, only for rules: `block`, `allow`.
* `delete_source` - (Optional) Should the rule delete the source field, relevant only for rules: `jsonparse`, `jsonstringify`. default is 'false'.
* `overwrite_destinaton` - (Optional) Should the rule overwrite the destination field or merge into it, relevant only for `jsonparse` rule. default is 'false'.
* `escaped_value` - (Optional) Indicate if the value is escaped, relevant only for rules: `jsonparse`, `jsonstringify`. default is 'true'.

---

Each `rule_matcher` block should contains the following:

* `field` - (Required) Rule Matcher field.
* `constraint` - (Required) Rule Matcher constraint.

## Attribute Reference

* `order` - Rule order number.

## Import

Rules can be imported using their ID and rules group ID.

```
$ terraform import coralogix_rule.rule <rules_group_id>/<rule_id>
```