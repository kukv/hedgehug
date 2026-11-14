import type { Page, TestInfo } from "@playwright/test";
import { expect, test } from "./fixtures";
import { prefixOf, sidebar, tokenUrl } from "./helpers";

// 検索の画面の通しのテスト(E1。04「検索」)。Store の変更は API で作り、作ったアイテムのタイトルに
// テストごとの印を入れて、サンプルや同じサーバーを使うほかのテストの件数に頼らない。

const e2eStore = "01E2ESTORE0000000000000001";
const auth = { Authorization: "Bearer e2e-token" };
const base = `/api/stores/${e2eStore}`;

const filterOf = (page: Page) => page.getByRole("combobox", { name: "絞り込み" });
const panelOf = (page: Page) => page.getByRole("region", { name: "詳細パネル" });

async function send(page: Page, method: "post" | "delete", path: string, data?: unknown) {
  const res = await page.request[method](path, { headers: auth, data });
  if (!res.ok()) throw new Error(`${path}: ${res.status()} ${await res.text()}`);
  return res.status() === 204 ? undefined : res.json();
}

// seed は、新しいワークスペースにプロジェクト(日本語の名前)と Epic を 1 つ作る。mark はタイトルの語
// (接頭辞を含めるので、ほかのテストと重ならない)。
async function seed(page: Page, info: TestInfo) {
  const prefix = prefixOf(info);
  const mark = `検索試験${prefix}`;
  const ws = await send(page, "post", `${base}/workspaces`, { name: `検索 ${prefix}`, prefix });
  const project = await send(page, "post", `${base}/workspaces/${ws.id}/projects`, {
    title: `探す計画${prefix}`,
  });
  const item = await send(page, "post", `${base}/items`, {
    project: project.id,
    kind: "Epic",
    title: `${mark}の件`,
  });
  return { prefix, mark, item, projectDisplay: `${prefix}-P${project.number}` };
}

async function openSearch(page: Page, info: TestInfo) {
  await page.goto(tokenUrl());
  const side = await sidebar(page, info);
  // 前の値の読み込みが終わる前に打つと、読んだ値で上書きされるので、応答を待つ。
  const loaded = page.waitForResponse(
    (r) => r.url().includes("/api/screen-states/search") && r.request().method() === "GET",
  );
  await side.getByRole("link", { name: "検索", exact: true }).click();
  await expect(page).toHaveURL(/\/search$/);
  await loaded;
  return filterOf(page);
}

test("opens the search screen from the sidebar and shows its frame", async ({ page }, info) => {
  await openSearch(page, info);
  const side = await sidebar(page, info);
  await expect(side.getByRole("link", { name: "検索", exact: true })).toHaveAttribute(
    "aria-current",
    "page",
  );
  const main = page.getByRole("main");
  await expect(main.getByRole("heading", { name: "検索", level: 1 })).toBeVisible();
  await expect(
    main.getByText("すべての Store のアイテムを、条件で絞り込んで見ます", { exact: true }),
  ).toBeVisible();
  await expect(filterOf(page)).toHaveAttribute(
    "placeholder",
    "絞り込み 例: assignee:@me status:InProgress system:Web",
  );
  for (const column of ["ID", "タイトル", "ステータス", "担当者", "優先度", "終了日", "ポイント"]) {
    await expect(main.getByRole("columnheader", { name: column, exact: true })).toBeVisible();
  }
});

test("finds an item by a word of its title, and opens the detail panel", async ({ page }, info) => {
  const { mark } = await seed(page, info);
  const filter = await openSearch(page, info);
  await filter.fill(mark);
  await expect(page.getByText("1 件", { exact: true })).toBeVisible();
  const row = page.getByRole("row", { name: new RegExp(mark) });
  await expect(row.getByRole("img", { name: "e2e", exact: true })).toBeVisible();
  await row.getByRole("link", { name: `${mark}の件` }).click();
  await expect(panelOf(page).getByRole("textbox", { name: "タイトル" })).toHaveValue(`${mark}の件`);
});

test("lists a deleted item only with is:deleted, as a row that cannot be opened", async ({
  page,
}, info) => {
  const { mark, item } = await seed(page, info);
  await send(page, "delete", `${base}/items/${item.id}`);
  const filter = await openSearch(page, info);
  await filter.fill(mark);
  await expect(page.getByText("0 件", { exact: true })).toBeVisible();
  await filter.fill(`is:deleted ${mark}`);
  await expect(page.getByText("1 件", { exact: true })).toBeVisible();
  const row = page.getByRole("row", { name: new RegExp(mark) });
  await expect(row.getByText("削除済み", { exact: true })).toBeVisible();
  await expect(row.getByText("あと 30 日で完全に削除", { exact: true })).toBeVisible();
  await expect(row.getByRole("link")).toHaveCount(0);
});

test("shows the error of a key without a value and a count of 0", async ({ page }, info) => {
  const filter = await openSearch(page, info);
  await filter.fill("kind:");
  await expect(page.getByRole("alert").getByText("「kind:」に値がありません")).toBeVisible();
  await expect(page.getByText("0 件", { exact: true })).toBeVisible();
});

test("completes a project by a part of its Japanese name", async ({ page }, info) => {
  const { prefix, projectDisplay } = await seed(page, info);
  const filter = await openSearch(page, info);
  // 絞り込みは画面の状態として残るので、前のテストの値を消してから打つ。
  await filter.fill("");
  await filter.pressSequentially(`project:探す計画${prefix}`);
  await expect(page.getByRole("listbox", { name: "候補" }).getByRole("option")).toHaveCount(1);
  await page.keyboard.press("Enter");
  await expect(filter).toHaveValue(`project:${projectDisplay}`);
});

test("moves to the filter with the slash key, and opens and closes the search help", async ({
  page,
}, info) => {
  await openSearch(page, info);
  await page.getByRole("main").getByRole("heading", { name: "検索", level: 1 }).click();
  await expect(filterOf(page)).not.toBeFocused();
  await page.keyboard.press("/");
  await expect(filterOf(page)).toBeFocused();

  await page.getByRole("button", { name: "検索ヘルプ" }).click();
  const help = page.getByRole("dialog", { name: "検索ヘルプ" });
  await expect(help).toBeVisible();
  await help.getByRole("button", { name: "閉じる" }).click();
  await expect(help).toBeHidden();
});

test("keeps the filter after a reload", async ({ page }, info) => {
  const { mark } = await seed(page, info);
  const filter = await openSearch(page, info);
  const saved = page.waitForResponse(
    (r) =>
      r.url().includes("/api/screen-states/search") && r.request().method() === "PUT" && r.ok(),
  );
  await filter.fill(mark);
  await saved;
  await page.reload();
  await expect(filterOf(page)).toHaveValue(mark);
  await expect(page.getByText("1 件", { exact: true })).toBeVisible();
});

test("opens the search with the query typed into the palette", async ({ page }) => {
  await page.goto(tokenUrl());
  await page.keyboard.press("ControlOrMeta+k");
  const palette = page.getByRole("dialog", { name: "コマンドパレット" });
  await palette.getByRole("combobox").fill("kind:bug");
  await palette.getByRole("option", { name: /^検索で開く/ }).click();
  await expect(palette).toBeHidden();
  await expect(page).toHaveURL(/\/search$/);
  await expect(filterOf(page)).toHaveValue("kind:bug");
});

test("shows the read-only band for an item of a read-only store", async ({ page }, info) => {
  const filter = await openSearch(page, info);
  await filter.fill("store:readonly ログイン画面");
  await expect(
    page.getByText(
      "readonly は閲覧のみの Store です。閲覧はできますが、作成・変更・削除はできません。",
    ),
  ).toBeVisible();
  await expect(page.getByText("1 件", { exact: true })).toBeVisible();
  await expect(page.getByRole("row", { name: /^readonly HH-2 ログイン画面を作る/ })).toBeVisible();
});
