// Screens this spec relies on (see screens.ts for the shared list):
//   - /settings/schema/<store> is the heading "スキーマ"; the navigation "スキーマの種類" has the links
//     ステータス / プロパティ / 関連 / ワークフロー (the kinds are fixed: see 2026-11-08-fixed-kinds-relations-design.md), and the tab shows a table whose rows are named by the
//     definition: the number of tasks is its last number, and its buttons are "編集（<名前>）" and
//     "マージ（<名前>）"
//   - the region "プロジェクトごとの上書き" lists a project with an override as a link to its settings
//   - the region "スキーマの検査" says "スキーマとタスクは合っています" when `hh schema check` finds nothing
//   - the dialog "ステータス「<名前>」を編集" has the textbox "名前" and the button "保存"; a toast says
//     "ステータス「<新しい名前>」を保存しました"
//   - the dialog "ステータス「<名前>」をマージ" has the combobox "マージ先", the sentence "<n> 件のタスクが
//     書き換わります" (the dry run) and the button "マージ"; a toast says "<n> 件を書き換えました"
import type { Page } from "@playwright/test";
import { expect, test } from "./harness";

const region = (page: Page, name: string) => page.getByRole("region", { name });
const statusRow = (page: Page, name: string) =>
  page.getByRole("row").filter({ has: page.getByRole("rowheader", { name, exact: true }) });
const SCHEMA = "/settings/schema/demo";

test("the number of tasks of a status is the number the CLI made", async ({ page, hh }) => {
  hh.add("One");
  hh.add("Two");
  const three = hh.add("Three");
  hh.hh("set", three.id, "status=waiting");
  await hh.open(page, SCHEMA);

  await expect(page.getByRole("heading", { level: 1, name: "スキーマ" })).toBeVisible();
  await expect(statusRow(page, "todo").getByRole("cell").nth(4)).toHaveText("2");
  await expect(statusRow(page, "waiting").getByRole("cell").nth(4)).toHaveText("1");
  await expect(statusRow(page, "done").getByRole("cell").nth(4)).toHaveText("0");

  // a task made while the page is open is counted at once
  hh.add("Four");
  await expect(statusRow(page, "todo").getByRole("cell").nth(4)).toHaveText("3");
});

test("a project with an override is listed with a link to its settings", async ({ page, hh }) => {
  await hh.open(page, SCHEMA);
  await expect(region(page, "プロジェクトごとの上書き")).toContainText(
    "上書きのあるプロジェクトはありません",
  );

  const project = hh.add("Release", "--kind", "project");
  hh.hh(
    "schema",
    "scope",
    "set",
    project.id,
    "--statuses",
    "backlog,todo,on_hold,in_progress,done,canceled",
  );
  const link = region(page, "プロジェクトごとの上書き").getByRole("link", { name: /Release/ });
  await expect(link).toBeVisible();
  await expect(link).toContainText(project.id);
  await link.click();
  await expect(page).toHaveURL(new RegExp(`/projects/${project.ulid}/settings$`));
});

test("schema check says the schema and the tasks agree", async ({ page, hh }) => {
  hh.add("One");
  await hh.open(page, SCHEMA);
  const check = region(page, "スキーマの検査");
  await expect(check.getByText("スキーマとタスクは合っています")).toBeVisible();
  await expect(
    check.getByRole("button", { name: "コマンドをコピー: hh schema check" }),
  ).toBeVisible();
});

test("a status is renamed in the edit dialog, and the tasks follow the name", async ({
  page,
  hh,
}) => {
  const one = hh.add("One");
  const two = hh.add("Two");
  hh.add("Other");
  hh.hh("set", one.id, "status=waiting");
  hh.hh("set", two.id, "status=waiting");
  await hh.open(page, SCHEMA);
  await expect(statusRow(page, "waiting").getByRole("cell").nth(4)).toHaveText("2");

  await statusRow(page, "waiting").getByRole("button", { name: "編集（waiting）" }).click();
  const dialog = page.getByRole("dialog", { name: "ステータス「waiting」を編集" });
  await dialog.getByRole("textbox", { name: "名前" }).fill("paused");
  await dialog.getByRole("button", { name: "保存" }).click();

  await expect(
    page.getByRole("status").filter({ hasText: "ステータス「paused」を保存しました" }),
  ).toBeVisible();
  await expect(dialog).toHaveCount(0);
  expect(hh.show(one.id).status).toBe("paused");
  expect(hh.show(two.id).status).toBe("paused");
  await expect(statusRow(page, "paused").getByRole("cell").nth(4)).toHaveText("2");
  await expect(statusRow(page, "waiting")).toHaveCount(0);
  // the table was drawn again with the new name, so the focus is not lost with the old row
  await expect(page.getByRole("heading", { level: 1, name: "スキーマ" })).toBeFocused();
});

test("a status is merged into another from the list of the others", async ({ page, hh }) => {
  const one = hh.add("One");
  hh.hh("set", one.id, "status=on_hold");
  await hh.open(page, SCHEMA);

  await statusRow(page, "on_hold").getByRole("button", { name: "マージ（on_hold）" }).click();
  const dialog = page.getByRole("dialog", { name: "ステータス「on_hold」をマージ" });
  await dialog.getByRole("combobox", { name: "マージ先" }).selectOption("backlog");
  await expect(dialog.getByText("1 件のタスクが書き換わります")).toBeVisible();
  await expect(
    dialog.getByText("「on_hold」は定義から消え、「backlog」にマージされます"),
  ).toBeVisible();
  await dialog.getByRole("button", { name: "マージ" }).click();

  await expect(page.getByRole("status").filter({ hasText: "1 件を書き換えました" })).toBeVisible();
  expect(hh.show(one.id).status).toBe("backlog");
  await expect(statusRow(page, "on_hold")).toHaveCount(0);
});

test("a tab is in the address, and a tab opened by address shows that tab", async ({
  page,
  hh,
}) => {
  await hh.open(page, SCHEMA);
  const tabs = page.getByRole("navigation", { name: "スキーマの種類" });
  await tabs.getByRole("link", { name: /^プロパティ/ }).click();
  await expect(page).toHaveURL(/\/settings\/schema\/demo\?tab=props$/);
  await expect(page.getByRole("table", { name: "プロパティ" })).toBeVisible();
  await tabs.getByRole("link", { name: /^ステータス/ }).click();
  await expect(page).toHaveURL(/\/settings\/schema\/demo$/);

  // the address of a tab opens the tab, as a new browser tab does
  await hh.open(page, `${SCHEMA}?tab=workflow`);
  await expect(page.getByRole("heading", { level: 2, name: "ワークフロー" })).toBeVisible();
  await expect(tabs.getByRole("link", { name: /^ワークフロー/ })).toHaveAttribute(
    "aria-current",
    "page",
  );
});
