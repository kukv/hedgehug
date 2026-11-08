// Screens this spec relies on (see screens.ts for the shared list):
//   - /settings/general has the region "診断と修復" with, for each Store, the region "診断: <Store>":
//     "問題は見つかりませんでした" when it is clean; else the findings (listitem) with the words of the
//     server and, for a task's, the link "<ID> を開く"; the buttons "もう一度実行" and "修復…"
//     (the dialog "doctor で修復する" has "修復する"; the toast says "<n> 件を修復しました")
//   - the same page has the region "削除済みタスクの完全削除": the textbox "N 日より古いもの", the button
//     "確認する", the list "完全に削除されるタスク", the button "完全に削除…" (the dialog "完全に削除する"
//     has "完全に削除する")
import { execFileSync } from "node:child_process";
import { readdirSync, readFileSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import type { Page } from "@playwright/test";
import { expect, type Hedgehug, test } from "./harness";

const doctor = (page: Page) =>
  page.getByRole("region", { name: "診断と修復" }).getByRole("region", { name: "診断: demo" });
const purge = (page: Page) => page.getByRole("region", { name: "削除済みタスクの完全削除" });

/** Marks the task deleted by hand in the work tree of the store and commits it, as an editor would. */
function deleteByHand(hh: Hedgehug, id: string) {
  const dir = join(hh.home, "hh", "stores", "demo");
  const tasks = join(dir, "tasks");
  const rel = readdirSync(tasks, { recursive: true, encoding: "utf8" })
    .map((f) => join("tasks", f))
    .find((f) => {
      try {
        return readFileSync(join(dir, f), "utf8").includes(`\nid: ${id}\n`);
      } catch {
        return false; // a directory
      }
    });
  if (!rel) throw new Error(`no file of ${id}`);
  const text = readFileSync(join(dir, rel), "utf8");
  const edited = text.replace("\nkind: ", "\ndeleted_at: 2026-10-30T12:00:00Z\nkind: ");
  if (edited === text) throw new Error(`${rel} has no kind line to put deleted_at before`);
  writeFileSync(join(dir, rel), edited);
  const git = (...args: string[]) =>
    execFileSync("git", [
      "-C",
      dir,
      "-c",
      "user.name=Alice",
      "-c",
      "user.email=a@example.com",
      "-c",
      "commit.gpgsign=false",
      ...args,
    ]);
  git("add", rel);
  git("commit", "--quiet", "-m", "hand edit");
}

test("a store with no problem says so", async ({ page, hh }) => {
  hh.add("Alpha");
  await hh.open(page, "/settings/general");
  await expect(doctor(page).getByText("問題は見つかりませんでした")).toBeVisible();
  await expect(doctor(page).getByRole("button", { name: "修復…" })).toHaveCount(0);
});

test("a problem of a task is listed with a link to the task, and doctor --fix repairs it", async ({
  page,
  hh,
}) => {
  const parent = hh.add("Parent");
  const child = hh.add("Child", "--parent", parent.id);
  deleteByHand(hh, parent.id);
  await hh.open(page, "/settings/general");

  const finding = doctor(page).getByRole("listitem").filter({ hasText: child.id });
  await expect(finding).toBeVisible();
  await expect(finding).toContainText("エラー");
  await finding.getByRole("link", { name: `${child.id} を開く` }).click();
  await expect(page).toHaveURL(new RegExp(`/tasks/${child.ulid}$`));
  await expect(page.getByRole("textbox", { name: "タイトル" })).toHaveValue("Child");

  await hh.open(page, "/settings/general");
  await doctor(page).getByRole("button", { name: "修復…" }).click();
  const dialog = page.getByRole("dialog", { name: "doctor で修復する" });
  await expect(
    dialog.getByRole("button", { name: "コマンドをコピー: hh doctor --fix" }),
  ).toBeVisible();
  await dialog.getByRole("button", { name: "修復する" }).click();
  await expect(page.getByRole("status").filter({ hasText: "1 件を修復しました" })).toBeVisible();
  const result = doctor(page).getByRole("region", { name: "修復の結果" });
  await expect(result).toContainText(child.id);
  await expect(result).toContainText(/親「.+」が削除されたため、「.+」の下に移しました/);
  await expect(doctor(page).getByText("問題は見つかりませんでした")).toBeVisible();
  // The root holds only projects: the task goes under the 未分類 project that doctor made.
  const projects = hh.json<{ tasks: { ulid: string; title: string }[] }>(
    "ls",
    "kind:project",
  ).tasks;
  const uncategorized = projects.find((t) => t.title === "未分類");
  expect(uncategorized).toBeDefined();
  expect(hh.show(child.id).parent).toBe(uncategorized?.ulid);
});

test("a deleted task is purged by age: checked, listed, then gone", async ({ page, hh }) => {
  const gone = hh.add("Delete me");
  const kept = hh.add("Keep me");
  hh.hh("rm", gone.id);
  await hh.open(page, "/settings/general");

  const days = purge(page).getByRole("textbox", { name: "N 日より古いもの" });
  // a negative number cannot be checked
  await days.fill("-1");
  await expect(purge(page).getByRole("button", { name: "確認する" })).toBeDisabled();
  await days.fill("0");
  await purge(page).getByRole("button", { name: "確認する" }).click();
  const list = purge(page).getByRole("list", { name: "完全に削除されるタスク" });
  await expect(list).toContainText(gone.id);
  await expect(list).toContainText("Delete me");
  await expect(list).not.toContainText(kept.id);

  await purge(page).getByRole("button", { name: "完全に削除…" }).click();
  const dialog = page.getByRole("dialog", { name: "完全に削除する" });
  await expect(dialog).toContainText("取り消せません");
  await expect(
    dialog.getByRole("button", { name: "コマンドをコピー: hh purge --older-than 0d" }),
  ).toBeVisible();
  await dialog.getByRole("button", { name: "完全に削除する" }).click();
  await expect(
    page.getByRole("status").filter({ hasText: /1 件のタスクを完全に削除しました（[0-9a-f]{7}）/ }),
  ).toBeVisible();
  await expect(purge(page).getByRole("list", { name: "完全に削除されるタスク" })).toHaveCount(0);

  expect(() => hh.show(gone.id)).toThrow();
  expect(hh.show(kept.id).title).toBe("Keep me");
});
