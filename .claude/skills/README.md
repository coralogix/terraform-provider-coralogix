# Project skills

Project-specific skills for the Coralogix Terraform provider. See the **Skill maintenance** section in `../../CLAUDE.md` for when to add or update skills.

Each skill is a directory containing a `SKILL.md` file:

```
.claude/skills/
  <skill-name>/
    SKILL.md
```

`SKILL.md` frontmatter:

```markdown
---
name: <kebab-case-name>
description: <one sentence — concrete triggers Claude matches against>
---
```

This directory is intentionally tracked in git so the knowledge base travels with the repo.
