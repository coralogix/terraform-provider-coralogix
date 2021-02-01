---
page_title: "Provider: Coralogix"
---

# Coralogix Provider

The Coralogix provider is used to interact with [Coralogix](https://coralogix.com/) resources.

The provider allows you to manage your Coralogix rules and alerts.
It needs to be configured with the proper credentials before it can be used.

Use the navigation to the left to read about the available resources.

## Example Usage

Terraform 0.13 and later:

```hcl
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
    api_key = ""
}
```

Terraform 0.12 and earlier:

```hcl
# Configure the Coralogix Provider
provider "coralogix" {
    api_key = ""
}
```

## Argument Reference

The following arguments are supported in the `provider` block:

* `url` - (Optional) This is the Coralogix API URL. It is optional, but
  it can be sourced from the `CORALOGIX_URL` environment variable (Default: `https://api.coralogix.com/api/v1`).
* `api_key` - (Required) This is the Coralogix API key. It must be provided, but
  it can also be sourced from the `CORALOGIX_API_KEY` environment variable.
* `timeout` - (Optional) This is the Coralogix API timeout. It is optional, but
  it can be sourced from the `CORALOGIX_API_TIMEOUT` environment variable (Default: `30`).