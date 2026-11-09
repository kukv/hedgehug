// Screens this spec relies on (see screens.ts for the shared list):
//   - /my is the tasks of the person: the heading "マイタスク", the line "自分 = <login>", the filter
//     field (its default is assignee:@me -status:done -status:canceled), the count "<n> ワークスペース ·
//     <m> 件", and the regions named by their group: "期限切れ", "今日", "今週", "来週以降", "期限なし"
//     (by due date) or the name of the project, "プロジェクトなし" (by project; the button "グループ: …"
//     opens the menu 期限 / ワークスペース / プロジェクト; a workspace group has the badge of its Store
//     by the name). A row shows the ID and the title as links, and the Store
//   - a Store that is hidden has its tasks left out, and the text "非表示の Store に <n> 件（<名前>）"
//     with the link "マウント中の Store を開く" (to /settings/stores) tells how many
//   - the sidebar ("メインナビゲーション") has the link "マイタスク" with the count of the same query
//   - a name of the query the server refuses ("status:in_progres") is explained, and the button named
//     by the right term ("status:in_progress") puts it in the field
import type { Page } from "@playwright/test";
import { expect, test } from "./harness";

const sidebar = (page: Page) => page.getByRole("navigation", { name: "メインナビゲーション" });
const filter = (page: Page) =>
  page.getByRole("textbox", { name: "フィルタ（hh ls と同じクエリ）" });

// The assignee is the ID of the account of the tests (host/login): the query's @me stands for it.
const ME = "github.com/e2e";
const ymd = (d: Date) =>
  `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;

test("the tasks of the person are grouped by due date, and the sidebar counts them", async ({
  page,
  hh,
}) => {
  const today = new Date();
  const later = new Date(today.getFullYear(), today.getMonth(), today.getDate() + 10);
  // The root holds only projects, which the sidebar lists by title: the tasks go under one.
  const under = ["--parent", hh.add("Work", "--kind", "project").id];
  const dueToday = hh.add(
    "Due today",
    ...under,
    "-p",
    `assignees=${ME}`,
    "-p",
    `due=${ymd(today)}`,
  );
  const dueLater = hh.add(
    "Due later",
    ...under,
    "-p",
    `assignees=${ME}`,
    "-p",
    `due=${ymd(later)}`,
  );
  const noDue = hh.add("No due date", ...under, "-p", `assignees=${ME}`);
  const finished = hh.add("Finished", ...under, "-p", `assignees=${ME}`);
  hh.hh("set", finished.id, "status=done");
  hh.add("Somebody else's", ...under);

  await hh.open(page);
  await sidebar(page)
    .getByRole("link", { name: /マイタスク/ })
    .click();
  await expect(page).toHaveURL(`${hh.origin}/my`);
  await expect(page.getByRole("heading", { level: 1, name: "マイタスク" })).toBeVisible();
  await expect(page.getByText("自分 = e2e")).toBeVisible();
  await expect(filter(page)).toHaveValue("assignee:@me -status:done -status:canceled");
  await expect(page.getByText("1 ワークスペース · 3 件")).toBeVisible();
  // the sidebar asks the same query
  await expect(sidebar(page).getByRole("link", { name: /マイタスク/ })).toContainText("3");

  const group = (name: string) => page.getByRole("region", { name, exact: true });
  await expect(group("今日").getByRole("row", { name: new RegExp(dueToday.id) })).toContainText(
    "Due today",
  );
  await expect(group("来週以降").getByRole("row", { name: new RegExp(dueLater.id) })).toContainText(
    "Due later",
  );
  await expect(group("期限なし").getByRole("row", { name: new RegExp(noDue.id) })).toContainText(
    "No due date",
  );
  await expect(page.getByText("Finished")).toHaveCount(0);
  await expect(page.getByText("Somebody else's")).toHaveCount(0);
  // the store is told on every row
  await expect(group("今日").getByRole("row").first()).toContainText("demo");

  // a task opens from its title
  await group("今日").getByRole("link", { name: "Due today" }).click();
  await expect(page).toHaveURL(new RegExp(`/tasks/${dueToday.ulid}$`));
});

test("the tasks are grouped by project from the menu", async ({ page, hh }) => {
  const project = hh.add("Release", "--kind", "project");
  hh.add("In the project", "-p", `assignees=${ME}`, "--parent", project.id);
  hh.add("Outside", "-p", `assignees=${ME}`);
  await hh.open(page, "/my");

  await page.getByRole("button", { name: "グループ: 期限" }).click();
  await page.getByRole("menuitem", { name: "プロジェクト" }).click();
  await expect(
    page.getByRole("region", { name: "Release", exact: true }).getByText("In the project"),
  ).toBeVisible();
  await expect(
    page.getByRole("region", { name: "プロジェクトなし" }).getByText("Outside"),
  ).toBeVisible();
});

test("a task that gets an assignee from the CLI appears and the count follows", async ({
  page,
  hh,
}) => {
  const t = hh.add("Not mine yet", "--parent", hh.add("Work", "--kind", "project").id);
  await hh.open(page, "/my");
  await expect(page.getByText("一致するタスクはありません")).toBeVisible();

  hh.hh("set", t.id, `assignees=${ME}`);
  await expect(page.getByRole("link", { name: "Not mine yet" })).toBeVisible();
  await expect(sidebar(page).getByRole("link", { name: /マイタスク/ })).toContainText("1");
});

test("a query that names a status wrong offers the right one", async ({ page, hh }) => {
  const work = hh.add("Work", "--kind", "project").id;
  const busy = hh.add(
    "In progress",
    "--parent",
    work,
    "-p",
    `assignees=${ME}`,
    "--status",
    "in_progress",
  );
  hh.add("Waiting its turn", "--parent", work, "-p", `assignees=${ME}`);
  await hh.open(page, "/my");
  await expect(page.getByRole("link", { name: "Waiting its turn" })).toBeVisible();

  await filter(page).fill("assignee:@me status:in_progres");
  await filter(page).press("Enter");
  await expect(page.getByRole("alert")).toContainText("クエリが正しくありません");
  const fix = page.getByRole("button", { name: "status:in_progress" });
  await expect(fix).toBeVisible();
  await expect(page.getByText("のことですか？")).toBeVisible();

  await fix.click();
  await expect(filter(page)).toHaveValue("assignee:@me status:in_progress");
  await expect(page.getByRole("alert")).toHaveCount(0);
  await expect(page.getByRole("link", { name: busy.title })).toBeVisible();
  await expect(page.getByRole("link", { name: "Waiting its turn" })).toHaveCount(0);
});

test("the tasks are grouped by workspace, and a hidden Store is left out with its count told", async ({
  page,
  hh,
}) => {
  hh.hh("store", "init", "second", "--prefix", "SC");
  const inDemo = hh.add("Work", "--kind", "project").id;
  const inSecond = hh.add("Work", "--kind", "project", "--store", "second").id;
  hh.add("In demo", "--parent", inDemo, "-p", `assignees=${ME}`);
  hh.add("In second", "--parent", inSecond, "-p", `assignees=${ME}`, "--store", "second");
  await hh.open(page, "/my");

  await page.getByRole("button", { name: "グループ: 期限" }).click();
  await page.getByRole("menuitem", { name: "ワークスペース" }).click();
  const heads = page.getByRole("region").locator(".g-head");
  await expect(heads).toHaveCount(2);
  await expect(heads.filter({ hasText: "demo" })).toBeVisible();
  await expect(heads.filter({ hasText: "second" })).toBeVisible();
  await expect(page.getByText("非表示の Store に")).toHaveCount(0);

  hh.hh("store", "set", "second", "--hidden=true");
  await expect(page.getByRole("link", { name: "In second" })).toHaveCount(0);
  await expect(heads).toHaveCount(1);
  await expect(page.getByRole("link", { name: "In demo" })).toBeVisible();
  await expect(page.getByText("非表示の Store に 1 件（second）")).toBeVisible();
  await page.getByRole("link", { name: "マウント中の Store を開く" }).click();
  await expect(page).toHaveURL(`${hh.origin}/settings/stores`);
});
