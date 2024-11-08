resource "coralogix_scope" "example" {
  display_name       = "ExampleScope"
  default_expression = "<v1>true"
  filters            = [
    {
      entity_type = "logs"
      expression  = "<v1>(subsystemName == 'purchases') || (subsystemName == 'signups')"
    }
  ]
}
