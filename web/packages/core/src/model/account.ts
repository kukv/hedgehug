// GitHub のアカウント(07「アカウント」)。トークンは画面に来ない。

export type AuthMethod = "device" | "gh" | "env" | "keychain";
// TokenMethod は、利用者が置いたトークンを読む認証方法。デバイスフローはサインインで登録・保存する。
export type TokenMethod = Exclude<AuthMethod, "device">;

export interface Account {
  id: string;
  name: string;
  login: string;
  method: AuthMethod;
}

// AccountRow は一覧の 1 行。reason は「認証エラー」の下の赤字(なければ空)。
export interface AccountRow extends Account {
  status: "ok" | "auth_error" | "app_missing";
  reason: string;
  orgs: string[]; // GitHub App が入っていない組織(認証エラーのときは空)
}

export interface AccountValues {
  name: string;
  login: string;
  method: AuthMethod;
}

export interface TokenValues {
  name: string;
  login: string;
  method: TokenMethod;
}

// SignInStart は、サインインを始める入力。account は編集・再認証するアカウントの ID(追加では省く)。
export interface SignInStart {
  account?: string;
  name: string;
  login: string;
}

export interface SignInCode {
  id: string;
  userCode: string;
  verificationUri: string;
  expiresInSec: number;
  intervalSec: number;
}

export type SignInStatus = "waiting" | "expired" | "denied" | "mismatch" | "failed" | "done";

// SignInResult は、サインインの問い合わせの答え。account は done のときだけ。
export interface SignInResult {
  status: SignInStatus;
  message: string;
  intervalSec: number;
  account: Account | null;
}

export type SignInPurpose = "add" | "edit" | "reauth";

// SignInTarget は、サインインで登録・保存するもの。original は編集・再認証するアカウント(追加では null)。
export interface SignInTarget {
  purpose: SignInPurpose;
  original: Account | null;
  name: string;
  login: string;
}

// SignInFailure は、サインインが終わったときの赤字。loginError はログイン名の欄の下、error はダイアログの赤字。
export interface SignInFailure {
  error?: string;
  loginError?: string;
}
