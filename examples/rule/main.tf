provider "coralogix" {
    api_key = var.api_key
}

resource "coralogix_rule" "example" {
    rules_group_id = var.rules_group_id
    name           = var.rule_name
    type           = "extract"
    description    = "Rule created by Terraform"
    expression     = "(?:^|[\\s\"'.:\\-\\[\\]\\(\\)\\{\\}])(?P<severity>DEBUG|TRACE|INFO|WARN|WARNING|ERROR|FATAL|EXCEPTION|[I|i]nfo|[W|w]arn|[E|e]rror|[E|e]xception)(?:$|[\\s\"'.:\\-\\[\\#\\]\\(\\)\\{\\}])"
}