import { createItemActionsPort } from "@hedgehug/api";
import { createItemDelete, createItemMove, createItemRefresh } from "@hedgehug/core";
import { drafts } from "./drafts";
import { catalog, detail, navigation, projectScreen, relations, toasts } from "./wire";

// アイテム・プロジェクトの操作(A4b)の組み立て。wire.ts が 300 行の上限なので、A4b の分はここに足す。
// 向きは wireActions.ts → wire.ts の一方向(wire.ts はここを import しない)。

export const itemActions = createItemActionsPort();
// 削除・移動・種類の変更・複製・並べ替えのあとに呼ぶ読み直し(ツリー・詳細パネル・関連の区画)。
export const itemRefresh = createItemRefresh({ detail, projectScreen, sections: [relations] });

// アイテムの削除の確認ダイアログ(02「削除」)。開くのは itemDelete.open(item)。
export const itemDelete = createItemDelete({
  actions: itemActions,
  catalog,
  toasts,
  refresh: itemRefresh,
  detail,
  drafts,
});

// 「移動…」の候補のメニュー(02「同じワークスペースの中での移動」)。開くのは itemMove.open(item, x, y)。
export const itemMove = createItemMove({
  actions: itemActions,
  catalog,
  toasts,
  refresh: itemRefresh,
});

// 画面を移ったら、出している確認ダイアログと候補のメニューを閉じる(01「画面の移り方」)。
navigation.route.subscribe(() => {
  itemDelete.cancel();
  itemMove.close();
});
