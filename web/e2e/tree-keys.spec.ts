import type { Locator } from "@playwright/test";
import { expect, test } from "./fixtures";
import {
  addEpic,
  createWorkspace,
  prefixOf,
  projectWithEpic,
  taskUnderEpic,
  treeOf,
  treeRow,
  treeTitles,
  unique,
} from "./helpers";

// ツリーのキー操作の通しのテスト(A4b。04「ツリーのキー操作」)。ワークスペースを 1 つだけ作り、1 つのテストの中で順に確かめる
// (足すアイテムは 6 件)。Cmd ではなく Ctrl を使う(CI は Linux)。

// 行を押して、キーのフォーカスをその行に付ける(ID を押す。タイトルを押すと詳細パネルが開く)。
const pick = (tree: Locator, id: string) => treeRow(tree, id).locator("code").click();

test("moves around, opens, changes and reorders the tree with the keyboard", async ({
  page,
  context,
}, info) => {
  const t = (s: string) => unique(info, s);
  const prefix = prefixOf(info);
  await createWorkspace(page, info, unique(info, "キー"), prefix);
  // Epic -1 の下に Task A(-2)・B(-3)・C(-4)、もう 1 つの Epic(-5)。
  await projectWithEpic(page, info, prefix);
  for (const name of ["A", "B", "C"]) await taskUnderEpic(page, info, `${prefix}-1`, name);
  await addEpic(page, info, prefix, "別の Epic");
  const tree = await treeOf(page, prefix);
  const current = tree.locator('li[aria-current="true"]');
  const [epic, epic2] = [t("通知"), t("別の Epic")];
  const order = () => treeTitles(tree);
  const status = (id: string) => treeRow(tree, id).getByRole("button", { name: /^ステータス: / });
  const panel = page.getByRole("region", { name: "詳細パネル" });

  await test.step("the tree is one tab stop, and the first row gets the focus on entering", async () => {
    await tree.focus();
    await expect(current).toContainText(`${prefix}-1`);
    // Tab は、行の中のボタンやリンクには止まらず、ツリーを出る。
    await page.keyboard.press("Tab");
    await expect(tree).not.toBeFocused();
    await expect(tree.locator(":focus")).toHaveCount(0);
    await page.keyboard.press("Shift+Tab");
    await expect(tree).toBeFocused();
    await expect(current).toContainText(`${prefix}-1`);
  });

  await test.step("arrows move through the visible rows and open or close them", async () => {
    await page.keyboard.press("ArrowUp");
    await expect(current).toContainText(`${prefix}-1`);
    await page.keyboard.press("ArrowDown");
    await expect(current).toContainText(`${prefix}-2`);
    await page.keyboard.press("ArrowLeft");
    await expect(current).toContainText(`${prefix}-1`);
    await page.keyboard.press("ArrowLeft");
    await expect(tree.getByRole("link", { name: t("A") })).toHaveCount(0);
    await page.keyboard.press("ArrowRight");
    await expect(tree.getByRole("link", { name: t("A") })).toBeVisible();
    await page.keyboard.press("ArrowRight");
    await expect(current).toContainText(`${prefix}-2`);
  });

  await test.step("after a click on a row's button, the keys act on the row they move to", async () => {
    // 開閉のボタンを押すと、その行にフォーカスが付く。続く ↓ と Enter は、移った行に効く。
    const twisty = treeRow(tree, `${prefix}-1`).getByRole("button", { name: "開閉" });
    await twisty.click();
    await expect(tree.getByRole("link", { name: t("A") })).toHaveCount(0);
    await twisty.click();
    await expect(tree.getByRole("link", { name: t("A") })).toBeVisible();
    await expect(current).toContainText(`${prefix}-1`);
    await page.keyboard.press("ArrowDown");
    await expect(current).toContainText(`${prefix}-2`);
    await page.keyboard.press("Enter");
    await expect(panel.getByRole("textbox", { name: "タイトル" })).toHaveValue(t("A"));
    // 狭い幅では、パネルがツリーに重なる。閉じてから次の行を押す。
    await panel.getByRole("button", { name: "閉じる" }).click();
  });

  await test.step("Ctrl+Enter opens the row in a new tab", async () => {
    await pick(tree, `${prefix}-3`);
    const [tab] = await Promise.all([
      context.waitForEvent("page"),
      page.keyboard.press("Control+Enter"),
    ]);
    const opened = tab.getByRole("region", { name: "詳細パネル" });
    await expect(opened.getByRole("textbox", { name: "タイトル" })).toHaveValue(t("B"));
    await tab.close();
  });

  await test.step("Space toggles done, S opens the status menu and the menu takes the keys", async () => {
    await pick(tree, `${prefix}-2`);
    await page.keyboard.press("Space");
    await expect(status(`${prefix}-2`)).toHaveAccessibleName("ステータス: Done");
    await page.keyboard.press("Space");
    await expect(status(`${prefix}-2`)).toHaveAccessibleName("ステータス: Todo");
    await page.keyboard.press("s");
    await expect(page.getByRole("menu", { name: "ステータス" })).toBeVisible();
    await page.keyboard.press("Escape");
    await expect(page.getByRole("menu")).toHaveCount(0);
    // メニューを閉じると、ツリーに戻る。いまの行は動かない。
    await expect(tree).toBeFocused();
    await expect(current).toContainText(`${prefix}-2`);
    await page.keyboard.press("s");
    // メニューは、開くと先頭の項目(Todo)にフォーカスを移す。移ってから、↓ で InProgress へ、Enter で選ぶ。
    await expect(page.getByRole("menuitemcheckbox", { name: /Todo/ })).toBeFocused();
    await page.keyboard.press("ArrowDown");
    await page.keyboard.press("Enter");
    await expect(status(`${prefix}-2`)).toHaveAccessibleName("ステータス: InProgress");
    await expect(tree).toBeFocused();
    await expect(current).toContainText(`${prefix}-2`);
  });

  await test.step("the menu key opens the row's menu; K does nothing on an Epic", async () => {
    await page.keyboard.press("Shift+F10");
    // 先頭の押せる項目(行のメニューの「詳細を開く」)にフォーカスが移る。-2 は先頭の兄弟なので「上へ移動」は無効。
    await expect(page.getByRole("menuitem", { name: /^詳細を開く/ })).toBeFocused();
    await expect(page.getByRole("menuitem", { name: /^上へ移動/ })).toBeDisabled();
    await page.keyboard.press("Escape");
    // 閉じるとツリーに戻り、いまの行は動かない。
    await expect(tree).toBeFocused();
    await expect(current).toContainText(`${prefix}-2`);
    await pick(tree, `${prefix}-1`);
    await page.keyboard.press("k");
    await expect(page.getByRole("menu")).toHaveCount(0);
    await pick(tree, `${prefix}-2`);
    await page.keyboard.press("k");
    await expect(page.getByRole("menu")).toBeVisible();
    await page.keyboard.press("Escape");
    await expect(page.getByRole("menu")).toHaveCount(0);
  });

  await test.step("C opens the create dialog for a sibling, and the tree ignores keys typed in it", async () => {
    await pick(tree, `${prefix}-3`);
    await page.keyboard.press("c");
    const dialog = page.getByRole("dialog", { name: "新規作成" });
    // 1 つだけ開く(App.svelte の文脈なしの C キーは、ツリーが受けたときは動かない)。
    await expect(dialog).toHaveCount(1);
    const title = dialog.getByRole("textbox", { name: "タイトル" });
    // ダイアログの入力欄で押したキーは、ツリーのキーにならない(行も動かず、ダイアログも増えない)。
    await title.pressSequentially("csk");
    await title.press("ArrowDown");
    await expect(dialog).toHaveCount(1);
    await expect(title).toHaveValue("csk");
    await expect(current).toContainText(`${prefix}-3`);
    // ダイアログを開いているあいだは、ツリーにフォーカスがあってもキーを使わない。
    await tree.focus();
    await expect(tree).toBeFocused();
    const shown = (await current.locator("code").textContent()) ?? "";
    const before = (await status(shown).getAttribute("aria-label")) ?? "";
    await page.keyboard.press("ArrowDown");
    await page.keyboard.press("Space");
    await expect(current).toContainText(shown);
    await expect(status(shown)).toHaveAccessibleName(before);
    await expect(dialog).toHaveCount(1);
    await title.fill(t("D"));
    await title.press("Enter");
    await expect(dialog).toBeHidden();
    // 作ったアイテムの詳細パネルが開く。狭い幅ではツリーに重なるので、閉じる。
    await panel.getByRole("button", { name: "閉じる" }).click();
    // D は、A・B・C と同じ Epic の下の末尾に入る。
    await expect.poll(order).toEqual([epic, t("A"), t("B"), t("C"), t("D"), epic2]);
  });

  await test.step("Alt+↑↓ reorder, and quick presses are carried out one by one", async () => {
    await pick(tree, `${prefix}-4`);
    await page.keyboard.press("Alt+ArrowUp");
    await expect.poll(order).toEqual([epic, t("A"), t("C"), t("B"), t("D"), epic2]);
    await page.keyboard.press("Alt+ArrowDown");
    await page.keyboard.press("Alt+ArrowDown");
    await expect.poll(order).toEqual([epic, t("A"), t("B"), t("D"), t("C"), epic2]);
    await expect(current).toContainText(`${prefix}-4`);
  });

  await test.step("Alt+Shift+↓ moves under the next Epic, and opens it if it is closed", async () => {
    // D(-6)を、次に見える Epic(-5)の下へ。
    await pick(tree, `${prefix}-6`);
    await page.keyboard.press("Alt+Shift+ArrowDown");
    await expect(page.getByText(`${prefix}-5 の下に移動しました`)).toBeVisible();
    await expect.poll(order).toEqual([epic, t("A"), t("B"), t("C"), epic2, t("D")]);
    await expect(current).toContainText(`${prefix}-6`);
    // -5 を閉じてから、C(-4)を移すと、-5 が開いて C にフォーカスが付く。
    await pick(tree, `${prefix}-5`);
    await page.keyboard.press("ArrowLeft");
    await expect(tree.getByRole("link", { name: t("D") })).toHaveCount(0);
    await pick(tree, `${prefix}-4`);
    await page.keyboard.press("Alt+Shift+ArrowDown");
    await expect(tree.getByRole("link", { name: t("D") })).toBeVisible();
    await expect(current).toContainText(`${prefix}-4`);
    await expect.poll(order).toEqual([epic, t("A"), t("B"), epic2, t("D"), t("C")]);
  });
});
