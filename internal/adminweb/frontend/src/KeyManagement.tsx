import { FormEvent, useState } from "react";
import type { AdminSession } from "./api";
import {
  useExecuteActionMutation,
  usePageDataQuery,
  usePrepareActionMutation,
} from "./api";

export function KeyManagement({ session }: { session: AdminSession }) {
  const keys = usePageDataQuery({ page: "keys", search: "" });
  const [prepare] = usePrepareActionMutation();
  const [execute, execution] = useExecuteActionMutation();
  const [message, setMessage] = useState("");
  const [busy, setBusy] = useState(false);

  async function run(
    command: string,
    targetId: string,
    reason: string,
    confirmation: string,
  ) {
    setBusy(true);
    setMessage("");
    try {
      const prepared = await prepare({
        command, target_id: targetId, csrf: session.csrf_token,
      }).unwrap();
      await execute({
        prepared,
        input: { algorithm: command === "keys.rotate" ? "RS256" : undefined, reason, confirmation },
        csrf: session.csrf_token,
      }).unwrap();
      setMessage(`${command} completed.`);
      await keys.refetch();
    } catch (error) {
      if (isFreshAuthError(error)) {
        const returnPath = window.location.pathname + window.location.search;
        window.location.assign(`/admin/auth/reauth?return=${encodeURIComponent(returnPath)}`);
        return;
      }
      setMessage(`${command} failed. Refresh the key list and verify the reason and confirmation.`);
    } finally {
      execution.reset();
      setBusy(false);
    }
  }

  return (
    <section className="mt-4 admin-mutation-surface" aria-labelledby="key-operations-title">
      <h2 className="h4" id="key-operations-title">Signing-key operations</h2>
      <p className="text-body-secondary">
        Rotation creates a new RSA key and keeps the previous key available for token verification.
        Emergency key deletion remains CLI-only.
      </p>
      {message && <div className="alert alert-info" role="status">{message}</div>}
      <GuardedKeyForm
        label="Rotate signing key"
        confirmation="ROTATE"
        busy={busy}
        onSubmit={(reason, confirmation) => run("keys.rotate", "", reason, confirmation)}
      />
      <div className="mt-3">
        {keys.data?.items?.filter((key) => key.status !== "active").map((key) => (
          <GuardedKeyForm
            key={key.id}
            label={`Retire ${key.primary}`}
            confirmation="RETIRE"
            busy={busy}
            onSubmit={(reason, confirmation) => run("keys.retire", key.id, reason, confirmation)}
          />
        ))}
      </div>
    </section>
  );
}

function GuardedKeyForm({ label, confirmation, busy, onSubmit }: {
  label: string;
  confirmation: string;
  busy: boolean;
  onSubmit: (reason: string, confirmation: string) => Promise<void>;
}) {
  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = event.currentTarget;
    const data = new FormData(form);
    void onSubmit(
      String(data.get("reason") ?? "").trim(),
      String(data.get("confirmation") ?? ""),
    ).then(() => form.reset());
  }

  return (
    <form className="card card-body mb-2" onSubmit={submit}>
      <div className="fw-semibold mb-2">{label}</div>
      <label className="form-label">
        Operator reason
        <input className="form-control" name="reason" required maxLength={500} />
      </label>
      <label className="form-label">
        Type {confirmation} to confirm
        <input className="form-control font-monospace" name="confirmation" required autoComplete="off" />
      </label>
      <div>
        <button className="btn btn-outline-danger" disabled={busy} type="submit">{label}</button>
      </div>
    </form>
  );
}

function isFreshAuthError(error: unknown): boolean {
  if (!error || typeof error !== "object") return false;
  const candidate = error as { status?: number; data?: { error?: string } };
  return candidate.status === 401 && candidate.data?.error === "fresh_auth_required";
}
