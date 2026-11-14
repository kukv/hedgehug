// 一覧の「選んでいる行」(F2b1)。行は { storeId, itemId } で見分ける(内部 ID は Store の中でだけ一意)。
// パレットの Selection(選んでいるアイテム 1 件)とは別のもの。

import { type Readable, readonly, writable } from "../../reactive/store";

export interface SelectedRow {
  storeId: string;
  itemId: string;
  display: string;
  readOnly: boolean;
}

export interface RowSelection {
  rows: Readable<SelectedRow[]>;
  toggle(row: SelectedRow, on: boolean): void;
  clear(): void;
  // prune は、visible の順に、選んでいる行だけを残す。display・readOnly は visible の値で置き換える。
  prune(visible: SelectedRow[]): void;
}

const same = (a: SelectedRow, b: SelectedRow) => a.storeId === b.storeId && a.itemId === b.itemId;

export function createRowSelection(): RowSelection {
  const rows = writable<SelectedRow[]>([]);
  let current: SelectedRow[] = [];
  const put = (next: SelectedRow[]) => {
    current = next;
    rows.set(next);
  };
  return {
    rows: readonly(rows),
    toggle(row, on) {
      const has = current.some((r) => same(r, row));
      if (on && !has) put([...current, row]);
      else if (!on && has) put(current.filter((r) => !same(r, row)));
    },
    clear() {
      if (current.length > 0) put([]);
    },
    prune(visible) {
      put(visible.filter((v) => current.some((r) => same(r, v))));
    },
  };
}

export function isSelected(rows: SelectedRow[], storeId: string, itemId: string): boolean {
  return rows.some((r) => r.storeId === storeId && r.itemId === itemId);
}

export function selectedText(n: number): string {
  return `${n} 件を選択中`;
}

export function allReadOnly(rows: SelectedRow[]): boolean {
  return rows.length > 0 && rows.every((r) => r.readOnly);
}
