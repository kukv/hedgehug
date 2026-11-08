// Screens this spec relies on (see screens.ts for the shared list):
//   - a row changed by someone else (the CLI) shows "CLI で更新 · 今" for 10 seconds
//   - a change the page made itself shows no such mark
//   - hh.open waits for the page's PUT /api/presence, which it sends once the event
//     stream is open, so a change made by the CLI after hh.open reaches the page as an event
//   - the sidebar shows the sync card, with the button "今すぐ同期"
//   - the detail at /tasks/<ulid> follows the store: the button "ステータス" shows the label of
//     the current status, the textbox "タイトル" holds the current title, and the status "ブロック"
//     (the band) lists the blockers that are not done yet and is gone when there are none
import { expect, test } from "./harness";
import { properties, row } from "./screens";

const MARK = "CLI で更新 · 今";

test("a task added by the CLI appears with the mark, which goes away", async ({ page, hh }) => {
  hh.add("Already here");
  await hh.openTree(page);
  await expect(row(page, "Already here")).toBeVisible();

  hh.add("From the terminal");
  const added = row(page, "From the terminal");
  await expect(added).toBeVisible();
  await expect(added).toContainText(MARK);

  await expect(added).not.toContainText(MARK, { timeout: 15000 });
});

test("a title changed by the CLI is shown in place, with the mark", async ({ page, hh }) => {
  const t = hh.add("Old title");
  await hh.openTree(page);
  await expect(row(page, "Old title")).toBeVisible();

  hh.hh("set", t.id, "title=New title");
  await expect(row(page, "New title")).toContainText(MARK);
  await expect(row(page, "Old title")).toHaveCount(0);
});

test("a task deleted by the CLI leaves the tree", async ({ page, hh }) => {
  const t = hh.add("Doomed");
  hh.add("Stays");
  await hh.openTree(page);
  await expect(row(page, "Doomed")).toBeVisible();

  hh.hh("rm", t.id);
  await expect(row(page, "Doomed")).toHaveCount(0);
  await expect(row(page, "Stays")).toBeVisible();
});

test("a task added in the page does not get the mark", async ({ page, hh }) => {
  const other = hh.add("Someone else's");
  await hh.openTree(page);

  await row(page, "Someone else's").click();
  await page.keyboard.press("Enter");
  await page.getByRole("textbox", { name: "新しいタスクのタイトル" }).fill("Made here");
  await page.keyboard.press("Enter");
  await expect(row(page, "Made here")).toBeVisible();

  // The events arrive in order: once the CLI's change has shown its mark, the
  // page's own change has been handled too.
  hh.hh("set", other.id, "title=Someone else's, changed");
  await expect(row(page, "Someone else's, changed")).toContainText(MARK);
  await expect(row(page, "Made here")).not.toContainText(MARK);
});

test("the sidebar lists the workspace and the sync card is there", async ({ page, hh }) => {
  await hh.open(page);
  const nav = page.getByRole("navigation", { name: "メインナビゲーション" });
  await expect(nav.getByRole("link", { name: /demo/ })).toBeVisible();
  await expect(nav.getByRole("button", { name: /今すぐ同期/ })).toBeVisible();
});

test("the detail follows a status changed by the CLI", async ({ page, hh }) => {
  const t = hh.add("Watched");
  await hh.open(page, `/tasks/${t.ulid}`);
  const status = properties(page).getByRole("button", { name: "ステータス" });
  await expect(status).toContainText("Todo");

  hh.hh("set", t.id, "status=in_progress");
  await expect(status).toContainText("In progress");
});

test("the detail follows a title changed by the CLI", async ({ page, hh }) => {
  const t = hh.add("Before the change");
  await hh.open(page, `/tasks/${t.ulid}`);
  const title = page.getByRole("textbox", { name: "タイトル" });
  await expect(title).toHaveValue("Before the change");

  hh.hh("set", t.id, "title=After the change");
  await expect(title).toHaveValue("After the change");
});

test("the band of the blockers goes when the CLI finishes the blocker", async ({ page, hh }) => {
  const blocked = hh.add("Waits for it");
  const blocker = hh.add("The blocker");
  hh.hh("dep", "add", blocked.id, "--blocked-by", blocker.id);
  await hh.open(page, `/tasks/${blocked.ulid}`);
  const band = page.getByRole("status", { name: "ブロック" });
  await expect(band).toContainText("The blocker");

  hh.hh("done", blocker.id);
  await expect(band).toHaveCount(0);
});
