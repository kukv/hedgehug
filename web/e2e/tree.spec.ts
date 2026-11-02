// Screens this spec relies on (see screens.ts for the shared list):
//   - "/" is the tree of the first workspace; the filter box "フィルタ（hh ls と同じクエリ）"
//     starts as "-status:canceled" and the foot of the tree shows the `hh ls` of the filter
//   - a click on a row selects and focuses it; the keys below act on the focused row
//       Enter       opens the textbox "新しいタスクのタイトル" below the row; Enter in it adds
//                   a sibling after the row (an add, then a move: two commits)
//       Tab/Shift+Tab   indent under the previous sibling / outdent after the parent
//       Alt+Up      move before the previous sibling
//       Space       toggles between the done and the todo default statuses
//   - a drag over a row's upper quarter puts the task before it, the lower quarter after it,
//     the middle makes it a child; a target the schema does not allow shows the chip
//     "ここには置けません" on the row and the status with the reason under the tree, and a
//     drop there shows a toast with the reason and changes nothing
//   - the row being dragged carries the class "dragging" and the refusal is shown only while
//     the pointer is over a refused target; both go away when the drag ends. The refusal lies
//     over the foot of the tree, which leaves room below the last row to scroll it clear
//   - a tree of more than 200 rows is drawn as a window of rows (virtual scrolling)
import { expect, type Hedgehug, test } from "./harness";
import { dragRow, row, toast, treeReady } from "./screens";

/** The titles of the tasks at the root, in the order of their ranks. */
function rootOrder(hh: Hedgehug): string[] {
  return hh
    .json<{ title: string; rank: string; parent: string | null }[]>("ls")
    .filter((t) => t.parent === null)
    .sort((x, y) => (x.rank < y.rank ? -1 : 1))
    .map((t) => t.title);
}

test("Enter adds a sibling right below the selected row", async ({ page, hh }) => {
  hh.add("Alpha");
  hh.add("Bravo");
  await hh.open(page);

  await row(page, "Alpha").click();
  await page.keyboard.press("Enter");
  await page.getByRole("textbox", { name: "新しいタスクのタイトル" }).fill("Alpha two");
  await page.keyboard.press("Enter");
  await expect(row(page, "Alpha two")).toBeVisible();

  await expect.poll(() => rootOrder(hh)).toEqual(["Alpha", "Alpha two", "Bravo"]);
});

test("Tab indents under the previous sibling and Shift+Tab brings it back", async ({
  page,
  hh,
}) => {
  const a = hh.add("Alpha");
  const b = hh.add("Bravo");
  await hh.open(page);

  await row(page, "Bravo").click();
  await page.keyboard.press("Tab");
  await expect.poll(() => hh.show(b.id).parent).toBe(a.ulid);
  await expect(row(page, "Bravo")).toHaveAttribute("aria-level", "2");

  await page.keyboard.press("Shift+Tab");
  await expect.poll(() => hh.show(b.id).parent).toBeNull();
  await expect(row(page, "Bravo")).toHaveAttribute("aria-level", "1");
});

test("Alt+Up moves a task before its previous sibling", async ({ page, hh }) => {
  hh.add("Alpha");
  hh.add("Bravo");
  hh.add("Charlie");
  await hh.open(page);

  await row(page, "Charlie").click();
  await page.keyboard.press("Alt+ArrowUp");
  await expect.poll(() => rootOrder(hh)).toEqual(["Alpha", "Charlie", "Bravo"]);
});

test("Space toggles done and back", async ({ page, hh }) => {
  const t = hh.add("Finish me");
  await hh.open(page);

  await row(page, "Finish me").click();
  await page.keyboard.press("Space");
  await expect.poll(() => hh.show(t.id).status).toBe("done");
  // The next press reads the status the row shows: wait until the row shows done.
  await expect(row(page, "Finish me").locator('svg[data-shape="check"]')).toBeVisible();
  await page.keyboard.press("Space");
  await expect.poll(() => hh.show(t.id).status).toBe("todo");
});

test("dragging a row after another reorders; onto the middle makes it a child", async ({
  page,
  hh,
}) => {
  hh.add("Alpha");
  const b = hh.add("Bravo");
  const c = hh.add("Charlie");
  await hh.open(page);
  await treeReady(page);

  await dragRow(page, row(page, "Alpha"), row(page, "Charlie"), "after");
  await expect.poll(() => rootOrder(hh)).toEqual(["Bravo", "Charlie", "Alpha"]);
  // Wait for the page to show the new order before the next drag measures the rows.
  await expect(page.getByRole("treeitem").last()).toContainText("Alpha");

  await dragRow(page, row(page, "Bravo"), row(page, "Charlie"), "child");
  await expect.poll(() => hh.show(b.id).parent).toBe(c.ulid);
  await expect(row(page, "Bravo")).toHaveAttribute("aria-level", "2");
});

test("a drop the schema does not allow says why and changes nothing", async ({ page, hh }) => {
  // The default schema lets an epic sit at the root or under a project only.
  const epic = hh.add("Big epic", "--kind", "epic");
  hh.add("Small task");
  await hh.open(page);
  await treeReady(page);

  await dragRow(page, row(page, "Big epic"), row(page, "Small task"), "child", async () => {
    await expect(page.getByText("ここには置けません")).toBeVisible();
    await expect(page.getByText("エピックはタスクの下に置けません")).toBeVisible();
  });
  await expect(toast(page, "エピックを置ける親はルートとプロジェクトだけです")).toBeVisible();
  expect(hh.show(epic.id).parent).toBeNull();
});

test("the dragging mark and the refusal go away when the drag ends", async ({ page, hh }) => {
  hh.add("Big epic", "--kind", "epic");
  hh.add("Small task");
  await hh.open(page);
  await treeReady(page);

  const marked = page.locator(".row.dragging");
  // The drop also shows a toast with the same words, so take the panel under the tree.
  const refusal = page.locator("aside.refusal");
  await dragRow(page, row(page, "Big epic"), row(page, "Small task"), "child", async () => {
    await expect(marked).toHaveCount(1);
    await expect(refusal).toBeVisible();
  });
  await expect(marked).toHaveCount(0);
  await expect(refusal).toHaveCount(0);

  // A drop that is allowed ends the drag as well.
  await dragRow(page, row(page, "Small task"), row(page, "Big epic"), "before");
  await expect(marked).toHaveCount(0);
});

test("the refusal does not cover the last row once the tree is scrolled to its end", async ({
  page,
  hh,
}) => {
  await hh.post("/api/tasks", { title: "Big epic", kind: "epic" });
  for (let i = 1; i <= 40; i++) {
    await hh.post("/api/tasks", { title: `Filler ${String(i).padStart(2, "0")}` });
  }
  await hh.open(page);
  await treeReady(page);

  await dragRow(page, row(page, "Big epic"), row(page, "Filler 01"), "child", async () => {
    const refusal = page.locator("aside.refusal");
    await expect(refusal).toBeVisible();
    const last = row(page, "Filler 40");
    // Scroll again on every poll: the room under the list grows once the panel is measured.
    // (Not the wheel: the browser does not reliably scroll under a drag in progress.)
    await expect
      .poll(async () => {
        await page.locator(".scroller").evaluate((el) => {
          el.scrollTop = el.scrollHeight;
        });
        const l = await last.boundingBox();
        const r = await refusal.boundingBox();
        return l && r ? r.y - (l.y + l.height) : -1;
      })
      .toBeGreaterThanOrEqual(0);
  });
});

test("the filter box narrows the tree and shows the command", async ({ page, hh }) => {
  hh.add("Keep this");
  hh.add("Hide this");
  const canceled = hh.add("Dropped");
  hh.hh("set", canceled.id, "status=canceled");
  await hh.open(page);

  await expect(row(page, "Keep this")).toBeVisible();
  await expect(row(page, "Dropped")).toHaveCount(0); // canceled tasks are out by default
  await expect(page.getByText("中止 1 件を除外")).toBeVisible();

  const filter = page.getByLabel("フィルタ（hh ls と同じクエリ）");
  await filter.fill("keep");
  await expect(row(page, "Hide this")).toHaveCount(0);
  await expect(row(page, "Keep this")).toBeVisible();
  await expect(page.getByText(/hh ls .*keep/)).toBeVisible();
});

test("a tree of 300 tasks is drawn as a window of rows and scrolls to the end", async ({
  page,
  hh,
}) => {
  test.setTimeout(120_000);
  for (let i = 1; i <= 300; i++) {
    await hh.post("/api/tasks", { title: `Task ${String(i).padStart(3, "0")}` });
  }
  await hh.open(page);

  const rows = page.getByRole("treeitem");
  await expect(row(page, "Task 001")).toBeVisible();
  // Only the rows near the window are in the page.
  expect(await rows.count()).toBeLessThan(100);
  await expect(row(page, "Task 300")).toHaveCount(0);

  // Scroll to the end: the last row is drawn then, and the rows far from it are not.
  await row(page, "Task 001").hover();
  await page.mouse.wheel(0, 1_000_000);
  await expect(row(page, "Task 300")).toBeVisible();
  await expect(row(page, "Task 001")).toHaveCount(0);
  expect(await rows.count()).toBeLessThan(100);
});
