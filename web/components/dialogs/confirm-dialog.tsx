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
  const variant = confirmColor === "error" ? "danger" : confirmColor === "secondary" ? "primary" : "primary";

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
            {busy ? "Processing..." : confirmText || "Confirm"}
          </Button>
        </>
      }
    >
      <p className="text-[12px] text-txt-secondary">{description}</p>
    </Dialog>
  );
}
