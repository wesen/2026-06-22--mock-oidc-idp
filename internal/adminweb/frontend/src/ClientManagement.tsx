import { FormEvent, useState } from "react";
import type { AdminListItem, AdminSession, ClientDetail, OneTimeSecretResult } from "./api";
import {
  useClientDetailQuery,
  useExecuteActionMutation,
  usePageDataQuery,
  usePrepareActionMutation,
} from "./api";

const authorizationCode = "authorization_code";
const refreshToken = "refresh_token";

export function ClientManagement({ session }: { session: AdminSession }) {
  const clients = usePageDataQuery({ page: "clients", search: "" });
  const [prepare] = usePrepareActionMutation();
  const [execute, execution] = useExecuteActionMutation();
  const [message, setMessage] = useState("");
  const [secret, setSecret] = useState<OneTimeSecretResult | null>(null);

  async function run(command: string, targetId: string, input: Record<string, unknown>) {
    setMessage("");
    setSecret(null);
    try {
      const prepared = await prepare({
        command, target_id: targetId, csrf: session.csrf_token,
      }).unwrap();
      const result = await execute({
        prepared, input, csrf: session.csrf_token,
      }).unwrap() as Partial<OneTimeSecretResult>;
      execution.reset();
      if (typeof result.secret === "string") {
        setSecret({ resource_id: result.resource_id ?? targetId, secret: result.secret });
        setMessage("Operation completed. Copy the one-time secret now.");
      } else {
        setMessage(`${command} completed.`);
      }
      await clients.refetch();
      return result;
    } catch (error) {
      execution.reset();
      if (isFreshAuthError(error)) {
        const returnPath = window.location.pathname + window.location.search;
        window.location.assign(`/admin/auth/reauth?return=${encodeURIComponent(returnPath)}`);
      }
      setMessage(`${command} failed. Review the exact URI, grant, and client fields.`);
      throw error;
    }
  }

  async function create(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = event.currentTarget;
    const data = new FormData(form);
    const id = String(data.get("id") ?? "").trim();
    try {
      await run("clients.create", id, clientInput(data));
      form.reset();
    } catch {
      // Status is reported by run.
    }
  }

  return (
    <section className="mt-4" aria-labelledby="client-operations-title">
      <h2 className="h4" id="client-operations-title">Application operations</h2>
      {message && <div className="alert alert-info" role="status">{message}</div>}
      {secret && (
        <div className="alert alert-warning" role="alert">
          <p className="fw-semibold mb-2">One-time client secret for <code>{secret.resource_id}</code></p>
          <textarea className="form-control font-monospace mb-2" readOnly value={secret.secret} rows={2} aria-label="One-time client secret" />
          <button className="btn btn-sm btn-outline-dark" type="button" onClick={() => setSecret(null)}>Clear secret</button>
        </div>
      )}
      <details className="card mb-3">
        <summary className="card-header fw-semibold">Register application</summary>
        <ClientForm onSubmit={create} includeId />
      </details>
      {clients.data?.items?.map((client) => (
        <ClientOperations key={client.id} client={client} run={run} />
      ))}
    </section>
  );
}

function ClientOperations({ client, run }: {
  client: AdminListItem;
  run: (command: string, targetId: string, input: Record<string, unknown>) => Promise<unknown>;
}) {
  const detail = useClientDetailQuery(client.id);
  const [busy, setBusy] = useState(false);

  async function update(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setBusy(true);
    try {
      await run("clients.update", client.id, clientInput(new FormData(event.currentTarget)));
      await detail.refetch();
    } finally {
      setBusy(false);
    }
  }

  async function guarded(command: string, confirmation: string, form: HTMLFormElement) {
    setBusy(true);
    try {
      await run(command, client.id, {
        reason: new FormData(form).get("reason"),
        confirmation,
      });
      form.reset();
      await detail.refetch();
    } finally {
      setBusy(false);
    }
  }

  return (
    <details className="card mb-2">
      <summary className="card-header">
        <span className="fw-semibold">{client.primary}</span>
        <span className="badge text-bg-secondary ms-2">{client.status}</span>
      </summary>
      <div className="card-body">
        {detail.isLoading && <p aria-busy="true">Loading application configuration…</p>}
        {detail.data && <ClientForm onSubmit={update} detail={detail.data} busy={busy} />}
        <div className="d-flex flex-wrap gap-2 mt-3">
          <GuardedClientForm
            label={client.status === "disabled" ? "Enable" : "Disable"}
            confirmation={client.status === "disabled" ? "" : "DISABLE"}
            busy={busy}
            onSubmit={(form) => guarded(client.status === "disabled" ? "clients.enable" : "clients.disable", client.status === "disabled" ? "" : "DISABLE", form)}
          />
          {detail.data && !detail.data.public && (
            <GuardedClientForm label="Rotate secret" confirmation="ROTATE" busy={busy} onSubmit={(form) => guarded("clients.rotate_secret", "ROTATE", form)} />
          )}
        </div>
      </div>
    </details>
  );
}

function ClientForm({ onSubmit, detail, includeId = false, busy = false }: {
  onSubmit: (event: FormEvent<HTMLFormElement>) => void;
  detail?: ClientDetail;
  includeId?: boolean;
  busy?: boolean;
}) {
  return (
    <form className="card-body row g-3" onSubmit={onSubmit}>
      {includeId && <Field name="id" label="Client ID" required />}
      <Checkbox name="public" label="Public client" defaultChecked={detail?.public} />
      <Checkbox name="require_pkce" label="Require PKCE" defaultChecked={detail?.require_pkce ?? true} />
      <Checkbox name="can_introspect" label="Can introspect" defaultChecked={detail?.can_introspect} />
      <Lines name="redirect_uris" label="Redirect URIs, one per line" values={detail?.redirect_uris} required />
      <Lines name="post_logout_redirect_uris" label="Post-logout redirect URIs" values={detail?.post_logout_redirect_uris} />
      <Lines name="allowed_scopes" label="Allowed scopes" values={detail?.allowed_scopes ?? ["openid"]} required />
      <Lines name="allowed_grant_types" label="Allowed grant types" values={detail?.allowed_grant_types ?? [authorizationCode, refreshToken]} required />
      <Lines name="allowed_audiences" label="Allowed audiences" values={detail?.allowed_audiences} />
      <div><button disabled={busy} className="btn btn-primary" type="submit">{includeId ? "Register once" : "Save configuration"}</button></div>
    </form>
  );
}

function clientInput(data: FormData): Record<string, unknown> {
  const lines = (name: string) => String(data.get(name) ?? "")
    .split(/\r?\n/).map((value) => value.trim()).filter(Boolean);
  return {
    public: data.get("public") === "on",
    require_pkce: data.get("require_pkce") === "on",
    can_introspect: data.get("can_introspect") === "on",
    redirect_uris: lines("redirect_uris"),
    post_logout_redirect_uris: lines("post_logout_redirect_uris"),
    allowed_scopes: lines("allowed_scopes"),
    allowed_grant_types: lines("allowed_grant_types"),
    allowed_audiences: lines("allowed_audiences"),
  };
}

function GuardedClientForm({ label, confirmation, busy, onSubmit }: {
  label: string; confirmation: string; busy: boolean; onSubmit: (form: HTMLFormElement) => Promise<void>;
}) {
  return (
    <form className="border rounded p-2" onSubmit={(event) => {
      event.preventDefault();
      void onSubmit(event.currentTarget);
    }}>
      <div className="small fw-semibold mb-1">{label}</div>
      <input className="form-control form-control-sm mb-1" name="reason" required placeholder="Reason" aria-label={`${label} reason`} />
      {confirmation && <input className="form-control form-control-sm mb-1" required value={confirmation} readOnly aria-label={`${label} confirmation`} />}
      <button disabled={busy} className="btn btn-sm btn-outline-danger" type="submit">{label}</button>
    </form>
  );
}

function Field({ name, label, required = false }: { name: string; label: string; required?: boolean }) {
  return <div className="col-md-6"><label className="form-label" htmlFor={`client-${name}`}>{label}</label><input className="form-control" id={`client-${name}`} name={name} required={required} /></div>;
}

function Checkbox({ name, label, defaultChecked = false }: { name: string; label: string; defaultChecked?: boolean }) {
  return <div className="col-md-4 form-check ms-2"><input className="form-check-input" id={`client-${name}`} name={name} type="checkbox" defaultChecked={defaultChecked} /><label className="form-check-label" htmlFor={`client-${name}`}>{label}</label></div>;
}

function Lines({ name, label, values = [], required = false }: { name: string; label: string; values?: string[]; required?: boolean }) {
  return <div className="col-md-6"><label className="form-label" htmlFor={`client-${name}`}>{label}</label><textarea className="form-control font-monospace" id={`client-${name}`} name={name} rows={3} defaultValue={values.join("\n")} required={required} /></div>;
}

function isFreshAuthError(error: unknown): boolean {
  if (!error || typeof error !== "object") return false;
  const candidate = error as { status?: number; data?: { error?: string } };
  return candidate.status === 401 && candidate.data?.error === "fresh_auth_required";
}
