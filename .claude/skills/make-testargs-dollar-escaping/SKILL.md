---
name: make-testargs-dollar-escaping
description: "Use when a CI acceptance job running `make testacc` fails with 'Syntax error: Unterminated quoted string', or a -run pattern ending in `$` passes through TESTARGS. Double each `$` for make."
---

# A `$` in a `make` variable must be doubled

**Trigger:** A job passes a `-run` pattern through `make testacc "TESTARGS=-run '<pattern>'"`, and the pattern contains `$`. The log shows the command without the end of the pattern and `/bin/sh: 1: Syntax error: Unterminated quoted string`.

**Fix:** Pass the pattern in an env var, and double every `$` before it reaches `make`:

```yaml
        env:
          RUN_PATTERN: ${{ matrix.pattern }}
        run: make testacc "TESTARGS=-run '${RUN_PATTERN//\$/\$\$}'"
```

Check the result with `make -n testacc "TESTARGS=..."`. A pattern checked with `go test` directly does not prove that it works through `make`.

**Why:** `make` expands `$x` in a variable value. In `'^TestX$'`, it reads `$'` as an empty variable, so the closing quote disappears. `$$` reaches the shell as one `$`.
