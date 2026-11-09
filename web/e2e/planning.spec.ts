import type { Page } from "@playwright/test";
import { expect, type Hedgehug, test } from "./harness";
import { dragRow } from "./screens";

const openTab = async (page: Page, hh: Hedgehug, tab: string) =>
  hh.open(page, (await hh.treePath()).replace(/\/tree$/, `/${tab}`));

const backlogRow = (page: Page, title: string) =>
  page
    .getByRole("list", { name: "バックログのタスク" })
    .getByRole("listitem")
    .filter({ has: page.getByRole("link", { name: title, exact: true }) });

test("the plan tabs always show, and the gantt and the reports come with the first sprint made on the sprints tab", async ({
  page,
  hh,
}) => {
  await openTab(page, hh, "sprints");
  await expect(page.getByRole("link", { name: "バックログ" })).toBeVisible();
  await expect(page.getByRole("link", { name: "リリース" })).toBeVisible();
  await expect(page.getByRole("link", { name: "ガント" })).toHaveCount(0);
  await expect(page.getByRole("link", { name: "レポート" })).toHaveCount(0);
  await page.getByRole("button", { name: "スプリント", exact: true }).click();
  const dialog = page.getByRole("dialog");
  await dialog.getByLabel("名前", { exact: true }).fill("Sprint 1");
  await dialog.getByRole("button", { name: "作成" }).click();
  await expect(page.getByRole("link", { name: "ガント" })).toBeVisible();
  await expect(page.getByRole("link", { name: "レポート" })).toBeVisible();
});

test("a sprint goes from the backlog through the board to closing", async ({ page, hh }) => {
  hh.hh("add", "work", "--kind", "project"); // DM-1: the backlog lists the tasks, not the projects
  hh.hh("add", "one", "--parent", "DM-1", "-p", "points=3");
  hh.hh("add", "two", "--parent", "DM-1", "-p", "points=2");
  hh.hh("add", "three", "--parent", "DM-1");
  hh.hh("sprint", "add", "Sprint 1", "--start", "2026-11-02", "--end", "2026-12-31");
  hh.hh("sprint", "add", "Sprint 2", "--start", "2027-01-01", "--end", "2027-01-14");
  await openTab(page, hh, "backlog");

  // Into a sprint by dragging; the panel sums the points.
  const panel = page.getByRole("region", { name: "Sprint 1" });
  await dragRow(page, backlogRow(page, "one"), panel, "child");
  await expect(panel.getByText("3 pt", { exact: true }).first()).toBeVisible();
  await expect(backlogRow(page, "one")).toHaveCount(0);

  // Into a sprint from the menu of the row, for the keyboard.
  await page.getByRole("button", { name: "two の移動先" }).click();
  await page.getByRole("menuitem", { name: "Sprint 1 に入れる" }).click();
  await expect(panel.getByText("5 pt", { exact: true }).first()).toBeVisible();

  // Start it from the panel, and work on the board with the days left at the top.
  await panel.getByRole("button", { name: "開始する" }).click();
  await page.getByRole("dialog").getByRole("button", { name: "開始する" }).click();
  await expect(panel.getByRole("link", { name: "カンバンを開く" })).toBeVisible();
  await panel.getByRole("link", { name: "カンバンを開く" }).click();
  await expect(page.getByRole("region", { name: "Sprint 1 の状況" })).toContainText("残り");
  await expect(page.getByLabel("カンバンの盤面")).toContainText("one");
  await expect(page.getByLabel("カンバンの盤面")).not.toContainText("three");

  hh.hh("set", "DM-2", "status=done");
  await expect(page.getByRole("cell", { name: "1/2" })).toBeVisible();

  // Closing carries what is left to the next sprint.
  await page.getByRole("button", { name: "スプリントを閉じる…" }).click();
  const close = page.getByRole("dialog");
  await expect(close.getByText("未完了の 1 件の移動先")).toBeVisible();
  await expect(close.getByLabel(/次のスプリント（Sprint 2/)).toBeChecked();
  await close.getByRole("button", { name: "閉じて 1 件を Sprint 2 へ" }).click();
  await expect(page.getByRole("row", { name: /Sprint 1 closed/ })).toBeVisible();
  const two = hh.json<{ sprint?: { name: string } }>("show", "DM-3");
  expect(two.sprint?.name).toBe("Sprint 2");
});

test("a sprint is made, changed and removed", async ({ page, hh }) => {
  hh.hh("sprint", "add", "Sprint 1");
  await openTab(page, hh, "sprints");
  await page.getByRole("button", { name: "Sprint 1 を編集" }).click();
  await page.getByLabel("ゴール").fill("出す");
  await page.getByRole("button", { name: "保存" }).click();
  await expect.poll(() => hh.hh("sprint", "ls", "--json")).toContain("出す");
  await page.getByRole("button", { name: "Sprint 1 を削除" }).click();
  await page.getByRole("dialog").getByRole("button", { name: "削除" }).click();
  await expect(page.getByText("スプリントはありません")).toBeVisible();
});

test("the backlog orders across parents without changing a parent", async ({ page, hh }) => {
  hh.hh("add", "p1", "--kind", "project");
  hh.hh("add", "p2", "--kind", "project");
  hh.hh("add", "a", "--parent", "DM-1");
  hh.hh("add", "b", "--parent", "DM-2");
  hh.hh("add", "c", "--parent", "DM-1");
  hh.hh("sprint", "add", "Sprint 1");
  await openTab(page, hh, "backlog");
  await expect(backlogRow(page, "p1")).toHaveCount(0);
  await expect(backlogRow(page, "c")).toBeVisible();

  await dragRow(page, backlogRow(page, "c"), backlogRow(page, "a"), "before");
  const list = page.getByRole("list", { name: "バックログのタスク" }).getByRole("listitem");
  await expect(list.first()).toContainText("c");
  const c = hh.json<{ parent: string | null; ulid: string }>("show", "DM-5");
  const p1 = hh.json<{ ulid: string }>("show", "DM-1");
  expect(c.parent).toBe(p1.ulid);
});

test("a release shows its progress and notes and changes state", async ({ page, hh }) => {
  hh.hh("release", "add", "v1", "--date", "2030-01-01");
  hh.hh("add", "done work", "--release", "v1");
  hh.hh("add", "open work", "--release", "v1");
  hh.hh("set", "DM-1", "status=done");
  await openTab(page, hh, "releases");
  const detail = page.getByRole("region", { name: "v1" });
  await expect(detail.getByText("/ 2 完了")).toBeVisible();
  await expect(detail.locator("pre")).toContainText("done work");
  await detail.getByRole("button", { name: "released にする" }).click();
  await expect.poll(() => hh.hh("release", "ls")).toContain("released");
});

test("the detail of a task has the sprint, the release and the time spent", async ({
  page,
  hh,
}) => {
  hh.hh("add", "one", "-p", "estimate=6h");
  hh.hh("sprint", "add", "Sprint 1");
  hh.hh("release", "add", "v1");
  const task = hh.json<{ ulid: string }>("show", "DM-1");
  await hh.open(page, `/tasks/${task.ulid}`);

  await page.getByRole("textbox", { name: "時間" }).fill("2h");
  await page.getByRole("textbox", { name: "メモ" }).fill("調査");
  await page.getByRole("button", { name: "記録" }).click();
  await expect(page.getByText("2h / 見積もり 6h")).toBeVisible();
  await expect(page.getByText("調査", { exact: true })).toBeVisible();
  expect(hh.hh("worklog", "ls", "DM-1")).toContain("調査");
  await page.getByRole("button", { name: /の記録を削除/ }).click();
  await expect(page.getByText("調査", { exact: true })).toHaveCount(0);
  await expect(page.getByText("0m / 見積もり 6h")).toBeVisible();

  await page.getByRole("button", { name: "スプリント" }).click();
  await page.getByRole("menuitem", { name: "Sprint 1" }).click();
  await expect.poll(() => hh.hh("ls", 'sprint:current,"Sprint 1"')).toContain("one");
  await page.getByRole("textbox", { name: "リリース" }).fill("v1");
  await page.getByRole("textbox", { name: "リリース" }).press("Enter");
  await expect.poll(() => hh.hh("ls", "release:v1")).toContain("one");
});

test("the table updates the rows picked in one commit", async ({ page, hh }) => {
  hh.hh("add", "one");
  hh.hh("add", "two");
  hh.hh("add", "three");
  hh.hh("sprint", "add", "Sprint 1");
  await openTab(page, hh, "table");
  await page.getByRole("checkbox", { name: "DM-1 を選択" }).check();
  await page.getByRole("checkbox", { name: "DM-2 を選択" }).check();
  const bar = page.getByRole("toolbar", { name: "選択したタスクの一括編集" });
  await expect(bar).toContainText("2 件を選択");
  await bar.getByRole("button", { name: "まとめて更新…" }).click();
  const dialog = page.getByRole("dialog");
  await dialog.getByLabel("項目").selectOption("sprint");
  await dialog.getByLabel("値").fill("Sprint 1");
  await expect(dialog.getByText("2 件が対象です")).toBeVisible();
  await dialog.getByRole("button", { name: "2 件を更新" }).click();
  await expect(dialog).toHaveCount(0);
  await expect.poll(() => hh.hh("ls", 'sprint:"Sprint 1"')).toContain("two");
  expect(hh.hh("ls", "sprint:none")).toContain("three");
});

test("the palette updates what a query finds", async ({ page, hh }) => {
  hh.hh("add", "one");
  hh.hh("add", "two");
  hh.hh("schema", "labels", "add", "x");
  hh.hh("set", "DM-1", "labels=x");
  hh.hh("set", "DM-2", "labels=x");
  await hh.open(page, "/");
  await page.keyboard.press("ControlOrMeta+k");
  await page.getByRole("combobox").fill("label:x");
  await page.getByRole("option", { name: /一致する 2 件をまとめて更新/ }).click();
  const dialog = page.getByRole("dialog");
  await dialog.getByLabel("値").selectOption("done");
  await expect(dialog.getByText("2 件が対象です")).toBeVisible();
  await dialog.getByRole("button", { name: "2 件を更新" }).click();
  await expect.poll(() => hh.hh("ls", "status:done")).toContain("two");
});

test("a saved board warns over the WIP limit and does not refuse", async ({ page, hh }) => {
  hh.hh("add", "one");
  hh.hh("add", "two");
  hh.hh("view", "save", "board", "--layout", "kanban", "--wip", "todo=1");
  await hh.open(page, `/views/demo/board`);
  await expect(page.getByText("WIP 上限 1 を超えています")).toBeVisible();
  await expect(page.getByText("2 / 1")).toBeVisible();
  await page.getByRole("button", { name: "WIP 上限" }).click();
  await page.getByLabel("Todo の上限").fill("3");
  await page.getByRole("button", { name: "適用" }).click();
  await expect(page.getByText("2 / 3")).toBeVisible();
  await expect(page.getByText("WIP 上限 1 を超えています")).toHaveCount(0);
});

test("a template is written in the settings and expanded from the create dialog", async ({
  page,
  hh,
}) => {
  await hh.open(page, `/settings/schema/demo?tab=templates`);
  await page.getByRole("button", { name: "＋ テンプレート" }).click();
  await page.getByLabel("テンプレートの名前").fill("rel");
  await page.getByRole("button", { name: "開く" }).click();
  const file = page.getByLabel("テンプレートのファイル");
  await file.fill("---\nbad: [\n---\n");
  await page.getByRole("button", { name: "保存" }).click();
  await expect(page.getByRole("alert")).toBeVisible();
  await file.fill("---\nkind: task\nchildren:\n  - title: child\n---\n本文\n");
  await page.getByRole("button", { name: "保存" }).click();
  await expect(page.getByRole("button", { name: /^rel / })).toContainText("子タスク 1");

  await hh.open(page, "/");
  await page.keyboard.press("c");
  await page.getByRole("textbox", { name: "タイトル" }).fill("from template");
  await page.getByLabel("テンプレート").selectOption("rel");
  await page.getByRole("button", { name: "作成", exact: false }).last().click();
  await expect.poll(() => hh.hh("ls")).toContain("child");
});

test("a workspace with a sprint says so where it cannot be removed", async ({ page, hh }) => {
  hh.hh("sprint", "add", "Sprint 1");
  await openTab(page, hh, "settings");
  await expect(page.getByText(/スプリント 1 件が残っているため削除できません/)).toBeVisible();
});

test("two active sprints are picked at the top of the board", async ({ page, hh }) => {
  hh.hh("sprint", "add", "Sprint 1", "--start", "2026-11-02", "--end", "2027-01-01");
  hh.hh("sprint", "add", "Sprint 2", "--start", "2026-11-02", "--end", "2027-01-01");
  hh.hh("add", "in one", "--sprint", "Sprint 1");
  hh.hh("add", "in two", "--sprint", "Sprint 2");
  hh.hh("sprint", "start", "Sprint 1");
  hh.hh("sprint", "start", "Sprint 2");
  await openTab(page, hh, "sprints");
  const board = page.getByLabel("カンバンの盤面");
  await expect(board).toContainText("in one");
  await expect(board).not.toContainText("in two");
  await page.getByRole("button", { name: "Sprint 2", exact: true }).click();
  await expect(board).toContainText("in two");
  await expect(board).not.toContainText("in one");
});

test("a project limits its templates", async ({ page, hh }) => {
  hh.hh("add", "p", "--kind", "project");
  const { ulid } = hh.json<{ ulid: string }>("show", "DM-1");
  await hh.open(page, "/settings/schema/demo?tab=templates");
  for (const name of ["a", "b"]) {
    await page.getByRole("button", { name: "＋ テンプレート" }).click();
    await page.getByLabel("テンプレートの名前").fill(name);
    await page.getByRole("button", { name: "開く" }).click();
    await page.getByLabel("テンプレートのファイル").fill("---\nkind: task\n---\n本文\n");
    await page.getByRole("button", { name: "保存" }).click();
    await expect(page.getByRole("button", { name: new RegExp(`^${name} `) })).toBeVisible();
  }
  await hh.open(page, `/projects/${ulid}/settings`);
  await page.getByRole("checkbox", { name: "a", exact: true }).check();
  await page.getByRole("button", { name: "保存" }).click();
  await expect(page.getByText("プロジェクトの設定を保存しました")).toBeVisible();
  await hh.open(page, `/projects/${ulid}`);
  await page.keyboard.press("c");
  const select = page.getByLabel("テンプレート");
  await expect(select.getByRole("option", { name: "a" })).toHaveCount(1);
  await expect(select.getByRole("option", { name: "b" })).toHaveCount(0);
});
