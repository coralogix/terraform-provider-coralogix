resource "coralogix_custom_role" "example" {
  name        = "Example custom role"
  description = "This role is created with terraform!"
  parent_role = "Standard User"
  permissions = ["spans.events2metrics:UpdateConfig"]
}

resource "coralogix_user" "example" {
  user_name = "example@coralogix.com"
  name = {
    given_name  = "example"
    family_name = "example"
  }
}

data "coralogix_user" "imported_by_id" {
  id = coralogix_user.example.id
}

data "coralogix_user" "imported_by_user_name" {
  user_name  = "example@coralogix.com"
  depends_on = [coralogix_user.example]
}

resource "coralogix_group" "members_by_email" {
  display_name = "example-members-by-email"
  role         = coralogix_custom_role.example.name
  members      = [data.coralogix_user.imported_by_user_name.id]
}
