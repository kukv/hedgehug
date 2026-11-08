// Screens this spec relies on (see screens.ts for the shared list):
//   - the palette (Ctrl+K) lists the command "直前の操作を取り消す" when the input starts with >;
//     Enter runs it without asking, and the toast says "<ID> の変更を取り消しました" (the
//     operation is named as the History tab of the detail names it: 変更, 作成 ...; the ID is
//     left out for a commit that is not about a task)
//   - the settings tab of a workspace (/workspaces/<store>/<ulid>/settings) has the button "削除", whose
//     dialog "ワークスペース <name> を削除" has the button "削除"; the toast "ワークスペース <name> を
//     削除しました" has the button "取り消し", which takes the removal back
import type { Page } from "@playwright/test";
import { expect, test } from "./harness";
import { properties, toast } from "./screens";

async function runUndoFromPalette(page: Page): Promise<void> {
  await page.keyboard.press("ControlOrMeta+k");
  const palette = page.getByRole("dialog", { name: "ID・検索・コマンド" });
  await palette.getByRole("combobox").fill(">取り消");
  await expect(palette.getByRole("option", { name: /直前の操作を取り消す/ })).toBeVisible();
  await page.keyboard.press("Enter");
  await expect(palette).toBeHidden();
}

test("the palette takes back my last operation without asking", async ({ page, hh }) => {
  const t = hh.add("Undo me");
  hh.hh("set", t.id, "status=done");
  expect(hh.show(t.id).status).toBe("done");
  await hh.open(page, `/tasks/${t.ulid}`);
  const status = properties(page).getByRole("button", { name: "ステータス" });
  await expect(status).toContainText("Done");

  await runUndoFromPalette(page);

  await expect(toast(page, `${t.id} の変更を取り消しました`)).toBeVisible();
  // no dialog asked first
  await expect(page.getByRole("dialog")).toHaveCount(0);
  await expect.poll(() => hh.show(t.id).status).toBe("todo");
  // the detail follows, and the page does not show its own undo as a change from outside
  await expect(status).toContainText("Todo");
  await expect(page.getByText("CLI で更新 · 今")).toHaveCount(0);
});

test("a second undo takes back the operation before", async ({ page, hh }) => {
  const t = hh.add("Twice");
  hh.hh("set", t.id, "status=in_progress");
  hh.hh("set", t.id, "status=done");
  await hh.open(page, `/tasks/${t.ulid}`);

  await runUndoFromPalette(page);
  await expect.poll(() => hh.show(t.id).status).toBe("in_progress");
  await runUndoFromPalette(page);
  await expect.poll(() => hh.show(t.id).status).toBe("todo");
});

test("the toast of a workspace removal takes it back", async ({ page, hh }) => {
  hh.hh("workspace", "add", "sandbox", "--prefix", "SBX");
  const list = () => hh.json<{ ulid: string; name: string }[]>("workspace", "ls");
  const sandbox = list().find((w) => w.name === "sandbox");
  if (!sandbox) throw new Error("no workspace sandbox");

  await hh.open(page, `/workspaces/demo/${sandbox.ulid}/settings`);
  await page.getByRole("button", { name: "削除", exact: true }).click();
  const dialog = page.getByRole("dialog", { name: "ワークスペース sandbox を削除" });
  await dialog.getByRole("button", { name: "削除", exact: true }).click();

  const removed = toast(page, "ワークスペース sandbox を削除しました");
  await expect(removed).toBeVisible();
  expect(list().map((w) => w.name)).not.toContain("sandbox");
  await removed.getByRole("button", { name: "取り消し" }).click();

  await expect(toast(page, "ワークスペースの変更を取り消しました")).toBeVisible();
  await expect.poll(() => list().map((w) => w.name)).toContain("sandbox");
  // The screen follows the undo through the events, not through a reload of its own.
  await expect(page.getByRole("link", { name: /sandbox/ }).first()).toBeVisible();
});
