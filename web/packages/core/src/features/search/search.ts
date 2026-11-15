import type { SearchPort } from "../../port/search";
import { get, readonly, writable } from "../../reactive/store";
import type { ScreenStates } from "../screenstate/screenStates";
import type { RowSelection } from "../selection/rowSelection";
import type { Timer, Toasts } from "../toasts/toasts";
import {
  initialSearchState,
  readSearchState,
  type SavedSearch,
  type Search,
  type SearchLayout,
  type SearchState,
  savedSearch,
} from "./layout";
import { selectable, tableRequest } from "./table";
import type { SearchResult } from "./types";

export type { Search, SearchState } from "./layout";

// 検索の画面の流れ(04「検索」、E1-16・E1-19)。絞り込みの文字列は画面の状態(初めは `search`)に
// `{query, layout, table, tree}`(E3a-2)で置き、入力のたびに wait ms 待ってから検索し、古い応答は捨てる。
// タブで最初の open が読み終えるまでは書かずに積み、読んだ値に重ねて 1 回書く(E3a-23)。

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
  // screen は、読み書きする画面の状態の鍵(初めは searchScreen)。fallback は、その画面の状態に何もないときに
  // readSearchState に渡す値(open のたびに呼ぶ)。
  screen?: string;
  fallback?: () => unknown;
}

export function createSearch(deps: SearchDeps): Search {
  const wait = deps.wait ?? searchWaitMs;
  const screen = deps.screen ?? searchScreen;
  const state = writable<SearchState>(initialSearchState());
  // runs は、古い応答を捨てるための通し番号。pending は、待っている検索の印。
  let runs = 0;
  let pending: unknown;
  // flight は、いま飛んでいる最新の検索の番号(なければ 0)。prepared は、検索を open に任せていてまだ検索していない印
  // (prepareQuery のあと、読む前の入力など)。
  let flight = 0;
  let prepared = false;
  let waiters: (() => void)[] = [];
  // loaded は open が読み終えたか。stash は読む前に変えた項目。queued は prepareQuery の印(次の open がテーブルにして重ねる)。
  let loaded = false;
  let stash: Partial<SavedSearch> = {};
  let queued: string | null = null;

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

  const write = () => deps.screenStates.write(screen, savedSearch(get(state)));

  // change は、項目を替えて書く。読む前は書かずに積む(読む前に書くと、保存した値を初めの値で上書きする)。
  function change(patch: Partial<SavedSearch>) {
    state.update((s) => ({ ...s, ...patch }));
    if (loaded) write();
    else stash = { ...stash, ...patch };
  }

  // put は、絞り込みを入れ替えて書く。prepareQuery の印より新しい。
  function put(patch: Partial<SavedSearch> & { query: string }) {
    queued = null;
    change(patch);
  }

  // later は、読む前なら検索を open に任せる(settled は open の検索まで解けない)。
  const later = () => {
    if (!loaded) prepared = true;
    return !loaded;
  };

  function setLayout(layout: SearchLayout) {
    change({ layout });
    if (layout !== "table") deps.selection?.clear();
  }

  return {
    state: readonly(state),
    async open() {
      cancel();
      // 読んだ値には、読んでいるあいだに書いたものも入っている(screenStates)。待っていた検索は open の検索にまとめる。
      const raw = await deps.screenStates.read<unknown>(screen, null);
      const saved = readSearchState(raw ?? deps.fallback?.());
      cancel();
      loaded = true;
      const next = { ...saved, ...stash };
      if (queued !== null) Object.assign(next, { query: queued, layout: "table" });
      const dirty = queued !== null || Object.keys(stash).length > 0;
      stash = {};
      queued = null;
      state.update((s) => ({ ...s, ...next }));
      // テーブル以外で開いたら、前にテーブルで選んでいた行を残さない(E3a-10)。
      if (next.layout !== "table") deps.selection?.clear();
      if (dirty) write();
      await run();
    },
    setQuery(text) {
      cancel();
      put({ query: text });
      // 飛んでいる検索の応答は古い絞り込みのものなので捨てる。
      runs += 1;
      state.update((s) => ({ ...s, loading: true }));
      if (later()) return;
      pending = deps.timer.set(() => {
        pending = undefined;
        void run();
      }, wait);
    },
    async replaceQuery(text) {
      cancel();
      put({ query: text, layout: "table" });
      if (!later()) await run();
    },
    prepareQuery(text) {
      cancel();
      prepared = true;
      queued = text;
      if (!loaded) state.update((s) => ({ ...s, query: text }));
    },
    setLayout,
    showTable() {
      if (!loaded || get(state).layout !== "table") setLayout("table");
    },
    setCollapsed(collapsed) {
      change({ tree: { collapsed } });
    },
    cancelPrepare() {
      prepared = false;
      queued = null;
      const typed = stash.query ?? initialSearchState().query;
      if (!loaded) state.update((s) => ({ ...s, query: typed }));
      release();
    },
    restore({ query, layout, table }) {
      cancel();
      put({ query, layout, table });
      if (layout !== "table") deps.selection?.clear();
      void run();
    },
    reload() {
      cancel();
      return run(false);
    },
    setTable(next) {
      const prev = get(state).table;
      change({ table: next });
      const same =
        prev.group === next.group && JSON.stringify(prev.sort) === JSON.stringify(next.sort);
      if (same) return;
      cancel();
      if (!later()) void run();
    },
    settled() {
      if (!busy()) return Promise.resolve();
      return new Promise<void>((resolve) => waiters.push(resolve));
    },
  };
}
