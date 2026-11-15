import type { MyTasksPort } from "../../port/myTasks";
import { type Readable, readonly, writable } from "../../reactive/store";
import type { SearchRow } from "../search/types";

// バッジの件数と候補の範囲に使う、絞り込みなしのマイタスク(E4a)。画面の絞り込みには影響されない。

export interface MyTaskBase {
  // rows は、まだ読めていないあいだ null。読めなかったときは前の値のまま(トーストは出さない)。
  rows: Readable<SearchRow[] | null>;
  reload(): Promise<void>;
}

// myTaskProjects は、行のあるプロジェクトの `{storeId}/{projectId}` の集まり。
export function myTaskProjects(rows: SearchRow[]): ReadonlySet<string> {
  return new Set(rows.map((r) => `${r.storeId}/${r.projectId}`));
}

export function createMyTaskBase(deps: { port: MyTasksPort }): MyTaskBase {
  const rows = writable<SearchRow[] | null>(null);
  let runs = 0;
  return {
    rows: readonly(rows),
    async reload() {
      runs += 1;
      const mine = runs;
      try {
        const r = await deps.port.list("");
        if (mine === runs) rows.set(r.rows);
      } catch {
        // 前の値のまま。
      }
    },
  };
}
