variable "api_key" {
    type        = string
    description = "Coralogix API key."
}

variable "alert_name" {
    type        = string
    description = "Coralogix Alert name."
}

variable "alert_enabled" {
    type        = bool
    description = "Coralogix Alert state."
    default     = true
}