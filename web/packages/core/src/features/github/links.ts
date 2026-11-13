import { NotFound, Refused } from "../../model/errors";
import { type LinkRow, linkKey } from "../../model/link";
import type { AccountsPort } from "../../port/accounts";
import type { LinksPort } from "../../port/links";
import { type Readable, readonly, writable } from "../../reactive/store";
import type { Toasts } from "../toasts/toasts";
import { unlinkedMessage, unlinkFailedMessage } from "./text";

// 詳細パネルの GitHub の欄(07「表示と更新」「解除」)の状態: 一覧、取り直しの失敗、アカウントの有無。

export interface LinkListState {
  // loaded は、一覧を読み終えたか(読むまでは「リンクはありません」を出さない)。
  loaded: boolean;
  rows: LinkRow[];
  // hasAccount は、このマシンにアカウントが 1 つ以上あるか(なければリンクの追加を無効にする。07)。
  hasAccount: boolean;
}

export interface LinkList {
  state: Readable<LinkListState>;
  // load は、一覧を出してから取り直す。
  load(storeId: string, itemId: string): Promise<void>;
  // reload は、一覧だけを読む(画面への反映)。
  reload(): Promise<void>;
  remove(row: LinkRow): Promise<void>;
  // forgetAll は、覚えた取り直しの失敗の文言をすべて消す(リンクしたあと、読み直す前に呼ぶ。付け直した行に
  // 前の失敗を残さない)。
  forgetAll(): void;
  reset(): void;
}

export interface LinkListDeps {
  links: LinksPort;
  accounts: Pick<AccountsPort, "list">;
  toasts: Toasts;
  // onChanged は、解除したあと(プロジェクトの概要の一覧を読み直す)。
  onChanged(): void;
}

const blank: LinkListState = { loaded: false, rows: [], hasAccount: true };

export function createLinkList(deps: LinkListDeps): LinkList {
  const state = writable<LinkListState>(blank);
  let target: { storeId: string; itemId: string } | null = null;
  // errors は、最後の取り直しの失敗の文言(一覧だけを読み直しても、行に残す)。
  let errors = new Map<string, string>();
  // lists・refreshes は、古い答えを捨てるための通し番号。一覧と取り直しは別に数える(取り直しの途中に画面への
  // 反映の一覧が入っても、取り直しの答えと失敗の文言を捨てない)。取り直しの答えを出したら、それより前に
  // 出した一覧の答え(古い値)も捨てる。
  let lists = 0;
  let refreshes = 0;

  const withErrors = (rows: LinkRow[]) =>
    rows.map((r) => ({ ...r, error: r.error || errors.get(linkKey(r)) || "" }));
  const show = (rows: LinkRow[]) =>
    state.update((s) => ({ ...s, loaded: true, rows: withErrors(rows) }));

  // list は、一覧を出す。失敗は出さない(画面への反映は失敗を出さない。A2 の決定 29)。
  async function list() {
    if (!target) return;
    const t = target;
    lists += 1;
    const mine = lists;
    try {
      const rows = await deps.links.list(t.storeId, t.itemId);
      if (mine === lists && target === t) show(rows);
    } catch (e) {
      if (e instanceof NotFound && mine === lists && target === t) show([]);
    }
  }

  // refresh は、GitHub から取り直した一覧を出し、失敗の文言を覚える。
  async function refresh() {
    if (!target) return;
    const t = target;
    refreshes += 1;
    const mine = refreshes;
    try {
      const rows = await deps.links.refresh(t.storeId, t.itemId);
      if (mine !== refreshes || target !== t) return;
      lists += 1;
      errors = new Map(rows.filter((r) => r.error).map((r) => [linkKey(r), r.error]));
      show(rows);
    } catch (e) {
      if (e instanceof NotFound && mine === refreshes && target === t) show([]);
    }
  }

  async function readAccounts() {
    try {
      const rows = await deps.accounts.list();
      if (target) state.update((s) => ({ ...s, hasAccount: rows.length > 0 }));
    } catch {
      // 読めなければ、いまの表示のまま(サーバーが断る)。
    }
  }

  return {
    state: readonly(state),
    async load(storeId, itemId) {
      if (target?.storeId !== storeId || target.itemId !== itemId) {
        target = { storeId, itemId };
        errors = new Map();
        state.set(blank);
      }
      await Promise.all([list(), readAccounts()]);
      await refresh();
    },
    reload: list,
    async remove(row) {
      if (!target) return;
      try {
        await deps.links.remove(target.storeId, target.itemId, row);
        errors.delete(linkKey(row));
        deps.toasts.show(unlinkedMessage);
      } catch (e) {
        if (!(e instanceof NotFound)) {
          deps.toasts.show(e instanceof Refused ? e.message : unlinkFailedMessage, { error: true });
        }
      }
      await list();
      deps.onChanged();
    },
    forgetAll() {
      errors = new Map();
    },
    reset() {
      target = null;
      lists += 1;
      refreshes += 1;
      errors = new Map();
      state.set(blank);
    },
  };
}
