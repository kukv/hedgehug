import { defineConfig, devices } from "@playwright/test";

// e2e 用にビルドしたサーバー(bin/hedgehug-e2e。口に apptest の偽物とサンプルデータを差し込んだもの)を、
// 決めた番号とトークンで起動する。幅は、スマホ・タブレット・デスクトップの 3 つ(設計書 6 節)。
const addr = "127.0.0.1:4173";

export default defineConfig({
  testDir: "e2e",
  testMatch: "**/*.spec.ts",
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? [["github"], ["list"], ["html", { open: "never" }]] : "list",
  use: {
    baseURL: `http://${addr}`,
    trace: "retain-on-failure",
    // Playwright は、言わなければ Chromium のサンドボックスを切る。
    launchOptions: { chromiumSandbox: true },
  },
  webServer: {
    command: "../bin/hedgehug-e2e",
    url: `http://${addr}/`,
    env: { HEDGEHUG_ADDR: addr, HEDGEHUG_TOKEN: "e2e-token" },
    reuseExistingServer: !process.env.CI,
  },
  projects: [
    { name: "phone", use: { ...devices["Desktop Chrome"], viewport: { width: 390, height: 844 } } },
    {
      name: "tablet",
      use: { ...devices["Desktop Chrome"], viewport: { width: 820, height: 1180 } },
    },
    {
      name: "desktop",
      use: { ...devices["Desktop Chrome"], viewport: { width: 1280, height: 800 } },
    },
  ],
});
