---
name: acc-test-helper-client-without-provider-meta
description: "Use when an acceptance test panics with 'interface {} is nil, not *clientset.ClientSet' in a CheckDestroy or check helper, or a helper calls testAccProvider.Meta()."
---

# Acceptance-test helpers must build their own client

**Trigger:** A `CheckDestroy` or `Check` helper panics with `interface conversion: interface {} is nil, not *clientset.ClientSet`, often only when the test runs alone (`-run '^TestX$'`) or in its own CI shard, such as a provider-migration job.

**Fix:** Build the client in the helper instead of reading it from `testAccProvider.Meta()`:

```go
cs, err := testAccNewClientSet()
if err != nil {
	return err
}
```

**Why:** `testAccProvider` is the SDKv2 provider, and it is only configured once some earlier test in the same process has used it. A framework-only or `ExternalProviders` test that runs alone never configures it, so `Meta()` is nil.
