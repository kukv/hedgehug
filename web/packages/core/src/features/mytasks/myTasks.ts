import type { MyTasksPort } from "../../port/myTasks";
import { get, type Readable, readonly, writable } from "../../reactive/store";
import type { ScreenStates } from "../screenstate/screenStates";
import { searchWaitMs } from "../search/search";
import type { SearchRow } from "../search/types";
import type { Timer, Toasts } from "../toasts/toasts";
import { type MyTasksGroupBy, myTasksGroupChoices } from "./group";

// マイタスクの画面の流れ(04「マイタスク」、E4a)。絞り込みとまとめ方は画面の状態 `my-tasks` に
// `{filter, groupBy}` で置く。入力のたびに wait ms 待ってから問い合わせ、古い応答は捨てる。

export const myTasksScreen = "my-tasks";

export interface MyTasksState {
  filter: string;
  groupBy: MyTasksGroupBy;
  rows: SearchRow[];
  errors: string[];
  // loaded は、一度でも読めたか。読めなかったときは前の行のまま変えない。
  loaded: boolean;
}

export interface MyTasks {
  state: Readable<MyTasksState>;
  // open は、画面の状態を読んで 1 回問い合わせる。
  open(): Promise<void>;
  // setFilter は、状態をすぐ替えて画面の状態に書き、待ってから問い合わせる。
  setFilter(text: string): void;
  // setGroupBy は、書くだけで問い合わせない。
  setGroupBy(by: MyTasksGroupBy): void;
  reload(): Promise<void>;
}

// readSaved は、形の合わない項目だけ初めの値にする(E4a-9)。
function readSaved(raw: unknown): { filter: string; groupBy: MyTasksGroupBy } {
  const r = typeof raw === "object" && raw !== null ? (raw as Record<string, unknown>) : {};
  return {
    filter: typeof r.filter === "string" ? r.filter : "",
    groupBy: myTasksGroupChoices.find((c) => c.id === r.groupBy)?.id ?? "due",
  };
}

export function createMyTasks(deps: {
  port: MyTasksPort;
  screenStates: ScreenStates;
  timer: Timer;
  toasts: Toasts;
}): MyTasks {
  const state = writable<MyTasksState>({
    filter: "",
    groupBy: "due",
    rows: [],
    errors: [],
    loaded: false,
  });
  let runs = 0;
  let pending: unknown;

  const cancel = () => {
    if (pending !== undefined) deps.timer.clear(pending);
    pending = undefined;
  };

  const write = () => {
    const { filter, groupBy } = get(state);
    deps.screenStates.write(myTasksScreen, { filter, groupBy });
  };

  // run は、いまの絞り込みで問い合わせる。失敗しても投げず、前の行を残してエラーのトーストで出す。
  async function run(): Promise<void> {
    runs += 1;
    const mine = runs;
    try {
      const r = await deps.port.list(get(state).filter);
      if (mine !== runs) return;
      state.update((s) => ({ ...s, rows: r.rows, errors: r.errors, loaded: true }));
    } catch (e) {
      if (mine !== runs) return;
      deps.toasts.show(e instanceof Error ? e.message : String(e), { error: true });
    }
  }

  return {
    state: readonly(state),
    async open() {
      cancel();
      const saved = readSaved(await deps.screenStates.read<unknown>(myTasksScreen, null));
      cancel();
      state.update((s) => ({ ...s, ...saved }));
      await run();
    },
    setFilter(text) {
      cancel();
      state.update((s) => ({ ...s, filter: text }));
      write();
      // 飛んでいる問い合わせの応答は古い絞り込みのものなので捨てる。
      runs += 1;
      pending = deps.timer.set(() => {
        pending = undefined;
        void run();
      }, searchWaitMs);
    },
    setGroupBy(by) {
      state.update((s) => ({ ...s, groupBy: by }));
      write();
    },
    reload() {
      cancel();
      return run();
    },
  };
}
