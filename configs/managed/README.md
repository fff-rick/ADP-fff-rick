# Managed YAML Configs

This directory stores source YAML files for runtime-managed ADP configs.

- `templates/`: command template definitions
- `policies/`: execution policy definitions
- `prompts/`: LLM prompt definitions
- `diagnosis_plans/`: diagnosis plan definitions

Load these files through `/api/v1/configs/{kind}` so the server can validate,
persist, and apply them at runtime.

Examples:

- `templates/disk_usage_check.yaml` -> `/api/v1/configs/templates`
- `policies/default.yaml` -> `/api/v1/configs/policies`
- `prompts/yaml_generator.yaml` -> `/api/v1/configs/prompts`
- `diagnosis_plans/http_unreachable.yaml` -> `/api/v1/configs/diagnosis_plans`
