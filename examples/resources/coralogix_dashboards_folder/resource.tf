resource "coralogix_dashboards_folder" "example" {
  name     = "example"
}

resource "coralogix_dashboards_folder" "example_2" {
  name     = "example2"
  parent_id = coralogix_dashboards_folder.example.id
}