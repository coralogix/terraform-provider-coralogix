---
name: protocol-baseline-record-on-master
description: "Use when adding a protocol-baseline golden harness for a resource switch to generated code. Record goldens on master's handwritten resource first so the switch commit's golden diff shows real behavior changes."
---

# Record protocol baselines on master first

**Trigger:** switching a resource to generated code and adding `testdata/<resource>_baseline/` goldens.

**Fix:** On `origin/master`, add only the baseline test, fake, and harness wiring. Run `UPDATE_GOLDEN=1 go test ./internal/provider -run Test<Resource>Baseline`, and commit that first. Rebase the generate/switch commits on top so the golden diff is the user-visible protocol change.

**Why:** Goldens recorded after the switch describe the new code and cannot show what changed. Reviewers reproduce this by copying the baseline tests onto master and watching scenarios fail.
