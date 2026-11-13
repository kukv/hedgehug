import type { Locator, Page } from "@playwright/test";
import { expect, test } from "./fixtures";
import { tokenUrl, treeRow, treeTitles } from "./helpers";

// 閲覧のみの Store のツリー(A4b)。書き込みのキーと、メニューの変更系の項目は何もしない・無効で、
// 読むだけのキー(矢印・Enter・メニューキー)は使える。サンプル(HH-1 の Epic「認証」と HH-2 の Task)は読むだけで、書き換えない。

const note = "閲覧のみの Store です";

async function openTree(page: Page) {
  await page.goto(tokenUrl("/s/readonly/p/HH-P1/tree"));
  const tree = page.getByRole("list", { name: "ツリー" });
  await expect(tree).toBeVisible();
  return tree;
}

test("a read-only store's tree ignores the write keys and disables the menu's write items", async ({
  page,
}) => {
  const tree = await openTree(page);
  const current = tree.locator('li[aria-current="true"]');
  const status = treeRow(tree, "HH-2").getByRole("button", { name: /^ステータス: / });
  await expect(status).toBeDisabled();
  const before = await treeTitles(tree);
  const statusName = await status.getAttribute("aria-label");

  // 読むだけのキーは使える。
  await tree.focus();
  await expect(current).toContainText("HH-1");
  await page.keyboard.press("ArrowDown");
  await expect(current).toContainText("HH-2");
  await page.keyboard.press("Enter");
  const panel = page.getByRole("region", { name: "詳細パネル" });
  await expect(panel.getByRole("textbox", { name: "タイトル" })).toHaveValue("ログイン画面を作る");
  // 狭い幅では、パネルがツリーに重なる。閉じてから、ツリーに戻る。
  await panel.getByRole("button", { name: "閉じる" }).click();

  // 書き込みのキーは、何もしない。トーストも出ない。
  await tree.focus();
  await page.keyboard.press("ArrowDown");
  await expect(current).toContainText("HH-2");
  for (const key of [
    "Space",
    "s",
    "k",
    "c",
    "Alt+ArrowUp",
    "Alt+ArrowDown",
    "Alt+Shift+ArrowDown",
  ]) {
    await page.keyboard.press(key);
  }
  await expect(page.getByRole("dialog")).toHaveCount(0);
  await expect(page.getByRole("menu")).toHaveCount(0);
  await expect(page.getByRole("status")).toHaveCount(0);
  await expect(status).toHaveAttribute("aria-label", statusName ?? "");
  expect(await treeTitles(tree)).toEqual(before);

  // メニューキーは使え、変更系の項目は無効で理由を添える。「詳細を開く」「ID をコピー」は使える。
  await page.keyboard.press("Shift+F10");
  for (const name of [/^上へ移動/, /^下へ移動/, /^移動…/, /^種類を変更/, /^複製/, /^削除…/]) {
    const item = page.getByRole("menuitem", { name });
    await expect(item).toBeDisabled();
    await expect(item).toContainText(note);
  }
  await expect(page.getByRole("menuitem", { name: /^詳細を開く/ })).toBeEnabled();
  await expect(page.getByRole("menuitem", { name: /^ID をコピー/ })).toBeEnabled();
});

// マウスやタッチで行を押したとき、ツリーにフォーカスが入っても先頭の行へは動かない(動かすと、画面の外の先頭へスクロールして、
// 押した行ではなく別の行に click が当たる)。書き込まないので、閲覧のみの Store のサンプルで確かめる。
test("pressing a row below an off-screen first row focuses that row and does not scroll", async ({
  page,
  browser,
  baseURL,
}) => {
  const pressSecond = async (p: Page, press: (code: Locator) => Promise<void>) => {
    const tree = await openTree(p);
    // 先頭の行を高くして、2 行目だけが見える位置にする。
    await tree.evaluate((el) => {
      (el.querySelector("li") as HTMLElement).style.minHeight = "1500px";
      (el.parentElement as HTMLElement).style.paddingBottom = "1500px";
    });
    const second = treeRow(tree, "HH-2");
    await second.scrollIntoViewIfNeeded();
    const scroll = () => p.evaluate(() => document.scrollingElement?.scrollTop ?? 0);
    const before = await scroll();
    await press(second.locator("code"));
    await expect(tree.locator('li[aria-current="true"]')).toContainText("HH-2");
    expect(Math.abs((await scroll()) - before)).toBeLessThan(5);
  };

  await test.step("with a mouse click", () => pressSecond(page, (code) => code.click()));

  await test.step("with a touch tap", async () => {
    const touch = await browser.newContext({
      viewport: page.viewportSize(),
      baseURL,
      hasTouch: true,
    });
    try {
      await pressSecond(await touch.newPage(), (code) => code.tap());
    } finally {
      await touch.close();
    }
  });
});
