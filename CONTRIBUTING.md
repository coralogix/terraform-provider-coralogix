# Contributing to Terraform - Coralogix Provider

Thank you for your interest in contributing to the Coralogix provider. We welcome your contributions. Here you'll find
information to help you get started with provider development.

You are contributing under the terms and conditions of the [Contributor License Agreement](LICENSE) (CLA).

For a set of general guidelines, see
the [CONTRIBUTING.md](https://github.com/hashicorp/terraform/blob/master/.github/CONTRIBUTING.md) page in the main
Terraform repository.

## Documentation

Hashicorp [provider development documentation](https://www.terraform.io/docs/extend/) provides a good start into
developing an understanding of provider development. It's the best entry point if you are new to contributing to this
provider.

To learn more about how to create issues and pull requests in this repository, and what happens after they are created,
you may refer to the resources below:

- [Bug reporting](.github/ISSUE_TEMPLATE/BUG_REPORT.md)
- [Feature requesting](.github/ISSUE_TEMPLATE/FEATURE_REQUEST.md)
- [Pull Request creation and lifecycle](.github/PULL_REQUEST_TEMPLATE.md)

Building the provider
---------------------

### Requirements

- [Terraform](https://www.terraform.io/downloads.html) 0.12.x
- [Go](https://golang.org/doc/install) 1.18.x (to build the provider plugin)

### Steps

Check If GOPATH is set

```sh
$ echo $GOPATH
```

If GOPATH is empty set it

```sh
$ export GOPATH=$HOME/go
```

Clone the repository locally.

```sh
$ mkdir -p $GOPATH/src/github.com/hashicorp; cd $GOPATH/src/github.com/hashicorp
$ git clone git@github.com:coralogix/terraform-provider-coralogix
```

Navigate to the provider directory and build the provider.
Inside the Makefile, change "OS_ARCH=darwin_arm64" to "OS_ARCH=darwin_amd64" if needed (Line 7).

```sh
$ cd $GOPATH/src/github.com/hashicorp/terraform-provider-coralogix
$ make install
```

Running examples
---------------------
Navigate to one of the example directories and initialize the Terraform configuration files.

```sh
$ cd examples/rules_group
$ terraform init
```

Add your api-key at the main.tf file (instead of the commented api_key), or add it as environment variable.

```sh
$ export CORALOGIX_API_KEY="<your api-key>"
```

Change to the desired Coralogix-Environment at the main.tf file (instead of the existed env), or add it as environment
variable.

```sh
$ export CORALOGIX_ENV="<desired Coralogix-Environment>" 
```

You can see the planed resource that will be created.

```sh
$ terraform plan
```

Create the resource.

```sh
$ terraform apply -auto-approve
```

Destroy the resource.

```sh
$ terraform destroy -auto-approve
```

Running tests
---------------------
In order to run the full suite of Acceptance tests, run `make testacc`.

*Note:* Acceptance tests create real resources.

```sh
$ make testacc
```

### Tests

In general, adding test coverage (unit tests and acceptance tests) to new features or bug fixes in your PRs, and sharing
the logs of a successful test run on your branch will greatly speed up the acceptance of your PR.

### Documentations

We use [terraform-plugin-docs](https://github.com/hashicorp/terraform-plugin-docs) for generating documentations
automatically.
In order to generate docs automatically, simply run `make generate`.

```sh
$ make generate
```
