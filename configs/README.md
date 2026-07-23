# ADP Configs

All service configuration files live under this directory.

```text
configs/
  server/
    adp.yaml
    adp.yaml.example
  worker/
    adp.yaml
    adp.yaml.example
  ai/
    ai_context.yaml
  env/
    app.env.example
  managed/
    templates/
    policies/
    prompts/
    diagnosis_plans/
```

- `server/adp.yaml`: server startup config, auto-loaded by `adp-server serve`; includes HTTP address and worker gRPC listen address.
- `worker/adp.yaml`: worker startup config, auto-loaded by `adp-worker run`; includes the worker gRPC server address to connect to.
- `ai/ai_context.yaml`: service profiles injected into prompts. All services use the same shape: `name`, `type`, `host`, `port`, `user`, `password_ref`, `logs`, `configs`, and `extra`.
- `env/app.env.example`: environment variable reference.
- `managed/`: source YAMLs that should be loaded through `/api/v1/configs/{kind}`.

The startup YAML files and managed runtime YAML files intentionally keep separate
schemas because they configure different resources. The AI context is unified
because it describes multiple services with the same reusable fields.
