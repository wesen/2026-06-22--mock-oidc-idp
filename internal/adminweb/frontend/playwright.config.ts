import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: "./tests",
  fullyParallel: false,
  forbidOnly: true,
  retries: 0,
  reporter: "line",
  use: {
    baseURL: "http://127.0.0.1:4174",
    browserName: "chromium",
    headless: true,
    reducedMotion: "reduce",
    launchOptions: {
      executablePath: "/usr/bin/chromium-browser",
    },
    screenshot: "only-on-failure",
    trace: "retain-on-failure",
  },
  webServer: {
    command: "pnpm run build && pnpm exec vite preview --host 127.0.0.1 --port 4174",
    url: "http://127.0.0.1:4174/static/admin/",
    reuseExistingServer: false,
  },
});
