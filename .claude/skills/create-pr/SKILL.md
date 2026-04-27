---
name: create-pr
description: Structure for a non-trivial bug-fix PR description in this repo (compounding issues, schema-level changes, framework-internals fixes). Replaces the default PR template body, which is minimal scaffolding. Trigger phrases include "create a PR", "open a PR", "draft the PR description". For feature PRs, fall back to the team's standard process.
---

# Create PR — bug-fix description structure

**Trigger:** opening a PR for a non-trivial bug fix. Trivial one-liners can keep the default template.

**Sections (use as `##` headers in order, replacing the default template body):**

1. **Summary** — one sentence: what bug is fixed and the link to the ticket if any.
2. **Root Cause** — the *mechanism*, not the symptom. If multiple compounding bugs, split into "Issue 1 / Issue 2 / …" with `file:function` refs and explain how each contributes.
3. **Why was it introduced** — run `git log --follow <file>` and `git blame` the offending lines. Cite the introducing commit + the original intent. If you can't find a clear answer, say so — don't speculate from commit-message phrasing.
4. **Breaking change?** — explicit yes/no. If no, list edge cases where behaviour shifts (e.g., post-import re-plan). If yes, describe the migration path.
5. **Risks** — bullet list. For each, how the fix mitigates it (or why it's acceptable).
6. **Changes** — `| File | Change |` table. One row per touched file.
7. **Test plan** — checklist with specific test function names that exercise the fix; flag existing tests being strengthened.

**Before creating the PR:** show the full assembled description to the user and wait for explicit approval. PR creation is a visible-to-others action — never `gh pr create` without confirmation, even after the description is "done". The user may want to tweak wording, drop a section, or hold the PR.

**Why:** reviewers shouldn't redo the archaeology you already did. This structure makes review fast and leaves a forensic trail for future regression triage. Symptom-and-fix descriptions push the work back onto the reviewer.
