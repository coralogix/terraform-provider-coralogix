---
name: custom-role-permission-alias-normalization
description: "Use when the coralogix_custom_role resource shows spurious plan diffs or apply errors after a server-side permission expression rename (e.g. 'alerts-map:Read' → 'alerts:MapRead'). Documents the alias-map pattern, the ModifyPlan suppression approach, and the ListAllPermissions dependency."
---

# Custom Role Permission Expression Alias Normalization

**Trigger:** `terraform plan` on `coralogix_custom_role` shows a diff or `terraform apply` errors with "permission X was not returned from API" after a permission expression is renamed on the server side.

**Fix:**

1. `flattenCustomRole` now takes a `map[string]string` alias map (lowercase deprecated → lowercase canonical) as a third argument. Both sides of the comparison are passed through `normalizePermission(expr, aliases)` before comparing.

2. `normalizePermission` does: `lower := strings.ToLower(expr); if canon, ok := aliases[lower]; ok { return canon }; return lower`.

3. The alias map is built from `PermissionsClient.ListAll()` (gRPC `PermissionsService/ListAllPermissions`) in `CustomRoleSource.Configure()`. Failures are non-fatal (empty map, warning diagnostic) so the resource degrades gracefully when the endpoint is not yet deployed.

4. `CustomRoleSource` also implements `resource.ResourceWithModifyPlan`. `ModifyPlan` suppresses the diff when every element in the config normalizes to the same canonical as its counterpart in state. This handles the edge case where state already has the canonical form but config still uses the deprecated alias.

**Key files:**
- `internal/provider/aaa/resource_coralogix_custom_role.go` — resource + `flattenCustomRole` + `ModifyPlan`
- `internal/clientset/clientset.go` — `PermissionsClient` field + accessor
- `coralogix-management-sdk/go/permissions-client.go` — SDK wrapper for `PermissionsService`
- `coralogix-management-sdk/go/internal/coralogixapis/aaa/rbac/v2/permissions*.go` — generated pb.go

**Why:** Server-side expression renames are invisible to the provider without an alias map. The pure `strings.ToLower` comparison fails once the API stops returning the old form. The alias map bridges old and canonical names so both `flattenCustomRole` (apply correctness) and `ModifyPlan` (plan diff suppression) stay stable across renames.
