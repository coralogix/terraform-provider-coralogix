# Run migration script for recording_rules_groups_set

Use this to generate Terraform from existing Coralogix recording rules and validate the provider (e.g. after changes to `resource_coralogix_recording_rules_groups_set`).

## Prerequisites

- Terraform, Go 1.23+, Python 3, and `hcl2json` installed (see [generating-terraform.md](./generating-terraform.md))
- `coralogix-management-sdk` cloned (e.g. at `../coralogix-management-sdk` from this repo)

## Steps

**1. Set environment**

```bash
export CORALOGIX_API_KEY="<your-api-key>"
export CORALOGIX_ENV="EU2"
```

Use your cluster (e.g. `EU2`, `US`, `AP2`). The API key must have access to recording rules.

**2. Run the script**

```bash
cd /Users/noya.itzhaki/repos/coralogix-management-sdk/tools/terraform-importer
./generate_and_migrate.sh
```

**3. In the script prompts**

- **Migration type:** `2` (Migrate based on a specific resource name)
- **Resource type:** `10` (recording_rules_groups_set)
- **Provider version:** `~>2.0.0` (or the version you are testing)
- **Apply:** type `yes` to import into state, or skip; `generated.tf` is written either way

**4. Output**

The script creates:

- `./recording_rules_groups_set_migration/generated.tf` – Terraform for existing recording rule group sets
- `./recording_rules_groups_set_migration/provider.tf`
- `./recording_rules_groups_set_migration/terraform.tfstate` (if you applied)

**5. Test a name change with your local provider**

To verify your name-handling fix: use the generated config, change the set-level `name`, then apply with your local provider build. If the fix works, apply succeeds and a second `terraform plan` shows no changes.

**5a. Build the provider (from this repo)**

```bash
mkdir -p /tmp/coralogix-provider
cd /Users/noya.itzhaki/repos/terraform-provider-coralogix
go build -o /tmp/coralogix-provider/terraform-provider-coralogix_2.0.0 .
```

(Use the same minor version as in the migration folder’s `provider.tf` / `.terraform.lock.hcl` if different. If `terraform init -plugin-dir` reports a version mismatch, use [dev overrides](#dev-overrides-alternative) instead.)

**5b. Use the migration folder with the local binary**

```bash
cd /Users/noya.itzhaki/repos/coralogix-management-sdk/tools/terraform-importer/recording_rules_groups_set_migration
terraform init -plugin-dir /tmp/coralogix-provider
```

**5c. Change the set `name` in `generated.tf`**

Edit the top-level `name` attribute of the resource (the recording rule group set name), e.g.:

```hcl
name = "Examplee"
```
to
```hcl
name = "Examplee-renamed"
```

**5d. Plan and apply**

```bash
terraform plan
```

You should see 1 change (in-place update: `name`).

```bash
terraform apply
```

Type `yes` to apply. If your fix is correct, the update succeeds and the backend returns the new name.

**5e. Confirm no drift**

```bash
terraform plan
```

Expected: “No changes.” If you see a change to `name` again, the backend or provider is still not persisting/returning the new name correctly.

---

### Dev overrides alternative

If `-plugin-dir` fails due to version mismatch, use a dev override so Terraform uses your local binary regardless of version:

1. Create `~/.terraformrc` (or set `TF_CLI_CONFIG_FILE`) with:

```hcl
provider_installation {
  dev_overrides {
    "coralogix/coralogix" = "/tmp/coralogix-provider"
  }
}
```

2. Build the provider into that directory:

```bash
mkdir -p /tmp/coralogix-provider
cd /Users/noya.itzhaki/repos/terraform-provider-coralogix
go build -o /tmp/coralogix-provider/terraform-provider-coralogix .
```

3. In the migration folder run `terraform init` (no `-plugin-dir`), then `terraform plan` / `terraform apply`. Terraform will use the binary from `/tmp/coralogix-provider`. Remove the `dev_overrides` block when done testing.
