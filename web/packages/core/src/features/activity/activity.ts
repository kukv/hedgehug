import type { ActivityFilter, ActivityRow } from "../../model/activity";
import type { ActivityPort } from "../../port/activity";
import { get, type Readable, readonly, writable } from "../../reactive/store";
import type { Toasts } from "../toasts/toasts";

// アクティビティの画面の状態(06「アクティビティ」)。行は新しい順に 300 件ずつ読み、「さらに読み込む」で続きを足す。

export interface ActivityState {
  filter: ActivityFilter;
  rows: ActivityRow[];
  // more は、「さらに読み込む」を出すか。loading は、最初の行か続きを読んでいるあいだ。
  more: boolean;
  loading: boolean;
  // loaded は、いまの絞り込みの一覧を読み込めたか。読み込みに失敗したままのときは false(空の表示を出さない)。
  loaded: boolean;
}

export interface Activity {
  state: Readable<ActivityState>;
  // open・more・setFilter は、読めなかったらエラーのトーストを出し、投げない。
  open(): Promise<void>;
  more(): Promise<void>;
  // setFilter は、絞り込みを変えて先頭から読み直す。
  setFilter(f: Partial<ActivityFilter>): Promise<void>;
  // reload は、画面への反映(出来事 store・sync)。出していた件数を覆うまで先頭から読み直す。失敗は出さない。
  reload(): Promise<void>;
}

export interface ActivityDeps {
  port: ActivityPort;
  toasts: Toasts;
}

export function createActivity(deps: ActivityDeps): Activity {
  const state = writable<ActivityState>({
    filter: { workspace: "", kind: "all" },
    rows: [],
    more: false,
    loading: false,
    loaded: false,
  });
  // loads は、古い答えを捨てるための通し番号。
  let loads = 0;

  // failed は、読み込みの失敗をエラーのトーストで出す。古い読み込みの失敗は、いまの表示と関係ないので出さない。
  function failed(mine: number, e: unknown) {
    if (mine !== loads) return;
    state.update((s) => ({ ...s, loading: false }));
    deps.toasts.show(e instanceof Error ? e.message : String(e), { error: true });
  }

  // first は、絞り込みの最初の 300 件を読む。
  async function first(filter: ActivityFilter) {
    loads += 1;
    const mine = loads;
    state.set({ filter, rows: [], more: false, loading: true, loaded: false });
    try {
      const page = await deps.port.list(filter);
      if (mine !== loads) return;
      state.set({ filter, rows: page.rows, more: page.more, loading: false, loaded: true });
    } catch (e) {
      failed(mine, e);
    }
  }

  // again は、いまの絞り込みを、出していた件数を覆うまで 300 件ずつ読み直す(C4d-14)。
  // 失敗したら、読み込み中の表示だけを戻して投げる。
  async function again() {
    loads += 1;
    const mine = loads;
    const { filter, rows: shown } = get(state);
    let rows: ActivityRow[] = [];
    let more = false;
    try {
      do {
        const page = await deps.port.list(filter, rows.at(-1)?.cursor);
        if (mine !== loads) return;
        rows = [...rows, ...page.rows];
        more = page.more;
      } while (more && rows.length < shown.length);
    } catch (e) {
      if (mine === loads) state.update((s) => ({ ...s, loading: false }));
      throw e;
    }
    state.set({ filter, rows, more, loading: false, loaded: true });
  }

  return {
    state: readonly(state),
    open: () => first(get(state).filter),
    setFilter: (f) => first({ ...get(state).filter, ...f }),
    async more() {
      const s = get(state);
      const last = s.rows.at(-1);
      if (!s.more || s.loading || !last) return;
      loads += 1;
      const mine = loads;
      state.set({ ...s, loading: true });
      try {
        const page = await deps.port.list(s.filter, last.cursor);
        if (mine !== loads) return;
        state.set({
          filter: s.filter,
          rows: [...s.rows, ...page.rows],
          more: page.more,
          loading: false,
          loaded: true,
        });
      } catch (e) {
        failed(mine, e);
      }
    },
    // 画面への反映は失敗を出さない(A2 の決定 29)。
    reload: () => again().catch(() => {}),
  };
}
