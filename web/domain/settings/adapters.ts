import { Hy2Settings } from "@/domain/settings/types";

function normalizeListenForDraft(value: string | undefined): string {
  const listen = (value || "").trim();
  if (!listen) {
    return "443";
  }
  if (listen.startsWith(":")) {
    return listen.slice(1);
  }
  return listen;
}

export function toSettingsDraft(settings: Hy2Settings): Hy2Settings {
  return {
    listen: normalizeListenForDraft(settings.listen),
    tlsEnabled: settings.tlsEnabled,
    tlsMode: settings.tlsMode || "acme",
    clientTLSInsecure: Boolean(settings.clientTLSInsecure),
    tls: settings.tls || { cert: "", key: "" },
    acme: settings.acme || { domains: [], email: "" },
    obfs: settings.obfs || undefined,
    masquerade: settings.masquerade || undefined,
    bandwidth: settings.bandwidth || undefined,
    ignoreClientBandwidth: Boolean(settings.ignoreClientBandwidth),
    speedTest: Boolean(settings.speedTest),
    disableUDP: Boolean(settings.disableUDP),
    udpIdleTimeout: settings.udpIdleTimeout || "",
    quicEnabled: settings.quicEnabled,
    quic: settings.quic || undefined,
  };
}

export function normalizeSettingsDraft(draft: Hy2Settings): Hy2Settings {
  const next = toSettingsDraft(draft);

  if (next.acme) {
    next.acme.domains = (next.acme.domains || []).map((item) => item.trim()).filter(Boolean);
    next.acme.email = (next.acme.email || "").trim();
  }

  if (next.tls) {
    next.tls.cert = (next.tls.cert || "").trim();
    next.tls.key = (next.tls.key || "").trim();
  }

  if (next.obfs) {
    const password = next.obfs.salamander?.password?.trim() || "";
    if (!next.obfs.type && !password) {
      next.obfs = undefined;
    }
  }

  if (next.bandwidth) {
    const up = next.bandwidth.up?.trim() || "";
    const down = next.bandwidth.down?.trim() || "";
    if (!up && !down) {
      next.bandwidth = undefined;
    } else {
      next.bandwidth = { up, down };
    }
  }

  if (next.udpIdleTimeout) {
    next.udpIdleTimeout = next.udpIdleTimeout.trim();
  }

  next.clientTLSInsecure = Boolean(next.clientTLSInsecure);

  return next;
}
