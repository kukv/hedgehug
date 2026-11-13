import { NotFound } from "../../model/errors";
import type { HistoryRow } from "../../model/history";
import type { NotesPort, NoteTarget } from "../../port/notes";
import { type Readable, readonly, writable } from "../../reactive/store";

// 履歴の区画の状態(02「履歴」): 新しい順の行を 40 件まで出し、古いものがあれば「すべて表示」を置く。

export const historyLimit = 40;

export interface HistoryState {
  // rows は、いま出す行。more は、「すべて表示」を置くか(40 件より古いものがあって、まだ全件を出していない)。
  rows: HistoryRow[];
  more: boolean;
  // loaded は、読み終えたか(読むまでは「履歴はありません」を出さない)。
  loaded: boolean;
}

export interface History {
  state: Readable<HistoryState>;
  // load は、対象の履歴を読む。同じ対象の読み直しでは、「すべて表示」を保つ。
  load(target: NoteTarget): Promise<void>;
  // reload は、画面への反映(06)。開いている対象を読み直す。開いていなければ何もしない。
  reload(): Promise<void>;
  showAll(): void;
  reset(): void;
}

export interface HistoryDeps {
  notes: Pick<NotesPort, "history">;
}

const blank: HistoryState = { rows: [], more: false, loaded: false };

export function createHistory(deps: HistoryDeps): History {
  const state = writable<HistoryState>(blank);
  let target: NoteTarget | null = null;
  let all: HistoryRow[] = [];
  let expanded = false;
  // loads は、古い答えを捨てるための通し番号。
  let loads = 0;

  const publish = () =>
    state.set({
      rows: expanded ? all : all.slice(0, historyLimit),
      more: !expanded && all.length > historyLimit,
      loaded: true,
    });

  const keyOf = (t: NoteTarget) => `${t.kind}/${t.storeId}/${t.id}`;

  async function read() {
    if (!target) return;
    const t = target;
    loads += 1;
    const mine = loads;
    try {
      const rows = await deps.notes.history(t);
      if (mine !== loads) return;
      all = rows;
      publish();
    } catch (e) {
      // なくなっていたら空にする。ほかの失敗は出さない(画面への反映は失敗を出さない。A2 の決定 29)。
      if (e instanceof NotFound && mine === loads) {
        all = [];
        publish();
      }
    }
  }

  return {
    state: readonly(state),
    async load(next) {
      if (!target || keyOf(target) !== keyOf(next)) {
        expanded = false;
        all = [];
        state.set(blank);
      }
      target = next;
      await read();
    },
    reload: read,
    showAll() {
      expanded = true;
      state.update((s) => (s.loaded ? { rows: all, more: false, loaded: true } : s));
    },
    reset() {
      target = null;
      loads += 1;
      expanded = false;
      all = [];
      state.set(blank);
    },
  };
}
