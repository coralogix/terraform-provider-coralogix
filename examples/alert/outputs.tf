output "id" {
  value       = coralogix_alert.example.id
  description = "Alert ID."
}

output "name" {
  value       = coralogix_alert.example.name
  description = "Alert name."
}

output "type" {
  value       = coralogix_alert.example.type
  description = "Alert type."
}

output "severity" {
  value       = coralogix_alert.example.severity
  description = "Alert severity."
}

output "enabled" {
  value       = coralogix_alert.example.enabled
  description = "Alert state."
}

output "filter" {
  value       = coralogix_alert.example.filter
  description = "Alert expression."
}

output "condition" {
  value       = coralogix_alert.example.condition
  description = "Alert condition."
}

output "notifications" {
  value       = coralogix_alert.example.notifications
  description = "Alert notifications."
}