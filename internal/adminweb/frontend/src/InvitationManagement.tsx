import { FormEvent, useState } from "react";
import type { AdminSession, OneTimeSecretResult } from "./api";
import {
  useExecuteActionMutation,
  usePageDataQuery,
  usePrepareActionMutation,
} from "./api";

export function InvitationManagement({ session }: { session: AdminSession }) {
  const invitations = usePageDataQuery({ page: "invitations", search: "" });
  const [prepare] = usePrepareActionMutation();
  const [execute, execution] = useExecuteActionMutation();
  const [message, setMessage] = useState("");
  const [issued, setIssued] = useState<OneTimeSecretResult | null>(null);

  async function issue(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = event.currentTarget;
    const values = new FormData(form);
    setIssued(null);
    setMessage("");
    try {
      const prepared = await prepare({
        command: "invitations.issue",
        csrf: session.csrf_token,
      }).unwrap();
      const result = await execute({
        prepared,
        csrf: session.csrf_token,
        input: {
          label: values.get("label"),
          audience: values.get("audience"),
          valid_for: values.get("valid_for"),
        },
      }).unwrap() as OneTimeSecretResult;
      execution.reset();
      setIssued(result);
      setMessage("Invitation issued. Copy the code now; it cannot be shown again.");
      form.reset();
      await invitations.refetch();
    } catch {
      execution.reset();
      setMessage("Invitation issuance failed. Review the audience and lifetime.");
    }
  }

  async function revoke(invitationId: string, form: HTMLFormElement) {
    const values = Object.fromEntries(new FormData(form));
    setMessage("");
    try {
      const prepared = await prepare({
        command: "invitations.revoke",
        target_id: invitationId,
        csrf: session.csrf_token,
      }).unwrap();
      await execute({ prepared, input: values, csrf: session.csrf_token }).unwrap();
      execution.reset();
      setMessage("Invitation revoked.");
      form.reset();
      await invitations.refetch();
    } catch {
      execution.reset();
      setMessage("Invitation revocation failed. Refresh before retrying.");
    }
  }

  return (
    <section className="mt-4" aria-labelledby="invitation-operations-title">
      <h2 className="h4" id="invitation-operations-title">Invitation operations</h2>
      {message && <div className="alert alert-info" role="status">{message}</div>}
      {issued && (
        <div className="alert alert-warning" role="alert">
          <p className="fw-semibold mb-2">One-time invitation code</p>
          <textarea className="form-control font-monospace mb-2" readOnly value={issued.secret} rows={2} aria-label="One-time invitation code" />
          <button className="btn btn-sm btn-outline-dark" type="button" onClick={() => setIssued(null)}>Clear code</button>
        </div>
      )}
      <details className="card mb-3">
        <summary className="card-header fw-semibold">Issue invitation</summary>
        <form className="card-body row g-3" onSubmit={issue}>
          <TextField prefix="invitation" name="label" label="Operator label" required />
          <TextField prefix="invitation" name="audience" label="Audience client ID" required />
          <TextField prefix="invitation" name="valid_for" label="Valid for" placeholder="24h" required />
          <div><button className="btn btn-primary" type="submit">Issue once</button></div>
        </form>
      </details>
      {invitations.data?.items?.map((invitation) => (
        <div className="card mb-2" key={invitation.id}>
          <div className="card-body">
            <div className="d-flex justify-content-between gap-2">
              <div>
                <div className="fw-semibold">{invitation.primary}</div>
                <div className="small text-body-secondary">{invitation.secondary} · <code>{invitation.id}</code></div>
              </div>
              <span className="badge text-bg-secondary align-self-start">{invitation.status}</span>
            </div>
            {invitation.status === "pending" && (
              <form className="row g-2 mt-2" onSubmit={(event) => {
                event.preventDefault();
                void revoke(invitation.id, event.currentTarget);
              }}>
                <TextField prefix={invitation.id} name="reason" label="Revocation reason" required />
                <TextField prefix={invitation.id} name="confirmation" label="Type REVOKE" required />
                <div><button className="btn btn-outline-danger" type="submit">Revoke</button></div>
              </form>
            )}
          </div>
        </div>
      ))}
    </section>
  );
}

function TextField({ prefix, name, label, required = false, placeholder }: {
  prefix: string; name: string; label: string; required?: boolean; placeholder?: string;
}) {
  const id = `${prefix}-${name}`;
  return (
    <div className="col-md-6">
      <label className="form-label" htmlFor={id}>{label}</label>
      <input className="form-control" id={id} name={name} required={required} placeholder={placeholder} />
    </div>
  );
}
