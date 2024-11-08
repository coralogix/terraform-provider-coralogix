resource "coralogix_alerts_scheduler" "example" {
  name        = "example"
  description = "example"
  filter      = {
    what_expression   = "source logs | filter $d.cpodId:string == '122'"
    alerts_unique_ids = ["ed6f3713-d827-49a2-9bb6-a8dba8b8c580"]
  }
  schedule = {
    operation = "mute"
    one_time  = {
      time_frame = {
        start_time = "2021-01-04T00:00:00.000"
        end_time   = "2025-01-01T00:00:50.000"
        time_zone  = "UTC+2"
      }
    }
  }
}

resource "coralogix_alerts_scheduler" "example_2" {
  name        = "example"
  description = "example"
  filter      = {
    what_expression = "source logs | filter $d.cpodId:string == '122'"
    meta_labels     = [
      {
        key   = "key"
        value = "value"
      }
    ]
  }
  schedule = {
    operation = "active"
    recurring = {
      dynamic = {
        repeat_every = 2
        frequency = {
          weekly = {
            days = ["Sunday"]
          }
        }
        time_frame = {
          start_time = "2021-01-04T00:00:00.000"
          duration = {
            for_over = 2
            frequency = "hours"
          }
          time_zone = "UTC+2"
        }
        termination_date = "2025-01-01T00:00:00.000"
      }
    }
  }
}