import { type Hy2Settings } from "@/domain/settings/types";

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
