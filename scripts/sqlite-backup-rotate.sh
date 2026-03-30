#!/usr/bin/env bash
set -euo pipefail

ENV_FILE="${ENV_FILE:-/opt/h2v2/.env.generated}"
KEEP_DAYS="${KEEP_DAYS:-14}"
PANEL_API_BIN="${PANEL_API_BIN:-/opt/h2v2/bin/panel-api}"
BACKUP_DIR=""
DB_PATH=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --env-file)
      ENV_FILE="$2"
      shift 2
      ;;
    --keep-days)
      KEEP_DAYS="$2"
      shift 2
      ;;
    --bin)
      PANEL_API_BIN="$2"
      shift 2
      ;;
    --db)
      DB_PATH="$2"
      shift 2
      ;;
    --backup-dir)
      BACKUP_DIR="$2"
      shift 2
      ;;
    *)
      printf "[error] Unknown argument: %s\n" "$1" >&2
      exit 1
      ;;
  esac
done

if [[ -f "${ENV_FILE}" ]]; then
  set -a
  # shellcheck disable=SC1090
  source "${ENV_FILE}"
  set +a
fi

if [[ -z "${DB_PATH}" ]]; then
  DB_PATH="${PANEL_SQLITE_PATH:-/var/lib/h2v2/data/h2v2.db}"
fi
if [[ -z "${BACKUP_DIR}" ]]; then
  BACKUP_DIR="${PANEL_STORAGE_ROOT:-/var/lib/h2v2}/backups"
fi

mkdir -p "${BACKUP_DIR}"

STAMP="$(date -u +%Y%m%d-%H%M)"
OUT_FILE="${BACKUP_DIR}/panel-${STAMP}.db"

"${PANEL_API_BIN}" sqlite-backup --db "${DB_PATH}" --out "${OUT_FILE}"

find "${BACKUP_DIR}" -maxdepth 1 -type f -name 'panel-*.db' -mtime "+${KEEP_DAYS}" -delete
