import { Browser, BrowserContext, expect, Page, test } from "@playwright/test";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";

const idpOrigin = "https://idp.localhost:8443";
const meetOrigin = "https://meet.localhost:8443";
const adminLogin = "admin@example.test";
const secretDir = resolve(process.cwd(), "../runtime/secrets");

function localSecret(envName: string, fileName: string): string {
  return process.env[envName] ??
    readFileSync(resolve(secretDir, fileName), "utf8").trim();
}

async function conferenceJoined(page: Page): Promise<boolean> {
  return page.evaluate(() => {
    const application = (window as typeof window & {
      APP?: { store?: { getState(): Record<string, unknown> } };
    }).APP;
    const state = application?.store?.getState() as {
      "features/base/conference"?: { conference?: { isJoined(): boolean } };
    } | undefined;
    return state?.["features/base/conference"]?.conference?.isJoined() === true;
  });
}

async function participantCount(page: Page): Promise<number> {
  return page.evaluate(() => {
    const application = (window as typeof window & {
      APP?: { store?: { getState(): Record<string, unknown> } };
    }).APP;
    const state = application?.store?.getState() as {
      "features/base/participants"?: {
        local?: unknown;
        remote: Map<string, unknown>;
      };
    } | undefined;
    const participants = state?.["features/base/participants"];
    return (participants?.local ? 1 : 0) + (participants?.remote?.size ?? 0);
  });
}

async function mediaConnected(page: Page): Promise<boolean> {
  return page.evaluate(() => {
    const application = (window as typeof window & {
      APP?: { conference?: { getConnectionState(): string } };
    }).APP;
    return application?.conference?.getConnectionState() === "connected";
  });
}

async function completeLogin(page: Page): Promise<void> {
  await expect(page).toHaveURL(new RegExp(`^${idpOrigin.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")}/authorize`));
  await page.getByLabel("Login").fill(adminLogin);
  await page.getByLabel("Password").fill(
    localSecret("TINYIDP_TEST_ADMIN_PASSWORD", "local-admin-password.txt")
  );
  await page.getByRole("button", { name: "Approve" }).click();
}

async function loginAs(page: Page, login: string, password: string): Promise<void> {
  await expect(page).toHaveURL(new RegExp(`^${idpOrigin.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")}/authorize`));
  await page.getByLabel("Login").fill(login);
  await page.getByLabel("Password").fill(password);
  await page.getByRole("button", { name: "Approve" }).click();
}

async function clickPrejoin(page: Page, name: string): Promise<void> {
  const nameField = page.getByRole("textbox", { name: "Enter your name" });
  if (await nameField.isVisible()) {
    await nameField.fill(name);
  }
  await page.getByTestId("prejoin.joinMeeting").click();
}

async function authenticateAndJoin(context: BrowserContext, room: string, name: string): Promise<Page> {
  const page = await context.newPage();
  await page.goto(`${meetOrigin}/${room}`);
  await clickPrejoin(page, name);
  await completeLogin(page);
  await expect(page).toHaveURL(new RegExp(`^${meetOrigin.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")}/${room}\\?jwt=`));
  await clickPrejoin(page, name);
  await expect.poll(() => conferenceJoined(page), { timeout: 30_000 }).toBe(true);
  return page;
}

async function newMediaContext(browser: Browser): Promise<BrowserContext> {
  const context = await browser.newContext({
    ignoreHTTPSErrors: true,
    permissions: ["camera", "microphone"]
  });
  return context;
}

test("Prosody rejects an empty token and Jitsi starts TinyIDP login", async ({ page }) => {
  const room = `login-${Date.now()}`;
  await page.goto(`${meetOrigin}/${room}`);
  await clickPrejoin(page, "Token Required");
  await expect(page).toHaveURL(new RegExp(`^${idpOrigin.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")}/authorize`));
  await expect(page.getByRole("heading", { name: "Sign in and approve access" })).toBeVisible();
});

test("canceling TinyIDP login returns a themed recoverable plugin error", async ({ page }) => {
  const room = `cancel-${Date.now()}`;
  await page.goto(`${meetOrigin}/${room}`);
  await clickPrejoin(page, "Canceled Login");
  await expect(page).toHaveURL(new RegExp(`^${idpOrigin.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")}/authorize`));
  await page.getByRole("button", { name: "Deny" }).click();
  await expect(page.getByRole("heading", { name: "Meeting access was not completed" })).toBeVisible();
  await expect(page.getByRole("alert")).toContainText("Authentication was canceled");
  await expect(page.locator('link[rel="stylesheet"]')).toHaveAttribute("href", "/static/themes/jitsi.css");
});

test("the Goja policy denies an identity without an email claim", async ({ page }) => {
  const room = `policy-denied-${Date.now()}`;
  await page.goto(`${meetOrigin}/${room}`);
  await clickPrejoin(page, "Policy Denied");
  await loginAs(
    page,
    "denied@example.test",
    localSecret("TINYIDP_TEST_POLICY_DENIED_PASSWORD", "local-policy-denied-password.txt")
  );
  await expect(page.getByRole("heading", { name: "Meeting access was not completed" })).toBeVisible();
  await expect(page.getByRole("alert")).toContainText("verified email address is required");
});

test("a malformed JWT cannot enter a Prosody conference", async ({ page }) => {
  const room = `bad-token-${Date.now()}`;
  await page.goto(`${meetOrigin}/${room}?jwt=not-a-jwt`);
  await clickPrejoin(page, "Bad Token");
  await expect(page.getByText("Authentication failed", { exact: true })).toBeVisible();
  await expect(page.getByText("Sorry, you're not allowed to join this call.")).toBeVisible();
  expect(await conferenceJoined(page)).toBe(false);
});

test("explicit signup creates an identity and returns a room-bound token", async ({ page }) => {
  const suffix = Date.now();
  const room = `signup-${suffix}`;
  await page.goto(`${idpOrigin}/integrations/jitsi/start?room=${room}&intent=signup`);
  await expect(page.getByRole("heading", { name: "Create an account" })).toBeVisible();
  await page.getByLabel("Display name").fill(`Jitsi Signup ${suffix}`);
  await page.getByLabel("Email").fill(`jitsi-signup-${suffix}@example.test`);
  await page.getByLabel("Password", { exact: true }).fill("jitsi signup password 2026!");
  await page.getByLabel("Confirm password").fill("jitsi signup password 2026!");
  await page.getByRole("button", { name: "Create account" }).click();
  if (page.url().startsWith(idpOrigin)) {
    const approve = page.getByRole("button", { name: "Approve" });
    if (await approve.isVisible()) {
      await approve.click();
    }
  }
  await expect(page).toHaveURL(new RegExp(`^${meetOrigin.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")}/${room}\\?jwt=`));
});

test("a remembered session reaches the account chooser", async ({ page }) => {
  const firstRoom = `remember-${Date.now()}`;
  await page.goto(`${meetOrigin}/${firstRoom}`);
  await clickPrejoin(page, "Remembered Administrator");
  await completeLogin(page);
  await expect(page).toHaveURL(new RegExp(`^${meetOrigin.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")}/${firstRoom}\\?jwt=`));

  await page.goto(`${idpOrigin}/integrations/jitsi/start?room=chooser-room&prompt=select_account`);
  await expect(page.getByRole("heading", { name: "Choose an account" })).toBeVisible();
  await expect(page.getByLabel("Local Jitsi Administrator")).toBeVisible();
});

test("provider logout clears the TinyIDP session before a new meeting login", async ({ page }) => {
  const firstRoom = `logout-seed-${Date.now()}`;
  await page.goto(`${meetOrigin}/${firstRoom}`);
  await clickPrejoin(page, "Logout Administrator");
  await completeLogin(page);
  await expect(page).toHaveURL(new RegExp(`^${meetOrigin.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")}/${firstRoom}\\?jwt=`));

  const postLogout = encodeURIComponent(`${meetOrigin}/`);
  await page.goto(`${idpOrigin}/end-session?client_id=tinyidp-jitsi-local&post_logout_redirect_uri=${postLogout}`);
  await expect(page).toHaveURL(`${meetOrigin}/`);

  const secondRoom = `logout-proof-${Date.now()}`;
  await page.goto(`${meetOrigin}/${secondRoom}`);
  await clickPrejoin(page, "Logout Proof");
  await expect(page).toHaveURL(new RegExp(`^${idpOrigin.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")}/authorize`));
  await expect(page.getByLabel("Login")).toBeVisible();
  await expect(page.getByLabel("Password")).toBeVisible();
});

test("two independently authenticated browsers join the same JVB conference", async ({ browser }) => {
  const room = `two-browser-${Date.now()}`;
  const firstContext = await newMediaContext(browser);
  const secondContext = await newMediaContext(browser);
  try {
    const first = await authenticateAndJoin(firstContext, room, "First Browser");
    const second = await authenticateAndJoin(secondContext, room, "Second Browser");
    await expect.poll(() => participantCount(first), { timeout: 30_000 }).toBeGreaterThanOrEqual(2);
    await expect.poll(() => participantCount(second), { timeout: 30_000 }).toBeGreaterThanOrEqual(2);
    await expect.poll(() => mediaConnected(first), { timeout: 30_000 }).toBe(true);
    await expect.poll(() => mediaConnected(second), { timeout: 30_000 }).toBe(true);
  } finally {
    await firstContext.close();
    await secondContext.close();
  }
});
