import { expect, test } from "./fixtures";
import { createWorkspace, newTaskUnder, prefixOf, tokenUrl, unique } from "./helpers";

// 作業時間の通しのテスト(A4a)。関連は relations.spec.ts が通す。

test("records, checks and deletes work time", async ({ page }, info) => {
  const prefix = prefixOf(info);
  await createWorkspace(page, info, unique(info, "作業時間"), prefix);
  const panel = await newTaskUnder(page, info, prefix);
  const section = panel.getByRole("region", { name: "作業時間", exact: true });
  const hours = section.getByLabel("時間", { exact: true });
  const memo = section.getByLabel("メモ", { exact: true });
  const record = section.getByRole("button", { name: "記録" });
  const del = section.getByRole("button", { name: "作業時間を削除" });

  // 0 以下・空は「時間を入力してください」、小数第 3 位以下は別の文言。どちらも記録しない。
  await hours.fill("0");
  await record.click();
  await expect(section.getByRole("alert")).toHaveText("時間を入力してください");
  await hours.fill("0.123");
  await record.click();
  await expect(section.getByRole("alert")).toHaveText("時間は小数第 2 位までで入力してください");
  await expect(del).toHaveCount(0);

  // 記録する。行には「N 時間」とメモ、見出しには合計。メモ欄の Enter でも記録する。
  await hours.fill("1.25");
  await memo.fill("調査");
  await record.click();
  await expect(del).toHaveCount(1);
  await expect(section).toContainText("1.25 時間");
  await expect(section).toContainText("調査");
  await expect(section.getByRole("alert")).toHaveCount(0);
  await hours.fill("0.75");
  await memo.press("Enter");
  await expect(del).toHaveCount(2);
  await expect(section.getByRole("heading")).toContainText("2 時間");

  // 削除は確認してから。古い順なので、先頭が 1.25 時間の記録。
  await del.first().click();
  const dialog = page.getByRole("dialog", { name: "作業時間を削除" });
  await expect(dialog).toContainText("1.25 時間");
  await dialog.getByRole("button", { name: "削除", exact: true }).click();
  await expect(page.getByText("作業時間を削除しました")).toBeVisible();
  await expect(del).toHaveCount(1);
  await expect(section.getByRole("heading")).toContainText("0.75 時間");

  // 読み直しても残る。
  await page.goto(tokenUrl(`/s/e2e/i/${prefix}-2`));
  await expect(
    page
      .getByRole("region", { name: "詳細パネル" })
      .getByRole("region", { name: "作業時間", exact: true })
      .getByRole("button", { name: "作業時間を削除" }),
  ).toHaveCount(1);
});
