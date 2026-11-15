import type { Locator, Page, TestInfo } from "@playwright/test";
import { expect, test } from "./fixtures";
import { prefixOf, resetSearchScreen, sidebar, tokenUrl } from "./helpers";
import { filterOf, search } from "./search-tree-helpers";

// 保存したビューの変更が、同じブラウザの別のタブに届く通しのテスト(E3b-3・E3b-6)。
// ビューは同じ worker のほかのテストと同じサーバーに残るので、名前に接頭辞を入れて重ならないようにする。

const auth = { Authorization: "Bearer e2e-token" };

// newView は、絞り込み kind:bug・テーブルのビューを API で作り、識別子を返す。
async function newView(page: Page, name: string) {
  const table = {
    columns: ["id", "title", "status", "assignee", "priority", "end", "points"],
    group: "none",
    sort: null,
  };
  const res = await page.request.post("/api/views", {
    headers: auth,
    data: { name, content: { query: "kind:bug", layout: "table", table } },
  });
  if (!res.ok()) throw new Error(`newView: ${res.status()} ${await res.text()}`);
  return ((await res.json()) as { id: string }).id;
}

const viewRow = (side: Locator, name: string) =>
  side.getByRole("region", { name: "保存したビュー" }).getByRole("link", { name, exact: true });

const mark = (page: Page) => page.getByText("未保存の変更", { exact: true });

// open は、2 つ目のタブ B を作り、2 つのタブで同じビューを開く。
async function open(page: Page, info: TestInfo) {
  const name = `バグ ${prefixOf(info)}`;
  const id = await newView(page, name);
  const other = await page.context().newPage();
  for (const p of [page, other]) {
    await p.goto(tokenUrl(`/views/${id}`));
    await expect(p.getByRole("heading", { name })).toBeVisible();
    await expect(filterOf(p)).toHaveValue("kind:bug");
  }
  return { name, id, a: page, b: other };
}

test.beforeEach(async ({ page }) => {
  await resetSearchScreen(page.request);
});

test("save in another tab clears the mark but keeps the filter", async ({ page }, info) => {
  const { a, b } = await open(page, info);
  await search(a, "kind:task");
  await expect(mark(a)).toBeVisible();
  // 絞り込み欄にフォーカスがあるタブは、画面への反映を待つ(06)。フォーカスを外して反映を受ける。
  await filterOf(a).blur();

  await search(b, "kind:task");
  await expect(mark(b)).toBeVisible();
  await b.getByRole("button", { name: "保存", exact: true }).click();
  await expect(b.getByRole("status").filter({ hasText: "ビューを保存しました" })).toBeVisible();

  await expect(mark(a)).toBeHidden();
  await expect(filterOf(a)).toHaveValue("kind:task");
});

test("rename in another tab updates the heading and the sidebar", async ({ page }, info) => {
  const { name, a, b } = await open(page, info);
  const renamed = `BUG ${prefixOf(info)}`;

  await b.getByRole("button", { name: `${name} のメニュー` }).click();
  await b.getByRole("menuitem", { name: "名前を変更…", exact: true }).click();
  const dialog = b.getByRole("dialog", { name: "ビューの名前を変更" });
  const input = dialog.getByRole("textbox", { name: "名前" });
  await input.fill(renamed);
  await input.press("Enter");
  await expect(dialog).toBeHidden();

  await expect(a.getByRole("heading", { name: renamed })).toBeVisible();
  const side = await sidebar(a, info);
  await expect(viewRow(side, renamed)).toBeVisible();
  await expect(viewRow(side, name)).toHaveCount(0);
});

test("delete in another tab moves to home and clears the sidebar", async ({ page }, info) => {
  const { name, a, b } = await open(page, info);

  await b.getByRole("button", { name: `${name} のメニュー` }).click();
  await b.getByRole("menuitem", { name: "削除…", exact: true }).click();
  const dialog = b.getByRole("dialog", { name: "ビューを削除" });
  await dialog.getByRole("button", { name: "削除", exact: true }).click();
  await expect(b).not.toHaveURL(/\/views\//);

  await expect(a).not.toHaveURL(/\/views\//);
  await expect(viewRow(await sidebar(a, info), name)).toHaveCount(0);
});
