# Coralogix Provider known issues

* Metric lucene alert - There is no validation when `arithmetic_operator_modifier` is set and `arithmetic_operator` is
  not "Percentile". This validation will be added later when we move to the new Terraform Plugin Framework.
* Action is created not hidden when `is_hidden` set to true - this bug will be fixed later. Meanwhile, it's possible to
  update `is_hidden` to true after creation.
* TCO-Policy lose tracking of terraform if they are updated externally - this bug will be fixed later.

#### TCO-Policy - _order_ can not be configured via terraform on creation.

* Currently, there is no support in the backend side to control the order of TCO policies on creation several Policies in parallel (it is planned to be supported).
Because of the way terraform works (applies creating requests in parallel) there are few ways the Policies order via terraform -
 1. Creating the policies in the desired order - manually, or by adding dependency in their creation order.

  e.g -

 ```
 resource "coralogix_tco_policy" "tco_policy_1" {
    ...
    order = 1
  }

  resource "coralogix_tco_policy" "tco_policy_2" {
    ....
    order = coralogix_tco_policy.tco_policy_1.order + 1
 }
 ```

2. Updating the state (terraform apply) after creation.

 ```
 resource "coralogix_tco_policy" "tco_policy_1" {
    ...
    order = 1
  }

  resource "coralogix_tco_policy" "tco_policy_2" {
    ....
    order = 2
 }
 ```

#### Events2Metrics - issue with `aggregations` updating

* terraform not tracking changes in `metric_fields.*.aggregations` - we investigate this issue.