import { createContext, useCallback, useContext, useRef, useState, type ReactNode } from "react";

import { Button } from "./Button";
import { Dialog } from "./Dialog";

type ConfirmOptions = {
  title: string;
  description: string;
  confirmText?: string;
  cancelText?: string;
};

type ConfirmContextValue = {
  confirm: (options: ConfirmOptions) => Promise<boolean>;
};

const ConfirmContext = createContext<ConfirmContextValue | null>(null);

export function ConfirmDialogProvider({ children }: { children: ReactNode }) {
  const [open, setOpen] = useState(false);
  const [options, setOptions] = useState<ConfirmOptions>({ title: "", description: "" });
  const resolveRef = useRef<((value: boolean) => void) | null>(null);

  const confirm = useCallback((opts: ConfirmOptions) => {
    setOptions(opts);
    setOpen(true);
    return new Promise<boolean>((resolve) => {
      resolveRef.current = resolve;
    });
  }, []);

  function handleClose(result: boolean) {
    setOpen(false);
    resolveRef.current?.(result);
    resolveRef.current = null;
  }

  return (
    <ConfirmContext.Provider value={{ confirm }}>
      {children}
      <Dialog
        open={open}
        onOpenChange={(value) => { if (!value) handleClose(false); }}
        title={options.title}
        description={options.description}
        footer={
          <div className="flex items-center justify-end gap-3">
            <Button size="sm" onClick={() => handleClose(false)} autoFocus>{options.cancelText || "Cancel"}</Button>
            <Button size="sm" variant="danger" onClick={() => handleClose(true)}>{options.confirmText || "Confirm"}</Button>
          </div>
        }
      />
    </ConfirmContext.Provider>
  );
}

export function useConfirmDialog(): ConfirmContextValue {
  const ctx = useContext(ConfirmContext);
  if (!ctx) throw new Error("useConfirmDialog must be used within ConfirmDialogProvider");
  return ctx;
}
