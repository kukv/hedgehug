import type { Page, TestInfo } from "@playwright/test";
import { expect, test } from "./fixtures";
import { prefixOf, resetSearchScreen, sidebar, tokenUrl } from "./helpers";

// 検索のテーブルの通しのテスト(E2a。04「テーブル」)。作ったアイテムのタイトルにテストごとの印を入れて絞り込み、
// サンプルや同じサーバーを使うほかのテストの件数に頼らない。

const e2eStore = "01E2ESTORE0000000000000001";
const auth = { Authorization: "Bearer e2e-token" };
const base = `/api/stores/${e2eStore}`;

async function send(page: Page, method: "post" | "put", path: string, data: unknown) {
  const res = await page.request[method](path, { headers: auth, data });
  if (!res.ok()) throw new Error(`${path}: ${res.status()} ${await res.text()}`);
  return res.status() === 204 ? undefined : res.json();
}

// seed は、新しいワークスペースに、Epic「{mark} A」とその下の Story「{mark} B」を作る。
async function seed(page: Page, info: TestInfo) {
  const prefix = prefixOf(info);
  const mark = `表試験${prefix}`;
  const ws = await send(page, "post", `${base}/workspaces`, { name: `表 ${prefix}`, prefix });
  const project = await send(page, "post", `${base}/workspaces/${ws.id}/projects`, {
    title: `表の計画${prefix}`,
  });
  const epic = await send(page, "post", `${base}/items`, {
    project: project.id,
    kind: "Epic",
    title: `${mark} A`,
  });
  await send(page, "post", `${base}/items`, {
    project: project.id,
    kind: "Story",
    parent: epic.id,
    title: `${mark} B`,
  });
  return { mark, workspaceId: ws.id as string };
}

// openWith は、テーブルの設定を初めの値に戻して検索の画面を開き、絞り込みを mark にして、その検索の応答を待つ。
async function openWith(page: Page, info: TestInfo, mark: string) {
  await resetSearchScreen(page.request);
  await page.goto(tokenUrl());
  const side = await sidebar(page, info);
  const loaded = page.waitForResponse(
    (r) => r.url().includes("/api/screen-states/search") && r.request().method() === "GET",
  );
  await side.getByRole("link", { name: "検索", exact: true }).click();
  await loaded;
  await type(page, mark);
}

async function type(page: Page, query: string) {
  const done = page.waitForResponse(
    (r) => r.url().endsWith("/api/search") && r.request().postDataJSON()?.query === query,
  );
  await page.getByRole("combobox", { name: "絞り込み" }).fill(query);
  await done;
}

const headers = (page: Page) => page.getByRole("main").getByRole("columnheader");
const rowOf = (page: Page, title: string) => page.getByRole("row", { name: new RegExp(title) });

test("shows a column chosen in the menu at the right end, and keeps it after a reload", async ({
  page,
}, info) => {
  const { mark } = await seed(page, info);
  await openWith(page, info, mark);
  await page.getByRole("button", { name: "表示する列", exact: true }).click();
  const menu = page.getByRole("menu", { name: "表示する列" });
  await expect(menu.getByRole("menuitemcheckbox", { name: "タイトル" })).toBeDisabled();
  const saved = page.waitForResponse(
    (r) =>
      r.url().includes("/api/screen-states/search") && r.request().method() === "PUT" && r.ok(),
  );
  await menu.getByRole("menuitemcheckbox", { name: "開始日" }).click();
  await saved;
  await expect(headers(page).last()).toHaveText("開始日");
  await expect(menu).toBeVisible();
  await expect(menu.getByRole("menuitemcheckbox", { name: "開始日" })).toHaveAttribute(
    "aria-checked",
    "true",
  );

  await page.reload();
  await expect(headers(page).last()).toHaveText("開始日");
});

test("sorts by the title header: ascending, descending, then none", async ({ page }, info) => {
  const { mark } = await seed(page, info);
  await openWith(page, info, mark);
  const title = headers(page).filter({ hasText: "タイトル" });
  const rows = rowOf(page, mark);

  await title.getByRole("button").click();
  await expect(title).toHaveAttribute("aria-sort", "ascending");
  await expect(title).toContainText("↑");
  await expect(rows.first()).toContainText(`${mark} A`);

  await title.getByRole("button").click();
  await expect(title).toHaveAttribute("aria-sort", "descending");
  await expect(title).toContainText("↓");
  await expect(rows.first()).toContainText(`${mark} B`);

  await title.getByRole("button").click();
  await expect(title).toHaveAttribute("aria-sort", "none");
  await expect(title).not.toContainText(/[↑↓]/);
});

test("groups by the group setting, or by group: when the setting is none", async ({
  page,
}, info) => {
  const { mark } = await seed(page, info);
  await openWith(page, info, mark);
  const group = page.getByRole("combobox", { name: "グループ" });
  const epicHead = page.getByRole("row", { name: "Epic 1", exact: true });
  const storyHead = page.getByRole("row", { name: "Story 1", exact: true });

  await group.selectOption({ label: "種類" });
  await expect(epicHead).toBeVisible();
  await expect(storyHead).toBeVisible();

  await group.selectOption({ label: "なし" });
  await expect(epicHead).toBeHidden();
  await type(page, `group:kind ${mark}`);
  await expect(epicHead).toBeVisible();
  await expect(storyHead).toBeVisible();

  // 設定が「なし」でなければ、`group:` より設定を使う。
  await group.selectOption({ label: "担当者" });
  await expect(page.getByRole("row", { name: "未割り当て 2", exact: true })).toBeVisible();
  await expect(epicHead).toBeHidden();
  // ツールバーの件数は、グループ分けをしても行の数のまま。
  await expect(page.getByText("2 件", { exact: true })).toBeVisible();
});

test("leaves the systems cell empty, and shows — for a property the kind does not use", async ({
  page,
}, info) => {
  const { mark, workspaceId } = await seed(page, info);
  // Epic では優先度を使わない設定にする(初めの設定ではすべての種類が使う)。
  await send(page, "put", `${base}/workspaces/${workspaceId}/properties/priority/kinds`, {
    kinds: ["Story", "Task", "Bug", "Step"],
  });
  await openWith(page, info, mark);
  await page.getByRole("button", { name: "表示する列", exact: true }).click();
  await page
    .getByRole("menu", { name: "表示する列" })
    .getByRole("menuitemcheckbox", { name: "関連システム" })
    .click();
  await page.keyboard.press("Escape");
  await expect(headers(page).last()).toHaveText("関連システム");

  const names = await headers(page).allTextContents();
  const systems = names.findIndex((n) => n.trim() === "関連システム");
  const priority = names.findIndex((n) => n.trim() === "優先度");
  const epic = rowOf(page, `${mark} A`);
  const story = rowOf(page, `${mark} B`);
  await expect(epic.getByRole("cell").nth(systems)).toHaveText("");
  await expect(epic.getByRole("cell").nth(priority)).toHaveText("—");
  await expect(story.getByRole("cell").nth(priority)).not.toHaveText("—");
});
