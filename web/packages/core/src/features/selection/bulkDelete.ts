import { Refused } from "../../model/errors";
import type { BulkDeleted, BulkDeletePreview, ItemRef } from "../../model/itemAction";
import type { ItemActionsPort } from "../../port/itemActions";
import { get, type Readable, readonly, writable } from "../../reactive/store";
import { type Drafts, isItemDraft } from "../drafts/drafts";
import { itemDeleteFailedMessage } from "../items/delete";
import type { Detail } from "../items/detail";
import type { Toasts } from "../toasts/toasts";
import type { RowSelection, SelectedRow } from "./rowSelection";

// 一覧で選んだ行の一括削除(F2b1)。確認ダイアログを出し、実行したら、選択の解除・パネルを閉じる・読み直しまでを行う。

// 確認ダイアログに並べる ID の数。これを超えたぶんは「ほか N 件」にする。
const listed = 10;

export interface BulkDeleteDialog {
  title: string;
  // lines は、本文(1 段落)と、閲覧のみの行があるときの補足(1 段落)。
  lines: string[];
  ok: string;
}

// bulkDeleteDialog は、確認ダイアログの中身。ID と件数は、閲覧のみでない行(実際に削除するもの)だけ数える。
export function bulkDeleteDialog(rows: SelectedRow[], p: BulkDeletePreview): BulkDeleteDialog {
  const targets = rows.filter((r) => !r.readOnly);
  const ids = targets.slice(0, listed).map((r) => r.display);
  const more = targets.length - ids.length;
  const named = more > 0 ? `${ids.join("、")} ほか ${more} 件 ` : `${ids.join("、")} `;
  const retention = p.stores >= 2 ? "保持日数" : `${p.retentionDays} 日`;
  const lines = [
    `${named}と、その下にあるものを削除します。設定の「削除済み」から戻せます。${retention}を過ぎると完全に削除されます。`,
  ];
  if (p.readOnly > 0) lines.push(`閲覧のみの Store の ${p.readOnly} 件は削除しません。`);
  return { title: "一括削除", lines, ok: `${targets.length} 件を削除` };
}

// bulkDeletedText は、削除したあとのトーストの文。
export function bulkDeletedText(r: BulkDeleted): string {
  const done = `${r.deleted} 件を削除しました`;
  return r.readOnly > 0 ? `${done}。${r.readOnly} 件は閲覧のみの Store のためそのままです` : done;
}

export interface BulkDeleteDeps {
  actions: Pick<ItemActionsPort, "bulkDeletePreview" | "bulkDelete">;
  toasts: Toasts;
  detail: Pick<Detail, "view" | "close" | "reload">;
  drafts: Pick<Drafts, "confirmLeave">;
}

export interface BulkDelete {
  // state は、出している確認ダイアログ(出していなければ null)。
  state: Readable<BulkDeleteDialog | null>;
  // open は、開いているパネルの書きかけを聞き、予告を読んでから確認ダイアログを出す。出しているあいだ、対象は開いた時点の選択に固定する。
  // after は、削除を試みたあと(成功も失敗も)に、画面を読み直すために呼ぶ。
  open(selection: RowSelection, after: () => Promise<void>): Promise<void>;
  confirm(): Promise<void>;
  cancel(): void;
}

interface Opened {
  selection: RowSelection;
  rows: SelectedRow[];
  after: () => Promise<void>;
}

const ref = (r: SelectedRow): ItemRef => ({ storeId: r.storeId, itemId: r.itemId });

export function createBulkDelete(deps: BulkDeleteDeps): BulkDelete {
  const state = writable<BulkDeleteDialog | null>(null);
  // opened は、ダイアログを出しているあいだの、開いた時点の選択。
  let opened: Opened | null = null;
  // seq は、開く・閉じるの世代。予告を待っているあいだに閉じた・開き直したら、古い結果を捨てる。
  let seq = 0;
  // busy は、削除の実行中。そのあいだの confirm は受けない。
  let busy = false;

  const failed = (e: unknown) =>
    deps.toasts.show(e instanceof Refused ? e.message : itemDeleteFailedMessage, { error: true });

  // inPanel は、開いているパネルのアイテムか祖先が、削除する行のものか(Store も比べる)。
  const inPanel = (rows: SelectedRow[], withAncestors: boolean) => {
    const panel = get(deps.detail.view);
    if (panel === null) return false;
    const ids = withAncestors
      ? [panel.item.id, ...panel.ancestors.map((a) => a.id)]
      : [panel.item.id];
    return rows.some(
      (r) => !r.readOnly && r.storeId === panel.item.storeId && ids.includes(r.itemId),
    );
  };

  return {
    state: readonly(state),
    async open(selection, after) {
      const rows = get(selection.rows);
      if (rows.length === 0) return;
      const gen = ++seq;
      if (inPanel(rows, true) && !(await deps.drafts.confirmLeave(isItemDraft))) return;
      if (gen !== seq) return;
      let preview: BulkDeletePreview;
      try {
        preview = await deps.actions.bulkDeletePreview(rows.map(ref));
      } catch (e) {
        if (gen === seq) failed(e);
        return;
      }
      if (gen !== seq) return;
      opened = { selection, rows, after };
      state.set(bulkDeleteDialog(rows, preview));
    },
    async confirm() {
      const o = opened;
      if (!o || get(state) === null || busy) return;
      busy = true;
      let result: BulkDeleted;
      try {
        result = await deps.actions.bulkDelete(o.rows.map(ref));
      } catch (e) {
        state.set(null);
        opened = null;
        failed(e);
        await o.after();
        return;
      } finally {
        busy = false;
      }
      state.set(null);
      opened = null;
      o.selection.clear();
      if (result.deleted > 0) deps.toasts.show(bulkDeletedText(result));
      if (inPanel(o.rows, false)) deps.detail.close();
      await deps.detail.reload();
      await o.after();
    },
    cancel() {
      seq += 1;
      opened = null;
      state.set(null);
    },
  };
}
