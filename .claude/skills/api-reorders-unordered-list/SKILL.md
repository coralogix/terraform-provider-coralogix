---
name: api-reorders-unordered-list
description: "Use when apply fails with 'inconsistent result' where list items swap places (targets[0] now holds targets[1]'s value), or a plan shows reorder-only diffs. Keep prior order when the items match as a set."
---

# API returns an unordered list in a different order

**Trigger:** "Provider produced inconsistent result after apply" where `x[0].id`, `x[1].id`, ... hold the same values as the plan, but at other indexes. Often intermittent: the same fixture passes in one test and fails in another.

**Fix:** When order has no meaning and the attribute is an already-released `ListNestedAttribute`, compare the API list with the prior list before flatten. Use the plan in Create/Update and the state in Read. Compare them as a multiset of the full item content. Same items in a different order: return the prior list. Different items: keep the API order, so the plan shows the drift. Compare full content, not a key, because keys can repeat (the API accepts two targets with the same connector and preset). See `orderTargetsLike` in `internal/provider/notifications/resource_coralogix_global_router.go`.

```go
alignRuleTargets(result.Router.GetRules(), router.Rules) // router = extracted from plan
plan, diags = flattenGlobalRouter(ctx, result.Router)
```

Do not change a released list to `SetNestedAttribute` for this: it needs a new schema version and breaks index references like `targets[0]`. For a new attribute, prefer a set.

**Why:** A Terraform list is ordered, so any order change counts as a change, even when the backend treats the items as a set.
