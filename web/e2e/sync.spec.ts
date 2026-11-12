import { expect, test } from "@playwright/test";
import { sidebar, tokenUrl, unique } from "./helpers";

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

// フォーカスを置いただけで打ち換えていないタイトルは、離れたときに古い値で書き戻さず、同期で受け取った値に合わせる。
test("an untouched focused title takes the synced title on blur", async ({
  page,
  context,
}, info) => {
  const task = unique(info, "反映のタイトル");
  await page.goto(tokenUrl("/s/e2e/p/HH-P1/tree"));
  await page.getByRole("list", { name: "ツリー" }).getByRole("link", { name: "認証" }).click();
  const panel = page.getByRole("region", { name: "詳細パネル" });
  await panel.getByRole("button", { name: "Task" }).click();
  const dialog = page.getByRole("dialog", { name: "新規作成" });
  await dialog.getByRole("textbox", { name: "タイトル" }).fill(task);
  await dialog.getByRole("textbox", { name: "タイトル" }).press("Enter");
  const title = panel.getByRole("textbox", { name: "タイトル" });
  await expect(title).toHaveValue(task);
  const id = await panel.locator("code.id").textContent();

  await title.focus();
  const other = await context.newPage();
  await other.goto(tokenUrl(`/s/e2e/i/${id}`));
  const otherTitle = other
    .getByRole("region", { name: "詳細パネル" })
    .getByRole("textbox", { name: "タイトル" });
  await expect(otherTitle).toHaveValue(task);
  await otherTitle.fill(`${task} 別タブ`);
  await otherTitle.press("Enter");
  await expect(otherTitle).toHaveValue(`${task} 別タブ`);

  await page.bringToFront();
  await expect(title).toBeFocused();
  await page.keyboard.press("Shift+Control+S");
  await expect(
    page.getByRole("status").filter({ hasText: "2 件の Store を同期しました" }),
  ).toBeVisible();
  await expect(title).toHaveValue(task);
  await title.blur();
  await expect(title).toHaveValue(`${task} 別タブ`);
  // サーバーに残った値を、別のページで開き直して確かめる(/i/ のアドレスはツリーへ移るので、reload では開かない)。
  await other.goto(tokenUrl(`/s/e2e/i/${id}`));
  await expect(otherTitle).toHaveValue(`${task} 別タブ`);
});
