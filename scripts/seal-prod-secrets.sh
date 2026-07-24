#!/usr/bin/env bash
set -euo pipefail

# Encrypt locally held production Secret sources with the cluster's public
# certificate. Source files under secrets/prod/ are ignored by Git; only the
# generated SealedSecret manifests may be committed.

cert_path="${SEALED_SECRETS_CERT:-sealed-secrets-cert.pem}"
source_dir="${ADP_SECRET_SOURCE_DIR:-secrets/prod}"
output_dir="deploy/k8s/overlays/prod/secrets"

command -v kubectl >/dev/null
command -v kubeseal >/dev/null
test -r "$cert_path"
test -r "$source_dir/postgres.env"
test -r "$source_dir/adp-server.env"
mkdir -p "$output_dir"

kubectl -n adp create secret generic postgres-secret \
  --from-env-file="$source_dir/postgres.env" \
  --dry-run=client -o yaml \
  | kubeseal --cert "$cert_path" --format yaml \
  > "$output_dir/sealed-postgres-secret.yaml"

kubectl -n adp create secret generic adp-server-secret \
  --from-env-file="$source_dir/adp-server.env" \
  --dry-run=client -o yaml \
  | kubeseal --cert "$cert_path" --format yaml \
  > "$output_dir/sealed-adp-server-secret.yaml"

echo "Generated SealedSecrets in $output_dir. Review and add the files to the production kustomization resources."
