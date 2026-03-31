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
import { type Hy2ConfigValidation, type Hy2Settings } from "@/domain/settings/types";
import { APIError } from "@/services/api";
import { Button, Tooltip } from "@/src/components/ui";
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

type ConfigHeaderActionsProps = {
  isBusy: boolean;
  isDirty: boolean;
  saving: boolean;
  applying: boolean;
  reloading: boolean;
  hasValidationErrors: boolean;
  onReload: () => void;
  onDiscard: () => void;
  onSave: () => void;
  onApply: () => void;
};

function ConfigHeaderActions({
  isBusy,
  isDirty,
  saving,
  applying,
  reloading,
  hasValidationErrors,
  onReload,
  onDiscard,
  onSave,
  onApply,
}: ConfigHeaderActionsProps) {
  return (
    <>
      <span className="header-btn inline-flex w-full items-center rounded-2xl bg-surface-2/70 px-3 text-[13px] font-medium text-txt-secondary shadow-[inset_0_1px_0_var(--shell-highlight)] sm:w-auto">
        {isDirty ? "Unsaved changes" : "Up to date"}
      </span>

      <Button loading={reloading} onClick={onReload} disabled={isBusy} className="header-btn w-full rounded-2xl px-4 sm:w-auto">
        <RefreshCw size={15} strokeWidth={1.8} />
        Reload
      </Button>

      <Button onClick={onDiscard} disabled={isBusy || !isDirty} className="header-btn w-full rounded-2xl px-4 sm:w-auto">
        <RotateCcw size={15} strokeWidth={1.8} />
        Discard
      </Button>

      <Tooltip content="Save">
        <Button variant="primary" loading={saving} onClick={onSave} disabled={isBusy} className="header-btn w-full rounded-2xl px-4 sm:w-auto">
          <Save size={15} strokeWidth={1.8} />
          Save
        </Button>
      </Tooltip>

      <Tooltip content="Apply">
        <span className="inline-flex w-full sm:w-auto">
          <ConfirmPopover
            title="Apply config"
            description="Restart hysteria-server?"
            confirmText="Apply"
            onConfirm={onApply}
          >
            <Button
              variant="primary"
              loading={applying}
              disabled={isBusy || hasValidationErrors || isDirty}
              className="header-btn w-full rounded-2xl px-4 sm:w-auto"
            >
              <Play size={15} strokeWidth={1.8} />
              Apply
            </Button>
          </ConfirmPopover>
        </span>
      </Tooltip>
    </>
  );
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
    <div className="space-y-6">
      <PageHeader
        title="Settings"
        actions={
          <ConfigHeaderActions
            isBusy={isBusy}
            isDirty={isDirty}
            saving={saving}
            applying={applying}
            reloading={reloading}
            hasValidationErrors={Boolean(validationErrors.length)}
            onReload={() => void load(false)}
            onDiscard={discardChanges}
            onSave={() => void saveDraft()}
            onApply={() => void applyConfig()}
          />
        }
      />
      <input ref={restoreInputRef} type="file" accept=".db,application/octet-stream" className="hidden" onChange={onRestoreFileSelected} />

      <ErrorBanner message={error} onDismiss={() => setError("")} />

      {loading ? (
        <div className="grid gap-4 xl:grid-cols-12">
          <div className="min-w-0 space-y-4 xl:col-span-8">
            {Array.from({ length: 4 }, (_, index) => (
              <section key={index} className="panel-card min-h-[168px] animate-pulse space-y-4">
                <div className="flex items-center gap-3">
                  <div className="h-9 w-9 rounded-xl bg-surface-3/55" />
                  <div className="h-4 w-24 rounded bg-surface-3/55" />
                </div>
                <div className="grid gap-3 md:grid-cols-2">
                  <div className="space-y-2"><div className="h-3 w-16 rounded bg-surface-3/45" /><div className="h-10 w-full rounded-lg bg-surface-3/45" /></div>
                  <div className="space-y-2"><div className="h-3 w-20 rounded bg-surface-3/45" /><div className="h-10 w-full rounded-lg bg-surface-3/45" /></div>
                </div>
              </section>
            ))}
          </div>
          <aside className="min-w-0 space-y-4 xl:col-span-4">
            <section className="panel-card-compact animate-pulse space-y-3">
              <div className="flex items-center gap-3">
                <div className="h-9 w-9 rounded-xl bg-surface-3/55" />
                <div className="h-4 w-20 rounded bg-surface-3/55" />
              </div>
              <div className="space-y-2">
                {Array.from({ length: 4 }, (_, i) => <div key={i} className="h-8 w-full rounded-lg bg-surface-3/35" />)}
              </div>
              <div className="h-9 w-full rounded-lg bg-surface-3/45" />
            </section>
            <section className="panel-card-compact animate-pulse space-y-3">
              <div className="flex items-center gap-3">
                <div className="h-9 w-9 rounded-xl bg-surface-3/55" />
                <div className="h-4 w-16 rounded bg-surface-3/55" />
              </div>
              <div className="h-40 w-full rounded-xl bg-surface-3/35" />
            </section>
          </aside>
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
    </div>
  );
}
