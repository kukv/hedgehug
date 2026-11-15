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

// deletedTreeText は、ツリーで `is:deleted` の検索をしたときに本体の代わりに出す文(E3a-7)。
export const deletedTreeText = "削除済みのアイテムはテーブルで表示します。";

// layoutNote は、ツリーで sort:・group: が書かれているときに、絞り込み欄の下へ灰色で出す注記(E3a-8)。
// 書いてあるかは応答から決める(画面は解析しない)。テーブルや結果がないときは null。
export function layoutNote(layout: SearchLayout, result: SearchResult | null): string | null {
  if (layout !== "tree" || !result) return null;
  const names = [result.hasSort && "sort:", result.group !== "" && "group:"].filter(Boolean);
  if (names.length === 0) return null;
  return `ツリー・ガントでは ${names.join(" と ")} は使われず、並び順のままです`;
}

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

// initialSearchState は、open が画面の状態を読む前の初めの状態(まだ検索していない)。
export function initialSearchState(): SearchState {
  return { ...readSearchState(null), result: null, resultQuery: null, loading: false };
}

// Search の setter は、このタブで最初の open が画面の状態を読み終えるまでは書かずに積み、open が読んだ値に
// 重ねて 1 回だけ書く(E3a-23。読む前に書くと、保存した値が初めの値で上書きされる)。読む前は検索もせず、
// open の 1 回の検索に任せる。
export interface Search {
  state: Readable<SearchState>;
  // open は、画面を開いたときに、保存した絞り込みとレイアウトで検索する。
  open(): Promise<void>;
  // setQuery は、入力のたびに呼ぶ。画面の状態に書き、wait ms 待ってから検索する。
  setQuery(text: string): void;
  // replaceQuery は、パレットなどから絞り込みを入れ替え、テーブルにする(E3a-9)。待たずに検索する。
  replaceQuery(text: string): Promise<void>;
  // prepareQuery は、ほかの画面から検索の画面を開くときに、絞り込みと「テーブルにする」の印だけを付ける(書かず、
  // 検索もしない)。移れて呼ばれる次の open が、読んだ値の絞り込みを置き換えてテーブルにし、1 回書いて 1 回検索する
  // (E3a-22・E3a-23)。このタブでまだ読んでいなければ、絞り込みを欄に出すだけはする(書かない)。
  prepareQuery(text: string): void;
  // setLayout は、レイアウトを書くだけで検索しない。テーブル以外にするときは選択を解除する(E3a-10)。
  setLayout(layout: SearchLayout): void;
  // showTable は、テーブルで開く入口(E3a-9)が呼ぶ。読んだあとですでにテーブルなら書かない。
  showTable(): void;
  // setCollapsed は、ツリーの閉じている行の鍵を書くだけ(E3a-11)。
  setCollapsed(collapsed: string[]): void;
  // cancelPrepare は、prepareQuery の印を消す(検索の画面へ移れなかったとき)。画面の状態は前のまま。
  // 読む前なら、欄の絞り込みを読む前に打ったもの(なければ初めの値)に戻す。settled を待つものを解く。
  cancelPrepare(): void;
  // reload は、Store が変わった知らせ(reloads)で、いまの絞り込みのまま読み直す。
  reload(): Promise<void>;
  // setTable は、テーブルの設定を画面の状態に書く。並べ替えかグループが変わったときだけ、待たずに検索する。
  setTable(next: TableSettings): void;
  // settled は、待ちの検索・飛んでいる検索・prepareQuery のあとのまだ検索していない絞り込みがなくなったら解ける。
  settled(): Promise<void>;
}
