import type { Locator, Page, TestInfo } from "@playwright/test";
import { expect, test } from "./fixtures";
import { prefixOf, resetSearchScreen, sidebar, tokenUrl } from "./helpers";
import { filterOf, search } from "./search-tree-helpers";

// ビューのメニュー(E3b-13)の通しのテスト。ビューは同じ worker のほかのテストと同じサーバーに残るので、
// 名前に接頭辞を入れて重ならないようにする。

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

// viewRow は、サイドバーの「保存したビュー」の、その名前の行。
const viewRow = (side: Locator, name: string) =>
  side.getByRole("region", { name: "保存したビュー" }).getByRole("link", { name, exact: true });

// choose は、ビューの画面の見出しのメニューを開き、項目を選ぶ。
async function choose(page: Page, name: string, item: string) {
  await page.getByRole("button", { name: `${name} のメニュー` }).click();
  await page.getByRole("menuitem", { name: item, exact: true }).click();
}

const mark = (page: Page) => page.getByText("未保存の変更", { exact: true });

async function start(page: Page, info: TestInfo) {
  const prefix = prefixOf(info);
  const name = `バグ ${prefix}`;
  const id = await newView(page, name);
  await page.goto(tokenUrl(`/views/${id}`));
  await expect(page.getByRole("heading", { name })).toBeVisible();
  await expect(filterOf(page)).toHaveValue("kind:bug");
  return { prefix, name, id };
}

test.beforeEach(async ({ page }) => {
  await resetSearchScreen(page.request);
});

test("rename", async ({ page }, info) => {
  const { prefix, name } = await start(page, info);
  const other = `別 ${prefix}`;
  await newView(page, other);
  await search(page, "kind:task");
  await expect(mark(page)).toBeVisible();

  await choose(page, name, "名前を変更…");
  const dialog = page.getByRole("dialog", { name: "ビューの名前を変更" });
  const input = dialog.getByRole("textbox", { name: "名前" });
  await expect(input).toHaveValue(name);

  await input.fill("");
  await input.press("Enter");
  await expect(dialog.getByRole("alert")).toHaveText("名前を入力してください");
  await input.fill(other);
  await input.press("Enter");
  await expect(dialog.getByRole("alert")).toHaveText("同じ名前のビューがあります");

  const renamed = `BUG ${prefix}`;
  await input.fill(renamed);
  await input.press("Enter");
  await expect(dialog).toBeHidden();
  await expect(page.getByRole("heading", { name: renamed })).toBeVisible();
  const side = await sidebar(page, info);
  await expect(viewRow(side, renamed)).toBeVisible();
  await expect(viewRow(side, name)).toHaveCount(0);
  // 未保存の変更は、名前を変えたあとも残る。
  await expect(mark(page)).toBeVisible();
  await expect(filterOf(page)).toHaveValue("kind:task");
});

test("duplicate", async ({ page }, info) => {
  const { name, id } = await start(page, info);
  await search(page, "kind:task");
  await expect(mark(page)).toBeVisible();

  await choose(page, name, "複製");
  const copy = `${name} のコピー`;
  await expect(page.getByRole("heading", { name: copy })).toBeVisible();
  await expect(filterOf(page)).toHaveValue("kind:bug");
  await expect(mark(page)).toBeHidden();

  await page.goto(tokenUrl(`/views/${id}`));
  await expect(page.getByRole("heading", { name })).toBeVisible();
  await choose(page, name, "複製");
  await expect(page.getByRole("heading", { name: `${copy} 2` })).toBeVisible();
});

// closeDrawer は、狭い幅で開いた引き出しを閉じる。暗くした背景の、引き出しに重ならない右上を押す(frame.spec.ts と同じ)。
async function closeDrawer(page: Page, info: TestInfo) {
  if (info.project.name === "desktop") return;
  const width = page.viewportSize()?.width ?? 0;
  await page
    .getByRole("button", { name: "メニューを閉じる" })
    .click({ position: { x: width - 10, y: 10 } });
  await expect(page.getByRole("complementary")).not.toBeInViewport();
}

test("delete", async ({ page }, info) => {
  const { name, id } = await start(page, info);
  // 削除する前は、サイドバーに行がある。
  await expect(viewRow(await sidebar(page, info), name)).toBeVisible();
  await closeDrawer(page, info);

  await choose(page, name, "削除…");
  const dialog = page.getByRole("dialog", { name: "ビューを削除" });
  await expect(dialog).toContainText(
    `ビュー「${name}」を削除します。アイテムは消えません。元に戻せません。`,
  );
  await dialog.getByRole("button", { name: "削除", exact: true }).click();
  await expect(page.getByRole("status").filter({ hasText: "ビューを削除しました" })).toBeVisible();
  await expect(page).not.toHaveURL(/\/views\//);
  await expect(viewRow(await sidebar(page, info), name)).toHaveCount(0);

  // 削除したビューのアドレスを開いてもホームになる。
  await page.goto(tokenUrl(`/views/${id}`));
  await expect(page).not.toHaveURL(/\/views\//);

  // 同じ名前で保存し直したビューには、印がない。
  const again = await newView(page, name);
  await page.goto(tokenUrl(`/views/${again}`));
  await expect(page.getByRole("heading", { name })).toBeVisible();
  await expect(filterOf(page)).toHaveValue("kind:bug");
  await expect(mark(page)).toBeHidden();
});
