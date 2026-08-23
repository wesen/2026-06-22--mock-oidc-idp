import { FormEvent, useState } from "react";
import type { AdminListItem, AdminSession } from "./api";
import {
  useExecuteActionMutation,
  usePageDataQuery,
  usePrepareActionMutation,
} from "./api";

interface Props {
  session: AdminSession;
}

export function UserManagement({ session }: Props) {
  const users = usePageDataQuery({ page: "users", search: window.location.search });
  const [prepare] = usePrepareActionMutation();
  const [execute] = useExecuteActionMutation();
  const [message, setMessage] = useState("");

  async function run(
    command: string,
    targetId: string | undefined,
    input: Record<string, unknown>,
  ) {
    setMessage("");
    try {
      const prepared = await prepare({
        command,
        target_id: targetId,
        csrf: session.csrf_token,
      }).unwrap();
      await execute({ prepared, input, csrf: session.csrf_token }).unwrap();
      setMessage(`${command} completed.`);
      await users.refetch();
    } catch (error) {
      if (isFreshAuthError(error)) {
        const returnPath = window.location.pathname + window.location.search;
        window.location.assign(`/admin/auth/reauth?return=${encodeURIComponent(returnPath)}`);
      }
      setMessage(`${command} failed. Review the fields and current resource state.`);
      throw error;
    }
  }

  async function create(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = event.currentTarget;
    const data = new FormData(form);
    try {
      await run("users.create", undefined, {
        login: data.get("login"),
        password: data.get("password"),
        email: data.get("email"),
        display_name: data.get("display_name"),
      });
      form.reset();
    } catch {
      setMessage("User creation failed. Review the fields and try again.");
    } finally {
      const password = form.elements.namedItem("password") as HTMLInputElement | null;
      if (password) password.value = "";
    }
  }

  return (
    <section className="mt-4 admin-mutation-surface" aria-labelledby="user-operations-title">
      <h2 className="h4" id="user-operations-title">User operations</h2>
      {message && <div className="alert alert-info" role="status">{message}</div>}
      <details className="card mb-3">
        <summary className="card-header fw-semibold">Create user</summary>
        <form className="card-body row g-3" onSubmit={create}>
          <Field idPrefix="create" name="login" label="Login" required />
          <Field idPrefix="create" name="email" label="Email" type="email" />
          <Field idPrefix="create" name="display_name" label="Display name" />
          <Field idPrefix="create" name="password" label="Password" type="password" required />
          <div><button className="btn btn-primary" type="submit">Create user</button></div>
        </form>
      </details>
      {users.isLoading && <p aria-busy="true">Loading users…</p>}
      {users.data?.items?.map((user) => (
        <UserOperations key={user.id} user={user} run={run} />
      ))}
    </section>
  );
}

function UserOperations({
  user,
  run,
}: {
  user: AdminListItem;
  run: (command: string, targetId: string, input: Record<string, unknown>) => Promise<void>;
}) {
  const [busy, setBusy] = useState(false);

  async function submit(
    event: FormEvent<HTMLFormElement>,
    command: string,
  ) {
    event.preventDefault();
    const form = event.currentTarget;
    const data = Object.fromEntries(new FormData(form));
    setBusy(true);
    try {
      await run(command, user.id, data);
      form.reset();
    } finally {
      const password = form.elements.namedItem("password") as HTMLInputElement | null;
      if (password) password.value = "";
      setBusy(false);
    }
  }

  return (
    <details className="card mb-2">
      <summary className="card-header">
        <span className="fw-semibold">{user.primary}</span>
        <span className="badge text-bg-secondary ms-2">{user.status}</span>
      </summary>
      <div className="card-body">
        <p className="text-body-secondary">{user.secondary} · <code>{user.id}</code></p>
        <form className="row g-2 mb-3" onSubmit={(event) => submit(event, "users.update")}>
          <Field idPrefix={user.id} name="email" label="Email" type="email" />
          <Field idPrefix={user.id} name="display_name" label="Display name" />
          <Field idPrefix={user.id} name="locale" label="Locale" />
          <div><button disabled={busy} className="btn btn-outline-primary" type="submit">Save profile</button></div>
        </form>
        <form className="row g-2 mb-3" onSubmit={(event) => submit(event, "users.set_password")}>
          <Field idPrefix={`${user.id}-password`} name="password" label="New password" type="password" required />
          <Field idPrefix={`${user.id}-password`} name="reason" label="Reason" required />
          <div><button disabled={busy} className="btn btn-outline-warning" type="submit">Set password</button></div>
        </form>
        <div className="d-flex flex-wrap gap-2">
          <ActionForm user={user} command={user.status === "disabled" ? "users.enable" : "users.disable"} confirmation={user.status === "disabled" ? "" : "DISABLE"} run={run} busy={busy} />
          <ActionForm user={user} command="users.unlock" confirmation="" run={run} busy={busy} />
          <ActionForm user={user} command="users.revoke_access" confirmation="REVOKE" run={run} busy={busy} />
        </div>
      </div>
    </details>
  );
}

function ActionForm({
  user,
  command,
  confirmation,
  run,
  busy,
}: {
  user: AdminListItem;
  command: string;
  confirmation: string;
  run: (command: string, targetId: string, input: Record<string, unknown>) => Promise<void>;
  busy: boolean;
}) {
  return (
    <form
      className="border rounded p-2"
      onSubmit={async (event) => {
        event.preventDefault();
        const data = Object.fromEntries(new FormData(event.currentTarget));
        try {
          await run(command, user.id, data);
          event.currentTarget.reset();
        } catch {
          // The parent status region reports the classified failure.
        }
      }}
    >
      <div className="small fw-semibold mb-1">{command}</div>
      <input className="form-control form-control-sm mb-1" name="reason" required aria-label={`${command} reason`} placeholder="Reason" />
      {confirmation && (
        <input className="form-control form-control-sm mb-1" name="confirmation" required aria-label={`${command} confirmation`} placeholder={`Type ${confirmation}`} />
      )}
      <button disabled={busy} className="btn btn-sm btn-outline-danger" type="submit">Run</button>
    </form>
  );
}

function Field({
  idPrefix,
  name,
  label,
  type = "text",
  required = false,
}: {
  idPrefix: string;
  name: string;
  label: string;
  type?: string;
  required?: boolean;
}) {
  const id = `user-${idPrefix}-${name}`;
  return (
    <div className="col-md-6">
      <label className="form-label" htmlFor={id}>{label}</label>
      <input className="form-control" id={id} name={name} type={type} required={required} />
    </div>
  );
}

function isFreshAuthError(error: unknown): boolean {
  if (!error || typeof error !== "object") return false;
  const candidate = error as { status?: number; data?: { error?: string } };
  return candidate.status === 401 && candidate.data?.error === "fresh_auth_required";
}
