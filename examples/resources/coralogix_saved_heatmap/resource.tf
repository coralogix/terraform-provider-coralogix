terraform {
  required_providers {
    coralogix = {
      source = "coralogix/coralogix"
    }
  }
}

resource "coralogix_saved_heatmap" "example" {
  action_name_filter = "ACTION_NAME_FILTER_ALL_CLICKS"
  app_name           = "example-app_name"
  filters = [
    {
      bool_filter = {
        field = "BOOL_FILTER_FIELD_HAS_SESSION_REPLAY"
        value = true
      }
    },
  ]
  mode        = "HEATMAP_MODE_TOP_ELEMENTS"
  name        = "example-name"
  snapshot_id = "example-snapshot_id"
  time_range = {
    calendar = "CALENDAR_WINDOW_TODAY"
  }
  view_name = "example-view_name"
}
