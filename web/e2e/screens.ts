import { expect, type Locator, type Page } from "@playwright/test";

// The words and roles the specs find the screens by. They are the ones the screens of
// Tasks 7 to 15 have (the tests of those tasks check the same words); when a screen
// changes a word, change it here and in the spec together.
//
//   Tree        the list is role "tree" named "<workspace or project> のタスク"; a row is a
//               flat element with role "treeitem" (data-ulid, aria-level, aria-expanded,
//               aria-selected) that shows its ID and title; a row changed by the CLI shows
//               "CLI で更新 · 今"; the whole row is the drag handle; the status icon of a
//               row is an svg with data-shape ("check" is done, "ring" is todo)
//   Add row     Enter on a selected row opens the textbox "新しいタスクのタイトル" below it
//   Dialogs     role "dialog", named by the title; the footer has the buttons
//   Toasts      one role "status" holds all the toasts: each has its message, and a link
//               ("開く") or the button "取り消し"; other regions with role "status" exist
//               (the save state, the bands of a task), so find a toast by its text
//   Detail      the title is the textbox "タイトル"; the body is the textbox "本文"; the
//               save state is the status "保存の状態" ("保存中…", "保存済み",
//               "保存済み · push 待ち"); the property panel is the complementary region
//               "プロパティ"; status, kind and the enum properties are buttons that open a
//               menu of menuitems, named by the label ("ステータス", "priority"); a date
//               property is an input labelled by its name ("due")
//   Overview    /workspaces/<store>/<ulid> shows "<n> 件 · プロジェクト <m>", the legend "未着手 <n>"
//               ... "中止 <n>" (exact text), and the regions "プロジェクト" and "プロジェクトの外の
//               タスク" (name them exactly: the second name starts with the first), whose rows are
//               named by the ID and the title; the buttons "タスクを作成" and "プロジェクトを作成"
//   Undo        the palette command "直前の操作を取り消す"; the toast says "<ID> の変更を取り消しました"

//
// Exceptions that hold on to a CSS class, because no role or text tells them apart: change them
// together with the screen. `.scroller` is the scrolling box of the tree (tree.spec.ts scrolls it
// itself while a drag is held, where the wheel does not work); `.row.dragging` is the row being
// dragged; `aside.refusal` is the panel under the tree that says why a drop is refused (the toast
// after the drop has the same words, and so does the chip, so the text cannot find the panel).

/** The row of the tree whose text has the title. Titles in the specs are unique. */
export function row(page: Page, title: string): Locator {
  return page.getByRole("treeitem").filter({ hasText: title });
}

/**
 * Waits until the tree has read the schema of the store: the add row then names the kind it
 * makes. A drop before that is refused ("少し待ってからやり直してください"); the keys that depend
 * on the schema (Space, Tab, S, K) wait for it, so they need no such wait.
 */
export async function treeReady(page: Page): Promise<void> {
  await expect(page.getByRole("button", { name: /種類: \S/ })).toBeVisible();
}

export function createDialog(page: Page): Locator {
  return page.getByRole("dialog", { name: "タスクを作成" });
}

/** Waits until the create dialog can create (it has read the schema of the store). */
export async function createReady(dialog: Locator): Promise<void> {
  await expect(dialog.getByRole("button", { name: /^作成/ })).toBeEnabled();
}

/** The toasts that have the text. */
export function toast(page: Page, text: string | RegExp): Locator {
  return page.getByRole("status").filter({ hasText: text });
}

export function properties(page: Page): Locator {
  return page.getByRole("complementary", { name: "プロパティ" });
}

/** The save state of the detail, once a change is on disk. */
export function saved(page: Page): Locator {
  return page.getByRole("status", { name: "保存の状態" }).filter({ hasText: /^保存済み/ });
}

/**
 * Drags the row `from` onto the row `to`: onto its upper quarter ("before"),
 * its lower quarter ("after") or its middle ("child"). `hold` runs while the
 * pointer is over the target and the button is still down.
 */
export async function dragRow(
  page: Page,
  from: Locator,
  to: Locator,
  where: "before" | "after" | "child",
  hold?: () => Promise<void>,
): Promise<void> {
  const a = await from.boundingBox();
  const b = await to.boundingBox();
  if (!a || !b) throw new Error("both rows must be visible to drag");
  const at = { before: 0.2, child: 0.5, after: 0.8 }[where];
  const x = b.x + b.width / 2;
  const y = b.y + b.height * at;
  await page.mouse.move(a.x + a.width / 2, a.y + a.height / 2);
  await page.mouse.down();
  // A first small move starts the drag; the rest passes over the target.
  await page.mouse.move(a.x + a.width / 2, a.y + a.height / 2 + 8, { steps: 2 });
  await page.mouse.move(x, y, { steps: 12 });
  await hold?.();
  await page.mouse.up();
}
