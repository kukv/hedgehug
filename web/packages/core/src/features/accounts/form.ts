import type {
  Account,
  AccountValues,
  SignInCode,
  SignInFailure,
  SignInTarget,
} from "../../model/account";
import { NotFound, Refused } from "../../model/errors";
import type { AccountsPort } from "../../port/accounts";
import { get, type Readable, readonly, writable } from "../../reactive/store";
import type { Toasts } from "../toasts/toasts";
import { addedMessage, changedMessage } from "./text";

// 「アカウントを追加」「アカウントを編集」(07「登録」「認証の確認」「編集」)。検査と確かめはサーバーに任せ、
// 断りの文言を赤字で出す。認証方法がデバイスフローなら、コードを取って「GitHub にサインイン」へ渡す。
// 編集でログイン名を変えたら、先に「アカウントを変更」の確認を出す(D2 画面の計画の決定 S7)。

export interface AccountFormState {
  // original は、編集するアカウントのいまの値。追加では null。
  original: Account | null;
  form: AccountValues;
  error: string;
  // loginError は、ログイン名の欄の下の赤字(デバイスフローで別の人が許可したとき)。
  loginError: string;
  // asking は、GitHub に問い合わせているあいだ。「追加」「保存」を無効にする。
  asking: boolean;
  // confirm は、「アカウントを変更」の確認の本文の行。開いていなければ null。
  confirm: string[] | null;
}

export interface AccountForm {
  state: Readable<AccountFormState | null>;
  openAdd(): void;
  openEdit(account: Account): void;
  update(patch: Partial<AccountValues>): void;
  submit(): Promise<void>;
  // confirm は確認の「変更」、cancelConfirm は「キャンセル」(編集に戻る)。
  confirm(): Promise<void>;
  cancelConfirm(): void;
  // back は、サインインが別の人の許可・保存の失敗・コードを取れないことで終わったとき、元のダイアログに戻す。
  back(target: SignInTarget, failure: SignInFailure): void;
  close(): void;
}

export interface AccountFormDeps {
  accounts: AccountsPort;
  toasts: Toasts;
  // signIn は、取ったコードで「GitHub にサインイン」を開く。
  signIn(target: SignInTarget, code: SignInCode): void;
  // onSaved は、保存したあと(と、編集中のアカウントがなかったとき)に一覧を読み直す。
  onSaved(): void;
}

const pick = (a: Account): Account => ({
  id: a.id,
  name: a.name,
  login: a.login,
  method: a.method,
});

export function createAccountForm(deps: AccountFormDeps): AccountForm {
  const state = writable<AccountFormState | null>(null);
  // opened は、開くたび・閉じるたびに進める。問い合わせのあいだに閉じた・開き直したかを見分ける。
  let opened = 0;
  // busy は、送っているあいだ(二重に送らない)。
  let busy = false;
  const patch = (p: Partial<AccountFormState>) => state.update((s) => (s ? { ...s, ...p } : s));

  function open(original: Account | null, form: AccountValues, failure: SignInFailure = {}) {
    opened += 1;
    state.set({
      original,
      form,
      error: failure.error ?? "",
      loginError: failure.loginError ?? "",
      asking: false,
      confirm: null,
    });
  }

  function close() {
    opened += 1;
    state.set(null);
  }

  async function signIn(s: AccountFormState, mine: number) {
    const { original, form } = s;
    const code = await deps.accounts.startSignIn({
      account: original?.id,
      name: form.name,
      login: form.login,
    });
    if (mine !== opened) {
      void deps.accounts.cancelSignIn(code.id).catch(() => undefined);
      return;
    }
    close();
    deps.signIn(
      { purpose: original ? "edit" : "add", original, name: form.name, login: form.login },
      code,
    );
  }

  async function saveToken(s: AccountFormState, mine: number) {
    const { original, form } = s;
    if (form.method === "device") return;
    const values = { name: form.name, login: form.login, method: form.method };
    if (original) await deps.accounts.edit(original.id, values);
    else await deps.accounts.add(values);
    if (mine === opened) close();
    deps.toasts.show(original ? changedMessage : addedMessage);
    deps.onSaved();
  }

  // save は、確かめて保存する。mine は送り始めたときの opened(あいだに閉じたら、結果をダイアログに出さない)。
  async function save(s: AccountFormState, mine: number) {
    patch({ asking: true, error: "", loginError: "", confirm: null });
    try {
      if (s.form.method === "device") await signIn(s, mine);
      else await saveToken(s, mine);
    } catch (e) {
      if (mine === opened) patch({ asking: false });
      if (e instanceof Refused) {
        if (mine === opened) patch({ error: e.message });
        return;
      }
      if (!(e instanceof NotFound)) throw e;
      if (mine === opened) close();
      deps.onSaved();
    }
  }

  // needsConfirm は、ログイン名を変えた編集で「あなた」が変わる Store があれば、確認の行を出して true を返す。
  async function needsConfirm(s: AccountFormState, mine: number): Promise<boolean> {
    if (!s.original || s.form.login === s.original.login) return false;
    const lines = await deps.accounts.editImpact(s.original.id, s.form.login);
    if (mine !== opened) return true;
    if (lines.length === 0) return false;
    patch({ confirm: lines });
    return true;
  }

  return {
    state: readonly(state),
    openAdd: () => open(null, { name: "", login: "", method: "device" }),
    openEdit: (a) => open(pick(a), { name: a.name, login: a.login, method: a.method }),
    update(p) {
      state.update((s) => (s ? { ...s, form: { ...s.form, ...p } } : s));
    },
    async submit() {
      const s = get(state);
      if (!s || busy) return;
      busy = true;
      const mine = opened;
      try {
        if (await needsConfirm(s, mine)) return;
        await save(s, mine);
      } catch (e) {
        if (!(e instanceof NotFound)) throw e;
        if (mine === opened) close();
        deps.onSaved();
      } finally {
        busy = false;
      }
    },
    async confirm() {
      const s = get(state);
      if (!s?.confirm || busy) return;
      busy = true;
      try {
        await save(s, opened);
      } finally {
        busy = false;
      }
    },
    cancelConfirm: () => patch({ confirm: null }),
    back: (target, failure) =>
      open(target.original, { name: target.name, login: target.login, method: "device" }, failure),
    close,
  };
}
