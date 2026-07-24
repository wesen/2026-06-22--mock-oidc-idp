import {
  WidgetRenderer,
  WidgetToastRegion,
  defaultWidgetRegistry,
} from "@go-go-golems/rag-evaluation-site";
import { useSessionQuery, useWidgetPageQuery } from "./api";
import { UserManagement } from "./UserManagement";

const pages = [
  ["overview", "Overview"],
  ["users", "Users"],
  ["invitations", "Invitations"],
  ["clients", "Applications"],
  ["keys", "Signing keys"],
  ["activity", "Activity"],
  ["operations", "Operations"],
] as const;

function currentPage(): string {
  const value = window.location.pathname.replace(/^\/admin\/?/, "").split("/")[0];
  return pages.some(([id]) => id === value) ? value : "overview";
}

export function App() {
  const page = currentPage();
  const session = useSessionQuery();
  const widget = useWidgetPageQuery(
    { page, search: window.location.search },
    { skip: !session.data },
  );

  if (session.isLoading) {
    return <main className="container py-5" aria-busy="true">Loading TinyIDP Console…</main>;
  }
  if (session.isError || !session.data) {
    return (
      <main className="container py-5">
        <h1>TinyIDP Console</h1>
        <p className="lead">Authenticate with the installation owner account to continue.</p>
        <a className="btn btn-primary" href={`/admin/auth/login?return=${encodeURIComponent(window.location.pathname)}`}>
          Sign in
        </a>
      </main>
    );
  }

  return (
    <div className="d-flex min-vh-100">
      <nav className="border-end bg-body-tertiary p-3 admin-nav" aria-label="Administration">
        <a className="h5 text-decoration-none d-block mb-4" href="/admin">TinyIDP Console</a>
        <ul className="nav nav-pills flex-column gap-1">
          {pages.map(([id, label]) => (
            <li className="nav-item" key={id}>
              <a
                className={`nav-link ${page === id ? "active" : ""}`}
                aria-current={page === id ? "page" : undefined}
                href={`/admin/${id}`}
              >
                {label}
              </a>
            </li>
          ))}
        </ul>
        <div className="small text-body-secondary mt-4 text-break">{session.data.subject}</div>
      </nav>
      <main className="flex-grow-1 p-3 p-lg-4 overflow-auto" id="main-content">
        {widget.isLoading && <div aria-busy="true">Loading page…</div>}
        {widget.isError && (
          <div className="alert alert-danger" role="alert">
            The administration page could not be loaded. Refresh or sign in again.
          </div>
        )}
        {widget.data && (
          <WidgetRenderer node={widget.data.root} registry={defaultWidgetRegistry} />
        )}
        {page === "users" && <UserManagement session={session.data} />}
      </main>
      <WidgetToastRegion />
    </div>
  );
}
