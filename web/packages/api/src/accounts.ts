import type { Account, AccountRow, AccountsPort, SignInResult } from "@hedgehug/core";
import { call } from "./errors";
import {
  type AccountBody,
  accountDeleteImpact,
  accountEditImpact,
  addAccount,
  cancelSignIn,
  deleteAccount,
  editAccount,
  listAccounts,
  pollSignIn,
  reauthAccount,
  startSignIn,
} from "./generated";

// アカウントの口(07「アカウント」)。サインインの問い合わせ(GET)は登録の副作用を持つので、
// キャッシュに乗せない(D2 画面の計画の決定 S8)。

const toAccount = (a: AccountBody): Account => ({
  id: a.id,
  name: a.name,
  login: a.login,
  method: a.method,
});

export function createAccountsPort(): AccountsPort {
  return {
    list: () =>
      call(async () =>
        ((await listAccounts()).accounts ?? []).map(
          (r): AccountRow => ({
            ...toAccount(r),
            status: r.status,
            reason: r.reason,
            orgs: r.orgs ?? [],
          }),
        ),
      ),
    add: (values) => call(async () => toAccount(await addAccount(values))),
    edit: (id, values) => call(async () => toAccount(await editAccount(id, values))),
    reauth: (id) => call(async () => toAccount(await reauthAccount(id))),
    remove: (id) => call(() => deleteAccount(id)),
    deleteImpact: (id) => call(async () => (await accountDeleteImpact(id)).lines ?? []),
    editImpact: (id, login) =>
      call(async () => (await accountEditImpact(id, { login })).lines ?? []),
    startSignIn: (input) =>
      call(async () => {
        const c = await startSignIn(input);
        return {
          id: c.id,
          userCode: c.user_code,
          verificationUri: c.verification_uri,
          expiresInSec: c.expires_in,
          intervalSec: c.interval,
        };
      }),
    pollSignIn: (id) =>
      call(async (): Promise<SignInResult> => {
        const r = await pollSignIn(id, { cache: "no-store" });
        return {
          status: r.state,
          message: r.message,
          intervalSec: r.interval,
          account: r.account ? toAccount(r.account) : null,
        };
      }),
    cancelSignIn: (id) => call(() => cancelSignIn(id)),
  };
}
