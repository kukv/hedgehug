import { createItemActionsPort } from "@hedgehug/api";
import { createItemRefresh } from "@hedgehug/core";
import { detail, projectScreen, relations } from "./wire";

// アイテム・プロジェクトの操作(A4b)の組み立て。wire.ts が 300 行の上限なので、A4b の分はここに足す。
// 向きは wireActions.ts → wire.ts の一方向(wire.ts はここを import しない)。

export const itemActions = createItemActionsPort();
// 削除・移動・種類の変更・複製・並べ替えのあとに呼ぶ読み直し(ツリー・詳細パネル・関連の区画)。
export const itemRefresh = createItemRefresh({ detail, projectScreen, sections: [relations] });
