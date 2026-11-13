import { NotFound, Refused } from "../../model/errors";
import type { Item } from "../../model/item";
import type { ItemsPort } from "../../port/items";
import { get, type Readable, readonly, writable } from "../../reactive/store";
import type { Catalog } from "../catalog/catalog";
import { itemDisplayId } from "../router/route";
import type { Toasts } from "../toasts/toasts";
import type { TransferDialog } from "../transfers/dialog";
import type { TemplateSubject } from "../workspaces/saveAsTemplate";
import { childKindOf } from "./createDefaults";
import type { ItemDialog } from "./createDialogTypes";
import type { ItemDelete } from "./delete";
import type { Detail } from "./detail";
import type { ItemDuplicate } from "./duplicate";
import type { ItemKind } from "./kind";
import { type ItemMenuId, itemMenu, itemMenuContext, type MenuEntry } from "./menu";
import type { ItemMove } from "./move";
import type { ItemRefresh } from "./refresh";
import { visibleNeighbors } from "./siblings";

// アイテムのメニューを開く、全体で 1 つの口(02「上から順の構成」、04「ツリー」の「行」)。詳細パネルの「⋯」、
// ツリーの行の「⋯」、メニューキー・Shift+F10 が、同じ itemMenuHost.open(item, ctx, x, y) を呼ぶ。
// 項目は core の itemMenu、選んだあとの動きは、操作ごとの controller(削除・移動・別のワークスペースへの移動・種類の変更・複製)に渡す。

// 断りでも「ない」でもない失敗(通信の失敗など)のときの文言。
export const reorderFailedMessage = "順番を変更できませんでした";

// menuAt は、「⋯」ボタンの下に出すメニューの位置。rect は getBoundingClientRect() の結果。
export const menuAt = (rect: { left: number; bottom: number }) => ({
  x: rect.left,
  y: rect.bottom + 4,
});

// ItemMenuHostContext は、メニューを開く画面が渡すもの。rows は、画面に見えている行を見えている順に並べたもの
// (ツリーは visibleRows、詳細パネルは DetailView.siblings。「上へ移動」「下へ移動」の基準を決める)。
// children は、そのアイテムの子(「テンプレートとして保存…」の「子と Step を含める」を出すかを決める)。
// row は、ツリーの行のメニューか(先頭に「詳細を開く」「{種類} を追加」が付く)。詳細パネルは false。
// 閲覧のみか・ほかのプロジェクトがあるかは、host が一覧(catalog)から調べる。
export interface ItemMenuHostContext {
  rows: readonly Item[];
  children: readonly Item[];
  row: boolean;
}

export interface ItemMenuState {
  item: Item;
  x: number;
  y: number;
  entries: MenuEntry[];
}

export interface ItemMenuHostDeps {
  // items は、ItemsPort.reorder だけを使う。
  items: Pick<ItemsPort, "reorder">;
  catalog: Catalog;
  toasts: Toasts;
  refresh: Pick<ItemRefresh, "afterChange">;
  // detail・dialog は、ツリーの行のメニューの「詳細を開く」「{種類} を追加」(行の「+」と同じ開き方)。
  detail: Pick<Detail, "open">;
  dialog: Pick<ItemDialog, "open">;
  remove: Pick<ItemDelete, "open">;
  move: Pick<ItemMove, "open">;
  kind: Pick<ItemKind, "open">;
  transfer: Pick<TransferDialog, "open">;
  duplicate: Pick<ItemDuplicate, "run">;
  copyId(displayId: string): Promise<void>;
}

export interface ItemMenuHost {
  // state は、開いているメニュー(なければ null)。template は、「テンプレートとして保存…」のダイアログの対象。
  state: Readable<ItemMenuState | null>;
  template: Readable<TemplateSubject | null>;
  // open は、item のメニューを (x, y) に開く。一覧にない item なら何もしない。
  open(item: Item, ctx: ItemMenuHostContext, x: number, y: number): void;
  // reorder は、メニューの「上へ移動」「下へ移動」と同じ動きを、メニューを開かずに行う(キーの Alt+↑↓。04)。rows は
  // open の ctx.rows と同じ(見えている行)。動かせたら true。見えている兄弟がない・閲覧のみの Store・断り・失敗・
  // ほかの並べ替えの実行中は false。
  reorder(item: Item, rows: readonly Item[], dir: "up" | "down"): Promise<boolean>;
  // select は、メニューの項目を選ぶ。メニューを閉じて、項目の操作を始める。無効の項目は何もしない。
  select(id: ItemMenuId): Promise<void>;
  close(): void;
  closeTemplate(): void;
}

export function createItemMenuHost(deps: ItemMenuHostDeps): ItemMenuHost {
  const state = writable<ItemMenuState | null>(null);
  const template = writable<TemplateSubject | null>(null);
  // current は、開いているメニューの文脈(select が見えている兄弟と子を引く)。
  let current: ItemMenuHostContext = { rows: [], children: [], row: false };
  // busy は、並べ替えの実行中。そのあいだの並べ替えは受けない(古い並びを基準に動かさない)。
  let busy = false;

  // reorder は、見えている直前(「上」)・直後(「下」)の兄弟を基準に、item の 1 件だけを動かす。成功してもトーストは
  // 出さない(02「上から順の構成」: アクティビティだけ)。基準の兄弟がないときは何もしない。読み直しが終わるまで
  // busy のままにする(読み直す前の並びを基準に、次を動かさない)。
  async function reorder(item: Item, rows: readonly Item[], dir: "up" | "down"): Promise<boolean> {
    if (busy || deps.catalog.store(item.storeId)?.readOnly) return false;
    const { prev, next } = visibleNeighbors(rows, item.id);
    const anchor = dir === "up" ? prev : next;
    if (!anchor) return false;
    busy = true;
    try {
      try {
        await deps.items.reorder(item.storeId, item.id, anchor.id, dir === "down");
      } catch (e) {
        // すでに消えているのは、画面を読み直して消す(トーストなし)。
        if (e instanceof NotFound) await deps.refresh.afterChange();
        else
          deps.toasts.show(e instanceof Refused ? e.message : reorderFailedMessage, {
            error: true,
          });
        return false;
      }
      await deps.refresh.afterChange();
      return true;
    } finally {
      busy = false;
    }
  }

  // run は、選んだ項目の操作を始める(メニューはもう閉じている)。
  async function run(id: ItemMenuId, s: ItemMenuState, ctx: ItemMenuHostContext): Promise<void> {
    const { item, x, y } = s;
    switch (id) {
      case "open":
        return deps.detail.open(item.storeId, item.id);
      case "add-child": {
        const kind = childKindOf(item.kind);
        if (!kind) return;
        const { workspaceId, projectId } = item;
        return deps.dialog.open({ workspaceId, projectId, kind, parentId: item.id });
      }
      case "move-up":
      case "move-down":
        return void (await reorder(item, ctx.rows, id === "move-up" ? "up" : "down"));
      case "move":
        return deps.move.open(item, x, y);
      case "transfer":
        return deps.transfer.open(item);
      case "change-kind":
        return deps.kind.open(item, x, y);
      case "duplicate":
        return deps.duplicate.run(item);
      case "save-as-template": {
        const store = deps.catalog.store(item.storeId);
        if (store) template.set({ store, item, children: [...ctx.children] });
        return;
      }
      case "copy-id": {
        const prefix = deps.catalog.workspace(item.storeId, item.workspaceId)?.prefix;
        if (prefix !== undefined) await deps.copyId(itemDisplayId(prefix, item.number));
        return;
      }
      case "delete":
        return deps.remove.open(item);
    }
  }

  return {
    state: readonly(state),
    template: readonly(template),
    reorder,
    open(item, ctx, x, y) {
      const store = deps.catalog.store(item.storeId);
      const workspace = deps.catalog.workspace(item.storeId, item.workspaceId);
      if (!store || !workspace) return;
      current = ctx;
      const entries = itemMenu(
        itemMenuContext(item, ctx.rows, {
          readOnly: store.readOnly,
          projectCount: workspace.projects.length,
          row: ctx.row,
        }),
      );
      state.set({ item, x, y, entries });
    },
    async select(id) {
      const s = get(state);
      const entry = s?.entries.find((e) => e.id === id);
      if (!s || entry?.disabled !== false) return;
      state.set(null);
      await run(id, s, current);
    },
    close() {
      state.set(null);
    },
    closeTemplate() {
      template.set(null);
    },
  };
}
