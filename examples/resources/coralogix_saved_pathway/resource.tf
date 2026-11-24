terraform {
  required_providers {
    coralogix = {
      source = "coralogix/coralogix"
    }
  }
}

resource "coralogix_saved_pathway" "example" {
  app_name  = "example-app_name"
  direction = "PATHWAY_DIRECTION_AFTER"
  filters = [
    {
      bool_filter = {
        field = "BOOL_FILTER_FIELD_HAS_SESSION_REPLAY"
        value = true
      }
    },
  ]
  name        = "example-name"
  pages_count = 1
  time_range = {
    calendar = "CALENDAR_WINDOW_TODAY"
  }
  top_n = 1
  view  = "example-view"
}
