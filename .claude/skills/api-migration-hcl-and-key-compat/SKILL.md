---
name: api-migration-hcl-and-key-compat
description: "Use when migrating a resource to a new backend API (SCIM, REST, gRPC, OpenAPI), or when create/update starts resolving a name via another service. Check HCL string identity and extra API-key permissions."
---

# API migration: keep HCL strings and API keys working

**Trigger:** Switching a resource's backend, or adding a lookup (name → id) on another service. "Terraform attributes are unchanged" is not a compatibility proof.

**Fix:** Before the PR, do these checks:

1. **Names.** For every HCL string the old API accepted and returned, confirm the new API lists that exact string. If it renamed values, ask backend for the full mapping. Do not alias one name and assume the rest match.
2. **Read name.** Write can accept the old HCL string while GET returns a different canonical name. Confirm GET. Do not alias one name in the provider to hide that. If they differ, it is a backend gap: the next plan reports a change. Do not assume write-acceptance means round-trip identity.
3. **Permissions.** List every RPC create/update/read now calls. Map each to a permission in `mapRoleToPermission` (`internal/provider/aaa/resource_coralogix_api_key.go`). A new RPC on another service needs that service's permission. Existing least-privilege keys that could do the old write will fail before the intended API. Prefer sending the name on the original service over a lookup on another service. If a lookup is required, document the extra permission as a breaking change.

Acc tests with a full admin key do not catch (3).

**Why:** Old writes often send a name in one call. New APIs often want an id, so the provider lists another service first. Listed names can also differ from the old round-trip string.
