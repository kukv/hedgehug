import { expect, test } from "./fixtures";
import { createWorkspace, newProject, newTaskUnder, prefixOf, tokenUrl, unique } from "./helpers";

// 別のタブで削除されたものに、このタブから説明を保存したときの通しのテスト(C3a-screen)。
// このタブは説明を編集しているあいだ(06「反映を待つとき」)なので、別のタブの削除が反映されず、画面が開いたまま保存できる。

const restoredToast = (id: string) =>
  `${id} は別の画面で削除されていましたが、変更を保存したため、削除を取り消しました`;

test("saving a description of an item deleted in another tab restores it", async ({
  page,
}, info) => {
  const prefix = prefixOf(info);
  await createWorkspace(page, info, unique(info, "別で削除 アイテム"), prefix);
  const panel = await newTaskUnder(page, info, prefix);
  const section = panel.getByRole("region", { name: "説明", exact: true });
  await section.getByRole("button", { name: "説明を追加" }).click();
  const edit = section.getByRole("textbox", { name: "説明" });
  await edit.fill("別のタブで削除されたあとの説明");

  // 別のタブで、ツリーの行のメニューからアイテム(接頭辞-2)を削除する。
  const other = await page.context().newPage();
  await other.goto(tokenUrl(`/s/e2e/p/${prefix}-P1/tree`));
  const tree = other.getByRole("list", { name: "ツリー" });
  await tree
    .getByRole("listitem")
    .filter({ has: other.getByText(`${prefix}-2`, { exact: true }) })
    .getByRole("button", { name: / のメニュー$/ })
    .click();
  await other.getByRole("menuitem", { name: /^削除…/ }).click();
  const dialog = other.getByRole("dialog", { name: "Task を削除" });
  await dialog.getByRole("button", { name: "削除", exact: true }).click();
  await expect(other.getByText(`${prefix}-2 を削除しました`)).toBeVisible();

  // このタブで保存すると、削除が取り消され、トーストが出る。
  await edit.press("Control+Enter");
  await expect(page.getByText(restoredToast(`${prefix}-2`))).toBeVisible();
  await expect(section).toContainText("別のタブで削除されたあとの説明");

  // 別のタブでアイテムのアドレスを開き直すと、詳細パネルが開く(削除済みではない)。
  await other.goto(tokenUrl(`/s/e2e/i/${prefix}-2`));
  await expect(
    other
      .getByRole("region", { name: "詳細パネル" })
      .getByRole("region", { name: "説明", exact: true }),
  ).toContainText("別のタブで削除されたあとの説明");
  await other.close();
});

test("saving a description of a project deleted in another tab restores it", async ({
  page,
}, info) => {
  const prefix = prefixOf(info);
  await createWorkspace(page, info, unique(info, "別で削除 プロジェクト"), prefix);
  await newProject(page, info, prefix);
  const section = page.getByRole("region", { name: "説明", exact: true });
  await section.getByRole("button", { name: "説明を追加" }).click();
  const edit = section.getByRole("textbox", { name: "説明" });
  await edit.fill("別のタブで削除されたあとの概要");

  // 別のタブで、設定のタブからプロジェクトを削除する。
  const other = await page.context().newPage();
  await other.goto(tokenUrl(`/s/e2e/p/${prefix}-P1/settings`));
  await other.getByRole("button", { name: "プロジェクトを削除", exact: true }).click();
  await other
    .getByRole("dialog", { name: "プロジェクトを削除" })
    .getByRole("button", { name: "削除", exact: true })
    .click();
  await expect(other.getByText("プロジェクトを削除しました")).toBeVisible();

  await edit.press("Control+Enter");
  await expect(page.getByText(restoredToast(`${prefix}-P1`))).toBeVisible();
  await expect(section).toContainText("別のタブで削除されたあとの概要");

  // 取り消したあとは、別のタブでプロジェクトの概要が開ける。
  await other.goto(tokenUrl(`/s/e2e/p/${prefix}-P1/overview`));
  await expect(other.getByRole("heading", { name: new RegExp(`^${prefix}-P1 `) })).toBeVisible();
  await other.close();
});
