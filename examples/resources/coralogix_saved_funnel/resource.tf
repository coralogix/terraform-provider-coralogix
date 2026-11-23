terraform {
  required_providers {
    coralogix = {
      source = "coralogix/coralogix"
    }
  }
}

resource "coralogix_saved_funnel" "example" {
  app_name = "example-app_name"
  breakdown = {
    field = "BREAKDOWN_FIELD_ACCOUNT_ID"
    limit = 1
  }
  definition = {
    conversion_window = "1s"
    mode              = "FUNNEL_MODE_USERS"
    steps = [
      {
        conditions = [
          {
            action = {
              element = {
                operator = "STRING_CONDITION_OPERATOR_EQUAL"
                values   = ["example-values"]
              }
              event = {
                operator = "STRING_CONDITION_OPERATOR_EQUAL"
                values   = ["example-values"]
              }
              filters = [
                {
                  bool_filter = {
                    field = "BOOL_FILTER_FIELD_HAS_SESSION_REPLAY"
                    value = true
                  }
                },
              ]
            }
          },
        ]
        label   = "example-label"
        step_id = "example-step_id"
      },
    ]
  }
  filters = [
    {
      bool_filter = {
        field = "BOOL_FILTER_FIELD_HAS_SESSION_REPLAY"
        value = true
      }
    },
  ]
  name = "example-name"
  time_range = {
    calendar = "CALENDAR_WINDOW_TODAY"
  }
}
