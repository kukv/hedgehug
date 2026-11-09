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
//               menu of menuitems, named by the label ("ステータス", "優先度"); a date
//               property is an input labelled by its label ("期限")
//   Overview    /workspaces/<store>/<ulid> shows "<n> 件 · プロジェクト <m>", the legend "未着手 <n>"
//               ... "中止 <n>" (exact text), and the region "プロジェクト", whose rows are
//               named by the ID and the title; the buttons "タスクを作成" and "プロジェクトを作成"
//   Undo        the palette command "直前の操作を取り消す"; the toast says "<ID> の変更を取り消しました"
//   Kanban      the board is the section "カンバンの盤面": a list for each column, ul[data-column=
//               "<status or category>"], whose cards are li[data-ulid="<ULID>"]; a card is dragged
//               onto a list (dragCard); a status that needs a field opens the dialog
//               "<ID> を<ステータス>にするには <項目> が必要です"
//   Table       a row is a set of inputs named by the column and the ID: the textbox "タイトル: <ID>",
//               the button "ステータス: <ID>"; the filter field is the textbox
//               "フィルタ（hh ls と同じクエリ）" on every list screen
//   Views       /views/<Store>/<名前> (heading <名前>, navigation "レイアウトの切り替え", the status
//               "未保存の変更" and the button "ビューを更新", the button "ビューの操作（名前の変更・削除）"),
//               /query/<layout>?q= (heading "クエリ"), the sidebar region "保存済みビュー", the
//               dialogs "ビューを保存" / "ビュー <名前> を改名" / "ビュー <名前> を削除"
//   Gantt       the tab "ガント" (/workspaces/<store>/<ulid>/gantt, /projects/<ULID>/gantt, /query/gantt): the list
//               "ガントのタスク"; a bar that moves is a button named "<ID> <題名> <開始> – <終了>。…", the others
//               are svg groups with a title, and every bar has data-ulid, data-kind (work, sprint, rollup)
//               and data-fill; the list "ガントの警告"; the group "表示する種類", the selects "開始", "終了", "目盛り"
//   Reports     the tab "レポート" (/workspaces/<store>/<ulid>/reports): the regions バーンダウン, ベロシティ, 累積
//               フロー図, サイクルタイム, 見積もりと実績; a chart is the img named by its report, and its details
//               "表で見る" holds the same numbers as a table; each region has the line of hh report
//   Project     /projects/<ULID>/settings is the tab "設定" at the right end of the navigation
//               "ビューの切り替え": the groups "表示するステータス" and "必須の項目を追加", the button "保存"
//   My tasks    /my (heading "マイタスク"); the groups are regions named "今日", "今週", ...; the sidebar
//               link "マイタスク" carries the count
//   Inbox       /inbox (heading "通知"); the navigation "通知の種類" (links すべて / 上書き / メンション /
//               同期の修復, each with the number unread; the tab is in the address: /inbox?tab=overwritten|mention|repair); a row is a listitem with the buttons
//               "自分の値に戻す", "このままにする" (overwritten) or "既読にする"; "すべて既読にする";
//               the sidebar link "通知" carries the number of unread
//   Activity    /activity (heading "アクティビティ"); the button "<n> / <m> ワークスペース" opens the dialog
//               "ワークスペースで絞り込む"; the group "変更の種類" has すべて / 人 / 同期; a commit is a listitem
//               with the author, the op, the subject and the ID as a link; the sidebar link "アクティビティ"
//   Maintenance /settings/general has the region "診断と修復" with, for each Store, the region "診断: <Store>"
//               ("問題は見つかりませんでした", or the findings as listitems with the link "<ID> を開く"; the
//               buttons "もう一度実行" and "修復…" open the dialog "doctor で修復する" with "修復する"; the
//               region "修復の結果"), and the region "削除済みタスクの完全削除" (the textbox "N 日より古い
//               もの", the buttons "確認する" and "完全に削除…", the dialog "完全に削除する")
//   Schema      /settings/schema/<store> (heading "スキーマ"); the navigation "スキーマの種類" has the links
//               ステータス / 優先度 / ラベル / プロパティ / ワークフロー / テンプレート（the relations are fixed and have no tab）, whose address is /settings/schema/<store>?tab=priorities|labels|props|workflow|templates; a row of a table is named by the definition
//               and its buttons are "編集（<名前>）" and "マージ（<名前>）"; the regions
//               "プロジェクトごとの上書き" and "スキーマの検査"; the dialog "ステータス「<名前>」を編集" has
//               the textbox "名前" and the button "保存"; the dialog "ステータス「<名前>」をマージ" has the
//               combobox "マージ先", the dry run "<n> 件のタスクが書き換わります" and the button "マージ"

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

/** Drags a card of the kanban onto the list of another column (the middle of the list). */
export async function dragCard(page: Page, card: Locator, cell: Locator): Promise<void> {
  await dragRow(page, card, cell, "child");
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
