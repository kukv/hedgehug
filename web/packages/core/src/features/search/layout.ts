// 検索の画面の状態 search の中身(E3a-2): `{query, layout, table, tree}`。形の合わない項目だけ初めの値にする。
// 検索の画面の流れ(search.ts の createSearch)の状態と操作の型もここに置く。

import type { Readable } from "../../reactive/store";
import { readTable, type TableSettings } from "./table";
import type { SearchResult } from "./types";

// SearchLayout は、検索の画面のレイアウト。カンバンは E5、ガントは E6 が値を足す。
export type SearchLayout = "tree" | "table";

// layoutChoices は、切り替えに出すレイアウト(E3a-1)。作ってあるものだけを、この順で並べる。
export const layoutChoices: { id: SearchLayout; label: string }[] = [
  { id: "tree", label: "ツリー" },
  { id: "table", label: "テーブル" },
];

// TreeSettings は、ツリーの設定。collapsed は閉じている行の鍵(行がなくなった鍵も残す。E3a-11)。
export interface TreeSettings {
  collapsed: string[];
}

export interface SavedSearch {
  query: string;
  layout: SearchLayout;
  table: TableSettings;
  tree: TreeSettings;
}

const isRecord = (v: unknown): v is Record<string, unknown> => typeof v === "object" && v !== null;

// readSearchState は、保存された値から画面の状態を作る。
export function readSearchState(raw: unknown): SavedSearch {
  const r = isRecord(raw) ? raw : {};
  const layout = layoutChoices.find((c) => c.id === r.layout)?.id ?? "table";
  const t = isRecord(r.tree) ? r.tree : {};
  const collapsed =
    Array.isArray(t.collapsed) && t.collapsed.every((k) => typeof k === "string")
      ? [...(t.collapsed as string[])]
      : [];
  return {
    query: typeof r.query === "string" ? r.query : "",
    layout,
    table: readTable(r.table),
    tree: { collapsed },
  };
}

// savedSearch は、画面の状態に書く値を取り出す(結果や読み込み中の印は書かない)。
export function savedSearch(s: SavedSearch): SavedSearch {
  return { query: s.query, layout: s.layout, table: s.table, tree: s.tree };
}

export interface SearchState extends SavedSearch {
  // result は、まだ一度も検索できていないあいだ null。検索に失敗したときは前の結果を残す。
  result: SearchResult | null;
  // resultQuery は、いまの result がどの絞り込みへの答えか。result が null のときと、最後の検索が失敗して前の結果を
  // 残しているときは null。
  resultQuery: string | null;
  loading: boolean;
}

export interface Search {
  state: Readable<SearchState>;
  // open は、画面を開いたときに、保存した絞り込みとレイアウトで検索する。
  open(): Promise<void>;
  // setQuery は、入力のたびに呼ぶ。画面の状態に書き、wait ms 待ってから検索する。
  setQuery(text: string): void;
  // replaceQuery は、パレットなどから絞り込みを入れ替え、テーブルにする(E3a-9)。待たずに検索する。
  replaceQuery(text: string): Promise<void>;
  // prepareQuery は、絞り込みを入れ替えてテーブルにするだけで、検索しない。ほかの画面から検索の画面を開くとき、
  // 開いた画面の open が 1 回だけ検索する。このタブでまだ open が読んでいなければ、読んだ値の絞り込みだけを
  // 置き換えてから書く(E3a-22。読む前に書くと、保存した列などが初めの値で上書きされる)。
  prepareQuery(text: string): void;
  // setLayout は、レイアウトを書くだけで検索しない。テーブル以外にするときは選択を解除する(E3a-10)。
  setLayout(layout: SearchLayout): void;
  // showTable は、テーブルで開く入口(E3a-9)が呼ぶ。まだ open が読んでいなければ、読んだあとでテーブルにする。
  showTable(): void;
  // setCollapsed は、ツリーの閉じている行の鍵を書くだけ(E3a-11)。
  setCollapsed(collapsed: string[]): void;
  // cancelPrepare は、prepareQuery の印を取り消す(検索の画面へ移れなかったとき)。settled を待つものを解く。
  cancelPrepare(): void;
  // reload は、Store が変わった知らせ(reloads)で、いまの絞り込みのまま読み直す。
  reload(): Promise<void>;
  // setTable は、テーブルの設定を画面の状態に書く。並べ替えかグループが変わったときだけ、待たずに検索する。
  setTable(next: TableSettings): void;
  // settled は、待ちの検索・飛んでいる検索・prepareQuery のあとのまだ検索していない絞り込みがなくなったら解ける。
  settled(): Promise<void>;
}
