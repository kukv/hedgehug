// Screens this spec relies on (see screens.ts for the shared list):
//   - /workspaces/<store>/<ulid>/table shows a row for each task, whose cells are inputs named by
//     the column and the ID: the textbox "タイトル: <ID>", the button "ステータス: <ID>" (it opens a
//     menu of the statuses), and so on. A cell saves 2 seconds after the last key or when it loses
//     the focus, as the detail does
import { expect, type Hedgehug, test } from "./harness";

async function tab(hh: Hedgehug, name: string): Promise<string> {
  return (await hh.treePath()).replace(/\/tree$/, `/${name}`);
}

test("editing the title of a cell saves the task when the cell loses the focus", async ({
  page,
  hh,
}) => {
  const alpha = hh.add("Old title");
  const other = hh.add("Other");
  await hh.open(page, await tab(hh, "table"));

  const title = page.getByRole("textbox", { name: `タイトル: ${alpha.id}` });
  await expect(title).toHaveValue("Old title");
  await title.fill("New title");
  await title.blur();
  await expect.poll(() => hh.show(alpha.id).title).toBe("New title");
  expect(hh.show(other.id).title).toBe("Other");
});

test("the status cell chooses a status from a menu", async ({ page, hh }) => {
  const alpha = hh.add("Alpha");
  await hh.open(page, await tab(hh, "table"));

  await page.getByRole("button", { name: `ステータス: ${alpha.id}` }).click();
  await page.getByRole("menuitem", { name: /進行中/ }).click();
  await expect.poll(() => hh.show(alpha.id).status).toBe("in_progress");
});

test("the filter field narrows the rows", async ({ page, hh }) => {
  const alpha = hh.add("Alpha");
  const bravo = hh.add("Bravo", "--status", "in_progress");
  await hh.open(page, await tab(hh, "table"));
  await expect(page.getByRole("textbox", { name: `タイトル: ${alpha.id}` })).toBeVisible();

  const filter = page.getByRole("textbox", { name: "フィルタ（hh ls と同じクエリ）" });
  await filter.fill("status:in_progress");
  await filter.press("Enter");
  await expect(page.getByRole("textbox", { name: `タイトル: ${bravo.id}` })).toBeVisible();
  await expect(page.getByRole("textbox", { name: `タイトル: ${alpha.id}` })).toHaveCount(0);
});
