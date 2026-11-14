import type { Page, TestInfo } from "@playwright/test";
import { expect, test } from "./fixtures";
import { tokenUrl } from "./helpers";
import { base, openWith, rowOf, seed, send, type } from "./search-table-helpers";

// 検索のテーブルの選択・選択バー・一括変更と一括削除・行末のメニューの通しのテスト(E2a。04「選択」)。
// 作ったアイテムのタイトルにテストごとの印を入れて絞り込み、ほかのテストのアイテムを選ばない。

const bar = (page: Page, n: number) => page.getByText(`${n} 件を選択中`, { exact: true });
const check = (page: Page, id: string) => page.getByRole("checkbox", { name: `${id} を選択` });
const menuOf = (page: Page, id: string) => page.getByRole("button", { name: `${id} のメニュー` });
const panelOf = (page: Page) => page.getByRole("region", { name: "詳細パネル" });
const paletteOf = (page: Page) => page.getByRole("dialog", { name: "コマンドパレット" });
const selectAll = (page: Page) => page.getByRole("checkbox", { name: "すべて選択" });

// seedThree は、seed の 2 件に、Epic の下の Story「{mark} C」(接頭辞-3)を足す。
async function seedThree(page: Page, info: TestInfo) {
  const s = await seed(page, info);
  await send(page, "post", `${base}/items`, {
    project: s.projectId,
    kind: "Story",
    parent: s.epicId,
    title: `${s.mark} C`,
  });
  return s;
}

test("a row shown under two groups is one selection", async ({ page }, info) => {
  const { mark, prefix, workspaceId, storyId } = await seed(page, info);
  const names = [`系A${prefix}`, `系B${prefix}`];
  for (const name of names) {
    const v = await send(page, "post", `${base}/workspaces/${workspaceId}/lists/systems/values`, {
      name,
    });
    await send(page, "post", `${base}/items/${storyId}/properties/systems/values/${v.id}`, {});
  }
  await openWith(page, info, mark);
  await page.getByRole("combobox", { name: "グループ" }).selectOption({ label: "関連システム" });
  for (const name of names) {
    await expect(page.getByRole("row", { name: `${name} 1`, exact: true })).toBeVisible();
  }
  const story = check(page, `${prefix}-2`);
  await expect(story).toHaveCount(2);
  await story.first().check();
  await expect(story.nth(0)).toBeChecked();
  await expect(story.nth(1)).toBeChecked();
  await expect(bar(page, 1)).toBeVisible();
  // 「すべて選択」も、2 つの見出しの下の行を 1 回だけ数える(Epic と Story の 2 件)。
  await story.first().uncheck();
  await selectAll(page).check();
  await expect(bar(page, 2)).toBeVisible();
  await selectAll(page).uncheck();
  await story.first().check();

  // 1 行だけ選んでいるときは、パレットの「選んでいるアイテム」になる(E2a-18)。
  await page.keyboard.press("ControlOrMeta+k");
  await expect(paletteOf(page).getByText(`${prefix}-2 に対して`)).toBeVisible();
});

test("bulk changes keep the selection, and bulk deletes clear it", async ({ page }, info) => {
  const { mark, prefix } = await seed(page, info);
  await openWith(page, info, mark);
  await check(page, `${prefix}-1`).check();
  await check(page, `${prefix}-2`).check();
  await expect(bar(page, 2)).toBeVisible();

  await page.getByRole("button", { name: "一括変更…" }).click();
  const dialog = page.getByRole("dialog", { name: "選択したアイテムを一括変更" });
  await dialog.getByLabel("項目").selectOption({ label: "優先度" });
  await dialog.getByLabel("値").selectOption({ label: "Highest" });
  await dialog.getByRole("button", { name: "優先度を変更" }).click();
  await page
    .getByRole("dialog", { name: "一括変更", exact: true })
    .getByRole("button", { name: "2 件を変更" })
    .click();
  await expect(page.getByText("2 件を変えました")).toBeVisible();
  await expect(rowOf(page, `${mark} A`)).toContainText("Highest");
  await expect(rowOf(page, `${mark} B`)).toContainText("Highest");
  await expect(bar(page, 2)).toBeVisible();

  await page.getByRole("button", { name: "一括削除…" }).click();
  await page
    .getByRole("dialog", { name: "一括削除" })
    .getByRole("button", { name: "2 件を削除" })
    .click();
  await expect(page.getByText("2 件を削除しました")).toBeVisible();
  await expect(rowOf(page, mark)).toHaveCount(0);
  await expect(bar(page, 2)).toHaveCount(0);
});

test("select all picks every row, and removes a row deleted elsewhere", async ({ page }, info) => {
  const { mark, prefix } = await seedThree(page, info);
  await openWith(page, info, mark);
  await selectAll(page).check();
  await expect(bar(page, 3)).toBeVisible();

  // 別のタブで 1 件(接頭辞-3)を削除すると、Store の変更の知らせで読み直し、選択から外す。
  const other = await page.context().newPage();
  try {
    await other.goto(tokenUrl(`/s/e2e/i/${prefix}-3`));
    await panelOf(other).getByRole("button", { name: "メニュー", exact: true }).click();
    await other.getByRole("menuitem", { name: /^削除…/ }).click();
    await other
      .getByRole("dialog", { name: "Story を削除" })
      .getByRole("button", { name: "削除", exact: true })
      .click();
    await expect(other.getByText(`${prefix}-3 を削除しました`, { exact: true })).toBeVisible();
  } finally {
    await other.close();
  }
  await expect(bar(page, 2)).toBeVisible();
  await expect(selectAll(page)).toBeChecked();

  await selectAll(page).uncheck();
  await expect(bar(page, 2)).toHaveCount(0);
});

test("a deleted row cannot be checked or open its menu", async ({ page }, info) => {
  const { mark, prefix, storyId } = await seed(page, info);
  await send(page, "delete", `${base}/items/${storyId}`, undefined);
  await openWith(page, info, mark);
  await type(page, `is:deleted ${mark}`);
  await expect(check(page, `${prefix}-2`)).toBeDisabled();
  await expect(menuOf(page, `${prefix}-2`)).toBeDisabled();
});

test("the row menu opens the details and deletes the row", async ({ page }, info) => {
  const { mark, prefix } = await seed(page, info);
  await openWith(page, info, mark);
  await menuOf(page, `${prefix}-2`).click();
  await page.getByRole("menuitem", { name: /^詳細を開く/ }).click();
  await expect(panelOf(page).getByRole("textbox", { name: "タイトル" })).toHaveValue(`${mark} B`);
  // 詳細パネルはテーブルの右端(行末の「⋯」)に重なるので、閉じてから押す。
  await panelOf(page).getByRole("button", { name: "閉じる" }).click();
  await expect(panelOf(page)).toBeHidden();

  await menuOf(page, `${prefix}-2`).click();
  await page.getByRole("menuitem", { name: /^削除…/ }).click();
  await page
    .getByRole("dialog", { name: "Story を削除" })
    .getByRole("button", { name: "削除", exact: true })
    .click();
  await expect(rowOf(page, `${mark} B`)).toHaveCount(0);
  await expect(rowOf(page, `${mark} A`)).toBeVisible();
});

test("the parent cell opens the parent's details", async ({ page }, info) => {
  const { mark, prefix } = await seed(page, info);
  await openWith(page, info, mark);
  await page.getByRole("button", { name: "表示する列", exact: true }).click();
  await page
    .getByRole("menu", { name: "表示する列" })
    .getByRole("menuitemcheckbox", { name: "親" })
    .click();
  await page.keyboard.press("Escape");
  await rowOf(page, `${mark} B`)
    .getByRole("button", { name: `${prefix}-1`, exact: true })
    .click();
  await expect(panelOf(page).getByRole("textbox", { name: "タイトル" })).toHaveValue(`${mark} A`);
});

test("rows only from a read-only store cannot be bulk changed or deleted", async ({
  page,
}, info) => {
  await openWith(page, info, "store:readonly ログイン画面");
  await check(page, "HH-2").check();
  await expect(bar(page, 1)).toBeVisible();
  for (const name of ["一括変更…", "一括削除…"]) {
    const button = page.getByRole("button", { name });
    await expect(button).toBeDisabled();
    await expect(button).toHaveAttribute("title", "閲覧のみの Store です");
  }
});
