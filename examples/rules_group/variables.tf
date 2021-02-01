variable "api_key" {
    type        = string
    description = "Coralogix API key."
}

variable "rules_group_name" {
    type        = string
    description = "Coralogix Parsing Rule Group name."
}

variable "rules_group_enabled" {
    type        = bool
    description = "Coralogix Parsing Rule Group state."
    default     = true
}