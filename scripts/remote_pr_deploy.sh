#!/usr/bin/env bash

set -euo pipefail

log() {
  printf '[adp-deploy] %s\n' "$*"
}

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    log "missing required command: $1"
    exit 1
  fi
}

require_var() {
  local name="$1"
  if [ -z "${!name:-}" ]; then
    log "missing required environment variable: $name"
    exit 1
  fi
}

qualified_image() {
  local image_name="$1"

  if [ -n "${ADP_IMAGE_REPOSITORY_PREFIX:-}" ]; then
    printf '%s/%s:%s' "${ADP_IMAGE_REPOSITORY_PREFIX}" "${image_name}" "${ADP_IMAGE_TAG}"
    return
  fi

  printf '%s:%s' "${image_name}" "${ADP_IMAGE_TAG}"
}

load_kind_image() {
  local image="$1"
  local cluster_name="${ADP_KIND_CLUSTER_NAME:-kind}"

  require_cmd kind
  kind load docker-image "${image}" --name "${cluster_name}"
}

load_k3s_image() {
  local image="$1"
  local temp_dir
  local archive

  temp_dir="$(mktemp -d)"
  archive="${temp_dir}/image.tar"
  docker save "${image}" -o "${archive}"
  sudo k3s ctr images import "${archive}"
  rm -rf "${temp_dir}"
}

load_containerd_image() {
  local image="$1"
  local temp_dir
  local archive
  local namespace="${ADP_CONTAINERD_NAMESPACE:-k8s.io}"

  require_cmd ctr

  temp_dir="$(mktemp -d)"
  archive="${temp_dir}/image.tar"
  docker save "${image}" -o "${archive}"
  sudo ctr -n "${namespace}" images import "${archive}"
  rm -rf "${temp_dir}"
}

import_images_if_needed() {
  local server_image="$1"
  local worker_image="$2"
  local runtime="${ADP_K8S_RUNTIME:-docker}"

  case "${runtime}" in
    ""|docker)
      log "assuming the Kubernetes runtime can access host-built Docker images"
      ;;
    containerd)
      load_containerd_image "${server_image}"
      load_containerd_image "${worker_image}"
      ;;
    kind)
      load_kind_image "${server_image}"
      load_kind_image "${worker_image}"
      ;;
    k3s)
      load_k3s_image "${server_image}"
      load_k3s_image "${worker_image}"
      ;;
    *)
      log "unsupported ADP_K8S_RUNTIME=${runtime}; supported values: docker, containerd, kind, k3s"
      exit 1
      ;;
  esac
}

require_cmd git
require_cmd docker
require_cmd kubectl
require_var GITHUB_REPOSITORY
require_var DEPLOY_REF
require_var DEPLOY_REPO_DIR

ADP_K8S_ENV_FILE="${ADP_K8S_ENV_FILE:-/etc/adp/adp.env}"
ADP_K8S_NAMESPACE="${ADP_K8S_NAMESPACE:-adp}"

if [ ! -d "${DEPLOY_REPO_DIR}/.git" ]; then
  require_var DEPLOY_REPO_URL
  log "cloning repository into ${DEPLOY_REPO_DIR}"
  git clone "${DEPLOY_REPO_URL}" "${DEPLOY_REPO_DIR}"
fi

if [ -n "${DEPLOY_REPO_URL:-}" ]; then
  git -C "${DEPLOY_REPO_DIR}" remote set-url origin "${DEPLOY_REPO_URL}"
fi

DEPLOY_SOURCE="${DEPLOY_SOURCE:-unknown}"

log "fetching latest revision: ${DEPLOY_REF}"
git -C "${DEPLOY_REPO_DIR}" fetch --prune origin
git -C "${DEPLOY_REPO_DIR}" fetch origin "${DEPLOY_REF}"
git -C "${DEPLOY_REPO_DIR}" checkout --force FETCH_HEAD

RELEASE_FILE="${DEPLOY_REPO_DIR}/deploy/k8s/release.env"
if [ ! -f "${RELEASE_FILE}" ]; then
  log "missing release file: ${RELEASE_FILE}"
  exit 1
fi

# shellcheck disable=SC1090
. "${RELEASE_FILE}"

require_var ADP_IMAGE_TAG

ADP_SERVER_IMAGE_NAME="${ADP_SERVER_IMAGE_NAME:-adp-server}"
ADP_WORKER_IMAGE_NAME="${ADP_WORKER_IMAGE_NAME:-adp-worker}"
ADP_SERVER_DEPLOYMENT_NAME="${ADP_SERVER_DEPLOYMENT_NAME:-adp-server}"
ADP_WORKER_DEPLOYMENT_NAME="${ADP_WORKER_DEPLOYMENT_NAME:-adp-worker}"
ADP_SERVER_CONTAINER_NAME="${ADP_SERVER_CONTAINER_NAME:-adp-server}"
ADP_WORKER_CONTAINER_NAME="${ADP_WORKER_CONTAINER_NAME:-adp-worker}"
ADP_PUSH_IMAGES="${ADP_PUSH_IMAGES:-false}"

SERVER_IMAGE="$(qualified_image "${ADP_SERVER_IMAGE_NAME}")"
WORKER_IMAGE="$(qualified_image "${ADP_WORKER_IMAGE_NAME}")"

log "building ${SERVER_IMAGE}"
docker build \
  -t "${SERVER_IMAGE}" \
  -f "${DEPLOY_REPO_DIR}/deploy/docker-compose/Dockerfile.server" \
  "${DEPLOY_REPO_DIR}"

log "building ${WORKER_IMAGE}"
docker build \
  -t "${WORKER_IMAGE}" \
  -f "${DEPLOY_REPO_DIR}/deploy/docker-compose/Dockerfile.worker" \
  "${DEPLOY_REPO_DIR}"

if [ "${ADP_PUSH_IMAGES}" = "true" ]; then
  log "pushing ${SERVER_IMAGE}"
  docker push "${SERVER_IMAGE}"
  log "pushing ${WORKER_IMAGE}"
  docker push "${WORKER_IMAGE}"
else
  import_images_if_needed "${SERVER_IMAGE}" "${WORKER_IMAGE}"
fi

if [ ! -f "${ADP_K8S_ENV_FILE}" ]; then
  log "missing runtime env file: ${ADP_K8S_ENV_FILE}"
  exit 1
fi

log "ensuring namespace ${ADP_K8S_NAMESPACE}"
kubectl create namespace "${ADP_K8S_NAMESPACE}" --dry-run=client -o yaml | kubectl apply -f -

log "syncing runtime secret from ${ADP_K8S_ENV_FILE}"
kubectl -n "${ADP_K8S_NAMESPACE}" create secret generic adp-runtime \
  --from-env-file="${ADP_K8S_ENV_FILE}" \
  --dry-run=client \
  -o yaml | kubectl apply -f -

log "applying Kubernetes manifests"
kubectl -n "${ADP_K8S_NAMESPACE}" apply -f "${DEPLOY_REPO_DIR}/deploy/k8s/manifests"

log "updating deployment images"
kubectl -n "${ADP_K8S_NAMESPACE}" set image deployment/"${ADP_SERVER_DEPLOYMENT_NAME}" \
  "${ADP_SERVER_CONTAINER_NAME}=${SERVER_IMAGE}"
kubectl -n "${ADP_K8S_NAMESPACE}" set image deployment/"${ADP_WORKER_DEPLOYMENT_NAME}" \
  "${ADP_WORKER_CONTAINER_NAME}=${WORKER_IMAGE}"

kubectl -n "${ADP_K8S_NAMESPACE}" annotate deployment/"${ADP_SERVER_DEPLOYMENT_NAME}" \
  --overwrite adp.io/deploy-source="${DEPLOY_SOURCE}" adp.io/revision="${DEPLOY_SHA:-unknown}"
kubectl -n "${ADP_K8S_NAMESPACE}" annotate deployment/"${ADP_WORKER_DEPLOYMENT_NAME}" \
  --overwrite adp.io/deploy-source="${DEPLOY_SOURCE}" adp.io/revision="${DEPLOY_SHA:-unknown}"

log "waiting for rollout"
kubectl -n "${ADP_K8S_NAMESPACE}" rollout status deployment/"${ADP_SERVER_DEPLOYMENT_NAME}" --timeout=180s
kubectl -n "${ADP_K8S_NAMESPACE}" rollout status deployment/"${ADP_WORKER_DEPLOYMENT_NAME}" --timeout=180s

log "deployment finished successfully"
