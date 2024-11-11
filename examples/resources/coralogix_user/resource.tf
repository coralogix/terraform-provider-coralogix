resource "coralogix_custom_role" "example" {
  name  = "Example custom role"
  description = "This role is created with terraform!"
  parent_role = "Standard User"
  permissions = ["spans.events2metrics:UpdateConfig"]
}

resource "coralogix_user" "example" {
  user_name = "example@coralogix.com"
  name      = {
    given_name  = "example"
    family_name = "example"
  }
}

resource "coralogix_group" "example" {
  display_name = "example"
  role         = coralogix_custom_role.example.name
  members      = [coralogix_user.example.id]
}

