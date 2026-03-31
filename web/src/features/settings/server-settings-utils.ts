import { type Hy2Settings } from "@/domain/settings/types";

export function escapeHtml(value: string): string {
  return value.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
}

export function highlightYaml(yaml: string): string {
  return yaml
    .split("\n")
    .map((line) => {
      if (/^\s*#/.test(line)) {
        return `<span class="text-txt-muted">${escapeHtml(line)}</span>`;
      }
      const matched = line.replace(
        /^(\s*)([\w./-]+)(:)(.*)/,
        (_match, indent, key, colon, rest) => {
          const restTrim = rest.trim();
          let value = escapeHtml(rest);
          if (/^\s*(true|false)$/i.test(rest)) {
            value = ` <span class="text-status-warning">${escapeHtml(restTrim)}</span>`;
          } else if (/^\s*\d+(\.\d+)?$/.test(rest)) {
            value = ` <span class="text-status-info">${escapeHtml(restTrim)}</span>`;
          } else if (restTrim.length > 0) {
            value = ` <span class="text-status-success">${escapeHtml(restTrim)}</span>`;
          }
          return `${escapeHtml(indent)}<span class="text-accent">${escapeHtml(key)}</span><span class="text-txt-muted">${escapeHtml(colon)}</span>${value}`;
        },
      );
      return matched === line ? escapeHtml(line) : matched;
    })
    .join("\n");
}

export function buildSnapshotItems(
  draft: Hy2Settings,
  tlsMode: "acme" | "tls",
  obfsType: "none" | "salamander",
  masqueradeType: string,
) {
  return [
    { label: "Listen", value: draft.listen || "-" },
    { label: "TLS", value: tlsMode.toUpperCase() },
    { label: "Masking", value: obfsType !== "none" ? "OBFS" : masqueradeType !== "none" ? masqueradeType : "None" },
    { label: "QUIC", value: draft.quicEnabled ? "On" : "Off" },
  ];
}
