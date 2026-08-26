data "coralogix_user" "imported_by_id" {
  id = coralogix_user.example.id
}

data "coralogix_user" "imported_by_user_name" {
  user_name = "example@coralogix.com"
}

resource "coralogix_group" "members_by_email" {
  display_name = "example-members-by-email"
  role         = coralogix_custom_role.example.name
  members      = [data.coralogix_user.imported_by_user_name.id]
}
