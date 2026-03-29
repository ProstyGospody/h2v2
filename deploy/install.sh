#!/usr/bin/env bash
set -euo pipefail

RECONFIGURE=0
NONINTERACTIVE="${H2V2_NONINTERACTIVE:-0}"
while [[ $# -gt 0 ]]; do
  case "$1" in
    --reconfigure)
      RECONFIGURE=1
      ;;
    --non-interactive)
      NONINTERACTIVE=1
      ;;
    *)
      printf "[error] Unknown argument: %s\n" "$1" >&2
      exit 1
      ;;
  esac
  shift
done

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

APP_ROOT="/opt/h2v2"
SRC_DIR="${APP_ROOT}/current"
BIN_DIR="${APP_ROOT}/bin"
ENV_FILE="${APP_ROOT}/.env.generated"
CREDENTIALS_FILE="/root/h2v2-initial-admin.txt"

ETC_ROOT="/etc/h2v2"
HY2_DIR="${ETC_ROOT}/hysteria"

PANEL_API_PORT="18080"
PANEL_WEB_PORT="13000"

ENV_OVERRIDE_KEYS=(
  PANEL_PUBLIC_HOST
  PANEL_PUBLIC_PORT
  PANEL_ACME_EMAIL
  SUBSCRIPTION_PUBLIC_HOST
  HY2_DOMAIN
  HY2_PORT
  HY2_OBFS_PASSWORD
  HY2_STATS_PORT
  INITIAL_ADMIN_EMAIL
  INITIAL_ADMIN_PASSWORD
)

action() {
  printf "\n==> %s\n" "$1"
}

fatal() {
  printf "[error] %s\n" "$1" >&2
  exit 1
}

apt_get() {
  local lock_timeout="${APT_LOCK_TIMEOUT:-900}"
  apt-get -o DPkg::Lock::Timeout="${lock_timeout}" "$@"
}

is_ipv4() {
  local value="$1"
  [[ "$value" =~ ^([0-9]{1,3}\.){3}[0-9]{1,3}$ ]]
}

is_ipv6() {
  local value="$1"
  [[ "$value" == *:* ]]
}

is_ip_address() {
  local value="$1"
  is_ipv4 "$value" || is_ipv6 "$value"
}

is_valid_dns_name() {
  local value="$1"
  [[ "$value" =~ ^[A-Za-z0-9.-]+$ ]] && [[ "$value" == *.* ]]
}

is_noninteractive() {
  case "${NONINTERACTIVE}" in
    1|true|TRUE|yes|YES)
      return 0
      ;;
    *)
      return 1
      ;;
  esac
}

validate_configuration() {
  [[ -n "${PANEL_PUBLIC_HOST}" ]] || fatal "PANEL_PUBLIC_HOST is required"
  [[ -n "${SUBSCRIPTION_PUBLIC_HOST}" ]] || fatal "SUBSCRIPTION_PUBLIC_HOST is required"
  [[ -n "${HY2_DOMAIN}" ]] || fatal "HY2_DOMAIN is required"

  if [[ "${PANEL_PUBLIC_HOST}" == *://* || "${PANEL_PUBLIC_HOST}" == */* ]]; then
    fatal "PANEL_PUBLIC_HOST must be a bare host name or IP, not a URL"
  fi
  if [[ "${SUBSCRIPTION_PUBLIC_HOST}" == *://* || "${SUBSCRIPTION_PUBLIC_HOST}" == */* ]]; then
    fatal "SUBSCRIPTION_PUBLIC_HOST must be a bare host name or IP, not a URL"
  fi
  if [[ "${HY2_DOMAIN}" == *://* || "${HY2_DOMAIN}" == */* ]]; then
    fatal "HY2_DOMAIN must be a bare DNS name, not a URL"
  fi
  if is_ip_address "${HY2_DOMAIN}"; then
    fatal "HY2_DOMAIN must be a DNS name; Caddy/ACME cannot provision cert for raw IP"
  fi
  if ! is_valid_dns_name "${HY2_DOMAIN}"; then
    fatal "HY2_DOMAIN must be a valid DNS name"
  fi
  if [[ "${PANEL_PUBLIC_PORT}" == "${PANEL_WEB_PORT}" ]]; then
    fatal "PANEL_PUBLIC_PORT must not equal internal PANEL_WEB_PORT (${PANEL_WEB_PORT})"
  fi
  if [[ "${PANEL_PUBLIC_PORT}" == "${PANEL_API_PORT}" ]]; then
    fatal "PANEL_PUBLIC_PORT must not equal internal PANEL_API_PORT (${PANEL_API_PORT})"
  fi
}

require_root() {
  if [[ "$(id -u)" -ne 0 ]]; then
    fatal "Run as root: sudo bash ./deploy/install.sh"
  fi
}

check_os() {
  if [[ ! -f /etc/os-release ]]; then
    fatal "Cannot detect operating system"
  fi
  # shellcheck disable=SC1091
  source /etc/os-release
  case "${ID}" in
    ubuntu)
      if [[ "${VERSION_ID}" != "24.04" ]]; then
        fatal "Ubuntu 24.04 is required; found Ubuntu ${VERSION_ID:-unknown}"
      fi
      ;;
    debian)
      local debian_major="${VERSION_ID%%.*}"
      if [[ ! "${debian_major}" =~ ^[0-9]+$ ]] || (( debian_major < 12 )); then
        fatal "Debian 12+ is required; found Debian ${VERSION_ID:-unknown}"
      fi
      ;;
    *)
      fatal "This installer supports Ubuntu 24.04 and Debian 12+ only"
      ;;
  esac
}

version_gte() {
  local required="$1"
  local current="$2"
  [[ "$(printf '%s\n' "${required}" "${current}" | sort -V | head -n1)" == "${required}" ]]
}

install_system_packages() {
  action "Installing system packages"
  export DEBIAN_FRONTEND=noninteractive
  apt_get update
  apt_get install -y \
    ca-certificates \
    curl \
    git \
    jq \
    rsync \
    unzip \
    tar \
    lsb-release \
    gnupg \
    build-essential \
    pkg-config \
    libssl-dev \
    zlib1g-dev \
    caddy \
    sudo
}

install_go() {
  local min_go="1.24.0"
  local go_version="${GO_VERSION:-1.24.3}"
  local go_url="https://dl.google.com/go/go${go_version}.linux-amd64.tar.gz"

  if command -v go >/dev/null 2>&1; then
    local current_go
    current_go="$(go version | awk '{print $3}' | sed 's/^go//')"
    if version_gte "${min_go}" "${current_go}"; then
      action "Go ${current_go} already installed"
      return
    fi
  fi

  action "Installing Go ${go_version}"
  curl -fsSL "${go_url}" -o /tmp/go.tgz
  rm -rf /usr/local/go
  tar -C /usr/local -xzf /tmp/go.tgz
  ln -sf /usr/local/go/bin/go /usr/local/bin/go
  ln -sf /usr/local/go/bin/gofmt /usr/local/bin/gofmt
}

install_node() {
  local required_node="20.19.0"
  local target_major="20"
  local current_node=""

  if command -v node >/dev/null 2>&1; then
    current_node="$(node -v | sed 's/^v//')"
  fi

  if [[ -n "${current_node}" ]] && command -v npm >/dev/null 2>&1 && version_gte "${required_node}" "${current_node}"; then
    action "Node.js v${current_node} and npm $(npm -v) already installed"
    return
  fi

  if [[ -n "${current_node}" ]] && ! command -v npm >/dev/null 2>&1; then
    action "Node.js v${current_node} is installed without npm; reinstalling Node.js ${target_major}.x"
  elif [[ -n "${current_node}" ]]; then
    action "Upgrading Node.js from v${current_node} to >= ${required_node}"
  else
    action "Installing Node.js >= ${required_node}"
  fi

  export DEBIAN_FRONTEND=noninteractive
  curl -fsSL "https://deb.nodesource.com/setup_${target_major}.x" | bash -
  apt_get install -y nodejs

  command -v node >/dev/null 2>&1 || fatal "node installation failed"
  command -v npm >/dev/null 2>&1 || fatal "npm installation failed"

  current_node="$(node -v | sed 's/^v//')"
  if ! version_gte "${required_node}" "${current_node}"; then
    fatal "Node.js >= ${required_node} is required after install, found v${current_node}"
  fi

  action "Installed Node.js v${current_node} and npm $(npm -v)"
}

install_hysteria() {
  local version="${HYSTERIA_VERSION:-2.6.5}"
  local url="https://github.com/apernet/hysteria/releases/download/app%2Fv${version}/hysteria-linux-amd64"

  if [[ -x /usr/local/bin/hysteria ]]; then
    action "Hysteria already installed"
    return
  fi

  action "Installing Hysteria ${version}"
  curl -fsSL "${url}" -o /tmp/hysteria-linux-amd64
  install -m 0755 /tmp/hysteria-linux-amd64 /usr/local/bin/hysteria
  command -v hysteria >/dev/null 2>&1 || fatal "Hysteria installation failed"
}

create_system_users() {
  action "Creating system users"
  id -u h2v2 >/dev/null 2>&1 || useradd --system --home /opt/h2v2 --shell /usr/sbin/nologin h2v2
  id -u hysteria >/dev/null 2>&1 || useradd --system --home /var/lib/hysteria --shell /usr/sbin/nologin hysteria

  usermod -a -G h2v2 hysteria || true
}

capture_env_overrides() {
  local key
  for key in "${ENV_OVERRIDE_KEYS[@]}"; do
    if [[ -n "${!key+x}" ]]; then
      printf -v "__OVERRIDE_${key}" '%s' "${!key}"
    fi
  done
}

apply_env_overrides() {
  local key override_name
  for key in "${ENV_OVERRIDE_KEYS[@]}"; do
    override_name="__OVERRIDE_${key}"
    if [[ -n "${!override_name+x}" ]]; then
      printf -v "${key}" '%s' "${!override_name}"
    fi
  done
}

prepare_directories() {
  action "Preparing directories"

  mkdir -p "${APP_ROOT}" "${BIN_DIR}" "${ETC_ROOT}" "${HY2_DIR}"
  mkdir -p /var/lib/h2v2 /var/lib/h2v2/backups /var/log/h2v2/audit /var/lib/hysteria /run/h2v2 /run/h2v2/locks /run/h2v2/tmp

  chown -R h2v2:h2v2 /var/lib/h2v2 /var/log/h2v2 /run/h2v2
  chown -R hysteria:hysteria /var/lib/hysteria

  chmod 0750 /run/h2v2 /run/h2v2/locks /run/h2v2/tmp

  chown root:h2v2 "${HY2_DIR}"
  chmod 2770 "${HY2_DIR}"
}

sync_source_tree() {
  action "Syncing repository to ${SRC_DIR}"
  mkdir -p "${SRC_DIR}"
  rsync -a --delete --exclude '.git' "${REPO_ROOT}/" "${SRC_DIR}/"
  chmod +x \
    "${SRC_DIR}/scripts/smoke-check.sh" \
    "${SRC_DIR}/scripts/sync-hysteria-cert.sh" \
    "${SRC_DIR}/deploy/install.sh" \
    "${SRC_DIR}/deploy/ubuntu24-host-install.sh"
}

load_existing_values() {
  if [[ -f "${ENV_FILE}" ]]; then
    # shellcheck disable=SC1090
    source "${ENV_FILE}"

    # Backward compatibility: older generated env files may only contain HY2_STATS_URL.
    if [[ -z "${HY2_STATS_PORT:-}" && -n "${HY2_STATS_URL:-}" ]]; then
      if [[ "${HY2_STATS_URL}" =~ :([0-9]{1,5})$ ]]; then
        HY2_STATS_PORT="${BASH_REMATCH[1]}"
      fi
    fi
  fi
  if [[ -f "${CREDENTIALS_FILE}" ]]; then
    # shellcheck disable=SC1090
    source "${CREDENTIALS_FILE}"
  fi
}

prompt_value() {
  local var_name="$1"
  local prompt_text="$2"
  local default_value="$3"
  local current="${!var_name:-}"

  if [[ -n "${current}" && "${RECONFIGURE}" -eq 0 ]]; then
    return
  fi

  if is_noninteractive; then
    if [[ -n "${current}" ]]; then
      return
    fi
    if [[ -n "${default_value}" ]]; then
      printf -v "${var_name}" '%s' "${default_value}"
      return
    fi
    fatal "Value required in non-interactive mode: ${var_name}"
  fi

  local answer=""
  if [[ -n "${current}" ]]; then
    default_value="${current}"
  fi

  if [[ -n "${default_value}" ]]; then
    read -r -p "${prompt_text} [${default_value}]: " answer
    answer="${answer:-${default_value}}"
  else
    read -r -p "${prompt_text}: " answer
  fi

  if [[ -z "${answer}" ]]; then
    fatal "Value required: ${var_name}"
  fi

  printf -v "${var_name}" '%s' "${answer}"
}

prompt_password() {
  local var_name="$1"
  local prompt_text="$2"
  local current="${!var_name:-}"

  if [[ -n "${current}" && "${RECONFIGURE}" -eq 0 ]]; then
    return
  fi

  if is_noninteractive; then
    if [[ -n "${current}" ]]; then
      return
    fi
    local generated
    generated="$(openssl rand -base64 24 | tr -d '=+/\n' | cut -c1-20)"
    printf -v "${var_name}" '%s' "${generated}"
    echo "Generated random initial admin password (non-interactive mode)"
    return
  fi

  local answer=""
  read -r -s -p "${prompt_text} (leave empty to auto-generate): " answer
  echo

  if [[ -z "${answer}" ]]; then
    answer="$(openssl rand -base64 24 | tr -d '=+/\n' | cut -c1-20)"
    echo "Generated random initial admin password"
  fi

  printf -v "${var_name}" '%s' "${answer}"
}

generate_if_empty() {
  local var_name="$1"
  local bytes="$2"
  if [[ -n "${!var_name:-}" ]]; then
    return
  fi
  local value
  value="$(openssl rand -hex "${bytes}")"
  printf -v "${var_name}" '%s' "${value}"
}

collect_configuration() {
  action "Collecting configuration"

  local default_host
  default_host="$(hostname -f 2>/dev/null || hostname)"

  prompt_value PANEL_PUBLIC_HOST "Panel public domain or IP" "${PANEL_PUBLIC_HOST:-${default_host}}"
  prompt_value PANEL_PUBLIC_PORT "Panel HTTPS port" "${PANEL_PUBLIC_PORT:-8443}"
  prompt_value PANEL_ACME_EMAIL "ACME email" "${PANEL_ACME_EMAIL:-admin@${PANEL_PUBLIC_HOST}}"
  prompt_value SUBSCRIPTION_PUBLIC_HOST "Subscription public domain or IP" "${SUBSCRIPTION_PUBLIC_HOST:-${PANEL_PUBLIC_HOST}}"

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
  SERVICE_LOG_LINES_MAX="120"

  AUTH_RATE_LIMIT_WINDOW="15m"
  AUTH_RATE_LIMIT_BURST="10"

  HY2_BINARY_PATH="/usr/local/bin/hysteria"
  HY2_CONFIG_PATH="${HY2_DIR}/server.yaml"
  HY2_CERT_PATH="${HY2_DIR}/tls.crt"
  HY2_KEY_PATH="${HY2_DIR}/tls.key"
}

write_env_files() {
  action "Writing generated env files"

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
PANEL_STORAGE_ROOT=${PANEL_STORAGE_ROOT}
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

  chown root:h2v2 "${ENV_FILE}"
  chmod 0640 "${ENV_FILE}"

  cat > "${CREDENTIALS_FILE}" <<EOF
INITIAL_ADMIN_EMAIL=${INITIAL_ADMIN_EMAIL}
INITIAL_ADMIN_PASSWORD=${INITIAL_ADMIN_PASSWORD}
PANEL_PUBLIC_URL=${PANEL_PUBLIC_URL}
SUBSCRIPTION_PUBLIC_URL=${SUBSCRIPTION_PUBLIC_URL}
EOF
  chmod 0600 "${CREDENTIALS_FILE}"

  cat > /etc/caddy/h2v2.env <<EOF
PANEL_PUBLIC_HOST=${PANEL_PUBLIC_HOST}
PANEL_PUBLIC_PORT=${PANEL_PUBLIC_PORT}
PANEL_API_PORT=${PANEL_API_PORT}
PANEL_WEB_PORT=${PANEL_WEB_PORT}
PANEL_ACME_EMAIL=${PANEL_ACME_EMAIL}
SUBSCRIPTION_PUBLIC_HOST=${SUBSCRIPTION_PUBLIC_HOST}
HY2_DOMAIN=${HY2_DOMAIN}
EOF
  chmod 0640 /etc/caddy/h2v2.env

  mkdir -p /etc/systemd/system/caddy.service.d
  cat > /etc/systemd/system/caddy.service.d/h2v2-env.conf <<'EOF'
[Service]
EnvironmentFile=/etc/caddy/h2v2.env
EOF
}

render_runtime_configs() {
  action "Rendering runtime configs"

  install -m 0644 "${SRC_DIR}/config/templates/Caddyfile.tmpl" /etc/caddy/Caddyfile
  if [[ "${HY2_DOMAIN}" != "${PANEL_PUBLIC_HOST}" && "${HY2_DOMAIN}" != "${SUBSCRIPTION_PUBLIC_HOST}" ]]; then
    cat >> /etc/caddy/Caddyfile <<EOF

${HY2_DOMAIN}:${PANEL_PUBLIC_PORT} {
  respond "hysteria-cert-bootstrap" 200
}
EOF
  fi
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

  @subscription path /api/hysteria/subscription/* /hysteria/subscription/*
  handle @subscription {
    reverse_proxy 127.0.0.1:${PANEL_API_PORT}
  }

  respond "not found" 404
}
EOF
  fi

  if ! PANEL_PUBLIC_HOST="${PANEL_PUBLIC_HOST}" \
    PANEL_PUBLIC_PORT="${PANEL_PUBLIC_PORT}" \
    PANEL_API_PORT="${PANEL_API_PORT}" \
    PANEL_WEB_PORT="${PANEL_WEB_PORT}" \
    PANEL_ACME_EMAIL="${PANEL_ACME_EMAIL}" \
    SUBSCRIPTION_PUBLIC_HOST="${SUBSCRIPTION_PUBLIC_HOST}" \
    HY2_DOMAIN="${HY2_DOMAIN}" \
    caddy validate --config /etc/caddy/Caddyfile --adapter caddyfile >/tmp/h2v2-caddy-validate.log 2>&1; then
    cat /tmp/h2v2-caddy-validate.log >&2 || true
    fatal "Caddy configuration validation failed; check PANEL_PUBLIC_HOST, SUBSCRIPTION_PUBLIC_HOST, PANEL_PUBLIC_PORT, HY2_DOMAIN"
  fi

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

  chown root:h2v2 "${HY2_DIR}/server.yaml"
  chmod 0660 "${HY2_DIR}/server.yaml"
}

build_backend() {
  action "Building Go backend"
  export PATH="/usr/local/go/bin:${PATH}"
  pushd "${SRC_DIR}" >/dev/null
  GOFLAGS="-mod=mod" go mod download
  GOFLAGS="-mod=mod" go build -ldflags "-s -w" -o "${BIN_DIR}/panel-api" ./cmd/panel-api
  popd >/dev/null
  chown root:h2v2 "${BIN_DIR}/panel-api"
  chmod 0750 "${BIN_DIR}/panel-api"
}

build_frontend() {
  action "Building Vite frontend"
  pushd "${SRC_DIR}/web" >/dev/null
  npm install --no-audit --no-fund
  VITE_API_PROXY_TARGET="${PANEL_API_INTERNAL_URL}" \
  VITE_CSRF_COOKIE_NAME="${CSRF_COOKIE_NAME}" \
  VITE_CSRF_HEADER_NAME="${CSRF_HEADER_NAME}" \
  npm run build
  popd >/dev/null
  chown -R h2v2:h2v2 "${SRC_DIR}/web"
}

wait_for_hysteria_certificate() {
  action "Waiting for Caddy to issue Hysteria certificate"
  bash "${SRC_DIR}/scripts/sync-hysteria-cert.sh" "${ENV_FILE}" --wait
}

install_sudoers_policy() {
  action "Installing restricted sudoers policy"

  cat > /etc/sudoers.d/h2v2-api <<'EOF'
Cmnd_Alias H2V2_SHOW = /usr/bin/systemctl show h2v2-api --property=ActiveState --property=SubState --property=MainPID --property=ActiveEnterTimestamp, /usr/bin/systemctl show h2v2-web --property=ActiveState --property=SubState --property=MainPID --property=ActiveEnterTimestamp, /usr/bin/systemctl show hysteria-server --property=ActiveState --property=SubState --property=MainPID --property=ActiveEnterTimestamp
Cmnd_Alias H2V2_RESTART = /usr/bin/systemctl restart h2v2-api, /usr/bin/systemctl restart h2v2-web, /usr/bin/systemctl restart hysteria-server
Cmnd_Alias H2V2_RELOAD = /usr/bin/systemctl reload h2v2-api, /usr/bin/systemctl reload h2v2-web, /usr/bin/systemctl reload hysteria-server
Cmnd_Alias H2V2_LOGS = /usr/bin/journalctl -u h2v2-api -n * --no-pager --output=short-iso, /usr/bin/journalctl -u h2v2-web -n * --no-pager --output=short-iso, /usr/bin/journalctl -u hysteria-server -n * --no-pager --output=short-iso
h2v2 ALL=(root) NOPASSWD: H2V2_SHOW, H2V2_RESTART, H2V2_RELOAD, H2V2_LOGS
EOF

  chmod 0440 /etc/sudoers.d/h2v2-api
  visudo -cf /etc/sudoers.d/h2v2-api >/dev/null
}

install_systemd_units() {
  action "Installing systemd units"

  install -m 0644 "${SRC_DIR}/systemd/h2v2-api.service" /etc/systemd/system/h2v2-api.service
  install -m 0644 "${SRC_DIR}/systemd/h2v2-web.service" /etc/systemd/system/h2v2-web.service
  install -m 0644 "${SRC_DIR}/systemd/hysteria-server.service" /etc/systemd/system/hysteria-server.service

  systemctl daemon-reload
}

bootstrap_admin() {
  action "Bootstrapping admin account"
  set -a
  # shellcheck disable=SC1090
  source "${ENV_FILE}"
  set +a
  runuser -u h2v2 -- "${BIN_DIR}/panel-api" bootstrap-admin --email "${INITIAL_ADMIN_EMAIL}" --password "${INITIAL_ADMIN_PASSWORD}"
}

start_services() {
  action "Starting services"
  local services=(h2v2-api h2v2-web caddy hysteria-server)

  local service
  for service in "${services[@]}"; do
    systemctl enable "${service}.service"
  done

  systemctl restart h2v2-api.service
  systemctl restart h2v2-web.service
  systemctl restart caddy.service

  wait_for_hysteria_certificate
  systemctl restart hysteria-server.service
}

run_smoke_checks() {
  action "Running smoke checks"
  SMOKE_ADMIN_EMAIL="${INITIAL_ADMIN_EMAIL}" SMOKE_ADMIN_PASSWORD="${INITIAL_ADMIN_PASSWORD}" bash "${SRC_DIR}/scripts/smoke-check.sh" "${ENV_FILE}"
}

print_summary() {
  cat <<EOF

Deployment completed.

Panel URL: ${PANEL_PUBLIC_URL}
Subscription URL base: ${SUBSCRIPTION_PUBLIC_URL}
Initial admin email: ${INITIAL_ADMIN_EMAIL}
Initial admin password file: ${CREDENTIALS_FILE}
Generated env file: ${ENV_FILE}
Storage root: ${PANEL_STORAGE_ROOT}
Audit dir: ${PANEL_AUDIT_DIR}

Systemd services:
  - h2v2-api.service
  - h2v2-web.service
  - hysteria-server.service
  - caddy.service

Useful commands:
  systemctl status h2v2-api h2v2-web hysteria-server caddy
  journalctl -u h2v2-api -n 100 --no-pager
  bash ${SRC_DIR}/scripts/smoke-check.sh ${ENV_FILE}
  sudo bash ${REPO_ROOT}/deploy/install.sh --reconfigure
EOF
}

main() {
  require_root
  check_os

  install_system_packages
  install_go
  install_node
  install_hysteria

  create_system_users
  prepare_directories
  sync_source_tree

  capture_env_overrides
  load_existing_values
  apply_env_overrides
  collect_configuration
  validate_configuration
  write_env_files

  build_backend
  build_frontend
  render_runtime_configs
  install_sudoers_policy
  install_systemd_units

  bootstrap_admin
  start_services
  run_smoke_checks
  print_summary
}

main "$@"
