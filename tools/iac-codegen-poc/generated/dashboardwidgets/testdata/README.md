Test fixtures of real dashboard widgets, in the API shape (camelCase, no
unknown fields):

- `content_json_*.json`: copies of the provider's dashboard test fixtures
  (`internal/provider/testdata/dashboards/`).
- `example_dashboard_with_var_path.json`: a copy of the provider's example
  (`examples/resources/coralogix_dashboard/dashboard_with_var_path.json`).
- `hexagon_absolute_time_frame.json`: handwritten. A hexagon whose DataPrime
  query has an absolute time frame, the only request timestamp in the
  widgets. No other fixture has one.
