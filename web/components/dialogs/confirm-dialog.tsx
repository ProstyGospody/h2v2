import { AlertTriangle, Loader2 } from "lucide-react";

import { Button, Dialog } from "@/src/components/ui";

export function ConfirmDialog({
  open,
  title,
  description,
  busy,
  confirmText,
  onConfirm,
  onClose,
  confirmColor = "error",
}: {
  open: boolean;
  title: string;
  description: string;
  busy?: boolean;
  confirmText?: string;
  onConfirm: () => void;
  onClose: () => void;
  confirmColor?: "error" | "primary" | "secondary";
}) {
  const variant = confirmColor === "error" ? "danger" : "primary";

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => { if (!next && !busy) onClose(); }}
      title={title}
      footer={
        <>
          <Button onClick={onClose} disabled={busy}>Cancel</Button>
          <Button variant={variant} onClick={onConfirm} disabled={busy}>
            {busy ? <><Loader2 size={16} strokeWidth={1.8} className="animate-spin" />Processing...</> : confirmText || "Confirm"}
          </Button>
        </>
      }
    >
      <div className="flex items-start gap-4">
        <div className="grid h-11 w-11 shrink-0 place-items-center rounded-full bg-status-warning/10">
          <AlertTriangle size={20} strokeWidth={1.6} className="text-status-warning" />
        </div>
        <p className="pt-2 text-[15px] leading-relaxed text-txt-secondary">{description}</p>
      </div>
    </Dialog>
  );
}
