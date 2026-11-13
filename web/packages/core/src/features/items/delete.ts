import { NotFound, Refused } from "../../model/errors";
import type { Item, Kind } from "../../model/item";
import type { DeletePreview } from "../../model/itemAction";
import type { ItemActionsPort } from "../../port/itemActions";
import { get, type Readable, readonly, writable } from "../../reactive/store";
import type { Catalog } from "../catalog/catalog";
import { type Drafts, isItemDraft } from "../drafts/drafts";
import { itemDisplayId } from "../router/route";
import type { Toasts } from "../toasts/toasts";
import { kinds } from "./createDialog";
import type { Detail } from "./detail";
import type { ItemRefresh } from "./refresh";

// アイテムの削除の確認ダイアログ(02「削除」)。Step の一覧の「×」とアイテムのメニューの「削除…」が同じものを開く。
// 予告(下にあるものの件数と保持日数)をサーバーから読んでから出し、実行したら、トースト・パネルを閉じる・読み直しまでを行う。

// 仕様にない文言(通信の失敗など、断りでも「ない」でもない失敗のとき)。決まったら差し替える。
export const itemDeleteFailedMessage = "削除できませんでした";
export const itemDeletedMessage = (displayId: string) => `${displayId} を削除しました`;

export interface DeleteDialogState {
  item: Item;
  displayId: string;
  title: string;
  // lines は、本文(1 段落)と補足(1 段落)。
  lines: string[];
  ok: string;
}

// kindList は、下にあるものの種類を Epic・Story・Task・Bug・Step の順で重ねずに「・」でつなぐ。
const kindList = (list: Kind[]) => kinds.filter((k) => list.includes(k)).join("・");

// itemDeleteDialog は、確認ダイアログの中身(02「削除」の表)。
export function itemDeleteDialog(it: Item, displayId: string, p: DeletePreview): DeleteDialogState {
  const below =
    p.count > 0 ? `下にある ${p.count} 件(${kindList(p.kinds)})も一緒に削除されます。` : "";
  return {
    item: it,
    displayId,
    title: `${it.kind} を削除`,
    lines: [
      `${displayId} ${it.title} を削除します。${below}`,
      `設定の「削除済み」から戻せます。${p.retentionDays} 日を過ぎると完全に削除されます。`,
    ],
    ok: "削除",
  };
}

export interface ItemDeleteDeps {
  actions: Pick<ItemActionsPort, "deletePreview" | "deleteItem">;
  catalog: Catalog;
  toasts: Toasts;
  refresh: Pick<ItemRefresh, "afterDelete">;
  // detail と drafts は、自分で削除するとき、開いているパネルの書きかけを先に聞くために使う。
  detail: Pick<Detail, "view">;
  drafts: Pick<Drafts, "confirmLeave">;
}

export interface ItemDelete {
  // state は、出している確認ダイアログ(出していなければ null)。
  state: Readable<DeleteDialogState | null>;
  // open は、予告を読んでから確認ダイアログを出す。閲覧のみの Store では何もしない。
  open(item: Item): Promise<void>;
  // confirm は、削除する。トーストを出し、開いているパネルが item のものなら閉じ、ツリーとパネルを読み直す。
  confirm(): Promise<void>;
  cancel(): void;
}

export function createItemDelete(deps: ItemDeleteDeps): ItemDelete {
  const state = writable<DeleteDialogState | null>(null);
  // seq は、開く・閉じるの世代。予告を待っているあいだに閉じた・開き直したら、古い結果を捨てる。
  let seq = 0;
  // busy は、削除の実行中。そのあいだの confirm は受けない。
  let busy = false;

  const failed = (e: unknown) =>
    deps.toasts.show(e instanceof Refused ? e.message : itemDeleteFailedMessage, { error: true });

  return {
    state: readonly(state),
    async open(item) {
      const prefix = deps.catalog.workspace(item.storeId, item.workspaceId)?.prefix;
      if (prefix === undefined || deps.catalog.store(item.storeId)?.readOnly) return;
      // 開いているパネルが item か、その祖先のものなら、書きかけの確認(「入力を破棄」)を先に聞く。
      const panel = get(deps.detail.view);
      const inPanel =
        panel !== null &&
        (panel.item.id === item.id || panel.ancestors.some((a) => a.id === item.id));
      const gen = ++seq;
      if (inPanel && !(await deps.drafts.confirmLeave(isItemDraft))) {
        // 聞いているあいだに別の open があれば、そちらを生かす。
        if (gen === seq) state.set(null);
        return;
      }
      if (gen !== seq) return;
      let preview: DeletePreview;
      try {
        preview = await deps.actions.deletePreview(item.storeId, item.id);
      } catch (e) {
        if (gen !== seq) return;
        // 別の経路ですでに消えていたら、確認なしで読み直す(閉じる・消える)。
        if (e instanceof NotFound) await deps.refresh.afterDelete(item);
        else failed(e);
        return;
      }
      if (gen !== seq) return;
      state.set(itemDeleteDialog(item, itemDisplayId(prefix, item.number), preview));
    },
    async confirm() {
      const s = get(state);
      if (!s || busy) return;
      busy = true;
      try {
        await deps.actions.deleteItem(s.item.storeId, s.item.id);
        state.set(null);
        deps.toasts.show(itemDeletedMessage(s.displayId));
      } catch (e) {
        state.set(null);
        // すでに消えているのは、消した結果と同じなので、トーストなしで読み直す。
        if (!(e instanceof NotFound)) return failed(e);
      } finally {
        busy = false;
      }
      await deps.refresh.afterDelete(s.item);
    },
    cancel() {
      seq += 1;
      state.set(null);
    },
  };
}
