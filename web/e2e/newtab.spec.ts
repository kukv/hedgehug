// Screens this spec relies on (see screens.ts for the shared list):
//   - the sidebar lists the workspace as a link named by it; Ctrl+click on it opens the
//     workspace in a new tab that keeps the token (the click calls window.open without
//     noopener, so the browser copies sessionStorage to the tab)
//   - a tab with no token (a fresh browser context) shows "この画面は hh ui が開いた URL
//     からだけ使えます"
import { expect, test } from "./harness";

test("Ctrl+click on a sidebar link opens a tab that keeps the token", async ({
  page,
  hh,
  context,
}) => {
  // The click calls window.open without noopener (app/router.ts), so the browser opens the
  // tab as a copy of this one and copies sessionStorage to it: the token goes with it.
  hh.add("Anything");
  await hh.open(page);
  const link = page
    .getByRole("navigation", { name: "メインナビゲーション" })
    .getByRole("link", { name: /demo/ });
  await expect(link).toBeVisible();
  const [tab] = await Promise.all([
    context.waitForEvent("page"),
    link.click({ modifiers: ["Control"] }),
  ]);
  await expect(tab).toHaveURL(/\/workspaces\/demo\//);
  await expect(tab.getByRole("tree", { name: "demo のタスク" })).toBeVisible();
  await expect(tab.getByText("この画面は hh ui が開いた URL からだけ使えます")).toHaveCount(0);
});

test("a fresh browser context has no token", async ({ browser, hh }) => {
  // The token lives in sessionStorage of the tab hh ui opened: a new browser context has
  // none, so the same URL there shows the screen that says to start from hh ui. (This does
  // not check a tab opened by a plain link; Chromium's own behaviour is not ours to test.)
  const context = await browser.newContext();
  const fresh = await context.newPage();
  await fresh.goto(`${hh.origin}/`);
  await expect(fresh.getByText("この画面は hh ui が開いた URL からだけ使えます")).toBeVisible();
  await context.close();
});
