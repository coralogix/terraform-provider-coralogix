resource "coralogix_api_key" "example" {
  name  = "My APM KEY"
  owner = {
    team_id : "4013254"
  }
  active = true
  presets = ["APM"]
  permissions = ["livetail:Read"]
}
