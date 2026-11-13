import { createItemActionsPort } from "@hedgehug/api";
import {
  createAfterMoved,
  createCopyId,
  createItemDelete,
  createItemDuplicate,
  createItemKind,
  createItemMenuHost,
  createItemMove,
  createItemRefresh,
  createProjectDelete,
  createTransferDialog,
  createWorkspaceDelete,
  leaveDeletedProject,
  leaveDeletedWorkspace,
} from "@hedgehug/core";
import { drafts } from "./drafts";
import { online as isOnline } from "./tab";
import {
  catalog,
  detail,
  itemDialog,
  itemsPort,
  navigation,
  projectScreen,
  relations,
  toasts,
  transfersPort,
  workspacesPort,
} from "./wire";

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
  workspaces: workspacesPort,
  catalog,
  toasts,
  refresh: itemRefresh,
});

// 「別のワークスペースへ移動…」のダイアログ(02「別のワークスペースと別の Store への移動」)。開くのは itemTransfer.open(item)。
export const itemTransfer = createTransferDialog({
  transfers: transfersPort,
  catalog,
  toasts,
  refresh: itemRefresh,
  detail,
  drafts,
  online: isOnline,
  afterMoved: createAfterMoved({ detail, refresh: itemRefresh, route: navigation.route }),
});

// 「種類を変更…」の種類のメニューと確認ダイアログ(02「種類の変更」)。開くのは itemKind.open(item, x, y)。
export const itemKind = createItemKind({
  actions: itemActions,
  items: itemsPort,
  catalog,
  toasts,
  refresh: itemRefresh,
});

// 「複製」(02「複製」)。実行すると詳細パネルが新しいアイテムに切り替わる。実行は itemDuplicate.run(item)。
export const itemDuplicate = createItemDuplicate({
  actions: itemActions,
  catalog,
  detail,
  toasts,
  refresh: itemRefresh,
});

// 「ID をコピー」(種類の行のボタンとアイテムのメニュー)。渡すのは ID の表記(HH-3)。
export const copyItemId = createCopyId({
  write: (text) => navigator.clipboard.writeText(text),
  toasts,
});

// アイテムのメニュー(詳細パネルの「⋯」・ツリーの行の「⋯」・メニューキー)。開くのは
// itemMenuHost.open(item, { rows, children, row }, x, y)。位置は menuAt(ボタンの getBoundingClientRect())で出す。
export const itemMenuHost = createItemMenuHost({
  items: itemsPort,
  catalog,
  toasts,
  refresh: itemRefresh,
  detail,
  dialog: itemDialog,
  remove: itemDelete,
  move: itemMove,
  kind: itemKind,
  transfer: itemTransfer,
  duplicate: itemDuplicate,
  copyId: copyItemId,
});

// プロジェクトの削除(02「名前の変更・移動・削除」の削除)。設定タブの「プロジェクトを削除」が開く。
// 削除できたあとの移り先(ワークスペースの概要へ replace、そのあと一覧を読み直す)は core の leaveDeletedProject。
export const projectDelete = createProjectDelete({
  actions: itemActions,
  catalog,
  toasts,
  afterDelete: (project) => leaveDeletedProject({ navigation, catalog }, project),
});

// ワークスペースの削除(03「削除」)。「一般」の「ワークスペースを削除…」とサイドバーのメニューの「削除…」が開く。
// 削除できたあとの移り先(ホームへ replace、詳細パネルを閉じ、一覧を読み直す)は core の leaveDeletedWorkspace。
export const workspaceDelete = createWorkspaceDelete({
  actions: workspacesPort,
  catalog,
  toasts,
  afterDelete: (ws) => leaveDeletedWorkspace({ navigation, catalog, detail }, ws),
});

// 画面を移ったら、出している確認ダイアログとメニューを閉じる(01「画面の移り方」)。
navigation.route.subscribe(() => {
  projectDelete.cancel();
  workspaceDelete.cancel();
  itemDelete.cancel();
  itemMove.close();
  itemMove.cancelMove();
  itemKind.cancel();
  itemTransfer.cancel();
  itemMenuHost.close();
  itemMenuHost.closeTemplate();
});
