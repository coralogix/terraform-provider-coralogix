---
name: ephemeral-team-for-singleton-tests
description: "Use when an acceptance test mutates team-wide singleton state (archive, TCO, quota, ip_access, enrichments) and flakes under concurrent CI runs. Isolates the test in a disposable team."
---

# Ephemeral team per singleton acceptance test

**Trigger:** a test mutates one-per-team state (ID-less API, atomic list overwrite, hardcoded resource ID) and concurrent workflow runs clobber each other. Applies to `coralogix_archive_logs`, `archive_metrics`, `archive_retentions`, `tco_policies_*`, `quota_allocation_rule_set`, `ip_access`, `data_enrichments`. Do NOT use for ID-addressed resources — unique names suffice there.

**Fix:** prepend the harness's provider override to every `TestStep` config:

```go
providerConfig := ephemeralteam.ProviderConfig(t) // "" when CORALOGIX_ORG_API_KEY unset
// ...
Config: providerConfig + testAccMyResourceConfig(),
```

`internal/ephemeralteam` creates a team with the org-scoped `CORALOGIX_ORG_API_KEY`, mints a team key, and deletes the team only if the test passed (kept + logged on failure; names start with `tf-acc-ephemeral`).

**Why:** the framework provider gives the HCL `api_key` precedence over `CORALOGIX_API_KEY`, so the injected team key redirects the whole test into the fresh team. The SDKv2 provider has the opposite precedence — this does NOT work for SDKv2 resources (e.g. `coralogix_enrichment`).
