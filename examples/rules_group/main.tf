provider "coralogix" {
    api_key = var.api_key
}

resource "coralogix_rules_group" "example" {
    name    = var.rules_group_name
    enabled = var.rules_group_enabled
}