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
      onOpenChange={(next) => {
        if (!next && !busy) {
          onClose();
        }
      }}
      title={title}
      footer={
        <>
          <Button onClick={onClose} disabled={busy}>
            Cancel
          </Button>
          <Button variant={variant} onClick={onConfirm} disabled={busy}>
            {busy ? (
              <>
                <Loader2 size={14} strokeWidth={1.6} className="animate-spin" />
                Processing...
              </>
            ) : (
              confirmText || "Confirm"
            )}
          </Button>
        </>
      }
    >
      <div className="flex items-start gap-3">
        <div className="grid h-9 w-9 shrink-0 place-items-center rounded-full bg-status-warning/10">
          <AlertTriangle size={17} strokeWidth={1.4} className="text-status-warning" />
        </div>
        <p className="pt-1.5 text-[13px] leading-relaxed text-txt-secondary">{description}</p>
      </div>
    </Dialog>
  );
}
