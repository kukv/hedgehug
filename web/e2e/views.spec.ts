import type { Page, TestInfo } from "@playwright/test";
import { expect, test } from "./fixtures";
import { prefixOf, resetSearchScreen, tokenUrl } from "./helpers";
import { filterOf, search } from "./search-tree-helpers";

// 検索の画面から「ビューとして保存」する流れ(E3b-10・11)と、ビューの画面(E3b-4・E3b-9・E3b-12)の通しのテスト。

// newView は、絞り込み kind:bug・テーブルのビューを API で作り、識別子と名前を返す。ビューは同じ worker のほかの
// テストと同じサーバーに残るので、名前に接頭辞を入れて重ならないようにする。
async function newView(page: Page, info: TestInfo) {
  const name = `バグ ${prefixOf(info)}`;
  const table = {
    columns: ["id", "title", "status", "assignee", "priority", "end", "points"],
    group: "none",
    sort: null,
  };
  const res = await page.request.post("/api/views", {
    headers: { Authorization: "Bearer e2e-token" },
    data: { name, content: { query: "kind:bug", layout: "table", table } },
  });
  if (!res.ok()) throw new Error(`newView: ${res.status()} ${await res.text()}`);
  const { id } = (await res.json()) as { id: string };
  return { id, name };
}

const layoutButton = (page: Page, name: string) =>
  page.getByRole("group", { name: "レイアウト" }).getByRole("button", { name, exact: true });

test.beforeEach(async ({ page }) => {
  await resetSearchScreen(page.request);
});

test("saves the search as a view", async ({ page }) => {
  await page.goto(tokenUrl("/search"));
  await search(page, "kind:bug");
  await page
    .getByRole("group", { name: "レイアウト" })
    .getByRole("button", { name: "ツリー", exact: true })
    .click();

  const open = () => page.getByRole("button", { name: "ビューとして保存", exact: true }).click();
  await open();
  const dialog = page.getByRole("dialog", { name: "ビューとして保存" });
  await expect(dialog).toContainText("レイアウト: ツリー");
  await expect(dialog.locator("code")).toHaveText("kind:bug");

  await dialog.getByRole("textbox", { name: "名前" }).press("Enter");
  await expect(dialog.getByRole("alert")).toHaveText("名前を入力してください");

  await dialog.getByRole("textbox", { name: "名前" }).fill("バグ");
  await dialog.getByRole("textbox", { name: "名前" }).press("Enter");
  await expect(
    page.getByRole("status").filter({ hasText: "ビュー「バグ」を保存しました" }),
  ).toBeVisible();
  await expect(page).toHaveURL(/\/views\/[^/#]+/);
  await expect(page.getByRole("heading", { name: "バグ" })).toBeVisible();

  // もう一度、検索の画面から同じ名前(前後に空白)で保存すると断られる。
  await page.goto(tokenUrl("/search"));
  await expect(filterOf(page)).toHaveValue("kind:bug");
  await open();
  const again = page.getByRole("dialog", { name: "ビューとして保存" });
  await again.getByRole("textbox", { name: "名前" }).fill(" バグ ");
  await again.getByRole("textbox", { name: "名前" }).press("Enter");
  await expect(again.getByRole("alert")).toHaveText("同じ名前のビューがあります");
});

test("unsaved changes survive reload", async ({ page }, info) => {
  const view = await newView(page, info);
  await page.goto(tokenUrl(`/views/${view.id}`));
  await expect(page.getByRole("heading", { name: view.name })).toBeVisible();
  await expect(page.getByText("保存したビュー", { exact: true })).toBeVisible();
  await expect(filterOf(page)).toHaveValue("kind:bug");
  const mark = page.getByText("未保存の変更", { exact: true });
  const discard = page.getByRole("button", { name: "変更を破棄", exact: true });
  const save = page.getByRole("button", { name: "保存", exact: true });
  await expect(mark).toBeHidden();
  await expect(page.getByRole("button", { name: `${view.name} のメニュー` })).toBeVisible();

  await search(page, "kind:task");
  await expect(mark).toBeVisible();
  await expect(discard).toBeVisible();
  await expect(save).toBeVisible();

  await page.reload();
  await expect(filterOf(page)).toHaveValue("kind:task");
  await expect(mark).toBeVisible();

  await discard.click();
  await expect(filterOf(page)).toHaveValue("kind:bug");
  await expect(mark).toBeHidden();
  await expect(save).toBeHidden();

  await layoutButton(page, "ツリー").click();
  await expect(mark).toBeVisible();
  await layoutButton(page, "テーブル").click();
  await expect(mark).toBeHidden();
});

test("save keeps the view", async ({ page }, info) => {
  const view = await newView(page, info);
  await page.goto(tokenUrl(`/views/${view.id}`));
  await expect(filterOf(page)).toHaveValue("kind:bug");
  await search(page, "kind:task");
  await page.getByRole("button", { name: "保存", exact: true }).click();
  await expect(page.getByRole("status").filter({ hasText: "ビューを保存しました" })).toBeVisible();
  await expect(page.getByText("未保存の変更", { exact: true })).toBeHidden();
  const res = await page.request.get("/api/views", {
    headers: { Authorization: "Bearer e2e-token" },
  });
  const { views } = (await res.json()) as { views: { id: string; content: { query: string } }[] };
  expect(views.find((v) => v.id === view.id)?.content.query).toBe("kind:task");

  await page.goto(tokenUrl("/search"));
  await expect(page.getByRole("heading", { name: "検索" })).toBeVisible();
  await page.goto(tokenUrl(`/views/${view.id}`));
  await expect(filterOf(page)).toHaveValue("kind:task");
  await expect(page.getByText("未保存の変更", { exact: true })).toBeHidden();
});

test("view screen has no save-as button", async ({ page }, info) => {
  const view = await newView(page, info);
  await page.goto(tokenUrl(`/views/${view.id}`));
  await expect(filterOf(page)).toHaveValue("kind:bug");
  await expect(page.getByRole("button", { name: "ビューとして保存", exact: true })).toHaveCount(0);
});
