import { expect, test } from "./fixtures";
import { send, sidebar, tokenUrl } from "./helpers";
import {
  dayFrom,
  filterBy,
  groupHeadings,
  markOf,
  myTask,
  openMyTasks,
  resetMyTasks,
  rowOf,
  shownDay,
} from "./my-tasks-helpers";
import { base } from "./search-table-helpers";

// マイタスクの画面の通しのテスト(04「マイタスク」、E4a)。アイテムはテストごとの印をタイトルに入れて作り、
// 絞り込みでほかのテストのアイテムを除く。

test.beforeEach(async ({ page }) => {
  await resetMyTasks(page);
});

test("groups my tasks by due", async ({ page }, info) => {
  const mark = markOf(info);
  const yesterday = dayFrom(-1);
  await myTask(page, `${mark} 昨日`, { end: yesterday, priority: "High" });
  await myTask(page, `${mark} 今日`, { end: dayFrom(0) });
  await myTask(page, `${mark} なし`);

  await openMyTasks(page);
  await expect(page.getByText("あなたが担当しているタスク")).toBeVisible();
  await filterBy(page, mark);
  await expect(groupHeadings(page)).toHaveText(["期限切れ 1", "今日 1", "期限なし 1"]);
  const late = rowOf(page, `${mark} 昨日`);
  await expect(late).toContainText(`期限切れ ${shownDay(yesterday)}`);
  await expect(late).toContainText("High");
});

test("switches grouping and keeps it", async ({ page }, info) => {
  const mark = markOf(info);
  await myTask(page, `${mark} 一`);
  await openMyTasks(page);
  await filterBy(page, mark);
  const grouping = page.getByRole("group", { name: "まとめ方" });
  // 書き終える前に読み込み直すと、書き込みが届かないことがある。
  const written = page.waitForResponse(
    (r) =>
      r.url().includes("/api/screen-states/my-tasks") &&
      r.request().method() === "PUT" &&
      (r.request().postData() ?? "").includes("project"),
  );
  await grouping.getByRole("button", { name: "プロジェクト" }).click();
  await expect(groupHeadings(page)).toHaveText(["ログイン 1"]);
  await written;

  await openMyTasks(page, true);
  await expect(grouping.getByRole("button", { name: "プロジェクト" })).toHaveAttribute(
    "aria-pressed",
    "true",
  );
  await expect(page.getByRole("combobox", { name: "絞り込み" })).toHaveValue(mark);
  await expect(groupHeadings(page)).toHaveText(["ログイン 1"]);
});

test("shows filter errors and empty states", async ({ page }, info) => {
  await openMyTasks(page);
  await filterBy(page, "status:");
  await expect(page.getByText("「status:」に値がありません")).toBeVisible();
  await expect(page.getByText("一致するものはありません")).toBeVisible();
  await expect(groupHeadings(page)).toHaveCount(0);

  await filterBy(page, `${markOf(info)}ない`);
  await expect(page.getByText("一致するものはありません")).toBeVisible();
  await expect(page.getByText("絞り込みを変えてみてください。")).toBeVisible();
});

test("changes status from the row", async ({ page }, info) => {
  const mark = markOf(info);
  await myTask(page, `${mark} 完了`);
  await myTask(page, `${mark} 残る`);
  await openMyTasks(page);
  await filterBy(page, mark);
  const row = rowOf(page, `${mark} 完了`);
  await row.getByRole("button", { name: "ステータス: Todo" }).click();
  await page
    .getByRole("menu", { name: "ステータス" })
    .getByRole("menuitemcheckbox", { name: "Done" })
    .click();
  await expect(row).toBeHidden();
  await expect(rowOf(page, `${mark} 残る`)).toBeVisible();
});

test("read-only rows cannot change status", async ({ page }) => {
  await openMyTasks(page);
  await filterBy(page, "ログイン画面を作る store:readonly");
  const status = rowOf(page, "ログイン画面を作る").getByRole("button", { name: /^ステータス: / });
  await expect(status).toBeDisabled();
  await expect(status).toHaveAttribute("title", "閲覧のみの Store です");
});

test("opens the detail panel from the title", async ({ page }, info) => {
  const mark = markOf(info);
  const title = `${mark} 詳細`;
  await myTask(page, title);
  await openMyTasks(page);
  await filterBy(page, mark);
  await rowOf(page, title).getByRole("link", { name: title }).click();
  const panel = page.getByRole("region", { name: "詳細パネル" });
  await expect(panel.getByRole("textbox", { name: "タイトル" })).toHaveValue(title);
});

test("deleted rows show the note instead of a status button", async ({ page }, info) => {
  const mark = markOf(info);
  const title = `${mark} 削除`;
  const id = await myTask(page, title);
  await send(page, "delete", `${base}/items/${id}`, undefined);
  await openMyTasks(page);
  await filterBy(page, `${mark} is:deleted`);
  const row = rowOf(page, title);
  await expect(row).toContainText(/あと \d+ 日で完全に削除/);
  await expect(row.getByRole("button", { name: /^ステータス: / })).toHaveCount(0);
  await expect(row.getByRole("link", { name: title })).toHaveCount(0);
});

// バッジは絞り込みなしの件数。マイタスクの画面の絞り込みでは変わらない(Review Focus 2)。
test("badge counts all my tasks", async ({ page }, info) => {
  const mark = markOf(info);
  // 先に 1 件作り、バッジが出ている(0 件で消えていない)ところから数える。
  await myTask(page, `${mark} 一`);
  await openMyTasks(page);
  const side = await sidebar(page, info);
  const badge = side.getByRole("img", { name: /^未完了 \d+ 件$/ });
  await expect(badge).toBeVisible();
  const before = Number(await badge.textContent());

  await myTask(page, `${mark} 二`);
  await expect(badge).toHaveAccessibleName(`未完了 ${before + 1} 件`);
  await expect(side.getByRole("link", { name: /^マイタスク/ })).toHaveAttribute(
    "aria-current",
    "page",
  );

  await filterBy(page, `${mark}ない`);
  await expect(page.getByText("一致するものはありません")).toBeVisible();
  await expect(badge).toHaveAccessibleName(`未完了 ${before + 1} 件`);
});

test("palette opens my tasks", async ({ page }) => {
  await page.goto(tokenUrl("/inbox"));
  await expect(page.getByRole("heading", { name: "受信箱", level: 1 })).toBeVisible();
  await page.keyboard.press("ControlOrMeta+k");
  const palette = page.getByRole("dialog", { name: "コマンドパレット" });
  await palette.getByRole("combobox").fill("マイタスクを開く");
  await palette.getByRole("option", { name: /^マイタスクを開く/ }).click();
  await expect(palette).toBeHidden();
  await expect(page.getByRole("heading", { name: "マイタスク", level: 1 })).toBeVisible();
  await expect(page).toHaveURL(/\/my-tasks$/);
});
