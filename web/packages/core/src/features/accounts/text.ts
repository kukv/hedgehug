import type { AuthMethod } from "../../model/account";

// アカウントの画面の文言(07「アカウント」)。

// methodChoices は、「認証方法」の選択肢。並びは 07「登録」の表の順で、既定はデバイスフロー。
export const methodChoices: { value: AuthMethod; label: string }[] = [
  { value: "device", label: "ブラウザでサインイン(デバイスフロー)" },
  { value: "gh", label: "gh CLI のトークン" },
  { value: "env", label: "環境変数" },
  { value: "keychain", label: "OS のキーチェーン" },
];

const listWords: Record<AuthMethod, string> = {
  device: "デバイスフローのトークン",
  gh: "gh CLI のトークン",
  env: "環境変数 GH_TOKEN",
  keychain: "OS のキーチェーン",
};

// methodLine は、一覧の行の「github.com · {一覧に出す文言}」。
export const methodLine = (m: AuthMethod) => `github.com · ${listWords[m]}`;

export const addedMessage = "アカウントを追加しました";
export const changedMessage = "アカウントを変更しました";
export const deletedMessage = "アカウントを削除しました";
export const signedInMessage = (login: string) => `@${login} としてサインインしました`;
export const reauthedMessage = (a: { name: string; login: string }) =>
  `${a.name}: @${a.login} として再認証しました`;

export const askingText = "GitHub に問い合わせています";
export const waitingText = "許可を待っています";
export const expiredText = "コードの期限が切れました";
export const deniedText = "GitHub で許可が拒否されました";
