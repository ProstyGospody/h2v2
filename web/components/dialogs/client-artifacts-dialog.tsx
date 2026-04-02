import { Copy } from "lucide-react";

import { qrURL } from "@/domain/clients/services";
import { HysteriaClient, HysteriaUserPayload, Protocol } from "@/domain/clients/types";
import { Button, Dialog } from "@/src/components/ui";

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
  const accessQRSource = currentClient?.id
    ? `${qrURL(currentClient.id, 320, "access")}&protocol=${preferredProtocol}`
    : "";
  const subscriptionQRSource = currentClient?.id
    ? `${qrURL(currentClient.id, 320, "subscription")}&protocol=${preferredProtocol}`
    : "";

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
        <div className="grid gap-5 sm:grid-cols-2">
          <div className="space-y-2">
            <label className="text-[12px] font-semibold uppercase tracking-wide text-txt-secondary">URL</label>
            <div className="w-fit rounded-lg bg-surface-2/65 p-2">
              <img
                src={accessQRSource}
                alt="URL"
                width={320}
                height={320}
                className="h-[220px] w-[220px] rounded-md object-contain sm:h-[320px] sm:w-[320px]"
              />
            </div>
            <Button className="w-full sm:w-auto" onClick={() => onCopy(accessURI)} disabled={!accessURI}>
              <Copy size={16} strokeWidth={1.6} />Copy
            </Button>
          </div>

          <div className="space-y-2">
            <label className="text-[12px] font-semibold uppercase tracking-wide text-txt-secondary">Subscription</label>
            <div className="w-fit rounded-lg bg-surface-2/65 p-2">
              <img
                src={subscriptionQRSource}
                alt="Subscription"
                width={320}
                height={320}
                className="h-[220px] w-[220px] rounded-md object-contain sm:h-[320px] sm:w-[320px]"
              />
            </div>
            <Button className="w-full sm:w-auto" onClick={() => onCopy(subscriptionURL)} disabled={!subscriptionURL}>
              <Copy size={16} strokeWidth={1.6} />Copy
            </Button>
          </div>
        </div>
      )}
    </Dialog>
  );
}
