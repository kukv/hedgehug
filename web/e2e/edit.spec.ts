// Screens this spec relies on (see screens.ts for the shared list):
//   - /tasks/<ulid> is the detail of a task
//   - the title is the textbox "タイトル"; the body is the textbox "本文" (CodeMirror)
//   - the property panel (region "プロパティ") has the button "ステータス" that opens a menu
//     of menuitems named "<label> · <name>" (Todo · todo, In progress · in_progress …), the
//     button "priority" with the menuitems low, mid, high, urgent, and the date input "due"
//   - the save state is the status "保存の状態": "保存済み" (or "保存済み · push 待ち")
//     once a change is on disk
//   - a change is saved 2 seconds after the last key, or when the field loses focus
import { expect, test } from "./harness";
import { properties, saved } from "./screens";

test("the title is saved when the field loses focus", async ({ page, hh }) => {
  const t = hh.add("Before");
  await hh.open(page, `/tasks/${t.ulid}`);
  const title = page.getByRole("textbox", { name: "タイトル" });
  await expect(title).toHaveValue("Before");

  await title.fill("After");
  await page.keyboard.press("Tab");
  await expect(saved(page)).toBeVisible();
  expect(hh.show(t.id).title).toBe("After");
});

test("a change is saved after two seconds without typing", async ({ page, hh }) => {
  const t = hh.add("Before");
  await hh.open(page, `/tasks/${t.ulid}`);
  const title = page.getByRole("textbox", { name: "タイトル" });

  await title.fill("Typed and left alone");
  await expect(title).toBeFocused();
  expect(hh.show(t.id).title).toBe("Before"); // nothing is written while the user types
  await expect(saved(page)).toBeVisible({ timeout: 8000 });
  expect(hh.show(t.id).title).toBe("Typed and left alone");
});

test("the status, an enum property and a date are saved from the property panel", async ({
  page,
  hh,
}) => {
  const t = hh.add("Props");
  await hh.open(page, `/tasks/${t.ulid}`);
  const panel = properties(page);

  await panel.getByRole("button", { name: "ステータス" }).click();
  await page.getByRole("menuitem", { name: /^In progress/ }).click();
  await expect(panel.getByRole("button", { name: "ステータス" })).toContainText("In progress");

  await panel.getByRole("button", { name: "優先度" }).click();
  await page.getByRole("menuitem", { name: "high" }).click();

  await panel.getByLabel("期限").fill("2026-12-01");
  await panel.getByLabel("期限").blur();

  await expect
    .poll(() => {
      const s = hh.show(t.id);
      return [s.status, s.props.priority, s.props.due];
    })
    .toEqual(["in_progress", "high", "2026-12-01"]);
  await expect(saved(page)).toBeVisible();
});

test("the body is saved from the Markdown editor, and a checkbox toggles from the list", async ({
  page,
  hh,
}) => {
  const t = hh.add("With a body");
  await hh.open(page, `/tasks/${t.ulid}`);
  const body = page.getByRole("textbox", { name: "本文" });

  await body.click();
  // Enter in a list item starts the next item, as in any Markdown editor.
  await page.keyboard.type("- [ ] first step\nsecond step");
  await page.getByRole("textbox", { name: "タイトル" }).click(); // the editor loses focus
  await expect(saved(page)).toBeVisible();
  await expect.poll(() => hh.show(t.id).body?.trim()).toBe("- [ ] first step\n- [ ] second step");

  await expect(page.getByRole("heading", { name: "チェックリスト 0/2" })).toBeVisible();
  await page.getByRole("checkbox", { name: "first step" }).check();
  await expect.poll(() => hh.show(t.id).body?.trim()).toBe("- [x] first step\n- [ ] second step");
});

test("a change from the CLI does not overwrite the field being edited", async ({ page, hh }) => {
  const t = hh.add("Original");
  await hh.open(page, `/tasks/${t.ulid}`);
  const title = page.getByRole("textbox", { name: "タイトル" });

  await title.click();
  await page.keyboard.press("End");
  await page.keyboard.type(" mine");
  await expect(title).toHaveValue("Original mine");

  // The page fetches the task again when the event arrives; wait for that.
  const refetched = page.waitForResponse(
    (r) => r.request().method() === "GET" && new URL(r.url()).pathname === `/api/tasks/${t.ulid}`,
  );
  hh.hh("set", t.id, "title=Theirs");
  await refetched;
  await expect(title).toHaveValue("Original mine");

  await page.keyboard.press("Tab"); // the last writer wins
  await expect(saved(page)).toBeVisible();
  await expect.poll(() => hh.show(t.id).title).toBe("Original mine");
});
