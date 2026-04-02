import { Copy, Link2 } from "lucide-react";

import { HysteriaClient, HysteriaUserPayload, Protocol } from "@/domain/clients/types";
import { Button, Dialog, Input } from "@/src/components/ui";

function resolveAccessURI(payload: HysteriaUserPayload | null, preferredProtocol: Protocol): string {
  if (!payload?.artifacts) {
    return "";
  }
  const unified = payload.artifacts.unified;
  if (preferredProtocol === "vless") {
    if (unified?.vless?.access_uri) {
      return unified.vless.access_uri;
    }
    if (payload.artifacts.uri) {
      return payload.artifacts.uri;
    }
    if (payload.artifacts.uri_hy2) {
      return payload.artifacts.uri_hy2;
    }
    if (unified?.hy2?.access_uri) {
      return unified.hy2.access_uri;
    }
    return "";
  }
  if (payload.artifacts.uri_hy2) {
    return payload.artifacts.uri_hy2;
  }
  if (payload.artifacts.uri) {
    return payload.artifacts.uri;
  }
  if (unified?.hy2?.access_uri) {
    return unified.hy2.access_uri;
  }
  if (unified?.vless?.access_uri) {
    return unified.vless.access_uri;
  }
  return "";
}

export function ClientArtifactsDialog({
  open, client, payload, loading, onClose, onCopy,
}: {
  open: boolean;
  client: HysteriaClient | null;
  payload: HysteriaUserPayload | null;
  loading: boolean;
  onClose: () => void;
  onCopy: (value: string) => void;
}) {
  const currentClient = payload?.user || client;
  const preferredProtocol: Protocol = (currentClient?.preferred_protocol || "hy2") as Protocol;
  const accessURI = resolveAccessURI(payload, preferredProtocol);
  const subscriptionURL = payload?.artifacts?.subscription_url || "";
  const configBody = preferredProtocol === "vless"
    ? (payload?.artifacts?.unified?.vless?.config || payload?.artifacts?.client_config || "")
    : (payload?.artifacts?.client_config || payload?.artifacts?.unified?.hy2?.config || payload?.artifacts?.unified?.vless?.config || "");

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => { if (!next) onClose(); }}
      title={currentClient?.username || "User"}
      contentClassName="max-w-[860px]"
      footer={<Button onClick={onClose}>Close</Button>}
    >
      {loading ? (
        <div className="flex min-h-[260px] items-center justify-center text-[14px] text-txt-secondary">Loading</div>
      ) : (
        <div className="space-y-4">
          <div className="space-y-2">
            <label className="text-[12px] font-semibold uppercase tracking-wide text-txt-secondary">URI</label>
            <Input value={accessURI} readOnly />
            <Button className="w-full sm:w-auto" onClick={() => onCopy(accessURI)} disabled={!accessURI}>
              <Copy size={16} strokeWidth={1.6} />Copy URI
            </Button>
          </div>

          <div className="space-y-2">
            <label className="text-[12px] font-semibold uppercase tracking-wide text-txt-secondary">Subscription</label>
            <Input value={subscriptionURL} readOnly />
            <Button variant="ghost" className="w-full sm:w-auto" onClick={() => onCopy(subscriptionURL)} disabled={!subscriptionURL}>
              <Link2 size={16} strokeWidth={1.6} />Copy URL
            </Button>
          </div>

          {configBody ? (
            <div className="space-y-2">
              <label className="text-[12px] font-semibold uppercase tracking-wide text-txt-secondary">Config</label>
              <textarea
                readOnly
                value={configBody}
                rows={10}
                className="w-full rounded-lg bg-surface-2/65 px-4 py-3 text-[13px] text-txt-primary outline-none"
              />
            </div>
          ) : null}
        </div>
      )}
    </Dialog>
  );
}
