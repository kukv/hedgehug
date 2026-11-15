import { expect, test } from "./fixtures";
import { resetSearchScreen, tokenUrl } from "./helpers";
import { filterOf, search } from "./search-tree-helpers";

// 検索の画面から「ビューとして保存」する流れ(E3b-10・11)の通しのテスト。

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
