import { createTableCells } from "@hedgehug/core";
import { reloads } from "./reloads";
import { catalogPort, itemsPort, planningPort, workspacesPort } from "./wire";
import { itemRefresh } from "./wireActions";
import { toasts } from "./wireBase";

// 検索と保存したビューのテーブルのセルの編集(E2b)の、選択肢の元と保存。元は、テーブル(ItemTable)が出している行で読む。
export const searchCells = createTableCells({
  items: itemsPort,
  planning: planningPort,
  workspaces: workspacesPort,
  catalogPort,
  toasts,
  changed: () => itemRefresh.afterChange(),
});

// Store が変わった知らせでは元を捨て、最後に出した行の元を読み直す(読めるまでは前の元で描く)。
reloads.add(async () => searchCells.reset());
