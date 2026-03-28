#!/usr/bin/env bash
set -euo pipefail

ENV_FILE="${1:-/opt/proxy-panel/.env.generated}"
SCRIPT_PATH="/opt/proxy-panel/current/scripts/smoke-check.sh"

if [[ -x "${SCRIPT_PATH}" ]]; then
  bash "${SCRIPT_PATH}" "${ENV_FILE}"
else
  REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
  bash "${REPO_ROOT}/scripts/smoke-check.sh" "${ENV_FILE}"
fi

