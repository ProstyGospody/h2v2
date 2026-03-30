import { Download, Play, Save, Upload } from "lucide-react";
import { type ChangeEvent, useCallback, useEffect, useRef, useState } from "react";

import { ConfirmPopover } from "@/components/dialogs/confirm-popover";
import { ServerSettingsForm } from "@/components/forms/server-settings-form";
import { PageHeader } from "@/components/ui/page-header";
import { normalizeSettingsDraft, toSettingsDraft } from "@/domain/settings/adapters";
import {
  applyHysteriaSettings,
  downloadSQLiteBackup,
  getHysteriaSettings,
  restoreSQLiteBackup,
  saveHysteriaSettings,
} from "@/domain/settings/services";
import { Hy2ConfigValidation, Hy2Settings } from "@/domain/settings/types";
import { APIError } from "@/services/api";
import { Button, Toast } from "@/src/components/ui";

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

export default function ConfigPage() {
  const cached = readSettingsCache();
  const [loading, setLoading] = useState(!cached);
  const [busy, setBusy] = useState(false);
  const [applying, setApplying] = useState(false);
  const [restoreFile, setRestoreFile] = useState<File | null>(null);
  const [error, setError] = useState("");
  const [snack, setSnack] = useState("");
  const [rawYaml, setRawYaml] = useState(cached?.raw_yaml || "");
  const [storageBusy, setStorageBusy] = useState(false);
  const [draft, setDraft] = useState<Hy2Settings>(() => toSettingsDraft(cached?.settings || { listen: ":443", tlsEnabled: true, tlsMode: "acme", quicEnabled: false } as Hy2Settings));
  const [validation, setValidation] = useState<Hy2ConfigValidation | null>(cached?.config_validation || null);
  const restoreInputRef = useRef<HTMLInputElement | null>(null);

  const load = useCallback(async () => {
    setError("");
    try {
      const p = await getHysteriaSettings();
      setRawYaml(p.raw_yaml || "");
      setDraft(toSettingsDraft(p.settings));
      setValidation(p.config_validation || null);
      writeSettingsCache({
        raw_yaml: p.raw_yaml || "",
        settings: p.settings,
        config_validation: p.config_validation || null,
      });
    } catch (err) {
      setError(err instanceof APIError ? err.message : "Failed to load server settings");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { void load(); }, [load]);

  async function saveDraft() {
    setBusy(true); setError("");
    try {
      const p = await saveHysteriaSettings(normalizeSettingsDraft(draft));
      setDraft(toSettingsDraft(p.settings)); setRawYaml(p.raw_yaml || rawYaml); setValidation(p.config_validation || null);
      writeSettingsCache({
        raw_yaml: p.raw_yaml || rawYaml,
        settings: p.settings,
        config_validation: p.config_validation || null,
      });
      setSnack(p.backup_path ? `Saved. Backup: ${p.backup_path}` : "Settings saved");
    } catch (err) { setError(extractValidationError(err, "Save failed")); }
    finally { setBusy(false); }
  }

  async function applyConfig() {
    setApplying(true); setError("");
    try { await applyHysteriaSettings(); setSnack("Hysteria restarted"); await load(); }
    catch (err) { setError(extractValidationError(err, "Apply failed")); }
    finally { setApplying(false); }
  }

  async function backupSQLite() {
    setStorageBusy(true); setError("");
    try {
      const fileName = await downloadSQLiteBackup();
      setSnack(`Backup downloaded: ${fileName}`);
    } catch (err) {
      setError(extractValidationError(err, "Backup failed"));
    } finally {
      setStorageBusy(false);
    }
  }

  function triggerRestorePicker() {
    if (busy || applying || storageBusy) return;
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
    setStorageBusy(true); setError("");
    try {
      await restoreSQLiteBackup(restoreFile);
      setRestoreFile(null);
      setSnack("Database restored");
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
          <>
            <Button onClick={() => void backupSQLite()} disabled={loading || busy || applying || storageBusy} className="h-12 w-full rounded-2xl px-5 sm:w-auto"><Download size={18} strokeWidth={1.6} />Backup</Button>
            {restoreFile ? (
              <>
                <Button onClick={triggerRestorePicker} disabled={loading || busy || applying || storageBusy} className="h-12 w-full rounded-2xl px-5 sm:w-auto"><Upload size={18} strokeWidth={1.6} />Select DB</Button>
                <ConfirmPopover
                  title="Restore database"
                  description={`Restore from ${restoreFile.name}?`}
                  confirmText="Restore"
                  onConfirm={() => void restoreSQLite()}
                >
                  <Button variant="danger" disabled={loading || busy || applying || storageBusy} className="h-12 w-full rounded-2xl px-5 sm:w-auto"><Upload size={18} strokeWidth={1.6} />Restore</Button>
                </ConfirmPopover>
              </>
            ) : (
              <Button variant="danger" onClick={triggerRestorePicker} disabled={loading || busy || applying || storageBusy} className="h-12 w-full rounded-2xl px-5 sm:w-auto"><Upload size={18} strokeWidth={1.6} />Restore</Button>
            )}
            <Button variant="primary" onClick={() => void saveDraft()} disabled={loading || busy || applying || storageBusy} className="h-12 w-full rounded-2xl px-5 sm:w-auto"><Save size={18} strokeWidth={1.6} />Save</Button>
            <ConfirmPopover
              title="Apply configuration"
              description="Restart hysteria-server?"
              confirmText="Apply"
              onConfirm={() => void applyConfig()}
            >
              <Button variant="primary" disabled={loading || busy || applying || storageBusy} className="h-12 w-full rounded-2xl px-5 sm:w-auto"><Play size={18} strokeWidth={1.6} />Apply</Button>
            </ConfirmPopover>
          </>
        }
      />
      <input ref={restoreInputRef} type="file" accept=".db,application/octet-stream" className="hidden" onChange={onRestoreFileSelected} />

      {error && <div className="rounded-xl border border-status-danger/20 bg-status-danger/8 px-5 py-3.5 text-[14px] text-status-danger">{error}</div>}
      {validation?.errors?.length ? <div className="rounded-xl border border-status-danger/20 bg-status-danger/8 px-5 py-3.5 text-[14px] text-status-danger">{validation.errors.join(" | ")}</div> : null}
      {validation?.warnings?.length ? <div className="rounded-xl border border-status-warning/20 bg-status-warning/8 px-5 py-3.5 text-[14px] text-status-warning">{validation.warnings.join(" | ")}</div> : null}

      {loading ? (
        <div className="grid gap-5 xl:grid-cols-12">
          {Array.from({ length: 4 }, (_, i) => (
            <section key={i} className="animate-pulse rounded-2xl bg-surface-2 p-6 xl:col-span-6">
              <div className="h-5 w-36 rounded bg-surface-3/55" />
              <div className="mt-5 space-y-3">
                <div className="h-10 rounded bg-surface-3/45" />
                <div className="h-10 rounded bg-surface-3/45" />
                <div className="h-10 rounded bg-surface-3/45" />
              </div>
            </section>
          ))}
        </div>
      ) : (
        <ServerSettingsForm draft={draft} rawYaml={rawYaml} onDraftChange={setDraft} />
      )}

      <Toast open={Boolean(snack)} onOpenChange={(open) => !open && setSnack("")} message={snack} variant="success" />
    </div>
  );
}
