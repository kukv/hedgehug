import type { SearchPort } from "../../port/search";
import { get, type Readable, readonly, writable } from "../../reactive/store";
import type { ScreenStates } from "../screenstate/screenStates";
import type { RowSelection } from "../selection/rowSelection";
import type { Timer, Toasts } from "../toasts/toasts";
import { defaultTable, readTable, selectable, type TableSettings, tableRequest } from "./table";
import type { SearchResult } from "./types";

// 検索の画面の流れ(04「検索」、E1-16・E1-19)。絞り込みの文字列は画面の状態 `search` に
// `{"query": "…", "table": {…}}` で置き、入力のたびに wait ms 待ってから検索し、古い応答は捨てる。

export const searchScreen = "search";
export const searchWaitMs = 150;

export interface SearchState {
  query: string;
  table: TableSettings;
  // result は、まだ一度も検索できていないあいだ null。検索に失敗したときは前の結果を残す。
  result: SearchResult | null;
  // resultQuery は、いまの result がどの絞り込みへの答えか。result が null のときと、最後の検索が失敗して前の結果を
  // 残しているときは null。
  resultQuery: string | null;
  loading: boolean;
}

export interface Search {
  state: Readable<SearchState>;
  // open は、画面を開いたときに、保存した絞り込みで検索する。
  open(): Promise<void>;
  // setQuery は、入力のたびに呼ぶ。画面の状態に書き、wait ms 待ってから検索する。
  setQuery(text: string): void;
  // replaceQuery は、パレットなどから絞り込みを入れ替える。待たずに検索する。
  replaceQuery(text: string): Promise<void>;
  // prepareQuery は、絞り込みを入れ替えて画面の状態に書くだけで、検索しない。ほかの画面から検索の画面を開くとき、
  // 開いた画面の open が 1 回だけ検索する。
  prepareQuery(text: string): void;
  // cancelPrepare は、prepareQuery の印を取り消す(検索の画面へ移れなかったとき)。settled を待つものを解く。
  cancelPrepare(): void;
  // reload は、Store が変わった知らせ(reloads)で、いまの絞り込みのまま読み直す。
  reload(): Promise<void>;
  // setTable は、テーブルの設定を画面の状態に書く。並べ替えかグループが変わったときだけ、待たずに検索する。
  setTable(next: TableSettings): void;
  // settled は、待ちの検索・飛んでいる検索・prepareQuery のあとのまだ検索していない絞り込みがなくなったら解ける。
  settled(): Promise<void>;
}

export interface SearchDeps {
  port: SearchPort;
  screenStates: ScreenStates;
  timer: Timer;
  toasts: Toasts;
  wait?: number;
  // selection は、結果が変わるたびに選べる行で整理する(E2a-16)。
  selection?: Pick<RowSelection, "prune">;
}

export function createSearch(deps: SearchDeps): Search {
  const wait = deps.wait ?? searchWaitMs;
  const state = writable<SearchState>({
    query: "",
    table: defaultTable,
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

  function write() {
    const { query, table } = get(state);
    deps.screenStates.write(searchScreen, { query, table });
  }

  function put(query: string) {
    touched += 1;
    state.update((s) => ({ ...s, query }));
    write();
  }

  return {
    state: readonly(state),
    async open() {
      cancel();
      const before = touched;
      const saved = await deps.screenStates.read<{ query?: unknown; table?: unknown } | null>(
        searchScreen,
        null,
      );
      // 読んでいるあいだに入力かパレットで絞り込みが入れ替わっていたら、新しいほうを使う。
      if (touched !== before) return;
      const query = saved && typeof saved.query === "string" ? saved.query : "";
      state.update((s) => ({ ...s, query, table: readTable(saved?.table) }));
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
      put(text);
      await run();
    },
    prepareQuery(text) {
      cancel();
      put(text);
      prepared = true;
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
