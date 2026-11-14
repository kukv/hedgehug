import { expect, test } from "./fixtures";
import { tokenUrl } from "./helpers";
import { startedSprint } from "./reports-helpers";

// 累積フロー図の期間の通しのテスト(F3)。

test("reports flow period", async ({ page }, info) => {
  const prefix = await startedSprint(page, info);
  await page.goto(tokenUrl(`/s/e2e/w/${prefix}/reports`));
  const flow = page.getByRole("region", { name: "累積フロー図" });
  const start = flow.getByLabel("開始", { exact: true });
  const end = flow.getByLabel("終了", { exact: true });
  const reset = flow.getByRole("button", { name: "直近 21 日に戻す" });
  await expect(flow.getByText("直近 21 日・件数")).toBeVisible();
  await expect(reset).toHaveCount(0);

  // 合う期間: 見出しの横が期間になり、「直近 21 日に戻す」が出る。戻すと初めの期間に戻る。
  await start.fill("2020-01-01");
  await expect(flow.getByText(/〜.*・件数/)).toBeVisible();
  await expect(flow.getByText("直近 21 日・件数")).toHaveCount(0);
  await reset.click();
  await expect(flow.getByText("直近 21 日・件数")).toBeVisible();
  await expect(reset).toHaveCount(0);
  await expect(start).not.toHaveValue("2020-01-01");

  // 合わない期間: 入力と理由が残り、図は前のまま。分類・ステータスを切り替えても消えない。
  await end.fill("2999-01-01");
  const reason = flow.getByText("終了日は今日以前にしてください");
  await expect(reason).toBeVisible();
  await expect(flow.getByText("直近 21 日・件数")).toBeVisible();
  await expect(flow.getByRole("img", { name: "累積フロー図" })).toBeVisible();
  await flow.getByRole("button", { name: "ステータス" }).click();
  await expect(flow.getByRole("button", { name: "ステータス" })).toHaveAttribute(
    "aria-pressed",
    "true",
  );
  await expect(reason).toBeVisible();
  await expect(end).toHaveValue("2999-01-01");
  await flow.getByRole("button", { name: "分類" }).click();
  await expect(reason).toBeVisible();
  await expect(end).toHaveValue("2999-01-01");
});
