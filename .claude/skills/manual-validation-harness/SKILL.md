---
name: manual-validation-harness
description: When validating a coralogix terraform-provider bug fix end-to-end against a real Coralogix env. Triggers on "test the fix locally", "reproduce the bug", "validate before merge".
---

# Manual end-to-end validation

**Trigger:** user wants to exercise a provider bug fix against a real env, post-build pre-merge.

**Procedure:**

1. `make install` from the PR branch (builds + installs to `~/.terraform.d/plugins/locally/debug/coralogix/1.5/<arch>`).
2. Isolated workdir: `mkdir /tmp/<ticket>-test && cd $_`.
3. Project-local `terraformrc` with `dev_overrides` for `coralogix/coralogix` pointing at the install dir above. `export TF_CLI_CONFIG_FILE=$PWD/terraformrc`.
4. Multi-step scenarios: keep step files in `./steps/`, `cp steps/stepN.tf main.tf` between runs. (Terraform reads every `.tf` in cwd → loose step files cause duplicate-resource errors.)
5. Each step: `terraform apply -auto-approve`, verify in UI, then `terraform plan` → expect `No changes.` (idempotency catches perpetual-diff regressions).
6. Cleanup: `terraform destroy -auto-approve && rm -rf /tmp/<ticket>-test`.

**Gotchas:**

- 403 from API = key/env mismatch, not a provider bug.
- "Provider development overrides are in effect" warning is expected.
- Don't write `.tf` files via heredoc in chat — paste indentation breaks them. Use an editor or have Claude Write them.

**Why:** unit + acceptance tests catch regressions; manual validation gives the reviewer end-to-end confidence and exposes UI-layer behaviour the test suite can't observe.
