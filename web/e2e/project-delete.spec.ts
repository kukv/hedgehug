import { expect, test } from "./fixtures";
import { createWorkspace, prefixOf, projectWithEpic, sidebar, tokenUrl, unique } from "./helpers";

// プロジェクトの設定タブからの削除の通しのテスト(A4b)。ワークスペースは毎回作る(サンプルは同じ worker のほかのテストが使う)。

test("deletes a project from its settings tab and goes back to the workspace", async ({
  page,
}, info) => {
  const prefix = prefixOf(info);
  await createWorkspace(page, info, unique(info, "削除"), prefix);
  // プロジェクト(接頭辞-P1)と Epic(接頭辞-1)。削除されていないアイテムが 1 件ある。
  await projectWithEpic(page, info, prefix);
  const projectTitle = unique(info, `計画 ${prefix} 1`);
  await page.goto(tokenUrl(`/s/e2e/w/${prefix}`));
  await expect((await sidebar(page, info)).getByRole("link", { name: projectTitle })).toBeVisible();

  await page.goto(tokenUrl(`/s/e2e/p/${prefix}-P1/settings`));
  await page.getByRole("button", { name: "プロジェクトを削除", exact: true }).click();
  const dialog = page.getByRole("dialog", { name: "プロジェクトを削除" });
  await expect(
    dialog.getByText(`「${projectTitle}」と、その下の 1 件のアイテムを削除します。`),
  ).toBeVisible();
  await dialog.getByRole("button", { name: "削除", exact: true }).click();

  // トーストが出て、ワークスペースの概要へ移り、サイドバーからプロジェクトが消える。
  await expect(page.getByText("プロジェクトを削除しました")).toBeVisible();
  await expect(page).toHaveURL(new RegExp(`/s/e2e/w/${prefix}(\\/|$)`));
  const side = await sidebar(page, info);
  await expect(side.getByRole("link", { name: projectTitle })).toHaveCount(0);
  // 削除したプロジェクトのアドレスは開けず、ホーム(見出し「hedgehug」)を開く(01「画面のアドレス」)。
  await page.goto(tokenUrl(`/s/e2e/p/${prefix}-P1/overview`));
  await expect(page.getByRole("heading", { name: "hedgehug" })).toBeVisible();
  await expect(page.getByRole("heading", { name: new RegExp(`^${prefix}-P1 `) })).toHaveCount(0);
});
