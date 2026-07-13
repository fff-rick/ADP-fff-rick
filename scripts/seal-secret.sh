#!/usr/bin/env bash
# Re-seal the adp-runtime secret after modifying adp-runtime.env
# Prerequisites: kubeseal, kubectl (for dry-run only)
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ENV_FILE="${SCRIPT_DIR}/../deploy/k8s/overlays/prod/adp-runtime.env"
CERT_FILE="${SCRIPT_DIR}/../sealed-secrets-cert.pem"
OUTPUT_FILE="${SCRIPT_DIR}/../deploy/k8s/overlays/prod/sealed-adp-runtime.yaml"

if [ ! -f "${ENV_FILE}" ]; then
  echo "Missing env file: ${ENV_FILE}"
  exit 1
fi

if [ ! -f "${CERT_FILE}" ]; then
  echo "Missing cert file: ${CERT_FILE}"
  echo "Run on the server: kubeseal --fetch-cert > sealed-secrets-cert.pem"
  exit 1
fi

echo "Sealing ${ENV_FILE} -> ${OUTPUT_FILE}"

kubectl create secret generic adp-runtime \
  --from-env-file="${ENV_FILE}" \
  --dry-run=client \
  -o yaml | kubeseal --cert "${CERT_FILE}" --format yaml -n adp -o yaml > "${OUTPUT_FILE}"

echo "Done: ${OUTPUT_FILE}"
