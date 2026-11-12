import type { Account, AccountRow } from "../../model/account";
import { NotFound, Refused } from "../../model/errors";
import type { AccountsPort } from "../../port/accounts";
import { get, type Readable, readonly, writable } from "../../reactive/store";
import type { Toasts } from "../toasts/toasts";
import { deletedMessage, reauthedMessage } from "./text";

// 設定「アカウントとメンバー」の「GitHub アカウント」の一覧(07「状態」「再認証」「削除の影響」)。

export interface AccountListState {
  rows: AccountRow[];
  // errors は、行の中(状態の下)の赤字。行の次の操作・画面を開き直す・成功のどれかで消す(D2 画面の計画の決定 S14)。
  errors: Record<string, string>;
  // asking は、GitHub に問い合わせている行の ID。そのあいだ「再認証」を無効にする。
  asking: string[];
  // deleting は、「アカウントを削除」の確認。開いていなければ null。
  deleting: { id: string; lines: string[] } | null;
}

export interface AccountList {
  state: Readable<AccountListState>;
  // open は、設定画面を開いたときに呼ぶ。行の赤字を消して読み直す。
  open(): Promise<void>;
  reload(): Promise<void>;
  // touch は、行の「編集」などを押したときに、その行の赤字を消す。
  touch(id: string): void;
  reauth(id: string): Promise<void>;
  askDelete(id: string): Promise<void>;
  confirmDelete(): Promise<void>;
  cancelDelete(): void;
}

export interface AccountListDeps {
  accounts: AccountsPort;
  toasts: Toasts;
  // reauthDevice は、デバイスフローの行の「再認証」。コードを取って「GitHub にサインイン」を開く。
  // コードを取れなければ Refused を投げる(行の赤字にする。決定 S13)。
  reauthDevice(account: Account): Promise<void>;
}

const initial = (): AccountListState => ({ rows: [], errors: {}, asking: [], deleting: null });

export function createAccountList(deps: AccountListDeps): AccountList {
  const state = writable<AccountListState>(initial());
  const patch = (p: Partial<AccountListState>) => state.update((s) => ({ ...s, ...p }));

  function setError(id: string, message: string) {
    state.update((s) => {
      const errors = Object.fromEntries(Object.entries(s.errors).filter(([k]) => k !== id));
      return { ...s, errors: message === "" ? errors : { ...errors, [id]: message } };
    });
  }

  function setAsking(id: string, on: boolean) {
    state.update((s) => ({
      ...s,
      asking: on ? [...s.asking, id] : s.asking.filter((x) => x !== id),
    }));
  }

  async function reload() {
    patch({ rows: await deps.accounts.list() });
  }

  async function ask(row: AccountRow) {
    if (row.method === "device") {
      await deps.reauthDevice(row);
      return;
    }
    deps.toasts.show(reauthedMessage(await deps.accounts.reauth(row.id)));
    await reload();
  }

  return {
    state: readonly(state),
    async open() {
      state.set(initial());
      await reload();
    },
    reload,
    touch: (id) => setError(id, ""),
    async reauth(id) {
      const row = get(state).rows.find((r) => r.id === id);
      if (!row || get(state).asking.includes(id)) return;
      setError(id, "");
      setAsking(id, true);
      try {
        await ask(row);
      } catch (e) {
        if (e instanceof Refused) setError(id, e.message);
        else if (e instanceof NotFound) await reload();
        else throw e;
      } finally {
        setAsking(id, false);
      }
    },
    async askDelete(id) {
      setError(id, "");
      try {
        patch({ deleting: { id, lines: await deps.accounts.deleteImpact(id) } });
      } catch (e) {
        if (!(e instanceof NotFound)) throw e;
        await reload();
      }
    },
    async confirmDelete() {
      const d = get(state).deleting;
      if (!d) return;
      patch({ deleting: null });
      try {
        await deps.accounts.remove(d.id);
        deps.toasts.show(deletedMessage);
      } catch (e) {
        if (!(e instanceof NotFound)) throw e;
      }
      await reload();
    },
    cancelDelete: () => patch({ deleting: null }),
  };
}
