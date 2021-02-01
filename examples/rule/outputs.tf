output "id" {
  value       = coralogix_rule.example.id
  description = "Rule ID."
}

output "name" {
  value       = coralogix_rule.example.name
  description = "Rule name."
}

output "type" {
  value       = coralogix_rule.example.type
  description = "Rule type."
}

output "order" {
  value       = coralogix_rule.example.order
  description = "Rule order."
}

output "enabled" {
  value       = coralogix_rule.example.enabled
  description = "Rule state."
}

output "expression" {
  value       = coralogix_rule.example.expression
  description = "Rule expression."
}