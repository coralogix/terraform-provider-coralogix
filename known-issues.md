# Coralogix Provider known issues

### Using the provider

- *resource_coralogix_alert*s are not tracked by the vendor if they have been updated outside terraform - this bug will
  be fixed soon.

- tracing alerts can not be created with on_trigger_and_resolved=true - this bug will be fixed soon.
