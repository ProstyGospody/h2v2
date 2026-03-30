#!/usr/bin/env bash
set -euo pipefail

MODE="install"
NONINTERACTIVE="${H2V2_NONINTERACTIVE:-0}"
DRY_RUN=0
MIGRATE_TO_SQLITE=0

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

APP_ROOT="/opt/h2v2"
SRC_DIR="${APP_ROOT}/current"
BIN_DIR="${APP_ROOT}/bin"
ENV_FILE="${APP_ROOT}/.env.generated"
CREDENTIALS_FILE="/root/h2v2-initial-admin.txt"
ETC_ROOT="/etc/h2v2"
HY2_DIR="${ETC_ROOT}/hysteria"

PANEL_API_PORT=18080
PANEL_WEB_PORT=13000

BACKUP_DIR=""
BACKUP_READY=0
ROLLBACK_RUNNING=0
CHANGED=()

ENV_OVERRIDE_KEYS=(
  PANEL_PUBLIC_HOST PANEL_PUBLIC_PORT PANEL_ACME_EMAIL SUBSCRIPTION_PUBLIC_HOST
  HY2_DOMAIN HY2_PORT HY2_OBFS_PASSWORD HY2_STATS_PORT
  INITIAL_ADMIN_EMAIL INITIAL_ADMIN_PASSWORD
  PANEL_STORAGE_DRIVER PANEL_SQLITE_PATH PANEL_STORAGE_ROOT PANEL_AUDIT_DIR PANEL_RUNTIME_DIR
)

phase() { printf "\n==> %s\n" "$1"; }
info() { printf "[info] %s\n" "$1"; }
warn() { printf "[warn] %s\n" "$1" >&2; }
fatal() { printf "[error] %s\n" "$1" >&2; exit 1; }
changed() { CHANGED+=("$1"); }

run() {
  if [[ "${DRY_RUN}" -eq 1 ]]; then
    printf "[dry-run] %q " "$@"
    printf "\n"
    return 0
  fi
  "$@"
}

parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --install) MODE="install" ;;
      --reconfigure) MODE="reconfigure" ;;
      --upgrade) MODE="upgrade" ;;
      --migrate-to-sqlite) MIGRATE_TO_SQLITE=1 ;;
      --non-interactive) NONINTERACTIVE=1 ;;
      --dry-run) DRY_RUN=1 ;;
      *) fatal "Unknown argument: $1" ;;
    esac
    shift
  done
}

is_noninteractive() {
  case "${NONINTERACTIVE}" in 1|true|TRUE|yes|YES) return 0 ;; *) return 1 ;; esac
}

require_root() {
  [[ "$(id -u)" -eq 0 ]] || fatal "Run as root: sudo bash ./deploy/install.sh"
}

check_os() {
  [[ -f /etc/os-release ]] || fatal "Cannot detect operating system"
  # shellcheck disable=SC1091
  source /etc/os-release
  case "${ID}" in
    ubuntu) [[ "${VERSION_ID}" == "24.04" ]] || fatal "Ubuntu 24.04 required" ;;
    debian)
      local major="${VERSION_ID%%.*}"
      [[ "${major}" =~ ^[0-9]+$ ]] || fatal "Cannot parse Debian version"
      (( major >= 12 )) || fatal "Debian 12+ required"
      ;;
    *) fatal "Supported OS: Ubuntu 24.04 or Debian 12+" ;;
  esac
}

detect_existing_installation() {
  local env_hit=0 unit_hit=0 dir_hit=0
  [[ -f "${ENV_FILE}" ]] && env_hit=1
  systemctl list-unit-files h2v2-api.service h2v2-web.service hysteria-server.service >/dev/null 2>&1 && unit_hit=1 || true
  [[ -d /var/lib/h2v2 || -d /etc/h2v2 ]] && dir_hit=1
  info "detected: env=${env_hit}, units=${unit_hit}, dirs=${dir_hit}"
}

wait_for_apt_locks() {
  local timeout="${1:-900}" start_ts now elapsed next_log=0
  start_ts="$(date +%s)"
  command -v fuser >/dev/null 2>&1 || return 0
  while true; do
    local held=""
    local lock
    for lock in /var/lib/apt/lists/lock /var/cache/apt/archives/lock /var/lib/dpkg/lock-frontend /var/lib/dpkg/lock; do
      if fuser "${lock}" >/dev/null 2>&1; then
        held="${lock}"
        break
      fi
    done
    [[ -z "${held}" ]] && return 0
    now="$(date +%s)"
    elapsed=$((now - start_ts))
    (( elapsed < timeout )) || fatal "Timed out waiting for apt lock ${held}"
    if (( now >= next_log )); then
      info "waiting for apt/dpkg lock: ${held}"
      next_log=$((now + 15))
    fi
    sleep 3
  done
}

apt_get() {
  local timeout="${APT_LOCK_TIMEOUT:-900}" retry="${APT_RETRY_COUNT:-5}" i
  for ((i=1; i<=retry; i++)); do
    wait_for_apt_locks "${timeout}"
    if run apt-get -o DPkg::Lock::Timeout="${timeout}" "$@"; then
      return 0
    fi
    warn "apt-get failed (attempt ${i}/${retry})"
    sleep 5
  done
  fatal "apt-get failed after ${retry} attempts"
}

version_gte() {
  local required="$1" current="$2"
  [[ "$(printf '%s\n' "${required}" "${current}" | sort -V | head -n1)" == "${required}" ]]
}

install_base_packages() {
  phase "build/install: packages"
  export DEBIAN_FRONTEND=noninteractive
  apt_get update
  apt_get install -y ca-certificates curl git jq rsync unzip tar lsb-release gnupg build-essential pkg-config libssl-dev zlib1g-dev caddy sudo
}

install_go() {
  local need=1 min="1.24.0" ver="${GO_VERSION:-1.24.3}"
  if command -v go >/dev/null 2>&1; then
    local cur
    cur="$(go version | awk '{print $3}' | sed 's/^go//')"
    version_gte "${min}" "${cur}" && need=0
  fi
  (( need == 0 )) && return 0
  phase "build/install: go"
  run curl -fsSL "https://dl.google.com/go/go${ver}.linux-amd64.tar.gz" -o /tmp/go.tgz
  run rm -rf /usr/local/go
  run tar -C /usr/local -xzf /tmp/go.tgz
  run ln -sf /usr/local/go/bin/go /usr/local/bin/go
  run ln -sf /usr/local/go/bin/gofmt /usr/local/bin/gofmt
  changed "go toolchain installed/updated"
}

install_node() {
  local target="20" req="20.19.0" need=1
  if command -v node >/dev/null 2>&1 && command -v npm >/dev/null 2>&1; then
    local cur
    cur="$(node -v | sed 's/^v//')"
    version_gte "${req}" "${cur}" && need=0
  fi
  (( need == 0 )) && return 0
  phase "build/install: node"
  run bash -c "curl -fsSL https://deb.nodesource.com/setup_${target}.x | bash -"
  apt_get install -y nodejs
  changed "node installed/updated"
}

install_hysteria() {
  [[ -x /usr/local/bin/hysteria ]] && return 0
  phase "build/install: hysteria"
  local ver="${HYSTERIA_VERSION:-2.6.5}"
  run curl -fsSL "https://github.com/apernet/hysteria/releases/download/app%2Fv${ver}/hysteria-linux-amd64" -o /tmp/hysteria-linux-amd64
  run install -m 0755 /tmp/hysteria-linux-amd64 /usr/local/bin/hysteria
  changed "hysteria installed"
}

create_users_and_dirs() {
  phase "build/install: users and dirs"
  id -u h2v2 >/dev/null 2>&1 || run useradd --system --home /opt/h2v2 --shell /usr/sbin/nologin h2v2
  id -u hysteria >/dev/null 2>&1 || run useradd --system --home /var/lib/hysteria --shell /usr/sbin/nologin hysteria
  run usermod -a -G h2v2 hysteria || true

  run mkdir -p "${APP_ROOT}" "${BIN_DIR}" "${ETC_ROOT}" "${HY2_DIR}"
  run mkdir -p /var/lib/h2v2 /var/lib/h2v2/backups /var/lib/h2v2/data /var/log/h2v2/audit /var/lib/hysteria /run/h2v2 /run/h2v2/locks /run/h2v2/tmp
  run chown -R h2v2:h2v2 /var/lib/h2v2 /var/log/h2v2 /run/h2v2
  run chown -R hysteria:hysteria /var/lib/hysteria
  run chmod 0750 /run/h2v2 /run/h2v2/locks /run/h2v2/tmp
  run chown root:h2v2 "${HY2_DIR}"
  run chmod 2770 "${HY2_DIR}"
}

sync_source() {
  phase "build/install: source sync"
  run mkdir -p "${SRC_DIR}"
  run rsync -a --delete --exclude '.git' "${REPO_ROOT}/" "${SRC_DIR}/"
  run chmod +x "${SRC_DIR}/scripts/smoke-check.sh" "${SRC_DIR}/scripts/sync-hysteria-cert.sh" "${SRC_DIR}/scripts/sqlite-backup-rotate.sh" "${SRC_DIR}/deploy/install.sh" "${SRC_DIR}/deploy/ubuntu24-host-install.sh"
}

build_backend() {
  phase "build/install: backend"
  (( DRY_RUN == 1 )) && return 0
  export PATH="/usr/local/go/bin:${PATH}"
  pushd "${SRC_DIR}" >/dev/null
  GOFLAGS="-mod=mod" go mod download
  GOFLAGS="-mod=mod" go build -ldflags "-s -w" -o "${BIN_DIR}/panel-api" ./cmd/panel-api
  popd >/dev/null
  run chown root:h2v2 "${BIN_DIR}/panel-api"
  run chmod 0750 "${BIN_DIR}/panel-api"
  changed "panel-api rebuilt"
}

build_frontend() {
  phase "build/install: frontend"
  (( DRY_RUN == 1 )) && return 0
  local api_target="${PANEL_API_INTERNAL_URL:-http://127.0.0.1:${PANEL_API_PORT}}"
  local csrf_cookie="${CSRF_COOKIE_NAME:-pp_csrf}"
  local csrf_header="${CSRF_HEADER_NAME:-X-CSRF-Token}"
  pushd "${SRC_DIR}/web" >/dev/null
  npm install --no-audit --no-fund
  VITE_API_PROXY_TARGET="${api_target}" VITE_CSRF_COOKIE_NAME="${csrf_cookie}" VITE_CSRF_HEADER_NAME="${csrf_header}" npm run build
  popd >/dev/null
  run chown -R h2v2:h2v2 "${SRC_DIR}/web"
  changed "panel-web rebuilt"
}

capture_env_overrides() {
  local key
  for key in "${ENV_OVERRIDE_KEYS[@]}"; do
    if [[ "${!key+x}" == "x" ]]; then
      printf -v "__OVERRIDE_${key}" '%s' "${!key}"
    fi
  done
}

load_existing_env() {
  load_kv_file "${ENV_FILE}"
  load_kv_file "${CREDENTIALS_FILE}"
  if [[ -z "${HY2_STATS_PORT:-}" && -n "${HY2_STATS_URL:-}" && "${HY2_STATS_URL}" =~ :([0-9]{1,5})$ ]]; then
    HY2_STATS_PORT="${BASH_REMATCH[1]}"
  fi
}

apply_env_overrides() {
  local key name override_value
  for key in "${ENV_OVERRIDE_KEYS[@]}"; do
    name="__OVERRIDE_${key}"
    override_value="${!name-__H2V2_UNSET__}"
    if [[ "${override_value}" != "__H2V2_UNSET__" ]]; then
      printf -v "${key}" '%s' "${override_value}"
    fi
  done
}

load_kv_file() {
  local file="$1"
  [[ -f "${file}" ]] || return 0

  local line key value
  while IFS= read -r line || [[ -n "${line}" ]]; do
    line="${line%$'\r'}"
    [[ -z "${line}" ]] && continue
    [[ "${line}" =~ ^[[:space:]]*# ]] && continue
    [[ "${line}" == *"="* ]] || continue

    key="${line%%=*}"
    value="${line#*=}"

    key="${key#"${key%%[![:space:]]*}"}"
    key="${key%"${key##*[![:space:]]}"}"

    [[ "${key}" =~ ^[A-Za-z_][A-Za-z0-9_]*$ ]] || continue
    printf -v "${key}" '%s' "${value}"
  done < "${file}"
}

prompt_value() {
  local var="$1" text="$2" default="$3"
  local current="${!var:-}"
  if [[ -n "${current}" && "${MODE}" != "reconfigure" ]]; then return; fi
  if is_noninteractive; then
    [[ -n "${current}" ]] && return
    [[ -n "${default}" ]] || fatal "Value required: ${var}"
    printf -v "${var}" '%s' "${default}"
    return
  fi
  local answer=""
  [[ -n "${current}" ]] && default="${current}"
  if [[ -n "${default}" ]]; then
    read -r -p "${text} [${default}]: " answer
    answer="${answer:-${default}}"
  else
    read -r -p "${text}: " answer
  fi
  [[ -n "${answer}" ]] || fatal "Value required: ${var}"
  printf -v "${var}" '%s' "${answer}"
}

prompt_password() {
  local var="$1" text="$2" current="${!var:-}" answer=""
  if [[ -n "${current}" && "${MODE}" != "reconfigure" ]]; then return; fi
  if is_noninteractive; then
    [[ -n "${current}" ]] || printf -v "${var}" '%s' "$(openssl rand -base64 24 | tr -d '=+/\n' | cut -c1-20)"
    return
  fi
  read -r -s -p "${text} (leave empty to auto-generate): " answer
  echo
  [[ -n "${answer}" ]] || answer="$(openssl rand -base64 24 | tr -d '=+/\n' | cut -c1-20)"
  printf -v "${var}" '%s' "${answer}"
}

generate_if_empty() {
  local var="$1" bytes="$2"
  [[ -n "${!var:-}" ]] || printf -v "${var}" '%s' "$(openssl rand -hex "${bytes}")"
}

normalize_driver() {
  local v
  v="$(echo "${1:-file}" | tr '[:upper:]' '[:lower:]' | xargs)"
  [[ "${v}" == "sqlite" ]] && echo "sqlite" || echo "file"
}

collect_config() {
  phase "config render: collect"
  local host
  host="$(hostname -f 2>/dev/null || hostname)"
  prompt_value PANEL_PUBLIC_HOST "Panel public domain or IP" "${PANEL_PUBLIC_HOST:-${host}}"
  prompt_value PANEL_PUBLIC_PORT "Panel HTTPS port" "${PANEL_PUBLIC_PORT:-8443}"
  prompt_value PANEL_ACME_EMAIL "ACME email" "${PANEL_ACME_EMAIL:-admin@${PANEL_PUBLIC_HOST}}"
  prompt_value SUBSCRIPTION_PUBLIC_HOST "Subscription public host" "${SUBSCRIPTION_PUBLIC_HOST:-${PANEL_PUBLIC_HOST}}"
  prompt_value HY2_DOMAIN "Hysteria public domain" "${HY2_DOMAIN:-${PANEL_PUBLIC_HOST}}"
  prompt_value HY2_PORT "Hysteria UDP port" "${HY2_PORT:-443}"
  prompt_value HY2_STATS_PORT "Hysteria local stats port" "${HY2_STATS_PORT:-8999}"
  prompt_value INITIAL_ADMIN_EMAIL "Initial admin email" "${INITIAL_ADMIN_EMAIL:-admin@${PANEL_PUBLIC_HOST}}"
  prompt_password INITIAL_ADMIN_PASSWORD "Initial admin password"

  generate_if_empty INTERNAL_AUTH_TOKEN 32
  generate_if_empty HY2_STATS_SECRET 32
  generate_if_empty HY2_OBFS_PASSWORD 16

  APP_ENV="${APP_ENV:-production}"
  PANEL_API_LISTEN_ADDR="127.0.0.1:${PANEL_API_PORT}"
  PANEL_API_INTERNAL_URL="http://127.0.0.1:${PANEL_API_PORT}"
  PANEL_PUBLIC_URL="https://${PANEL_PUBLIC_HOST}:${PANEL_PUBLIC_PORT}"
  SUBSCRIPTION_PUBLIC_URL="https://${SUBSCRIPTION_PUBLIC_HOST}:${PANEL_PUBLIC_PORT}"
  HY2_STATS_URL="http://127.0.0.1:${HY2_STATS_PORT}"
  PANEL_STORAGE_ROOT="${PANEL_STORAGE_ROOT:-/var/lib/h2v2}"
  PANEL_AUDIT_DIR="${PANEL_AUDIT_DIR:-/var/log/h2v2/audit}"
  PANEL_RUNTIME_DIR="${PANEL_RUNTIME_DIR:-/run/h2v2}"
  PANEL_STORAGE_DRIVER="$(normalize_driver "${PANEL_STORAGE_DRIVER:-file}")"
  (( MIGRATE_TO_SQLITE == 1 )) && PANEL_STORAGE_DRIVER="sqlite"
  PANEL_SQLITE_PATH="${PANEL_SQLITE_PATH:-${PANEL_STORAGE_ROOT}/data/h2v2.db}"
  SESSION_COOKIE_NAME="${SESSION_COOKIE_NAME:-pp_session}"
  CSRF_COOKIE_NAME="${CSRF_COOKIE_NAME:-pp_csrf}"
  CSRF_HEADER_NAME="${CSRF_HEADER_NAME:-X-CSRF-Token}"
  SESSION_TTL="${SESSION_TTL:-24h}"
  SECURE_COOKIES="${SECURE_COOKIES:-true}"
  HY2_POLL_INTERVAL="${HY2_POLL_INTERVAL:-20s}"
  SERVICE_POLL_INTERVAL="${SERVICE_POLL_INTERVAL:-60s}"
  MANAGED_SERVICES="h2v2-api,h2v2-web,hysteria-server"
  SYSTEMCTL_PATH="/usr/bin/systemctl"
  SUDO_PATH="/usr/bin/sudo"
  JOURNALCTL_PATH="/usr/bin/journalctl"
  SERVICE_LOG_LINES_MAX=120
  AUTH_RATE_LIMIT_WINDOW=15m
  AUTH_RATE_LIMIT_BURST=10
  HY2_BINARY_PATH=/usr/local/bin/hysteria
  HY2_CONFIG_PATH="${HY2_DIR}/server.yaml"
  HY2_CERT_PATH="${HY2_DIR}/tls.crt"
  HY2_KEY_PATH="${HY2_DIR}/tls.key"
}

validate_config() {
  [[ -n "${PANEL_PUBLIC_HOST:-}" && -n "${SUBSCRIPTION_PUBLIC_HOST:-}" && -n "${HY2_DOMAIN:-}" ]] || fatal "public hosts and HY2_DOMAIN are required"
  [[ "${PANEL_PUBLIC_PORT}" != "${PANEL_API_PORT}" && "${PANEL_PUBLIC_PORT}" != "${PANEL_WEB_PORT}" ]] || fatal "PANEL_PUBLIC_PORT conflicts with internal port"
  PANEL_STORAGE_DRIVER="$(normalize_driver "${PANEL_STORAGE_DRIVER}")"
}

write_env_files() {
  phase "config render: write env"
  (( DRY_RUN == 1 )) && return 0
  cat > "${ENV_FILE}" <<EOF
APP_ENV=${APP_ENV}
PANEL_API_LISTEN_ADDR=${PANEL_API_LISTEN_ADDR}
PANEL_API_PORT=${PANEL_API_PORT}
PANEL_WEB_PORT=${PANEL_WEB_PORT}
PANEL_PUBLIC_HOST=${PANEL_PUBLIC_HOST}
PANEL_PUBLIC_PORT=${PANEL_PUBLIC_PORT}
PANEL_PUBLIC_URL=${PANEL_PUBLIC_URL}
SUBSCRIPTION_PUBLIC_HOST=${SUBSCRIPTION_PUBLIC_HOST}
SUBSCRIPTION_PUBLIC_URL=${SUBSCRIPTION_PUBLIC_URL}
PANEL_API_INTERNAL_URL=${PANEL_API_INTERNAL_URL}
PANEL_ACME_EMAIL=${PANEL_ACME_EMAIL}
PANEL_STORAGE_DRIVER=${PANEL_STORAGE_DRIVER}
PANEL_STORAGE_ROOT=${PANEL_STORAGE_ROOT}
PANEL_SQLITE_PATH=${PANEL_SQLITE_PATH}
PANEL_AUDIT_DIR=${PANEL_AUDIT_DIR}
PANEL_RUNTIME_DIR=${PANEL_RUNTIME_DIR}

SESSION_COOKIE_NAME=${SESSION_COOKIE_NAME}
CSRF_COOKIE_NAME=${CSRF_COOKIE_NAME}
CSRF_HEADER_NAME=${CSRF_HEADER_NAME}
SESSION_TTL=${SESSION_TTL}
SECURE_COOKIES=${SECURE_COOKIES}
INITIAL_ADMIN_EMAIL=${INITIAL_ADMIN_EMAIL}

INTERNAL_AUTH_TOKEN=${INTERNAL_AUTH_TOKEN}

HY2_DOMAIN=${HY2_DOMAIN}
HY2_PORT=${HY2_PORT}
HY2_STATS_PORT=${HY2_STATS_PORT}
HY2_CONFIG_PATH=${HY2_CONFIG_PATH}
HY2_CERT_PATH=${HY2_CERT_PATH}
HY2_KEY_PATH=${HY2_KEY_PATH}
HY2_STATS_URL=${HY2_STATS_URL}
HY2_STATS_SECRET=${HY2_STATS_SECRET}
HY2_OBFS_PASSWORD=${HY2_OBFS_PASSWORD}
HY2_POLL_INTERVAL=${HY2_POLL_INTERVAL}

SERVICE_POLL_INTERVAL=${SERVICE_POLL_INTERVAL}
MANAGED_SERVICES=${MANAGED_SERVICES}
SYSTEMCTL_PATH=${SYSTEMCTL_PATH}
SUDO_PATH=${SUDO_PATH}
JOURNALCTL_PATH=${JOURNALCTL_PATH}
SERVICE_LOG_LINES_MAX=${SERVICE_LOG_LINES_MAX}

AUTH_RATE_LIMIT_WINDOW=${AUTH_RATE_LIMIT_WINDOW}
AUTH_RATE_LIMIT_BURST=${AUTH_RATE_LIMIT_BURST}

HY2_BINARY_PATH=${HY2_BINARY_PATH}
EOF
  run chown root:h2v2 "${ENV_FILE}"
  run chmod 0640 "${ENV_FILE}"
  cat > "${CREDENTIALS_FILE}" <<EOF
INITIAL_ADMIN_EMAIL=${INITIAL_ADMIN_EMAIL}
INITIAL_ADMIN_PASSWORD=${INITIAL_ADMIN_PASSWORD}
PANEL_PUBLIC_URL=${PANEL_PUBLIC_URL}
SUBSCRIPTION_PUBLIC_URL=${SUBSCRIPTION_PUBLIC_URL}
EOF
  run chmod 0600 "${CREDENTIALS_FILE}"
  cat > /etc/caddy/h2v2.env <<EOF
PANEL_PUBLIC_HOST=${PANEL_PUBLIC_HOST}
PANEL_PUBLIC_PORT=${PANEL_PUBLIC_PORT}
PANEL_API_PORT=${PANEL_API_PORT}
PANEL_WEB_PORT=${PANEL_WEB_PORT}
PANEL_ACME_EMAIL=${PANEL_ACME_EMAIL}
SUBSCRIPTION_PUBLIC_HOST=${SUBSCRIPTION_PUBLIC_HOST}
HY2_DOMAIN=${HY2_DOMAIN}
EOF
  run chmod 0640 /etc/caddy/h2v2.env
  run mkdir -p /etc/systemd/system/caddy.service.d
  cat > /etc/systemd/system/caddy.service.d/h2v2-env.conf <<'EOF'
[Service]
EnvironmentFile=/etc/caddy/h2v2.env
EOF
  changed "env/config generated"
}

render_runtime_configs() {
  phase "config render: templates"
  (( DRY_RUN == 1 )) && return 0
  run install -m 0644 "${SRC_DIR}/config/templates/Caddyfile.tmpl" /etc/caddy/Caddyfile
  cat > "${HY2_DIR}/server.yaml" <<EOF
listen: :${HY2_PORT}
tls:
  cert: ${HY2_CERT_PATH}
  key: ${HY2_KEY_PATH}
auth:
  type: userpass
  userpass:
    __bootstrap__: ${INTERNAL_AUTH_TOKEN}
trafficStats:
  listen: 127.0.0.1:${HY2_STATS_PORT}
  secret: ${HY2_STATS_SECRET}
obfs:
  type: salamander
  salamander:
    password: ${HY2_OBFS_PASSWORD}
EOF
  run chown root:h2v2 "${HY2_DIR}/server.yaml"
  run chmod 0660 "${HY2_DIR}/server.yaml"
  changed "runtime templates rendered"
}

install_sudoers_and_units() {
  phase "build/install: sudoers + systemd"
  (( DRY_RUN == 1 )) && return 0
  cat > /etc/sudoers.d/h2v2-api <<'EOF'
Cmnd_Alias H2V2_SHOW = /usr/bin/systemctl show h2v2-api --property=ActiveState --property=SubState --property=MainPID --property=ActiveEnterTimestamp, /usr/bin/systemctl show h2v2-web --property=ActiveState --property=SubState --property=MainPID --property=ActiveEnterTimestamp, /usr/bin/systemctl show hysteria-server --property=ActiveState --property=SubState --property=MainPID --property=ActiveEnterTimestamp
Cmnd_Alias H2V2_RESTART = /usr/bin/systemctl restart h2v2-api, /usr/bin/systemctl restart h2v2-web, /usr/bin/systemctl restart hysteria-server
Cmnd_Alias H2V2_RELOAD = /usr/bin/systemctl reload h2v2-api, /usr/bin/systemctl reload h2v2-web, /usr/bin/systemctl reload hysteria-server
Cmnd_Alias H2V2_LOGS = /usr/bin/journalctl -u h2v2-api -n * --no-pager --output=short-iso, /usr/bin/journalctl -u h2v2-web -n * --no-pager --output=short-iso, /usr/bin/journalctl -u hysteria-server -n * --no-pager --output=short-iso
h2v2 ALL=(root) NOPASSWD: H2V2_SHOW, H2V2_RESTART, H2V2_RELOAD, H2V2_LOGS
EOF
  run chmod 0440 /etc/sudoers.d/h2v2-api
  run visudo -cf /etc/sudoers.d/h2v2-api >/dev/null
  run install -m 0644 "${SRC_DIR}/systemd/h2v2-api.service" /etc/systemd/system/h2v2-api.service
  run install -m 0644 "${SRC_DIR}/systemd/h2v2-web.service" /etc/systemd/system/h2v2-web.service
  run install -m 0644 "${SRC_DIR}/systemd/hysteria-server.service" /etc/systemd/system/hysteria-server.service
  run systemctl daemon-reload
  changed "sudoers and units updated"
}

create_backup() {
  phase "backup"
  local stamp
  stamp="$(date -u +%Y%m%d-%H%M%S)"
  BACKUP_DIR="/var/lib/h2v2/backups/install-${stamp}"
  if (( DRY_RUN == 1 )); then
    info "backup path: ${BACKUP_DIR}"
    BACKUP_READY=1
    return
  fi
  run mkdir -p "${BACKUP_DIR}/env" "${BACKUP_DIR}/etc" "${BACKUP_DIR}/systemd" "${BACKUP_DIR}/storage" "${BACKUP_DIR}/audit"
  [[ -f "${ENV_FILE}" ]] && run cp -a "${ENV_FILE}" "${BACKUP_DIR}/env/.env.generated"
  [[ -f "${CREDENTIALS_FILE}" ]] && run cp -a "${CREDENTIALS_FILE}" "${BACKUP_DIR}/env/credentials.txt"
  [[ -d "${ETC_ROOT}" ]] && run rsync -a "${ETC_ROOT}/" "${BACKUP_DIR}/etc/h2v2/"
  [[ -f /etc/caddy/h2v2.env ]] && run cp -a /etc/caddy/h2v2.env "${BACKUP_DIR}/etc/caddy.env"
  for u in h2v2-api.service h2v2-web.service hysteria-server.service; do
    [[ -f "/etc/systemd/system/${u}" ]] && run cp -a "/etc/systemd/system/${u}" "${BACKUP_DIR}/systemd/${u}"
  done
  [[ -d /var/lib/h2v2/state ]] && run rsync -a /var/lib/h2v2/state/ "${BACKUP_DIR}/storage/state/"
  [[ -d /var/lib/h2v2/snapshots ]] && run rsync -a /var/lib/h2v2/snapshots/ "${BACKUP_DIR}/storage/snapshots/"
  [[ -d /var/lib/h2v2/data ]] && run rsync -a /var/lib/h2v2/data/ "${BACKUP_DIR}/storage/data/"
  [[ -d /var/log/h2v2/audit ]] && run rsync -a /var/log/h2v2/audit/ "${BACKUP_DIR}/audit/"
  BACKUP_READY=1
  changed "backup created at ${BACKUP_DIR}"
}

rollback_from_backup() {
  (( BACKUP_READY == 1 )) || return
  (( DRY_RUN == 1 )) && { warn "dry-run: rollback skipped"; return; }
  warn "rollback from ${BACKUP_DIR}"
  ROLLBACK_RUNNING=1
  [[ -f "${BACKUP_DIR}/env/.env.generated" ]] && run cp -a "${BACKUP_DIR}/env/.env.generated" "${ENV_FILE}"
  [[ -f "${BACKUP_DIR}/env/credentials.txt" ]] && run cp -a "${BACKUP_DIR}/env/credentials.txt" "${CREDENTIALS_FILE}"
  [[ -d "${BACKUP_DIR}/etc/h2v2" ]] && run rsync -a --delete "${BACKUP_DIR}/etc/h2v2/" "${ETC_ROOT}/"
  [[ -f "${BACKUP_DIR}/etc/caddy.env" ]] && run cp -a "${BACKUP_DIR}/etc/caddy.env" /etc/caddy/h2v2.env
  for u in h2v2-api.service h2v2-web.service hysteria-server.service; do
    [[ -f "${BACKUP_DIR}/systemd/${u}" ]] && run cp -a "${BACKUP_DIR}/systemd/${u}" "/etc/systemd/system/${u}"
  done
  [[ -d "${BACKUP_DIR}/storage/state" ]] && run rsync -a --delete "${BACKUP_DIR}/storage/state/" /var/lib/h2v2/state/
  [[ -d "${BACKUP_DIR}/storage/snapshots" ]] && run rsync -a --delete "${BACKUP_DIR}/storage/snapshots/" /var/lib/h2v2/snapshots/
  [[ -d "${BACKUP_DIR}/storage/data" ]] && run rsync -a --delete "${BACKUP_DIR}/storage/data/" /var/lib/h2v2/data/
  [[ -d "${BACKUP_DIR}/audit" ]] && run rsync -a --delete "${BACKUP_DIR}/audit/" /var/log/h2v2/audit/
  run systemctl daemon-reload
  run systemctl restart h2v2-api.service h2v2-web.service caddy.service hysteria-server.service || true
}

on_error() {
  local line="$1"
  warn "installer failed at line ${line}"
  (( ROLLBACK_RUNNING == 0 )) && rollback_from_backup
  exit 1
}

run_migration_if_needed() {
  (( MIGRATE_TO_SQLITE == 1 )) || return 0
  [[ "${PANEL_STORAGE_DRIVER}" == "sqlite" ]] || { warn "migrate requested but driver=${PANEL_STORAGE_DRIVER}; skip"; return 0; }
  phase "migrate: file -> sqlite"
  (( DRY_RUN == 1 )) && return 0
  run runuser -u h2v2 -- "${BIN_DIR}/panel-api" migrate-to-sqlite --db "${PANEL_SQLITE_PATH}" --storage-root "${PANEL_STORAGE_ROOT}" --audit-dir "${PANEL_AUDIT_DIR}" --runtime-dir "${PANEL_RUNTIME_DIR}"
  changed "sqlite migration executed"
}

bootstrap_admin() {
  phase "finalize: bootstrap admin"
  (( DRY_RUN == 1 )) && return 0
  set -a
  # shellcheck disable=SC1090
  source "${ENV_FILE}"
  set +a
  run runuser -u h2v2 -- "${BIN_DIR}/panel-api" bootstrap-admin --email "${INITIAL_ADMIN_EMAIL}" --password "${INITIAL_ADMIN_PASSWORD}"
}

restart_services() {
  phase "finalize: restart services"
  local s
  for s in h2v2-api h2v2-web caddy hysteria-server; do
    run systemctl enable "${s}.service"
  done
  run systemctl restart h2v2-api.service
  run systemctl restart h2v2-web.service
  run systemctl restart caddy.service
  (( DRY_RUN == 0 )) && run bash "${SRC_DIR}/scripts/sync-hysteria-cert.sh" "${ENV_FILE}" --wait
  run systemctl restart hysteria-server.service
}

health_checks() {
  phase "health checks"
  (( DRY_RUN == 1 )) && return 0
  run systemctl is-active --quiet h2v2-api.service
  run systemctl is-active --quiet h2v2-web.service
  run systemctl is-active --quiet caddy.service
  run systemctl is-active --quiet hysteria-server.service
  run curl -fsS "http://127.0.0.1:${PANEL_API_PORT}/healthz" >/dev/null
  SMOKE_ADMIN_EMAIL="${INITIAL_ADMIN_EMAIL}" SMOKE_ADMIN_PASSWORD="${INITIAL_ADMIN_PASSWORD}" run bash "${SRC_DIR}/scripts/smoke-check.sh" "${ENV_FILE}"
}

run_build_phase() {
  if [[ "${MODE}" == "reconfigure" ]]; then
    create_users_and_dirs
    sync_source
    return
  fi
  install_base_packages
  install_go
  install_node
  install_hysteria
  create_users_and_dirs
  sync_source
  build_backend
  build_frontend
}

summary() {
  phase "summary"
  printf "changed:\n"
  if [[ "${#CHANGED[@]}" -eq 0 ]]; then
    printf "  - none\n"
  else
    local item
    for item in "${CHANGED[@]}"; do printf "  - %s\n" "${item}"; done
  fi
  printf "backup: %s\n" "${BACKUP_DIR:-not-created}"
  printf "active storage driver: %s\n" "${PANEL_STORAGE_DRIVER:-file}"
  printf "\nverify:\n"
  printf "  systemctl status h2v2-api h2v2-web hysteria-server caddy\n"
  printf "  bash %s/scripts/smoke-check.sh %s\n" "${SRC_DIR}" "${ENV_FILE}"
  printf "\nrollback:\n"
  printf "  rsync -a --delete %s/storage/state/ /var/lib/h2v2/state/\n" "${BACKUP_DIR:-/path/to/backup}"
  printf "  cp %s/env/.env.generated %s\n" "${BACKUP_DIR:-/path/to/backup}" "${ENV_FILE}"
}

main() {
  parse_args "$@"
  trap 'on_error $LINENO' ERR
  phase "preflight checks"
  require_root
  check_os
  detect_existing_installation
  create_backup
  run_build_phase
  capture_env_overrides
  load_existing_env
  apply_env_overrides
  collect_config
  validate_config
  write_env_files
  render_runtime_configs
  install_sudoers_and_units
  run_migration_if_needed
  bootstrap_admin
  restart_services
  health_checks
  summary
}

main "$@"
