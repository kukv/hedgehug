import type {
  Account,
  SignInCode,
  SignInFailure,
  SignInResult,
  SignInStart,
  SignInTarget,
} from "../../model/account";
import { NotFound, Refused } from "../../model/errors";
import type { AccountsPort } from "../../port/accounts";
import { get, type Readable, readonly, writable } from "../../reactive/store";
import type { Timer, Toasts } from "../toasts/toasts";
import { changedMessage, reauthedMessage, signedInMessage } from "./text";

// 「GitHub にサインイン」(07「デバイスフロー」)。サーバーの流れに、サーバーが返す間隔ごとに 1 回ずつ問い合わせる
// (同時に 1 つ)。コードの期限が来たら、間隔を待たずに問い合わせる(D2 画面の計画の決定 S8〜S12)。

// SignInEnd は、待つのをやめた理由。failed は再認証の失敗(error に文言)。
export type SignInEnd = "" | "expired" | "denied" | "failed";

export interface SignInDialogState {
  target: SignInTarget;
  code: SignInCode;
  ended: SignInEnd;
  error: string;
  // retrying は、「再試行」で新しいコードを取っているあいだ。
  retrying: boolean;
}

export interface SignIn {
  state: Readable<SignInDialogState | null>;
  // begin は、取ったコードで開き、許可を待ち始める。
  begin(target: SignInTarget, code: SignInCode): void;
  // reauth は、デバイスフローの行の「再認証」。コードを取ってから開く。取れなければ Refused を投げる。
  reauth(account: Account): Promise<void>;
  retry(): Promise<void>;
  // close は、「キャンセル」・Esc・外のクリック・画面の移動。サーバーの流れも捨てる(何も登録しない)。
  close(): void;
}

export interface SignInDeps {
  accounts: AccountsPort;
  toasts: Toasts;
  timer: Timer;
  // back は、追加・編集のサインインが失敗で終わったとき、元のダイアログに戻して赤字を出す。
  back(target: SignInTarget, failure: SignInFailure): void;
  // onDone は、サインインが済んだあとに一覧を読み直す。
  onDone(): void;
}

const second = 1000;

const startOf = (t: SignInTarget): SignInStart => ({
  account: t.original?.id,
  name: t.name,
  login: t.login,
});

function doneMessage(target: SignInTarget, a: Account): string {
  if (target.purpose === "add") return signedInMessage(a.login);
  if (target.purpose === "edit") return changedMessage;
  return reauthedMessage(a);
}

export function createSignIn(deps: SignInDeps): SignIn {
  const state = writable<SignInDialogState | null>(null);
  // flow は、いま待っているサーバーの流れの ID。待っていなければ null。
  let flow: string | null = null;
  let next: unknown = null;
  let expiry: unknown = null;
  // asking は、問い合わせ中の流れの ID。別の流れの問い合わせとは重ねて数えない。
  let asking: string | null = null;
  // again は、問い合わせのあいだに期限が来たとき、答えを受けたらすぐ問い合わせ直す印。
  let again = false;
  // generation は、begin と close のたびに進める。コードを待つあいだに閉じた・別のサインインを始めたかを見分ける。
  let generation = 0;
  const patch = (p: Partial<SignInDialogState>) => state.update((s) => (s ? { ...s, ...p } : s));

  function stop() {
    if (next !== null) deps.timer.clear(next);
    if (expiry !== null) deps.timer.clear(expiry);
    next = null;
    expiry = null;
    again = false;
  }

  // drop は、流れを捨てる(問い合わせを止め、サーバーにも捨てさせる)。
  function drop() {
    const id = flow;
    flow = null;
    stop();
    if (id !== null) void deps.accounts.cancelSignIn(id).catch(() => undefined);
  }

  function wait(sec: number) {
    next = deps.timer.set(() => {
      next = null;
      void poll();
    }, sec * second);
  }

  function begin(target: SignInTarget, code: SignInCode) {
    generation += 1;
    drop();
    flow = code.id;
    state.set({ target, code, ended: "", error: "", retrying: false });
    wait(code.intervalSec);
    expiry = deps.timer.set(() => {
      expiry = null;
      void poll();
    }, code.expiresInSec * second);
  }

  function end(ended: SignInEnd, error = "") {
    drop();
    patch({ ended, error, retrying: false });
  }

  function fail(target: SignInTarget, failure: SignInFailure) {
    if (target.purpose === "reauth") {
      end("failed", failure.loginError ?? failure.error ?? "");
      return;
    }
    drop();
    state.set(null);
    deps.back(target, failure);
  }

  function finish(target: SignInTarget, account: Account | null) {
    flow = null;
    stop();
    state.set(null);
    const a: Account = account ?? {
      id: target.original?.id ?? "",
      name: target.name,
      login: target.login,
      method: "device",
    };
    deps.toasts.show(doneMessage(target, a));
    deps.onDone();
  }

  function answer(target: SignInTarget, r: SignInResult) {
    switch (r.status) {
      case "waiting":
        if (again) {
          again = false;
          void poll();
        } else wait(r.intervalSec);
        return;
      case "expired":
      case "denied":
        end(r.status);
        return;
      case "done":
        finish(target, r.account);
        return;
      case "mismatch":
        fail(target, { loginError: r.message });
        return;
      case "failed":
        fail(target, { error: r.message });
        return;
    }
  }

  // failed は、問い合わせ自体が失敗したとき。届かなければ待ち続け、流れがなければ期限切れ、断りは失敗と同じ(決定 S9)。
  function failed(s: SignInDialogState, e: unknown) {
    if (e instanceof NotFound) end("expired");
    else if (e instanceof Refused) fail(s.target, { error: e.message });
    else {
      again = false;
      wait(s.code.intervalSec);
    }
  }

  async function poll() {
    const id = flow;
    const s = get(state);
    if (id === null || s === null) return;
    if (asking === id) {
      again = true;
      return;
    }
    if (next !== null) {
      deps.timer.clear(next);
      next = null;
    }
    asking = id;
    let r: SignInResult;
    try {
      r = await deps.accounts.pollSignIn(id);
    } catch (e) {
      if (asking === id) asking = null;
      if (id === flow) failed(s, e);
      return;
    }
    if (asking === id) asking = null;
    if (id === flow) answer(s.target, r);
    // 閉じたあとに済んだと分かったら、一覧だけ読み直す(決定 S10)。
    else if (r.status === "done") deps.onDone();
  }

  return {
    state: readonly(state),
    begin,
    async reauth(account) {
      const target: SignInTarget = {
        purpose: "reauth",
        original: {
          id: account.id,
          name: account.name,
          login: account.login,
          method: account.method,
        },
        name: account.name,
        login: account.login,
      };
      const mine = generation;
      let code: SignInCode;
      try {
        code = await deps.accounts.startSignIn(startOf(target));
      } catch (e) {
        if (mine !== generation && e instanceof Refused) return;
        throw e;
      }
      if (mine !== generation) {
        void deps.accounts.cancelSignIn(code.id).catch(() => undefined);
        return;
      }
      begin(target, code);
    },
    async retry() {
      const s = get(state);
      if (!s || s.ended === "" || s.retrying) return;
      patch({ retrying: true, error: "" });
      const mine = generation;
      try {
        const code = await deps.accounts.startSignIn(startOf(s.target));
        if (mine !== generation) {
          void deps.accounts.cancelSignIn(code.id).catch(() => undefined);
          return;
        }
        begin(s.target, code);
      } catch (e) {
        if (mine !== generation) {
          if (e instanceof Refused) return;
          throw e;
        }
        if (!(e instanceof Refused)) {
          patch({ retrying: false });
          throw e;
        }
        fail(s.target, { error: e.message });
      }
    },
    close() {
      generation += 1;
      drop();
      state.set(null);
    },
  };
}
