import { expect, test } from "@playwright/test";
import { sidebar, tokenUrl } from "./helpers";

test("the sync card shows the stores and syncs them all", async ({ page }, info) => {
  await page.goto(tokenUrl());
  const side = await sidebar(page, info);
  await expect(side.getByText(/最終同期 .+ · Store 2 件/)).toBeVisible();
  const all = side.getByRole("button", { name: "すべて同期" });
  await expect(all).toHaveAttribute("title", /Shift\+(Ctrl|Cmd)\+S/);
  await all.click();
  await expect(
    page.getByRole("status").filter({ hasText: "2 件の Store を同期しました" }),
  ).toBeVisible();
  await expect(side.getByText("すべて最新")).toBeVisible();
});

test("Shift+Ctrl+S syncs while typing in the title", async ({ page }) => {
  await page.goto(tokenUrl("/s/e2e/i/HH-2"));
  const panel = page.getByRole("region", { name: "詳細パネル" });
  const title = panel.getByRole("textbox", { name: "タイトル" });
  await title.fill("入力中のタイトル");
  await page.keyboard.press("Shift+Control+S");
  await expect(
    page.getByRole("status").filter({ hasText: "2 件の Store を同期しました" }),
  ).toBeVisible();
  await expect(title).toHaveValue("入力中のタイトル");
  await expect(title).toBeFocused();
  // サーバーは 3 つの幅とほかのテストで共有なので、元のタイトルに戻してから離れる(保存されない)。
  await title.fill("ログイン画面を作る");
  await title.blur();
});
