import type { SearchPort } from "../../port/search";
import { get, readonly, writable } from "../../reactive/store";
import type { ScreenStates } from "../screenstate/screenStates";
import type { RowSelection } from "../selection/rowSelection";
import type { Timer, Toasts } from "../toasts/toasts";
import {
  readSearchState,
  type Search,
  type SearchLayout,
  type SearchState,
  savedSearch,
} from "./layout";
import { selectable, tableRequest } from "./table";
import type { SearchResult } from "./types";

export type { Search, SearchState } from "./layout";

// 検索の画面の流れ(04「検索」、E1-16・E1-19)。絞り込みの文字列は画面の状態 `search` に
// `{query, layout, table, tree}`(E3a-2)で置き、入力のたびに wait ms 待ってから検索し、古い応答は捨てる。

export const searchScreen = "search";
export const searchWaitMs = 150;

export interface SearchDeps {
  port: SearchPort;
  screenStates: ScreenStates;
  timer: Timer;
  toasts: Toasts;
  wait?: number;
  // selection は、結果が変わるたびに選べる行で整理し(E2a-16)、テーブルを離れるときに解除する(E3a-10)。
  selection?: Pick<RowSelection, "prune" | "clear">;
}

export function createSearch(deps: SearchDeps): Search {
  const wait = deps.wait ?? searchWaitMs;
  const state = writable<SearchState>({
    ...readSearchState(null),
    result: null,
    resultQuery: null,
    loading: false,
  });
  // runs は、古い応答を捨てるための通し番号。pending は、待っている検索の印。
  // touched は、絞り込みを入れ替えた回数(open が読んでいるあいだの入力を見分ける)。
  let runs = 0;
  let touched = 0;
  let pending: unknown;
  // flight は、いま飛んでいる最新の検索の番号(なければ 0)。prepared は、prepareQuery のあとまだ検索していない印。
  let flight = 0;
  let prepared = false;
  let waiters: (() => void)[] = [];
  // loaded は、open が画面の状態を読み終えたか。読む前の印: queued は prepareQuery の絞り込み、toTable はテーブルにすること。
  let loaded = false;
  let queued: string | null = null;
  let toTable = false;

  const busy = () => pending !== undefined || prepared || (flight !== 0 && flight === runs);
  const release = () => {
    if (busy()) return;
    const done = waiters;
    waiters = [];
    for (const w of done) w();
  };

  const cancel = () => {
    if (pending !== undefined) deps.timer.clear(pending);
    pending = undefined;
  };

  // run は、いまの絞り込みで検索する。画面に未処理の拒否を残さないため、失敗しても投げない。
  // 失敗は、前の結果を残してエラーのトーストで出す(読み込みの失敗の出し方は受信箱と同じ)。
  // reload は Store の変更の反映なので、失敗を出さない(A2 の決定 29)。古い検索の失敗も出さない。
  async function run(report = true): Promise<void> {
    runs += 1;
    const mine = runs;
    flight = mine;
    prepared = false;
    state.update((s) => ({ ...s, loading: true }));
    let result: SearchResult | null = null;
    let failure: Error | null = null;
    const { query, table } = get(state);
    try {
      result = await deps.port.search(query, tableRequest(table));
    } catch (e) {
      failure = e instanceof Error ? e : new Error(String(e));
    }
    if (mine !== runs) {
      release();
      return;
    }
    flight = 0;
    state.update((s) => ({
      ...s,
      result: result ?? s.result,
      resultQuery: result ? query : null,
      loading: false,
    }));
    if (result) deps.selection?.prune(selectable(result.rows));
    release();
    if (failure && report) deps.toasts.show(failure.message, { error: true });
  }

  const write = () => deps.screenStates.write(searchScreen, savedSearch(get(state)));

  // put は、絞り込みを入れ替えて書く。読む前の prepareQuery の絞り込みより新しい。
  function put(query: string) {
    touched += 1;
    queued = null;
    state.update((s) => ({ ...s, query }));
    write();
  }

  function setLayout(layout: SearchLayout) {
    state.update((s) => ({ ...s, layout }));
    write();
    if (layout !== "table") deps.selection?.clear();
  }

  return {
    state: readonly(state),
    async open() {
      cancel();
      const before = touched;
      const saved = readSearchState(await deps.screenStates.read<unknown>(searchScreen, null));
      loaded = true;
      const query = queued;
      const switched = toTable || query !== null;
      queued = null;
      toTable = false;
      // 読んでいるあいだに入力かパレットで絞り込みが入れ替わっていたら、新しいほうを使う。
      if (touched !== before) return;
      if (query !== null) saved.query = query;
      if (switched) saved.layout = "table";
      state.update((s) => ({ ...s, ...saved }));
      // テーブル以外で開いたら、前にテーブルで選んでいた行を残さない(E3a-10)。
      if (saved.layout !== "table") deps.selection?.clear();
      if (switched) write();
      await run();
    },
    setQuery(text) {
      cancel();
      put(text);
      // 飛んでいる検索の応答は古い絞り込みのものなので捨てる。
      runs += 1;
      state.update((s) => ({ ...s, loading: true }));
      pending = deps.timer.set(() => {
        pending = undefined;
        void run();
      }, wait);
    },
    async replaceQuery(text) {
      cancel();
      state.update((s) => ({ ...s, layout: "table" }));
      put(text);
      await run();
    },
    prepareQuery(text) {
      cancel();
      prepared = true;
      if (loaded) {
        state.update((s) => ({ ...s, layout: "table" }));
        put(text);
        return;
      }
      // まだ読んでいない: 書かずに印だけを付け、open が読んだ値の絞り込みを置き換える。
      queued = text;
      state.update((s) => ({ ...s, query: text }));
    },
    setLayout,
    showTable() {
      if (loaded) setLayout("table");
      else toTable = true;
    },
    setCollapsed(collapsed) {
      state.update((s) => ({ ...s, tree: { collapsed } }));
      write();
    },
    cancelPrepare() {
      prepared = false;
      release();
    },
    reload() {
      cancel();
      return run(false);
    },
    setTable(next) {
      const prev = get(state).table;
      state.update((s) => ({ ...s, table: next }));
      write();
      const same =
        prev.group === next.group && JSON.stringify(prev.sort) === JSON.stringify(next.sort);
      if (same) return;
      touched += 1;
      cancel();
      void run();
    },
    settled() {
      if (!busy()) return Promise.resolve();
      return new Promise<void>((resolve) => waiters.push(resolve));
    },
  };
}
