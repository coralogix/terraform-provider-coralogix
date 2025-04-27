data "coralogix_custom_role" "imported_by_id" {
  id = coralogix_custom_role.example.id
}

data "coralogix_custom_role" "imported_by_name" {
  name = coralogix_custom_role.example.name
}
