import { expect, type Locator, type Page, test } from "@playwright/test";
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

// ツリーの行のメニューから行う、並べ替え・複製・移動・種類の変更・削除の通しのテスト(A4b)。
// サーバーは 3 つの幅で共有で、Store が大きいと sync.spec の時間の窓が足りなくなるので、
// ワークスペースを 1 つだけ作り、1 つのテストの中で順に確かめる(足すアイテムは 7 件)。サンプルは書き換えない。

// 行のメニューのボタン(読み上げの名前は「{ID} のメニュー」。詳細パネルの「メニュー」と取り違えない)。
const menuOf = (row: Locator) => row.getByRole("button", { name: / のメニュー$/ });
const entry = (page: Page, name: RegExp) => page.getByRole("menuitem", { name });

async function openMenu(page: Page, row: Locator) {
  await menuOf(row).click();
  await expect(page.getByRole("menu")).toBeVisible();
}

async function closeMenu(page: Page) {
  await page.keyboard.press("Escape");
  await expect(page.getByRole("menu")).toHaveCount(0);
}

test("moves, duplicates, changes the kind of and deletes items from the tree's row menu", async ({
  page,
}, info) => {
  const t = (s: string) => unique(info, s);
  const prefix = prefixOf(info);
  await createWorkspace(page, info, unique(info, "操作"), prefix);
  // Epic -1 の下に Task A(-2。Step「手順」を -3 に持つ)・B(-4)・C(-5)、もう 1 つの Epic(-6)。
  await projectWithEpic(page, info, prefix);
  const panel = await taskUnderEpic(page, info, `${prefix}-1`, "A");
  const step = panel.getByRole("textbox", { name: "Step を追加" });
  await step.fill(t("手順"));
  await step.press("Enter");
  await expect(panel.getByText(t("手順"))).toBeVisible();
  await taskUnderEpic(page, info, `${prefix}-1`, "B");
  await taskUnderEpic(page, info, `${prefix}-1`, "C");
  await addEpic(page, info, prefix, "別の Epic");
  const tree = await treeOf(page, prefix);
  const [epic, epic2] = [t("通知"), t("別の Epic")];
  await expect
    .poll(() => treeTitles(tree))
    .toEqual([epic, t("A"), t("手順"), t("B"), t("C"), epic2]);

  await test.step("disables the items that do not apply, with the reason", async () => {
    await openMenu(page, treeRow(tree, `${prefix}-1`));
    await expect(entry(page, /^上へ移動/)).toBeDisabled();
    await expect(entry(page, /^上へ移動/)).toContainText("先頭です");
    await expect(entry(page, /^種類を変更/)).toBeDisabled();
    await expect(entry(page, /^種類を変更/)).toContainText("Epic の種類は変更できません");
    await expect(entry(page, /^ID をコピー/)).toBeEnabled();
    await closeMenu(page);
    await openMenu(page, treeRow(tree, `${prefix}-2`));
    await expect(entry(page, /^上へ移動/)).toContainText("先頭です");
    await closeMenu(page);
    await openMenu(page, treeRow(tree, `${prefix}-5`));
    await expect(entry(page, /^下へ移動/)).toBeDisabled();
    await expect(entry(page, /^下へ移動/)).toContainText("末尾です");
    await closeMenu(page);
  });

  await test.step("moves a row up and down among its siblings", async () => {
    await openMenu(page, treeRow(tree, `${prefix}-4`));
    await entry(page, /^上へ移動/).click();
    await expect
      .poll(() => treeTitles(tree))
      .toEqual([epic, t("B"), t("A"), t("手順"), t("C"), epic2]);
    await openMenu(page, treeRow(tree, `${prefix}-4`));
    await entry(page, /^下へ移動/).click();
    await expect
      .poll(() => treeTitles(tree))
      .toEqual([epic, t("A"), t("手順"), t("B"), t("C"), epic2]);
  });

  await test.step("duplicates a row at the end of its siblings", async () => {
    await openMenu(page, treeRow(tree, `${prefix}-4`));
    await entry(page, /^複製/).click();
    // トーストは出ず、詳細パネルが新しいアイテムに切り替わる。狭い幅ではツリーに重なるので、閉じる。
    const copy = page.getByRole("region", { name: "詳細パネル" });
    await expect(copy.getByRole("textbox", { name: "タイトル" })).toHaveValue(`${t("B")} のコピー`);
    await copy.getByRole("button", { name: "閉じる" }).click();
    await expect
      .poll(() => treeTitles(tree))
      .toEqual([epic, t("A"), t("手順"), t("B"), t("C"), `${t("B")} のコピー`, epic2]);
  });

  await test.step("moves a row under another Epic from the move menu", async () => {
    await openMenu(page, treeRow(tree, `${prefix}-5`));
    await entry(page, /^移動…/).click();
    await page.getByRole("textbox", { name: "移動先を検索" }).fill(epic2);
    await page.getByRole("menuitem", { name: new RegExp(`^${prefix}-6 `) }).click();
    await expect(page.getByText(`${prefix}-6 の下に移動しました`)).toBeVisible();
    await expect
      .poll(() => treeTitles(tree))
      .toEqual([epic, t("A"), t("手順"), t("B"), `${t("B")} のコピー`, epic2, t("C")]);
  });

  await test.step("changes the kind after a confirmation", async () => {
    await openMenu(page, treeRow(tree, `${prefix}-4`));
    await entry(page, /^種類を変更/).click();
    await page.getByRole("menuitem", { name: "Bug", exact: true }).click();
    const dialog = page.getByRole("dialog", { name: "種類を変更" });
    await expect(
      dialog.getByText(`${prefix}-4 の種類を Task から Bug に変更します。`),
    ).toBeVisible();
    await dialog.getByRole("button", { name: "変更", exact: true }).click();
    await expect(treeRow(tree, `${prefix}-4`).getByRole("img", { name: "Bug" })).toBeVisible();
  });

  await test.step("deletes a row together with what is below it", async () => {
    await openMenu(page, treeRow(tree, `${prefix}-2`));
    await entry(page, /^削除…/).click();
    const dialog = page.getByRole("dialog", { name: "Task を削除" });
    await expect(dialog.getByText("下にある 1 件(Step)も一緒に削除されます。")).toBeVisible();
    await dialog.getByRole("button", { name: "削除", exact: true }).click();
    await expect(page.getByText(`${prefix}-2 を削除しました`)).toBeVisible();
    await expect(tree.getByRole("link", { name: t("A"), exact: true })).toHaveCount(0);
    await expect(tree.getByRole("link", { name: t("手順") })).toHaveCount(0);
  });

  await test.step("deletes a Step from the × in the panel", async () => {
    await tree.getByRole("link", { name: t("C"), exact: true }).click();
    const panel = page.getByRole("region", { name: "詳細パネル" });
    await expect(panel.getByRole("textbox", { name: "タイトル" })).toHaveValue(t("C"));
    const input = panel.getByRole("textbox", { name: "Step を追加" });
    await input.fill(t("消す手順"));
    await input.press("Enter");
    await expect(panel.getByText(t("消す手順"))).toBeVisible();
    await panel.getByRole("button", { name: "Step を削除" }).click();
    const dialog = page.getByRole("dialog", { name: "Step を削除" });
    await dialog.getByRole("button", { name: "削除", exact: true }).click();
    await expect(page.getByText(new RegExp(`^${prefix}-\\d+ を削除しました$`))).toBeVisible();
    await expect(panel.getByText(t("消す手順"))).toHaveCount(0);
  });
});
