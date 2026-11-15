import { expect, test } from "./fixtures";
import { tokenUrl } from "./helpers";
import {
  dayFrom,
  filterBy,
  groupHeadings,
  markOf,
  myTask,
  openMyTasks,
  resetMyTasks,
  shownDay,
} from "./my-tasks-helpers";

// 日付が変わったときの更新の通しのテスト(E4a-16)。ブラウザーの時計だけを 24 時間進め、終了日が今日の Task が
// 期限切れに替わるのを見る。時計は開く前に入れる(入れたあとに読んだページの時刻が、進める時計に従う)。

test.beforeEach(async ({ page }) => {
  await resetMyTasks(page);
});

test("my tasks regroup when the day changes", async ({ page }, info) => {
  const mark = markOf(info);
  await myTask(page, `${mark} 今日`, { end: dayFrom(0) });

  await page.clock.install({ time: new Date() });
  await openMyTasks(page);
  await filterBy(page, mark);
  await expect(groupHeadings(page)).toHaveText(["今日 1"]);

  // 日付が替わると store を積み、マイタスクを読み直す(E4a-16)。入力欄にフォーカスがあると読み直しを待つので、外す。
  await page.getByRole("combobox", { name: "絞り込み" }).blur();
  const reloaded = page.waitForRequest(
    (r) => r.url().endsWith("/api/my-tasks") && r.method() === "POST",
  );
  await page.clock.fastForward("24:00:00");
  await reloaded;
  await expect(groupHeadings(page)).toHaveText(["期限切れ 1"]);
  await expect(page.getByRole("combobox", { name: "絞り込み" })).toHaveValue(mark);
});

test("tree due label updates", async ({ page }, info) => {
  const title = `${markOf(info)} ツリー`;
  const today = dayFrom(0);
  await myTask(page, title, { end: today });

  await page.clock.install({ time: new Date() });
  await page.goto(tokenUrl("/s/e2e/p/HH-P1/tree"));
  const tree = page.getByRole("list", { name: "ツリー" });
  const row = tree.getByRole("listitem").filter({ hasText: title }).last();
  await expect(row).toContainText(`今日 ${shownDay(today)}`);

  await page.clock.fastForward("24:00:00");
  await expect(row).toContainText(`期限切れ ${shownDay(today)}`);
});
