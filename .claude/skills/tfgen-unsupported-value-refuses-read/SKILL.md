---
name: tfgen-unsupported-value-refuses-read
description: "Use when a generated resource must refuse to read a value that the typed schema cannot show (a deprecated field, an empty oneOf). Covers unsupportedArms, unsupportedIfEmpty, AllowUnsupported."
---

# Refuse to read what the schema cannot show

**Trigger:** The released resource returned an error for a response that holds a value without a typed attribute (a deprecated `query`, a visualization with no arm), and the generated one reads it as null and drops the data.

**Fix:** List the field in `unsupportedArms` with `unsupportedSummary` and `unsupportedDetail` (a plain field works, not only a oneOf arm), or put `unsupportedIfEmpty: {summary, detail}` on an object field. Pass `AllowUnsupported` in the hooks and return true when the resource is managed by a document (`content_json`): that dashboard must still read.

**Why:** Without the check, a read writes a state that hides the data, and the next apply can erase it. The error is the released behavior.
