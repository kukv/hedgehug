// Screens this spec relies on (see screens.ts for the shared list):
//   - the sidebar (navigation "メインナビゲーション") links a workspace by its name; a Store that is
//     hidden has none
//   - /query/tree?q= is the tree of every Store (heading "クエリ"); a row is a treeitem with its ID and
//     title
//   - Ctrl+K (⌘K on a Mac) opens the palette "ID・検索・コマンド" with the combobox; a task found is an
//     option "<ID> <title>"
//   - the detail of a task of a hidden Store has the status "非表示" ("この Store は非表示です") with
//     the button "表示する"; the toast says "<名前> を表示しました"
//   - /settings/stores lists each Store as the article "<名前>"; the button "<名前> の色を変える
//     （このマシンだけ）" holds the dot with [data-tone]
import type { Locator, Page } from "@playwright/test";
import { expect, type Hedgehug, test } from "./harness";
import { row, toast } from "./screens";

const side = (page: Page): Locator =>
  page.getByRole("navigation", { name: "メインナビゲーション" });
const hiddenOf = (hh: Hedgehug, name: string): boolean | undefined =>
  hh.json<{ name: string; hidden: boolean }[]>("store", "list").find((s) => s.name === name)
    ?.hidden;
const dot = (page: Page): Locator =>
  page
    .getByRole("article", { name: "second" })
    .getByRole("button", { name: "second の色を変える（このマシンだけ）" })
    .locator("[data-tone]");

test("a hidden Store is left out of the sidebar, the tree and the palette, and comes back by the band", async ({
  page,
  hh,
}) => {
  hh.add("Demo task");
  hh.hh("store", "init", "second", "--prefix", "SC");
  const second = hh.add("Second task", "--store", "second");
  expect(second.id).toBe("SC-1");

  // Before: both Stores are listed and found.
  await hh.open(page, "/query/tree?q=");
  await expect(side(page).getByRole("link", { name: /second/ })).toBeVisible();
  await expect(row(page, "Second task")).toBeVisible();

  hh.hh("store", "set", "second", "--hidden=true");
  expect(hiddenOf(hh, "second")).toBe(true);

  // The sidebar and the tree of every Store leave it out (the page follows the event).
  await expect(side(page).getByRole("link", { name: /second/ })).toHaveCount(0);
  await expect(row(page, "Second task")).toHaveCount(0);
  await expect(row(page, "Demo task")).toBeVisible();

  // The palette does not find its task by the title.
  await page.keyboard.press("ControlOrMeta+k");
  const palette = page.getByRole("dialog", { name: "ID・検索・コマンド" });
  await palette.getByRole("combobox").fill("Second task");
  await expect(palette.getByRole("option", { name: /Demo task/ })).toHaveCount(0);
  await palette.getByRole("combobox").fill("Demo");
  await expect(palette.getByRole("option", { name: /Demo task/ })).toBeVisible();
  await expect(palette.getByRole("option", { name: /Second task/ })).toHaveCount(0);
  await page.keyboard.press("Escape");

  // Opened by its address, the task is shown, with the band to show the Store again.
  await hh.open(page, `/tasks/${second.ulid}`);
  await expect(page.getByRole("textbox", { name: "タイトル" })).toHaveValue("Second task");
  await expect(page.getByRole("status", { name: "非表示" })).toHaveText(
    /この Store は非表示です\s*表示する/,
  );
  await page.getByRole("button", { name: "表示する" }).click();
  await expect(toast(page, "second を表示しました")).toBeVisible();
  await expect(page.getByRole("status", { name: "非表示" })).toHaveCount(0);
  await expect(side(page).getByRole("link", { name: /second/ })).toBeVisible();
  expect(hiddenOf(hh, "second")).toBe(false);
});

test("the colour of a Store is the same after it is hidden and shown again", async ({
  page,
  hh,
}) => {
  hh.hh("store", "init", "second", "--prefix", "SC");
  await hh.open(page, "/settings/stores");
  await expect(dot(page)).toHaveAttribute("data-tone", "orange");

  // The settings do not leave a hidden Store out, and it keeps its colour there.
  hh.hh("store", "set", "second", "--hidden=true");
  await expect(
    page.getByRole("article", { name: "second" }).getByRole("checkbox", { name: "表示" }),
  ).not.toBeChecked();
  await expect(dot(page)).toHaveAttribute("data-tone", "orange");

  hh.hh("store", "set", "second", "--hidden=false");
  await expect(
    page.getByRole("article", { name: "second" }).getByRole("checkbox", { name: "表示" }),
  ).toBeChecked();
  await expect(dot(page)).toHaveAttribute("data-tone", "orange");
});
