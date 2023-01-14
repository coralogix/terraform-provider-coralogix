# Coralogix Provider known issues

* Metric lucene alert - There is no validation when `arithmetic_operator_modifier` is set and `arithmetic_operator` is not "Percentile". This validation will be added later.
* Action is created not hidden when `is_hidden` set to true - this bug will be fixed later. Meanwhile, it's possible to update `is_hidden` to true after creation.
* Action `source_type` can't be change after creation - this bug will be fixed later.