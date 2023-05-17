# Coralogix Provider known issues

* Metric lucene alert - There is no validation when `arithmetic_operator_modifier` is set and `arithmetic_operator` is
  not "Percentile". This validation will be added later when we move to the new Terraform Plugin Framework.
* Action is created not hidden when `is_hidden` set to true - this bug will be fixed later. Meanwhile, it's possible to
  update `is_hidden` to true after creation.
* TCO-Policy lose tracking of terraform if they are updated externally - this bug will be fixed later.

#### TCO-Policy - _order_ gets an incorrect value via terraform.

* Currently, there is no support in the backend side to control the order of TCO policies on creation several Policies in parallel (it is planned to be supported).
Therefore, it required to apply the policies without parallelism (`terraform apply -auto-approve -parallelism=1`).

#### Events2Metrics - issue with `aggregations` updating

* terraform not tracking changes in `metric_fields.*.aggregations` - we investigate this issue.