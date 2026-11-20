---
name: generated-empty-list-in-set-element
description: "Use when a generated resource fails with 'Provider produced inconsistent result' on a set element, and the element holds a list that the API returns as []. Covers the null read and the SizeAtLeast(1) rule."
---

# An empty list inside a set element cannot be restored from a prior value

**Trigger:** `planned set element ... does not correlate with any element in actual`, where the planned element has `filters = null` and the API returns `"filters": []`.

**Rule (new resources, no key):** a list or set below the root reads an empty response as null (`nestedEmptyReadsNull`). The schema adds `SizeAtLeast(1)` to a list inside a set element (`emptyListInSetItem`), because a configured `[]` cannot round-trip there: a set element has no prior element to copy the form from.

**Released resource:** the rule does not apply, because a user can have configured `[]`. Add `readEmptyAs: "null"` on the field line. It works for lists of strings, numbers, times, and objects.

**Why:** proto3 cannot tell an empty list from a missing one, and `keepPriorEmpty` matches elements of a list or an object by index, not elements of a set.
