---
subcategory: ""
page_title: "Coralogix API Authorization"
---

# Authorization

To allow interaction with Coralogix API you need to configure provider with API key and the relevant api URL.

## To get your API key:
1. [Login to your Coralogix account](https://dashboard.coralogix.com/#/login)
2. On the upper options bar navigate the following: 
	``Data Flow`` -> ``API Keys`` -> ``Alerts, Rules and Tags API Key``
3. If the box containing the masked key is empty, click ``Generate new API key``

## To Get your URL string (Coralogix Endpoints):

| Region  | Logs Endpoint
|---------|------------------------------------------|
| EU      | `https://api.coralogix.com/api/v1`       |
| EU2     | `https://api.eu2.coralogix.com/api/v1`   |
| US      | `https://api.coralogix.us/api/v1`        |
| SG      | `https://api.coralogixsg.com/api/v1`     |
| IN      | `https://api.app.coralogix.in/api/v1`    |

## Example for application:
```hcl
# Set provider and it's source
terraform {
  required_providers {
    coralogix = {
      source  = "coralogix/coralogix"
      version = "~> 1.0"
    }
  }
}

# Configure the Coralogix Provider
provider "coralogix" {
    api_key = "<my secret Alerts, Rules and Tags API Key>"
    url = "https://api.eu2.coralogix.com/api/v1"
    timeout = 30
}

```