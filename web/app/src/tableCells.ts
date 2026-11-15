import { createTableCells, get } from "@hedgehug/core";
import { reloads } from "./reloads";
import { search } from "./search";
import { catalogPort, itemsPort, planningPort, workspacesPort } from "./wire";
import { itemRefresh } from "./wireActions";
import { toasts } from "./wireBase";

// 検索のテーブルのセルの編集(E2b)の、選択肢の元と保存。結果の行が変わるたびに、まだ読んでいないワークスペースの元を読む。
export const searchCells = createTableCells({
  items: itemsPort,
  planning: planningPort,
  workspaces: workspacesPort,
  catalogPort,
  toasts,
  changed: () => itemRefresh.afterChange(),
});

search.state.subscribe((s) => searchCells.load(s.result?.rows ?? []));

// Store が変わった知らせでは元を捨て、いまの行の元を読み直す(読めるまでは前の元で描く)。
reloads.add(async () => {
  searchCells.reset();
  searchCells.load(get(search.state).result?.rows ?? []);
});
