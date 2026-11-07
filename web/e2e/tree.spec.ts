// Screens this spec relies on (see screens.ts for the shared list):
//   - the tree is the "ツリー" tab of the first workspace, /workspaces/<store>/<ulid>/tree (the
//     harness opens it with hh.openTree); the filter box
//     "フィルタ（hh ls と同じクエリ）" starts as "-status:canceled" and the foot of the tree
//     shows the `hh ls` of the filter
//   - a click on a row selects and focuses it; the keys below act on the focused row
//       Enter       opens the textbox "新しいタスクのタイトル" below the row; Enter in it adds
//                   a sibling right after the row: one create request that carries `after`
//                   (one commit), even when siblings hidden by the filter sit in between
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
//   - a tree of more than 200 rows (VIRTUALIZE_AT in app/tree/flatten.ts) is drawn as a window
//     of rows (virtual scrolling); a tree of 200 or fewer draws every row
//   - a drop on the lower quarter of a row whose children are shown makes the task its child;
//     on a row without shown children it puts the task after the row
import { expect, type Hedgehug, test } from "./harness";
import { dragRow, row, toast, treeReady } from "./screens";

/** The number of commits in the store. */
function commitCount(hh: Hedgehug): number {
  return hh.hh("log").trim().split("\n").length;
}

/** The titles of the tasks at the root, in the order of their ranks. */
function rootOrder(hh: Hedgehug): string[] {
  return hh
    .json<{ tasks: { title: string; rank: string; parent: string | null }[] }>("ls")
    .tasks.filter((t) => t.parent === null)
    .sort((x, y) => (x.rank < y.rank ? -1 : 1))
    .map((t) => t.title);
}

test("Enter adds a sibling right below the selected row", async ({ page, hh }) => {
  hh.add("Alpha");
  hh.add("Bravo");
  await hh.openTree(page);

  await row(page, "Alpha").click();
  await page.keyboard.press("Enter");
  await page.getByRole("textbox", { name: "新しいタスクのタイトル" }).fill("Alpha two");
  await page.keyboard.press("Enter");
  await expect(row(page, "Alpha two")).toBeVisible();

  await expect.poll(() => rootOrder(hh)).toEqual(["Alpha", "Alpha two", "Bravo"]);
});

test("Enter adds right below the row in one commit, even past a hidden sibling", async ({
  page,
  hh,
}) => {
  const alpha = hh.add("Alpha");
  hh.add("Bravo");
  // The canceled task sits between Alpha and Bravo; the default filter hides it.
  const hidden = hh.add("Hidden between", "--after", alpha.id);
  hh.hh("set", hidden.id, "status=canceled");
  expect(rootOrder(hh)).toEqual(["Alpha", "Hidden between", "Bravo"]);
  await hh.openTree(page);
  await expect(row(page, "Hidden between")).toHaveCount(0);
  const before = commitCount(hh);

  await row(page, "Alpha").click();
  await page.keyboard.press("Enter");
  await page.getByRole("textbox", { name: "新しいタスクのタイトル" }).fill("Right after");
  await page.keyboard.press("Enter");
  await expect(row(page, "Right after")).toBeVisible();

  await expect
    .poll(() => rootOrder(hh))
    .toEqual(["Alpha", "Right after", "Hidden between", "Bravo"]);
  expect(commitCount(hh)).toBe(before + 1);
});

test("a second title typed straight after the first lands right below it", async ({ page, hh }) => {
  hh.add("Alpha");
  hh.add("Bravo");
  await hh.openTree(page);

  await row(page, "Alpha").click();
  await page.keyboard.press("Enter");
  const input = page.getByRole("textbox", { name: "新しいタスクのタイトル" });
  await input.fill("First");
  await page.keyboard.press("Enter");
  // The input is empty again once the task is made, and keeps the focus while the tree reads the
  // new task: the keys typed now go into it.
  await expect(input).toHaveValue("");
  await expect(input).toBeFocused();
  await page.keyboard.type("Second");
  await page.keyboard.press("Enter");
  await expect(row(page, "Second")).toBeVisible();

  await expect.poll(() => rootOrder(hh)).toEqual(["Alpha", "First", "Second", "Bravo"]);
  await expect(input).toBeFocused();
});

test("Esc closes the input even when the task is still being made, and the row keys work", async ({
  page,
  hh,
}) => {
  hh.add("Alpha");
  hh.add("Bravo");
  await hh.openTree(page);
  await treeReady(page);
  // A slow commit (a signed one, say): the answer of the create comes after the Esc.
  await page.route(/\/api\/tasks\?/, async (route) => {
    if (route.request().method() === "POST") await new Promise((r) => setTimeout(r, 1000));
    await route.continue();
  });

  await row(page, "Alpha").click();
  await page.keyboard.press("Enter");
  const input = page.getByRole("textbox", { name: "新しいタスクのタイトル" });
  await input.fill("Slow");
  await page.keyboard.press("Enter");
  await page.keyboard.press("Escape");
  await expect(input).toHaveCount(0);
  // The create finishes later: the input must not come back and take the focus.
  await expect(row(page, "Slow")).toBeVisible();
  await expect(input).toHaveCount(0);
  await expect(row(page, "Alpha")).toBeFocused();

  // The keys act on the rows again: K opens the menu of kinds, and Alt+Down moves the task.
  await page.keyboard.press("k");
  await expect(page.getByRole("menu")).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(page.getByRole("menu")).toHaveCount(0);
  await page.keyboard.press("Alt+ArrowDown");
  await expect.poll(() => rootOrder(hh)).toEqual(["Slow", "Alpha", "Bravo"]);
});

test("the row keys work after the ID or the chevron of the row was clicked", async ({
  page,
  hh,
}) => {
  const parent = hh.add("Parent");
  hh.add("Child", "--parent", parent.id);
  hh.add("Other");
  await hh.openTree(page);
  await treeReady(page);

  // The ID and the chevron are buttons: a click focuses them, and they stop the keys.
  await row(page, "Other").locator("button.id").click();
  await expect(row(page, "Other")).toBeFocused();
  await expect(row(page, "Other")).toHaveAttribute("aria-selected", "true");
  await row(page, "Parent").locator("button.id").click();
  await expect(row(page, "Parent")).toBeFocused();
  await expect(row(page, "Parent")).toHaveAttribute("aria-selected", "true");
  await page.keyboard.press("k");
  await expect(page.getByRole("menu")).toBeVisible();
  await page.keyboard.press("Escape");

  await row(page, "Parent").locator("button.chev").click();
  await expect(row(page, "Parent")).toBeFocused();
  await page.keyboard.press("Alt+ArrowDown");
  await expect.poll(() => rootOrder(hh)).toEqual(["Other", "Parent"]);
});

test("a click on a row closes the input, also while the task is being made", async ({
  page,
  hh,
}) => {
  hh.add("Alpha");
  hh.add("Bravo");
  await hh.openTree(page);
  await treeReady(page);
  const input = page.getByRole("textbox", { name: "新しいタスクのタイトル" });

  // Typed and not sent: the click closes the input and drops the text.
  await row(page, "Alpha").click();
  await page.keyboard.press("Enter");
  await input.fill("draft");
  await row(page, "Bravo").click();
  await expect(input).toHaveCount(0);
  await expect(row(page, "Bravo")).toBeFocused();
  await expect(row(page, "Bravo")).toHaveAttribute("aria-selected", "true");

  // Sent, and the answer is late: the click closes the input, and it stays closed.
  await page.route(/\/api\/tasks\?/, async (route) => {
    if (route.request().method() === "POST") await new Promise((r) => setTimeout(r, 1000));
    await route.continue();
  });
  await page.keyboard.press("Enter");
  await input.fill("Slow");
  await page.keyboard.press("Enter");
  await row(page, "Alpha").click();
  await expect(input).toHaveCount(0);
  await expect(row(page, "Slow")).toBeVisible();
  await expect(input).toHaveCount(0);
  await expect(row(page, "Alpha")).toBeFocused();
  await page.keyboard.press("k");
  await expect(page.getByRole("menu")).toBeVisible();
});

test("Alt+Up pressed three times in a row moves a task three places, one commit each", async ({
  page,
  hh,
}) => {
  for (const t of ["A", "B", "C", "D", "E"]) hh.add(t);
  await hh.openTree(page);
  await treeReady(page);
  await page.route(/\/api\/tasks\/[^/]+\/move\?/, async (route) => {
    await new Promise((r) => setTimeout(r, 700));
    await route.continue();
  });
  const before = commitCount(hh);

  await row(page, "E").click();
  await page.keyboard.press("Alt+ArrowUp");
  await page.keyboard.press("Alt+ArrowUp");
  await page.keyboard.press("Alt+ArrowUp");

  await expect.poll(() => rootOrder(hh), { timeout: 15000 }).toEqual(["A", "E", "B", "C", "D"]);
  expect(commitCount(hh)).toBe(before + 3);
  await expect(row(page, "E")).toBeFocused();
});

test("presses in a row still all go through when the CLI commits in between", async ({
  page,
  hh,
}) => {
  for (const t of ["A", "B", "C", "D", "E"]) hh.add(t);
  await hh.openTree(page);
  await treeReady(page);
  await page.route(/\/api\/tasks\/[^/]+\/move\?/, async (route) => {
    await new Promise((r) => setTimeout(r, 500));
    await route.continue();
  });

  await row(page, "E").click();
  await page.keyboard.press("Alt+ArrowUp");
  await page.keyboard.press("Alt+ArrowUp");
  await page.keyboard.press("Alt+ArrowUp");
  // Other commits land in the same poll windows as the moves.
  hh.add("Zed one");
  hh.add("Zed two");

  await expect
    .poll(() => rootOrder(hh), { timeout: 15000 })
    .toEqual(["A", "E", "B", "C", "D", "Zed one", "Zed two"]);
  await expect(toast(page, /失敗|できません/)).toHaveCount(0);
});

test("a failed move drops the presses still waiting and says so once", async ({ page, hh }) => {
  for (const t of ["A", "B", "C", "D", "E"]) hh.add(t);
  await hh.openTree(page);
  await treeReady(page);
  let requests = 0;
  await page.route(/\/api\/tasks\/[^/]+\/move\?/, async (route) => {
    requests++;
    await new Promise((r) => setTimeout(r, 300));
    await route.abort();
  });

  await row(page, "E").click();
  await page.keyboard.press("Alt+ArrowUp");
  await page.keyboard.press("Alt+ArrowUp");
  await page.keyboard.press("Alt+ArrowUp");

  await expect(page.locator(".toast")).toHaveCount(1);
  await page.waitForTimeout(1000);
  expect(requests).toBe(1);
  expect(rootOrder(hh)).toEqual(["A", "B", "C", "D", "E"]);
});

test("Tab indents under the previous sibling and Shift+Tab brings it back", async ({
  page,
  hh,
}) => {
  const a = hh.add("Alpha");
  const b = hh.add("Bravo");
  await hh.openTree(page);

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
  await hh.openTree(page);

  await row(page, "Charlie").click();
  await page.keyboard.press("Alt+ArrowUp");
  await expect.poll(() => rootOrder(hh)).toEqual(["Alpha", "Charlie", "Bravo"]);
});

test("Alt+Up pressed twice moves a task two places", async ({ page, hh }) => {
  hh.add("Alpha");
  hh.add("Bravo");
  hh.add("Charlie");
  hh.add("Delta");
  await hh.openTree(page);

  await row(page, "Delta").click();
  await page.keyboard.press("Alt+ArrowUp");
  await expect.poll(() => rootOrder(hh)).toEqual(["Alpha", "Bravo", "Delta", "Charlie"]);
  // The page moves the row when it has read the tree again; the key then acts on the rows as
  // they are now, and the row still has the focus.
  await expect(page.getByRole("treeitem")).toHaveText([/Alpha/, /Bravo/, /Delta/, /Charlie/]);
  await expect(row(page, "Delta")).toBeFocused();
  await page.keyboard.press("Alt+ArrowUp");
  await expect.poll(() => rootOrder(hh)).toEqual(["Alpha", "Delta", "Bravo", "Charlie"]);
  await expect(row(page, "Delta")).toBeFocused();
});

test("Alt+Down pressed twice moves a task two places", async ({ page, hh }) => {
  hh.add("Alpha");
  hh.add("Bravo");
  hh.add("Charlie");
  hh.add("Delta");
  await hh.openTree(page);

  await row(page, "Alpha").click();
  await page.keyboard.press("Alt+ArrowDown");
  await expect.poll(() => rootOrder(hh)).toEqual(["Bravo", "Alpha", "Charlie", "Delta"]);
  await expect(page.getByRole("treeitem")).toHaveText([/Bravo/, /Alpha/, /Charlie/, /Delta/]);
  await expect(row(page, "Alpha")).toBeFocused();
  await page.keyboard.press("Alt+ArrowDown");
  await expect.poll(() => rootOrder(hh)).toEqual(["Bravo", "Charlie", "Alpha", "Delta"]);
  await expect(page.getByRole("treeitem")).toHaveText([/Bravo/, /Charlie/, /Alpha/, /Delta/]);
  await expect(row(page, "Alpha")).toBeFocused();
});

test("Space toggles done and back", async ({ page, hh }) => {
  const t = hh.add("Finish me");
  await hh.openTree(page);

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
  await hh.openTree(page);
  await treeReady(page);

  await dragRow(page, row(page, "Alpha"), row(page, "Charlie"), "after");
  await expect.poll(() => rootOrder(hh)).toEqual(["Bravo", "Charlie", "Alpha"]);
  // Wait for the page to show the new order before the next drag measures the rows.
  await expect(page.getByRole("treeitem").last()).toContainText("Alpha");

  await dragRow(page, row(page, "Bravo"), row(page, "Charlie"), "child");
  await expect.poll(() => hh.show(b.id).parent).toBe(c.ulid);
  await expect(row(page, "Bravo")).toHaveAttribute("aria-level", "2");
});

test("dropping on the lower quarter of an open parent makes the task its child", async ({
  page,
  hh,
}) => {
  const parent = hh.add("Parent");
  hh.add("Existing child", "--parent", parent.id);
  const moved = hh.add("Mover");
  await hh.openTree(page);
  await treeReady(page);
  await expect(row(page, "Existing child")).toBeVisible();

  await dragRow(page, row(page, "Mover"), row(page, "Parent"), "after");
  await expect.poll(() => hh.show(moved.id).parent).toBe(parent.ulid);
  await expect(row(page, "Mover")).toHaveAttribute("aria-level", "2");
});

test("a drop the schema does not allow says why and changes nothing", async ({ page, hh }) => {
  // The default schema lets an epic sit at the root or under a project only.
  const epic = hh.add("Big epic", "--kind", "epic");
  hh.add("Small task");
  await hh.openTree(page);
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
  await hh.openTree(page);
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
  await hh.openTree(page);
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
  await hh.openTree(page);

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
  // One request at a time: the order of the ranks is the order of the titles. Requests made
  // at the same time were slower (the server writes one at a time and they fight for the
  // lock: 15 s against 9 s for 300 tasks) and made the order a race.
  for (let i = 1; i <= 300; i++) {
    await hh.post("/api/tasks", { title: `Task ${String(i).padStart(3, "0")}` });
  }
  const first = "Task 001";
  const last = "Task 300";
  await hh.openTree(page);

  const rows = page.getByRole("treeitem");
  await expect(row(page, first)).toBeVisible();
  // Only the rows near the window are in the page.
  expect(await rows.count()).toBeLessThan(100);
  await expect(row(page, last)).toHaveCount(0);

  // Scroll to the end: the last row is drawn then, and the rows far from it are not.
  await row(page, first).hover();
  await page.mouse.wheel(0, 1_000_000);
  await expect(row(page, last)).toBeVisible();
  await expect(row(page, first)).toHaveCount(0);
  expect(await rows.count()).toBeLessThan(100);
});
