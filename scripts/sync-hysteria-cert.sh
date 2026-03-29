#!/usr/bin/env bash
set -euo pipefail

ENV_FILE="${1:-/opt/h2v2/.env.generated}"
MODE="${2:-}"

if [[ -f "${ENV_FILE}" ]]; then
  set -a
  # shellcheck disable=SC1090
  source "${ENV_FILE}"
  set +a
fi

log() {
  printf '[sync-hysteria-cert] %s\n' "$1"
}

fatal() {
  printf '[error] %s\n' "$1" >&2
  exit 1
}

: "${HY2_DOMAIN:?HY2_DOMAIN is required}"
HY2_CERT_PATH="${HY2_CERT_PATH:-/etc/h2v2/hysteria/tls.crt}"
HY2_KEY_PATH="${HY2_KEY_PATH:-/etc/h2v2/hysteria/tls.key}"
HY2_CERT_WAIT_TIMEOUT="${HY2_CERT_WAIT_TIMEOUT:-180}"
CADDY_CERT_SEARCH_ROOT="${CADDY_CERT_SEARCH_ROOT:-/var/lib/caddy}"

find_caddy_cert() {
  find "${CADDY_CERT_SEARCH_ROOT}" -type f -path "*/${HY2_DOMAIN}/${HY2_DOMAIN}.crt" 2>/dev/null | sort | head -n1
}

find_caddy_key() {
  find "${CADDY_CERT_SEARCH_ROOT}" -type f -path "*/${HY2_DOMAIN}/${HY2_DOMAIN}.key" 2>/dev/null | sort | head -n1
}

wait_for_caddy_cert() {
  local deadline=$((SECONDS + HY2_CERT_WAIT_TIMEOUT))
  while (( SECONDS <= deadline )); do
    CADDY_CERT_SOURCE="$(find_caddy_cert)"
    CADDY_KEY_SOURCE="$(find_caddy_key)"
    if [[ -n "${CADDY_CERT_SOURCE}" && -n "${CADDY_KEY_SOURCE}" ]]; then
      return 0
    fi
    sleep 2
  done
  return 1
}

copy_if_changed() {
  local source_path="$1"
  local target_path="$2"
  local changed_ref_name="$3"

  if [[ -f "${target_path}" ]] && cmp -s "${source_path}" "${target_path}"; then
    return 0
  fi

  install -D -o root -g h2v2 -m 0640 "${source_path}" "${target_path}"
  printf -v "${changed_ref_name}" '%s' 1
}

main() {
  local CADDY_CERT_SOURCE="$(find_caddy_cert)"
  local CADDY_KEY_SOURCE="$(find_caddy_key)"

  if [[ "${MODE}" == "--wait" ]]; then
    if ! wait_for_caddy_cert; then
      fatal "Timed out waiting for a Caddy-issued certificate for ${HY2_DOMAIN} under ${CADDY_CERT_SEARCH_ROOT}"
    fi
    CADDY_CERT_SOURCE="$(find_caddy_cert)"
    CADDY_KEY_SOURCE="$(find_caddy_key)"
  fi

  [[ -n "${CADDY_CERT_SOURCE}" ]] || fatal "Could not find Caddy certificate for ${HY2_DOMAIN}"
  [[ -n "${CADDY_KEY_SOURCE}" ]] || fatal "Could not find Caddy private key for ${HY2_DOMAIN}"

  local changed=0
  copy_if_changed "${CADDY_CERT_SOURCE}" "${HY2_CERT_PATH}" changed
  copy_if_changed "${CADDY_KEY_SOURCE}" "${HY2_KEY_PATH}" changed

  if [[ "${changed}" -eq 1 ]]; then
    log "Updated Hysteria TLS files from Caddy for ${HY2_DOMAIN}"
  else
    log "Hysteria TLS files already up to date for ${HY2_DOMAIN}"
  fi
}

main "$@"