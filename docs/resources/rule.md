---
subcategory: "Rules"
page_title: "Coralogix: coralogix_rule"
---

# coralogix_rule

Provides the Coralogix Rule resource. This allows Rule to be created, updated, and deleted.

## Example Usage

```hcl
# Create "My Rule" Rule
resource "coralogix_rule" "example" {
    rules_group_id = "e10ef9d1-36ab-11e8-af8f-02420a00070c"
    name           = "My Rule"
    type           = "extract"
    description    = "My Rule created with Terraform"
    expression     = "(?:^|[\\s\"'.:\\-\\[\\]\\(\\)\\{\\}])(?P<severity>DEBUG|TRACE|INFO|WARN|WARNING|ERROR|FATAL|EXCEPTION|[I|i]nfo|[W|w]arn|[E|e]rror|[E|e]xception)(?:$|[\\s\"'.:\\-\\[\\]\\(\\)\\{\\}])"
    
    rule_matcher {
        field      = "text"
        constraint = "(?:^|[\\s\"'.:\\-\\[\\]\\(\\)\\{\\}])(?P<severity>DEBUG|TRACE|INFO|WARN|WARNING|ERROR|FATAL|EXCEPTION|[I|i]nfo|[W|w]arn|[E|e]rror|[E|e]xception)(?:$|[\\s\"'.:\\-\\[\\]\\(\\)\\{\\}])"
    }
}
```

## Argument Reference

* `rules_group_id` - (Required) Rules Group ID.
* `name` - (Required) Rule name.
* `type` - (Required) Rule type, one of the following: `extract`, `jsonextract`, `parse`, `replace`, `allow`, `block`.
* `description` - (Optional) Rule description.
* `enabled` - (Optional) Rule state.
* `rule_matcher` - (Optional) A `rule_matcher` block as documented below.
* `expression` - (Required) Rule expression. Should be valid regular expression.
* `source_field` - (Optional) Rule source field.
* `destination_field` - (Optional) Rule destination field.
* `replace_value` - (Optional) Rule replace value.

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