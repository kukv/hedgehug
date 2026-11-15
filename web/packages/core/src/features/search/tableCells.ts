import { Refused } from "../../model/errors";
import type { PropertyInput } from "../../model/item";
import type { CatalogPort } from "../../port/catalog";
import type { ItemsPort } from "../../port/items";
import type { PlanningPort } from "../../port/planning";
import type { WorkspacesPort } from "../../port/workspaces";
import { type Readable, readonly, writable } from "../../reactive/store";
import { type Context, createContexts } from "../items/detailContext";
import { chosen, releaseChoices, sprintChoices } from "../planning/choices";
import { sprintSaveFailed } from "../planning/sprintDialogs";
import type { Toasts } from "../toasts/toasts";
import type { SearchRow } from "./types";

// テーブルのセルの編集(E2b)の、選択肢の元(行のワークスペースごとの設定・メンバー・スプリント・リリース)と保存。

export type CellEdit =
  | { kind: "property"; propertyId: string; value: PropertyInput }
  | { kind: "toggle"; propertyId: string; valueId: string; on: boolean }
  | { kind: "plan"; key: "sprint" | "release"; id: string };

export interface TableCells {
  // loaded は、選択肢の元を読み終えるたびに増える(行が描き直すため)。
  loaded: Readable<number>;
  context(storeId: string, workspaceId: string): Context | undefined;
  // load は、まだ読んでいない行のワークスペースの元を読み始める(テーブルが出している行が変わるたびに呼ぶ)。
  load(rows: SearchRow[]): void;
  // unload は、テーブルが消えるときに呼ぶ。最後に load した行がその行なら忘れる(別のテーブルが先に load していたら何もしない)。
  unload(rows: SearchRow[]): void;
  // reset は、元を捨て、最後に load した行の元を読み直す(読み終えるまでは前の元を返す)。
  reset(): void;
  // save は、1 回保存する。保存したら true。成否にかかわらず changed() と reload() を待つ。
  save(row: SearchRow, edit: CellEdit, reload: () => Promise<void>): Promise<boolean>;
}

export interface TableCellsDeps {
  items: Pick<ItemsPort, "setProperty" | "addValue" | "removeValue">;
  planning: PlanningPort;
  workspaces: WorkspacesPort;
  catalogPort: CatalogPort;
  toasts: Pick<Toasts, "show">;
  changed: () => Promise<void>;
}

const keyOf = (storeId: string, workspaceId: string) => `${storeId}/${workspaceId}`;

export function createTableCells(deps: TableCellsDeps): TableCells {
  // 読み込み(飛んでいるものも)は contexts、読めたものは loaded。reset は読み込みだけを捨て、読めるまでは前の元を返す。
  // round は reset した回数で、reset の前に始めた読み込みの結果は入れない(マイタスクの設定と同じ読み方)。
  let reading = new Map<string, Promise<Context>>();
  let contexts = createContexts(deps);
  const ready = new Map<string, Context>();
  let round = 0;
  const count = writable(0);
  // last は、最後に load した行(reset で読み直す)。
  let last: SearchRow[] = [];

  function load(rows: SearchRow[]) {
    last = rows;
    for (const r of rows) {
      const key = keyOf(r.storeId, r.workspaceId);
      if (reading.has(key)) continue;
      const mine = round;
      const map = reading;
      const p = contexts.of(r.storeId, r.workspaceId, false);
      map.set(key, p);
      p.then(
        (c) => {
          if (mine !== round) return;
          ready.set(key, c);
          count.update((n) => n + 1);
        },
        () => {
          if (map.get(key) === p) map.delete(key);
        },
      );
    }
  }

  function unload(rows: SearchRow[]) {
    if (last === rows) last = [];
  }

  function reset() {
    round += 1;
    reading = new Map();
    contexts = createContexts(deps);
    load(last);
  }

  async function write(row: SearchRow, edit: CellEdit): Promise<boolean> {
    const { storeId, itemId } = row;
    switch (edit.kind) {
      case "property":
        await deps.items.setProperty(storeId, itemId, edit.propertyId, edit.value);
        return true;
      case "toggle": {
        if (edit.on) await deps.items.addValue(storeId, itemId, edit.propertyId, edit.valueId);
        else await deps.items.removeValue(storeId, itemId, edit.propertyId, edit.valueId);
        return true;
      }
      case "plan": {
        const plan = ready.get(keyOf(storeId, row.workspaceId))?.plan;
        const current = row.values[edit.key];
        const choices = edit.key === "sprint" ? sprintChoices : releaseChoices;
        // 表示している値と同じなら送らない(使えない参照を書き換えない)。
        if (plan && edit.id === chosen(choices(plan, current), current)) return false;
        if (edit.key === "sprint")
          await deps.planning.setItemSprint(storeId, itemId, edit.id || null);
        else await deps.planning.setItemRelease(storeId, itemId, edit.id || null);
        return true;
      }
    }
  }

  // 断られたらその文を、ほかの失敗は「保存できませんでした」を、エラーのトーストに出す(どちらも読み直す)。
  async function save(row: SearchRow, edit: CellEdit, reload: () => Promise<void>) {
    let ok = true;
    try {
      if (!(await write(row, edit))) return true;
    } catch (e) {
      deps.toasts.show(e instanceof Refused ? e.message : sprintSaveFailed, { error: true });
      ok = false;
    }
    await deps.changed();
    await reload();
    return ok;
  }

  return {
    loaded: readonly(count),
    context: (storeId, workspaceId) => ready.get(keyOf(storeId, workspaceId)),
    load,
    unload,
    reset,
    save,
  };
}
