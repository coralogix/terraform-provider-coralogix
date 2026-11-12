---
name: tfgen-sibling-rules-on-parent-validator
description: "Use when plan or validate of a generated resource is slow, or its schema has path.MatchRelative validators inside list items. A sibling rule must run once on the parent object."
---

# Sibling rules run once on the parent object

**Trigger:** `ValidateResourceConfig` time grows faster than the number of widgets or items, a profile shows `Config.PathMatches`, or the generated `schema.go` has `ExactlyOneOf(path.MatchRelative()...)` or `ConflictsWith(path.MatchRelative()...)` on an attribute inside a list item.

**Fix:** Write the rule as a field line (`exactlyOneOf: [other]`, `conflictsWith: [other]`) on a field that is inside an object, and name only siblings. The generator turns it into a group of the `oneOfArmsValidator` of the parent object. A target with `../` and a top-level attribute keep the path form. Check the result: `grep -c MatchRelative internal/provider/generated/<resource>/schema.go` stays small, and the offline validate time on 23 widgets stays near the handwritten one.

**Why:** A path expression walks the whole configuration for each sibling, for each object. The parent validator reads only its own object.
