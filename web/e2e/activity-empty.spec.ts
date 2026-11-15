import { expect, test } from "./fixtures";
import { tokenUrl } from "./helpers";

// アクティビティが空のときの表示(C4d)。同じ worker のほかのテストが作る自動化の変更が残らないよう、別のサーバーで動かす。
test.use({ ownServer: true });

test("the automation-only filter shows the empty message", async ({ page }) => {
  await page.goto(tokenUrl("/activity"));
  await expect(page.getByRole("heading", { name: "アクティビティ" })).toBeVisible();
  await page.getByRole("combobox", { name: "種類" }).selectOption({ label: "自動化だけ" });
  await expect(page.getByText("この条件のコミットはありません")).toBeVisible();
});
