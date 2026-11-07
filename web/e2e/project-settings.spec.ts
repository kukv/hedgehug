// Screens this spec relies on (see screens.ts for the shared list):
//   - /projects/<ULID>/settings is the settings of a project (the tab "設定" at the right end of the tabs
//     in the navigation "ビューの切り替え"): the group "表示するステータス" has a checkbox for each status
//     of the Store (named by its label); the group "必須の項目を追加" has the combobox "必須にするステータス", a
//     checkbox for each field (named by the field) and the button "必須の項目を追加"; the button "保存"
//     writes the override and a toast says "プロジェクトの設定を保存しました"; "すべて Store の値に戻す…"
//     opens the dialog "プロジェクトの設定を Store の値に戻す", whose button "すべて戻す" takes the
//     override off
//   - the kanban of a project shows the statuses it keeps (ul[data-column="<status>"]) and asks for a
//     field the workflow needs (see kanban.spec.ts)
import type { Page } from "@playwright/test";
import { expect, type Hedgehug, test } from "./harness";
import { dragCard } from "./screens";

const column = (page: Page, status: string) => page.locator(`ul[data-column="${status}"]`);
const statuses = (page: Page) => page.getByRole("group", { name: "表示するステータス" });
const saved = (page: Page) =>
  page.getByRole("status").filter({ hasText: "プロジェクトの設定を保存しました" });
const tab = (page: Page, name: string) =>
  page.getByRole("navigation", { name: "ビューの切り替え" }).getByRole("link", { name });
const scopeShow = (hh: Hedgehug, id: string) => hh.hh("schema", "scope", "show", id);

test("a status taken off is saved, and the kanban of the project loses its column", async ({
  page,
  hh,
}) => {
  const project = hh.add("Release", "--kind", "project");
  hh.add("Ship it", "--parent", project.id);
  await hh.open(page, `/projects/${project.ulid}/kanban`);
  await expect(column(page, "waiting")).toBeVisible();

  await tab(page, "設定").click();
  await statuses(page)
    .getByRole("checkbox", { name: /待機中/ })
    .uncheck();
  await page.getByRole("button", { name: "保存", exact: true }).click();
  await expect(saved(page)).toBeVisible();

  expect(scopeShow(hh, project.id)).toContain(
    "statuses: backlog, todo, on_hold, in_progress, done, canceled",
  );
  await tab(page, "カンバン").click();
  await expect(column(page, "in_progress")).toBeVisible();
  await expect(column(page, "waiting")).toHaveCount(0);
});

test("a field the workflow needs is asked for on the kanban after it is saved", async ({
  page,
  hh,
}) => {
  const project = hh.add("Release", "--kind", "project");
  const ship = hh.add("Ship it", "--parent", project.id);
  await hh.open(page, `/projects/${project.ulid}/settings`);

  const form = page.getByRole("group", { name: "必須の項目を追加" });
  await form
    .getByRole("combobox", { name: "必須にするステータス" })
    .selectOption({ label: "進行中" });
  await form.getByRole("checkbox", { name: "points", exact: true }).check();
  await form.getByRole("button", { name: "必須の項目を追加" }).click();
  await page.getByRole("button", { name: "保存", exact: true }).click();
  await expect(saved(page)).toBeVisible();
  expect(scopeShow(hh, project.id)).toMatch(/required:\n\s+in_progress: points/);

  await tab(page, "カンバン").click();
  const card = page.locator(`li[data-ulid="${ship.ulid}"]`);
  await expect(card).toBeVisible();
  await dragCard(page, card, column(page, "in_progress"));
  await expect(
    page.getByRole("dialog", { name: `${ship.id} を進行中にするには ポイント が必要です` }),
  ).toBeVisible();
  expect(hh.show(ship.id).status).toBe("todo");
});

test("the whole override goes back to the Store's, after a question", async ({ page, hh }) => {
  const project = hh.add("Release", "--kind", "project");
  hh.hh("schema", "scope", "set", project.id, "--required", "done=points");
  await hh.open(page, `/projects/${project.ulid}/settings`);

  await expect(page.getByRole("list", { name: "必須の項目" })).toContainText("points");
  await page.getByRole("button", { name: "すべて Store の値に戻す…" }).click();
  const dialog = page.getByRole("dialog", { name: "プロジェクトの設定を Store の値に戻す" });
  await expect(dialog).toBeVisible();
  // asking changes nothing
  expect(scopeShow(hh, project.id)).toContain("done: points");
  await dialog.getByRole("button", { name: "すべて戻す" }).click();
  await expect(dialog).toBeHidden();

  expect(scopeShow(hh, project.id)).toContain("no override");
});
