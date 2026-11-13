---
name: dashboard-fake-acceptance-run
description: "Use when running the handwritten coralogix_dashboard acceptance tests without a tenant. They run against the fake API of the baseline harness."
---

# Run the dashboard acceptance tests on the fake API

**Trigger:** A change to the dashboard schema, hooks, or generated package, and no tenant at hand.

**Fix:**
```bash
DASHBOARD_FAKE_ACC=1 TF_ACC=1 CORALOGIX_API_KEY=placeholder CORALOGIX_DOMAIN=eu2.coralogix.com \
  go test ./internal/provider -run 'TestAcc.*Dashboard' -skip 'Folder|Grafana|Hosted' -count=1 -timeout 90m
```
A failure names a place where the fake, or the recorded baseline, differs from the real API. The fake accepts any text, so a rule that only the API checks (a duration, a query, a bound) passes here and fails on a tenant: run the generated test (`go test ./internal/provider/generated/dashboard -run TestAccCoralogixResourceDashboardGenerated`) on a tenant too.

**Why:** The walk and scenario baselines do not apply, change, and remove every value through real Terraform. These tests do, and they found gaps that the baselines did not.
