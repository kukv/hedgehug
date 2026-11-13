import { createAccountsPort } from "@hedgehug/api";
import { createAccountForm, createAccountList, createSignIn } from "@hedgehug/core";
import { githubReloads } from "./reloads";
import { navigation } from "./wire";
import { timer, toasts } from "./wireBase";

// 設定「アカウントとメンバー」の GitHub アカウント(07)。wire.ts が 300 行なので、ここで作る。
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
// github の知らせ(認証エラーの確かめ直しで戻ったアカウントなど。D4a-18)で、一覧を読み直す。
// wire.ts はこのファイルを import できない(ここが wire.ts を使う)ので、reloads.ts に登録する。
githubReloads.add(() => accountList.reload());

// 画面を移ったら、アカウントのダイアログを閉じる(01「画面の移り方」。wire.ts のほかのダイアログと同じ)。
navigation.route.subscribe(() => {
  accountForm.close();
  signIn.close();
  accountList.cancelDelete();
});
