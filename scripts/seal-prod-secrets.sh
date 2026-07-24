#!/usr/bin/env bash
set -euo pipefail

# Encrypt locally held production Secret sources with the cluster's public
# certificate. Source files under secrets/prod/ are ignored by Git; only the
# generated SealedSecret manifests may be committed.

cert_path="${SEALED_SECRETS_CERT:-sealed-secrets-cert.pem}"
source_dir="${ADP_SECRET_SOURCE_DIR:-secrets/prod}"
output_dir="deploy/k8s/overlays/prod/secrets"

require_readable() {
  if [[ ! -r "$1" ]]; then
    echo "error: required file is missing or unreadable: $1" >&2
    exit 1
  fi
}

require_readable "$cert_path"
require_readable "$source_dir/postgres.env"
require_readable "$source_dir/adp-server.env"

if command -v kubeseal >/dev/null 2>&1; then
  seal_cmd=(kubeseal)
elif command -v go >/dev/null 2>&1; then
  echo "info: kubeseal not found; using go run kubeseal@v0.27.3" >&2
  seal_cmd=(go run github.com/bitnami-labs/sealed-secrets/cmd/kubeseal@v0.27.3)
else
  echo "error: install kubeseal or Go 1.22+ before sealing Secrets" >&2
  exit 1
fi

secret_yaml() {
  local name="$1" env_file="$2"
  printf '%s\n' 'apiVersion: v1' 'kind: Secret' 'metadata:' "  name: $name" '  namespace: adp' 'type: Opaque' 'data:'
  while IFS='=' read -r key value || [[ -n "$key" ]]; do
    [[ -z "$key" || "$key" == \#* ]] && continue
    if [[ ! "$key" =~ ^[A-Za-z_][A-Za-z0-9_]*$ ]]; then
      echo "error: invalid environment variable name in $env_file: $key" >&2
      exit 1
    fi
    printf '  %s: %s\n' "$key" "$(printf '%s' "$value" | base64 | tr -d '\n')"
  done < "$env_file"
}

seal_one() {
  local name="$1" env_file="$2" destination="$3" temporary
  temporary="$(mktemp "${destination}.tmp.XXXXXX")"
  if ! secret_yaml "$name" "$env_file" | "${seal_cmd[@]}" --cert "$cert_path" --format yaml --secret-file /dev/stdin > "$temporary"; then
    rm -f "$temporary"
    echo "error: failed to generate $destination" >&2
    exit 1
  fi
  mv "$temporary" "$destination"
  echo "generated: $destination" >&2
}

mkdir -p "$output_dir"
seal_one postgres-secret "$source_dir/postgres.env" "$output_dir/sealed-postgres-secret.yaml"
seal_one adp-server-secret "$source_dir/adp-server.env" "$output_dir/sealed-adp-server-secret.yaml"

echo "Generated SealedSecrets in $output_dir. The production kustomization already references both files."
