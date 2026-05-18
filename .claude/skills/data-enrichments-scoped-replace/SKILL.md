---
name: data-enrichments-scoped-replace
description: "Use when changing coralogix_data_enrichments update semantics or drift tests. Prefer state-ID delete plus add over partial atomic overwrite for scoped replaces."
---

# Data Enrichments Scoped Replace

**Trigger:** updating `internal/provider/enrichment_rules/resource_coralogix_data_enrichments.go` or debugging duplicated `geo_ip`, `suspicious_ip`, `aws`, or `custom` fields after update.

**Fix:** For resource-scoped updates, read prior state IDs, call `EnrichmentServiceRemoveEnrichments` for only those IDs, then call `EnrichmentServiceAddEnrichments` with the planned fields. Do not use `EnrichmentServiceAtomicOverwriteEnrichments` for mixed non-custom resources.

**Why:** The partial atomic-overwrite API is typed and can append planned enrichments beside existing ones when Terraform manages multiple enrichment types, producing inconsistent-result errors and duplicated backend state.
