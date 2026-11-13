import type { Locator, Page } from "@playwright/test";
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

// スプリントの通しのテスト(F1)。ワークスペースは毎回作る(サンプルは同じ worker のほかのテストが使う)。

// createSprint は、スプリントのタブの「スプリントを作成」から、既定の名前のまま作る。
async function createSprint(page: Page, name: string) {
  await page.getByRole("button", { name: "スプリントを作成" }).first().click();
  const dialog = page.getByRole("dialog", { name: "スプリントを作成" });
  await expect(dialog.getByRole("textbox", { name: "名前" })).toHaveValue(name);
  await dialog.getByRole("button", { name: "作成", exact: true }).click();
  await expect(dialog).toBeHidden();
  await expect(page.getByText(`${name} を作成しました`)).toBeVisible();
}

// selected は、セレクトでいま選ばれている選択肢。
const selected = (select: Locator) => select.locator("option:checked");

test("creates, starts, closes and deletes sprints", async ({ page }, info) => {
  const prefix = prefixOf(info);
  await createWorkspace(page, info, unique(info, "計画"), prefix);
  // プロジェクト(接頭辞-P1)、Epic(接頭辞-1)、Story(接頭辞-2)。
  await projectWithEpic(page, info, prefix);
  await storyUnderEpic(page, info, `${prefix}-1`, "検索");

  const sprintsTab = tokenUrl(`/s/e2e/w/${prefix}/sprints`);
  await page.goto(sprintsTab);
  await createSprint(page, "Sprint 1");
  await createSprint(page, "Sprint 2");

  // Story の詳細パネルで Sprint 1 を選ぶ。
  await page.goto(tokenUrl(`/s/e2e/i/${prefix}-2`));
  const panel = page.getByRole("region", { name: "詳細パネル" });
  const sprintSelect = panel.getByRole("combobox", { name: "スプリント" });
  const assigned = saved(page, "/sprint");
  await sprintSelect.selectOption({ label: "Sprint 1" });
  await assigned;

  await page.goto(sprintsTab);
  const s1 = page.getByRole("article", { name: "Sprint 1" });
  const s2 = page.getByRole("article", { name: "Sprint 2" });
  await expect(s1.getByText("アイテム 1 件")).toBeVisible();

  // 開始する。見出しに「実行中」が出て、ほかのスプリントは開始できない。
  await s1.getByRole("button", { name: "開始", exact: true }).click();
  const start = page.getByRole("dialog", { name: "Sprint 1" });
  await start.getByRole("button", { name: "開始", exact: true }).click();
  await expect(start).toBeHidden();
  await expect(page.getByText("Sprint 1 を開始しました")).toBeVisible();
  await expect(page.getByText(/^実行中: Sprint 1\(.+〜.+\)$/)).toBeVisible();
  const startS2 = s2.getByRole("button", { name: "開始", exact: true });
  await expect(startS2).toBeDisabled();
  await expect(startS2).toHaveAttribute("title", "実行中のスプリントを先に完了してください");

  // 完了する。未完了の移動先の既定は計画中の Sprint 2 なので、バックログを選び直す。
  await s1.getByRole("button", { name: "完了…" }).click();
  const close = page.getByRole("dialog", { name: "Sprint 1" });
  await expect(close.getByText("完了 0 件・未完了 1 件です。")).toBeVisible();
  const destination = close.getByRole("combobox", { name: "未完了の 1 件の移動先" });
  await expect(selected(destination)).toHaveText("Sprint 2");
  await destination.selectOption({ label: "バックログ" });
  await close.getByRole("button", { name: "完了", exact: true }).click();
  await expect(close).toBeHidden();
  await expect(page.getByText("Sprint 1 を完了しました")).toBeVisible();

  await page.goto(tokenUrl(`/s/e2e/i/${prefix}-2`));
  await expect(selected(sprintSelect)).toHaveText("バックログ");

  // 計画中の Sprint 2 を削除する。
  await page.goto(sprintsTab);
  await s2.getByRole("button", { name: "Sprint 2 のメニュー" }).click();
  await page.getByRole("menuitem", { name: "削除…" }).click();
  const remove = page.getByRole("dialog", { name: "スプリントを削除" });
  await expect(remove.getByText(/設定の「削除済み」から戻せます。/)).toBeVisible();
  await remove.getByRole("button", { name: "削除", exact: true }).click();
  await expect(remove).toBeHidden();
  await expect(page.getByText("スプリントを削除しました")).toBeVisible();
  await expect(s2).toHaveCount(0);

  // 完了した Sprint 1 と同じ名前では作れない(断りはダイアログの中に出る)。
  await page.getByRole("button", { name: "スプリントを作成" }).first().click();
  const create = page.getByRole("dialog", { name: "スプリントを作成" });
  await create.getByRole("textbox", { name: "名前" }).fill("Sprint 1");
  await create.getByRole("button", { name: "作成", exact: true }).click();
  await expect(create.getByText("同じ名前のスプリントがあります")).toBeVisible();
});

// 完了のダイアログは、いまのカードで出す。開いているあいだに別のタブが未完了を足し、保存に失敗して
// 読み直したら、未完了の件数と移動先のセレクトが出る(開いたときのカードのままにしない)。
test("the close dialog follows the cards read again while it is open", async ({ page }, info) => {
  const prefix = prefixOf(info);
  await createWorkspace(page, info, unique(info, "読み直し"), prefix);
  await projectWithEpic(page, info, prefix);
  await storyUnderEpic(page, info, `${prefix}-1`, "検索");
  await page.goto(tokenUrl(`/s/e2e/w/${prefix}/sprints`));
  await createSprint(page, "Sprint 1");
  const s1 = page.getByRole("article", { name: "Sprint 1" });
  await s1.getByRole("button", { name: "開始", exact: true }).click();
  const start = page.getByRole("dialog", { name: "Sprint 1" });
  await start.getByRole("button", { name: "開始", exact: true }).click();
  await expect(start).toBeHidden();

  // 未完了 0 件で開く。移動先は出ない。
  await s1.getByRole("button", { name: "完了…" }).click();
  const close = page.getByRole("dialog", { name: "Sprint 1" });
  await expect(close.getByText("完了 0 件・未完了 0 件です。")).toBeVisible();
  await expect(close.getByRole("combobox")).toHaveCount(0);

  // 別のタブで、Story(接頭辞-2)を Sprint 1 に入れる(このタブはダイアログのあいだ反映を待つ)。
  const other = await page.context().newPage();
  try {
    await other.goto(tokenUrl(`/s/e2e/i/${prefix}-2`));
    const assigned = saved(other, "/sprint");
    await other
      .getByRole("region", { name: "詳細パネル" })
      .getByRole("combobox", { name: "スプリント" })
      .selectOption({ label: "Sprint 1(実行中)" });
    await assigned;
  } finally {
    await other.close();
  }

  // 完了の保存が失敗すると、エラーのトーストを出してカードを読み直す。ダイアログはいまのカードで出し直す。
  const closing = /\/sprints\/[^/]+\/close$/;
  await page.route(closing, (route) => route.fulfill({ status: 500 }));
  await close.getByRole("button", { name: "完了", exact: true }).click();
  await expect(page.getByText("保存できませんでした")).toBeVisible();
  await expect(close.getByText("完了 0 件・未完了 1 件です。")).toBeVisible();
  await expect(close.getByRole("combobox", { name: "未完了の 1 件の移動先" })).toBeVisible();
  await page.unroute(closing);
});
