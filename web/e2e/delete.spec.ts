// Screens this spec relies on (see screens.ts for the shared list):
//   - the detail header has the button "削除"; it opens the dialog named
//     "<ID> <title> を削除", which says "配下の <n> 件も一緒に削除します", lists the
//     tasks that go with it and has the button "<n + 1> 件を削除" (just "削除" when
//     nothing is under the task); after the delete the toast says "「<title>」を削除しました"
//     and has the button "取り消し"
//   - the detail of a deleted task shows the status "削除済み" with the button "復元";
//     "復元" shows the toast "<ID> を復元しました"
import { expect, test } from "./harness";
import { toast } from "./screens";

test("deleting a task takes its children too, and Restore brings them back", async ({
  page,
  hh,
}) => {
  const parent = hh.add("Release prep", "--kind", "project");
  const child = hh.add("Rollback notes", "--parent", parent.id);
  await hh.open(page, `/tasks/${parent.ulid}`);

  await page.getByRole("button", { name: "削除", exact: true }).click();
  const dialog = page.getByRole("dialog", { name: /を削除$/ });
  await expect(dialog).toContainText("配下の 1 件も一緒に削除します");
  await expect(dialog).toContainText(`${child.id} Rollback notes`);
  await dialog.getByRole("button", { name: "2 件を削除" }).click();

  await expect(toast(page, "「Release prep」を削除しました")).toBeVisible();
  const band = page.getByRole("status", { name: "削除済み" });
  await expect(band).toBeVisible();
  expect(hh.show(parent.id).deleted_at).not.toBeNull();
  expect(hh.show(child.id).deleted_at).not.toBeNull();

  await page.getByRole("button", { name: "復元" }).click();
  await expect(toast(page, `${parent.id} を復元しました`)).toBeVisible();
  await expect(band).toBeHidden();
  expect(hh.show(parent.id).deleted_at).toBeNull();
  expect(hh.show(child.id).deleted_at).toBeNull();
});

test("Undo in the toast restores the task", async ({ page, hh }) => {
  const t = hh.add("Deleted by mistake");
  await hh.open(page, `/tasks/${t.ulid}`);

  await page.getByRole("button", { name: "削除", exact: true }).click();
  await page
    .getByRole("dialog", { name: /を削除$/ })
    .getByRole("button", { name: "削除", exact: true })
    .click();
  const deleted = toast(page, "「Deleted by mistake」を削除しました");
  await expect(deleted).toBeVisible();
  expect(hh.show(t.id).deleted_at).not.toBeNull();

  await deleted.getByRole("button", { name: "取り消し" }).click();
  await expect.poll(() => hh.show(t.id).deleted_at).toBeNull();
});
