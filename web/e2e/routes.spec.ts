// Screens this spec relies on:
//   - "/" is the overview of the first workspace: the heading (level 1) is the workspace name, and
//     the navigation "ビューの切り替え" holds the links 概要, ツリー, カンバン, テーブル and 設定; the
//     current one has aria-current="page". The tabs are links (the address, back and reload keep the
//     tab). A project has the same navigation with the links ツリー, カンバン and テーブル
//   - the sidebar ("メインナビゲーション") has the link "設定" (the gear, /settings) and the link
//     "マイタスク" (/my); it has no entry for the notifications and the activity yet
//   - for each workspace the sidebar has the button "<名前> の操作（設定を開く・削除）" that opens
//     the menu "ワークスペースの操作" with 設定を開く and 削除…
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

test("the … of a workspace row opens its settings", async ({ page, hh }) => {
  await hh.open(page);
  await sidebar(page).getByRole("button", { name: "demo の操作（設定を開く・削除）" }).click();
  const menu = page.getByRole("menu", { name: "ワークスペースの操作" });
  await expect(menu.getByRole("menuitem")).toHaveText(["設定を開く", "削除…"]);
  await menu.getByRole("menuitem", { name: "設定を開く" }).click();
  await expect(page).toHaveURL(/\/workspaces\/demo\/[^/]+\/settings$/);
  await expect(menu).toHaveCount(0);
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
  await expect(tabs(page).getByRole("link")).toHaveText(["ツリー", "カンバン", "テーブル"]);
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

test("the sidebar has the tasks of the person and none of the screens of 4e-4", async ({
  page,
  hh,
}) => {
  await hh.open(page);
  await expect(sidebar(page).getByRole("link", { name: /マイタスク/ })).toHaveAttribute(
    "href",
    "/my",
  );
  await expect(sidebar(page).getByText("通知")).toHaveCount(0);
  await expect(sidebar(page).getByText("アクティビティ")).toHaveCount(0);
});
