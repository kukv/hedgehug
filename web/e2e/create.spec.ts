// Screens this spec relies on (see screens.ts for the shared list):
//   - C opens the dialog named "タスクを作成" (CreateTask) when no input has focus
//   - the dialog has the textbox "タイトル", the textbox "本文" (CodeMirror), the radio
//     group "種類" (a radio is named by the kind's label: タスク, バグ, エピック …; a kind
//     the parent does not accept is disabled), the parent as "<ID> <title>" with the
//     button "親を外す" (or the hint "親: なし" when there is none), and the buttons
//     "作成 ⌘↵" / "作成 Ctrl+↵", "続けて作成", "キャンセル"
//   - after a create the toast says "<ID> を作成しました · <workspace>" and has the link
//     "開く", which opens /tasks/<ulid>
//   - a click on a tree row selects it; the dialog's parent is the selected row
import { expect, test } from "./harness";
import { createDialog, createReady, row, toast } from "./screens";

test("the dialog creates a task in one commit and the toast opens its detail", async ({
  page,
  hh,
}) => {
  await hh.openTree(page);
  await page.keyboard.press("c");
  const dialog = createDialog(page);
  await expect(dialog).toBeVisible();
  await expect(dialog.getByText("親: なし")).toBeVisible();

  await dialog.getByRole("textbox", { name: "タイトル" }).fill("Write the spec");
  await dialog.getByRole("radio", { name: "バグ" }).check({ force: true }); // the input is hidden behind its label
  await dialog.getByRole("textbox", { name: "本文" }).click();
  await page.keyboard.type("first line\n\nsecond line");
  await dialog.getByRole("button", { name: /^作成/ }).click();

  const made = toast(page, "DM-1 を作成しました");
  await expect(made).toBeVisible();
  await expect(dialog).toBeHidden();
  await expect(row(page, "Write the spec")).toBeVisible();

  const t = hh.show("DM-1");
  expect(t.title).toBe("Write the spec");
  expect(t.kind).toBe("bug");
  expect(t.body?.trim()).toBe("first line\n\nsecond line");
  expect(t.history).toHaveLength(1); // the title, the kind and the body are one commit

  await made.getByRole("link", { name: "開く" }).click();
  await expect(page).toHaveURL(new RegExp(`/tasks/${t.ulid}$`));
});

test("Ctrl+Enter creates", async ({ page, hh }) => {
  await hh.openTree(page);
  await page.keyboard.press("c");
  const dialog = createDialog(page);
  await dialog.getByRole("textbox", { name: "タイトル" }).fill("From the keyboard");
  await createReady(dialog);
  await page.keyboard.press("ControlOrMeta+Enter");
  await expect(toast(page, "DM-1 を作成しました")).toBeVisible();
  await expect(row(page, "From the keyboard")).toBeVisible();
  expect(hh.show("DM-1").title).toBe("From the keyboard");
});

test("C on a task detail makes a child of that task", async ({ page, hh }) => {
  const parent = hh.add("Parent task");
  await hh.open(page, `/tasks/${parent.ulid}`);
  await page.keyboard.press("c");
  const dialog = createDialog(page);
  await expect(dialog).toBeVisible();
  await expect(dialog.getByText(parent.id, { exact: true })).toBeVisible();
  await dialog.getByRole("textbox", { name: "タイトル" }).fill("Child of the detail");
  await createReady(dialog);
  await dialog.getByRole("button", { name: /^作成/ }).click();
  await expect(toast(page, "DM-2 を作成しました")).toBeVisible();
  expect(hh.show("DM-2").parent).toBe(parent.ulid);
});

test("Create more keeps the dialog and empties the title and the body", async ({ page, hh }) => {
  await hh.open(page);
  await page.keyboard.press("c");
  const dialog = createDialog(page);
  const title = dialog.getByRole("textbox", { name: "タイトル" });
  await title.fill("First");
  await dialog.getByRole("button", { name: "続けて作成" }).click();
  await expect(toast(page, "DM-1 を作成しました")).toBeVisible();
  await expect(dialog).toBeVisible();
  await expect(title).toHaveValue("");

  await title.fill("Second");
  await dialog.getByRole("button", { name: /^作成/ }).click();
  await expect(toast(page, "DM-2 を作成しました")).toBeVisible();
  expect(hh.show("DM-1").title).toBe("First");
  expect(hh.show("DM-2").title).toBe("Second");
});

test("the selected row is the parent, and the kinds it cannot hold are disabled", async ({
  page,
  hh,
}) => {
  const project = hh.add("Release", "--kind", "project");
  const task = hh.add("A plain task", "--parent", project.id);
  await hh.openTree(page);

  await row(page, "Release").click();
  await page.keyboard.press("c");
  const dialog = createDialog(page);
  await expect(dialog.getByText(project.id, { exact: true })).toBeVisible();
  await expect(dialog.getByRole("button", { name: "親を外す" })).toBeVisible();
  await expect(dialog.getByRole("radio", { name: "エピック" })).toBeEnabled();
  await dialog.getByRole("textbox", { name: "タイトル" }).fill("Under the project");
  await dialog.getByRole("button", { name: /^作成/ }).click();
  await expect(toast(page, "DM-3 を作成しました")).toBeVisible();
  expect(hh.show("DM-3").parent).toBe(project.ulid);

  await row(page, "A plain task").click();
  await page.keyboard.press("c");
  await expect(dialog.getByText(task.id, { exact: true })).toBeVisible();
  await expect(dialog.getByRole("radio", { name: "エピック" })).toBeDisabled();
  await expect(dialog.getByRole("radio", { name: "タスク" })).toBeEnabled();
});

test("Escape closes the dialog without creating", async ({ page, hh }) => {
  await hh.open(page);
  await page.keyboard.press("c");
  const dialog = createDialog(page);
  await dialog.getByRole("textbox", { name: "タイトル" }).fill("Never made");
  await page.keyboard.press("Escape");
  await expect(dialog).toBeHidden();
  expect(hh.json<unknown[]>("ls")).toHaveLength(0);
});
