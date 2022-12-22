terraform {
  required_providers {
    coralogix = {
      version = "~> 1.3"
      source  = "coralogix/coralogix"
    }
  }
}

provider "coralogix" {
  #api_key = "<add your api key here or add env variable CORALOGIX_API_KEY>"
  #env = "<add the environment you want to work at or add env variable CORALOGIX_ENV>"
}

resource "coralogix_rules_group" "rules_group_example" {
  name         = "Example rule-group from terraform"
  description  = "rule_group creates by coralogix terraform provider"
  applications = ["nginx"] //change here for existing applications from your account
  subsystems   = ["subsystem-name"] //change here for existing subsystems from your account
  severities   = ["Warning"]

  rule_subgroups {
    order = 3
    rules {
      extract {
        name               = "Severity Rule"
        description        = "Look for default severity text"
        source_field       = "text"
        regular_expression = "message\\s*:s*(?P<bytes>\\d+)\\s*.*?status\\sis\\s(?P<status>\\[^\"]+)"
      }
    }

    rules {
      json_extract {
        name              = "Worker to category"
        description       = "Extracts value from 'worker' and populates 'Category'"
        json_key          = "worker"
        destination_field = "Category"
      }
    }

    rules {
      replace {
        name               = "Delete prefix"
        description        = "Deletes data before Json"
        source_field       = "text"
        destination_field  = "text"
        replacement_string = "{"
        regular_expression = ".*{"
      }
    }

    rules {
      block {
        name               = "Block 28000"
        description        = "Block 2800 pg error"
        source_field       = "text"
        regular_expression = "sql_error_code\\s*=\\s*28000"
      }
    }
  }

  rule_subgroups {
    rules {
      parse {
        name               = "HttpRequestParser1"
        description        = "Parse the fields of the HTTP request"
        source_field       = "text"
        destination_field  = "text"
        regular_expression = "(?P<remote_addr>\\d{1,3}.\\d{1,3}.\\d{1,3}.\\d{1,3})\\s*-\\s*(?P<user>[^ ]+)\\s*\\[(?P<timestemp>\\d{4}-\\d{2}\\-\\d{2}T\\d{2}\\:\\d{2}\\:\\d{2}\\.\\d{1,6}Z)\\]\\s*\\\\\\\"(?P<method>[A-z]+)\\s[\\/\\\\]+(?P<request>[^\\s]+)\\s*(?P<protocol>[A-z0-9\\/\\.]+)\\\\\\\"\\s*(?P<status>\\d+)\\s*(?P<body_bytes_sent>\\d+)?\\s*?\\\\\\\"(?P<http_referer>[^\"]+)\\\"\\s*\\\\\\\"(?P<http_user_agent>[^\"]+)\\\"\\s(?P<request_time>\\d{1,6})\\s*(?P<response_time>\\d{1,6})"
      }
    }

    rules {
      parse {
        name               = "HttpRequestParser2"
        description        = "Parse the fields of the HTTP request - will be applied after HttpRequestParser1"
        source_field       = "text"
        destination_field  = "text"
        regular_expression = "(?P<remote_addr>\\d{1,3}.\\d{1,3}.\\d{1,3}.\\d{1,3})\\s*-\\s*(?P<user>[^ ]+)\\s*\\[(?P<timestemp>\\d{4}-\\d{2}\\-\\d{2}T\\d{2}\\:\\d{2}\\:\\d{2}\\.\\d{1,6}Z)\\]\\s*\\\\\\\"(?P<method>[A-z]+)\\s[\\/\\\\]+(?P<request>[^\\s]+)\\s*(?P<protocol>[A-z0-9\\/\\.]+)\\\\\\\"\\s*(?P<status>\\d+)\\s*(?P<body_bytes_sent>\\d+)?\\s*?\\\\\\\"(?P<http_referer>[^\"]+)\\\"\\s*\\\\\\\"(?P<http_user_agent>[^\"]+)\\\"\\s(?P<request_time>\\d{1,6})\\s*(?P<response_time>\\d{1,6})"
      }
    }

    rules {
      json_stringify {
        name              = "json_stringify rule"
        source_field      = "text"
        destination_field = "text"
        keep_source_field = "false"
        #for better example look at - https://coralogix.com/docs/log-parsing-rules/#stringify-json-fields
      }
    }

  }
}

data "coralogix_rules_group" "imported_rules_group_example" {
  id = coralogix_rules_group.rules_group_example.id
}

resource "coralogix_rules_group" "extract_timestamp_example" {
  name         = "Example extract-timestamp rule-group from terraform"
  description  = "rule_group created by coralogix terraform provider"
  applications = ["nginx"] //change here for existing applications from your account
  subsystems   = ["subsystem-name"] //change here for existing subsystems from your account
  severities   = ["Warning"]

  rule_subgroups {
    order = 2
    rules {
      extract_timestamp {
        name                  = "example extract-timestamp rule from terraform"
        description           = "rule created by coralogix terraform provider"
        source_field          = "text"
        field_format_standard = "Strftime"
        time_format           = "%Y-%m-%dT%H:%M:%S.%f%z"
      }
    }
  }
}

resource "coralogix_rules_group" "remove_fields_example" {
  name         = "Example remove-fields rule-group from terraform"
  description  = "rule_group created by coralogix terraform provider"
  applications = ["nginx"] //change here for existing applications from your account
  subsystems   = ["subsystem-name"] //change here for existing subsystems from your account
  severities   = ["Warning"]
  rule_subgroups {
    order = 1
    rules {
      remove_fields {
        name            = "Example remove-fields rule from terraform"
        description     = "rule created by coralogix terraform provider"
        excluded_fields = ["coralogix.metadata.applicationName", "coralogix.metadata.className"]
      }
    }
  }
}

resource "coralogix_rules_group" "parse_json_field_example" {
  order = 0
  name         = "Example parse-json-field rule-group from terraform"
  description  = "rule_group created by coralogix terraform provider"
  applications = ["nginx"] //change here for existing applications from your account
  subsystems   = ["subsystem-name"] //change here for existing subsystems from your account
  severities   = ["Info"]
  rule_subgroups {
    rules {
      parse_json_field {
        name                   = "Example remove-fields rule from terraform"
        description            = "rule created by coralogix terraform provider"
        source_field           = "text"
        destination_field      = "text"
        keep_source_field      = "true"
        keep_destination_field = "true"
      }
    }
  }
}