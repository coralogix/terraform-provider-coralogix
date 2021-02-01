output "id" {
  value       = coralogix_rules_group.example.id
  description = "Rules Group ID."
}

output "name" {
  value       = coralogix_rules_group.example.name
  description = "Rules Group name."
}

output "order" {
  value       = coralogix_rules_group.example.order
  description = "Rules Group order."
}

output "enabled" {
  value       = coralogix_rules_group.example.enabled
  description = "Rules Group state."
}

output "rules" {
  value       = coralogix_rules_group.example.rules
  description = "Rules Group rules."
}