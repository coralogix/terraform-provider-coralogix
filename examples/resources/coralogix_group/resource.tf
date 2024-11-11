resource "coralogix_user" "example" {
  user_name = "example@coralogix.com"
  name      = {
    given_name  = "example"
    family_name = "example"
  }
}

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

resource "coralogix_group" "example" {
  display_name = "example"
  role         = "Read Only"
  members      = [coralogix_user.example.id]
  scope_id     = coralogix_scope.example.id
}

