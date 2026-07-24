import AxeBuilder from "@axe-core/playwright";
import { expect, test, type Page } from "@playwright/test";

const screens = [
  "overview", "users", "invitations", "clients", "keys", "activity", "operations",
] as const;
const fixtureStates = [
  "loading", "empty", "no-results", "forbidden", "stale",
  "session-expired", "audit-degraded", "operation-failed",
] as const;

const session = {
  subject: "owner-subject",
  grant_id: "owner-grant",
  grant_version: 1,
  csrf_token: "csrf-test-token",
  expires_at: "2026-07-24T18:00:00Z",
};

function widgetPage(pageID: string, message = "Ready") {
  return {
    schemaVersion: "0.1.0",
    id: pageID,
    title: title(pageID),
    root: {
      kind: "component",
      type: "Stack",
      props: { gap: "lg" },
      children: [{
        kind: "component",
        type: "SectionBlock",
        props: { density: "flush", label: title(pageID), level: 1, rule: true },
        children: [{ kind: "text", text: message }],
      }],
    },
  };
}

function title(pageID: string) {
  return pageID.replace(/(^|-)([a-z])/g, (_, prefix, letter) =>
    `${prefix ? " " : ""}${letter.toUpperCase()}`,
  );
}

async function mockConsole(
  page: Page,
  options: {
    sessionStatus?: number;
    sessionDelay?: number;
    widgetStatus?: number;
    widgetDelay?: number;
    message?: string;
    executeStatus?: number;
  } = {},
) {
  await page.route("**/admin/**", async (route) => {
    if (route.request().resourceType() !== "document") {
      await route.fallback();
      return;
    }
    const response = await route.fetch({
      url: "http://127.0.0.1:4174/static/admin/",
    });
    await route.fulfill({ response });
  });
  await page.route("**/api/admin/session", async (route) => {
    if (options.sessionDelay) {
      await new Promise((resolve) => setTimeout(resolve, options.sessionDelay));
    }
    await route.fulfill({
      status: options.sessionStatus ?? 200,
      contentType: "application/json",
      body: options.sessionStatus && options.sessionStatus !== 200
        ? JSON.stringify({ error: "authentication_required" })
        : JSON.stringify(session),
    });
  });
  await page.route("**/api/widget/pages/**", async (route) => {
    if (options.widgetDelay) {
      await new Promise((resolve) => setTimeout(resolve, options.widgetDelay));
    }
    const pageID = new URL(route.request().url()).pathname.split("/").pop() ?? "overview";
    await route.fulfill({
      status: options.widgetStatus ?? 200,
      contentType: "application/json",
      body: options.widgetStatus && options.widgetStatus !== 200
        ? JSON.stringify({ error: "administration_denied" })
        : JSON.stringify(widgetPage(pageID, options.message)),
    });
  });
  await page.route("**/api/admin/pages/**", (route) => route.fulfill({
    status: 200,
    contentType: "application/json",
    body: JSON.stringify({
      id: "operations",
      title: "Operations",
      items: [
        { id: "backup-1", primary: "backup", secondary: "", status: "completed" },
        { id: "diagnostics-1", primary: "diagnostics", secondary: "", status: "completed" },
        { id: "failed-1", primary: "doctor", secondary: "doctor_failed", status: "failed" },
      ],
    }),
  }));
  await page.route("**/api/widget/actions/prepare", (route) => route.fulfill({
    status: 200,
    contentType: "application/json",
    body: JSON.stringify({
      action_handle: "signed-handle",
      command: "operations.doctor",
      require_fresh: false,
      require_reason: false,
      expires_at: "2026-07-24T18:00:00Z",
    }),
  }));
  await page.route("**/api/widget/actions/execute", (route) => route.fulfill({
    status: options.executeStatus ?? 200,
    contentType: "application/json",
    body: options.executeStatus === 409
      ? JSON.stringify({ error: "version_conflict" })
      : JSON.stringify({ operation: { id: "new-operation", status: "pending" } }),
  }));
  await page.route("**/api/admin/operations/**/downloads", (route) => route.fulfill({
    status: 201,
    contentType: "application/json",
    body: JSON.stringify({
      download_handle: "one-use-handle",
      download_name: "diagnostics.json",
      expires_at: "2026-07-24T18:00:00Z",
    }),
  }));
}

test("renders every required fixture state on every screen", async ({ page }) => {
  test.setTimeout(90_000);
  for (const screen of screens) {
    for (const state of fixtureStates) {
      await page.unrouteAll({ behavior: "wait" });
      const options = state === "loading"
        ? { widgetDelay: 100 }
        : state === "forbidden"
          ? { widgetStatus: 403 }
          : state === "session-expired"
            ? { sessionStatus: 401 }
            : { message: fixtureMessage(state) };
      await mockConsole(page, options);
      await page.goto(`/admin/${screen}`);
      if (state === "loading") {
        await expect(page.getByText("Loading page…")).toBeVisible();
        await expect(page.getByText("Ready")).toBeVisible();
      } else if (state === "forbidden") {
        await expect(page.getByText(/could not be loaded/)).toBeVisible();
      } else if (state === "session-expired") {
        await expect(page.getByRole("link", { name: "Sign in" })).toBeVisible();
      } else {
        await expect(page.getByText(fixtureMessage(state))).toBeVisible();
      }
    }
  }
});

function fixtureMessage(state: typeof fixtureStates[number]) {
  switch (state) {
    case "empty": return "No records are available.";
    case "no-results": return "No records match these filters.";
    case "stale": return "This page is stale; reload before changing data.";
    case "audit-degraded": return "Audit delivery is degraded.";
    case "operation-failed": return "The operation failed safely.";
    default: return "Ready";
  }
}

test("keyboard navigation, landmarks, labels, and axe scan pass", async ({ page }) => {
  await mockConsole(page);
  await page.goto("/admin/operations");
  await expect(page.getByRole("navigation", { name: "Administration" })).toBeVisible();
  await expect(page.getByRole("main")).toBeVisible();
  await expect(page.getByRole("heading", { name: "Managed operations" })).toBeVisible();
  await page.keyboard.press("Tab");
  await expect(page.getByRole("link", { name: "Skip to content" })).toBeFocused();
  const results = await new AxeBuilder({ page }).analyze();
  expect(results.violations).toEqual([]);
});

test("the reviewed CSP blocks inline and external scripts in a real browser", async ({ page }) => {
  await mockConsole(page);
  const response = await page.goto("/admin/overview");
  const csp = response?.headers()["content-security-policy"] ?? "";
  expect(csp).toContain("script-src 'self'");
  expect(csp).toContain("object-src 'none'");
  expect(csp).not.toContain("'unsafe-inline'");
  expect(csp).not.toContain("'unsafe-eval'");

  await expect(page.addScriptTag({
    content: "window.__tinyidpInlineScriptExecuted = true",
  })).rejects.toThrow();
  expect(await page.evaluate(() =>
    (window as typeof window & { __tinyidpInlineScriptExecuted?: boolean })
      .__tinyidpInlineScriptExecuted,
  )).toBeUndefined();

  let externalError = "";
  try {
    await page.addScriptTag({
      url: "https://example.invalid/tinyidp-csp-test.js",
    });
  } catch (error) {
    externalError = String(error);
  }
  expect(externalError).toMatch(/Content Security Policy|script-src/);
});

test("tablet layout and reduced motion have a stable visual snapshot", async ({ page }) => {
  await page.setViewportSize({ width: 820, height: 1180 });
  await mockConsole(page, { message: "Audit delivery is degraded" });
  await page.goto("/admin/operations");
  await expect(page.locator(".admin-nav")).toBeVisible();
  const transition = await page.locator(".admin-shell").evaluate((element) =>
    getComputedStyle(element).transitionDuration,
  );
  expect(["0s", "0.01ms"]).toContain(transition);
  await expect(page).toHaveScreenshot("operations-tablet.png", {
    animations: "disabled",
    fullPage: true,
  });
});

test("small screens become read-only instead of exposing mutation forms", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await mockConsole(page);
  await page.goto("/admin/keys");
  await expect(page.locator(".small-screen-warning")).toBeVisible();
  await expect(page.locator(".admin-mutation-surface")).toBeHidden();
});

test("loading, forbidden, expired-session, stale, audit, and failed-operation states render", async ({ page }) => {
  await mockConsole(page, { sessionDelay: 250 });
  await page.goto("/admin/overview");
  await expect(page.getByText("Loading TinyIDP Console…")).toBeVisible();
  await expect(page.getByText("Ready")).toBeVisible();

  await page.unrouteAll({ behavior: "wait" });
  await mockConsole(page, { widgetStatus: 403 });
  await page.goto("/admin/users");
  await expect(page.getByText(/could not be loaded/)).toBeVisible();

  await page.unrouteAll({ behavior: "wait" });
  await mockConsole(page, { sessionStatus: 401 });
  await page.goto("/admin/invitations");
  await expect(page.getByRole("link", { name: "Sign in" })).toBeVisible();

  await page.unrouteAll({ behavior: "wait" });
  await mockConsole(page, { executeStatus: 409 });
  await page.goto("/admin/operations");
  await page.getByRole("button", { name: "Run doctor" }).click();
  await expect(page.getByText("operations.doctor failed.")).toBeVisible();

  await page.unrouteAll({ behavior: "wait" });
  await mockConsole(page, { message: "Audit delivery pending; doctor_failed" });
  await page.goto("/admin/operations");
  await expect(page.getByText("Audit delivery pending; doctor_failed")).toBeVisible();
  await expect(page.getByText("doctor_failed")).toBeVisible();
});
