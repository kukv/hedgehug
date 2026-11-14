import { NotFound } from "../../model/errors";
import type { InboxCounts, InboxPage, InboxRow, InboxRowKind, InboxTab } from "../../model/inbox";
import type { InboxPort } from "../../port/inbox";
import { get, type Readable, readonly, writable } from "../../reactive/store";
import type { Toasts } from "../toasts/toasts";

// 受信箱の画面の状態(06「受信箱」)。行は新しい順に 300 件ずつ読み、「さらに読み込む」で続きを足す。

export const inboxTabs: { id: InboxTab; label: string }[] = [
  { id: "all", label: "すべて" },
  { id: "mention", label: "メンション" },
  { id: "assign", label: "割り当て" },
  { id: "watch", label: "ウォッチ" },
  { id: "overwrite", label: "上書き" },
  { id: "automation", label: "自動化" },
  { id: "repair", label: "同期の修復" },
];

const pills: Record<InboxRowKind, string> = {
  mention: "メンション",
  assign: "割り当て",
  watch: "ウォッチ",
  overwrite: "上書き",
  automation: "自動化",
  // 自動化が制約で止まった通知も「自動化」。
  automation_stopped: "自動化",
  repair: "同期の修復",
};

// pillOf は、行の種類のピルの名前。
export const pillOf = (kind: InboxRowKind): string => pills[kind];

export const inboxEmptyTitle = "お知らせはありません";
const emptyNotes: Partial<Record<InboxTab, string>> = {
  overwrite:
    "このマシンで変えた値が、同期であとから届いた変更に負けると、ここに出ます。試すには、値を変えてからサイドバーの同期ボタンを押してください。",
  repair:
    "同期のときに直したもの(番号の重なり、削除された親、親の循環、移動した親に合わせて移した子など)がここに出ます。",
};

// emptyOf は、行のないタブに出す文。
export function emptyOf(tab: InboxTab): { title: string; note?: string } {
  const note = emptyNotes[tab];
  return note ? { title: inboxEmptyTitle, note } : { title: inboxEmptyTitle };
}

export const inboxAppliedMessage = "適用しました";

export interface InboxState {
  tab: InboxTab;
  rows: InboxRow[];
  // more は、「さらに読み込む」を出すか。loading は、タブの最初の行か続きを読んでいるあいだ。
  more: boolean;
  loading: boolean;
  // loaded は、いまのタブの一覧を読み込めたか。読み込みに失敗したままのときは false(空の表示を出さない)。
  loaded: boolean;
}

export interface Inbox {
  state: Readable<InboxState>;
  // unread は、タブごとの未読数。canReadAll は、いまのタブの未読が 1 以上か(「すべて既読にする」を押せるか)。
  unread: Readable<InboxCounts>;
  canReadAll: Readable<boolean>;
  // open は、画面を開いたときに、いまのタブの行を読む。open・select・more は、読めなかったらエラーのトーストを出し、投げない。
  open(): Promise<void>;
  select(tab: InboxTab): Promise<void>;
  more(): Promise<void>;
  // toggle は左端の点、markRead は行のクリック(未読のときだけ既読にする)。
  toggle(row: InboxRow): Promise<void>;
  markRead(row: InboxRow): Promise<void>;
  readAll(): Promise<void>;
  // apply は「制約を無視して…」、keep は「変更しない」。
  apply(row: InboxRow): Promise<void>;
  keep(row: InboxRow): Promise<void>;
  // reload は、画面への反映(出来事 inbox)と操作のあと。出していた件数を覆うまで読み直す。
  reload(): Promise<void>;
  reloadUnread(): Promise<void>;
}

export interface InboxDeps {
  port: InboxPort;
  toasts: Toasts;
}

const zero: InboxCounts = {
  all: 0,
  mention: 0,
  assign: 0,
  watch: 0,
  overwrite: 0,
  automation: 0,
  repair: 0,
};

export function createInbox(deps: InboxDeps): Inbox {
  const state = writable<InboxState>({
    tab: "all",
    rows: [],
    more: false,
    loading: false,
    loaded: false,
  });
  const unread = writable<InboxCounts>(zero);
  const canReadAll = writable(false);
  // loads は、古い答えを捨てるための通し番号。
  let loads = 0;

  const publishUnread = (counts: InboxCounts) => {
    unread.set(counts);
    canReadAll.set(counts[get(state).tab] > 0);
  };

  // failed は、読み込みの失敗をエラーのトーストで出す。画面に未処理の拒否を残さないため、投げない。
  // 古い読み込みの失敗は、いまの表示と関係ないので出さない。
  function failed(mine: number, e: unknown) {
    if (mine !== loads) return;
    state.update((s) => ({ ...s, loading: false }));
    deps.toasts.show(e instanceof Error ? e.message : String(e), { error: true });
  }

  // first は、タブの最初の 300 件を読む。
  async function first(tab: InboxTab) {
    loads += 1;
    const mine = loads;
    state.set({ tab, rows: [], more: false, loading: true, loaded: false });
    canReadAll.set(get(unread)[tab] > 0);
    try {
      const page = await deps.port.list(tab);
      if (mine !== loads) return;
      state.set({ tab, rows: page.rows, more: page.more, loading: false, loaded: true });
      publishUnread(page.unread);
    } catch (e) {
      failed(mine, e);
    }
  }

  // again は、いまのタブを、出していた件数を覆うまで読み直す(続きの行を出したまま位置を保つ)。
  // 失敗したら、読み込み中の表示だけを戻して投げる(最初の読み込みの途中で失敗しても、読み込み中のまま残さない)。
  async function again() {
    loads += 1;
    const mine = loads;
    const { tab, rows: shown } = get(state);
    let rows: InboxRow[] = [];
    let page: InboxPage;
    try {
      do {
        const last = rows.at(-1);
        page = await deps.port.list(tab, last && { at: last.at, id: last.id });
        if (mine !== loads) return;
        rows = [...rows, ...page.rows];
      } while (page.more && rows.length < shown.length);
    } catch (e) {
      if (mine === loads) state.update((s) => ({ ...s, loading: false }));
      throw e;
    }
    state.set({ tab, rows, more: page.more, loading: false, loaded: true });
    publishUnread(page.unread);
  }

  // write は、書き込みのあとに読み直す。ないものは黙り、ほかの失敗(断りを含む)はその文言をエラーのトーストで
  // 出す。画面に未処理の拒否を残さないため、投げない。
  async function write(fn: () => Promise<void>, done?: () => void) {
    try {
      await fn();
      done?.();
    } catch (e) {
      if (!(e instanceof NotFound))
        deps.toasts.show(e instanceof Error ? e.message : String(e), { error: true });
    }
    await again().catch(() => {});
  }

  return {
    state: readonly(state),
    unread: readonly(unread),
    canReadAll: readonly(canReadAll),
    open: () => first(get(state).tab),
    select: (tab) => first(tab),
    async more() {
      const s = get(state);
      const last = s.rows.at(-1);
      if (!s.more || s.loading || !last) return;
      loads += 1;
      const mine = loads;
      state.set({ ...s, loading: true });
      try {
        const page = await deps.port.list(s.tab, { at: last.at, id: last.id });
        if (mine !== loads) return;
        state.set({
          tab: s.tab,
          rows: [...s.rows, ...page.rows],
          more: page.more,
          loading: false,
          loaded: true,
        });
        publishUnread(page.unread);
      } catch (e) {
        failed(mine, e);
      }
    },
    toggle: (row) => write(() => deps.port.setRead(row.id, !row.read)),
    async markRead(row) {
      if (!row.read) await write(() => deps.port.setRead(row.id, true));
    },
    readAll: () => write(() => deps.port.readAll(get(state).tab)),
    apply: (row) =>
      write(
        () => deps.port.apply(row.id),
        () => deps.toasts.show(inboxAppliedMessage),
      ),
    keep: (row) => write(() => deps.port.keep(row.id)),
    // 画面への反映は失敗を出さない(A2 の決定 29)。
    reload: () => again().catch(() => {}),
    async reloadUnread() {
      try {
        publishUnread(await deps.port.unread());
      } catch {
        // 画面への反映は失敗を出さない(A2 の決定 29)。
      }
    },
  };
}
