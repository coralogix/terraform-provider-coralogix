
resource "coralogix_hosted_dashboard" dashboard {
  grafana {
    config_json = file("./grafana_dashboard.json")
    folder = coralogix_grafana_folder.test_folder.id
  }
}

resource "coralogix_grafana_folder" "test_folder" {
  title = "Terraform Test Folder"
}

