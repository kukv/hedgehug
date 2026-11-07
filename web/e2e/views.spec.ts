// Screens this spec relies on (see screens.ts for the shared list):
//   - the tab "ツリー" of a workspace has the button "ビューを保存"; it opens the dialog "ビューを保存"
//     (the textbox "名前", the buttons "保存" and "キャンセル", the line of the command hh view save).
//     Saving shows the toast "ビュー <名前> を <Store> に保存しました" and opens /views/<Store>/<名前>
//   - /views/<Store>/<名前> has the heading (level 1) <名前>, the navigation "レイアウトの切り替え"
//     (the links ツリー, カンバン, テーブル; the one of the saved layout is aria-current="page"; a
//     click saves the layout and keeps the address), the board "カンバンの盤面" of the kanban, the
//     table with the textboxes "タイトル: <ID>", the status "未保存の変更" with the button
//     "ビューを更新" once the filter differs from the saved query, and the button "ビューの操作
//     （改名・削除）" whose menu has 改名… and 削除…
//   - rename: the dialog "ビュー <名前> の名前を変更" (the textbox "新しい名前", the button "変更", the toast
//     "ビュー <a> を <b> に改名しました"); delete: the dialog "ビュー <名前> を削除" (the button
//     "削除", the toast "ビュー <名前> を削除しました" with the button "取り消し")
//   - /query/<layout>?q= is the heading "クエリ" with the same navigation and the button
//     "ビューを保存"; the filter field writes the query into the address, and the tabs keep it
//   - the sidebar has the region "保存済みビュー" with a link for each view (its title says where it
//     is saved: "<Store> の views/ に保存"); the palette lists the views as "<名前> を開く", and a
//     query that found tasks has the group "このクエリで" (テーブルで開く, ビューとして保存…)
import type { Page } from "@playwright/test";
import { expect, test } from "./harness";
import { row, toast } from "./screens";

type SavedView = { name: string; layout: string; query: string };

const sidebar = (page: Page) => page.getByRole("navigation", { name: "メインナビゲーション" });
const saved = (page: Page) => sidebar(page).getByRole("region", { name: "保存済みビュー" });
const layouts = (page: Page) => page.getByRole("navigation", { name: "レイアウトの切り替え" });
const filter = (page: Page) =>
  page.getByRole("textbox", { name: "フィルタ（hh ls と同じクエリ）" });
const board = (page: Page) => page.locator('section[aria-label="カンバンの盤面"]');
const palette = (page: Page) => page.getByRole("dialog", { name: "ID・検索・コマンド" });

test("a tree saved as a view opens, changes its layout, is updated, renamed and deleted", async ({
  page,
  hh,
}) => {
  const doing = hh.add("Doing it", "--status", "in_progress");
  hh.add("Not yet");
  const views = () => hh.json<SavedView[]>("view", "list");

  await hh.openTree(page);
  await filter(page).fill("status:in_progress");
  await filter(page).press("Enter");
  await expect(row(page, "Doing it")).toBeVisible();
  await expect(row(page, "Not yet")).toHaveCount(0);

  // save what the tree shows
  await page.getByRole("button", { name: "ビューを保存" }).click();
  const save = page.getByRole("dialog", { name: "ビューを保存" });
  await save.getByRole("textbox", { name: "名前" }).fill("doing");
  await expect(save.getByRole("button", { name: /コマンドをコピー/ })).toContainText(
    "hh view save doing",
  );
  await save.getByRole("button", { name: "保存", exact: true }).click();
  await expect(toast(page, "ビュー doing を demo に保存しました")).toBeVisible();
  await expect(page).toHaveURL(`${hh.origin}/views/demo/doing`);
  expect(views()).toMatchObject([
    { name: "doing", layout: "tree", query: "workspace:demo status:in_progress" },
  ]);

  // it opens as saved: the tasks of the query, the sidebar marks it
  await expect(page.getByRole("heading", { level: 1, name: "doing" })).toBeVisible();
  await expect(row(page, "Doing it")).toBeVisible();
  await expect(row(page, "Not yet")).toHaveCount(0);
  await expect(saved(page).getByRole("link", { name: "doing" })).toHaveAttribute(
    "aria-current",
    "page",
  );
  await expect(saved(page).getByRole("link", { name: "doing" })).toHaveAttribute(
    "title",
    "demo の views/ に保存",
  );

  // the tabs change the layout of the saved view, and the address stays
  await layouts(page).getByRole("link", { name: "カンバン" }).click();
  await expect.poll(() => views()[0]?.layout).toBe("kanban");
  await expect(board(page)).toBeVisible();
  await expect(board(page).locator(`li[data-ulid="${doing.ulid}"]`)).toBeVisible();
  await expect(page).toHaveURL(`${hh.origin}/views/demo/doing`);
  await page.reload();
  await expect(board(page)).toBeVisible();

  await layouts(page).getByRole("link", { name: "テーブル" }).click();
  await expect.poll(() => views()[0]?.layout).toBe("table");
  await expect(page.getByRole("textbox", { name: `タイトル: ${doing.id}` })).toBeVisible();

  // a changed filter is not saved until the view is updated
  await expect(page.getByText("未保存の変更")).toHaveCount(0);
  // the field of a saved view holds the whole query: nothing is fixed on a screen of every store
  await expect(filter(page)).toHaveValue("workspace:demo status:in_progress");
  await filter(page).fill("workspace:demo status:in_progress kind:task");
  await filter(page).press("Enter");
  await expect(page.getByText("未保存の変更")).toBeVisible();
  expect(views()[0]?.query).toBe("workspace:demo status:in_progress");
  await page.getByRole("button", { name: "ビューを更新" }).click();
  await expect.poll(() => views()[0]?.query).toBe("workspace:demo status:in_progress kind:task");
  await expect(page.getByText("未保存の変更")).toHaveCount(0);

  // rename
  await page.getByRole("button", { name: "ビューの操作（名前の変更・削除）" }).click();
  await page.getByRole("menuitem", { name: "名前を変更…" }).click();
  const rename = page.getByRole("dialog", { name: "ビュー doing の名前を変更" });
  await rename.getByRole("textbox", { name: "新しい名前" }).fill("busy");
  await rename.getByRole("button", { name: "変更", exact: true }).click();
  await expect(toast(page, "ビュー doing の名前を busy に変更しました")).toBeVisible();
  await expect(page).toHaveURL(`${hh.origin}/views/demo/busy`);
  expect(views().map((v) => v.name)).toEqual(["busy"]);
  await expect(saved(page).getByRole("link", { name: "busy" })).toBeVisible();

  // delete, and take it back (at once: the undo comes before the poll of the server has seen the
  // delete, so no views.changed comes, and the tab reads the views again after the undo itself)
  await page.getByRole("button", { name: "ビューの操作（名前の変更・削除）" }).click();
  await page.getByRole("menuitem", { name: "削除…" }).click();
  const remove = page.getByRole("dialog", { name: "ビュー busy を削除" });
  await remove.getByRole("button", { name: "削除", exact: true }).click();
  const gone = toast(page, "ビュー busy を削除しました");
  await expect(gone).toBeVisible();
  expect(views()).toEqual([]);
  await expect(saved(page)).toHaveCount(0);
  await expect(page).toHaveURL(`${hh.origin}/`);

  await gone.getByRole("button", { name: "取り消し" }).click();
  await expect.poll(() => views().map((v) => v.name)).toEqual(["busy"]);
  await expect(saved(page).getByRole("link", { name: "busy" })).toBeVisible();
});

test("a view the CLI saves appears in the sidebar and the palette, and opens", async ({
  page,
  hh,
}) => {
  const alpha = hh.add("Alpha");
  hh.add("Bravo", "--status", "in_progress");
  await hh.open(page);
  await expect(saved(page)).toHaveCount(0);

  hh.hh("view", "save", "from-cli", "--query", "status:todo", "--layout", "table");
  const link = saved(page).getByRole("link", { name: "from-cli" });
  await expect(link).toBeVisible();

  await link.click();
  await expect(page).toHaveURL(`${hh.origin}/views/demo/from-cli`);
  await expect(page.getByRole("textbox", { name: `タイトル: ${alpha.id}` })).toBeVisible();
  await expect(page.getByRole("textbox", { name: /^タイトル: / })).toHaveCount(1);

  // and the palette opens it from anywhere
  await page.goBack();
  await expect(page).toHaveURL(`${hh.origin}/`);
  await page.keyboard.press("ControlOrMeta+k");
  const box = palette(page).getByRole("combobox");
  await box.fill(">from-cli");
  await expect(palette(page).getByRole("option", { name: /from-cli を開く/ })).toBeVisible();
  await box.press("Enter");
  await expect(page).toHaveURL(`${hh.origin}/views/demo/from-cli`);
});

test("a query from the palette opens as a table, changes layout without saving, and is saved", async ({
  page,
  hh,
}) => {
  const alpha = hh.add("Alpha");
  const bravo = hh.add("Bravo", "--status", "in_progress");
  const views = () => hh.json<SavedView[]>("view", "list");
  await hh.open(page);

  await page.keyboard.press("ControlOrMeta+k");
  const box = palette(page).getByRole("combobox");
  await box.fill("status:in_progress");
  await page.getByRole("option", { name: /テーブルで開く/ }).click();
  await expect(page).toHaveURL(/\/query\/table\?q=status%3Ain_progress$/);
  await expect(page.getByRole("heading", { level: 1, name: "クエリ" })).toBeVisible();
  await expect(page.getByRole("textbox", { name: `タイトル: ${bravo.id}` })).toBeVisible();
  await expect(page.getByRole("textbox", { name: `タイトル: ${alpha.id}` })).toHaveCount(0);

  // the tab changes the layout and keeps the query; nothing is saved
  await layouts(page).getByRole("link", { name: "カンバン" }).click();
  await expect(page).toHaveURL(/\/query\/kanban\?q=status%3Ain_progress$/);
  await expect(board(page).locator(`li[data-ulid="${bravo.ulid}"]`)).toBeVisible();
  expect(views()).toEqual([]);

  // the field writes the query into the address
  await filter(page).fill("status:todo");
  await filter(page).press("Enter");
  await expect(page).toHaveURL(/\/query\/kanban\?q=status%3Atodo$/);
  await expect(board(page).locator(`li[data-ulid="${alpha.ulid}"]`)).toBeVisible();

  // and the screen saves it as a view in the layout it shows
  await page.getByRole("button", { name: "ビューを保存" }).click();
  const save = page.getByRole("dialog", { name: "ビューを保存" });
  await save.getByRole("textbox", { name: "名前" }).fill("todo-board");
  await save.getByRole("button", { name: "保存", exact: true }).click();
  await expect(page).toHaveURL(`${hh.origin}/views/demo/todo-board`);
  expect(views()).toMatchObject([{ name: "todo-board", layout: "kanban", query: "status:todo" }]);
});

test("the palette saves the query it was given as a view", async ({ page, hh }) => {
  hh.add("Alpha");
  await hh.open(page);

  await page.keyboard.press("ControlOrMeta+k");
  await palette(page).getByRole("combobox").fill("status:todo");
  await page.getByRole("option", { name: /ビューとして保存/ }).click();
  const save = page.getByRole("dialog", { name: "ビューを保存" });
  await expect(save).toContainText("status:todo");
  await save.getByRole("textbox", { name: "名前" }).fill("todo-table");
  await save.getByRole("button", { name: "保存", exact: true }).click();
  await expect(page).toHaveURL(`${hh.origin}/views/demo/todo-table`);
  expect(hh.json<SavedView[]>("view", "list")).toMatchObject([
    { name: "todo-table", layout: "table", query: "status:todo" },
  ]);
});
