import {
  Play,
  RefreshCw,
  RotateCcw,
  Save,
} from "lucide-react";
import { type ChangeEvent, useCallback, useEffect, useMemo, useRef, useState } from "react";

import { ConfirmPopover } from "@/components/dialogs/confirm-popover";
import { ServerSettingsForm } from "@/components/forms/server-settings-form";
import { ErrorBanner } from "@/components/ui/error-banner";
import { PageHeader } from "@/components/ui/page-header";
import { normalizeSettingsDraft, toSettingsDraft } from "@/domain/settings/adapters";
import {
  applyHysteriaSettings,
  downloadSQLiteBackup,
  getHysteriaSettings,
  restoreSQLiteBackup,
  saveHysteriaSettings,
  validateHysteriaSettings,
} from "@/domain/settings/services";
import { Hy2ConfigValidation, Hy2Settings } from "@/domain/settings/types";
import { APIError } from "@/services/api";
import { Button } from "@/src/components/ui";
import { useToast } from "@/src/components/ui/Toast";
import { setUnsavedChangesGuard } from "@/src/state/navigation-guard";

const SETTINGS_CACHE_KEY = "h2v2.settings.cache.v1";

type SettingsCachePayload = {
  raw_yaml: string;
  settings: Hy2Settings;
  config_validation: Hy2ConfigValidation | null;
};

function extractValidationError(err: unknown, fallback: string): string {
  if (!(err instanceof APIError)) return fallback;
  const details = err.details;
  if (!details || typeof details !== "object") return err.message;
  const maybeErrors = (details as { errors?: unknown }).errors;
  if (Array.isArray(maybeErrors)) {
    const errors = maybeErrors.filter((item): item is string => typeof item === "string" && item.trim().length > 0);
    if (errors.length > 0) return `${err.message}: ${errors.join(" | ")}`;
  }
  return err.message;
}

function readSettingsCache(): SettingsCachePayload | null {
  if (typeof window === "undefined") return null;
  try {
    const raw = window.localStorage.getItem(SETTINGS_CACHE_KEY);
    if (!raw) return null;
    const parsed = JSON.parse(raw) as Partial<SettingsCachePayload>;
    if (!parsed || typeof parsed !== "object" || !parsed.settings) return null;
    return {
      raw_yaml: typeof parsed.raw_yaml === "string" ? parsed.raw_yaml : "",
      settings: parsed.settings as Hy2Settings,
      config_validation: parsed.config_validation ?? null,
    };
  } catch {
    return null;
  }
}

function writeSettingsCache(payload: SettingsCachePayload) {
  if (typeof window === "undefined") return;
  try {
    window.localStorage.setItem(SETTINGS_CACHE_KEY, JSON.stringify(payload));
  } catch {
    // no-op
  }
}

function draftSnapshot(settings: Hy2Settings): string {
  return JSON.stringify(normalizeSettingsDraft(settings));
}

export default function ConfigPage() {
  const cached = readSettingsCache();
  const initialDraft = toSettingsDraft(
    cached?.settings || ({ listen: ":443", tlsEnabled: true, tlsMode: "acme", quicEnabled: false } as Hy2Settings),
  );

  const [loading, setLoading] = useState(!cached);
  const [reloading, setReloading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [validating, setValidating] = useState(false);
  const [applying, setApplying] = useState(false);
  const [storageBusy, setStorageBusy] = useState(false);
  const [restoreFile, setRestoreFile] = useState<File | null>(null);
  const [error, setError] = useState("");
  const [rawYaml, setRawYaml] = useState(cached?.raw_yaml || "");
  const [savedRawYaml, setSavedRawYaml] = useState(cached?.raw_yaml || "");
  const [draft, setDraft] = useState<Hy2Settings>(initialDraft);
  const [savedDraft, setSavedDraft] = useState<Hy2Settings>(initialDraft);
  const [validation, setValidation] = useState<Hy2ConfigValidation | null>(cached?.config_validation || null);
  const [savedValidation, setSavedValidation] = useState<Hy2ConfigValidation | null>(cached?.config_validation || null);

  const restoreInputRef = useRef<HTMLInputElement | null>(null);
  const toast = useToast();

  const isDirty = useMemo(() => draftSnapshot(draft) !== draftSnapshot(savedDraft), [draft, savedDraft]);
  const validationErrors = validation?.errors || [];
  const isBusy = loading || reloading || saving || validating || applying || storageBusy;

  const load = useCallback(async (showSkeleton = false) => {
    if (showSkeleton) {
      setLoading(true);
    } else {
      setReloading(true);
    }
    setError("");
    try {
      const payload = await getHysteriaSettings();
      const nextDraft = toSettingsDraft(payload.settings);
      const nextValidation = payload.config_validation || null;
      const nextYaml = payload.raw_yaml || "";

      setRawYaml(nextYaml);
      setSavedRawYaml(nextYaml);
      setDraft(nextDraft);
      setSavedDraft(nextDraft);
      setValidation(nextValidation);
      setSavedValidation(nextValidation);

      writeSettingsCache({
        raw_yaml: nextYaml,
        settings: payload.settings,
        config_validation: nextValidation,
      });
    } catch (err) {
      setError(err instanceof APIError ? err.message : "Failed to load settings");
    } finally {
      setLoading(false);
      setReloading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  useEffect(() => {
    setUnsavedChangesGuard(isDirty);
    return () => {
      setUnsavedChangesGuard(false);
    };
  }, [isDirty]);

  useEffect(() => {
    if (!isDirty) return;
    const onBeforeUnload = (event: BeforeUnloadEvent) => {
      event.preventDefault();
      event.returnValue = "";
    };
    window.addEventListener("beforeunload", onBeforeUnload);
    return () => {
      window.removeEventListener("beforeunload", onBeforeUnload);
    };
  }, [isDirty]);

  async function validateDraft() {
    setValidating(true);
    setError("");
    try {
      const payload = await validateHysteriaSettings(normalizeSettingsDraft(draft));
      setValidation(payload.config_validation || null);
      setRawYaml(payload.raw_yaml || rawYaml);
      if (payload.config_validation?.valid) {
        toast.notify("Valid");
      } else {
        toast.notify("Validation issues", "error");
      }
    } catch (err) {
      setError(extractValidationError(err, "Validate failed"));
    } finally {
      setValidating(false);
    }
  }

  async function saveDraft() {
    setSaving(true);
    setError("");
    try {
      const payload = await saveHysteriaSettings(normalizeSettingsDraft(draft));
      const nextDraft = toSettingsDraft(payload.settings);
      const nextValidation = payload.config_validation || null;
      const nextYaml = payload.raw_yaml || rawYaml;

      setDraft(nextDraft);
      setSavedDraft(nextDraft);
      setRawYaml(nextYaml);
      setSavedRawYaml(nextYaml);
      setValidation(nextValidation);
      setSavedValidation(nextValidation);

      writeSettingsCache({
        raw_yaml: nextYaml,
        settings: payload.settings,
        config_validation: nextValidation,
      });

      toast.notify(payload.backup_path ? `Saved: ${payload.backup_path}` : "Saved");
    } catch (err) {
      setError(extractValidationError(err, "Save failed"));
    } finally {
      setSaving(false);
    }
  }

  async function applyConfig() {
    setApplying(true);
    setError("");
    try {
      await applyHysteriaSettings();
      toast.notify("Applied");
      await load();
    } catch (err) {
      setError(extractValidationError(err, "Apply failed"));
    } finally {
      setApplying(false);
    }
  }

  function discardChanges() {
    setDraft(toSettingsDraft(savedDraft));
    setRawYaml(savedRawYaml);
    setValidation(savedValidation);
    setError("");
  }

  async function backupSQLite() {
    setStorageBusy(true);
    setError("");
    try {
      const fileName = await downloadSQLiteBackup();
      toast.notify(`Backup: ${fileName}`);
    } catch (err) {
      setError(extractValidationError(err, "Backup failed"));
    } finally {
      setStorageBusy(false);
    }
  }

  function triggerRestorePicker() {
    if (isBusy) return;
    restoreInputRef.current?.click();
  }

  function onRestoreFileSelected(event: ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0];
    event.target.value = "";
    if (!file) return;
    setRestoreFile(file);
  }

  async function restoreSQLite() {
    if (!restoreFile) return;
    setStorageBusy(true);
    setError("");
    try {
      await restoreSQLiteBackup(restoreFile);
      setRestoreFile(null);
      toast.notify("Restored");
      await load();
    } catch (err) {
      setError(extractValidationError(err, "Restore failed"));
    } finally {
      setStorageBusy(false);
    }
  }

  return (
    <div className="space-y-5 pb-40 sm:pb-24">
      <PageHeader title="Settings" />
      <input ref={restoreInputRef} type="file" accept=".db,application/octet-stream" className="hidden" onChange={onRestoreFileSelected} />

      <ErrorBanner message={error} onDismiss={() => setError("")} />

      {loading ? (
        <div className="grid gap-4 md:grid-cols-2">
          {Array.from({ length: 4 }, (_, index) => (
            <section key={index} className="panel-card min-h-[168px] animate-pulse space-y-3">
              <div className="h-4 w-28 rounded bg-surface-3/55" />
              <div className="h-10 rounded bg-surface-3/45" />
              <div className="h-10 rounded bg-surface-3/45" />
            </section>
          ))}
        </div>
      ) : (
        <ServerSettingsForm
          draft={draft}
          rawYaml={rawYaml}
          onDraftChange={setDraft}
          snapshotStorage={{
            busy: isBusy,
            restoreFileName: restoreFile?.name || "",
            onBackup: () => void backupSQLite(),
            onSelectRestore: triggerRestorePicker,
            onRestore: () => restoreSQLite(),
          }}
        />
      )}

      <div className="fixed inset-x-0 bottom-0 z-40 px-2 pb-2 sm:bottom-4 sm:left-1/2 sm:w-[calc(100vw-16px)] sm:max-w-[980px] sm:-translate-x-1/2 sm:px-0 sm:pb-0">
        <div className="flex flex-col gap-2 rounded-2xl bg-surface-2/95 p-2 shadow-[0_20px_46px_-12px_var(--dialog-shadow)] backdrop-blur-xl sm:flex-row sm:flex-wrap sm:items-center sm:gap-2 sm:px-3 sm:py-2.5">
          <span className="inline-flex h-9 items-center justify-center rounded-lg bg-surface-3/45 px-2.5 py-1 text-[12px] font-medium text-txt-secondary sm:h-auto sm:w-auto sm:justify-start">
            {isDirty ? "Unsaved changes" : "Up to date"}
          </span>
          <div className="grid w-full grid-cols-2 gap-2 sm:ml-auto sm:flex sm:w-auto sm:flex-wrap sm:items-center sm:gap-2">
            <Button onClick={() => void load(false)} disabled={isBusy} className="h-10 w-full justify-center sm:h-10 sm:w-auto">
              <RefreshCw size={15} strokeWidth={1.8} />
              Reload
            </Button>
            <Button onClick={discardChanges} disabled={isBusy || !isDirty} className="h-10 w-full justify-center sm:h-10 sm:w-auto">
              <RotateCcw size={15} strokeWidth={1.8} />
              Discard
            </Button>
            <Button variant="primary" onClick={() => void saveDraft()} disabled={isBusy} className="h-10 w-full justify-center sm:h-10 sm:w-auto">
              <Save size={15} strokeWidth={1.8} />
              Save
            </Button>
            <ConfirmPopover
              title="Apply config"
              description="Restart hysteria-server?"
              confirmText="Apply"
              onConfirm={() => void applyConfig()}
            >
              <Button variant="primary" disabled={isBusy || Boolean(validationErrors.length)} className="h-10 w-full justify-center sm:h-10 sm:w-auto">
                <Play size={15} strokeWidth={1.8} />
                Apply
              </Button>
            </ConfirmPopover>
          </div>
        </div>
      </div>
    </div>
  );
}
