#!/usr/bin/env bash
set -euo pipefail

MODE="install"
NONINTERACTIVE="${H2V2_NONINTERACTIVE:-0}"
DRY_RUN=0

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

APP_ROOT="/opt/h2v2"
SRC_DIR="${APP_ROOT}/current"
BIN_DIR="${APP_ROOT}/bin"
ENV_FILE="${APP_ROOT}/.env.generated"
CREDENTIALS_FILE="/root/h2v2-initial-admin.txt"
ETC_ROOT="/etc/h2v2"
HY2_DIR="${ETC_ROOT}/hysteria"
SINGBOX_DIR="${ETC_ROOT}/sing-box"

PANEL_API_PORT=18080
SINGBOX_SERVICE_NAME="${SINGBOX_SERVICE_NAME:-sing-box}"

BACKUP_DIR=""
BACKUP_READY=0
ROLLBACK_RUNNING=0
CHANGED=()

ENV_OVERRIDE_KEYS=(
  PANEL_PUBLIC_HOST PANEL_PUBLIC_PORT PANEL_ACME_EMAIL SUBSCRIPTION_PUBLIC_HOST
  HY2_DOMAIN HY2_PORT
  SINGBOX_SERVICE_NAME SINGBOX_BINARY_PATH SINGBOX_CONFIG_PATH
  INITIAL_ADMIN_EMAIL INITIAL_ADMIN_PASSWORD
  PANEL_SQLITE_PATH PANEL_STORAGE_ROOT PANEL_RUNTIME_DIR
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
  systemctl list-unit-files h2v2-api.service >/dev/null 2>&1 && unit_hit=1 || true
  systemctl list-unit-files "${SINGBOX_SERVICE_NAME}.service" >/dev/null 2>&1 && unit_hit=1 || true
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

install_singbox() {
  if [[ -x /usr/local/bin/sing-box ]]; then
    if run /usr/local/bin/sing-box version >/dev/null 2>&1; then
      return 0
    fi
    warn "existing sing-box binary is not runnable, reinstalling"
  fi
  phase "build/install: sing-box"
  local ver="${SINGBOX_VERSION:-1.13.0}"
  local archive="/tmp/sing-box-linux-amd64.tar.gz"
  local unpack_dir="/tmp/singbox-install"
  run rm -rf "${unpack_dir}" "${archive}"
  run curl -fsSL "https://github.com/SagerNet/sing-box/releases/download/v${ver}/sing-box-${ver}-linux-amd64.tar.gz" -o "${archive}"
  run mkdir -p "${unpack_dir}"
  run tar -xzf "${archive}" -C "${unpack_dir}"
  run install -m 0755 "${unpack_dir}/sing-box-${ver}-linux-amd64/sing-box" /usr/local/bin/sing-box
  run /usr/local/bin/sing-box version >/dev/null
  run rm -rf "${unpack_dir}" "${archive}"
  changed "sing-box installed"
}

create_users_and_dirs() {
  phase "build/install: users and dirs"
  id -u h2v2 >/dev/null 2>&1 || run useradd --system --home /opt/h2v2 --shell /usr/sbin/nologin h2v2
  id -u singbox >/dev/null 2>&1 || run useradd --system --home /var/lib/sing-box --shell /usr/sbin/nologin singbox
  run usermod -a -G h2v2 singbox || true

  run mkdir -p "${APP_ROOT}" "${BIN_DIR}" "${ETC_ROOT}" "${HY2_DIR}" "${SINGBOX_DIR}"
  run mkdir -p /var/lib/h2v2 /var/lib/h2v2/backups /var/lib/h2v2/data /var/lib/sing-box /run/h2v2 /run/h2v2/locks /run/h2v2/tmp
  run chown -R h2v2:h2v2 /var/lib/h2v2 /run/h2v2
  run chown -R singbox:singbox /var/lib/sing-box
  run chmod 0750 /run/h2v2 /run/h2v2/locks /run/h2v2/tmp
  run chown root:h2v2 "${HY2_DIR}"
  run chown root:h2v2 "${SINGBOX_DIR}"
  run chmod 2770 "${HY2_DIR}"
  run chmod 2770 "${SINGBOX_DIR}"
}

sync_source() {
  phase "build/install: source sync"
  run mkdir -p "${SRC_DIR}"
  run rsync -a --delete --exclude '.git' "${REPO_ROOT}/" "${SRC_DIR}/"
  run chmod +x "${SRC_DIR}/scripts/smoke-check.sh" "${SRC_DIR}/scripts/sqlite-backup-rotate.sh" "${SRC_DIR}/deploy/install.sh" "${SRC_DIR}/deploy/ubuntu24-host-install.sh"
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
  # Caddy (running as caddy user) serves web/dist directly; ensure it is world-readable.
  if [[ -d "${SRC_DIR}/web/dist" ]]; then
    run chmod -R a+rX "${SRC_DIR}/web/dist"
  fi
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
  local var="$1"
  local text="$2"
  local default="$3"
  local current=""
  [[ "${var}" =~ ^[A-Za-z_][A-Za-z0-9_]*$ ]] || fatal "Invalid variable name in prompt_value: ${var}"
  current="${!var-}"
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
  local var="$1"
  local text="$2"
  local current=""
  local answer=""
  [[ "${var}" =~ ^[A-Za-z_][A-Za-z0-9_]*$ ]] || fatal "Invalid variable name in prompt_password: ${var}"
  current="${!var-}"
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
  local var="$1"
  local bytes="$2"
  local current=""
  [[ "${var}" =~ ^[A-Za-z_][A-Za-z0-9_]*$ ]] || fatal "Invalid variable name in generate_if_empty: ${var}"
  current="${!var-}"
  [[ -n "${current}" ]] || printf -v "${var}" '%s' "$(openssl rand -hex "${bytes}")"
}

collect_config() {
  phase "config render: collect"
  local host
  host="$(hostname -f 2>/dev/null || hostname)"
  prompt_value PANEL_PUBLIC_HOST "Panel public domain or IP" "${PANEL_PUBLIC_HOST:-${host}}"
  prompt_value PANEL_PUBLIC_PORT "Panel HTTPS port" "${PANEL_PUBLIC_PORT:-8443}"
  prompt_value PANEL_ACME_EMAIL "ACME email" "${PANEL_ACME_EMAIL:-admin@${PANEL_PUBLIC_HOST}}"
  prompt_value SUBSCRIPTION_PUBLIC_HOST "Subscription public host" "${SUBSCRIPTION_PUBLIC_HOST:-${PANEL_PUBLIC_HOST}}"
  prompt_value HY2_DOMAIN "HY2 public domain" "${HY2_DOMAIN:-${PANEL_PUBLIC_HOST}}"
  prompt_value HY2_PORT "HY2 UDP port" "${HY2_PORT:-443}"
  prompt_value INITIAL_ADMIN_EMAIL "Initial admin email" "${INITIAL_ADMIN_EMAIL:-admin@${PANEL_PUBLIC_HOST}}"
  prompt_password INITIAL_ADMIN_PASSWORD "Initial admin password"

  generate_if_empty INTERNAL_AUTH_TOKEN 32

  APP_ENV="${APP_ENV:-production}"
  PANEL_API_LISTEN_ADDR="127.0.0.1:${PANEL_API_PORT}"
  PANEL_API_INTERNAL_URL="http://127.0.0.1:${PANEL_API_PORT}"
  PANEL_PUBLIC_URL="https://${PANEL_PUBLIC_HOST}:${PANEL_PUBLIC_PORT}"
  SUBSCRIPTION_PUBLIC_URL="https://${SUBSCRIPTION_PUBLIC_HOST}:${PANEL_PUBLIC_PORT}"
  PANEL_STORAGE_ROOT="${PANEL_STORAGE_ROOT:-/var/lib/h2v2}"
  PANEL_RUNTIME_DIR="${PANEL_RUNTIME_DIR:-/run/h2v2}"
  PANEL_SQLITE_PATH="${PANEL_SQLITE_PATH:-${PANEL_STORAGE_ROOT}/data/h2v2.db}"
  SESSION_COOKIE_NAME="${SESSION_COOKIE_NAME:-pp_session}"
  CSRF_COOKIE_NAME="${CSRF_COOKIE_NAME:-pp_csrf}"
  CSRF_HEADER_NAME="${CSRF_HEADER_NAME:-X-CSRF-Token}"
  SESSION_TTL="${SESSION_TTL:-24h}"
  SECURE_COOKIES="${SECURE_COOKIES:-true}"
  RUNTIME_POLL_INTERVAL="${RUNTIME_POLL_INTERVAL:-20s}"
  SERVICE_POLL_INTERVAL="${SERVICE_POLL_INTERVAL:-60s}"
  SINGBOX_SERVICE_NAME="${SINGBOX_SERVICE_NAME:-sing-box}"
  MANAGED_SERVICES="h2v2-api,${SINGBOX_SERVICE_NAME}"
  SYSTEMCTL_PATH="/usr/bin/systemctl"
  SUDO_PATH="/usr/bin/sudo"
  JOURNALCTL_PATH="/usr/bin/journalctl"
  SERVICE_COMMAND_TIMEOUT=30s
  SERVICE_LOG_LINES_MAX=120
  AUTH_RATE_LIMIT_WINDOW=15m
  AUTH_RATE_LIMIT_BURST=10
  SINGBOX_BINARY_PATH="${SINGBOX_BINARY_PATH:-/usr/local/bin/sing-box}"
  SINGBOX_CONFIG_PATH="${SINGBOX_CONFIG_PATH:-${SINGBOX_DIR}/config.json}"
  HY2_CERT_PATH="${HY2_DIR}/server.crt"
  HY2_KEY_PATH="${HY2_DIR}/server.key"
}

validate_config() {
  [[ -n "${PANEL_PUBLIC_HOST:-}" && -n "${SUBSCRIPTION_PUBLIC_HOST:-}" && -n "${HY2_DOMAIN:-}" ]] || fatal "public hosts and HY2_DOMAIN are required"
  [[ "${PANEL_PUBLIC_PORT}" != "${PANEL_API_PORT}" ]] || fatal "PANEL_PUBLIC_PORT conflicts with internal port"
}

write_env_files() {
  phase "config render: write env"
  (( DRY_RUN == 1 )) && return 0
  cat > "${ENV_FILE}" <<EOF
APP_ENV=${APP_ENV}
PANEL_API_LISTEN_ADDR=${PANEL_API_LISTEN_ADDR}
PANEL_API_PORT=${PANEL_API_PORT}
PANEL_PUBLIC_HOST=${PANEL_PUBLIC_HOST}
PANEL_PUBLIC_PORT=${PANEL_PUBLIC_PORT}
PANEL_PUBLIC_URL=${PANEL_PUBLIC_URL}
SUBSCRIPTION_PUBLIC_HOST=${SUBSCRIPTION_PUBLIC_HOST}
SUBSCRIPTION_PUBLIC_URL=${SUBSCRIPTION_PUBLIC_URL}
PANEL_API_INTERNAL_URL=${PANEL_API_INTERNAL_URL}
PANEL_ACME_EMAIL=${PANEL_ACME_EMAIL}
PANEL_STORAGE_ROOT=${PANEL_STORAGE_ROOT}
PANEL_SQLITE_PATH=${PANEL_SQLITE_PATH}
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
HY2_CERT_PATH=${HY2_CERT_PATH}
HY2_KEY_PATH=${HY2_KEY_PATH}
RUNTIME_POLL_INTERVAL=${RUNTIME_POLL_INTERVAL}
SINGBOX_BINARY_PATH=${SINGBOX_BINARY_PATH}
SINGBOX_CONFIG_PATH=${SINGBOX_CONFIG_PATH}
SINGBOX_SERVICE_NAME=${SINGBOX_SERVICE_NAME}

SERVICE_POLL_INTERVAL=${SERVICE_POLL_INTERVAL}
MANAGED_SERVICES=${MANAGED_SERVICES}
SYSTEMCTL_PATH=${SYSTEMCTL_PATH}
SUDO_PATH=${SUDO_PATH}
JOURNALCTL_PATH=${JOURNALCTL_PATH}
SERVICE_COMMAND_TIMEOUT=${SERVICE_COMMAND_TIMEOUT}
SERVICE_LOG_LINES_MAX=${SERVICE_LOG_LINES_MAX}

AUTH_RATE_LIMIT_WINDOW=${AUTH_RATE_LIMIT_WINDOW}
AUTH_RATE_LIMIT_BURST=${AUTH_RATE_LIMIT_BURST}
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
  if [[ "${SUBSCRIPTION_PUBLIC_HOST}" != "${PANEL_PUBLIC_HOST}" ]]; then
    cat >> /etc/caddy/Caddyfile <<EOF

${SUBSCRIPTION_PUBLIC_HOST}:${PANEL_PUBLIC_PORT} {
  encode gzip zstd

  header {
    Strict-Transport-Security "max-age=31536000; includeSubDomains"
    X-Frame-Options "DENY"
    X-Content-Type-Options "nosniff"
    Referrer-Policy "same-origin"
  }

  @panel_api path /api/* /sub/* /healthz /readyz
  handle @panel_api {
    reverse_proxy 127.0.0.1:${PANEL_API_PORT}
  }

  handle {
    root * /opt/h2v2/current/web/dist
    @hashed_asset path_regexp ^/assets/.+\.[0-9a-f]{8,}\.[^.]+$
    header @hashed_asset Cache-Control "public, max-age=31536000, immutable"
    try_files {path} /index.html
    file_server
  }
}
EOF
  fi
  # Provision ACME cert for HY2_DOMAIN on an alternate TLS port so that sing-box
  # can bind :443 (TCP+UDP). Caddy issues the cert via HTTP-01 challenge on :80,
  # cert-sync copies it to the sing-box cert dir.
  if [[ "${HY2_DOMAIN}" != "${PANEL_PUBLIC_HOST}" && "${HY2_DOMAIN}" != "${SUBSCRIPTION_PUBLIC_HOST}" ]]; then
    cat >> /etc/caddy/Caddyfile <<EOF

${HY2_DOMAIN}:${PANEL_PUBLIC_PORT} {
  tls {
    issuer acme {
      disable_tlsalpn_challenge
    }
  }
  respond 204
}
EOF
  fi
  run mkdir -p "$(dirname "${SINGBOX_CONFIG_PATH}")"
  cat > "${SINGBOX_CONFIG_PATH}" <<EOF
{
  "log": {
    "level": "warn"
  },
  "inbounds": [],
  "outbounds": [
    {
      "type": "direct",
      "tag": "direct"
    },
    {
      "type": "block",
      "tag": "block"
    }
  ],
  "route": {
    "final": "direct"
  }
}
EOF
  run chown root:h2v2 "${SINGBOX_CONFIG_PATH}"
  run chmod 0660 "${SINGBOX_CONFIG_PATH}"
  if ! run runuser -u singbox -- "${SINGBOX_BINARY_PATH}" check -c "${SINGBOX_CONFIG_PATH}" >/dev/null 2>&1; then
    fatal "sing-box config validation failed: ${SINGBOX_CONFIG_PATH}"
  fi
  # Ensure TLS cert files exist and are valid for HY2_DOMAIN so sing-box can start.
  # On first install, or when the existing cert covers a different domain (leftover from
  # a previous install), a self-signed placeholder is generated.  The cert-sync unit
  # will replace it with the real ACME cert that Caddy obtains for HY2_DOMAIN.
  cert_ok=0
  if [[ -f "${HY2_CERT_PATH}" && -f "${HY2_KEY_PATH}" ]]; then
    openssl x509 -noout -checkhost "${HY2_DOMAIN}" -in "${HY2_CERT_PATH}" 2>/dev/null && cert_ok=1
  fi
  if (( cert_ok == 0 )); then
    openssl req -x509 -newkey ec -pkeyopt ec_paramgen_curve:P-256 \
      -days 3650 -nodes \
      -keyout "${HY2_KEY_PATH}" \
      -out "${HY2_CERT_PATH}" \
      -subj "/CN=${HY2_DOMAIN}" \
      -addext "subjectAltName=DNS:${HY2_DOMAIN}" \
      2>/dev/null
    run chown root:h2v2 "${HY2_CERT_PATH}" "${HY2_KEY_PATH}"
    run chmod 0640 "${HY2_CERT_PATH}" "${HY2_KEY_PATH}"
  fi
  changed "runtime templates rendered"
}

install_sudoers_and_units() {
  phase "build/install: sudoers + systemd"
  (( DRY_RUN == 1 )) && return 0
  cat > /etc/sudoers.d/h2v2-api <<EOF
Cmnd_Alias H2V2_SHOW = /usr/bin/systemctl show h2v2-api --property=ActiveState --property=SubState --property=MainPID --property=ActiveEnterTimestamp, /usr/bin/systemctl show ${SINGBOX_SERVICE_NAME} --property=ActiveState --property=SubState --property=MainPID --property=ActiveEnterTimestamp
Cmnd_Alias H2V2_RESTART = /usr/bin/systemctl restart h2v2-api, /usr/bin/systemctl restart ${SINGBOX_SERVICE_NAME}
Cmnd_Alias H2V2_RELOAD = /usr/bin/systemctl reload h2v2-api, /usr/bin/systemctl reload ${SINGBOX_SERVICE_NAME}
Cmnd_Alias H2V2_LOGS = /usr/bin/journalctl -u h2v2-api -n * --no-pager --output=short-iso, /usr/bin/journalctl -u ${SINGBOX_SERVICE_NAME} -n * --no-pager --output=short-iso
h2v2 ALL=(root) NOPASSWD: H2V2_SHOW, H2V2_RESTART, H2V2_RELOAD, H2V2_LOGS
EOF
  run chmod 0440 /etc/sudoers.d/h2v2-api
  run visudo -cf /etc/sudoers.d/h2v2-api >/dev/null
  run install -m 0644 "${SRC_DIR}/systemd/h2v2-api.service" /etc/systemd/system/h2v2-api.service
  run install -m 0644 "${SRC_DIR}/systemd/sing-box.service" "/etc/systemd/system/${SINGBOX_SERVICE_NAME}.service"
  # Migration: remove legacy vite-preview unit if left over from previous installs.
  if [[ -f /etc/systemd/system/h2v2-web.service ]]; then
    systemctl disable --now h2v2-web.service 2>/dev/null || true
    run rm -f /etc/systemd/system/h2v2-web.service
  fi
  run sed -i "s|__SINGBOX_BINARY_PATH__|${SINGBOX_BINARY_PATH}|g; s|__SINGBOX_CONFIG_PATH__|${SINGBOX_CONFIG_PATH}|g" "/etc/systemd/system/${SINGBOX_SERVICE_NAME}.service"
  systemctl disable --now hysteria-server.service 2>/dev/null || true
  systemctl disable --now xray.service 2>/dev/null || true
  run rm -f /etc/systemd/system/hysteria-server.service /etc/systemd/system/xray.service
  if grep -q "__SINGBOX_" "/etc/systemd/system/${SINGBOX_SERVICE_NAME}.service"; then
    fatal "sing-box unit placeholders were not rendered"
  fi
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
  run mkdir -p "${BACKUP_DIR}/env" "${BACKUP_DIR}/etc" "${BACKUP_DIR}/systemd" "${BACKUP_DIR}/storage"
  [[ -f "${ENV_FILE}" ]] && run cp -a "${ENV_FILE}" "${BACKUP_DIR}/env/.env.generated"
  [[ -f "${CREDENTIALS_FILE}" ]] && run cp -a "${CREDENTIALS_FILE}" "${BACKUP_DIR}/env/credentials.txt"
  [[ -d "${ETC_ROOT}" ]] && run rsync -a "${ETC_ROOT}/" "${BACKUP_DIR}/etc/h2v2/"
  [[ -f /etc/caddy/h2v2.env ]] && run cp -a /etc/caddy/h2v2.env "${BACKUP_DIR}/etc/caddy.env"
  for u in h2v2-api.service "${SINGBOX_SERVICE_NAME}.service"; do
    [[ -f "/etc/systemd/system/${u}" ]] && run cp -a "/etc/systemd/system/${u}" "${BACKUP_DIR}/systemd/${u}"
  done
  [[ -d /var/lib/h2v2/data ]] && run rsync -a /var/lib/h2v2/data/ "${BACKUP_DIR}/storage/data/"
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
  for u in h2v2-api.service "${SINGBOX_SERVICE_NAME}.service"; do
    [[ -f "${BACKUP_DIR}/systemd/${u}" ]] && run cp -a "${BACKUP_DIR}/systemd/${u}" "/etc/systemd/system/${u}"
  done
  [[ -d "${BACKUP_DIR}/storage/data" ]] && run rsync -a --delete "${BACKUP_DIR}/storage/data/" /var/lib/h2v2/data/
  run systemctl daemon-reload
  run systemctl restart h2v2-api.service caddy.service "${SINGBOX_SERVICE_NAME}.service" || true
}

on_error() {
  local line="$1"
  warn "installer failed at line ${line}"
  (( ROLLBACK_RUNNING == 0 )) && rollback_from_backup
  exit 1
}

install_cert_sync() {
  phase "build/install: cert-sync"
  (( DRY_RUN == 1 )) && return 0
  # Write cert-sync script: locates Caddy's ACME cert for HY2_DOMAIN using
  # find (robust against storage-path variations) and copies it to the HY2
  # cert directory, then reloads sing-box.
  cat > "${BIN_DIR}/cert-sync.sh" <<'SCRIPT'
#!/usr/bin/env bash
set -euo pipefail
ENV_FILE="/opt/h2v2/.env.generated"
[[ -f "${ENV_FILE}" ]] && { set -a; source "${ENV_FILE}"; set +a; }
DOMAIN="${HY2_DOMAIN:-}"
DST_CRT="${HY2_CERT_PATH:-/etc/h2v2/hysteria/server.crt}"
DST_KEY="${HY2_KEY_PATH:-/etc/h2v2/hysteria/server.key}"
SINGBOX_SVC="${SINGBOX_SERVICE_NAME:-sing-box}"
[[ -n "${DOMAIN}" ]] || exit 0

# Caddy stores ACME certs under its data directory. The default XDG path is:
#   /var/lib/caddy/.local/share/caddy/certificates/acme-v02.api.letsencrypt.org-directory/<domain>/
# But it may also be under /root/.local/share/caddy or HOME-relative.
find_caddy_cert() {
  local domain="$1" crt
  for root in \
    /var/lib/caddy/.local/share/caddy \
    /root/.local/share/caddy \
    /var/lib/caddy \
    /root; do
    [[ -d "${root}" ]] || continue
    crt="$(find "${root}" -name "${domain}.crt" -path "*/certificates/*" 2>/dev/null | head -1)"
    [[ -n "${crt}" && -f "${crt%.crt}.key" ]] && echo "${crt}" && return 0
  done
  return 1
}

crt="$(find_caddy_cert "${DOMAIN}")" || exit 0
key="${crt%.crt}.key"

# Validate cert covers the right domain before copying.
openssl x509 -noout -checkhost "${DOMAIN}" -in "${crt}" 2>/dev/null || exit 0

# Skip if already up to date.
cmp -s "${crt}" "${DST_CRT}" 2>/dev/null && cmp -s "${key}" "${DST_KEY}" 2>/dev/null && exit 0

install -m 0640 -o root -g h2v2 "${crt}" "${DST_CRT}"
install -m 0640 -o root -g h2v2 "${key}" "${DST_KEY}"
systemctl reload-or-restart "${SINGBOX_SVC}.service" 2>/dev/null || true
logger -t h2v2-cert-sync "synced ${DOMAIN} cert from ${crt}"
SCRIPT
  run chmod 0750 "${BIN_DIR}/cert-sync.sh"
  run chown root:h2v2 "${BIN_DIR}/cert-sync.sh"
  cat > /etc/systemd/system/h2v2-cert-sync.service <<EOF
[Unit]
Description=H2V2 cert sync (Caddy -> HY2 dir)
After=caddy.service

[Service]
Type=oneshot
ExecStart=${BIN_DIR}/cert-sync.sh
EOF
  cat > /etc/systemd/system/h2v2-cert-sync.timer <<EOF
[Unit]
Description=Periodic HY2 cert sync from Caddy ACME storage

[Timer]
OnBootSec=30s
OnUnitActiveSec=5min
Persistent=true

[Install]
WantedBy=timers.target
EOF
  run systemctl daemon-reload
  run systemctl enable h2v2-cert-sync.timer
  changed "cert-sync installed"
}

bootstrap_admin() {
  phase "finalize: bootstrap admin"
  (( DRY_RUN == 1 )) && return 0
  set -a
  # shellcheck disable=SC1090
  source "${ENV_FILE}"
  set +a
  run runuser -u h2v2 -- "${BIN_DIR}/panel-api" bootstrap-admin --email "${INITIAL_ADMIN_EMAIL}" --password "${INITIAL_ADMIN_PASSWORD}"
  run runuser -u h2v2 -- "${BIN_DIR}/panel-api" bootstrap-inbounds
  # Re-normalize existing inbounds so upgrades pick up new defaults (Reality
  # keys, AllowInsecure for the panel-managed HY2 cert path, etc.).
  run runuser -u h2v2 -- "${BIN_DIR}/panel-api" refresh-inbounds || true
}

restart_services() {
  phase "finalize: restart services"
  local s
  for s in h2v2-api caddy "${SINGBOX_SERVICE_NAME}"; do
    run systemctl enable "${s}.service"
  done
  run systemctl restart h2v2-api.service
  run systemctl restart caddy.service
  # Wait up to 90 s for Caddy to obtain the ACME cert for HY2_DOMAIN, then
  # sync it to the HY2 cert dir so sing-box starts with a valid certificate.
  # If the cert does not appear in time, the self-signed placeholder is used.
  local waited=0 found=0
  info "waiting for Caddy ACME cert for ${HY2_DOMAIN} (up to 90s)..."
  while (( waited < 90 )); do
    for root in /var/lib/caddy /root /home; do
      [[ -d "${root}" ]] || continue
      if find "${root}" -name "${HY2_DOMAIN}.crt" -path "*/certificates/*" 2>/dev/null | grep -q .; then
        found=1; break 2
      fi
    done
    sleep 3
    (( waited += 3 ))
  done
  if (( found == 1 )); then
    info "Caddy cert found, syncing to HY2 dir"
    "${BIN_DIR}/cert-sync.sh" 2>/dev/null || true
  else
    warn "Caddy cert for ${HY2_DOMAIN} not yet available; sing-box will use self-signed placeholder until cert-sync runs"
  fi
  run systemctl restart "${SINGBOX_SERVICE_NAME}.service"
  run systemctl start h2v2-cert-sync.timer
}

wait_for_panel_api() {
  local url="http://127.0.0.1:${PANEL_API_PORT}/healthz"
  local attempts="${PANEL_API_HEALTH_ATTEMPTS:-30}"
  local sleep_sec="${PANEL_API_HEALTH_SLEEP_SEC:-2}"
  local i
  for ((i=1; i<=attempts; i++)); do
    if curl -fsS --connect-timeout 2 --max-time 5 "${url}" >/dev/null 2>&1; then
      return 0
    fi
    if ! systemctl is-active --quiet h2v2-api.service; then
      return 1
    fi
    sleep "${sleep_sec}"
  done
  return 1
}

require_service_active() {
  local service="$1"
  if systemctl is-active --quiet "${service}.service"; then
    return 0
  fi
  systemctl status "${service}.service" --no-pager -l || true
  if [[ "${service}" == "${SINGBOX_SERVICE_NAME}" ]]; then
    local cfg_path="${SINGBOX_CONFIG_PATH:-/etc/h2v2/sing-box/config.json}"
    if [[ -x "${SINGBOX_BINARY_PATH}" ]]; then
      runuser -u singbox -- "${SINGBOX_BINARY_PATH}" check -c "${cfg_path}" || true
    fi
    journalctl -u "${SINGBOX_SERVICE_NAME}" -n 120 --no-pager || true
  fi
  return 1
}

health_checks() {
  phase "health checks"
  (( DRY_RUN == 1 )) && return 0
  require_service_active h2v2-api
  require_service_active caddy
  require_service_active "${SINGBOX_SERVICE_NAME}"
  if ! wait_for_panel_api; then
    systemctl status h2v2-api.service --no-pager -l || true
    journalctl -u h2v2-api -n 100 --no-pager || true
    return 1
  fi
  SMOKE_ADMIN_EMAIL="${INITIAL_ADMIN_EMAIL}" SMOKE_ADMIN_PASSWORD="${INITIAL_ADMIN_PASSWORD}" run bash "${SRC_DIR}/scripts/smoke-check.sh" "${ENV_FILE}"
}

run_build_phase() {
  if [[ "${MODE}" == "reconfigure" ]]; then
    install_singbox
    create_users_and_dirs
    sync_source
    return
  fi
  install_base_packages
  install_go
  install_node
  install_singbox
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
  printf "active storage driver: sqlite\n"
  printf "\nverify:\n"
  printf "  systemctl status h2v2-api %s caddy\n" "${SINGBOX_SERVICE_NAME}"
  printf "  bash %s/scripts/smoke-check.sh %s\n" "${SRC_DIR}" "${ENV_FILE}"
  printf "\nrollback:\n"
  printf "  rsync -a --delete %s/storage/data/ /var/lib/h2v2/data/\n" "${BACKUP_DIR:-/path/to/backup}"
  printf "  cp %s/env/.env.generated %s\n" "${BACKUP_DIR:-/path/to/backup}" "${ENV_FILE}"
}

main() {
  parse_args "$@"
  trap 'on_error $LINENO' ERR
  phase "preflight checks"
  require_root
  check_os
  detect_existing_installation
  capture_env_overrides
  load_existing_env
  apply_env_overrides
  create_backup
  run_build_phase
  collect_config
  validate_config
  write_env_files
  render_runtime_configs
  install_sudoers_and_units
  install_cert_sync
  bootstrap_admin
  restart_services
  health_checks
  summary
}

main "$@"
