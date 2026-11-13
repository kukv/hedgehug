import { NotFound, Refused } from "../../model/errors";
import type { Worklog } from "../../model/notes";
import type { NotesPort } from "../../port/notes";
import { get, type Readable, readonly, writable } from "../../reactive/store";
import { formatDay } from "../dates/date";
import type { Toasts } from "../toasts/toasts";
import { parseHours } from "./hours";

// 作業時間の区画の状態(02「作業時間」)。アイテムの詳細パネルだけが持つ。

export const worklogDeleteDialog = { title: "作業時間を削除", ok: "削除" };
export const worklogDeletedMessage = "作業時間を削除しました";
// 断りでも「ない」でもない失敗(通信の失敗など)のときのエラーのトースト(02「作業時間」)。
export const worklogFailedMessage = "作業時間を保存できませんでした";

interface Target {
  storeId: string;
  itemId: string;
}

export interface WorklogsState {
  target: Target | null;
  rows: Worklog[];
  hours: string;
  memo: string;
  // error は、入力欄の下の赤字。時間の入力を変えるか、記録できたら消す。
  error: string;
  // deleting は、削除の確認。出していなければ null。lines は確認の本文。
  deleting: { id: string; lines: string[] } | null;
  // busy は、送信中(二重に記録しない)。
  busy: boolean;
}

export interface Worklogs {
  state: Readable<WorklogsState>;
  // open は、アイテムの作業時間を読む。アイテムが変わったら入力を空にする。同じアイテムなら入力は残す。
  open(storeId: string, itemId: string): Promise<void>;
  // reload は、画面への反映(06)。入力は書き換えない。
  reload(): Promise<void>;
  close(): void;
  setHours(text: string): void;
  setMemo(text: string): void;
  // add は、記録する。時間が不正なら、赤字を出して送らない(false)。
  add(): Promise<boolean>;
  askDelete(id: string): void;
  cancelDelete(): void;
  confirmDelete(): Promise<void>;
}

export interface WorklogsDeps {
  notes: Pick<NotesPort, "listWorklogs" | "addWorklog" | "deleteWorklog">;
  toasts: Toasts;
  // now は、削除の確認の日付を「今年なら M/D」にするための、このマシンの時刻。
  now: () => Date;
}

const initial = (): WorklogsState => ({
  target: null,
  rows: [],
  hours: "",
  memo: "",
  error: "",
  deleting: null,
  busy: false,
});

const same = (a: Target | null, b: Target) =>
  a !== null && a.storeId === b.storeId && a.itemId === b.itemId;

export function createWorklogs(deps: WorklogsDeps): Worklogs {
  const state = writable<WorklogsState>(initial());
  const patch = (p: Partial<WorklogsState>) => state.update((s) => ({ ...s, ...p }));
  const isOpen = (target: Target) => same(get(state).target, target);
  // 読み込みは重なりうるので、最後に始めたものの結果だけを使う。
  let latest = 0;

  async function load(target: Target) {
    latest += 1;
    const mine = latest;
    try {
      const rows = await deps.notes.listWorklogs(target.storeId, target.itemId);
      // 待っているあいだに別のアイテムへ移った(または閉じた)なら、この結果は捨てる。
      if (mine !== latest || !isOpen(target)) return;
      // 読み直したら消えていた記録の削除の確認は、続けられないので外す。
      state.update((s) => ({
        ...s,
        rows,
        deleting: rows.some((r) => r.id === s.deleting?.id) ? s.deleting : null,
      }));
    } catch (e) {
      // アイテムがなくなっていたら、詳細パネルが閉じるので何もしない。
      if (!(e instanceof NotFound)) throw e;
    }
  }

  // settled は、書き込みが終わったときの送信中の印の戻し。別のアイテムへ移っていたら、そちらの印を触らない。
  const settled = (target: Target) => {
    if (isOpen(target)) patch({ busy: false });
  };

  async function add(): Promise<boolean> {
    const { target, hours, memo, busy } = get(state);
    if (!target || busy) return false;
    const parsed = parseHours(hours);
    if (!parsed.ok) {
      patch({ error: parsed.message });
      return false;
    }
    patch({ busy: true, error: "" });
    try {
      await deps.notes.addWorklog(target.storeId, target.itemId, hours.trim(), memo.trim());
    } catch (e) {
      settled(target);
      if (e instanceof NotFound) await load(target);
      else if (!(e instanceof Refused)) deps.toasts.show(worklogFailedMessage, { error: true });
      else if (isOpen(target)) patch({ error: e.message });
      return false;
    }
    settled(target);
    // 送っているあいだに打ち換えた入力は残す(送った入力のままのときだけ空にする)。
    const now = get(state);
    if (isOpen(target) && now.hours === hours && now.memo === memo) patch({ hours: "", memo: "" });
    await load(target);
    return true;
  }

  async function confirmDelete() {
    const { target, deleting } = get(state);
    patch({ deleting: null });
    if (!target || !deleting) return;
    try {
      await deps.notes.deleteWorklog(target.storeId, target.itemId, deleting.id);
    } catch (e) {
      // ほかで消えていたら、読み直すだけにする。
      if (e instanceof NotFound) await load(target);
      else
        deps.toasts.show(e instanceof Refused ? e.message : worklogFailedMessage, { error: true });
      return;
    }
    deps.toasts.show(worklogDeletedMessage);
    await load(target);
  }

  return {
    state: readonly(state),
    open(storeId, itemId) {
      const target = { storeId, itemId };
      if (!isOpen(target)) state.set({ ...initial(), target });
      return load(target);
    },
    async reload() {
      const { target } = get(state);
      if (target) await load(target);
    },
    close() {
      latest += 1;
      state.set(initial());
    },
    setHours: (text) => patch({ hours: text, error: "" }),
    setMemo: (text) => patch({ memo: text }),
    add,
    askDelete(id) {
      const row = get(state).rows.find((r) => r.id === id);
      if (!row) return;
      const day = formatDay(row.date, deps.now().getFullYear());
      patch({ deleting: { id, lines: [`${day} の ${row.hours} 時間の記録を削除します。`] } });
    },
    cancelDelete: () => patch({ deleting: null }),
    confirmDelete,
  };
}
