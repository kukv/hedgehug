// Screens this spec relies on (see schema.spec.ts and screens.ts for the shared list):
//   - the tab "ステータス" of /settings/schema/<store> has the button "追加" (the dialog "ステータスを追加" with
//     the textbox "名前", the textbox "表示名", the combobox "カテゴリ", the radio group "色" whose radios are
//     named by the color, e.g. "ティール", and the button "追加"); a row has "編集（<名前>）" (the dialog
//     "ステータス「<名前>」を編集", button "保存"), "削除（<名前>）" (the dialog "ステータス「<名前>」を削除",
//     button "削除", disabled while a task uses the status) and the handle "並べ替え（<名前>）", which Alt+Up
//     moves one row up
//   - the tab "プロパティ" (?tab=props) has the same buttons; the dialog "プロパティを追加" has the combobox "型"
//     and, for an enum, the button "値を追加" with a textbox "値" per value; the dialog of a row has the
//     textbox "名前" too, and a toast says "プロパティ「<名前>」を保存しました"
//   - the tabs "優先度" (?tab=priorities) and "ラベル" (?tab=labels) are lists: the textbox "値" per value, the
//     button "値を追加", the button "保存" (a toast says "ラベルを保存しました"); the detail of a task (the
//     region "プロパティ") chooses a label from the combobox "ラベル" and shows it with "<名前> を外す"
//   - renaming a value of the priorities list (the textbox "値") changes the priority of its tasks; the detail
//     shows it on the button "優先度"
//   - a status with a color draws its icon with the color in the custom property --c of the svg
import type { Page } from "@playwright/test";
import { expect, test } from "./harness";
import { properties, row } from "./screens";

const SCHEMA = "/settings/schema/demo";
const statusRow = (page: Page, name: string) =>
  page.getByRole("row").filter({ has: page.getByRole("rowheader", { name, exact: true }) });
const names = (page: Page) =>
  page.getByRole("table", { name: "ステータス" }).getByRole("rowheader");

test("a status is added, edited, moved, drawn in its color and deleted from the schema screen", async ({
  page,
  hh,
}) => {
  await hh.open(page, SCHEMA);
  await expect(page.getByRole("heading", { level: 1, name: "スキーマ" })).toBeVisible();

  // add: the new status is the last row
  await page.getByRole("button", { name: "追加" }).click();
  const add = page.getByRole("dialog", { name: "ステータスを追加" });
  await add.getByRole("textbox", { name: "名前" }).fill("review");
  await add.getByRole("combobox", { name: "カテゴリ" }).selectOption({ label: "進行中" });
  // the native radio lies under its label: click the label, then see the radio is checked
  await add.getByRole("radiogroup", { name: "色" }).getByText("ティール").click();
  await expect(add.getByRole("radio", { name: "ティール" })).toBeChecked();
  await add.getByRole("button", { name: "追加" }).click();
  await expect(add).toHaveCount(0);
  await expect(names(page).last()).toHaveText("review");
  await expect(statusRow(page, "review")).toContainText("進行中");

  // edit: the label shows in the table
  await statusRow(page, "review").getByRole("button", { name: "編集（review）" }).click();
  const edit = page.getByRole("dialog", { name: "ステータス「review」を編集" });
  await edit.getByRole("textbox", { name: "表示名" }).fill("レビュー");
  await edit.getByRole("button", { name: "保存" }).click();
  await expect(edit).toHaveCount(0);
  await expect(statusRow(page, "review")).toContainText("レビュー");

  // move: Alt+Up on the handle puts it one row up
  const before = await names(page).allInnerTexts();
  expect(before.at(-1)?.trim()).toBe("review");
  await page.getByRole("button", { name: "並べ替え（review）" }).focus();
  await page.keyboard.press("Alt+ArrowUp");
  await expect(async () => {
    const after = (await names(page).allInnerTexts()).map((s) => s.trim());
    expect(after.indexOf("review")).toBe(before.length - 2);
  }).toPass();

  // the icon of a task in the status is drawn in the color of the status
  const task = hh.add("In review");
  hh.hh("set", task.id, "status=review");
  await hh.openTree(page);
  const icon = row(page, "In review").locator("svg[data-shape]").first();
  await expect(icon).toHaveAttribute("style", /--c:\s*var\(--store-teal-dot\)/);

  // delete: refused while the task is in the status, then the row goes
  await hh.open(page, SCHEMA);
  const del = statusRow(page, "review").getByRole("button", { name: "削除（review）" });
  await expect(del).toBeDisabled();
  await expect(del).toHaveAttribute("title", /タスクが使っているため削除できません/);
  hh.hh("set", task.id, "status=todo");
  await expect(del).toBeEnabled();
  await del.click();
  const confirm = page.getByRole("dialog", { name: "ステータス「review」を削除" });
  await confirm.getByRole("button", { name: "削除" }).click();
  await expect(confirm).toHaveCount(0);
  await expect(statusRow(page, "review")).toHaveCount(0);
});

test("an enum is added with its values, then renamed and given a value from the edit dialog", async ({
  page,
  hh,
}) => {
  await hh.open(page, `${SCHEMA}?tab=props`);
  await page.getByRole("button", { name: "追加", exact: true }).click();
  const add = page.getByRole("dialog", { name: "プロパティを追加" });
  await add.getByRole("textbox", { name: "名前" }).fill("severity");
  await add.getByRole("combobox", { name: "型" }).selectOption("enum");
  await add.getByRole("button", { name: "値を追加" }).click();
  await add.getByRole("textbox", { name: "値" }).fill("low");
  await add.getByRole("button", { name: "追加", exact: true }).click();
  await expect(add).toHaveCount(0);
  const propRow = (name: string) =>
    page.getByRole("row").filter({ has: page.getByRole("rowheader", { name, exact: true }) });
  await expect(propRow("severity")).toBeVisible();

  // edit: the name changes and a value is added, in one save
  await propRow("severity").getByRole("button", { name: "編集（severity）" }).click();
  const edit = page.getByRole("dialog", { name: "プロパティ「severity」を編集" });
  await edit.getByRole("textbox", { name: "名前" }).fill("level");
  await edit.getByRole("button", { name: "値を追加" }).click();
  await edit.getByRole("textbox", { name: "値" }).last().fill("high");
  await edit.getByRole("button", { name: "保存" }).click();
  await expect(edit).toHaveCount(0);
  await expect(
    page.getByRole("status").filter({ hasText: "プロパティ「level」を保存しました" }),
  ).toBeVisible();
  await expect(propRow("level")).toBeVisible();
  await expect(propRow("severity")).toHaveCount(0);

  // the values were saved with the name
  await propRow("level").getByRole("button", { name: "編集（level）" }).click();
  const again = page.getByRole("dialog", { name: "プロパティ「level」を編集" });
  await expect(again.getByRole("textbox", { name: "値" })).toHaveCount(2);
  await expect(again.getByRole("textbox", { name: "値" }).first()).toHaveValue("low");
  await expect(again.getByRole("textbox", { name: "値" }).last()).toHaveValue("high");
});

test("a label is added in the labels tab, and then chosen from the list in the detail", async ({
  page,
  hh,
}) => {
  const t = hh.add("Labelled");
  await hh.open(page, `/tasks/${t.ulid}`);
  // with no label in the list, the detail points to where they are added
  await expect(page.getByRole("link", { name: "設定の「ラベル」で足してください" })).toBeVisible();

  await hh.open(page, `${SCHEMA}?tab=labels`);
  await expect(page.getByRole("heading", { level: 2, name: "ラベル" })).toBeVisible();
  await page.getByRole("button", { name: "値を追加" }).click();
  await page.getByRole("textbox", { name: "値" }).fill("bug");
  await page.getByRole("button", { name: "保存" }).click();
  await expect(page.getByRole("status").filter({ hasText: "ラベルを保存しました" })).toBeVisible();

  await hh.open(page, `/tasks/${t.ulid}`);
  await page.getByRole("combobox", { name: "ラベル" }).selectOption("bug");
  await expect(page.getByRole("button", { name: "bug を外す" })).toBeVisible();
  await expect.poll(() => hh.show(t.id).labels).toEqual(["bug"]);

  // a label a task holds cannot be deleted from the list
  await hh.open(page, `${SCHEMA}?tab=labels`);
  await expect(page.getByRole("button", { name: "削除（値 bug）" })).toBeDisabled();
});

test("a priority added in the priorities tab is offered in the detail", async ({ page, hh }) => {
  await hh.open(page, `${SCHEMA}?tab=priorities`);
  await expect(page.getByRole("heading", { level: 2, name: "優先度" })).toBeVisible();
  await expect(page.getByRole("textbox", { name: "値" })).toHaveCount(4);
  await page.getByRole("button", { name: "値を追加" }).click();
  await page.getByRole("textbox", { name: "値" }).last().fill("blocker");
  await page.getByRole("button", { name: "保存" }).click();
  await expect(page.getByRole("status").filter({ hasText: "優先度を保存しました" })).toBeVisible();

  const t = hh.add("Prioritized");
  await hh.open(page, `/tasks/${t.ulid}`);
  await properties(page).getByRole("button", { name: "優先度" }).click();
  await expect(page.getByRole("menuitem", { name: "blocker" })).toBeVisible();
});

test("a priority renamed in the priorities tab changes the priority of the tasks that hold it", async ({
  page,
  hh,
}) => {
  const t = hh.add("Renamed priority");
  hh.hh("set", t.id, "priority=mid");
  await hh.open(page, `${SCHEMA}?tab=priorities`);
  const values = page.getByRole("textbox", { name: "値" });
  await expect(values.nth(1)).toHaveValue("mid");
  await values.nth(1).fill("medium");
  await page.getByRole("button", { name: "保存" }).click();
  await expect(page.getByRole("status").filter({ hasText: "優先度を保存しました" })).toBeVisible();
  await expect.poll(() => hh.show(t.id).priority).toBe("medium");

  await hh.open(page, `/tasks/${t.ulid}`);
  await expect(properties(page).getByRole("button", { name: "優先度" })).toContainText("medium");
});
