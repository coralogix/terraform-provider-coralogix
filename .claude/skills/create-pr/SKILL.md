---
name: create-pr
description: "Use when creating a PR for a non-trivial bug fix in this repo. Assembles a structured PR description following .github/PULL_REQUEST_TEMPLATE.md with expanded sections for root cause, risks, and test plan. Do NOT use for trivial one-liner fixes."
---

# Create PR

**Trigger:** opening a PR for a non-trivial bug fix. Trivial one-liners can keep the default template body as-is.

**Use `.github/PULL_REQUEST_TEMPLATE.md` as the skeleton** — keep every section (Description, Acceptance tests, Release Note, References, Community Note). Don't drop the Community Note or other boilerplate.

**Inside the Description section,** use these `##` headers in order:
1. **Summary** — one sentence: what bug is fixed; link the ticket if any.
2. **Root Cause** — the *mechanism*, not the symptom. Split into "Issue 1 / Issue 2 …" with `file:function` refs if compounding.
3. **Why was it introduced** — cite the introducing commit via `git log --follow` / `git blame`. If unclear, say so; don't speculate from commit-message phrasing.
4. **Breaking change?** — explicit yes/no, with edge cases or migration path.
5. **Risks** — bullets, with mitigation per item.
6. **Changes** — `| File | Change |` table.
7. **Test plan** — checklist with specific test function names.

**Fill the rest of the template:** answer the Acceptance tests checkboxes, paste relevant `make testacc TESTARGS='-run=...'` output, write a meaningful Release Note (`NONE` for chore/internal), link related PRs/issues in References, keep Community Note verbatim.

**Before `gh pr create`:** show the assembled description and wait for explicit approval. PR creation is visible-to-others.

**Why:** reviewers shouldn't redo the archaeology you did. Symptom-and-fix descriptions push the work back onto them.
