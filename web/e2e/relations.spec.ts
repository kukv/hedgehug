// Screens this spec relies on (see screens.ts for the shared list):
//   - the detail at /tasks/<ulid> has the section "関連" with one group per relation that has a value
//     (a fieldset named by the display name: "Causes", "Clones", "Caused by", "Cloned by"), which lists the IDs of the tasks
//   - the button "関連を追加" opens the button "関連の種類" (a menu of the five display names) and the combobox "タスクを検索",
//     whose option (the ID and the title) is picked to add the relation
// The relations are fixed (docs/superpowers/specs/2026-11-08-fixed-kinds-relations-design.md): a task
// shows a relation under its own name, and the task it points at shows it under the name of the other way.
import { expect, test } from "./harness";

test("causes and clones show under the other task as Caused by and Cloned by", async ({
  page,
  hh,
}) => {
  const bug = hh.add("The crash");
  const effect = hh.add("The data loss");
  const original = hh.add("The first release");

  await hh.open(page, `/tasks/${bug.ulid}`);
  for (const [relation, target] of [
    ["Causes", effect],
    ["Clones", original],
  ] as const) {
    await page.getByRole("button", { name: "関連を追加" }).click();
    await page.getByRole("button", { name: "関連の種類" }).click();
    await page.getByRole("menuitem", { name: relation, exact: true }).click();
    await page.getByRole("combobox", { name: "タスクを検索" }).fill(target.title);
    await page.getByRole("option", { name: target.title }).click();
    await expect(page.getByRole("group", { name: relation })).toContainText(target.id);
  }
  await expect(page.getByRole("group", { name: "Causes" })).toContainText(effect.id);
  await expect(page.getByRole("group", { name: "Clones" })).toContainText(original.id);

  await hh.open(page, `/tasks/${effect.ulid}`);
  await expect(page.getByRole("group", { name: "Caused by" })).toContainText(bug.id);

  await hh.open(page, `/tasks/${original.ulid}`);
  await expect(page.getByRole("group", { name: "Cloned by" })).toContainText(bug.id);
});

test("the query caused_by: finds the effect from the cause", async ({ hh }) => {
  const bug = hh.add("The crash");
  const effect = hh.add("The data loss");
  hh.add("Unrelated");
  hh.hh("dep", "add", bug.id, "--causes", effect.id);

  const out = hh.hh("ls", `caused_by:${bug.id}`);
  expect(out).toContain(effect.id);
  expect(out).not.toContain("Unrelated");
});
