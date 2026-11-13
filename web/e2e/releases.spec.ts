import { expect, test } from "./fixtures";
import {
  createWorkspace,
  prefixOf,
  projectWithEpic,
  saved,
  storyUnderEpic,
  tokenUrl,
  unique,
} from "./helpers";

// リリースの通しのテスト(F1)。ワークスペースは毎回作る(サンプルは同じ worker のほかのテストが使う)。

test("releases with an unfinished item and archives the release", async ({ page }, info) => {
  const prefix = prefixOf(info);
  await createWorkspace(page, info, unique(info, "出荷"), prefix);
  // プロジェクト(接頭辞-P1)、Epic(接頭辞-1)、Story 2 つ(接頭辞-2・接頭辞-3)。
  await projectWithEpic(page, info, prefix);
  await storyUnderEpic(page, info, `${prefix}-1`, "書き出し");
  await storyUnderEpic(page, info, `${prefix}-1`, "読み込み");

  const releasesTab = tokenUrl(`/s/e2e/w/${prefix}/releases`);
  await page.goto(releasesTab);
  await page.getByRole("button", { name: "リリースを作成" }).click();
  const create = page.getByRole("dialog", { name: "リリースを作成" });
  await create.getByRole("textbox", { name: "名前" }).fill("v2.1.0");
  await create.getByRole("button", { name: "作成", exact: true }).click();
  await expect(create).toBeHidden();
  await expect(page.getByText("v2.1.0 を作成しました")).toBeVisible();

  // 1 つ目の Story だけを v2.1.0 に入れる。
  const panel = page.getByRole("region", { name: "詳細パネル" });
  const releaseSelect = panel.getByRole("combobox", { name: "リリース" });
  await page.goto(tokenUrl(`/s/e2e/i/${prefix}-2`));
  const assigned = saved(page, "/release");
  await releaseSelect.selectOption({ label: "v2.1.0" });
  await assigned;

  // 未完了が残るので、リリース済みにする前に確認する。予定日がなければ今日が入る。
  await page.goto(releasesTab);
  const card = page.getByRole("article", { name: "v2.1.0" });
  await expect(card.getByText("アイテム 1 件")).toBeVisible();
  await card.getByRole("button", { name: "リリース済みにする" }).click();
  const confirm = page.getByRole("dialog", { name: "リリース済みにする" });
  await expect(
    confirm.getByText(
      "v2.1.0 には未完了のアイテムが 1 件あります。このままリリース済みにしますか?",
    ),
  ).toBeVisible();
  await confirm.getByRole("button", { name: "リリース済みにする" }).click();
  await expect(confirm).toBeHidden();
  await expect(page.getByText("v2.1.0 をリリース済みにしました")).toBeVisible();
  const now = new Date();
  await expect(card.getByText("リリース済み", { exact: true })).toBeVisible();
  await expect(card.getByText(`${now.getMonth() + 1}/${now.getDate()}`)).toBeVisible();
  await expect(card.getByRole("button", { name: "リリース済みにする" })).toHaveCount(0);

  // アーカイブにすると、入っている Story の選択肢には残り、ほかの Story の選択肢には出ない。
  await card.getByRole("button", { name: "v2.1.0 のメニュー" }).click();
  await page.getByRole("menuitem", { name: "アーカイブにする" }).click();
  await expect(page.getByText("v2.1.0 をアーカイブにしました")).toBeVisible();
  await expect(card.getByText("アーカイブ", { exact: true })).toBeVisible();

  await page.goto(tokenUrl(`/s/e2e/i/${prefix}-2`));
  await expect(releaseSelect.locator("option")).toHaveText(["なし", "v2.1.0"]);
  await expect(releaseSelect.locator("option:checked")).toHaveText("v2.1.0");
  await page.goto(tokenUrl(`/s/e2e/i/${prefix}-3`));
  await expect(releaseSelect.locator("option")).toHaveText(["なし"]);
});
