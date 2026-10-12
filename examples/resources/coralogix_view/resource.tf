terraform {
  required_providers {
    coralogix = {
      version = "~> 3.0"
      source  = "coralogix/coralogix"
    }
  }
}

resource "coralogix_view" "example" {
  name            = "Logs view"
  view_type       = "VIEW_TYPE_LOGS"
  is_compact_mode = false
  search_query = {
    query       = "coralogix.metadata.applicationName:\"production\""
    syntax_type = "SYNTAX_TYPE_LUCENE"
  }
  time_selection = {
    quick_selection = {
      seconds = 3600
    }
  }
}
