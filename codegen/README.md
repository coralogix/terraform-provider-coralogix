# OpenAPI → Terraform code-generation experiment

This directory is a **throwaway experiment**, not production code. It evaluates
HashiCorp's [Terraform Provider Code Generation](https://developer.hashicorp.com/terraform/plugin/code-generation/openapi-generator)
(`tfplugingen-openapi` + `tfplugingen-framework`) against the split OpenAPI specs
shipped in [`coralogix/coralogix-management-sdk`](https://github.com/coralogix/coralogix-management-sdk)
(`specs/*.json`), for two resources: **alerts** and **parsing rules (rule groups)**.

See [`COMPARISON.md`](./COMPARISON.md) for the findings and the side-by-side with our
current hand-written resources.

## Tooling

```bash
go install github.com/hashicorp/terraform-plugin-codegen-openapi/cmd/tfplugingen-openapi@latest
go install github.com/hashicorp/terraform-plugin-codegen-framework/cmd/tfplugingen-framework@latest
```

## Layout

| Path | What it is |
|------|-----------|
| `alert_definitions_service.json`, `rule_groups_service.json` | Vendored snapshots of the SDK split specs (input). |
| `configs/*.yml` | Generator configs mapping CRUD ops → resource/data-source. |
| `*.flat.json` | Specs after the manual `oneOf`-stripping workaround (see below). |
| `_output/*_provider_code_spec.json` | Provider Code Spec, generated **out of the box** (empty — see findings). |
| `_output/*_provider_code_spec.flat.json` | Provider Code Spec, generated from the flattened specs. |
| `_output/go_alerts/`, `_output/go_parsing_rules/` | Go framework scaffolding from the flattened specs. |

## Reproduce

```bash
cd codegen

# 1. Out of the box — both resources come out EMPTY (oneOf unsupported).
tfplugingen-openapi generate --config configs/alerts_generator_config.yml \
  --output _output/alerts_provider_code_spec.json alert_definitions_service.json
tfplugingen-openapi generate --config configs/parsing_rules_generator_config.yml \
  --output _output/parsing_rules_provider_code_spec.json rule_groups_service.json

# 2. Manual workaround: strip the root-level `oneOf` composition so the generator
#    falls back to the flat property list (see gen_flat_specs.py).
python3 gen_flat_specs.py

# 3. Generate again from the flattened specs.
tfplugingen-openapi generate --config configs/alerts_generator_config.yml \
  --output _output/alerts_provider_code_spec.flat.json alert_definitions_service.flat.json
tfplugingen-openapi generate --config configs/parsing_rules_generator_config.yml \
  --output _output/parsing_rules_provider_code_spec.flat.json rule_groups_service.flat.json

# 4. Emit Go framework scaffolding (schema + models only, no CRUD).
tfplugingen-framework generate resources \
  --input _output/alerts_provider_code_spec.flat.json --output _output/go_alerts --package alerts_generated
tfplugingen-framework generate resources \
  --input _output/parsing_rules_provider_code_spec.flat.json --output _output/go_parsing_rules --package parsing_rules_generated
```
