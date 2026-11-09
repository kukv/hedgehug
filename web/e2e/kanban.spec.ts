// Screens this spec relies on (see screens.ts for the shared list):
//   - /workspaces/<store>/<ulid>/kanban and /projects/<ulid>/kanban show the board
//     (section "カンバンの盤面"): one list for each column, ul[data-column="<status>"], whose cards are
//     li[data-ulid="<ULID>"]; a card is dragged onto the list of another column to change its status
//   - a status that needs a field (the workflow of the project) asks for it first, in the dialog
//     "<ID> を<ステータス>にするには <項目> が必要です": the field is labelled by its name, the button
//     "<ステータス>にする" is disabled until the field is filled, and "やめる" closes it
import type { Page } from "@playwright/test";
import { expect, type Hedgehug, test } from "./harness";
import { dragCard } from "./screens";

// The path of a tab of the first workspace of the first store.
async function tab(hh: Hedgehug, name: string): Promise<string> {
  return (await hh.treePath()).replace(/\/tree$/, `/${name}`);
}

const cell = (page: Page, column: string) => page.locator(`ul[data-column="${column}"]`);
const card = (page: Page, ulid: string) => page.locator(`li[data-ulid="${ulid}"]`);
const commits = (hh: Hedgehug) => hh.hh("log").trim().split("\n").length;

test("dragging a card to another column changes its status", async ({ page, hh }) => {
  const alpha = hh.add("Alpha");
  hh.add("Bravo");
  await hh.open(page, await tab(hh, "kanban"));
  await expect(card(page, alpha.ulid)).toBeVisible();
  await expect(cell(page, "todo").locator(`li[data-ulid="${alpha.ulid}"]`)).toBeVisible();

  const before = commits(hh);
  await dragCard(page, card(page, alpha.ulid), cell(page, "in_progress"));
  await expect(cell(page, "in_progress").locator(`li[data-ulid="${alpha.ulid}"]`)).toBeVisible();
  await expect.poll(() => hh.show(alpha.id).status).toBe("in_progress");
  expect(commits(hh)).toBe(before + 1);
});

test("a status that needs a field asks for it, and sets both in one commit", async ({
  page,
  hh,
}) => {
  const project = hh.add("Release", "--kind", "project");
  const ship = hh.add("Ship it", "--parent", project.id);
  hh.hh("schema", "scope", "set", project.id, "--required", "in_progress=points");
  await hh.open(page, `/projects/${project.ulid}/kanban`);
  await expect(card(page, ship.ulid)).toBeVisible();

  const before = commits(hh);
  await dragCard(page, card(page, ship.ulid), cell(page, "in_progress"));
  const dialog = page.getByRole("dialog", {
    name: `${ship.id} をIn progressにするには ポイント が必要です`,
  });
  await expect(dialog).toBeVisible();
  // nothing is written until the field is given
  expect(hh.show(ship.id).status).toBe("todo");
  const go = dialog.getByRole("button", { name: "In progressにする" });
  await expect(go).toBeDisabled();

  await dialog.getByLabel("ポイント", { exact: true }).fill("5");
  await expect(go).toBeEnabled();
  await go.click();
  await expect(dialog).toBeHidden();
  await expect(cell(page, "in_progress").locator(`li[data-ulid="${ship.ulid}"]`)).toBeVisible();

  const after = hh.show(ship.id);
  expect(after.status).toBe("in_progress");
  expect(after.points).toBe(5);
  expect(commits(hh)).toBe(before + 1);
});

test("closing the dialog of a missing field leaves the task where it was", async ({ page, hh }) => {
  const project = hh.add("Release", "--kind", "project");
  const ship = hh.add("Ship it", "--parent", project.id);
  hh.hh("schema", "scope", "set", project.id, "--required", "in_progress=points");
  await hh.open(page, `/projects/${project.ulid}/kanban`);

  const before = commits(hh);
  await dragCard(page, card(page, ship.ulid), cell(page, "in_progress"));
  const dialog = page.getByRole("dialog", {
    name: `${ship.id} をIn progressにするには ポイント が必要です`,
  });
  await dialog.getByRole("button", { name: "キャンセル" }).click();
  await expect(dialog).toBeHidden();
  await expect(cell(page, "todo").locator(`li[data-ulid="${ship.ulid}"]`)).toBeVisible();
  expect(hh.show(ship.id).status).toBe("todo");
  expect(commits(hh)).toBe(before);
});
