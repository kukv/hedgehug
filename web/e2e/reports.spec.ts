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
import { startedSprint } from "./reports-helpers";

// レポートの通しのテスト(F3)。ワークスペースは毎回作る。

test("reports burndown and velocity", async ({ page }, info) => {
  const prefix = await startedSprint(page, info);
  await page.goto(tokenUrl(`/s/e2e/w/${prefix}/reports`));
  await expect(page.getByText("git 履歴から集計しています。")).toBeVisible();
  await expect(page.getByText("総量 5 pt")).toBeVisible();
  await expect(page.getByText("残り 2 pt")).toBeVisible();

  const velocity = page.getByRole("region", { name: "ベロシティ" });
  await velocity.getByText("表で見る").click();
  const row = velocity.getByRole("row", { name: /Sprint 1/ });
  await expect(row.getByRole("cell").nth(1)).toHaveText("5");
  await expect(row.getByRole("cell").nth(2)).toHaveText("3");
  await expect(page.getByText("完了した直近 3 スプリントの平均 —")).toBeVisible();

  const savedUnit = saved(page, "/reports/selection");
  await page.getByRole("combobox", { name: "単位" }).selectOption({ label: "件数" });
  await savedUnit;
  await expect(page.getByText("総量 2 件")).toBeVisible();
  await page.reload();
  await expect(page.getByRole("combobox", { name: "単位" }).locator("option:checked")).toHaveText(
    "件数",
  );
});

test("points fallback", async ({ page }, info) => {
  const prefix = prefixOf(info);
  await createWorkspace(page, info, unique(info, "件数"), prefix);
  await projectWithEpic(page, info, prefix);
  await storyUnderEpic(page, info, `${prefix}-1`, "A");
  await page.goto(tokenUrl(`/s/e2e/w/${prefix}/reports`));
  await expect(
    page.getByText("ポイントのあるアイテムがないため、件数で数えています"),
  ).toBeVisible();
});

test("reports flow, cycle and estimates", async ({ page }, info) => {
  const prefix = await startedSprint(page, info);
  // A に見積もり 2 時間と作業時間 1.5 時間を記録する。
  await page.goto(tokenUrl(`/s/e2e/i/${prefix}-2`));
  const panel = page.getByRole("region", { name: "詳細パネル" });
  const estimate = panel.getByLabel("見積もり時間");
  const estimated = saved(page, "/properties");
  await estimate.fill("2");
  await estimate.press("Enter");
  await estimated;
  const worklog = panel.getByRole("region", { name: "作業時間", exact: true });
  await worklog.getByLabel("時間", { exact: true }).fill("1.5");
  await worklog.getByRole("button", { name: "記録" }).click();
  await expect(worklog.getByRole("button", { name: "作業時間を削除" })).toHaveCount(1);

  await page.goto(tokenUrl(`/s/e2e/w/${prefix}/reports`));
  const flow = page.getByRole("region", { name: "累積フロー図" });
  await expect(flow.getByText("直近 21 日・件数")).toBeVisible();
  await flow.getByLabel("終了", { exact: true }).fill("2999-01-01");
  await expect(flow.getByText("終了日は今日以前にしてください")).toBeVisible();
  await flow.getByRole("button", { name: "ステータス" }).click();
  await expect(flow.getByText("Done", { exact: true }).first()).toBeVisible();

  await expect(page.getByText("着手→完了・1 件")).toBeVisible();

  const table = page.getByRole("region", { name: "見積もりと実績" });
  const a = table.getByRole("row", { name: new RegExp(`${prefix}-2`) });
  await expect(a.getByRole("cell").nth(4)).toHaveText("-0.5 時間");
  await expect(table.getByRole("row", { name: /合計/ })).toBeVisible();

  await table.getByRole("combobox", { name: "対象" }).selectOption({ label: "Sprint 1" });
  await expect(
    page.getByText(
      "作業時間は、アイテムに付いた記録のすべてです(別のスプリントにいたあいだの記録も含みます)。",
    ),
  ).toBeVisible();
});

test("sprint card and overview", async ({ page }, info) => {
  const prefix = await startedSprint(page, info);
  // 計画中の Sprint 2 を作る(「バーンダウンを見る」が無効なことを確かめる)。
  await page.goto(tokenUrl(`/s/e2e/w/${prefix}/sprints`));
  await page.getByRole("button", { name: "スプリントを作成" }).first().click();
  const create = page.getByRole("dialog", { name: "スプリントを作成" });
  await create.getByRole("button", { name: "作成", exact: true }).click();
  await expect(create).toBeHidden();

  const s1 = page.getByRole("article", { name: "Sprint 1" });
  await s1.getByText("表で見る").click();
  // 今日より後の行は残りが「—」なので、「—」のない最後の行が今日の行。
  const today = s1
    .getByRole("table", { name: "バーンダウンの表" })
    .getByRole("row")
    .filter({ hasNotText: "—" })
    .last();
  await expect(today.getByRole("cell").nth(2)).toHaveText("2");

  const planned = page.getByRole("article", { name: "Sprint 2" });
  await planned.getByRole("button", { name: "Sprint 2 のメニュー" }).click();
  await expect(page.getByRole("menuitem", { name: "バーンダウンを見る" })).toBeDisabled();
  await page.keyboard.press("Escape");

  await s1.getByRole("button", { name: "Sprint 1 のメニュー" }).click();
  await page.getByRole("menuitem", { name: "バーンダウンを見る" }).click();
  await expect(page).toHaveURL(new RegExp(`/w/${prefix}/reports$`));
  await expect(
    page.getByRole("combobox", { name: "スプリントを選択" }).locator("option:checked"),
  ).toHaveText("Sprint 1");

  await page.goto(tokenUrl(`/s/e2e/w/${prefix}`));
  await expect(page.getByText("Sprint 1 の完了ポイント")).toBeVisible();
  await expect(page.getByText("3 / 5 pt")).toBeVisible();
});
