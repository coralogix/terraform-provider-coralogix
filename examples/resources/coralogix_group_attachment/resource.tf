terraform {
  required_providers {
    coralogix = {
      version = "~> 2.0"
      source  = "coralogix/coralogix"
    }
  }
}

provider "coralogix" {
  #api_key = "<add your api key here or add env variable CORALOGIX_API_KEY>"
  #env = "<add the environment you want to work at or add env variable CORALOGIX_ENV>"
}

resource "coralogix_user" "example" {
  user_name = "example@coralogix.com"
  name      = {
    given_name  = "example"
    family_name = "example"
  }
}

resource "coralogix_user" "example2" {
  user_name = "example2@coralogix.com"
  name      = {
    given_name  = "example2"
    family_name = "example2"
  }
}

data "coralogix_group" "example" {
  display_name = "ReadOnlyUsers"
}

resource "coralogix_group_attachment" "example" {
  group_id = data.coralogix_group.example.id
  user_ids  = [coralogix_user.example.id, coralogix_user.example2.id]
}