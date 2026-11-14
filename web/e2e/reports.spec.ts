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
