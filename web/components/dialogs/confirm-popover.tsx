import * as Popover from "@radix-ui/react-popover";
import { AlertTriangle, Loader2 } from "lucide-react";
import { type ReactNode, useRef, useState } from "react";

import { Button } from "@/src/components/ui";

export function ConfirmPopover({
  children,
  title,
  description,
  confirmText = "Confirm",
  onConfirm,
}: {
  children: ReactNode;
  title: string;
  description: string;
  confirmText?: string;
  onConfirm: () => void | Promise<void>;
}) {
  const [open, setOpen] = useState(false);
  const [busy, setBusy] = useState(false);
  const cancelRef = useRef<HTMLButtonElement | null>(null);

  async function handleConfirm() {
    setBusy(true);
    try {
      await onConfirm();
      setOpen(false);
    } finally {
      setBusy(false);
    }
  }

  return (
    <Popover.Root open={open} onOpenChange={(v) => !busy && setOpen(v)}>
      <Popover.Trigger asChild>{children}</Popover.Trigger>
      <Popover.Portal>
        <Popover.Content
          sideOffset={8}
          align="end"
          onOpenAutoFocus={(event) => {
            event.preventDefault();
            cancelRef.current?.focus();
          }}
          className="z-50 w-[min(260px,calc(100vw-24px))] rounded-xl bg-surface-2/95 p-4 shadow-[0_18px_42px_-16px_var(--dialog-shadow)] backdrop-blur-xl"
        >
          <div className="flex items-start gap-3">
            <div className="grid h-8 w-8 shrink-0 place-items-center rounded-lg bg-status-warning/10">
              <AlertTriangle size={16} strokeWidth={1.6} className="text-status-warning" />
            </div>
            <div>
              <p className="text-[15px] font-semibold text-txt-primary">{title}</p>
              <p className="mt-1 text-[14px] leading-relaxed text-txt-secondary">{description}</p>
            </div>
          </div>
          <div className="mt-3 flex items-center justify-end gap-2">
            <Button ref={cancelRef} size="sm" onClick={() => setOpen(false)} disabled={busy}>Cancel</Button>
            <Button size="sm" variant="danger" onClick={() => void handleConfirm()} disabled={busy}>
              {busy ? <Loader2 size={14} className="animate-spin" /> : null}
              {confirmText}
            </Button>
          </div>
          <Popover.Arrow className="fill-surface-2/95" />
        </Popover.Content>
      </Popover.Portal>
    </Popover.Root>
  );
}
