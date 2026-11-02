// Screens this spec relies on (see screens.ts for the shared list):
//   - Ctrl+K (⌘K on a Mac) opens the command palette: the dialog "ID・検索・コマンド" with
//     the combobox, a listbox of options; typing an ID or a word finds the task as an option
//     "<ID> <title>"
//   - Enter opens the detail of the option in this tab; Ctrl+Enter opens it in a new tab,
//     which is a copy of this one (window.open without noopener), so the token goes with it
import { expect, test } from "./harness";

test("the palette finds a task by its ID and opens it", async ({ page, hh }) => {
  const t = hh.add("Find me");
  hh.add("Another one");
  await hh.open(page);

  await page.keyboard.press("ControlOrMeta+k");
  const palette = page.getByRole("dialog", { name: "ID・検索・コマンド" });
  await expect(palette).toBeVisible();
  await palette.getByRole("combobox").fill(t.id);
  await expect(palette.getByRole("option", { name: new RegExp(`${t.id}.*Find me`) })).toBeVisible();
  await page.keyboard.press("Enter");

  await expect(page).toHaveURL(new RegExp(`/tasks/${t.ulid}$`));
  await expect(page.getByRole("textbox", { name: "タイトル" })).toHaveValue("Find me");
});

test("Ctrl+Enter opens the task in a new tab that keeps the token", async ({ page, hh }) => {
  const t = hh.add("In a new tab");
  await hh.open(page);

  await page.keyboard.press("ControlOrMeta+k");
  const palette = page.getByRole("dialog", { name: "ID・検索・コマンド" });
  await palette.getByRole("combobox").fill(t.id);
  await expect(palette.getByRole("option", { name: new RegExp(t.id) })).toBeVisible();
  const [popup] = await Promise.all([
    page.waitForEvent("popup"),
    page.keyboard.press("ControlOrMeta+Enter"),
  ]);

  await expect(popup).toHaveURL(new RegExp(`/tasks/${t.ulid}$`));
  // The token came with the tab: the detail loads instead of the "open it from hh ui" screen.
  await expect(popup.getByRole("textbox", { name: "タイトル" })).toHaveValue("In a new tab");
});
