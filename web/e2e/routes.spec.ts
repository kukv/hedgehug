// Screens this spec relies on:
//   - "/" is the overview of the first workspace: the heading (level 1) is the workspace name, and
//     the navigation "ビューの切り替え" holds the links 概要, ツリー and 設定; the current one has
//     aria-current="page". The tabs are links (the address, back and reload keep the tab)
//   - the sidebar ("メインナビゲーション") has the link "設定" (the gear, /settings)
//   - the settings take the window: the navigation "設定のナビゲーション" (with the link
//     "Hedgehug に戻る" and the version) is shown in place of the sidebar
import type { Page } from "@playwright/test";
import { expect, test } from "./harness";
import { row } from "./screens";

const tabs = (page: Page) => page.getByRole("navigation", { name: "ビューの切り替え" });
const sidebar = (page: Page) => page.getByRole("navigation", { name: "メインナビゲーション" });
const heading = (page: Page) => page.getByRole("heading", { level: 1, name: "demo" });

test("the tabs of a workspace are links: back, forward and reload keep the tab", async ({
  page,
  hh,
}) => {
  hh.add("Alpha");
  await hh.open(page);
  await expect(heading(page)).toBeVisible();
  await expect(tabs(page).getByRole("link", { name: "概要" })).toHaveAttribute(
    "aria-current",
    "page",
  );

  await tabs(page).getByRole("link", { name: "ツリー" }).click();
  await expect(page).toHaveURL(/\/workspaces\/demo\/[^/]+\/tree$/);
  await expect(row(page, "Alpha")).toBeVisible();
  // The heading stays above the tree.
  await expect(heading(page)).toBeVisible();
  await expect(tabs(page).getByRole("link", { name: "ツリー" })).toHaveAttribute(
    "aria-current",
    "page",
  );

  await page.goBack();
  await expect(page).toHaveURL(`${hh.origin}/`);
  await expect(tabs(page).getByRole("link", { name: "概要" })).toHaveAttribute(
    "aria-current",
    "page",
  );

  await page.goForward();
  await expect(row(page, "Alpha")).toBeVisible();
  await page.reload();
  await expect(row(page, "Alpha")).toBeVisible();
  await expect(tabs(page).getByRole("link", { name: "ツリー" })).toHaveAttribute(
    "aria-current",
    "page",
  );
});

test("the settings tab is the right end of the tabs", async ({ page, hh }) => {
  await hh.open(page);
  await tabs(page).getByRole("link", { name: "設定" }).click();
  await expect(page).toHaveURL(/\/workspaces\/demo\/[^/]+\/settings$/);
  await expect(tabs(page).getByRole("link", { name: "設定" })).toHaveAttribute(
    "aria-current",
    "page",
  );
  await expect(heading(page)).toBeVisible();
});

test("/settings goes to the workspaces of the first store, with its own nav in place of the sidebar", async ({
  page,
  hh,
}) => {
  await hh.open(page, "/settings");
  await expect(page).toHaveURL(/\/settings\/workspaces\/demo$/);
  const nav = page.getByRole("navigation", { name: "設定のナビゲーション" });
  await expect(nav).toBeVisible();
  await expect(nav.getByRole("link", { name: /ワークスペース/ })).toHaveAttribute(
    "aria-current",
    "page",
  );
  await expect(nav.getByText(/^Hedgehug \S+$/)).toBeVisible();
  await expect(sidebar(page)).toBeHidden();

  await nav.getByRole("link", { name: /Hedgehug に戻る/ }).click();
  await expect(sidebar(page)).toBeVisible();
  await expect(heading(page)).toBeVisible();
});

test("the gear of the sidebar opens the settings", async ({ page, hh }) => {
  await hh.open(page);
  await sidebar(page).getByRole("link", { name: "設定", exact: true }).click();
  await expect(page).toHaveURL(/\/settings\/workspaces\/demo$/);
  await expect(page.getByRole("navigation", { name: "設定のナビゲーション" })).toBeVisible();
});
