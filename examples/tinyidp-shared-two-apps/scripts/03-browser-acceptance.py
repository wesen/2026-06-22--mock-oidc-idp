#!/usr/bin/env python3
"""Exercise the shared TinyIDP stack through real HTTPS browser flows.

The script deliberately uses only Python's standard library.  It maintains
browser cookie jars, follows redirects across the two relying parties and the
IDP, submits the rendered HTML forms, and calls the applications with their
host-issued CSRF tokens.  JavaScript-owned application actions are reproduced
as the same HTTP requests made by the checked-in frontend.
"""

from __future__ import annotations

import http.cookiejar
import json
import re
import secrets
import ssl
import subprocess
import sys
import time
import urllib.error
import urllib.parse
import urllib.request
from dataclasses import dataclass
from html.parser import HTMLParser
from pathlib import Path
from typing import Any


EXAMPLE_DIR = Path(__file__).resolve().parents[1]
COMPOSE_FILE = EXAMPLE_DIR / "compose.yaml"
TRUST_FILE = EXAMPLE_DIR / "runtime" / "caddy-local-root.crt"
MESSAGE_ORIGIN = "https://message.localhost:8443"
GOJA_ORIGIN = "https://goja.localhost:8443"
IDP_ORIGIN = "https://idp.localhost:8443"
OUTBOX_ORIGIN = "http://127.0.0.1:8025"
ADMIN_LOGIN = "admin@example.test"
ADMIN_PASSWORD = (EXAMPLE_DIR / "runtime" / "secrets" / "local-admin-password.txt").read_text(
    encoding="utf-8"
).strip()
INVITEE_LOGIN = "invitee@example.test"
INVITEE_PASSWORD = (EXAMPLE_DIR / "runtime" / "secrets" / "local-invitee-password.txt").read_text(
    encoding="utf-8"
).strip()


class AcceptanceFailure(RuntimeError):
    pass


@dataclass
class HTTPResult:
    status: int
    url: str
    headers: Any
    body: str

    def json(self) -> Any:
        try:
            return json.loads(self.body)
        except json.JSONDecodeError as exc:
            raise AcceptanceFailure(
                f"expected JSON from {self.url}, got status {self.status}: {self.body[:500]}"
            ) from exc


@dataclass
class HTMLForm:
    action: str
    method: str
    values: dict[str, str]


class FirstFormParser(HTMLParser):
    def __init__(self) -> None:
        super().__init__()
        self.form: HTMLForm | None = None
        self._inside_first_form = False

    def handle_starttag(self, tag: str, attrs: list[tuple[str, str | None]]) -> None:
        attributes = {name: value or "" for name, value in attrs}
        if tag == "form" and self.form is None:
            self.form = HTMLForm(
                action=attributes.get("action", ""),
                method=attributes.get("method", "get").lower(),
                values={},
            )
            self._inside_first_form = True
            return
        if not self._inside_first_form or self.form is None:
            return
        if tag in {"input", "button"} and attributes.get("name"):
            self.form.values.setdefault(attributes["name"], attributes.get("value", ""))

    def handle_endtag(self, tag: str) -> None:
        if tag == "form" and self._inside_first_form:
            self._inside_first_form = False


class Browser:
    def __init__(self) -> None:
        if not TRUST_FILE.is_file():
            raise AcceptanceFailure(f"missing local CA: {TRUST_FILE}; run scripts/01-export-browser-ca.sh")
        context = ssl.create_default_context(cafile=str(TRUST_FILE))
        self.cookies = http.cookiejar.CookieJar()
        self.opener = urllib.request.build_opener(
            urllib.request.ProxyHandler({}),
            urllib.request.HTTPSHandler(context=context),
            urllib.request.HTTPCookieProcessor(self.cookies),
        )

    def request(
        self,
        method: str,
        url: str,
        *,
        data: bytes | None = None,
        headers: dict[str, str] | None = None,
    ) -> HTTPResult:
        request = urllib.request.Request(url, data=data, headers=headers or {}, method=method)
        try:
            response = self.opener.open(request, timeout=20)
        except urllib.error.HTTPError as response:
            return HTTPResult(
                status=response.code,
                url=response.geturl(),
                headers=response.headers,
                body=response.read().decode("utf-8", "replace"),
            )
        with response:
            return HTTPResult(
                status=response.status,
                url=response.geturl(),
                headers=response.headers,
                body=response.read().decode("utf-8", "replace"),
            )

    def get(self, url: str) -> HTTPResult:
        return self.request("GET", url)

    def json_request(
        self,
        method: str,
        url: str,
        payload: dict[str, Any] | None = None,
        *,
        csrf: str = "",
    ) -> HTTPResult:
        headers = {"Accept": "application/json"}
        data = None
        if payload is not None:
            data = json.dumps(payload).encode("utf-8")
            headers["Content-Type"] = "application/json"
        if csrf:
            headers["X-CSRF-Token"] = csrf
        return self.request(method, url, data=data, headers=headers)

    def submit_first_form(self, page: HTTPResult, values: dict[str, str]) -> HTTPResult:
        parser = FirstFormParser()
        parser.feed(page.body)
        if parser.form is None:
            raise AcceptanceFailure(f"no HTML form found at {page.url}: {page.body[:500]}")
        form = parser.form
        form.values.update(values)
        action = urllib.parse.urljoin(page.url, form.action)
        origin_parts = urllib.parse.urlsplit(page.url)
        origin = f"{origin_parts.scheme}://{origin_parts.netloc}"
        encoded = urllib.parse.urlencode(form.values).encode("utf-8")
        return self.request(
            form.method.upper(),
            action,
            data=encoded,
            headers={"Content-Type": "application/x-www-form-urlencoded", "Origin": origin},
        )


def require(condition: bool, message: str) -> None:
    if not condition:
        raise AcceptanceFailure(message)


def require_status(result: HTTPResult, expected: int, label: str) -> None:
    require(
        result.status == expected,
        f"{label}: expected HTTP {expected}, got {result.status} at {result.url}: {result.body[:700]}",
    )


def parse_form(page: HTTPResult) -> HTMLForm:
    parser = FirstFormParser()
    parser.feed(page.body)
    if parser.form is None:
        raise AcceptanceFailure(f"expected a form at {page.url}: {page.body[:500]}")
    return parser.form


def complete_idp_prompts(browser: Browser, result: HTTPResult) -> HTTPResult:
    """Approve post-authentication consent without masking credential errors."""
    for _ in range(3):
        if not result.url.startswith(IDP_ORIGIN):
            return result
        form = parse_form(result)
        credential_fields = {
            "login",
            "password",
            "password_confirmation",
            "email",
            "display_name",
            "invite_code",
        }
        if credential_fields.intersection(form.values):
            return result
        result = browser.submit_first_form(result, {"action": "approve"})
        require_status(result, 200, "approve OIDC consent")
    raise AcceptanceFailure(f"OIDC flow did not terminate after consent at {result.url}")


def compose(*args: str) -> str:
    completed = subprocess.run(
        ["docker", "compose", "-f", str(COMPOSE_FILE), *args],
        cwd=EXAMPLE_DIR,
        check=False,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
        timeout=30,
    )
    if completed.returncode != 0:
        raise AcceptanceFailure(
            f"docker compose {' '.join(args)} failed ({completed.returncode}):\n"
            f"stdout:\n{completed.stdout}\nstderr:\n{completed.stderr}"
        )
    return completed.stdout


def outbox_request(path: str) -> HTTPResult:
    request = urllib.request.Request(urllib.parse.urljoin(OUTBOX_ORIGIN, path))
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
    try:
        response = opener.open(request, timeout=5)
    except urllib.error.HTTPError as response:
        return HTTPResult(response.code, response.geturl(), response.headers, response.read().decode("utf-8", "replace"))
    with response:
        return HTTPResult(response.status, response.geturl(), response.headers, response.read().decode("utf-8", "replace"))


def outbox_code(recipient: str) -> str:
    query = urllib.parse.urlencode({"query": f'to:"{recipient}"'})
    for _ in range(40):
        result = outbox_request(f"/view/latest.txt?{query}")
        if result.status == 200:
            match = re.search(r"verification code is:\s*([A-Z2-7]{8})", result.body)
            if match:
                return match.group(1)
        elif result.status != 404:
            raise AcceptanceFailure(f"private outbox query failed with HTTP {result.status}: {result.body[:500]}")
        time.sleep(0.25)
    raise AcceptanceFailure(f"private outbox did not receive a challenge for {recipient}")


def restart_idp_and_wait(browser: Browser) -> None:
    compose("restart", "idp")
    for _ in range(40):
        try:
            result = browser.get(f"{IDP_ORIGIN}/readyz")
            if result.status == 200:
                return
        except urllib.error.URLError:
            pass
        time.sleep(0.25)
    raise AcceptanceFailure("TinyIDP did not become ready after challenge-state restart")


def issue_signup_invitation() -> dict[str, Any]:
    output = compose(
        "exec",
        "-T",
        "idp",
        "tinyidp",
        "admin",
        "--db=/state/tinyidp.sqlite",
        "invitation",
        "issue",
        "--audience=goja-auth-host-demo",
        "--policy-version=signup-invite-v1",
        "--ttl=1h",
        "--lookup-key-file=/state/.secrets/invitation_lookup_key",
        "--output=json",
    )
    rows = json.loads(output)
    require(isinstance(rows, list) and len(rows) == 1, f"unexpected invitation issue output: {output}")
    require(rows[0].get("code") and rows[0].get("invitation_id"), f"incomplete invitation: {rows[0]}")
    return rows[0]


def begin_membership_invitation(browser: Browser, token: str) -> dict[str, Any]:
    response = browser.json_request("POST", f"{GOJA_ORIGIN}/org-invites/begin", {"token": token})
    require_status(response, 200, "begin application invitation")
    body = response.json()
    require(body.get("registrationUrl") and body.get("loginUrl"), f"incomplete pending invitation: {body}")
    return body


def login(browser: Browser, entry_url: str, login_name: str, password: str, expected_origin: str) -> HTTPResult:
    page = browser.get(entry_url)
    require_status(page, 200, "render login form")
    form = parse_form(page)
    require("login" in form.values and "password" in form.values, f"unexpected login form at {page.url}")
    result = browser.submit_first_form(
        page,
        {"login": login_name, "password": password, "action": "approve"},
    )
    require_status(result, 200, "complete OIDC login")
    result = complete_idp_prompts(browser, result)
    require(result.url.startswith(expected_origin), f"OIDC login ended at unexpected URL: {result.url}")
    return result


def signup(
    browser: Browser,
    entry_url: str,
    *,
    display_name: str,
    email: str,
    password: str,
    expected_origin: str,
    invite_code: str | None,
    restart_after_delivery: bool = False,
    retry_wrong_code: bool = False,
) -> tuple[HTTPResult, str]:
    page = browser.get(entry_url)
    require_status(page, 200, "render signup form")
    form = parse_form(page)
    expected_fields = {"display_name", "email"}
    require(expected_fields.issubset(form.values), f"unexpected signup fields at {page.url}: {form.values.keys()}")
    require("password" not in form.values, "signup collected a password before email verification")
    if invite_code is None:
        require("invite_code" not in form.values, "open-signup client unexpectedly requested an invite code")
    else:
        require("invite_code" in form.values, "invite-gated client did not request an invite code")
    values = {
        "display_name": display_name,
        "email": email,
        "action": "submit",
    }
    if invite_code is not None:
        values["invite_code"] = invite_code
    code_page = browser.submit_first_form(page, values)
    require_status(code_page, 200, "start email challenge")
    code_form = parse_form(code_page)
    require("email_code" in code_form.values, f"email challenge form is missing at {code_page.url}")
    code = outbox_code(email)
    if restart_after_delivery:
        restart_idp_and_wait(browser)
    if retry_wrong_code:
        wrong = browser.submit_first_form(code_page, {"email_code": "AAAAAAAA", "action": "submit"})
        require_status(wrong, 400, "reject incorrect email challenge code")
        require("email_code" in parse_form(wrong).values, "incorrect code did not preserve the challenge form")
        code_page = wrong
    password_page = browser.submit_first_form(code_page, {"email_code": code, "action": "submit"})
    require_status(password_page, 200, "verify email challenge")
    password_form = parse_form(password_page)
    require(
        {"password", "password_confirmation"}.issubset(password_form.values),
        f"password form is missing after email verification at {password_page.url}",
    )
    require('minlength="15"' in password_page.body, "password form did not expose the production minimum length")
    require("Use at least 15 characters." in password_page.body, "password form omitted visible length guidance")
    result = browser.submit_first_form(
        password_page,
        {"password": password, "password_confirmation": password, "action": "submit"},
    )
    require_status(result, 200, "complete verified OIDC signup")
    result = complete_idp_prompts(browser, result)
    require(result.url.startswith(expected_origin), f"OIDC signup ended at unexpected URL: {result.url}")
    return result, code


def reject_cross_site_registration_with_themed_page() -> None:
    browser = Browser()
    page = browser.get(f"{MESSAGE_ORIGIN}/auth/register?return_to=/")
    require_status(page, 200, "render registration form for rejection probe")
    form = parse_form(page)
    form.values.update(
        {
            "display_name": "Must Not Persist",
            "email": "rejected-cross-site@example.test",
            "action": "submit",
        }
    )
    action = urllib.parse.urljoin(page.url, form.action)
    result = browser.request(
        form.method.upper(),
        action,
        data=urllib.parse.urlencode(form.values).encode("utf-8"),
        headers={
            "Content-Type": "application/x-www-form-urlencoded",
            "Origin": "https://attacker.example.test",
            "Sec-Fetch-Site": "cross-site",
            "Sec-Fetch-Mode": "navigate",
            "Sec-Fetch-Dest": "document",
            "Sec-Fetch-User": "?1",
        },
    )
    require_status(result, 403, "reject cross-site registration")
    require(
        result.headers.get_content_type() == "text/html",
        f"registration rejection was not HTML: {result.headers.get('Content-Type')}",
    )
    require(result.headers.get("Cache-Control") == "no-store", "registration rejection was cacheable")
    require(
        result.headers.get("Content-Security-Policy")
        == "default-src 'none'; style-src 'self'; frame-ancestors 'none'; form-action 'self'; base-uri 'none'",
        f"registration rejection had unexpected CSP: {result.headers.get('Content-Security-Policy')}",
    )
    require("/static/themes/message-desk.css" in result.body, "registration rejection did not use Message Desk CSS")
    require("Registration could not be completed" in result.body, "registration rejection omitted safe public guidance")
    for forbidden in ("rejected-cross-site@example.test", "Must Not Persist", "<form", "csrf_token"):
        require(forbidden not in result.body, f"registration rejection disclosed forbidden value {forbidden!r}")


def begin_signup_after_local_logout() -> None:
    browser = Browser()
    login(browser, f"{MESSAGE_ORIGIN}/auth/login?return_to=/", ADMIN_LOGIN, ADMIN_PASSWORD, MESSAGE_ORIGIN)
    session = browser.get(f"{MESSAGE_ORIGIN}/api/session")
    require_status(session, 200, "load Message Desk session before local logout")
    csrf = session.json().get("csrfToken", "")
    require(csrf, "Message Desk session omitted CSRF token before local logout")
    logged_out = browser.request(
        "POST",
        f"{MESSAGE_ORIGIN}/auth/logout/local",
        headers={"X-CSRF-Token": csrf},
    )
    require_status(logged_out, 204, "log out of Message Desk only")
    signup_page = browser.get(f"{MESSAGE_ORIGIN}/auth/register?return_to=/")
    require_status(signup_page, 200, "start signup with remembered TinyIDP session")
    signup_form = parse_form(signup_page)
    require(
        {"display_name", "email"}.issubset(signup_form.values),
        f"remembered-session signup did not render identity fields: {signup_form.values.keys()}",
    )


def goja_session(browser: Browser) -> dict[str, Any]:
    response = browser.get(f"{GOJA_ORIGIN}/auth/session")
    require_status(response, 200, "load goja session")
    session = response.json()
    require(session.get("csrfToken"), f"goja session has no CSRF token: {session}")
    return session


def issue_membership_invitation(admin: Browser, email: str) -> dict[str, Any]:
    session = goja_session(admin)
    response = admin.json_request(
        "POST",
        f"{GOJA_ORIGIN}/orgs/o1/invites",
        {"email": email, "role": "viewer"},
        csrf=session["csrfToken"],
    )
    require_status(response, 200, "issue application membership invitation")
    invitation = response.json()
    require(invitation.get("token") and invitation.get("capabilityId"), f"incomplete app invitation: {invitation}")
    return invitation


def postgres_scalar(sql: str) -> str:
    return compose("exec", "-T", "postgres", "psql", "-U", "goja", "-d", "goja_auth", "-Atc", sql).strip()


def main() -> None:
    run_id = secrets.token_hex(6)
    password = f"phase-five-{run_id}-correct-horse-battery-staple"

    print("1/9 themed cross-site registration rejection")
    reject_cross_site_registration_with_themed_page()
    print("OK rejected registration returned Message Desk HTML without submitted identity data")

    print("2/9 signup after Message Desk-only logout")
    begin_signup_after_local_logout()
    print("OK remembered TinyIDP session did not block explicit signup")

    print("3/9 Message Desk open-signup browser journey")
    message_email = f"message-{run_id}@example.test"
    message_browser = Browser()
    _, message_code = signup(
        message_browser,
        f"{MESSAGE_ORIGIN}/auth/register?return_to=/",
        display_name="Phase Five Message User",
        email=message_email,
        password=password,
        expected_origin=MESSAGE_ORIGIN,
        invite_code=None,
        restart_after_delivery=True,
    )
    message_session = message_browser.get(f"{MESSAGE_ORIGIN}/api/session")
    require_status(message_session, 200, "load Message Desk session")
    require(message_session.json().get("authenticated") is True, "Message Desk signup did not establish a session")
    print(f"OK open signup established Message Desk session for {message_email}")

    print("4/9 administrator OIDC login and application invitation issuance")
    admin = Browser()
    login(admin, f"{GOJA_ORIGIN}/auth/login?return_to=/", ADMIN_LOGIN, ADMIN_PASSWORD, GOJA_ORIGIN)
    admin_session = goja_session(admin)
    require(admin_session.get("emailVerified") is True, f"admin fixture is not verified: {admin_session}")

    new_goja_email = f"goja-new-{run_id}@example.test"
    new_user_app_invite = issue_membership_invitation(admin, new_goja_email)
    new_user_pending = begin_membership_invitation(Browser(), new_user_app_invite["token"])
    print(f"OK issued email-bound application invite for {new_goja_email}")

    print("5/9 invite-gated TinyIDP signup and OIDC callback")
    signup_invite = issue_signup_invitation()
    new_goja_browser = Browser()
    completed, goja_code = signup(
        new_goja_browser,
        urllib.parse.urljoin(GOJA_ORIGIN, new_user_pending["registrationUrl"]),
        display_name="Phase Five Goja User",
        email=new_goja_email,
        password=password,
        expected_origin=GOJA_ORIGIN,
        invite_code=signup_invite["code"],
        retry_wrong_code=True,
    )
    pending_handle = urllib.parse.parse_qs(urllib.parse.urlsplit(completed.url).query).get("pending", [""])[0]
    require(pending_handle, f"pending app invitation was not restored after signup: {completed.url}")
    new_session = goja_session(new_goja_browser)
    require(new_session.get("email") == new_goja_email, f"unexpected normalized new user: {new_session}")
    require(new_session.get("emailVerified") is True, f"verified signup did not produce a verified app user: {new_session}")
    print("OK pending handle and verified email survived registration, authorization, callback, and app session creation")

    print("6/9 newly verified user accepts the email-bound application invitation")
    accepted_new = new_goja_browser.json_request(
        "POST",
        f"{GOJA_ORIGIN}/org-invites/accept",
        {"pending": pending_handle},
        csrf=new_session["csrfToken"],
    )
    require_status(accepted_new, 200, "accept newly verified application invitation")
    accepted_new_body = accepted_new.json()
    require(
        accepted_new_body.get("orgId") == "o1" and accepted_new_body.get("role") == "viewer",
        f"unexpected new-user acceptance: {accepted_new_body}",
    )
    new_pending_replay = new_goja_browser.json_request(
        "POST",
        f"{GOJA_ORIGIN}/org-invites/accept",
        {"pending": pending_handle},
        csrf=new_session["csrfToken"],
    )
    require(new_pending_replay.status in {403, 409}, f"new-user pending replay returned {new_pending_replay.status}")
    new_raw_replay = Browser().json_request(
        "POST", f"{GOJA_ORIGIN}/org-invites/begin", {"token": new_user_app_invite["token"]}
    )
    require_status(new_raw_replay, 400, "reject consumed new-user raw application invitation")
    new_membership_count = postgres_scalar(
        "SELECT count(*) FROM auth_app_memberships m JOIN auth_app_users u ON u.id=m.user_id "
        f"WHERE lower(u.email)=lower('{new_goja_email}') AND m.tenant_id='o1' "
        "AND m.role='viewer' AND m.revoked_at IS NULL"
    )
    require(new_membership_count == "1", f"expected one new-user viewer membership, got {new_membership_count}")
    print("OK verified signup immediately received one membership and rejected both replay paths")

    print("7/9 one-time TinyIDP signup invitation replay rejection")
    replay_browser = Browser()
    replay_page = replay_browser.get(f"{GOJA_ORIGIN}/auth/register?return_to=/")
    replay_email = f"goja-replay-{run_id}@example.test"
    replay_result = replay_browser.submit_first_form(
        replay_page,
        {
            "display_name": "Replay Attempt",
            "email": replay_email,
            "invite_code": signup_invite["code"],
            "action": "submit",
        },
    )
    require_status(replay_result, 400, "render replay denial")
    require(replay_result.url.startswith(IDP_ORIGIN), "replayed signup invitation unexpectedly left TinyIDP")
    require("This value could not be accepted." in replay_result.body, "replay denial was not rendered on the form")
    print("OK consumed signup invitation produced a stable field-level denial")

    no_replay_mail = outbox_request(
        f"/view/latest.txt?{urllib.parse.urlencode({'query': f'to:\"{replay_email}\"'})}"
    )
    require_status(no_replay_mail, 404, "invalid signup invitation must not send email")
    print("OK consumed signup invitation was denied before mail delivery")

    print("8/9 verified existing-user membership acceptance remains supported")
    existing_invite = issue_membership_invitation(admin, INVITEE_LOGIN)
    existing_browser = Browser()
    existing_pending = begin_membership_invitation(existing_browser, existing_invite["token"])
    login(
        existing_browser,
        urllib.parse.urljoin(GOJA_ORIGIN, existing_pending["loginUrl"]),
        INVITEE_LOGIN,
        INVITEE_PASSWORD,
        GOJA_ORIGIN,
    )
    existing_session = goja_session(existing_browser)
    require(existing_session.get("email") == INVITEE_LOGIN, f"unexpected invitee session: {existing_session}")
    require(existing_session.get("emailVerified") is True, f"invitee fixture is not verified: {existing_session}")
    # The opaque handle is carried inside the login URL's local return_to.
    return_to = urllib.parse.parse_qs(urllib.parse.urlsplit(existing_pending["loginUrl"]).query)["return_to"][0]
    accepted_handle = urllib.parse.parse_qs(urllib.parse.urlsplit(return_to).query)["pending"][0]
    accepted = existing_browser.json_request(
        "POST",
        f"{GOJA_ORIGIN}/org-invites/accept",
        {"pending": accepted_handle},
        csrf=existing_session["csrfToken"],
    )
    require_status(accepted, 200, "accept verified application invitation")
    accepted_body = accepted.json()
    require(accepted_body.get("orgId") == "o1" and accepted_body.get("role") == "viewer", f"unexpected acceptance: {accepted_body}")
    replay = existing_browser.json_request(
        "POST",
        f"{GOJA_ORIGIN}/org-invites/accept",
        {"pending": accepted_handle},
        csrf=existing_session["csrfToken"],
    )
    require(replay.status in {403, 409}, f"accepted pending handle replay returned {replay.status}: {replay.body}")
    raw_replay_response = Browser().json_request(
        "POST", f"{GOJA_ORIGIN}/org-invites/begin", {"token": existing_invite["token"]}
    )
    require_status(raw_replay_response, 400, "reject consumed raw application invitation")
    membership_count = postgres_scalar(
        "SELECT count(*) FROM auth_app_memberships m JOIN auth_app_users u ON u.id=m.user_id "
        "WHERE lower(u.email)=lower('invitee@example.test') AND m.tenant_id='o1' "
        "AND m.role='viewer' AND m.revoked_at IS NULL"
    )
    require(membership_count == "1", f"expected one active viewer membership, got {membership_count}")
    print("OK verified invitee received one membership and both pending/raw replay paths were rejected")

    print("9/9 operational audit evidence")
    audit_response = admin.get(f"{GOJA_ORIGIN}/orgs/o1/audit?limit=100")
    require_status(audit_response, 200, "query application audit")
    audit_text = json.dumps(audit_response.json(), sort_keys=True)
    require("org.invite.issued" in audit_text, "application audit is missing invitation issuance")
    require("org.invite.accepted" in audit_text, "application audit is missing invitation acceptance")
    require(
        new_user_app_invite["capabilityId"] in audit_text and existing_invite["capabilityId"] in audit_text,
        "tenant audit does not contain both capabilities accepted in this run",
    )
    require(
        new_user_app_invite["token"] not in audit_text and existing_invite["token"] not in audit_text,
        "application audit contains a raw membership invitation token",
    )
    idp_audit = compose(
        "exec",
        "-T",
        "idp",
        "sh",
        "-c",
        "cat /state/tinyidp.sqlite.audit.jsonl /state/audit/audit.jsonl",
    )
    require("signup_invitation.issued" in idp_audit, "TinyIDP audit is missing signup invitation issuance")
    require("signup_invitation.consumed" in idp_audit, "TinyIDP audit is missing signup invitation consumption")
    require("workflow.signup.email_challenge_send" in idp_audit, "TinyIDP audit is missing email challenge delivery")
    require(
        signup_invite["invitation_id"] in idp_audit,
        "TinyIDP audit does not contain the signup invitation consumed in this run",
    )
    require(signup_invite["code"] not in idp_audit, "TinyIDP audit contains a raw signup invitation code")
    require(message_code not in idp_audit and goja_code not in idp_audit, "TinyIDP audit contains a raw email code")
    service_logs = compose("logs", "--no-color", "idp", "goja-auth")
    require(
        signup_invite["code"] not in service_logs and message_code not in service_logs and goja_code not in service_logs,
        "service logs contain a raw invitation or email challenge code",
    )
    print("OK both services expose issuance/acceptance evidence without raw bearer values")

    print("PASS: shared TinyIDP Phase 5 browser acceptance completed")


if __name__ == "__main__":
    try:
        main()
    except (AcceptanceFailure, OSError, subprocess.SubprocessError) as exc:
        print(f"FAIL: {exc}", file=sys.stderr)
        raise SystemExit(1)
