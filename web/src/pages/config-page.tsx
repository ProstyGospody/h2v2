import {
  AlertTriangle,
  CheckCircle2,
  Clock3,
  Database,
  Download,
  Play,
  RefreshCw,
  RotateCcw,
  Save,
  Upload,
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
import { Button, cn } from "@/src/components/ui";
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

function formatSavedAt(value: number | null): string {
  if (!value) return "--";
  return new Date(value).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
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
  const [savedAt, setSavedAt] = useState<number | null>(cached ? Date.now() : null);

  const restoreInputRef = useRef<HTMLInputElement | null>(null);
  const toast = useToast();

  const isDirty = useMemo(() => draftSnapshot(draft) !== draftSnapshot(savedDraft), [draft, savedDraft]);
  const validationErrors = validation?.errors || [];
  const validationWarnings = validation?.warnings || [];
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
      setSavedAt(Date.now());

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
      setSavedAt(Date.now());

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

  const statusLabel = applying
    ? "Applying"
    : reloading
      ? "Reloading"
    : saving
      ? "Saving"
      : validating
        ? "Validating"
        : isDirty
          ? "Unsaved"
          : "Saved";

  return (
    <div className="space-y-5 pb-28 sm:pb-24">
      <PageHeader title="Settings" />
      <input ref={restoreInputRef} type="file" accept=".db,application/octet-stream" className="hidden" onChange={onRestoreFileSelected} />

      <section className="panel-card-compact flex flex-wrap items-center gap-3">
        <span
          className={cn(
            "inline-flex items-center rounded-lg px-2.5 py-1 text-[12px] font-semibold",
            statusLabel === "Saved" && "bg-status-success/12 text-status-success",
            statusLabel === "Unsaved" && "bg-status-warning/12 text-status-warning",
            statusLabel !== "Saved" && statusLabel !== "Unsaved" && "bg-status-info/12 text-status-info",
          )}
        >
          {statusLabel}
        </span>
        <span className="inline-flex items-center gap-1.5 text-[13px] text-txt-secondary">
          <Clock3 size={14} strokeWidth={1.7} />
          {formatSavedAt(savedAt)}
        </span>
        <span className="ml-auto inline-flex items-center gap-2 rounded-lg bg-surface-3/40 px-2.5 py-1 text-[12px] text-txt-secondary">
          <AlertTriangle size={13} strokeWidth={1.8} />
          {validationErrors.length}
          <span className="text-txt-muted">/</span>
          {validationWarnings.length}
        </span>
      </section>

      <ErrorBanner message={error} onDismiss={() => setError("")} />

      <div className="grid gap-4 xl:grid-cols-12">
        <section className="panel-card-compact space-y-3 xl:col-span-4">
          <div className="flex items-center gap-2 text-[14px] font-semibold text-txt-primary">
            <Database size={16} strokeWidth={1.8} />
            Storage
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <Button onClick={() => void backupSQLite()} disabled={isBusy}>
              <Download size={16} strokeWidth={1.7} />
              Backup
            </Button>
            {restoreFile ? (
              <>
                <Button onClick={triggerRestorePicker} disabled={isBusy}>
                  <Upload size={16} strokeWidth={1.7} />
                  Select DB
                </Button>
                <ConfirmPopover
                  title="Restore database"
                  description={`Restore ${restoreFile.name}?`}
                  confirmText="Restore"
                  onConfirm={() => void restoreSQLite()}
                >
                  <Button variant="danger" disabled={isBusy}>
                    <Upload size={16} strokeWidth={1.7} />
                    Restore
                  </Button>
                </ConfirmPopover>
              </>
            ) : (
              <Button variant="danger" onClick={triggerRestorePicker} disabled={isBusy}>
                <Upload size={16} strokeWidth={1.7} />
                Restore
              </Button>
            )}
          </div>
          {restoreFile ? (
            <div className="rounded-lg bg-surface-3/35 px-3 py-2 text-[12px] text-txt-secondary">
              {restoreFile.name}
            </div>
          ) : null}
        </section>

        <section className="panel-card-compact space-y-2.5 xl:col-span-8">
          <div className="flex items-center gap-2 text-[14px] font-semibold text-txt-primary">
            <CheckCircle2 size={16} strokeWidth={1.8} />
            Validation
          </div>
          {validationErrors.length ? (
            <div className="rounded-lg bg-status-danger/10 px-3 py-2 text-[13px] text-status-danger">
              {validationErrors.slice(0, 3).join(" | ")}
            </div>
          ) : (
            <div className="rounded-lg bg-status-success/10 px-3 py-2 text-[13px] text-status-success">No errors</div>
          )}
          {validationWarnings.length ? (
            <div className="rounded-lg bg-status-warning/10 px-3 py-2 text-[13px] text-status-warning">
              {validationWarnings.slice(0, 3).join(" | ")}
            </div>
          ) : null}
        </section>
      </div>

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
        <ServerSettingsForm draft={draft} rawYaml={rawYaml} onDraftChange={setDraft} />
      )}

      <div className="fixed bottom-4 left-1/2 z-40 w-[min(980px,calc(100vw-14px))] -translate-x-1/2">
        <div className="flex flex-wrap items-center gap-2 rounded-2xl bg-surface-2/95 px-3 py-2.5 shadow-[0_20px_46px_-12px_var(--dialog-shadow)] backdrop-blur-xl">
          <span className="inline-flex items-center rounded-lg bg-surface-3/45 px-2.5 py-1 text-[12px] font-medium text-txt-secondary">
            {isDirty ? "Unsaved changes" : "Up to date"}
          </span>
          <div className="ml-auto flex flex-wrap items-center gap-2">
            <Button onClick={() => void load(false)} disabled={isBusy}>
              <RefreshCw size={15} strokeWidth={1.8} />
              Reload
            </Button>
            <Button onClick={discardChanges} disabled={isBusy || !isDirty}>
              <RotateCcw size={15} strokeWidth={1.8} />
              Discard
            </Button>
            <Button variant="primary" onClick={() => void saveDraft()} disabled={isBusy}>
              <Save size={15} strokeWidth={1.8} />
              Save
            </Button>
            <ConfirmPopover
              title="Apply config"
              description="Restart hysteria-server?"
              confirmText="Apply"
              onConfirm={() => void applyConfig()}
            >
              <Button variant="primary" disabled={isBusy || Boolean(validationErrors.length)}>
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
