// Screens this spec relies on:
//   - "/" is the overview of the first workspace: the heading (level 1) is the workspace name, and
//     the navigation "ビューの切り替え" holds the links 概要, ツリー, カンバン, テーブル and 設定; the
//     current one has aria-current="page". The tabs are links (the address, back and reload keep the
//     tab). A project has the same navigation with the links ツリー, カンバン, テーブル and 設定
//   - the sidebar ("メインナビゲーション") has the link "設定" (the gear, /settings/general) and the link
//     "マイタスク" (/my); it has no entry for the notifications and the activity yet
//   - for each workspace the sidebar has the button "<名前> の操作（設定を開く・削除）" that opens
//     the menu "ワークスペースの操作" with 設定を開く and 削除…
//   - the settings take the window: the navigation "設定のナビゲーション" (with the link
//     "Hedgehug に戻る" and the version) is shown in place of the sidebar; it has the groups "このマシン"
//     (the link 同期と保守) and "Store ごとの設定" (the link ワークスペース)
//   - /settings/general has the heading 同期と保守, the region "同期のタイミング" and, for each Store,
//     the region "<Store>" under 同期の状態 with the button 今すぐ同期
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

test("the … of a workspace row opens its settings", async ({ page, hh }) => {
  await hh.open(page);
  await sidebar(page).getByRole("button", { name: "demo の操作（設定を開く・削除）" }).click();
  const menu = page.getByRole("menu", { name: "ワークスペースの操作" });
  await expect(menu.getByRole("menuitem")).toHaveText(["設定を開く", "削除…"]);
  await menu.getByRole("menuitem", { name: "設定を開く" }).click();
  await expect(page).toHaveURL(/\/workspaces\/demo\/[^/]+\/settings$/);
  await expect(menu).toHaveCount(0);
});

test("/settings is the sync and upkeep page, with its own nav in place of the sidebar", async ({
  page,
  hh,
}) => {
  await hh.open(page, "/settings");
  await expect(page).toHaveURL(/\/settings\/general$/);
  // /settings was replaced, not pushed: back leaves the settings instead of landing on it again
  await page.goBack();
  await expect(page).not.toHaveURL(/\/settings/);
  await page.goForward();
  await expect(page).toHaveURL(/\/settings\/general$/);
  const nav = page.getByRole("navigation", { name: "設定のナビゲーション" });
  await expect(nav).toBeVisible();
  await expect(
    nav.getByRole("region", { name: "このマシン" }).getByRole("link", { name: /同期と保守/ }),
  ).toHaveAttribute("aria-current", "page");
  await expect(nav.getByText(/^Hedgehug \S+$/)).toBeVisible();
  await expect(sidebar(page)).toBeHidden();

  const body = page.getByRole("region", { name: "設定の本文" });
  await expect(body.getByRole("heading", { level: 1, name: "同期と保守" })).toBeVisible();
  const timing = body.getByRole("region", { name: "同期のタイミング" });
  await expect(timing).toContainText("書き込みの");
  await expect(timing).toContainText("非表示の間は 10 分ごと");
  await expect(body.getByRole("region", { name: "Store: demo" })).toBeVisible();

  await nav
    .getByRole("region", { name: "Store ごとの設定" })
    .getByRole("link", { name: /ワークスペース/ })
    .click();
  await expect(page).toHaveURL(/\/settings\/workspaces\/demo$/);

  await nav.getByRole("link", { name: /Hedgehug に戻る/ }).click();
  await expect(sidebar(page)).toBeVisible();
  await expect(heading(page)).toBeVisible();
});

test("the gear of the sidebar opens the sync and upkeep settings", async ({ page, hh }) => {
  await hh.open(page);
  await sidebar(page).getByRole("link", { name: "設定", exact: true }).click();
  await expect(page).toHaveURL(/\/settings\/general$/);
  await expect(page.getByRole("navigation", { name: "設定のナビゲーション" })).toBeVisible();
});

test("the kanban and the table are tabs of a workspace and of a project", async ({ page, hh }) => {
  const project = hh.add("Release", "--kind", "project");
  const child = hh.add("Ship it", "--parent", project.id);
  const board = page.locator('section[aria-label="カンバンの盤面"]');
  await hh.open(page);

  await expect(tabs(page).getByRole("link")).toHaveText([
    "概要",
    "ツリー",
    "カンバン",
    "テーブル",
    "設定",
  ]);
  await tabs(page).getByRole("link", { name: "カンバン" }).click();
  await expect(page).toHaveURL(/\/workspaces\/demo\/[^/]+\/kanban$/);
  await expect(board).toBeVisible();
  await expect(heading(page)).toBeVisible();
  await tabs(page).getByRole("link", { name: "テーブル" }).click();
  await expect(page).toHaveURL(/\/workspaces\/demo\/[^/]+\/table$/);
  await expect(page.getByRole("textbox", { name: `タイトル: ${child.id}` })).toBeVisible();

  await hh.open(page, `/projects/${project.ulid}`);
  await expect(page.getByRole("heading", { level: 1, name: "Release" })).toBeVisible();
  await expect(tabs(page).getByRole("link")).toHaveText(["ツリー", "カンバン", "テーブル", "設定"]);
  await expect(row(page, "Ship it")).toBeVisible();
  await tabs(page).getByRole("link", { name: "カンバン" }).click();
  await expect(page).toHaveURL(new RegExp(`/projects/${project.ulid}/kanban$`));
  await expect(board.locator(`li[data-ulid="${child.ulid}"]`)).toBeVisible();
  await tabs(page).getByRole("link", { name: "テーブル" }).click();
  await expect(page).toHaveURL(new RegExp(`/projects/${project.ulid}/table$`));
  await expect(page.getByRole("textbox", { name: `タイトル: ${child.id}` })).toBeVisible();
  await tabs(page).getByRole("link", { name: "ツリー" }).click();
  await expect(page).toHaveURL(new RegExp(`/projects/${project.ulid}$`));
  await expect(row(page, "Ship it")).toBeVisible();
});

test("the sidebar has the tasks of the person and the inbox and the activity", async ({
  page,
  hh,
}) => {
  await hh.open(page);
  await expect(sidebar(page).getByRole("link", { name: /マイタスク/ })).toHaveAttribute(
    "href",
    "/my",
  );
  await expect(sidebar(page).getByRole("link", { name: /通知/ })).toHaveAttribute("href", "/inbox");
  await expect(sidebar(page).getByRole("link", { name: "アクティビティ" })).toHaveAttribute(
    "href",
    "/activity",
  );
});
