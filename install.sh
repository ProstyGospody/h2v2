#!/usr/bin/env bash
set -euo pipefail

fatal() {
  printf "[error] %s\n" "$1" >&2
  exit 1
}

require_cmd() {
  local name="$1"
  command -v "${name}" >/dev/null 2>&1 || fatal "${name} is required"
}

REPO_OWNER="${H2V2_REPO_OWNER:-ProstyGospody}"
REPO_NAME="${H2V2_REPO_NAME:-h2v2}"
REPO_REF="${H2V2_REPO_REF:-main}"
ARCHIVE_URL="${H2V2_ARCHIVE_URL:-https://github.com/${REPO_OWNER}/${REPO_NAME}/archive/refs/heads/${REPO_REF}.tar.gz}"

require_cmd curl
require_cmd tar

WORK_DIR="$(mktemp -d /tmp/h2v2-install.XXXXXX)"
cleanup() {
  rm -rf "${WORK_DIR}"
}
trap cleanup EXIT

printf "==> Downloading %s\n" "${ARCHIVE_URL}"
curl -fsSL "${ARCHIVE_URL}" -o "${WORK_DIR}/repo.tar.gz"

printf "==> Extracting installer\n"
tar -xzf "${WORK_DIR}/repo.tar.gz" -C "${WORK_DIR}"

REPO_DIR="$(find "${WORK_DIR}" -mindepth 1 -maxdepth 1 -type d | head -n1)"
[[ -n "${REPO_DIR}" ]] || fatal "failed to resolve extracted repository path"
[[ -f "${REPO_DIR}/deploy/install.sh" ]] || fatal "deploy/install.sh is missing in extracted archive"

if [[ "$(id -u)" -ne 0 ]]; then
  if ! command -v sudo >/dev/null 2>&1; then
    fatal "root access is required; install sudo or run this command as root"
  fi
  exec sudo -E bash "${REPO_DIR}/deploy/install.sh" "$@"
fi

exec bash "${REPO_DIR}/deploy/install.sh" "$@"
