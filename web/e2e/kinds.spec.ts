// Screens this spec relies on (see screens.ts and create.spec.ts for the shared list):
//   - the kinds are fixed (docs/superpowers/specs/2026-11-08-fixed-kinds-relations-design.md): a project sits
//     at the root, an epic under a project, a story / task / bug under a project or an epic, and a step
//     under a story, a task or a bug
//   - the create dialog offers as radios ("種類") only the kinds the parent can hold, and its parent is
//     the selected row of the tree
import { expect, test } from "./harness";
import { createDialog, createReady, row, toast, treeReady } from "./screens";

test("a Project, an Epic, a Story and a Step are made in turn, and a Task does not go under a Story", async ({
  page,
  hh,
}) => {
  await hh.openTree(page);
  await treeReady(page);

  // At the root only a project can be made.
  await page.keyboard.press("c");
  const dialog = createDialog(page);
  await expect(dialog.getByRole("radio", { name: "Project" })).toBeChecked();
  await expect(dialog.getByRole("radio")).toHaveCount(1);
  await dialog.getByRole("textbox", { name: "タイトル" }).fill("Roadmap");
  await createReady(dialog);
  await dialog.getByRole("button", { name: /^作成/ }).click();
  await expect(toast(page, "DM-1 を作成しました")).toBeVisible();
  await expect(dialog).toBeHidden();

  // Under the project: an epic, a story, a task or a bug; a task is the default.
  await row(page, "Roadmap").click();
  await page.keyboard.press("c");
  await expect(dialog.getByRole("radio", { name: "Task" })).toBeChecked();
  await expect(dialog.getByRole("radio", { name: "Epic" })).toBeEnabled();
  await expect(dialog.getByRole("radio", { name: "Story" })).toBeEnabled();
  await expect(dialog.getByRole("radio", { name: "Bug" })).toBeEnabled();
  await expect(dialog.getByRole("radio", { name: "Project" })).toHaveCount(0);
  await expect(dialog.getByRole("radio", { name: "Step" })).toHaveCount(0);
  await dialog.getByRole("radio", { name: "Epic" }).check({ force: true }); // the input is hidden behind its label
  await dialog.getByRole("textbox", { name: "タイトル" }).fill("Q1");
  await dialog.getByRole("button", { name: /^作成/ }).click();
  await expect(toast(page, "DM-2 を作成しました")).toBeVisible();

  // Under the epic: a story (no project, no epic).
  await row(page, "Q1").click();
  await page.keyboard.press("c");
  await expect(dialog.getByRole("radio", { name: "Epic" })).toHaveCount(0);
  await expect(dialog.getByRole("radio", { name: "Project" })).toHaveCount(0);
  await dialog.getByRole("radio", { name: "Story" }).check({ force: true });
  await dialog.getByRole("textbox", { name: "タイトル" }).fill("Login");
  await dialog.getByRole("button", { name: /^作成/ }).click();
  await expect(toast(page, "DM-3 を作成しました")).toBeVisible();

  // Under the story: only a step, which is also the kind it starts with.
  await row(page, "Login").click();
  await page.keyboard.press("c");
  await expect(dialog.getByRole("radio", { name: "Step" })).toBeChecked();
  await expect(dialog.getByRole("radio", { name: "Task" })).toHaveCount(0);
  await expect(dialog.getByRole("radio", { name: "Story" })).toHaveCount(0);
  await expect(dialog.getByRole("radio", { name: "Bug" })).toHaveCount(0);
  await dialog.getByRole("textbox", { name: "タイトル" }).fill("Write the form");
  await dialog.getByRole("button", { name: /^作成/ }).click();
  await expect(toast(page, "DM-4 を作成しました")).toBeVisible();
  await expect(row(page, "Write the form")).toHaveAttribute("aria-level", "4");

  const [project, epic, story, step] = ["DM-1", "DM-2", "DM-3", "DM-4"].map((id) => hh.show(id));
  expect([project?.kind, epic?.kind, story?.kind, step?.kind]).toEqual([
    "project",
    "epic",
    "story",
    "step",
  ]);
  expect([epic?.parent, story?.parent, step?.parent]).toEqual([
    project?.ulid,
    epic?.ulid,
    story?.ulid,
  ]);

  // The CLI refuses what the dialog does not offer, and nothing is made.
  expect(() => hh.hh("add", "A task under a story", "--parent", "DM-3", "--kind", "task")).toThrow(
    /the change breaks the schema:\s+parent: a task cannot be placed under story/,
  );
  expect(hh.json<{ tasks: unknown[] }>("ls").tasks).toHaveLength(4);
});
