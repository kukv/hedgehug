import { createAccountsPort } from "@hedgehug/api";
import { createAccountForm, createAccountList, createSignIn } from "@hedgehug/core";
import { timer, toasts } from "./wireBase";

// 設定「アカウントとメンバー」の GitHub アカウント(07)。wire.ts が 300 行の上限に近いので分けた(wire.ts が再公開する)。
// 3 つは互いを呼ぶので、呼ぶのは作ったあと。
const accountsPort = createAccountsPort();
export const accountList = createAccountList({
  accounts: accountsPort,
  toasts,
  reauthDevice: (account) => signIn.reauth(account),
});
export const accountForm = createAccountForm({
  accounts: accountsPort,
  toasts,
  signIn: (target, code) => signIn.begin(target, code),
  onSaved: () => void accountList.reload(),
});
export const signIn = createSignIn({
  accounts: accountsPort,
  toasts,
  timer,
  back: (target, failure) => accountForm.back(target, failure),
  onDone: () => void accountList.reload(),
});
