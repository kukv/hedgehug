import type { SearchPort } from "../../port/search";
import { get, type Readable, readonly, writable } from "../../reactive/store";
import type { ScreenStates } from "../screenstate/screenStates";
import type { Timer, Toasts } from "../toasts/toasts";
import type { SearchResult } from "./types";

// 検索の画面の流れ(04「検索」、E1-16・E1-19)。絞り込みの文字列は画面の状態 `search` に
// `{"query": "…"}` で置き、入力のたびに wait ms 待ってから検索し、古い応答は捨てる。

export const searchScreen = "search";
export const searchWaitMs = 150;

export interface SearchState {
  query: string;
  // result は、まだ一度も検索できていないあいだ null。検索に失敗したときは前の結果を残す。
  result: SearchResult | null;
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
  // reload は、Store が変わった知らせ(reloads)で、いまの絞り込みのまま読み直す。
  reload(): Promise<void>;
}

export interface SearchDeps {
  port: SearchPort;
  screenStates: ScreenStates;
  timer: Timer;
  toasts: Toasts;
  wait?: number;
}

export function createSearch(deps: SearchDeps): Search {
  const wait = deps.wait ?? searchWaitMs;
  const state = writable<SearchState>({ query: "", result: null, loading: false });
  // runs は、古い応答を捨てるための通し番号。pending は、待っている検索の印。
  // touched は、絞り込みを入れ替えた回数(open が読んでいるあいだの入力を見分ける)。
  let runs = 0;
  let touched = 0;
  let pending: unknown;

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
    state.update((s) => ({ ...s, loading: true }));
    let result: SearchResult | null = null;
    let failure: Error | null = null;
    try {
      result = await deps.port.search(get(state).query);
    } catch (e) {
      failure = e instanceof Error ? e : new Error(String(e));
    }
    if (mine !== runs) return;
    state.update((s) => ({ ...s, result: result ?? s.result, loading: false }));
    if (failure && report) deps.toasts.show(failure.message, { error: true });
  }

  function put(query: string) {
    touched += 1;
    state.update((s) => ({ ...s, query }));
    deps.screenStates.write(searchScreen, { query });
  }

  return {
    state: readonly(state),
    async open() {
      cancel();
      const before = touched;
      const saved = await deps.screenStates.read<{ query?: unknown } | null>(searchScreen, null);
      // 読んでいるあいだに入力かパレットで絞り込みが入れ替わっていたら、新しいほうを使う。
      if (touched !== before) return;
      const query = saved && typeof saved.query === "string" ? saved.query : "";
      state.update((s) => ({ ...s, query }));
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
    reload() {
      cancel();
      return run(false);
    },
  };
}
