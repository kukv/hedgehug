import { expect, test } from "./fixtures";
import { createWorkspace, prefixOf, projectWithEpic, sidebar, tokenUrl, unique } from "./helpers";

// ワークスペースの削除の通しのテスト(B5b)。ワークスペースは毎回作る(サンプルの「Web」は削除しない)。

const nameLabel = "確認のため、ワークスペースの名前を入力してください";

test("deletes a workspace from the general settings after typing its name", async ({
  page,
}, info) => {
  const prefix = prefixOf(info);
  const name = unique(info, "削除");
  await createWorkspace(page, info, name, prefix);
  await projectWithEpic(page, info, prefix);

  await page.goto(tokenUrl(`/s/e2e/w/${prefix}/settings`));
  await page.getByRole("button", { name: "ワークスペースを削除…", exact: true }).click();
  const dialog = page.getByRole("dialog", { name: "ワークスペースを削除" });
  await expect(
    dialog.getByText(
      `「${name}」と、その中の 1 件のプロジェクト・1 件のアイテム、スプリント・リリース・設定を削除します。`,
    ),
  ).toBeVisible();

  // 違う名前では削除されず、ダイアログは開いたまま。
  const input = dialog.getByRole("textbox", { name: nameLabel });
  await input.fill("違う名前");
  await dialog.getByRole("button", { name: "削除", exact: true }).click();
  await expect(dialog.getByRole("alert")).toHaveText("名前が一致しません");
  await expect(dialog).toBeVisible();

  await input.fill(name);
  await expect(dialog.getByRole("alert")).toHaveCount(0);
  await dialog.getByRole("button", { name: "削除", exact: true }).click();

  // トーストが出て、ホームへ移り、サイドバーからワークスペースが消える。
  await expect(page.getByText(`「${name}」を削除しました`)).toBeVisible();
  await expect(page.getByRole("heading", { name: "hedgehug" })).toBeVisible();
  const side = await sidebar(page, info);
  await expect(side.getByRole("link", { name, exact: true })).toHaveCount(0);

  // 削除したワークスペースのアドレスはホームを開く。同じ名前と接頭辞で作り直せる。
  await page.goto(tokenUrl(`/s/e2e/w/${prefix}`));
  await expect(page.getByRole("heading", { name: "hedgehug" })).toBeVisible();
  await createWorkspace(page, info, name, prefix);
});

test("opens the same dialog from the sidebar menu", async ({ page }, info) => {
  const prefix = prefixOf(info);
  const name = unique(info, "メニュー削除");
  await createWorkspace(page, info, name, prefix);

  await page.goto(tokenUrl());
  const side = await sidebar(page, info);
  await side
    .getByRole("region", { name: "e2e" })
    .locator(".ws-row")
    .filter({ has: page.getByRole("link", { name, exact: true }) })
    .getByRole("button", { name: "ワークスペースのメニュー" })
    .click();
  await page.getByRole("menuitem", { name: "削除…" }).click();
  const dialog = page.getByRole("dialog", { name: "ワークスペースを削除" });
  await expect(dialog.getByRole("textbox", { name: nameLabel })).toBeVisible();
  await dialog.getByRole("button", { name: "キャンセル" }).click();
  await expect(dialog).toBeHidden();

  await page.goto(tokenUrl(`/s/e2e/w/${prefix}`));
  await expect(page.getByRole("heading", { name })).toBeVisible();
});

test("disables deleting in a read-only store", async ({ page }, info) => {
  await page.goto(tokenUrl("/s/readonly/w/HH"));
  const side = await sidebar(page, info);
  await side
    .getByRole("region", { name: "readonly" })
    .getByRole("button", { name: "ワークスペースのメニュー" })
    .first()
    .click();
  await expect(page.getByRole("menuitem", { name: "削除…" })).toBeDisabled();
});

test("a second page that kept the settings open sees a save failure, then the home after a reload", async ({
  page,
  context,
}, info) => {
  const prefix = prefixOf(info);
  const name = unique(info, "別ページ削除");
  await createWorkspace(page, info, name, prefix);

  const other = await context.newPage();
  await other.goto(tokenUrl(`/s/e2e/w/${prefix}/settings`));
  const otherName = other.getByRole("textbox", { name: "名前", exact: true });
  await expect(otherName).toHaveValue(name);

  await page.goto(tokenUrl(`/s/e2e/w/${prefix}/settings`));
  await page.getByRole("button", { name: "ワークスペースを削除…", exact: true }).click();
  const dialog = page.getByRole("dialog", { name: "ワークスペースを削除" });
  await dialog.getByRole("textbox", { name: nameLabel }).fill(name);
  await dialog.getByRole("button", { name: "削除", exact: true }).click();
  await expect(page.getByText(`「${name}」を削除しました`)).toBeVisible();

  // 2 つ目のページで名前を変えて保存すると、エラーのトーストが出る。画面は壊れない。
  await other.bringToFront();
  await otherName.fill(`${name} 変更`);
  await otherName.press("Enter");
  await expect(other.getByText("保存できませんでした")).toBeVisible();

  await other.reload();
  await expect(other.getByRole("heading", { name: "hedgehug" })).toBeVisible();
});
