terraform {
  required_providers {
    coralogix = {
      version = "~> 1.7"
      source  = "coralogix/coralogix"
    }
  }
}

provider "coralogix" {
  #api_key = "<add your api key here or add env variable CORALOGIX_API_KEY>"
  #env = "<add the environment you want to work at or add env variable CORALOGIX_ENV>"
}

resource "coralogix_tco_policy_traces" "tco_policy_1" {
  name         = "Example tco_policy from terraform 1"
  priority     = "low"
  order        = 1
  applications = {
    rule_type = "starts_with"
    names     = ["prod"]
  }
  subsystems = {
    rule_type = "is"
    names     = ["mobile", "web"]
  }
  actions = {
    rule_type = "is_not"
    names     = ["action-name", "action-name2"]
  }
  services = {
    rule_type = "is"
    names     = ["service-name", "service-name2"]
  }
  tags = {
    "tags.http.method" = {
      rule_type = "includes"
      names     = ["GET"]
    }
  }
  archive_retention_id = "e1c980d0-c910-4c54-8326-67f3cf95645a"
}

data "coralogix_tco_policy_traces" "imported_co_policy_traces_example" {
  id = coralogix_tco_policy_traces.tco_policy_1.id
}


resource "coralogix_tco_policy_traces" "tco_policy_2" {
  name         = "Example tco_policy from terraform 2"
  priority     = "medium"
  order        = 2
  applications = {
    rule_type = "starts_with"
    names     = ["staging"]
  }
  subsystems = {
    rule_type = "is_not"
    names     = ["mobile", "web"]
  }
  actions = {
    names = ["action-name", "action-name2"]
  }
  services = {
    names = ["service-name", "service-name2"]
  }
  tags = {
    "tags.http.method" = {
      rule_type = "is_not"
      names     = ["GET", "POST"]
    }
  }
}

resource "coralogix_tco_policy_traces" "tco_policy_3" {
  name         = "Example tco_policy from terraform 3"
  priority     = "medium"
  order        = 3
  applications = {
    rule_type = "starts_with"
    names     = ["staging"]
  }
  subsystems = {
    rule_type = "is_not"
    names     = ["mobile", "web"]
  }
  actions = {
    names = ["action-name", "action-name2"]
  }
  services = {
    names = ["service-name", "service-name2"]
  }
  tags = {
    "tags.http.method" = {
      rule_type = "is_not"
      names     = ["GET", "POST"]
    }
  }
}