import { expect, Page, test } from "@playwright/test";
import { execFile } from "node:child_process";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { promisify } from "node:util";

const messageOrigin = process.env.TINYIDP_TEST_MESSAGE_ORIGIN ?? "https://message.localhost:8443";
const idpOrigin = process.env.TINYIDP_TEST_IDP_ORIGIN ?? "https://idp.localhost:8443";
const gojaOrigin = process.env.TINYIDP_TEST_GOJA_ORIGIN ?? "https://goja.localhost:8443";
const outboxOrigin = process.env.TINYIDP_TEST_OUTBOX_ORIGIN ?? "http://127.0.0.1:8025";
const outboxAuthorization = process.env.TINYIDP_TEST_OUTBOX_AUTHORIZATION;
const execFileAsync = promisify(execFile);
const composeFile = resolve(process.cwd(), "../compose.yaml");
const secretDir = resolve(process.cwd(), "../runtime/secrets");

function localSecret(envName: string, fileName: string): string {
  return process.env[envName] ??
    readFileSync(resolve(secretDir, fileName), "utf8").trim();
}

async function issueSignupInvitation(audience: string, ttl = "1h"): Promise<string> {
  const namespace = process.env.TINYIDP_TEST_KUBECTL_NAMESPACE;
  const command = namespace ? "kubectl" : "docker";
  const args = namespace ? [
    "-n", namespace, "exec", "deploy/tinyidp", "--",
    "tinyidp", "admin", "--db=/var/lib/tinyidp/tinyidp.sqlite", "invitation", "issue",
    `--audience=${audience}`,
    "--policy-version=signup-invite-v1",
    `--ttl=${ttl}`,
    "--lookup-key-file=/run/tinyidp-secrets/invitation-lookup.key",
    "--output=json"
  ] : [
    "compose", "-f", composeFile, "exec", "-T", "idp",
    "tinyidp", "admin", "--db=/state/tinyidp.sqlite", "invitation", "issue",
    `--audience=${audience}`,
    "--policy-version=signup-invite-v1",
    `--ttl=${ttl}`,
    "--lookup-key-file=/state/.secrets/invitation_lookup_key",
    "--output=json"
  ];
  const {stdout} = await execFileAsync(command, args);
  const rows = JSON.parse(stdout) as Array<{ code?: string }>;
  if (rows.length !== 1 || !rows[0].code) throw new Error("invitation issue command returned no one-time code");
  return rows[0].code;
}

async function expectMessageDeskTheme(page: Page): Promise<void> {
  await expect(page.locator('link[rel="stylesheet"]')).toHaveAttribute(
    "href",
    "/static/themes/message-desk.css"
  );
}

async function expectGojaAuthTheme(page: Page): Promise<void> {
  await expect(page.locator('link[rel="stylesheet"]')).toHaveAttribute(
    "href",
    "/static/themes/goja-auth-lab.css"
  );
}

async function beginMessageSignup(page: Page): Promise<void> {
  await page.goto(`${messageOrigin}/auth/register?return_to=/`);
  await expect(page).toHaveURL(new RegExp(`^${idpOrigin.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")}/authorize`));
  await expect(page.getByRole("heading", { name: "Create an account" })).toBeVisible();
  await expectMessageDeskTheme(page);
}

async function beginGojaSignup(page: Page): Promise<void> {
  await page.goto(`${gojaOrigin}/auth/register?return_to=/`);
  await expect(page).toHaveURL(new RegExp(`^${idpOrigin.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")}/authorize`));
  await expect(page.getByRole("heading", { name: "Create an account" })).toBeVisible();
  await expectGojaAuthTheme(page);
}

async function latestEmailCode(page: Page, recipient: string): Promise<string> {
  const query = encodeURIComponent(`to:"${recipient}"`);
  const headers = outboxAuthorization ? {Authorization: outboxAuthorization} : undefined;
  await expect
    .poll(async () => {
      const response = await page.request.get(`${outboxOrigin}/view/latest.txt?query=${query}`, {
        headers
      });
      if (!response.ok()) return "";
      return (await response.text()).match(/verification code is:\s*([A-Z2-7]{8})/)?.[1] ?? "";
    })
    .not.toBe("");
  const response = await page.request.get(`${outboxOrigin}/view/latest.txt?query=${query}`, {
    headers
  });
  const code = (await response.text()).match(/verification code is:\s*([A-Z2-7]{8})/)?.[1];
  if (!code) throw new Error(`Mailpit did not expose a verification code for ${recipient}`);
  return code;
}

async function submitIdentity(page: Page, displayName: string, email: string): Promise<void> {
  await page.getByLabel("Display name").fill(displayName);
  await page.getByLabel("Email").fill(email);
  await page.getByRole("button", { name: "Create account" }).click();
}

async function loginToMessageDesk(
  page: Page,
  login = "admin@example.test",
  password = localSecret("TINYIDP_TEST_ADMIN_PASSWORD", "local-admin-password.txt")
): Promise<void> {
  await page.goto(`${messageOrigin}/auth/login?return_to=/`);
  await page.getByLabel("Login").fill(login);
  await page.getByLabel("Password").fill(password);
  await page.getByRole("button", { name: /continue|sign in|approve/i }).first().click();
  if (page.url().startsWith(idpOrigin)) {
    await page.getByRole("button", { name: /approve|continue/i }).first().click();
  }
  await expect(page).toHaveURL(new RegExp(`^${messageOrigin.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")}/`));
  await expect(page.getByText("SIGNED IN")).toBeVisible();
}

async function loginToGojaAuth(
  page: Page,
  login = "admin@example.test",
  password = localSecret("TINYIDP_TEST_ADMIN_PASSWORD", "local-admin-password.txt")
): Promise<void> {
  await page.goto(`${gojaOrigin}/auth/login?return_to=/`);
  if (page.url().startsWith(idpOrigin)) {
    const loginField = page.getByLabel("Login");
    if (await loginField.isVisible()) {
      await loginField.fill(login);
      await page.getByLabel("Password").fill(password);
      await page.getByRole("button", { name: /continue|sign in|approve/i }).first().click();
    }
    if (page.url().startsWith(idpOrigin)) {
      await page.getByRole("button", { name: /approve|continue/i }).first().click();
    }
  }
  await expect(page).toHaveURL(new RegExp(`^${gojaOrigin.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")}/`));
  await expect(page.locator("#session-status")).toContainText("Logged in");
}

test.beforeEach(async ({ page }) => {
  const pageErrors: string[] = [];
  page.on("pageerror", error => pageErrors.push(error.message));
  (page as Page & { productPageErrors?: string[] }).productPageErrors = pageErrors;
});

test.afterEach(async ({ page }) => {
  expect((page as Page & { productPageErrors?: string[] }).productPageErrors).toEqual([]);
});

test("Message Desk presents signup and login as separate vertically stacked actions", async ({ page }) => {
  await page.goto(messageOrigin);
  const signup = page.getByRole("link", { name: "Create an account with Tiny-IDP" });
  const login = page.getByRole("link", { name: "I already have an account" });
  await expect(signup).toBeVisible();
  await expect(login).toBeVisible();
  const signupBox = await signup.boundingBox();
  const loginBox = await login.boundingBox();
  expect(signupBox).not.toBeNull();
  expect(loginBox).not.toBeNull();
  expect(loginBox!.y).toBeGreaterThan(signupBox!.y + signupBox!.height - 1);
});

test("malformed signup email stays on the themed form with native validation", async ({ page }) => {
  await beginMessageSignup(page);
  await page.getByLabel("Display name").fill("Malformed Email");
  await page.getByLabel("Email").fill("not-an-email");
  await page.getByRole("button", { name: "Create account" }).click();
  await expect(page.getByLabel("Email")).toBeFocused();
  expect(await page.getByLabel("Email").evaluate((input: HTMLInputElement) => input.validity.typeMismatch)).toBe(true);
  await expect(page.getByRole("heading", { name: "Create an account" })).toBeVisible();
  await expectMessageDeskTheme(page);
});

test("signup identity form enforces required and bounded display names before submission", async ({ page }) => {
  await beginMessageSignup(page);
  const displayName = page.getByLabel("Display name");
  await expect(displayName).toHaveAttribute("maxlength", "120");
  await page.getByLabel("Email").fill("display-name-validation@example.test");
  await page.getByRole("button", { name: "Create account" }).click();
  await expect(displayName).toBeFocused();
  expect(await displayName.evaluate((input: HTMLInputElement) => input.validity.valueMissing)).toBe(true);
  await expect(page.getByRole("heading", { name: "Create an account" })).toBeVisible();
  await expectMessageDeskTheme(page);
});

test("open Message Desk signup reaches the signed-in application through TinyIDP", async ({ page, context }) => {
  const suffix = Date.now();
  const displayName = `Playwright Happy Signup ${suffix}`;
  const email = `playwright-happy-signup-${suffix}@example.test`;
  await beginMessageSignup(page);
  await submitIdentity(page, displayName, email);
  await page.getByLabel("Email verification code").fill(await latestEmailCode(page, email));
  await page.getByRole("button", { name: "Create account" }).click();
  await expect(page.getByLabel("Password", { exact: true })).toBeVisible();
  await page.getByLabel("Password", { exact: true }).fill("playwright happy signup password 2026!");
  await page.getByLabel("Confirm password").fill("playwright happy signup password 2026!");
  await page.getByRole("button", { name: "Create account" }).click();
  if (page.url().startsWith(idpOrigin)) {
    await expect.poll(async () => (await context.cookies(idpOrigin)).some(cookie => cookie.name === "tinyidp_session" && cookie.value !== "")).toBe(true);
    await page.getByRole("button", { name: /approve|continue/i }).first().click();
  }
  await expect(page).toHaveURL(new RegExp(`^${messageOrigin.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")}/`));
  await expect(page.getByText(displayName, { exact: true })).toBeVisible();
  await expect(page.getByText("SIGNED IN")).toBeVisible();
});

test("remembered TinyIDP session can submit the first add-account signup step", async ({ page }) => {
  await loginToMessageDesk(page);
  await page.getByRole("button", { name: "Log out of Message Desk" }).click();
  await expect(page.getByText("GUEST MODE")).toBeVisible();
  await beginMessageSignup(page);
  const email = `playwright-add-account-${Date.now()}@example.test`;
  await submitIdentity(page, "Playwright Add Account", email);
  await expect(page.getByLabel("Email verification code")).toBeVisible();
  await expectMessageDeskTheme(page);
});

test("Message Desk local logout leaves the TinyIDP and Goja Auth sessions usable", async ({ page }) => {
  await loginToMessageDesk(page);
  await loginToGojaAuth(page);
  const beforeLogout = await page.request.get(`${gojaOrigin}/auth/session`);
  expect(beforeLogout.ok()).toBe(true);
  expect((await beforeLogout.json()).email).toBe("admin@example.test");

  await page.goto(messageOrigin);
  await expect(page.getByText("SIGNED IN")).toBeVisible();
  await page.getByRole("button", { name: "Log out of Message Desk" }).click();
  await expect(page.getByText("GUEST MODE")).toBeVisible();

  const afterLogout = await page.request.get(`${gojaOrigin}/auth/session`);
  expect(afterLogout.ok()).toBe(true);
  expect((await afterLogout.json()).email).toBe("admin@example.test");
  await page.goto(gojaOrigin);
  await expect(page.locator("#session-status")).toContainText("Logged in");
});

test("logging out everywhere clears Message Desk and TinyIDP browser sessions", async ({ page, context }) => {
  await loginToMessageDesk(page);
  await page.getByRole("button", { name: "Log out everywhere" }).click();
  await expect(page).toHaveURL(new RegExp(`^${messageOrigin.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")}/`));
  await expect(page.getByText("GUEST MODE")).toBeVisible();
  const idpCookies = await context.cookies(idpOrigin);
  expect(idpCookies.some(cookie => cookie.name === "tinyidp_session" && cookie.value !== "")).toBe(false);
});

test("account chooser remembers two password logins and supports switching accounts", async ({ page }) => {
  await loginToMessageDesk(page);
  await page.getByRole("link", { name: "Change account" }).click();
  await expect(page.getByRole("heading", { name: "Choose an account" })).toBeVisible();
  await expect(page.getByLabel("Local Administrator")).toBeVisible();
  await page.getByRole("button", { name: "Use another account" }).click();

  await page.getByLabel("Login").fill("invitee@example.test");
  await page.getByLabel("Password").fill(
    localSecret("TINYIDP_TEST_INVITEE_PASSWORD", "local-invitee-password.txt")
  );
  await page.getByRole("button", { name: /continue|sign in|approve/i }).first().click();
  if (page.url().startsWith(idpOrigin)) {
    await page.getByRole("button", { name: /approve|continue/i }).first().click();
  }
  await expect(page).toHaveURL(new RegExp(`^${messageOrigin.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")}/`));
  await expect(page.getByText("Local Invitee", { exact: true })).toBeVisible();

  await page.getByRole("link", { name: "Change account" }).click();
  await expect(page.getByRole("heading", { name: "Choose an account" })).toBeVisible();
  await expect(page.getByLabel("Local Administrator")).toBeVisible();
  await expect(page.getByLabel("Local Invitee")).toBeVisible();
  await page.getByLabel("Local Administrator").check();
  await page.getByRole("button", { name: "Continue" }).click();
  await expect(page).toHaveURL(new RegExp(`^${messageOrigin.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")}/`));
  await expect(page.getByText("Local Administrator", { exact: true })).toBeVisible();

  await page.getByRole("link", { name: "Change account" }).click();
  await page.getByLabel("Local Invitee").check();
  await page.getByRole("button", { name: "Remove account" }).click();
  await expect(page.getByLabel("Local Invitee")).toHaveCount(0);
  await expect(page.getByLabel("Local Administrator")).toBeVisible();
});

test("short password is rejected by native validation before the password workflow posts", async ({ page }) => {
  const email = `playwright-short-password-${Date.now()}@example.test`;
  await beginMessageSignup(page);
  await submitIdentity(page, "Playwright Short Password", email);
  await page.getByLabel("Email verification code").fill(await latestEmailCode(page, email));
  await page.getByRole("button", { name: /create account|continue/i }).click();
  const password = page.getByLabel("Password", { exact: true });
  await password.fill("too-short-2026");
  await page.getByLabel("Confirm password").fill("too-short-2026");
  await page.getByRole("button", { name: "Create account" }).click();
  await expect(password).toBeFocused();
  expect(await password.evaluate((input: HTMLInputElement) => input.validity.tooShort)).toBe(true);
  await expectMessageDeskTheme(page);
});

test("password mismatch stays on the themed password form with a confirmation error", async ({ page }) => {
  const email = `playwright-password-mismatch-${Date.now()}@example.test`;
  await beginMessageSignup(page);
  await submitIdentity(page, "Playwright Password Mismatch", email);
  await page.getByLabel("Email verification code").fill(await latestEmailCode(page, email));
  await page.getByRole("button", { name: /create account|continue/i }).click();
  await page.getByLabel("Password", { exact: true }).fill("first acceptable playwright password 2026!");
  const confirmation = page.getByLabel("Confirm password");
  await confirmation.fill("different acceptable playwright password 2026!");
  await page.getByRole("button", { name: "Create account" }).click();

  await expect(page.getByText("The values do not match.")).toBeVisible();
  await expect(confirmation).toHaveAttribute("aria-invalid", "true");
  await expect(page.getByLabel("Password", { exact: true })).toHaveValue("");
  await expect(confirmation).toHaveValue("");
  await expectMessageDeskTheme(page);
});

test("wrong email verification code keeps a themed retry form with resend", async ({ page }) => {
  const email = `playwright-wrong-code-${Date.now()}@example.test`;
  await beginMessageSignup(page);
  await submitIdentity(page, "Playwright Wrong Code", email);
  const code = page.getByLabel("Email verification code");
  await expect(code).toBeVisible();
  await code.fill("AAAAAAAA");
  await page.getByRole("button", { name: "Create account" }).click();

  await expect(code).toBeVisible();
  await expect(code).toHaveValue("");
  await expect(page.getByText("This value could not be accepted.")).toBeVisible();
  await expect(page.getByRole("button", { name: "Send another code" })).toBeVisible();
  await expectMessageDeskTheme(page);
});

test("email-code resend keeps a blank themed retry workflow", async ({ page }) => {
  const email = `playwright-resend-code-${Date.now()}@example.test`;
  await beginMessageSignup(page);
  await submitIdentity(page, "Playwright Resend Code", email);
  await page.getByRole("button", { name: "Send another code" }).click();
  const code = page.getByLabel("Email verification code");
  await expect(code).toBeVisible();
  await expect(code).toHaveValue("");
  await expect(page.getByRole("button", { name: "Send another code" })).toBeVisible();
  await expectMessageDeskTheme(page);
});

test("email-code exhaustion stays themed and a resend invalidates the old code", async ({ page }) => {
	const email = `playwright-code-exhaustion-${Date.now()}@example.test`;
	await beginMessageSignup(page);
	await submitIdentity(page, "Playwright Code Exhaustion", email);
	const originalCode = await latestEmailCode(page, email);
	const code = page.getByLabel("Email verification code");
	for (let attempt = 0; attempt < 5; attempt++) {
		await code.fill("AAAAAAAA");
		await page.getByRole("button", { name: "Create account" }).click();
		await expect(code).toHaveValue("");
	}
	await expect(page.getByText("Too many incorrect verification codes were entered. Request a new code to try again.")).toBeVisible();
	await expect(page.getByLabel("Password", { exact: true })).toHaveCount(0);
	await expect(page.getByRole("button", { name: "Send another code" })).toBeVisible();
	await expectMessageDeskTheme(page);

	await page.getByRole("button", { name: "Send another code" }).click();
	await expect(code).toBeVisible();
	await expect(code).toHaveValue("");
	const replacementCode = await latestEmailCode(page, email);
	expect(replacementCode).not.toBe(originalCode);

	await code.fill(originalCode);
	await page.getByRole("button", { name: "Create account" }).click();
	await expect(page.getByText("This value could not be accepted.")).toBeVisible();
	await expect(code).toHaveValue("");

	await code.fill(replacementCode);
	await page.getByRole("button", { name: "Create account" }).click();
	await expect(page.getByLabel("Password", { exact: true })).toBeVisible();
});

test("email-code resend limit remains themed and preserves the verification form", async ({ page }) => {
  const email = `playwright-resend-limit-${Date.now()}@example.test`;
  await beginMessageSignup(page);
  await submitIdentity(page, "Playwright Resend Limit", email);
  const resend = page.getByRole("button", { name: "Send another code" });
  await resend.click();
  await resend.click();
  await resend.click();
  await expect(page.getByText("No more verification codes can be sent for this registration. Enter the most recent code or restart registration.")).toBeVisible();
  await expect(page.getByLabel("Email verification code")).toHaveValue("");
  await expectMessageDeskTheme(page);
});

test("replayed signup form returns a themed restart document instead of raw provider text", async ({ page }) => {
	const email = `playwright-stale-signup-${Date.now()}@example.test`;
	await beginMessageSignup(page);
	await submitIdentity(page, "Playwright Stale Signup", email);
	const code = page.getByLabel("Email verification code");
	await expect(code).toBeVisible();
	const verificationCode = await latestEmailCode(page, email);
	const replay = await page.locator("form").evaluate(form => ({
		// The provider's hidden input is named "action", which shadows the
		// DOM form.action property. Read the literal attribute instead.
		postURL: form.getAttribute("action") || "",
		fields: Object.fromEntries(new FormData(form).entries())
	}));
	(replay.fields as Record<string, string>).email_code = verificationCode;
	await code.fill(verificationCode);
	await page.getByRole("button", { name: "Create account" }).click();
	await expect(page.getByLabel("Password", { exact: true })).toBeVisible();

	await page.evaluate(request => {
		const form = document.createElement("form");
		form.method = "post";
		form.action = request.postURL;
		for (const [name, value] of Object.entries(request.fields)) {
			const input = document.createElement("input");
			input.type = "hidden";
			input.name = name;
			input.value = value;
			form.append(input);
		}
		document.body.append(form);
		form.submit();
	}, replay);
	await expect(page.getByRole("heading", { name: "Registration needs to be restarted" })).toBeVisible();
	await expect(page.getByText("This registration page is no longer active. Return to the application and begin registration again.")).toBeVisible();
	await expectMessageDeskTheme(page);
	expect((await page.locator("body").innerText()).toLowerCase()).not.toContain("registration request was not accepted");
});

for (const [name, login, password] of [
  ["unknown login", "not-a-real-account@example.test", "not-the-right-password-2026!"],
  ["wrong password", "admin@example.test", "not-the-right-password-2026!"],
]) {
  test(`${name} retains the login name and presents a themed generic credential error`, async ({ page }) => {
    await page.goto(`${messageOrigin}/auth/login?return_to=/`);
    await page.getByLabel("Login").fill(login);
    await page.getByLabel("Password").fill(password);
    await page.getByRole("button", { name: /continue|sign in|approve/i }).first().click();

    await expect(page.getByText("Invalid login or password.")).toBeVisible();
    await expect(page.getByLabel("Login")).toHaveValue(login);
    await expect(page.getByLabel("Password")).toHaveValue("");
    await expectMessageDeskTheme(page);
  });
}

test("Goja Auth invalid credentials retain login and use the Goja-specific theme", async ({ page }) => {
  await page.goto(`${gojaOrigin}/auth/login?return_to=/`);
  await page.getByLabel("Login").fill("not-a-real-goja-account@example.test");
  await page.getByLabel("Password").fill("not-the-right-password-2026!");
  await page.getByRole("button", { name: /continue|sign in|approve/i }).first().click();

  await expect(page.getByText("Invalid login or password.")).toBeVisible();
  await expect(page.getByLabel("Login")).toHaveValue("not-a-real-goja-account@example.test");
  await expect(page.getByLabel("Password")).toHaveValue("");
  await expectGojaAuthTheme(page);
});

test("Goja signup rejects an unknown invitation with a themed field error", async ({ page }) => {
  await beginGojaSignup(page);
  await page.getByLabel("Display name").fill("Goja Unknown Invitation");
  await page.getByLabel("Email").fill(`playwright-goja-invite-${Date.now()}@example.test`);
  await page.getByLabel("Invite code").fill("not-a-real-invitation-code");
  await page.getByRole("button", { name: "Create account" }).click();

  await expect(page.getByLabel("Invite code")).toHaveValue("not-a-real-invitation-code");
  await expect(page.getByText("This value could not be accepted.")).toBeVisible();
  await expectGojaAuthTheme(page);
});

test("Goja signup rejects a missing invitation with a themed field error", async ({ page }) => {
  await beginGojaSignup(page);
  await page.getByLabel("Display name").fill("Goja Missing Invitation");
  await page.getByLabel("Email").fill(`playwright-goja-missing-invite-${Date.now()}@example.test`);
  const invite = page.getByLabel("Invite code");
  await page.getByRole("button", { name: "Create account" }).click();

  await expect(invite).toHaveAttribute("aria-invalid", "true");
  await expect(page.getByText("This value could not be accepted.")).toBeVisible();
  await expect(page.getByLabel("Email verification code")).toHaveCount(0);
  await expectGojaAuthTheme(page);
});

test("invited Goja signup verifies email and establishes an application session", async ({ page }) => {
  const suffix = Date.now();
  const email = `playwright-goja-happy-${suffix}@example.test`;
  const invitation = await issueSignupInvitation("goja-auth-host-demo");
  await beginGojaSignup(page);
  await page.getByLabel("Display name").fill(`Goja Happy Signup ${suffix}`);
  await page.getByLabel("Email").fill(email);
  await page.getByLabel("Invite code").fill(invitation);
  await page.getByRole("button", {name: "Create account"}).click();
  await page.getByLabel("Email verification code").fill(await latestEmailCode(page, email));
  await page.getByRole("button", {name: /create account|continue/i}).click();
  await page.getByLabel("Password", {exact: true}).fill("playwright invited goja signup password 2026!");
  await page.getByLabel("Confirm password").fill("playwright invited goja signup password 2026!");
  await page.getByRole("button", {name: "Create account"}).click();
  if (page.url().startsWith(idpOrigin)) {
    await page.getByRole("button", {name: /approve|continue/i}).first().click();
  }
  await expect(page).toHaveURL(new RegExp(`^${gojaOrigin.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")}/`));
  const response = await page.request.get(`${gojaOrigin}/auth/session`);
  expect(response.ok()).toBe(true);
  expect((await response.json()).email).toBe(email);
});

test("a consumed Goja signup invitation remains a themed non-revealing field error", async ({ page }) => {
  const suffix = Date.now();
  const invitation = await issueSignupInvitation("goja-auth-host-demo");
  const firstEmail = `playwright-goja-consume-first-${suffix}@example.test`;
  await beginGojaSignup(page);
  await page.getByLabel("Display name").fill(`Goja Consume First ${suffix}`);
  await page.getByLabel("Email").fill(firstEmail);
  await page.getByLabel("Invite code").fill(invitation);
  await page.getByRole("button", {name: "Create account"}).click();
  await page.getByLabel("Email verification code").fill(await latestEmailCode(page, firstEmail));
  await page.getByRole("button", {name: /create account|continue/i}).click();
  await page.getByLabel("Password", {exact: true}).fill("playwright consumed invitation password 2026!");
  await page.getByLabel("Confirm password").fill("playwright consumed invitation password 2026!");
  await page.getByRole("button", {name: "Create account"}).click();
  if (page.url().startsWith(idpOrigin)) {
    await page.getByRole("button", {name: /approve|continue/i}).first().click();
  }
  await expect(page).toHaveURL(new RegExp(`^${gojaOrigin.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")}/`));

  await beginGojaSignup(page);
  await page.getByLabel("Display name").fill(`Goja Consume Replay ${suffix}`);
  await page.getByLabel("Email").fill(`playwright-goja-consume-replay-${suffix}@example.test`);
  const replayedInvitation = page.getByLabel("Invite code");
  await replayedInvitation.fill(invitation);
  await page.getByRole("button", {name: "Create account"}).click();
  await expect(page.getByText("This value could not be accepted.")).toBeVisible();
  await expect(replayedInvitation).toHaveAttribute("aria-invalid", "true");
  await expect(page.getByLabel("Email verification code")).toHaveCount(0);
  await expectGojaAuthTheme(page);
});

for (const invitationCase of [
  { name: "expired", audience: "goja-auth-host-demo", ttl: "1ms" },
  { name: "wrong-audience", audience: "tinyidp-message-app", ttl: "1h" }
]) {
  test(`Goja signup rejects a ${invitationCase.name} invitation with the same themed field error`, async ({ page }) => {
    const invitation = await issueSignupInvitation(invitationCase.audience, invitationCase.ttl);
    await beginGojaSignup(page);
    await page.getByLabel("Display name").fill(`Goja ${invitationCase.name} Invitation`);
    await page.getByLabel("Email").fill(`playwright-goja-${invitationCase.name}-${Date.now()}@example.test`);
    await page.getByLabel("Invite code").fill(invitation);
    await page.getByRole("button", { name: "Create account" }).click();

    await expect(page.getByText("This value could not be accepted.")).toBeVisible();
    await expect(page.getByLabel("Invite code")).toHaveAttribute("aria-invalid", "true");
    await expect(page.getByLabel("Email verification code")).toHaveCount(0);
    await expectGojaAuthTheme(page);
  });
}

test("Message Desk OIDC callback error is an application-styled recovery page", async ({ page }) => {
  await page.goto(`${messageOrigin}/auth/callback?error=access_denied&error_description=untrusted-provider-text&state=missing`);
  await expect(page.getByRole("heading", { name: "Sign-in was cancelled" })).toBeVisible();
  await expect(page.locator('link[rel="stylesheet"]')).toHaveAttribute("href", "/static/app/assets/index.css");
  await expect(page.getByRole("link", { name: "Try signing in again" })).toBeVisible();
  await expect(page.getByRole("link", { name: "Return to Message Desk" })).toBeVisible();
  expect(await page.locator("body").innerText()).not.toContain("untrusted-provider-text");
});

test("Goja Auth OIDC callback error is an application-styled safe recovery page", async ({ page }) => {
  await page.goto(`${gojaOrigin}/auth/callback?error=access_denied&error_description=untrusted-provider-text&state=missing`);
  await expect(page.getByRole("heading", { name: "Sign-in was canceled" })).toBeVisible();
  await expect(page.locator('link[rel="stylesheet"]')).toHaveAttribute("href", "/static/styles.css");
  await expect(page.getByRole("link", { name: "Try signing in again" })).toBeVisible();
  await expect(page.getByRole("link", { name: "Return to the application" })).toBeVisible();
  expect(await page.locator("body").innerText()).not.toContain("untrusted-provider-text");
});

test("duplicate email produces a themed actionable signup error", async ({ page }) => {
  const email = "admin@example.test";
  await beginMessageSignup(page);
  await submitIdentity(page, "Duplicate Administrator", email);
  await expect(page.getByLabel("Email verification code")).toBeVisible();
  await page.getByLabel("Email verification code").fill(await latestEmailCode(page, email));
  await page.getByRole("button", { name: /create account|continue/i }).click();
  await expect(page.getByRole("heading", { name: "Create an account" })).toBeVisible();
  await page.getByLabel("Password", { exact: true }).fill("duplicate account password 2026!");
  await page.getByLabel("Confirm password").fill("duplicate account password 2026!");
  await page.getByRole("button", { name: "Create account" }).click();

  await expect(page.getByText("An account already uses this email address.")).toBeVisible();
  await expect(page.getByRole("link", { name: "Return to application" })).toBeVisible();
  await expectMessageDeskTheme(page);
  expect((await page.locator("body").innerText()).toLowerCase()).not.toContain("registration request was not accepted");
});

test("duplicate display name is rejected on the themed identity form before email verification", async ({ page }) => {
  const suffix = Date.now();
  const displayName = `Playwright Unique Name ${suffix}`;
  const firstEmail = `playwright-name-first-${suffix}@example.test`;
  await beginMessageSignup(page);
  await submitIdentity(page, displayName, firstEmail);
  await expect(page.getByLabel("Email verification code")).toBeVisible();
  await page.getByLabel("Email verification code").fill(await latestEmailCode(page, firstEmail));
  await page.getByRole("button", { name: /create account|continue/i }).click();
  await expect(page.getByLabel("Password", { exact: true })).toBeVisible();
  await page.getByLabel("Password", { exact: true }).fill("playwright unique display name password 2026!");
  await page.getByLabel("Confirm password").fill("playwright unique display name password 2026!");
  await page.getByRole("button", { name: "Create account" }).click();
  await expect(page.getByRole("heading", { name: "Approve access" })).toBeVisible();

  await beginMessageSignup(page);
  await submitIdentity(page, displayName, `playwright-name-second-${suffix}@example.test`);
  await expect(page.getByRole("heading", { name: "Create an account" })).toBeVisible();
  await expect(page.getByText("That display name is already in use. Choose another.")).toBeVisible();
  await expect(page.getByLabel("Email verification code")).toHaveCount(0);
  await expectMessageDeskTheme(page);
});
