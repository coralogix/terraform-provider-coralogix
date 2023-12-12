terraform {
  required_providers {
    coralogix = {
      version = "~> 1.9"
      source  = "coralogix/coralogix"
    }
  }
}

provider "coralogix" {
  #api_key = "<add your api key here or add env variable CORALOGIX_API_KEY>"
  #env = "<add the environment you want to work at or add env variable CORALOGIX_ENV>"
}

resource "coralogix_archive_retentions" "example" {
  retentions = [
    {
      id   = "e1c980d0-c910-4c54-8326-67f3cf95645a"
      name = "test1"
    },
    {
      id   = "729ee424-60de-4d31-9983-e6431250e5f2"
      name = "test2"
    },
    {
      id   = "6e6ed3ac-a365-4ded-ac00-7c1cfd429d1d"
      name = "test3"
    },
    {
      id   = "d3a169a7-b9ca-4187-bf97-3267eb89b882"
      name = "test4"
    },
  ]
}