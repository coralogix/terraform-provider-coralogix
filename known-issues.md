# Coralogix Provider known issues

* Metric lucene alert - There is no validation when `arithmetic_operator_modifier` is set and `arithmetic_operator` is
  not "Percentile". This validation will be added later when we move to the new Terraform Plugin Framework.
* Action is created not hidden when `is_hidden` set to true - this bug will be fixed later. Meanwhile, it's possible to
  update `is_hidden` to true after creation.
* TCO-Policy lose tracking of terraform if they are updated externally - this bug will be fixed later.