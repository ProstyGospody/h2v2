import { CheckCircle2, Play, RefreshCw, Save } from "lucide-react";
import { useCallback, useEffect, useState } from "react";

import { ConfirmDialog } from "@/components/dialogs/confirm-dialog";
import { ServerSettingsForm } from "@/components/forms/server-settings-form";
import { PageHeader } from "@/components/ui/page-header";
import { normalizeSettingsDraft, toSettingsDraft } from "@/domain/settings/adapters";
import {
  applyHysteriaSettings,
  getHysteriaSettings,
  saveHysteriaSettings,
  validateHysteriaSettings,
} from "@/domain/settings/services";
import { Hy2ConfigValidation, Hy2Settings } from "@/domain/settings/types";
import { APIError } from "@/services/api";
import { Button, Toast } from "@/src/components/ui";

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

export default function ConfigPage() {
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [applying, setApplying] = useState(false);
  const [applyDialog, setApplyDialog] = useState(false);
  const [error, setError] = useState("");
  const [snack, setSnack] = useState("");
  const [rawYaml, setRawYaml] = useState("");
  const [draft, setDraft] = useState<Hy2Settings>(toSettingsDraft({ listen: ":443", tlsEnabled: true, tlsMode: "acme", quicEnabled: false } as Hy2Settings));
  const [validation, setValidation] = useState<Hy2ConfigValidation | null>(null);

  const load = useCallback(async () => {
    setError("");
    try {
      const p = await getHysteriaSettings();
      setRawYaml(p.raw_yaml || "");
      setDraft(toSettingsDraft(p.settings));
      setValidation(p.config_validation || null);
    } catch (err) {
      setError(err instanceof APIError ? err.message : "Failed to load server settings");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { void load(); }, [load]);

  async function validateDraft() {
    setBusy(true); setError("");
    try {
      const p = await validateHysteriaSettings(normalizeSettingsDraft(draft));
      setDraft(toSettingsDraft(p.settings)); setRawYaml(p.raw_yaml || rawYaml); setValidation(p.config_validation || null);
      setSnack(p.config_validation.valid ? "Configuration is valid" : "Validation returned issues");
    } catch (err) { setError(extractValidationError(err, "Validation failed")); }
    finally { setBusy(false); }
  }

  async function saveDraft() {
    setBusy(true); setError("");
    try {
      const p = await saveHysteriaSettings(normalizeSettingsDraft(draft));
      setDraft(toSettingsDraft(p.settings)); setRawYaml(p.raw_yaml || rawYaml); setValidation(p.config_validation || null);
      setSnack(p.backup_path ? `Saved. Backup: ${p.backup_path}` : "Settings saved");
    } catch (err) { setError(extractValidationError(err, "Save failed")); }
    finally { setBusy(false); }
  }

  async function applyConfig() {
    setApplying(true); setError("");
    try { await applyHysteriaSettings(); setApplyDialog(false); setSnack("Hysteria restarted"); await load(); }
    catch (err) { setError(extractValidationError(err, "Apply failed")); }
    finally { setApplying(false); }
  }

  if (loading) {
    return (
      <div className="flex min-h-[360px] items-center justify-center">
        <div className="flex flex-col items-center gap-3">
          <div className="h-6 w-6 animate-spin rounded-full border-2 border-accent/20 border-t-accent-light" />
          <p className="text-[14px] text-txt-secondary">Loading server settings...</p>
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <PageHeader
        title="Settings"
        actions={
          <>
            <Button onClick={() => void load()} disabled={busy || applying} className="h-12 w-full rounded-2xl px-5 sm:w-auto"><RefreshCw size={18} strokeWidth={1.6} />Reload</Button>
            <Button onClick={() => void validateDraft()} disabled={busy || applying} className="h-12 w-full rounded-2xl px-5 sm:w-auto"><CheckCircle2 size={18} strokeWidth={1.6} />Validate</Button>
            <Button variant="primary" onClick={() => void saveDraft()} disabled={busy || applying} className="h-12 w-full rounded-2xl px-5 sm:w-auto"><Save size={18} strokeWidth={1.6} />Save</Button>
            <Button variant="primary" onClick={() => setApplyDialog(true)} disabled={busy || applying} className="h-12 w-full rounded-2xl px-5 sm:w-auto"><Play size={18} strokeWidth={1.6} />Apply</Button>
          </>
        }
      />

      {error && <div className="rounded-xl border border-status-danger/20 bg-status-danger/8 px-5 py-3.5 text-[14px] text-status-danger">{error}</div>}
      {validation?.errors?.length ? <div className="rounded-xl border border-status-danger/20 bg-status-danger/8 px-5 py-3.5 text-[14px] text-status-danger">{validation.errors.join(" | ")}</div> : null}
      {validation?.warnings?.length ? <div className="rounded-xl border border-status-warning/20 bg-status-warning/8 px-5 py-3.5 text-[14px] text-status-warning">{validation.warnings.join(" | ")}</div> : null}

      <ServerSettingsForm draft={draft} rawYaml={rawYaml} onDraftChange={setDraft} />

      <ConfirmDialog open={applyDialog} title="Apply configuration" description="Restart hysteria-server with the current saved settings?" busy={applying} confirmColor="secondary" confirmText="Apply & Restart" onClose={() => setApplyDialog(false)} onConfirm={() => void applyConfig()} />
      <Toast open={Boolean(snack)} onOpenChange={(open) => !open && setSnack("")} message={snack} variant="success" />
    </div>
  );
}
