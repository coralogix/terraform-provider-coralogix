# Coralogix Provider known issues

### Using the provider


- *resource_coralogix_alert*s are not tracked by the vendor if they have been updated outside terraform - this bug will be fixed soon.

### Developing the provider

- *rule_group*s *acc-test*s are not stable for github action - If **TestAccCoralogixResourceRuleGroup_xxx** fails due to the next error - 
  `ImportStateVerify attributes not equivalent. Difference is shown below. Top is actual, bottom is expected.`, try to re-run the test.


- *logs2metric*s *acc-test*s are not stable for github action - If **TestAccCoralogixResourceLogs2Metric_xxx** fails due to the next error -
  `Error: invalid argument - rpc error: code = InvalidArgument desc = {"message":"Bad Request","status":400,"details":[{"code":"custom","path":[],"message":"Metric fields are already in use: method, geo_point"}]}`, try to re-run the test.